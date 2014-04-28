package lease

import (
	"fmt"
	"github.com/gobby/src/config"
	"github.com/gobby/src/rpc/leaserpc"
	"github.com/gobby/src/rpc/rpcwrapper"
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
	LEASE_LEN   = 30 // 30 * 100 milliseconds
	REFRESH_LEN = 5  // 10 * 100 milliseconds
)

var (
	LOGV = log.New(os.Stdout, "VERBOSE", log.Lmicroseconds|log.Lshortfile)
	/* LOGV = log.New(ioutil.Discard, "VERBOSE", log.Lmicroseconds|log.Lshortfile) */
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
	leaseLen               int // remaining lease length period
	renewLeaseLen          int // length of the renewing lease
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
		leaseLen:   0,
		Nh:         0,
		Na:         0,
		peers:      make([]Node, numNodes),
	}

	for i := 0; i < numNodes; i++ {
		node.peers[i].HostPort = config.Nodes[i].Address + ":" + strconv.Itoa(config.Nodes[i].Port)
		node.peers[i].NodeID = i
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", node.port))
	if err != nil {
		return nil, err
	}
	if err = rpc.RegisterName("LeaseNode", leaserpc.Wrap(node)); err != nil {
		return nil, err
	}
	rpc.HandleHTTP()
	go http.Serve(listener, nil)
	go node.leaseManager()

	return node, nil
}

func (ln *leaseNode) Prepare(args *leaserpc.Args, reply *leaserpc.Reply) error {
	ln.leaseMutex.Lock()
	if args.N < ln.Nh {
		reply.Status = leaserpc.Reject
	} else {
		ln.Nh = args.N
		if ln.leaseLen > 0 {
			reply.Status = leaserpc.Reject
		} else {
			reply.Status = leaserpc.OK
		}
	}
	ln.leaseMutex.Unlock()
	return nil
}

func (ln *leaseNode) Accept(args *leaserpc.Args, reply *leaserpc.Reply) error {
	ln.leaseMutex.Lock()
	if args.N < ln.Nh {
		reply.Status = leaserpc.Reject
	} else {
		ln.Nh = args.N
		ln.Na = args.N
		ln.leaseLen = LEASE_LEN
		reply.Status = leaserpc.OK
	}
	ln.leaseMutex.Unlock()
	return nil
}

func (ln *leaseNode) dofunction(args *leaserpc.Args, reply *leaserpc.Reply, funcname string) error {
	replychan := make(chan *leaserpc.Reply, len(ln.peers))

	for _, n := range ln.peers {
		go func(peernode Node) {
			if peernode.HostPort == ln.addrport {
				//LOGV.Printf("node %d call locally %s\n", ln.nodeID, funcname)
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
				//LOGV.Printf("node %d call remote %s\n", ln.nodeID, funcname)
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
	return ln.dofunction(args, reply, "LeaseNode.Prepare")
}

func (ln *leaseNode) DoAccept(args *leaserpc.Args, reply *leaserpc.Reply) error {
	return ln.dofunction(args, reply, "LeaseNode.Accept")
}

func (ln *leaseNode) RenewPrepare(args *leaserpc.Args, reply *leaserpc.Reply) error {
	ln.leaseMutex.Lock()
	if ln.Nh < args.N {
		ln.Nh = args.N
	}
	if ln.Na == args.N || ln.leaseLen == 0 {
		reply.Status = leaserpc.OK
	} else {
		reply.Status = leaserpc.Reject
	}
	ln.leaseMutex.Unlock()
	return nil
}

func (ln *leaseNode) RenewAccept(args *leaserpc.Args, reply *leaserpc.Reply) error {
	ln.leaseMutex.Lock()
	if ln.Nh < args.N {
		ln.Nh = args.N
	}
	if ln.Na == args.N || ln.leaseLen == 0 {
		ln.Na = args.N
		ln.leaseLen = LEASE_LEN
		reply.Status = leaserpc.OK
	} else {
		reply.Status = leaserpc.Reject
	}
	ln.leaseMutex.Unlock()
	return nil
}

func (ln *leaseNode) DoRenewPrepare(args *leaserpc.Args, reply *leaserpc.Reply) error {
	return ln.dofunction(args, reply, "LeaseNode.RenewPrepare")
}

func (ln *leaseNode) DoRenewAccept(args *leaserpc.Args, reply *leaserpc.Reply) error {
	return ln.dofunction(args, reply, "LeaseNode.RenewAccept")
}

func (ln *leaseNode) renewLease() {
	LOGV.Printf("node %d is renewing the lease\n", ln.nodeID)
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
	ln.renewLeaseLen = LEASE_LEN - 1 // for safe now, shorten the length a bit
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
	ln.isMaster = true
	ln.leaseLen = ln.renewLeaseLen
	ln.Na = acceptArgs.N
	ln.leaseMutex.Unlock()
	LOGV.Printf("node %d again is THE MASTER NOW!\n", ln.nodeID)
}

func (ln *leaseNode) getLease() {
	LOGV.Printf("node %d is trying to get the lease\n", ln.nodeID)
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
	ln.leaseLen = LEASE_LEN - 1 // for safe now, shorten the length a bit
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
	ln.isMaster = true
	ln.Na = acceptArgs.N
	ln.leaseMutex.Unlock()
	LOGV.Printf("node %d IS THE MASTER NOW!\n", ln.nodeID)
}

func (ln *leaseNode) CheckMaster() bool {
	ln.leaseMutex.Lock()
	res := (ln.isMaster && ln.leaseLen > 0)
	ln.leaseMutex.Unlock()
	return res
}

func (ln *leaseNode) leaseManager() {
	// manager runs for every 50 millisecond
	t := time.NewTicker(100 * time.Millisecond)
	for {
		select {
		case <-t.C:
			willget := false
			willrenew := false
			ln.leaseMutex.Lock()
			if ln.renewLeaseLen > 0 {
				ln.renewLeaseLen--
			}
			if ln.leaseLen > 0 {
				ln.leaseLen--
				if ln.leaseLen == 0 && ln.isMaster {
					ln.isMaster = false
					willget = true
				}
				if ln.leaseLen <= REFRESH_LEN && ln.isMaster {
					willrenew = true
				}
			} else {
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
