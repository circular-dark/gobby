package lease

import (
    "sync"
    "github.com/gobby/src/rpc/leaserpc"
    "github.com/gobby/src/config"
    "strconv"
    "net"
    "fmt"
    "net/http"
	/* "io/ioutil" */
    /* "log" */
    "net/rpc"
)

var (
	/* LOGE = log.New(os.Stderr, "ERROR", log.Lmicroseconds|log.Lshortfile) */
	/* LOGE = log.New(ioutil.Discard, "ERROR", log.Lmicroseconds|log.Lshortfile) */
	/* LOGV = log.New(os.Stdout, "VERBOSE", log.Lmicroseconds|log.Lshortfile) */
	/* LOGV = log.New(ioutil.Discard, "VERBOSE", log.Lmicroseconds|log.Lshortfile) */
	/* _ = ioutil.Discard */
)

type Node struct {
	HostPort string
	NodeID   int
}

type leaseNode struct {
	nodeID, port, numNodes int
	addrport               string
	peers            []Node               //including itself
    nextBallot     int
	leaseMutex sync.Mutex
    isMaster bool
    inLease bool
    leaseLen int // remaining lease length period
}

//Current setting: all settings are static
func NewLeaseNode(nodeID int, numNodes int) (LeaseNode, error) {
	node := &leaseNode{
	    nodeID: nodeID,
	    port: config.Nodes[nodeID].Port,
	    numNodes: numNodes,
	    addrport: config.Nodes[nodeID].Address + ":" + strconv.Itoa(config.Nodes[nodeID].Port),
        isMaster: false,
        nextBallot: nodeID,
        inLease: false,
        leaseLen: 0,
	    peers: make([]Node, numNodes),
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

func (ln *leaseNode) Prepare(args *leaserpc.PrepareArgs, reply *leaserpc.PrepareReply) error {
	ln.leaseMutex.Lock()
	v, ok := ln.tempSlots[args.SlotIdx]
	if ok {
		if args.N <= v.Nh { //n<nh
			reply.Status = leaserpc.Reject
			//to speed up, return the Nh the node have seen
			//TODO:now it is not used, but we should consider it when we want to optimize the system a little bit
			reply.Na = v.Nh
		} else {
			if v.isAccepted || v.isCommited { //accepted or commited state
				v.Nh = args.N
				ln.tempSlots[args.SlotIdx] = v
				reply.Status = leaserpc.Existed
				reply.Na = v.Na
				reply.Va = v.V
			} else { //prepare state
				ic := IndexCommand{}
				ic.Index = args.SlotIdx
				ic.Na = -1
				ic.Nh = args.N
				ic.isAccepted = false
				ic.isCommited = false
				ln.tempSlots[args.SlotIdx] = ic

				reply.Status = leaserpc.OK
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
		ln.tempSlots[args.SlotIdx] = ic

		reply.Status = leaserpc.OK
		reply.Na = ic.Na
		reply.Va = ic.V
	}
	ln.cmdMutex.Unlock()
	return nil
}

func (ln *leaseNode) Accept(args *leaserpc.AcceptArgs, reply *leaserpc.AcceptReply) error {
	ln.cmdMutex.Lock()
	v, ok := ln.tempSlots[args.SlotIdx]
	if ok && args.N >= v.Nh {
		/*if v.isAccepted || v.isCommited { //accepted or commited state
			v.Nh = args.N
			reply.Status = leaserpc.OK
			return nil
		} else { //prepare state*/
		v.isAccepted = true
		v.Na = args.N
		v.Nh = args.N
		v.V = args.V
		ln.tempSlots[args.SlotIdx] = v

		reply.Status = leaserpc.OK
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
		ln.tempSlots[args.SlotIdx] = ic

		reply.Status = leaserpc.OK
		return nil

	} else {
		reply.Status = leaserpc.Reject
	}
	ln.cmdMutex.Unlock()
	return nil
}

func (ln *leaseNode) DoPrepare(args *leaserpc.PrepareArgs, reply *leaserpc.PrepareReply) error {

	replychan := make(chan *leaserpc.PrepareReply, len(ln.peers))

	for _, n := range ln.peers {
		go func(peernode Node) {
			r := leaserpc.PrepareReply{}
			peer, err := rpc.DialHTTP("tcp", peernode.HostPort)
			if err != nil {
				r.Status = leaserpc.Reject
				replychan <- &r
				return
			}
			prepareCall := peer.Go("LeaseNode.Prepare", args, &r, nil)
			select {
			case _, _ = <-prepareCall.Done:
				replychan <- &r
			case _ = <-time.After(time.Second):
				r.Status = leaserpc.Reject
				replychan <- &r
			}
			peer.Close()
		}(n)
	}

	numOK := 0
	numRej := 0
	reply.Na = -1
	for num := 0; num < len(ln.peers); num++ {
		r, _ := <-replychan
		if r.Status != leaserpc.Reject {
			numOK++
			if r.Status == leaserpc.Existed && r.Na > reply.Na {
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

	if numOK > len(ln.peers)/2 {
		reply.Status = leaserpc.OK
		return nil //return nil error, and let caller to do the accept step
	} else {
		reply.Status = leaserpc.Reject
		return nil
	}
}

func (ln *leaseNode) DoAccept(args *leaserpc.AcceptArgs, reply *leaserpc.AcceptReply) error {
	replychan := make(chan *leaserpc.AcceptReply, len(ln.peers))

	for _, n := range ln.peers {
		go func(peernode Node) {
			r := new(leaserpc.AcceptReply)
			peer, err := rpc.DialHTTP("tcp", peernode.HostPort)
			if err != nil {
				r.Status = leaserpc.Reject
				replychan <- r
				return
			}
			prepareCall := peer.Go("LeaseNode.Accept", args, r, nil)
			select {
			case _, _ = <-prepareCall.Done:
				replychan <- r
			case _ = <-time.After(time.Second):
				r.Status = leaserpc.Reject
				replychan <- r
			}
			peer.Close()
		}(n)
	}

	numOK := 0
	numRej := 0
	for num := 0; num < ln.numNodes; num++ {
		r, _ := <-replychan
		if r.Status != leaserpc.Reject {
			numOK++
		} else {
			numRej++
		}
	}

	if numOK > len(ln.peers)/2 {
		reply.Status = leaserpc.OK
		return nil
	} else {
		reply.Status = leaserpc.Reject
		return nil
	}
}

func (ln *leaseNode) GetLease() (int, error) {
	// Prepare
    ln.leaseMutex.Lock()
	prepareArgs := leaserpc.PrepareArgs{
        N: ln.nextBallot,
    }
    ln.nextBallot += ln.numNodes
    ln.leaseMutex.Unlock()
	prepareReply := leaserpc.PrepareReply{}

	ln.DoPrepare(&prepareArgs, &prepareReply)
	if prepareReply.Status == leaserpc.Reject {
		return -1, nil
	}

	// Accept
	acceptArgs := leaserpc.AcceptArgs{}
	acceptArgs.SlotIdx = prepareArgs.SlotIdx
	//TODO:need check, maybe wrong
	if prepareReply.Na == prepareArgs.N {
		acceptArgs.V = *command
	} else {
		acceptArgs.V = prepareReply.Va
	}
	acceptArgs.N = prepareArgs.N
	acceptReply := leaserpc.AcceptReply{}
	ln.DoAccept(&acceptArgs, &acceptReply)

	if acceptReply.Status == leaserpc.Reject {
		return false, false, acceptArgs.N
	}

	//Commit
	commitArgs := leaserpc.CommitArgs{}
	commitArgs.SlotIdx = prepareArgs.SlotIdx
	commitArgs.N = acceptArgs.N
	commitArgs.V = acceptArgs.V
	ln.DoCommit(&commitArgs)

	//TODO:need check about the return number
	if prepareReply.Na != prepareArgs.N {
		return true, false, prepareArgs.N
	} else {
		return true, true, prepareArgs.N
	}
}

func (ln *leaseNode) CheckMaster() bool {
    ln.leaseMutex.Lock()
    res := ln.isMaster
    ln.leaseMutex.Unlock()
    return res
}

func (ln *leaseNode) leaseManager() {
    // manager runs for every 50 millisecond
    t := time.NewTicker(50 * time.Millisecond)
    for {
        select {
        case <-t.C:
            ln.leaseMutex.Lock()

            ln.leaseMutex.Unock()
        }
    }
}
