package lease

import (
    "github.com/gobby/src/rpc/leaserpc"
)

type LeaseNode interface {
	Prepare(args *leaserpc.PrepareArgs, reply *leaserpc.PrepareReply) error
	Accept(args *leaserpc.AcceptArgs, reply *leaserpc.AcceptReply) error
    CheckMaster() bool
}
