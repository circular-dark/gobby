package paxos

import (
    "math"
	"errors"
	"fmt"
	"github.com/gobby/src/command"
	"github.com/gobby/src/config"
	"github.com/gobby/src/rpc/paxosrpc"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"strconv"
	"sync"
	"time"
)

const (
    ConnRetryLimit = 3
)

var (
	LOGE = log.New(os.Stderr, "ERROR", log.Lmicroseconds|log.Lshortfile)
	LOGV = log.New(os.Stdout, "VERBOSE", log.Lmicroseconds|log.Lshortfile)
	_    = ioutil.Discard
)


type IndexCommand struct {
	Index      int
	Na, Nh     int
	V          command.Command
	isAccepted bool
	isCommitted bool
    seqnum     int
}

type paxosNode struct {
	nodeID   int
    port     int
	addrport string
	cmdSlots  map[int]*IndexCommand
    peerConns map[int]*rpc.Client
	nextIndex int
	callback  PaxosCallBack
	lock  sync.Mutex
}

func NewPaxosNode(address string,  port int, nodeID int, callback PaxosCallBack) (PaxosNode, error) {
	node := new(paxosNode)

	node.nodeID = nodeID
	node.port = port
	node.addrport = address + strconv.Itoa(port)
	node.callback = callback
	node.cmdSlots = make(map[int]*IndexCommand)
    node.peerConns = make(map[int]*rpc.Client)
    node.nextIndex = 0

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, err
	}
	err = rpc.RegisterName("PaxosNode", paxosrpc.Wrap(node))
	if err != nil {
		return nil, err
	}
	rpc.HandleHTTP()
	go http.Serve(listener, nil)

	return node, nil
}

func (pn *paxosNode) GetConns() {
    pn.lock.Lock()
    for p, n := range config.Nodes {
        if conn, ok := pn.peerConns[p]; !ok || conn == nil {
            hostport := n.Address + ":" + strconv.Itoa(n.Port)
            for i := 0; i < ConnRetryLimit; i++ {
                if peer, err := rpc.DialHTTP("tcp", hostport); err == nil {
                    pn.peerConns[p] = peer
                    break
                } else {
                    pn.peerConns[p] = nil
                }
            }
        }
    }
    pn.lock.Unlock()
}

func (pn *paxosNode) Prepare(args *paxosrpc.PrepareArgs, reply *paxosrpc.PrepareReply) error {
	pn.lock.Lock()
	v, ok := pn.cmdSlots[args.SlotIdx]
	if ok {
        if v.isCommitted {
            reply.Status = paxosrpc.Committed
            reply.V = v.V
        } else if v.isAccepted {
            reply.Status = paxosrpc.Accepted
            reply.N = v.Na
            reply.V = v.V
        } else if args.N > v.Nh {
            reply.Status = paxosrpc.OK
        } else {
			reply.Status = paxosrpc.Reject
        }
        if args.N > v.Nh {
            v.Nh = args.N
        }
    } else {
        ic := &IndexCommand{
            Index: args.SlotIdx,
            Na: 0,
            Nh: args.N,
            isAccepted: false,
            isCommitted: false,
            seqnum: 1,
        }
        pn.cmdSlots[args.SlotIdx] = ic
        reply.Status = paxosrpc.OK
    }
	pn.lock.Unlock()
	return nil
}

func (pn *paxosNode) Accept(args *paxosrpc.AcceptArgs, reply *paxosrpc.AcceptReply) error {
	pn.lock.Lock()
	v, ok := pn.cmdSlots[args.SlotIdx]
	if ok {
        if v.isCommitted {
            reply.Status = paxosrpc.Committed
            reply.V = v.V
        } else if v.isAccepted {
            reply.Status = paxosrpc.Reject
        } else if args.N >= v.Nh {
            reply.Status = paxosrpc.OK
            v.isAccepted = true
            v.Na = args.N
            v.V = args.V
        } else {
            reply.Status = paxosrpc.Reject
        }
        if args.N > v.Nh {
            v.Nh = args.N
        }
    } else {
		ic := &IndexCommand{
            Index: args.SlotIdx,
            Na: args.N,
            Nh: args.N,
            V: args.V,
            isAccepted: true,
            isCommitted: false,
            seqnum: 1,
        }
		pn.cmdSlots[args.SlotIdx] = ic

		reply.Status = paxosrpc.OK
	}
	pn.lock.Unlock()
	return nil
}

func (pn *paxosNode) Commit(args *paxosrpc.CommitArgs, reply *paxosrpc.CommitReply) error {
	v, ok := pn.cmdSlots[args.SlotIdx]
	pn.lock.Lock()
	if ok {
        if v.isCommitted {
            LOGE.Println("Fatal error here! Double commit!!!")
            return errors.New("Fatal error here! Double commit!!!")
        } else {
            reply.Status = paxosrpc.OK
            v.V = args.V
            v.isCommitted = true
            pn.callback(v.Index, v.V)
        }
    } else {
        reply.Status = paxosrpc.OK
        ic := &IndexCommand{
            Index: args.SlotIdx,
            V: args.V,
            isCommitted: true,
            seqnum: 1,
        }
        pn.cmdSlots[args.SlotIdx] = ic
	}
	pn.lock.Unlock()
	return nil
}

func (pn *paxosNode) DoPrepare(args *paxosrpc.PrepareArgs, reply *paxosrpc.PrepareReply) error {
	replychan := make(chan *paxosrpc.PrepareReply, config.NumNodes)

	for _, conn := range pn.peerConns {
		go func(c *rpc.Client) {
			r := new(paxosrpc.PrepareReply)
            if c == nil {
				r.Status = paxosrpc.Reject
                replychan <- r
            } else {
                prepareCall := c.Go("PaxosNode.Prepare", args, &r, nil)
                select {
                case _, _ = <-prepareCall.Done:
                    replychan <- r
                case _ = <-time.After(time.Second):
                    r.Status = paxosrpc.Reject
                    replychan <- r
                }
            }
		}(conn)
	}

	numOK := 0
	numRej := 0
    reply.N = -1
	for i := 0; i < config.NumNodes; i++ {
		r, _ := <-replychan
        if r.Status == paxosrpc.Committed {
            reply.Status = paxosrpc.Committed
            reply.V = r.V
            return nil
        } else if r.Status == paxosrpc.Accepted {
            numOK++
            if r.N > reply.N {
                reply.N = r.N
                reply.V = r.V
            }
        } else if r.Status == paxosrpc.Reject {
			numRej++
        } else {
            numOK++
        }
	}

	if numOK > config.NumNodes / 2 {
		reply.Status = paxosrpc.OK
		return nil
	} else {
		reply.Status = paxosrpc.Reject
		return nil
	}
}

func (pn *paxosNode) DoAccept(args *paxosrpc.AcceptArgs, reply *paxosrpc.AcceptReply) error {
	replychan := make(chan *paxosrpc.AcceptReply, config.NumNodes)

	for _, conn := range pn.peerConns {
		go func(c *rpc.Client) {
			r := new(paxosrpc.AcceptReply)
            if c == nil {
				r.Status = paxosrpc.Reject
                replychan <- r
            } else {
                prepareCall := c.Go("PaxosNode.Accept", args, &r, nil)
                select {
                case _, _ = <-prepareCall.Done:
                    replychan <- r
                case _ = <-time.After(time.Second):
                    r.Status = paxosrpc.Reject
                    replychan <- r
                }
            }
		}(conn)
	}

	numOK := 0
	numRej := 0
	for i := 0; i < config.NumNodes; i++ {
		r, _ := <-replychan
        if r.Status == paxosrpc.Committed {
            reply.Status = paxosrpc.Committed
            reply.V = r.V
            return nil
        } else if r.Status == paxosrpc.Reject {
			numRej++
		} else {
			numOK++
		}
	}

	if numOK > config.NumNodes / 2 {
		reply.Status = paxosrpc.OK
		return nil
	} else {
		reply.Status = paxosrpc.Reject
		return nil
	}
}

func (pn *paxosNode) DoCommit(args *paxosrpc.CommitArgs) error {
	for _, conn := range pn.peerConns {
		go func(c *rpc.Client) {
            if c != nil {
                prepareCall := c.Go("PaxosNode.Commit", args, nil, nil)
                select {
                case _, _ = <-prepareCall.Done:
                case _ = <-time.After(time.Second):
                }
            }
		}(conn)
    }
	return nil
}


func (pn *paxosNode) Replicate(command *command.Command) error {
    for {
        if success := pn.doReplicate(command); success {
            return nil
        }
    }
}

func (pn *paxosNode) doReplicate(command *command.Command) bool {

    // getting the next unused index
    pn.lock.Lock()
    index := pn.nextIndex
    for {
        if v, ok := pn.cmdSlots[index]; ok && v.isCommitted {
            index++
        } else {
            pn.nextIndex = index + 1
            break
        }
    }
    pn.lock.Unlock()

	// Prepare phase
	prepareArgs := paxosrpc.PrepareArgs{
        SlotIdx: index,
        N: pn.nextSeqnum(index),
    }
	prepareReply := paxosrpc.PrepareReply{}
	prepareReply.Status = paxosrpc.Reject
	pn.DoPrepare(&prepareArgs, &prepareReply)
	if prepareReply.Status != paxosrpc.OK {
		return false
	}

	// Accept phase
    changeValue := false
	acceptArgs := paxosrpc.AcceptArgs{}
	acceptArgs.SlotIdx = prepareArgs.SlotIdx
	if prepareReply.N == -1 {
        acceptArgs.N = prepareArgs.N
        acceptArgs.V = *command
	} else {
        acceptArgs.N = prepareReply.N
        acceptArgs.V = prepareReply.V
        changeValue = true
    }
    acceptReply := paxosrpc.AcceptReply{}
	pn.DoAccept(&acceptArgs, &acceptReply)
	if acceptReply.Status != paxosrpc.OK {
		return false
	}

	// Commit phase
	commitArgs := paxosrpc.CommitArgs{}
	commitArgs.SlotIdx = prepareArgs.SlotIdx
	commitArgs.N = acceptArgs.N
	commitArgs.V = acceptArgs.V
	pn.DoCommit(&commitArgs)
    if changeValue {
        return false
    } else {
        return true
    }
}

func (pn *paxosNode) nextSeqnum(index int) int {
    pn.lock.Lock()
    var seqnum int = 0
    if v, ok := pn.cmdSlots[index]; ok {
        for seqnum < v.Nh {
            seqnum = int(math.Pow(float64(pn.nodeID), float64(v.seqnum)))
            v.seqnum++
        }
    } else {
        seqnum = pn.nodeID
    }
    pn.lock.Unlock()
    return seqnum
}
