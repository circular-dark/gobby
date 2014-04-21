package paxos

import (
	"errors"
	"github.com/gobby/src/command"
	"github.com/gobby/src/rpc/paxosrpc"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"strconv"
	//"sync"
	"container/list"
	"fmt"
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

	peers []Node //including itself
	//pendingCommands []command.Command //Buffer to accept commands from app
	pendingCommands  *list.List
	commitedCommands []command.Command //Log in memory

	//prepareSlots map[int]IndexCommand  //CommandSlotIndex -> Na
	//acceptSlots map[int]IndexCommand   //CommandSlotIndex -> Na
	tempSlots  map[int]IndexCommand //CommandSlotIndex -> Na
	maxSlotIdx int

	//stateMachine *StateMachine //A pointer to the real state machine

	pushBackChan  chan *command.Command
	pushFrontChan chan *command.Command
	popChan       chan *command.Command
}

//Current setting: all settings are static
//TODO:When the basic logic has been implemented, we can consider about a dynamic setting
const (
	//nodeslist []int= {9990,9991,9992}
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

	node.peers = make([]Node, 0, numNodes)
	for _, n := range nodeslist {
		//node.peers = append(node.peers, n)
		node.peers = append(node.peers, n)
	}
	if node.peers[0].HostPort != node.addrport {
		var i int
		for i = 0; i < len(node.peers); i++ {
			if node.peers[i].HostPort == node.addrport {
				break
			}
		}
		node.peers[i] = node.peers[0]
		node.peers[0] = nodeslist[nodeID]
	}

	//node.pendingCommands = make([]command.Command, 0)
	node.pendingCommands = list.New()
	node.commitedCommands = make([]command.Command, 0)
	node.tempSlots = make(map[int]IndexCommand)
	node.pushBackChan = make(chan *command.Command)
	node.pushFrontChan = make(chan *command.Command)
	node.popChan = make(chan *command.Command)

	//stateMachine = nil

	//list
	//go pendingListHandler(&node)
	//go replicateHandler(&node)

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

	return &node, nil
}

//Now it is not Multi-Paxos, but just Paxos
//Each command slot has its own na,nh,v, or say, version control
func (pn *paxosNode) Prepare(args *paxosrpc.PrepareArgs, reply *paxosrpc.PrepareReply) error {
	v, ok := pn.tempSlots[args.SlotIdx]
	if ok {
		if args.N <= v.Nh { //n<nh
			reply.Status = paxosrpc.Reject
			return nil
		} else {
			if v.isAccepted || v.isCommited { //accepted or commited state
				v.Nh = args.N
				pn.tempSlots[args.SlotIdx] = v
				reply.Status = paxosrpc.Existed
				reply.Na = v.Na
				reply.Va = v.V
				return nil
			} else { //prepare state
				ic := IndexCommand{}
				ic.Index = args.SlotIdx
				ic.Na = args.N
				ic.Nh = args.N
				ic.V = args.V
				ic.isAccepted = false
				ic.isCommited = false
				pn.tempSlots[args.SlotIdx] = ic

				reply.Status = paxosrpc.OK
				reply.Na = ic.Na
				reply.Va = ic.V

				return nil
			}
		}
	} else { //empty slot
		ic := IndexCommand{}
		ic.Index = args.SlotIdx
		ic.Na = args.N
		ic.Nh = args.N
		ic.V = args.V
		ic.isAccepted = false
		ic.isCommited = false
		pn.tempSlots[args.SlotIdx] = ic

		reply.Status = paxosrpc.OK
		reply.Na = ic.Na
		reply.Va = ic.V

		return nil
	}
	//return errors.New("not implemented")
}

func (pn *paxosNode) Accept(args *paxosrpc.AcceptArgs, reply *paxosrpc.AcceptReply) error {
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
		return nil
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

	}
	reply.Status = paxosrpc.Reject
	return nil
	//return errors.New("not implemented")
}

//We do not use this!!!
func (pn *paxosNode) CommitAndReply(args *paxosrpc.CommitArgs, reply *paxosrpc.CommitReply) error {
	return errors.New("not implemented")
}

func (pn *paxosNode) Commit(args *paxosrpc.CommitArgs, reply *paxosrpc.CommitReply) error {
	v, ok := pn.tempSlots[args.SlotIdx]
	if ok && args.N == v.Na {
		v.isCommited = true
		pn.tempSlots[args.SlotIdx] = v

		//write to log

		//make a change to the state machine

	}
	return errors.New("Not exist!")
}

func (pn *paxosNode) DoPrepare(args *paxosrpc.PrepareArgs, reply *paxosrpc.PrepareReply) error {
	replychan := make(chan *paxosrpc.PrepareReply, len(pn.peers))

	for _, n := range pn.peers {
		go func() {
			peer, err := rpc.DialHTTP("tcp", n.HostPort)
			if err != nil {
				LOGE.Println("Cannot reach peer " + n.HostPort)
				return
			}
			r := paxosrpc.PrepareReply{}
			prepareCall := peer.Go("PaxosNode.Prepare", args, r, nil)
			select {
			case _, _ = <-prepareCall.Done:
				replychan <- &r
			case _ = <-time.After(time.Second):
				//TODO: how to handle timeout correctly?
				r.Status = paxosrpc.Reject
				replychan <- &r
			}
		}()
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
	replychan := make(chan *paxosrpc.AcceptReply, len(pn.peers))

	for _, n := range pn.peers {
		go func() {
			peer, err := rpc.DialHTTP("tcp", n.HostPort)
			if err != nil {
				LOGE.Println("Cannot reach peer " + n.HostPort)
				return
			}
			r := paxosrpc.AcceptReply{}
			prepareCall := peer.Go("PaxosNode.Accept", args, r, nil)
			select {
			case _, _ = <-prepareCall.Done:
				replychan <- &r
			case _ = <-time.After(time.Second):
				//TODO: how to handle timeout correctly?
				r.Status = paxosrpc.Reject
				replychan <- &r
			}
		}()
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
	for _, n := range pn.peers {
		go func() {
			peer, err := rpc.DialHTTP("tcp", n.HostPort)
			if err != nil {
				LOGE.Println("Cannot reach peer " + n.HostPort)
				return
			}
			prepareCall := peer.Go("PaxosNode.CommitArgs", args, nil, nil)
			select {
			case _, _ = <-prepareCall.Done:
				//replychan <- r
			case _ = <-time.After(time.Second):
				//TODO: how to handle timeout correctly?
				//r.Status = paxosrpc.Reject
				//replychan <- r
			}
		}()
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
	prepareArgs.SlotIdx = len(pn.commitedCommands) + 1
	prepareArgs.N = 0
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
	} else if prepareReply.Na != prepareArgs.N {
		//put command back to the pending queue
		pn.pendingCommands.PushFront(command)
	}

	//Accept
	acceptArgs := paxosrpc.AcceptArgs{}
	acceptArgs.SlotIdx = prepareArgs.SlotIdx
	acceptArgs.N = prepareReply.Na
	acceptArgs.V = prepareReply.Va
	acceptReply := paxosrpc.AcceptReply{}
	pn.DoAccept(&acceptArgs, &acceptReply)

	if acceptReply.Status == paxosrpc.Reject {
		return false
	}

	//Commit
	commitArgs := paxosrpc.CommitArgs{}
	commitArgs.SlotIdx = prepareArgs.SlotIdx
	commitArgs.N = prepareReply.Na
	commitArgs.V = prepareReply.Va
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
		case c, ok := <-pn.pushFrontChan:
			if !ok {
				break
			}
			pn.pendingCommands.PushFront(c)
		case popChan <- p:
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
