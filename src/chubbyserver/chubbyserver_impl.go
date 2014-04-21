package chubbyserver

import (
    "github.com/gobby/src/rpc/chubbyrpc"
    "github.com/gobby/src/paxos"
    "github.com/gobby/src/command"
)

const (
    masterHostPort = "masterHostPort"
    clientReqPrefix = "%"
    lockPrefix = "#"
)

type chubbyserver struct {
    kvstore map[string]string
    kvlock map[string]bool
    isMaster bool
    masterHostPort string
    commandLog []*command.Command
    paxosnode paxos.PaxosNode
}

func NewChubbyServer(hostport string, numNodes int, port int, nodeID int) (Chubbyserver, error) {
    server := new(chubbyserver)
    server.kvstore = make(map[string]string)
    server.kvlock = make(map[string]bool)
    server.isMaster = false
    server.masterHostPort = ""
    var err error
    if server.paxosnode, err = paxos.NewPaxosNode(hostport, numNodes, port, nodeID); err != nil {
        return nil, err
    } else {
        return server, nil
    }
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
