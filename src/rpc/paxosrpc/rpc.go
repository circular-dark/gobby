package paxosrpc

type RemotePaxosNode interface {
	Prepare(args *PrepareArgs, reply *PrepareReply) error
	Accept(args *AcceptArgs, reply *AcceptReply) error
	CommitAndReply(args *CommitArgs, reply *CommitReply) error
	Commit(args *CommitArgs) error
}

type PaxosNode struct {
	RemotePaxosNode
}

func Warp(t RemotePaxosNode) RemotePaxosNode {
	return &RemotePaxosNode{t}
}
