package paxos

import (
	"errors"
	"fmt"
	"github.com/gobby/src/command"
	"github.com/gobby/src/config"
	"github.com/gobby/src/rpc/paxosrpc"
	"github.com/gobby/src/rpc/rpcwrapper"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"strconv"
	"sync"
	"time"
)

var (
	//LOGE = log.New(os.Stderr, "ERROR", log.Lmicroseconds|log.Lshortfile)
	LOGE = log.New(ioutil.Discard, "ERROR", log.Lmicroseconds|log.Lshortfile)
	LOGV = log.New(os.Stdout, "VERBOSE", log.Lmicroseconds|log.Lshortfile)
	//LOGV2 = log.New(os.Stdout, "VERBOSE", log.Lmicroseconds|log.Lshortfile)
	LOGV2 = log.New(ioutil.Discard, "VERBOSE", log.Lmicroseconds|log.Lshortfile)
	_     = ioutil.Discard
)

//TODO:move this to paxosrpc, in order to implement dynamically add node
type Node struct {
	HostPort string
	NodeID   int
}

type Gap struct {
	from, to int //[from, to)
}

type IndexCommand struct {
	Index      int
	Na, Nh     int
	V          command.Command
	isAccepted bool
	isCommited bool
}

type paxosNode struct {
	nodeID, port, numNodes int
	addrport               string

	peers            []Node               //including itself
	commitedCommands []command.Command    //Log in memory
	tempSlots        map[int]IndexCommand //CommandSlotIndex -> Na
	catchupCounter   int                  //Used to indicate how many catchup routine is running. And a "master" node should have no catchup routine for read optimization

	cmdMutex     sync.Mutex
	catchupMutex sync.Mutex //Lock of catchup counter

	callback PaxosCallBack
	gapchan  chan Gap

	listener *net.Listener
}

//Current setting: all settings are static
func NewPaxosNode(nodeID int, numNodes int, callback PaxosCallBack) (PaxosNode, error) {
	node := paxosNode{}

	node.nodeID = nodeID
	node.port = config.Nodes[nodeID].Port
	node.numNodes = numNodes
	node.addrport = config.Nodes[nodeID].Address + ":" + strconv.Itoa(node.port)
	node.catchupCounter = 0
	node.callback = callback

	node.peers = make([]Node, node.numNodes)
	for i := 0; i < numNodes; i++ {
		node.peers[i].HostPort = config.Nodes[i].Address + ":" + strconv.Itoa(config.Nodes[i].Port)
		node.peers[i].NodeID = i
	}

	file, _ := os.OpenFile(strconv.Itoa(nodeID)+"file.txt", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	LOGV = log.New(file, "VERBOSE", log.Lmicroseconds|log.Lshortfile)
	node.commitedCommands = make([]command.Command, 0)
	node.tempSlots = make(map[int]IndexCommand)
	node.gapchan = make(chan Gap)

	//rpc
	LOGV.Printf("Node %d tried listen on tcp:%d.\n", node.nodeID, node.port)
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", node.port))
	if err != nil {
		return nil, err
	}
	LOGV.Printf("Node %d tried register to tcp:%d.\n", node.nodeID, node.port)
	err = rpc.RegisterName("PaxosNode", paxosrpc.Wrap(&node))
	if err != nil {
		return nil, err
	}
	node.listener = &listener
	rpc.HandleHTTP()
	go http.Serve(*node.listener, nil)
	go CatchUpHandler(&node)

	rand.Seed(time.Now().UTC().UnixNano())

	return &node, nil
}

//Now it is not Multi-Paxos, but just Paxos
//Each command slot has its own na,nh,v, or say, version control
func (pn *paxosNode) Prepare(args *paxosrpc.PrepareArgs, reply *paxosrpc.PrepareReply) error {
	LOGV.Printf("node %d OnPrepare:%d %s %d\n", pn.nodeID, args.SlotIdx, args.V.ToString(), args.N)
	LOGV2.Printf("node %d get lock in Prepare()\n", pn.nodeID)
	pn.cmdMutex.Lock()
	v, ok := pn.tempSlots[args.SlotIdx]
	if ok {
		if args.N <= v.Nh { //n<nh
			reply.Status = paxosrpc.Reject
			//to speed up, return the Nh the node have seen
			//TODO:now it is not used, but we should consider it when we want to optimize the system a little bit
			reply.Na = v.Nh
		} else {
			if v.isAccepted || v.isCommited { //accepted or commited state
				v.Nh = args.N
				pn.tempSlots[args.SlotIdx] = v
				reply.Status = paxosrpc.Existed
				reply.Na = v.Na
				reply.Va = v.V
			} else { //prepare state
				ic := IndexCommand{}
				ic.Index = args.SlotIdx
				ic.Na = -1
				ic.Nh = args.N
				ic.isAccepted = false
				ic.isCommited = false
				pn.tempSlots[args.SlotIdx] = ic

				reply.Status = paxosrpc.OK
				reply.Na = ic.Na
				reply.Va = ic.V
			}
		}
	} else { //empty slot
		ic := IndexCommand{}
		ic.Index = args.SlotIdx
		ic.Na = -1
		ic.Nh = args.N
		ic.isAccepted = false
		ic.isCommited = false
		pn.tempSlots[args.SlotIdx] = ic

		reply.Status = paxosrpc.OK
		reply.Na = ic.Na
		reply.Va = ic.V
	}
	LOGV2.Printf("node %d release lock in Prepare()\n", pn.nodeID)
	pn.cmdMutex.Unlock()
	LOGV.Printf("node %d leaving OnPrepare(%d %s %d)\n\t[%d %d %s]\n", pn.nodeID, args.SlotIdx, args.V.ToString(), args.N, reply.Status, reply.Na, reply.Va.ToString())
	return nil
}

func (pn *paxosNode) Accept(args *paxosrpc.AcceptArgs, reply *paxosrpc.AcceptReply) error {
	LOGV.Printf("node %d OnAccept:%d %s %d\n", pn.nodeID, args.SlotIdx, args.V.ToString(), args.N)
	LOGV2.Printf("node %d get lock in Accept()\n", pn.nodeID)
	pn.cmdMutex.Lock()
	v, ok := pn.tempSlots[args.SlotIdx]
	if ok && args.N >= v.Nh {
		/*if v.isAccepted || v.isCommited { //accepted or commited state
			v.Nh = args.N
			reply.Status = paxosrpc.OK
			return nil
		} else { //prepare state*/
		v.isAccepted = true
		v.Na = args.N
		v.Nh = args.N
		v.V = args.V
		pn.tempSlots[args.SlotIdx] = v

		reply.Status = paxosrpc.OK
		//return nil
		//	}
	} else if !ok {
		ic := IndexCommand{}
		ic.Index = args.SlotIdx
		ic.Na = args.N
		ic.Nh = args.N
		ic.V = args.V
		ic.isAccepted = true
		ic.isCommited = false
		pn.tempSlots[args.SlotIdx] = ic

		reply.Status = paxosrpc.OK

	} else {
		reply.Status = paxosrpc.Reject
	}
	LOGV2.Printf("node %d release lock in Accept()\n", pn.nodeID)
	pn.cmdMutex.Unlock()
	LOGV.Printf("node %d leaving OnAccept(%d %s %d)\t[%d]\n", pn.nodeID, args.SlotIdx, args.V.ToString(), args.N, reply.Status)
	return nil
}

func (pn *paxosNode) Commit(args *paxosrpc.CommitArgs, reply *paxosrpc.CommitReply) error {
	LOGV.Printf("node %d OnCommit:%d %s %d\n", pn.nodeID, args.SlotIdx, args.V.ToString(), args.N)
	LOGV2.Printf("node %d get lock in Commit()\n", pn.nodeID)
	pn.cmdMutex.Lock()
	v, ok := pn.tempSlots[args.SlotIdx]
	gap := 0
	if !ok || (ok && !v.isCommited) {
		//if !ok || !v.isCommited {
		if !ok {
			v = IndexCommand{}
			v.isAccepted = true
			v.Index = args.SlotIdx
		}
		v.isCommited = true
		v.V = args.V //TODO:Is it correct?
		pn.tempSlots[args.SlotIdx] = v

		//write to log
		for len(pn.commitedCommands) <= args.SlotIdx {
			pn.commitedCommands = append(pn.commitedCommands, command.Command{})
			gap++
		}
		pn.commitedCommands[args.SlotIdx] = v.V
		pn.callback(v.Index, v.V)
	}
	LOGV2.Printf("node %d release lock in Commit()\n", pn.nodeID)
	pn.cmdMutex.Unlock()
	//Send to CatchUpHandler
	if gap > 1 {
		para := Gap{args.SlotIdx - gap + 1, args.SlotIdx}
		pn.gapchan <- para
	}
	LOGV.Printf("node %d leaving OnCommit:%d %s %d\n", pn.nodeID, args.SlotIdx, args.V.ToString(), args.N)
	return nil
}

func (pn *paxosNode) DoPrepare(args *paxosrpc.PrepareArgs, reply *paxosrpc.PrepareReply) error {
	LOGV.Printf("node %d DoPrepare:%d %s %d\n", pn.nodeID, args.SlotIdx, args.V.ToString(), args.N)
	replychan := make(chan *paxosrpc.PrepareReply, len(pn.peers))

	for i, n := range pn.peers {
		go func(idx int, peernode Node) {
			r := paxosrpc.PrepareReply{}
			//if localhost, call locally
			if peernode.HostPort == pn.addrport {
				pn.Prepare(args, &r)
				replychan <- &r
			} else { //else, call rpc
				peer, err := rpcwrapper.DialHTTP("tcp", peernode.HostPort)
				if err != nil {
					LOGE.Printf("node %d Cannot reach peer %d:%s\n", pn.nodeID, idx, peernode.HostPort)
					r.Status = paxosrpc.Reject
					replychan <- &r
					return
				}
				prepareCall := peer.Go("PaxosNode.Prepare", args, &r, nil)
				select {
				case _, _ = <-prepareCall.Done:
					replychan <- &r
				case _ = <-time.After(time.Second):
					//TODO: how to handle timeout correctly?
					r.Status = paxosrpc.Reject
					replychan <- &r
				}
				peer.Close()
			}
		}(i, n)
	}

	numOK := 0
	numRej := 0
	reply.Na = -1
	for num := 0; num < len(pn.peers); num++ {
		r, _ := <-replychan
		if r.Status != paxosrpc.Reject {
			numOK++
			if r.Status == paxosrpc.Existed && r.Na > reply.Na {
				reply.Na = r.Na
				reply.Va = r.Va
			}
		} else {
			numRej++
		}
	}
	if reply.Na == -1 {
		reply.Na = args.N
		reply.Va = args.V
	}
	LOGV.Printf("node %d DoPrepare %d result:%s %d[%dOK %dRej]\n", pn.nodeID, args.SlotIdx, reply.Va.ToString(), reply.Na, numOK, numRej)

	if numOK > len(pn.peers)/2 {
		reply.Status = paxosrpc.OK
		return nil //return nil error, and let caller to do the accept step
	} else {
		reply.Status = paxosrpc.Reject
		return nil
	}
}

func (pn *paxosNode) DoAccept(args *paxosrpc.AcceptArgs, reply *paxosrpc.AcceptReply) error {
	LOGV.Printf("node %d DoAccept:%d %s %d\n", pn.nodeID, args.SlotIdx, args.V.ToString(), args.N)
	replychan := make(chan *paxosrpc.AcceptReply, len(pn.peers))

	for i, n := range pn.peers {
		go func(idx int, peernode Node) {
			r := new(paxosrpc.AcceptReply)
			if peernode.HostPort == pn.addrport {
				pn.Accept(args, r)
				replychan <- r
			} else {
				peer, err := rpcwrapper.DialHTTP("tcp", peernode.HostPort)
				if err != nil {
					LOGE.Printf("node %d Cannot reach peer %d:%s\n", pn.nodeID, idx, peernode.HostPort)
					r.Status = paxosrpc.Reject
					replychan <- r
					return
				}
				prepareCall := peer.Go("PaxosNode.Accept", args, r, nil)
				select {
				case _, _ = <-prepareCall.Done:
					replychan <- r
				case _ = <-time.After(time.Second):
					r.Status = paxosrpc.Reject
					replychan <- r
				}
				peer.Close()
			}
		}(i, n)
	}

	numOK := 0
	numRej := 0
	for num := 0; num < pn.numNodes; num++ {
		r, _ := <-replychan
		if r.Status != paxosrpc.Reject {
			numOK++
		} else {
			numRej++
		}
	}
	LOGV.Printf("node %d DoAccept %d result: [%dOK %dRej]\n", pn.nodeID, args.SlotIdx, numOK, numRej)

	if numOK > len(pn.peers)/2 {
		reply.Status = paxosrpc.OK
		return nil
	} else {
		reply.Status = paxosrpc.Reject
		return nil
	}
}

func (pn *paxosNode) DoCommit(args *paxosrpc.CommitArgs) error {
	LOGV.Printf("node %d DoCommit:%d %s %d\n", pn.nodeID, args.SlotIdx, args.V.ToString(), args.N)
	replychan := make(chan *paxosrpc.CommitReply, len(pn.peers))

	for i, n := range pn.peers {
		go func(idx int, peernode Node) {
			r := new(paxosrpc.CommitReply)
			if peernode.HostPort == pn.addrport {
				pn.Commit(args, r)
				replychan <- r
			} else {
				peer, err := rpcwrapper.DialHTTP("tcp", peernode.HostPort)
				if err != nil {
					LOGE.Printf("node %d Cannot reach peer %d:%s\n", pn.nodeID, idx, peernode.HostPort)
					r.Status = paxosrpc.Reject
					replychan <- r
					return
				}
				prepareCall := peer.Go("PaxosNode.Commit", args, r, nil)
				select {
				case <-prepareCall.Done:
					replychan <- r
				case <-time.After(time.Second):
					r.Status = paxosrpc.Reject
					replychan <- r
				}
				peer.Close()
			}
		}(i, n)
	}

	for num := 0; num < pn.numNodes; num++ {
		_, _ = <-replychan
		LOGV.Printf("in loop, receive %d\n", num)
	}
	return nil
}

func (pn *paxosNode) Replicate(command *command.Command) error {
	i := 1
	command.AddrPort = pn.addrport
	LOGV2.Printf("node %d get lock in DoReplicate()\n", pn.nodeID)
	pn.cmdMutex.Lock()
	index := len(pn.commitedCommands)
	LOGV.Printf("in replicate, cur len %d\n", index)
	LOGV2.Printf("node %d release lock in DoReplicate()\n", pn.nodeID)
	pn.cmdMutex.Unlock()
	_, success, num := pn.DoReplicate(command, 0, index)
	for !success {
		LOGV.Printf("node %d last Paxos is not success, waiting to try again...\n", pn.nodeID)
		time.Sleep(time.Duration(rand.Int31n(1000)) * time.Millisecond)
		LOGV.Printf("node %d last Paxos slot %d is not success, try again... iter:%d\n", pn.nodeID, index, i)
		LOGV2.Printf("node %d get lock in DoReplicate()\n", pn.nodeID)
		pn.cmdMutex.Lock()
		length := len(pn.commitedCommands)
		LOGV.Printf("in retry, cur len %d\n", index)
		LOGV2.Printf("node %d release lock in DoReplicate()\n", pn.nodeID)
		pn.cmdMutex.Unlock()
		if index < length { // the slot has been passed
			//LOGV.Printf("node %d slot %d need no retry, since other nodes commited %s\n", pn.nodeID, index, pn.commitedCommands[index].ToString())
			if pn.commitedCommands[index].AddrPort == "" {
				//empty slot here, index keeps the same, iter increases
				i = (num/pn.numNodes + 1)
			} else if pn.commitedCommands[index].AddrPort == pn.addrport {
				//Has commited by other nodes, just return
				return nil
			} else {
				//the slot has been occupied by other node's command
				//try to contend to a new slot
				index = length
				i = 0
			}
		} else {
		    i = (num/pn.numNodes + 1)
		}
		_, success, num = pn.DoReplicate(command, i, index)
	}
	return nil
}

//Use NOP to detect the gap slots [from, to)
func CatchUp(pn *paxosNode, from, to int) {
	pn.catchupMutex.Lock()
	pn.catchupCounter++
	pn.catchupMutex.Unlock()
	for index := from; index < to; index++ {
		i := 1
		c := new(command.Command)
		c.Type = command.NOP
		LOGV.Printf("node %d Try to catch up with slot %d\n", pn.nodeID, index)
		success, _, num := pn.DoReplicate(c, 0, index)
		for !success {
			LOGV.Printf("node %d last Paxos is not success, waiting to try again...\n", pn.nodeID)
			//TODO:maybe do not need to wait
			time.Sleep(time.Duration(rand.Int31n(1000)) * time.Millisecond)
			LOGV.Printf("node %d last Paxos is not success, try again...\n", pn.nodeID)
			//i++
			LOGV.Printf("node %d Try to catch up with slot %d\n", pn.nodeID, index)
			i = (num/pn.numNodes + 1)
			success, _, num = pn.DoReplicate(c, i, index)
		}
		LOGV.Printf("Catched up with slot %d\n", index)
	}
	pn.catchupMutex.Lock()
	pn.catchupCounter--
	pn.catchupMutex.Unlock()
}

func CatchUpHandler(pn *paxosNode) {
	for {
		gap, ok := <-pn.gapchan
		if ok {
			LOGV.Printf("node %d is trying to catuch up from %d to %d\n", pn.nodeID, gap.from, gap.to)
			go CatchUp(pn, gap.from, gap.to)
		} else {
			break
		}
	}
}

//command is the command the app/paxos want to commit/nop
//iter is the current iter round, a new command use this to increase its own N
//index is the index of commitedCommands. When we want to commit a new command, set
//index=-1, and when we want to fill the gap in slot i, sent index=i.
//Now the functon return three values. The first is a boolean varible indicating the
//current Paxos is success or not. The second is a boolean indicating the current
//command is commited or rejected. The third is the current slot's highest Vh.
func (pn *paxosNode) DoReplicate(command *command.Command, iter, index int) (bool, bool, int) {
	//Prepare
	prepareArgs := paxosrpc.PrepareArgs{}
	/*if index == -1 {
		LOGV2.Printf("node %d get lock in DoReplicate()\n", pn.nodeID)
		pn.cmdMutex.Lock()
		prepareArgs.SlotIdx = len(pn.commitedCommands)
		LOGV2.Printf("node %d release lock in DoReplicate()\n", pn.nodeID)
		pn.cmdMutex.Unlock()
	} else {
		prepareArgs.SlotIdx = index
	}*/
	prepareArgs.SlotIdx = index
	prepareArgs.N = pn.nodeID + iter*pn.numNodes
	prepareArgs.V = *command

	prepareReply := paxosrpc.PrepareReply{}
	prepareReply.Status = paxosrpc.Reject

	pn.DoPrepare(&prepareArgs, &prepareReply)
	if prepareReply.Status == paxosrpc.Reject {
		return false, false, prepareReply.Na
	}

	//Accept
	acceptArgs := paxosrpc.AcceptArgs{}
	acceptArgs.SlotIdx = prepareArgs.SlotIdx
	LOGV.Printf("Before DoAccept:%d %d\n", acceptArgs.SlotIdx, acceptArgs.N)
	//TODO:need check, maybe wrong
	if prepareReply.Na == prepareArgs.N {
		acceptArgs.V = *command
	} else {
		acceptArgs.V = prepareReply.Va
	}
	acceptArgs.N = prepareArgs.N
	acceptReply := paxosrpc.AcceptReply{}
	pn.DoAccept(&acceptArgs, &acceptReply)

	if acceptReply.Status == paxosrpc.Reject {
		return false, false, acceptArgs.N
	}

	//Commit
	commitArgs := paxosrpc.CommitArgs{}
	commitArgs.SlotIdx = prepareArgs.SlotIdx
	commitArgs.N = acceptArgs.N
	commitArgs.V = acceptArgs.V
	pn.DoCommit(&commitArgs)

	//TODO:need check about the return number
	if prepareReply.Na != prepareArgs.N {
		return true, false, prepareArgs.N
	} else {
		return true, true, prepareArgs.N
	}
}

func (pn *paxosNode) Terminate() error {
	return errors.New("not implemented")
}

//stop rpc server
func (pn *paxosNode) Pause() error {
	LOGV.Printf("Node %d stopped listening on tcp:%d.\n", pn.nodeID, pn.port)
	(*pn.listener).Close()
	return nil
	//return errors.New("not implemented")
}

func (pn *paxosNode) Resume() error {
	LOGV.Printf("Node %d tried listen on tcp:%d.\n", pn.nodeID, pn.port)
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", pn.port))
	if err != nil {
		return err
	}
	pn.listener = &listener
	LOGV.Printf("Node %d resume rpc on port:%d.\n", pn.nodeID, pn.port)
	go http.Serve(*pn.listener, nil)
	return nil
}

func (pn *paxosNode) DumpLog() error {
	LOGV.Printf("node %d start dumping log...", pn.nodeID)
	logname := "dumplog_" + strconv.Itoa(pn.nodeID)
	if f, err := os.Create(logname); err == nil {
		for i, n := range pn.commitedCommands {
			s := fmt.Sprintf("%d: %s\n", i, n.ToString())
			if _, err = f.WriteString(s); err != nil {
				return err
			}
		}
		f.Close()
		LOGV.Printf("node %d finish dumping log...", pn.nodeID)
		return nil
	} else {
		return err
	}
}
