package leaserpc

type RemoteLeaseNode interface {
	Prepare(args *PrepareArgs, reply *PrepareReply) error
	Accept(args *AcceptArgs, reply *AcceptReply) error
}

type LeaseNode struct {
	RemoteLeaseNode
}

func Wrap(t RemoteLeaseNode) RemoteLeaseNode {
	return &LeaseNode{t}
}
