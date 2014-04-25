package chubbyserver

import (
    "sync"
    "strconv"
    "time"
    "net/rpc"
    "github.com/gobby/src/rpc/chubbyrpc"
    "github.com/gobby/src/paxos"
    "github.com/gobby/src/command"
)

type kstate struct {
    value string
    lockstamp string // a unixtime int64 stamp for identifying the lock (sequencer value)
    locked bool
}

type chubbyserver struct {
    store map[string]*kstate
    commandLog []*command.Command
    nextIndex int // next index of command that should be executed
    nextCID int // next command ID that should be assigned
    replyChs map[int]chan *chubbyrpc.ChubbyReply // maps command id to channels that block RPC calls
    lock sync.Mutex
    paxosnode paxos.PaxosNode
}

func NewChubbyServer(nodeID int, numNodes int) (Chubbyserver, error) {
    server := new(chubbyserver)
    server.store = make(map[string]*kstate)
    server.commandLog = make([]*command.Command, 0)
    server.nextIndex = 0
    if node, err := paxos.NewPaxosNode(nodeID, numNodes, server.getCommand); err != nil {
        return nil, err
    } else {
        server.paxosnode = node
    }

	if err := rpc.RegisterName("ChubbyServer", chubbyrpc.Wrap(server)); err != nil {
		return nil, err
	} else {
        return server, nil
    }
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
    server.paxosnode.Replicate(c)

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
    server.paxosnode.Replicate(c)

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
    server.paxosnode.Replicate(c)

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
    server.paxosnode.Replicate(c)

    // blocking for reply
    r := <-replyCh
    reply.Status = r.Status
    return nil
}

func (server *chubbyserver) getCommand(index int, c command.Command) {
    server.lock.Lock()
    for len(server.commandLog) < index + 1 {
        server.commandLog = append(server.commandLog, nil)
    }
    server.commandLog[index] = &c
    server.lock.Unlock()
    server.executeCommands()
}

func (server *chubbyserver) executeCommands() {
    server.lock.Lock()
    for server.nextIndex < len(server.commandLog) && server.commandLog[server.nextIndex] != nil {
        c := server.commandLog[server.nextIndex]
        cid := c.ID
        replyCh := server.replyChs[cid]
        delete(server.replyChs, cid)

        switch (c.Type) {
        case command.Put:
            if st, ok := server.store[c.Key]; ok {
                st.value = c.Value
                replyCh <- &chubbyrpc.ChubbyReply{
                    Status: chubbyrpc.OK,
                }
            } else {
                server.store[c.Key] = &kstate{
                    value: c.Value,
                    locked: false,
                }
                replyCh <- &chubbyrpc.ChubbyReply{
                    Status: chubbyrpc.OK,
                }
            }
        case command.Get:
            if st, ok := server.store[c.Key]; ok {
                replyCh <- &chubbyrpc.ChubbyReply{
                    Status: chubbyrpc.OK,
                    Value: st.value,
                }
            } else {
                replyCh <- &chubbyrpc.ChubbyReply{
                    Status: chubbyrpc.FAIL,
                }
            }
        case command.Acquire:
            if st, ok := server.store[c.Key]; ok {
                if st.locked {
                    replyCh <- &chubbyrpc.ChubbyReply{
                        Status: chubbyrpc.FAIL,
                    }
                } else {
                    st.locked = true
                    st.lockstamp = strconv.FormatInt(time.Now().UnixNano(), 10)
                    replyCh <- &chubbyrpc.ChubbyReply{
                        Status: chubbyrpc.OK,
                        Value: st.lockstamp,
                    }
                }
            } else {
                replyCh <- &chubbyrpc.ChubbyReply{
                    Status: chubbyrpc.FAIL,
                }
            }
        case command.Release:
            if st, ok := server.store[c.Key]; ok {
                if !st.locked || st.lockstamp != c.Value {
                    replyCh <- &chubbyrpc.ChubbyReply{
                        Status: chubbyrpc.FAIL,
                    }
                } else {
                    st.locked = false
                    replyCh <- &chubbyrpc.ChubbyReply{
                        Status: chubbyrpc.OK,
                    }
                }
            } else {
                replyCh <- &chubbyrpc.ChubbyReply{
                    Status: chubbyrpc.FAIL,
                }
            }
        }
        server.nextIndex++
    }
    server.lock.Unlock()
}
