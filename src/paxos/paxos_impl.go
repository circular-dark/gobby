package paxos

import (
	"container/list"
	"errors"
	"fmt"
	"github.com/gobby/src/command"
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
	_    = ioutil.Discard
)

//TODO:move this to paxosrpc, in order to implement dynamically add node
type Node struct {
	HostPort string
	NodeID   int
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

	peers            []Node //including itself
	pendingCommands  *list.List
	commitedCommands []command.Command //Log in memory

	tempSlots  map[int]IndexCommand //CommandSlotIndex -> Na
	maxSlotIdx int

	//stateMachine *StateMachine //A pointer to the real state machine

	pushBackChan  chan *command.Command
	pushFrontChan chan *command.Command
	popChan       chan *command.Command
	callback      chan *IndexCommand

	tempSlotsMutex sync.Mutex
	commitedMutex  sync.Mutex
}

//Current setting: all settings are static
//TODO:When the basic logic has been implemented, we can consider about a dynamic setting
const (
	totalnum int = 3
)

//var nodeslist = []string{"localhost:9990", "localhost:9991", "localhost:9992"}
var nodeslist = []Node{Node{"localhost:9990", 0}, Node{"localhost:9991", 1}, Node{"localhost:9992", 2}}

//remoteHostPort is null now, numNodes always equals 3
func NewPaxosNode(remoteHostPort string, numNodes, port int, nodeID int, callback PaxosCallBack) (PaxosNode, error) {
	node := paxosNode{}

	node.nodeID = nodeID
	node.port = port
	node.numNodes = numNodes
	node.addrport = "localhost:" + strconv.Itoa(port)
	node.callback = callback

	node.peers = make([]Node, 0, numNodes)
	for _, n := range nodeslist {
		node.peers = append(node.peers, n)
		LOGV.Println(n.HostPort)
	}

	node.pendingCommands = list.New()
	node.commitedCommands = make([]command.Command, 0)
	node.tempSlots = make(map[int]IndexCommand)
	node.pushBackChan = make(chan *command.Command)
	node.pushFrontChan = make(chan *command.Command)
	node.popChan = make(chan *command.Command)

	//stateMachine = nil

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
	rpc.HandleHTTP()
	go http.Serve(listener, nil)

	//list
	go pendingListHandler(&node)
	go replicateHandler(&node)

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
			//LOGV.Printf("node %d leaving OnPrepare(%d %s %d) [Rej]\n", pn.nodeID, args.SlotIdx, args.V.ToString(), args.N)
			//return nil
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
				ic.V.Action = "nop"
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
		ic.V.Action = "nop"
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
		return nil

	} else {
		reply.Status = paxosrpc.Reject
	}
	pn.tempSlotsMutex.Unlock()
	LOGV.Printf("node %d leaving OnAccept(%d %s %d)\t[%d]\n", pn.nodeID, args.SlotIdx, args.V.ToString(), args.N, reply.Status)
	return nil
	//return errors.New("not implemented")
}

//We do not use this!!!
func (pn *paxosNode) CommitAndReply(args *paxosrpc.CommitArgs, reply *paxosrpc.CommitReply) error {
	return errors.New("not implemented")
}

func (pn *paxosNode) Commit(args *paxosrpc.CommitArgs, reply *paxosrpc.CommitReply) error {
	LOGV.Printf("node %d OnCommit:%d %s %d\n", pn.nodeID, args.SlotIdx, args.V.ToString(), args.N)
	v, ok := pn.tempSlots[args.SlotIdx]
	pn.commitedMutex.Lock()
	if ok && args.N == v.Na {
		v.isCommited = true
		pn.tempSlots[args.SlotIdx] = v

		//write to log
		for len(pn.commitedCommands) <= args.SlotIdx {
			//TODO:command state
			pn.commitedCommands = append(pn.commitedCommands, command.Command{})
		}
		pn.commitedCommands[args.SlotIdx] = v.V

		//make a change to the state machine
		pn.callback <- &v

	}
	pn.commitedMutex.Unlock()
	//return errors.New("Not exist!")
	return nil
}

func (pn *paxosNode) DoPrepare(args *paxosrpc.PrepareArgs, reply *paxosrpc.PrepareReply) error {
	LOGV.Printf("node %d DoPrepare:%d %s %d\n", pn.nodeID, args.SlotIdx, args.V.ToString(), args.N)
	replychan := make(chan *paxosrpc.PrepareReply, len(pn.peers))

	for _, n := range pn.peers {
		LOGV.Println("Try to call prepare on " + n.HostPort)
		go func(peernode Node) {
			//LOGV.Println("in routie:Try to call prepare on " + n.HostPort)
			peer, err := rpc.DialHTTP("tcp", peernode.HostPort)
			if err != nil {
				LOGE.Println("Cannot reach peer " + peernode.HostPort)
				return
			}
			r := paxosrpc.PrepareReply{}
			prepareCall := peer.Go("PaxosNode.Prepare", args, &r, nil)
			select {
			case _, _ = <-prepareCall.Done:
				replychan <- &r
			case _ = <-time.After(time.Second):
				//TODO: how to handle timeout correctly?
				r.Status = paxosrpc.Reject
				replychan <- &r
			}
		}(n)
	}

	numOK := 0
	numRej := 0
	reply.Na = -1
	for num := 0; num < len(pn.peers); num++ {
		r, _ := <-replychan
		if r.Status != paxosrpc.Reject {
			numOK++
			if r.Na > reply.Na {
				reply.Na = r.Na
				reply.Va = r.Va
			}
		} else {
			numRej++
		}
	}
	LOGV.Printf("node %d DoPrepare %d result:%s %d[%dOK %dRej]\n", pn.nodeID, args.SlotIdx, reply.Va.ToString(), reply.Na, numOK, numRej)

	if numOK > len(pn.peers)/2 {
		reply.Status = paxosrpc.OK
		return nil //return nil error, and let caller to do the accept step
	} else {
		reply.Status = paxosrpc.Reject
		return nil
	}
	//return errors.New("not implemented")
}

func (pn *paxosNode) DoAccept(args *paxosrpc.AcceptArgs, reply *paxosrpc.AcceptReply) error {
	LOGV.Printf("node %d DoAccept:%d %s %d\n", pn.nodeID, args.SlotIdx, args.V.ToString(), args.N)
	replychan := make(chan *paxosrpc.AcceptReply, len(pn.peers))

	for _, n := range pn.peers {
		go func(peernode Node) {
			peer, err := rpc.DialHTTP("tcp", peernode.HostPort)
			if err != nil {
				LOGE.Println("Cannot reach peer " + peernode.HostPort)
				return
			}
			r := paxosrpc.AcceptReply{}
			prepareCall := peer.Go("PaxosNode.Accept", args, &r, nil)
			select {
			case _, _ = <-prepareCall.Done:
				replychan <- &r
			case _ = <-time.After(time.Second):
				//TODO: how to handle timeout correctly?
				r.Status = paxosrpc.Reject
				replychan <- &r
			}
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
	//return errors.New("not implemented")
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
		}(n)
	}
	return nil
	//return errors.New("not implemented")
}

func (pn *paxosNode) Replicate(command *command.Command) error {
	//Put command into the pendingCommand
	//pn.pendingCommands.PushBack(command)
	pn.pushBackChan <- command
	return nil
	//return errors.New("not implemented")
}

func (pn *paxosNode) DoReplicate(command *command.Command) bool {
	//Prepare
	prepareArgs := paxosrpc.PrepareArgs{}
	//prepareArgs.SlotIdx = pn.maxSlotIdx
	//pn.maxSlotIdx++
	prepareArgs.SlotIdx = len(pn.commitedCommands)
	prepareArgs.N = pn.nodeID
	prepareArgs.V = *command

	prepareReply := paxosrpc.PrepareReply{}
	prepareReply.Status = paxosrpc.Reject

	/*for prepareReply.Status != paxosrpc.Reject {
	  pn.doPrepare(&prepareArgs, &prepareReply)
	  if prepareReply != paxosrpc.Reject {

	  } else if{
	    prepareArgs.
	  } else {

	  }
	}*/
	pn.DoPrepare(&prepareArgs, &prepareReply)
	if prepareReply.Status == paxosrpc.Reject {
		return false
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
		return false
	}

	//Commit
	commitArgs := paxosrpc.CommitArgs{}
	commitArgs.SlotIdx = prepareArgs.SlotIdx
	commitArgs.N = acceptArgs.N
	commitArgs.V = acceptArgs.V
	pn.DoCommit(&commitArgs)

	//return true
	if prepareReply.Na != prepareArgs.N {
		return false
	} else {
		return true
	}
}

func pendingListHandler(pn *paxosNode) {
	var popChan chan *command.Command
	var p *command.Command
	for {
		if pn.pendingCommands.Len() > 0 {
			popChan = pn.popChan
			p = pn.pendingCommands.Front().Value.(*command.Command)
			pn.pendingCommands.Remove(pn.pendingCommands.Front())
		} else {
			popChan = nil
			p = nil
		}
		select {
		case c, ok := <-pn.pushBackChan:
			if !ok {
				break
			}
			pn.pendingCommands.PushBack(c)
			LOGV.Printf("Get one command:%s\n", c.ToString())
		case c, ok := <-pn.pushFrontChan:
			if !ok {
				break
			}
			pn.pendingCommands.PushFront(c)
		case popChan <- p:
			LOGV.Printf("Send one command to replacteHandler:%s\n", p.ToString())
		}
	}
	close(pn.popChan)
}

func replicateHandler(pn *paxosNode) {
	for {
		v, ok := <-pn.popChan
		if !ok {
			break
		} else {
			success := pn.DoReplicate(v)
			if !success {
				LOGV.Printf("Paxos failes this time, wait for a time and try later.\n")
				//Wait for a random time
				time.Sleep(time.Duration(rand.Int31n(1000)) * time.Millisecond)
				pn.pushFrontChan <- v
			}
		}
	}
}

func (pn *paxosNode) CatchUp() {

}

func (pn *paxosNode) Terminate() error {
	close(pn.pushBackChan)
	close(pn.pushFrontChan)
	//close(pn.popChan)
	return nil
}

func (pn *paxosNode) Pause() error {
	return errors.New("not implemented")
}
