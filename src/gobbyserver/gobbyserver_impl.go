package gobbyserver

import (
	"fmt"
	"github.com/gobby/src/command"
	"github.com/gobby/src/config"
	"github.com/gobby/src/lease"
	"github.com/gobby/src/paxos"
	"github.com/gobby/src/rpc/gobbyrpc"
	"net"
	"net/rpc"
	"strconv"
	"sync"
)

type kstate struct {
	value     string
	lockstamp string // a unixtime int64 stamp for identifying the lock (sequencer value)
	locked    bool
	watchers  map[string]string //watcher->responsible server
}

type gobbyserver struct {
	addrport   string
	store      map[string]*kstate
	commandLog []*command.Command
	nextIndex  int                               // next index of command that should be executed
	nextCID    int                               // next command ID that should be assigned
	replyChs   map[int]chan *gobbyrpc.GobbyReply // maps command id to channels that block RPC calls
	lock       sync.Mutex
	paxosnode  paxos.PaxosNode
	leasenode  lease.LeaseNode
	pending    *Queue
}

func NewGobbyServer(nodeID int, numNodes int) (Gobbyserver, error) {
	server := new(gobbyserver)
	server.addrport = config.Nodes[nodeID].Address + ":" + strconv.Itoa(config.Nodes[nodeID].Port)
	server.store = make(map[string]*kstate)
	server.commandLog = make([]*command.Command, 0)
	server.nextIndex = 0
	server.replyChs = make(map[int]chan *gobbyrpc.GobbyReply)
	server.pending = NewQueue()
	if pnode, err := paxos.NewPaxosNode(nodeID, numNodes, server.getCommand); err != nil {
		return nil, err
	} else {
		server.paxosnode = pnode
	}
	if lnode, err := lease.NewLeaseNode(nodeID, numNodes); err != nil {
		return nil, err
	} else {
		server.leasenode = lnode
	}

	if err := rpc.RegisterName("gobbyServer", gobbyrpc.Wrap(server)); err != nil {
		return nil, err
	} else {
		go ReplicateRoutine(server.pending, server.paxosnode)
		return server, nil
	}

	//TODO:Now the listener is in the lease node, and we should let gobbyserver contain listener
}

func (server *gobbyserver) Put(args *gobbyrpc.PutArgs, reply *gobbyrpc.GobbyReply) error {
	server.lock.Lock()
	c := &command.Command{
		Key:   args.Key,
		Value: args.Value,
		Type:  command.Put,
		ID:    server.nextCID,
	}
	replyCh := make(chan *gobbyrpc.GobbyReply)
	server.nextCID++
	server.replyChs[c.ID] = replyCh
	server.lock.Unlock()
	//go server.paxosnode.Replicate(c)
	server.pending.Enqueue(c)

	// blocking for reply
	r := <-replyCh
	reply.Status = r.Status
	return nil
}

func (server *gobbyserver) Get(args *gobbyrpc.GetArgs, reply *gobbyrpc.GobbyReply) error {
	server.lock.Lock()
	c := &command.Command{
		Key:  args.Key,
		Type: command.Get,
		ID:   server.nextCID,
	}
	replyCh := make(chan *gobbyrpc.GobbyReply)
	server.nextCID++
	server.replyChs[c.ID] = replyCh
	server.lock.Unlock()
	//go server.paxosnode.Replicate(c)
	server.pending.Enqueue(c)

	// blocking for reply
	r := <-replyCh
	reply.Status = r.Status
	reply.Value = r.Value
	return nil
}

func (server *gobbyserver) Acquire(args *gobbyrpc.AcquireArgs, reply *gobbyrpc.GobbyReply) error {
	server.lock.Lock()
	c := &command.Command{
		Key:  args.Key,
		Type: command.Acquire,
		ID:   server.nextCID,
	}
	replyCh := make(chan *gobbyrpc.GobbyReply)
	server.nextCID++
	server.replyChs[c.ID] = replyCh
	server.lock.Unlock()
	//go server.paxosnode.Replicate(c)
	server.pending.Enqueue(c)

	// blocking for reply
	r := <-replyCh
	reply.Status = r.Status
	reply.Value = r.Value // lockstamp
	return nil
}

func (server *gobbyserver) Release(args *gobbyrpc.ReleaseArgs, reply *gobbyrpc.GobbyReply) error {
	server.lock.Lock()
	c := &command.Command{
		Key:   args.Key,
		Value: args.Lockstamp,
		Type:  command.Release,
		ID:    server.nextCID,
	}
	replyCh := make(chan *gobbyrpc.GobbyReply)
	server.nextCID++
	server.replyChs[c.ID] = replyCh
	server.lock.Unlock()
	//go server.paxosnode.Replicate(c)
	server.pending.Enqueue(c)

	// blocking for reply
	r := <-replyCh
	reply.Status = r.Status
	return nil
}

func (server *gobbyserver) CheckMaster(args *gobbyrpc.CheckArgs, reply *gobbyrpc.GobbyReply) error {
	if server.leasenode.CheckMaster() {
		reply.Status = gobbyrpc.OK
	} else {
		reply.Status = gobbyrpc.FAIL
	}
	return nil
}

func (server *gobbyserver) Watch(args *gobbyrpc.WatchArgs, reply *gobbyrpc.GobbyReply) error {
	server.lock.Lock()
	c := &command.Command{
		Key:   args.Key,
		Value: args.HostAddr,
		Type:  command.Watch,
		ID:    server.nextCID,
	}
	replyCh := make(chan *gobbyrpc.GobbyReply)
	server.nextCID++
	server.replyChs[c.ID] = replyCh
	server.lock.Unlock()
	//go server.paxosnode.Replicate(c)
	server.pending.Enqueue(c)

	// blocking for reply
	r := <-replyCh
	reply.Status = r.Status
	return nil
}

func (server *gobbyserver) getCommand(index int, c command.Command) {
	paxos.LOGV.Println("in getCommand")
	server.lock.Lock()
	for len(server.commandLog) < index+1 {
		server.commandLog = append(server.commandLog, nil)
	}
	server.commandLog[index] = &c
	server.lock.Unlock()
	server.executeCommands()
	paxos.LOGV.Println("leave getCommand")
}

func (server *gobbyserver) executeCommands() {
	paxos.LOGV.Println("in executeCommand")
	server.lock.Lock()
	for server.nextIndex < len(server.commandLog) && server.commandLog[server.nextIndex] != nil {
		c := server.commandLog[server.nextIndex]
		cid := c.ID
		//TODO: how to distinguish other nodes and itself's commands?
		var replyCh chan *gobbyrpc.GobbyReply = nil
		if c.AddrPort == server.addrport {
			replyCh = server.replyChs[cid]
			delete(server.replyChs, cid)
		}

		switch c.Type {
		case command.Put:
			paxos.LOGV.Println("in executeCommand:handle put")
			if st, ok := server.store[c.Key]; ok {
				st.value = c.Value
				//TODO:Notify watchers
				notifyWatchers(c.Value, st, server)
				st.watchers = make(map[string]string)
				if replyCh != nil {
					replyCh <- &gobbyrpc.GobbyReply{
						Status: gobbyrpc.OK,
					}
				}
			} else {
				server.store[c.Key] = &kstate{
					value:    c.Value,
					locked:   false,
					watchers: make(map[string]string),
				}
				if replyCh != nil {
					replyCh <- &gobbyrpc.GobbyReply{
						Status: gobbyrpc.OK,
					}
				}
			}
			paxos.LOGV.Println("in executeCommand:handle put done")
		case command.Get:
			if st, ok := server.store[c.Key]; ok {
				if replyCh != nil {
					replyCh <- &gobbyrpc.GobbyReply{
						Status: gobbyrpc.OK,
						Value:  st.value,
					}
				}
			} else {
				if replyCh != nil {
					replyCh <- &gobbyrpc.GobbyReply{
						Status: gobbyrpc.FAIL,
					}
				}
			}
		case command.Acquire:
			if st, ok := server.store[c.Key]; ok {
				if st.locked {
					if replyCh != nil {
						replyCh <- &gobbyrpc.GobbyReply{
							Status: gobbyrpc.FAIL,
						}
					}
				} else {
					st.locked = true
					st.lockstamp = c.Value
					if replyCh != nil {
						replyCh <- &gobbyrpc.GobbyReply{
							Status: gobbyrpc.OK,
							Value:  st.lockstamp,
						}
					}
				}
			} else {
				if replyCh != nil {
					replyCh <- &gobbyrpc.GobbyReply{
						Status: gobbyrpc.FAIL,
					}
				}
			}
		case command.Release:
			if st, ok := server.store[c.Key]; ok {
				if !st.locked || st.lockstamp != c.Value {
					if replyCh != nil {
						replyCh <- &gobbyrpc.GobbyReply{
							Status: gobbyrpc.FAIL,
						}
					}
				} else {
					st.locked = false
					if replyCh != nil {
						replyCh <- &gobbyrpc.GobbyReply{
							Status: gobbyrpc.OK,
						}
					}
				}
			} else {
				if replyCh != nil {
					replyCh <- &gobbyrpc.GobbyReply{
						Status: gobbyrpc.FAIL,
					}
				}
			}
		case command.Watch:
			if st, ok := server.store[c.Key]; ok {
				st.watchers[c.Value] = c.AddrPort
				if replyCh != nil {
					replyCh <- &gobbyrpc.GobbyReply{
						Status: gobbyrpc.OK,
						Value:  st.value,
					}
				}
			} else {
				if replyCh != nil {
					replyCh <- &gobbyrpc.GobbyReply{
						Status: gobbyrpc.FAIL,
					}
				}
			}
		}
		server.nextIndex++
	}
	server.lock.Unlock()
	paxos.LOGV.Println("leaving executeCommand")
}

func notifyWatchers(v string, st *kstate, server *gobbyserver) {
	for w, s := range st.watchers {
		if s == server.addrport {
			go func(addrport string) {
				//Notify all clients
				serverAddr, err := net.ResolveUDPAddr("udp", addrport)
				if err != nil {
					fmt.Printf("cannot resolve %s:%s\n", addrport, err)
					return
				}
				conn, err := net.DialUDP("udp", nil, serverAddr)
				if err != nil {
					fmt.Printf("cannot connect %s:%s\n", addrport, err)
					return
				}
				fmt.Println("WatchNotify:" + v)
				conn.Write([]byte(v))
				conn.Close()
			}(w)
		}
	}
}
