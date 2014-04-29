package chubbyserver

import (
    "sync"
    "strconv"
    "time"
    "net/rpc"
    "github.com/gobby/src/rpc/chubbyrpc"
    "github.com/gobby/src/paxos"
    "github.com/gobby/src/command"
    "github.com/gobby/src/config"
    "github.com/gobby/src/lease"
)

type kstate struct {
    value string
    lockstamp string // a unixtime int64 stamp for identifying the lock (sequencer value)
    locked bool
}

type chubbyserver struct {
    addrport string
    store map[string]*kstate
    commandLog []*command.Command
    nextIndex int // next index of command that should be executed
    nextCID int // next command ID that should be assigned
    replyChs map[int]chan *chubbyrpc.ChubbyReply // maps command id to channels that block RPC calls
    lock sync.Mutex
    paxosnode paxos.PaxosNode
    leasenode lease.LeaseNode
    pending *Queue
}

func NewChubbyServer(nodeID int, numNodes int) (Chubbyserver, error) {
    server := new(chubbyserver)
    server.addrport =  config.Nodes[nodeID].Address + ":" + strconv.Itoa(config.Nodes[nodeID].Port)
    server.store = make(map[string]*kstate)
    server.commandLog = make([]*command.Command, 0)
    server.nextIndex = 0
    server.replyChs = make(map[int]chan *chubbyrpc.ChubbyReply)
    server.pending = NewQueue()
    if pnode, err := paxos.NewPaxosNode(nodeID, numNodes, server.getCommand); err != nil {
        return nil, err
    } else {
        server.paxosnode = pnode
    }
    /*if lnode, err := lease.NewLeaseNode(nodeID, numNodes); err != nil {
        return nil, err
    } else {
        server.leasenode = lnode
    }*/

	if err := rpc.RegisterName("ChubbyServer", chubbyrpc.Wrap(server)); err != nil {
		return nil, err
	} else {
    go ReplicateRoutine(server.pending, server.paxosnode)
        return server, nil
    }

    //TODO:Now the listener is in the lease node, and we should let chubbyserver contain listener
}

func (server *chubbyserver) Put(args *chubbyrpc.PutArgs, reply *chubbyrpc.ChubbyReply) error {
    server.lock.Lock()
    c := &command.Command{
        Key: args.Key,
        Value: args.Value,
        Type: command.Put,
        ID: server.nextCID,
    }
    replyCh := make(chan *chubbyrpc.ChubbyReply)
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

func (server *chubbyserver) Get(args *chubbyrpc.GetArgs, reply *chubbyrpc.ChubbyReply) error {
    server.lock.Lock()
    c := &command.Command{
        Key: args.Key,
        Type: command.Get,
        ID: server.nextCID,
    }
    replyCh := make(chan *chubbyrpc.ChubbyReply)
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

func (server *chubbyserver) Aquire(args *chubbyrpc.AquireArgs, reply *chubbyrpc.ChubbyReply) error {
    server.lock.Lock()
    c := &command.Command{
        Key: args.Key,
        Type: command.Acquire,
        ID: server.nextCID,
    }
    replyCh := make(chan *chubbyrpc.ChubbyReply)
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

func (server *chubbyserver) Release(args *chubbyrpc.ReleaseArgs, reply *chubbyrpc.ChubbyReply) error {
    server.lock.Lock()
    c := &command.Command{
        Key: args.Key,
        Value: args.Lockstamp,
        Type: command.Release,
        ID: server.nextCID,
    }
    replyCh := make(chan *chubbyrpc.ChubbyReply)
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

func (server *chubbyserver) getCommand(index int, c command.Command) {
paxos.LOGV.Println("in getCommand")
    server.lock.Lock()
    for len(server.commandLog) < index + 1 {
        server.commandLog = append(server.commandLog, nil)
    }
    server.commandLog[index] = &c
    server.lock.Unlock()
    server.executeCommands()
paxos.LOGV.Println("leave getCommand")
}

func (server *chubbyserver) executeCommands() {
paxos.LOGV.Println("in executeCommand")
    server.lock.Lock()
    for server.nextIndex < len(server.commandLog) && server.commandLog[server.nextIndex] != nil {
        c := server.commandLog[server.nextIndex]
        cid := c.ID
	//TODO: how to distinguish other nodes and itself's commands?
	var replyCh chan *chubbyrpc.ChubbyReply = nil
	if c.AddrPort == server.addrport {
        replyCh = server.replyChs[cid]
        delete(server.replyChs, cid)
	}

        switch (c.Type) {
        case command.Put:
paxos.LOGV.Println("in executeCommand:handle put")
            if st, ok := server.store[c.Key]; ok {
                st.value = c.Value
		if replyCh != nil {
                replyCh <- &chubbyrpc.ChubbyReply{
                    Status: chubbyrpc.OK,
                }
		}
            } else {
                server.store[c.Key] = &kstate{
                    value: c.Value,
                    locked: false,
                }
		if replyCh != nil {
                replyCh <- &chubbyrpc.ChubbyReply{
                    Status: chubbyrpc.OK,
                }
		}
            }
paxos.LOGV.Println("in executeCommand:handle put done")
        case command.Get:
            if st, ok := server.store[c.Key]; ok {
		if replyCh != nil {
                replyCh <- &chubbyrpc.ChubbyReply{
                    Status: chubbyrpc.OK,
                    Value: st.value,
                }
		}
            } else {
		if replyCh != nil {
                replyCh <- &chubbyrpc.ChubbyReply{
                    Status: chubbyrpc.FAIL,
                }
		}
            }
        case command.Acquire:
            if st, ok := server.store[c.Key]; ok {
                if st.locked {
		if replyCh != nil {
                    replyCh <- &chubbyrpc.ChubbyReply{
                        Status: chubbyrpc.FAIL,
                    }
		}
                } else {
                    st.locked = true
                    st.lockstamp = strconv.FormatInt(time.Now().UnixNano(), 10)
		if replyCh != nil {
                    replyCh <- &chubbyrpc.ChubbyReply{
                        Status: chubbyrpc.OK,
                        Value: st.lockstamp,
                    }
		}
                }
            } else {
		if replyCh != nil {
                replyCh <- &chubbyrpc.ChubbyReply{
                    Status: chubbyrpc.FAIL,
                }
		}
            }
        case command.Release:
            if st, ok := server.store[c.Key]; ok {
                if !st.locked || st.lockstamp != c.Value {
		if replyCh != nil {
                    replyCh <- &chubbyrpc.ChubbyReply{
                        Status: chubbyrpc.FAIL,
                    }
		}
                } else {
                    st.locked = false
		if replyCh != nil {
                    replyCh <- &chubbyrpc.ChubbyReply{
                        Status: chubbyrpc.OK,
                    }
		}
                }
            } else {
		if replyCh != nil {
                replyCh <- &chubbyrpc.ChubbyReply{
                    Status: chubbyrpc.FAIL,
                }
		}
            }
        }
        server.nextIndex++
    }
    server.lock.Unlock()
paxos.LOGV.Println("leaving executeCommand")
}
