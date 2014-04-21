package paxosrpc

type RemotePaxosNode interface {
	Prepare(args *PrepareArgs, reply *PrepareReply) error
	Accept(args *AcceptArgs, reply *AcceptReply) error
	CommitAndReply(args *CommitArgs, reply *CommitReply) error
	Commit(args *CommitArgs, reply *CommitReply) error
}

type PaxosNode struct {
	RemotePaxosNode
}

func Wrap(t RemotePaxosNode) RemotePaxosNode {
	return &PaxosNode{t}
}
