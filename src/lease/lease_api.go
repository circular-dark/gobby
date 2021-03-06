package lease

import (
	"github.com/gobby/src/rpc/leaserpc"
)

type LeaseNode interface {
	Prepare(args *leaserpc.Args, reply *leaserpc.Reply) error
	Accept(args *leaserpc.Args, reply *leaserpc.Reply) error
	RenewPrepare(args *leaserpc.Args, reply *leaserpc.Reply) error
	RenewAccept(args *leaserpc.Args, reply *leaserpc.Reply) error
	CheckMaster() bool
}
