package chubbyserver

import (
	"github.com/gobby/src/rpc/chubbyrpc"
)

type Chubbyserver interface {
	Put(args *chubbyrpc.PutArgs, reply *chubbyrpc.ChubbyReply) error
	Get(args *chubbyrpc.GetArgs, reply *chubbyrpc.ChubbyReply) error
	Acquire(args *chubbyrpc.AcquireArgs, reply *chubbyrpc.ChubbyReply) error
	Release(args *chubbyrpc.ReleaseArgs, reply *chubbyrpc.ChubbyReply) error
    CheckMaster(args *chubbyrpc.CheckArgs, reply *chubbyrpc.ChubbyReply) error
}
