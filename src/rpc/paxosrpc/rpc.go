package paxosrpc

type RemotePaxosNode interface {
	Prepare(args *PrepareArgs, reply *PrepareReply) error
	Accept(args *AcceptArgs, reply *AcceptReply) error
	Commit(args *CommitArgs, reply *CommitReply) error
}

type PaxosNode struct {
	RemotePaxosNode
}

func Warp(t RemotePaxosNode) RemotePaxosNode {
	return &RemotePaxosNode{t}
}
