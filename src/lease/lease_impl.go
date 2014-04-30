package lease

import (
	"github.com/gobby/src/config"
	"github.com/gobby/src/rpc/leaserpc"
	"github.com/gobby/src/rpc/rpcwrapper"
	"io/ioutil"
	"log"
	"net/rpc"
	"os"
	"strconv"
	"sync"
	"time"
)

const (
    PERIORD_LEN = 1000 // number of milliseconds in a period
	LEASE_LEN   = 5    // number of periods in a lease length
	REFRESH_LEN = 2    // number of remaining master periods when trying to renew
)

var (
	/* LOGV = log.New(os.Stdout, "VERBOSE", log.Lmicroseconds|log.Lshortfile) */
	LOGV = log.New(ioutil.Discard, "VERBOSE", log.Lmicroseconds|log.Lshortfile)
	LOGE = log.New(os.Stdout, "VERBOSE", log.Lmicroseconds|log.Lshortfile)
	/* LOGE = log.New(ioutil.Discard, "VERBOSE", log.Lmicroseconds|log.Lshortfile) */
	_ = ioutil.Discard
)

type Node struct {
	HostPort string
	NodeID   int
}

type leaseNode struct {
	nodeID, port, numNodes int
	addrport               string
	peers                  []Node //including itself
	nextBallot             int
	leaseMutex             sync.Mutex
	isMaster               bool
	masterLeaseLen         int // remaining master lease length period
	renewLeaseLen          int // remaining renew master lease length period
	acceptLeaseLen         int // remaining accept lease length period
	Nh                     int
	Na                     int
}

//Current setting: all settings are static
func NewLeaseNode(nodeID int, numNodes int) (LeaseNode, error) {
	node := &leaseNode{
		nodeID:     nodeID,
		port:       config.Nodes[nodeID].Port,
		numNodes:   numNodes,
		addrport:   config.Nodes[nodeID].Address + ":" + strconv.Itoa(config.Nodes[nodeID].Port),
		isMaster:   false,
		nextBallot: nodeID,
        masterLeaseLen: 0,
        renewLeaseLen:  0,
        acceptLeaseLen: 0,
		Nh:         0,
		Na:         0,
		peers:      make([]Node, numNodes),
	}

	for i := 0; i < numNodes; i++ {
		node.peers[i].HostPort = config.Nodes[i].Address + ":" + strconv.Itoa(config.Nodes[i].Port)
		node.peers[i].NodeID = i
	}

	if err := rpc.RegisterName("LeaseNode", leaserpc.Wrap(node)); err != nil {
		return nil, err
	}
	go node.leaseManager()

	return node, nil
}

func (ln *leaseNode) Prepare(args *leaserpc.Args, reply *leaserpc.Reply) error {
    LOGV.Printf("node %d receives prepare %d\n", ln.nodeID, args.N)
	ln.leaseMutex.Lock()
	if args.N < ln.Nh {
		reply.Status = leaserpc.Reject
	} else {
		ln.Nh = args.N
		if ln.masterLeaseLen > 0 || ln.acceptLeaseLen > 0 {
			reply.Status = leaserpc.Reject
            LOGV.Printf("node %d rejects prepare %d\n", ln.nodeID, args.N)
		} else {
			reply.Status = leaserpc.OK
            LOGV.Printf("node %d accepts prepare %d\n", ln.nodeID, args.N)
		}
	}
	ln.leaseMutex.Unlock()
	return nil
}

func (ln *leaseNode) Accept(args *leaserpc.Args, reply *leaserpc.Reply) error {
    LOGV.Printf("node %d receives accept %d\n", ln.nodeID, args.N)
	ln.leaseMutex.Lock()
	if args.N < ln.Nh {
		reply.Status = leaserpc.Reject
        LOGV.Printf("node %d rejects accept %d\n", ln.nodeID, args.N)
	} else {
        ln.Nh = args.N
		ln.Na = args.N
        ln.acceptLeaseLen = LEASE_LEN
        reply.Status = leaserpc.OK
        LOGV.Printf("node %d accepts accept %d\n", ln.nodeID, args.N)
	}
	ln.leaseMutex.Unlock()
	return nil
}

func (ln *leaseNode) dofunction(args *leaserpc.Args, reply *leaserpc.Reply, funcname string) error {
	replychan := make(chan *leaserpc.Reply, len(ln.peers))

	for _, n := range ln.peers {
		go func(peernode Node) {
			if peernode.HostPort == ln.addrport {
				r := leaserpc.Reply{}
				switch {
				case funcname == "LeaseNode.Prepare":
					ln.Prepare(args, &r)
				case funcname == "LeaseNode.Accept":
					ln.Accept(args, &r)
				case funcname == "LeaseNode.RenewPrepare":
					ln.RenewPrepare(args, &r)
				case funcname == "LeaseNode.RenewAccept":
					ln.RenewAccept(args, &r)
				}
				replychan <- &r
			} else {
				r := leaserpc.Reply{}
				peer, err := rpcwrapper.DialHTTP("tcp", peernode.HostPort)
				if err != nil {
					r.Status = leaserpc.Reject
					replychan <- &r
					return
				}
				prepareCall := peer.Go(funcname, args, &r, nil)
				select {
				case _, _ = <-prepareCall.Done:
					replychan <- &r
				case _ = <-time.After(time.Second):
					r.Status = leaserpc.Reject
					replychan <- &r
				}
				peer.Close()
			}
		}(n)
	}

	numOK, numRej := 0, 0
	for num := 0; num < len(ln.peers); num++ {
		r, _ := <-replychan
		if r.Status == leaserpc.OK {
			numOK++
		} else {
			numRej++
		}
	}

	if numOK > ln.numNodes/2 {
		reply.Status = leaserpc.OK
		return nil
	} else {
		reply.Status = leaserpc.Reject
		return nil
	}
}

func (ln *leaseNode) DoPrepare(args *leaserpc.Args, reply *leaserpc.Reply) error {
    LOGV.Printf("node %d sends out prepare %d\n", ln.nodeID, args.N)
    err := ln.dofunction(args, reply, "LeaseNode.Prepare")
    if reply.Status == leaserpc.OK {
        LOGV.Printf("node %d's prepare %d SUCCEED\n", ln.nodeID, args.N)
    } else {
        LOGV.Printf("node %d's prepare %d FAIL\n", ln.nodeID, args.N)
    }
    return err
}

func (ln *leaseNode) DoAccept(args *leaserpc.Args, reply *leaserpc.Reply) error {
    LOGV.Printf("node %d sends out accept %d\n", ln.nodeID, args.N)
    err := ln.dofunction(args, reply, "LeaseNode.Accept")
    if reply.Status == leaserpc.OK {
        LOGV.Printf("node %d's accept %d SUCCEED\n", ln.nodeID, args.N)
    } else {
        LOGV.Printf("node %d's accept %d FAIL\n", ln.nodeID, args.N)
    }
    return err
}

func (ln *leaseNode) RenewPrepare(args *leaserpc.Args, reply *leaserpc.Reply) error {
    LOGV.Printf("node %d receives renewprepare %d\n", ln.nodeID, args.N)
	ln.leaseMutex.Lock()
    if ln.Nh < args.N {
        ln.Nh = args.N
    }
	if ln.Na == args.N || (ln.acceptLeaseLen == 0 && ln.masterLeaseLen == 0) {
		reply.Status = leaserpc.OK
        LOGV.Printf("node %d accepts renewprepare %d\n", ln.nodeID, args.N)
	} else {
		reply.Status = leaserpc.Reject
        LOGV.Printf("node %d rejects renewprepare %d\n", ln.nodeID, args.N)
	}
	ln.leaseMutex.Unlock()
	return nil
}

func (ln *leaseNode) RenewAccept(args *leaserpc.Args, reply *leaserpc.Reply) error {
    LOGV.Printf("node %d receives renewaccept %d\n", ln.nodeID, args.N)
	ln.leaseMutex.Lock()
    if ln.Nh < args.N {
        ln.Nh = args.N
    }
	if ln.Na == args.N || (ln.acceptLeaseLen == 0 && ln.masterLeaseLen == 0) {
		ln.acceptLeaseLen = LEASE_LEN
		reply.Status = leaserpc.OK
        LOGV.Printf("node %d accepts renewaccept %d\n", ln.nodeID, args.N)
	} else {
		reply.Status = leaserpc.Reject
        LOGV.Printf("node %d rejects renewaccept %d\n", ln.nodeID, args.N)
	}
	ln.leaseMutex.Unlock()
	return nil
}

func (ln *leaseNode) DoRenewPrepare(args *leaserpc.Args, reply *leaserpc.Reply) error {
    LOGV.Printf("node %d sends out renewprepare %d\n", ln.nodeID, args.N)
	err := ln.dofunction(args, reply, "LeaseNode.RenewPrepare")
    if reply.Status == leaserpc.OK {
        LOGV.Printf("node %d's renewprepare %d SUCCEED\n", ln.nodeID, args.N)
    } else {
        LOGV.Printf("node %d's renewprepare %d FAIL\n", ln.nodeID, args.N)
    }
    return err
}

func (ln *leaseNode) DoRenewAccept(args *leaserpc.Args, reply *leaserpc.Reply) error {
    LOGV.Printf("node %d sends out renewaccept %d\n", ln.nodeID, args.N)
	err := ln.dofunction(args, reply, "LeaseNode.RenewAccept")
    if reply.Status == leaserpc.OK {
        LOGV.Printf("node %d's renewaccept %d SUCCEED\n", ln.nodeID, args.N)
    } else {
        LOGV.Printf("node %d's renewaccept %d FAIL\n", ln.nodeID, args.N)
    }
    return err
}

func (ln *leaseNode) renewLease() {
	// Prepare
	ln.leaseMutex.Lock()
	prepareArgs := leaserpc.Args{
		N: ln.Na,
	}
	ln.leaseMutex.Unlock()
	prepareReply := leaserpc.Reply{}

	ln.DoRenewPrepare(&prepareArgs, &prepareReply)
	if prepareReply.Status == leaserpc.Reject {
		return
	}

	// Accept
	ln.leaseMutex.Lock()
	ln.renewLeaseLen = LEASE_LEN - 2 // for safe now, shorten the length a bit
	ln.leaseMutex.Unlock()
	acceptArgs := leaserpc.Args{
		N: prepareArgs.N,
	}
	acceptReply := leaserpc.Reply{}

	ln.DoRenewAccept(&acceptArgs, &acceptReply)
	if acceptReply.Status == leaserpc.Reject {
		return
	}

	ln.leaseMutex.Lock()
    if ln.renewLeaseLen > 0 {
        ln.isMaster = true
        ln.masterLeaseLen = ln.renewLeaseLen
        LOGE.Printf("node %d IS AGAIN THE MASTER NOW!\n", ln.nodeID)
    }
	ln.Na = acceptArgs.N
	ln.leaseMutex.Unlock()
}

func (ln *leaseNode) getLease() {
	// Prepare
	ln.leaseMutex.Lock()
	for ln.nextBallot <= ln.Nh {
		ln.nextBallot += ln.numNodes
	}
	prepareArgs := leaserpc.Args{
		N: ln.nextBallot,
	}
	ln.leaseMutex.Unlock()
	prepareReply := leaserpc.Reply{}

	ln.DoPrepare(&prepareArgs, &prepareReply)
	if prepareReply.Status == leaserpc.Reject {
		return
	}

	// Accept
	ln.leaseMutex.Lock()
	ln.masterLeaseLen = LEASE_LEN - 2 // for safe now, shorten the length a bit
	ln.leaseMutex.Unlock()
	acceptArgs := leaserpc.Args{
		N: prepareArgs.N,
	}
	acceptReply := leaserpc.Reply{}

	ln.DoAccept(&acceptArgs, &acceptReply)
	if acceptReply.Status == leaserpc.Reject {
		return
	}

	ln.leaseMutex.Lock()
    if ln.masterLeaseLen > 0 {
        ln.isMaster = true
        LOGE.Printf("node %d IS THE MASTER NOW!\n", ln.nodeID)
    }
	ln.Na = acceptArgs.N
	ln.leaseMutex.Unlock()
}

func (ln *leaseNode) CheckMaster() bool {
	ln.leaseMutex.Lock()
	res := (ln.isMaster && ln.masterLeaseLen > 0)
	ln.leaseMutex.Unlock()
	return res
}

func (ln *leaseNode) leaseManager() {
	t := time.NewTicker(PERIORD_LEN * time.Millisecond)
	for {
		select {
		case <-t.C:
            LOGV.Printf("node %d: master %d, accepts %d\n", ln.nodeID, ln.masterLeaseLen, ln.acceptLeaseLen)
			willget := false
			willrenew := false
			ln.leaseMutex.Lock()
			if ln.renewLeaseLen > 0 {
				ln.renewLeaseLen--
			}
			if ln.masterLeaseLen > 0 {
				ln.masterLeaseLen--
			}
			if ln.acceptLeaseLen > 0 {
				ln.acceptLeaseLen--
			}
            if ln.isMaster {
                if ln.masterLeaseLen == 0 {
                    ln.isMaster = false
                    LOGE.Printf("node %d IS  NOT  THE MASTER NOW!\n", ln.nodeID)
                } else if ln.masterLeaseLen < REFRESH_LEN {
                    willrenew = true
                }
            }
            if ln.acceptLeaseLen == 0 {
                willget = true
            }
			ln.leaseMutex.Unlock()
			if willrenew {
				ln.renewLease()
			} else if willget {
				ln.getLease()
			}
		}
	}
}
