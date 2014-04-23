package paxos

import (
	"errors"
	"fmt"
	"github.com/gobby/src/command"
	"github.com/gobby/src/config"
	"github.com/gobby/src/rpc/paxosrpc"
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
	LOGE = log.New(os.Stderr, "ERROR", log.Lmicroseconds|log.Lshortfile)
	LOGV = log.New(os.Stdout, "VERBOSE", log.Lmicroseconds|log.Lshortfile)
	//LOGV = log.New(ioutil.Discard, "VERBOSE", log.Lmicroseconds|log.Lshortfile)
	_ = ioutil.Discard
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

	tempSlotsMutex sync.Mutex
	commitedMutex  sync.Mutex

	callback PaxosCallBack
	gapchan  chan Gap

	listener *net.Listener
}

//Current setting: all settings are static
func NewPaxosNode(address string, port int, nodeID int, callback PaxosCallBack) (PaxosNode, error) {
	node := paxosNode{}

	node.nodeID = nodeID
	node.port = port
	node.addrport = "localhost:" + strconv.Itoa(port)
	node.callback = callback

	node.numNodes = len(config.Nodes)
	node.peers = make([]Node, 0, node.numNodes)
	for _, n := range config.Nodes {
		n1 := Node{n.Address + fmt.Sprintf(":%d", n.Port), n.NodeID}
		node.peers = append(node.peers, n1)
		LOGV.Println(n1.HostPort)
	}

	node.commitedCommands = make([]command.Command, 0)
	node.tempSlots = make(map[int]IndexCommand)
	node.gapchan = make(chan Gap)

	//rpc
	LOGV.Printf("Node %d tried listen on tcp:%d.\n", nodeID, port)
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, err
	}
	LOGV.Printf("Node %d tried register to tcp:%d.\n", nodeID, port)
	err = rpc.RegisterName("PaxosNode", paxosrpc.Wrap(&node))
	if err != nil {
		return nil, err
	}
	node.listener = &listener
	rpc.HandleHTTP()
	go http.Serve(*node.listener, nil)
	go CatchUpHandler(&node)

	rand.Seed( time.Now().UTC().UnixNano())

	return &node, nil
}

//Now it is not Multi-Paxos, but just Paxos
//Each command slot has its own na,nh,v, or say, version control
func (pn *paxosNode) Prepare(args *paxosrpc.PrepareArgs, reply *paxosrpc.PrepareReply) error {
	LOGV.Printf("node %d OnPrepare:%d %s %d\n", pn.nodeID, args.SlotIdx, args.V.ToString(), args.N)
	pn.tempSlotsMutex.Lock()
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
				ic.Na = args.N
				ic.Nh = args.N
				//ic.V = args.V
				//ic.V.Action = "nop"
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
		ic.Na = args.N
		ic.Nh = args.N
		//ic.V = args.V
		//ic.V.Action = "nop"
		ic.isAccepted = false
		ic.isCommited = false
		pn.tempSlots[args.SlotIdx] = ic

		reply.Status = paxosrpc.OK
		reply.Na = ic.Na
		reply.Va = ic.V
	}
	pn.tempSlotsMutex.Unlock()
	LOGV.Printf("node %d leaving OnPrepare(%d %s %d)\n\t[%d %d %s]\n", pn.nodeID, args.SlotIdx, args.V.ToString(), args.N, reply.Status, reply.Na, reply.Va.ToString())
	return nil
}

func (pn *paxosNode) Accept(args *paxosrpc.AcceptArgs, reply *paxosrpc.AcceptReply) error {
	LOGV.Printf("node %d OnAccept:%d %s %d\n", pn.nodeID, args.SlotIdx, args.V.ToString(), args.N)
	pn.tempSlotsMutex.Lock()
	v, ok := pn.tempSlots[args.SlotIdx]
	if ok && args.N <= v.Nh {
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
		return nil

	} else {
		reply.Status = paxosrpc.Reject
	}
	pn.tempSlotsMutex.Unlock()
	LOGV.Printf("node %d leaving OnAccept(%d %s %d)\t[%d]\n", pn.nodeID, args.SlotIdx, args.V.ToString(), args.N, reply.Status)
	return nil
}

func (pn *paxosNode) Commit(args *paxosrpc.CommitArgs, reply *paxosrpc.CommitReply) error {
	LOGV.Printf("node %d OnCommit:%d %s %d\n", pn.nodeID, args.SlotIdx, args.V.ToString(), args.N)
	pn.tempSlotsMutex.Lock()
	v, ok := pn.tempSlots[args.SlotIdx]
	pn.tempSlotsMutex.Unlock()
	pn.commitedMutex.Lock()
	gap := 0
	if ok && args.N == v.Na && !v.isCommited{
		v.isCommited = true
		pn.tempSlots[args.SlotIdx] = v

		//write to log
		for len(pn.commitedCommands) <= args.SlotIdx {
			//TODO:Calculate from, to and send to CatchUpHandler
			pn.commitedCommands = append(pn.commitedCommands, command.Command{})
			gap++
		}
		pn.commitedCommands[args.SlotIdx] = v.V
		pn.callback(v.Index, v.V)

	}
	//make a change to the state machine
	pn.commitedMutex.Unlock()
	//Send to CatchUpHandler
	if gap > 1 {
	  para := Gap{args.SlotIdx-gap+1, args.SlotIdx}
	  pn.gapchan<-para
	}
	return nil
}

func (pn *paxosNode) DoPrepare(args *paxosrpc.PrepareArgs, reply *paxosrpc.PrepareReply) error {
	LOGV.Printf("node %d DoPrepare:%d %s %d\n", pn.nodeID, args.SlotIdx, args.V.ToString(), args.N)
	replychan := make(chan *paxosrpc.PrepareReply, len(pn.peers))

	for _, n := range pn.peers {
		go func(peernode Node) {
			r := paxosrpc.PrepareReply{}
			peer, err := rpc.DialHTTP("tcp", peernode.HostPort)
			if err != nil {
				LOGE.Println("Cannot reach peer " + peernode.HostPort)
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
		}(n)
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
	  reply.Na =args.N
	  reply.Va=args.V
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

	for _, n := range pn.peers {
		go func(peernode Node) {
			r := paxosrpc.AcceptReply{}
			peer, err := rpc.DialHTTP("tcp", peernode.HostPort)
			if err != nil {
				LOGE.Println("Cannot reach peer " + peernode.HostPort)
				r.Status = paxosrpc.Reject
				replychan <- &r
				return
			}
			prepareCall := peer.Go("PaxosNode.Accept", args, &r, nil)
			select {
			case _, _ = <-prepareCall.Done:
				replychan <- &r
			case _ = <-time.After(time.Second):
				//TODO: how to handle timeout correctly?
				r.Status = paxosrpc.Reject
				replychan <- &r
			}
			peer.Close()
		}(n)
	}

	numOK := 0
	numRej := 0
	for num := 0; num < len(pn.peers); num++ {
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
	for _, n := range pn.peers {
		go func(peernode Node) {
			peer, err := rpc.DialHTTP("tcp", peernode.HostPort)
			if err != nil {
				LOGE.Println("Cannot reach peer " + peernode.HostPort)
				return
			}
			prepareCall := peer.Go("PaxosNode.Commit", args, nil, nil)
			select {
			case _, _ = <-prepareCall.Done:
				//replychan <- r
			case _ = <-time.After(time.Second):
				//TODO: how to handle timeout correctly?
				//r.Status = paxosrpc.Reject
				//replychan <- r
			}
			peer.Close()
		}(n)
	}
	return nil
}

func (pn *paxosNode) Replicate(command *command.Command) error {
	i := 1
	_, success, num := pn.DoReplicate(command, 0, -1)
	for !success {
		LOGV.Println("Last Paxos is not success, waiting to try again...")
		time.Sleep(time.Duration(rand.Int31n(1000)) * time.Millisecond)
		LOGV.Println("Last Paxos is not success, try again...")
		//i++
		i = (num/pn.numNodes + 1)
		_, success, num = pn.DoReplicate(command, i, -1)

	}
	return nil
}

//Use NOP to detect the gap slots [from, to)
func CatchUp(pn *paxosNode, from, to int) {
	for index := from; index < to; index++ {
		i := 1
		LOGV.Printf("Try to catch up with slot %d\n", index)
		c := command.Command{"", "", command.NOP}
		success, _, num := pn.DoReplicate(&c, 0, index)
		for !success {
			LOGV.Println("Last Paxos is not success, waiting to try again...")
			//TODO:maybe do not need to wait
			time.Sleep(time.Duration(rand.Int31n(1000)) * time.Millisecond)
			LOGV.Println("Last Paxos is not success, try again...")
			//i++
			i = (num/pn.numNodes + 1)
			success, _, num = pn.DoReplicate(&c, i, index)

		}
		LOGV.Printf("Catched up with slot %d\n", index)
	}
}

func CatchUpHandler(pn *paxosNode) {
	for {
		gap, ok := <-pn.gapchan
		if ok {
			LOGV.Printf("%d\t%d\n",gap.from, gap.to)
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
	if index == -1 {
		prepareArgs.SlotIdx = len(pn.commitedCommands)
	} else {
		prepareArgs.SlotIdx = index
	}
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
	acceptArgs.N = prepareReply.Na
	LOGV.Printf("Before DoAccept:%d %d\n", acceptArgs.SlotIdx, acceptArgs.N)
	//TODO:need check, maybe wrong
	if acceptArgs.N == prepareArgs.N {
		acceptArgs.V = *command
	} else {
		acceptArgs.V = prepareReply.Va
	}
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
	//close(pn.pushBackChan)
	//close(pn.pushFrontChan)
	//close(pn.popChan)
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
	for i, n := range pn.commitedCommands {
		fmt.Printf("node %d [%d]:%s\n", pn.nodeID, i, n.ToString())
		//fmt.Printf("node %d [%d] Va:%s Na:%d Nh:%d\n", pn.nodeID, i, n.V.ToString(), n.Na, n.Nh)
	}
	return errors.New("not implemented")
}
