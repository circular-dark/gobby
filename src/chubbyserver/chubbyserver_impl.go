package chubbyserver

import (
    "sync"
    "time"
    "github.com/gobby/src/rpc/chubbyrpc"
    "github.com/gobby/src/paxos"
    "github.com/gobby/src/command"
)

const (
    masterLeaseLength int = 20 // 20 tickUnits
    slaveLeaseLength int = 30 // 30 tickUnits
    tickUnit int = 100 // 100 milliseconds
)

type chubbyserver struct {
    kvstore map[string]string
    kvlock map[string]string
    masterHostPort string
    myHostPort string
    commandLog []*command.Command
    masterLease int // lease length is a multiple of 100ms
    nextIndex int // next index of command that should be executed
    closeCh chan struct{} // channel for close associate goroutines
    lock *sync.Mutex
    paxosnode paxos.PaxosNode
}

func NewChubbyServer(hostport string, numNodes int, port int, nodeID int) (Chubbyserver, error) {
    server := new(chubbyserver)
    server.kvstore = make(map[string]string)
    server.kvlock = make(map[string]string)
    server.masterHostPort = ""
    server.myHostPort = hostport
    server.commandLog = make([]*command.Command, 0)
    server.nextIndex = 0
    server.closeCh = make(chan struct{})
    server.lock = new(sync.Mutex)
    var err error
    if server.paxosnode, err = paxos.NewPaxosNode(hostport, numNodes, port, nodeID); err != nil {
        return nil, err
    } else {
        go server.leaseManager()
        return server, nil
    }
}

func (server *chubbyserver) Close() {
    close(server.closeCh)
}

func (server *chubbyserver) Put(args *chubbyrpc.PutArgs, reply *chubbyrpc.PutReply) error {
    return nil
}

func (server *chubbyserver) Get(args *chubbyrpc.GetArgs, reply *chubbyrpc.GetReply) error {
    return nil
}

func (server *chubbyserver) Aquire(args *chubbyrpc.AquireArgs, reply *chubbyrpc.AquireReply) (bool, error) {
    return false, nil
}

func (server *chubbyserver) Release(args *chubbyrpc.ReleaseArgs, reply *chubbyrpc.ReleaseReply) error {
    return nil
}

func (server *chubbyserver) GetMasterHostport(args *chubbyrpc.GetMasterArgs, reply *chubbyrpc.GetMasterReply) error {
    return nil
}

func (server *chubbyserver) getCommandCallBack(index int, c *command.Command) {
    server.lock.Lock()
    if index >= len(server.commandLog) {
        for len(server.commandLog) < index + 1 {
            server.commandLog = append(server.commandLog, nil)
        }
    }
    server.commandLog[index] = c
    server.lock.Unlock()
    server.executeCommands()
}

func (server *chubbyserver) executeCommands() {
    server.lock.Lock()
    for server.nextIndex < len(server.commandLog) && server.commandLog[server.nextIndex] != nil {
        c := server.commandLog[server.nextIndex]
        switch (c.Type) {
        case command.Put:
            server.kvstore[c.Key] = c.Value
        case command.Get:
        case command.Acquire:
            if val, ok := server.kvlock[c.Key]; ok && val == "" {
                server.kvlock[c.Key] = c.Value
            }
        case command.Release:
            if val, ok := server.kvlock[c.Key]; ok && val == c.Value {
                server.kvlock[c.Key] = ""
            }
        case command.Bemaster:
            if c.Key == server.myHostPort {
                server.masterHostPort = server.myHostPort
                server.masterLease = masterLeaseLength
            } else {
                server.masterHostPort = c.Key
                server.masterLease = slaveLeaseLength
            }
        }
        server.nextIndex++
    }
    server.lock.Unlock()
}

func (server *chubbyserver) leaseManager() {
    tick := time.NewTicker(time.Duration(tickUnit) * time.Millisecond)
    for {
        select {
        case <-tick.C:
            server.lock.Lock()
            if server.masterHostPort == "" || server.masterLease <= 0 {
                server.masterHostPort = ""
                c := &command.Command{
                    Key: server.myHostPort,
                    Type: command.Bemaster,
                }
                server.paxosnode.Replicate(c)
            } else {
                server.masterLease--
            }
            server.lock.Unlock()
        case <-server.closeCh:
            return
        }
    }
}
