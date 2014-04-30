package gobbyserver

import (
	"github.com/gobby/src/rpc/gobbyrpc"
)

type gobbyserver interface {
	Put(args *gobbyrpc.PutArgs, reply *gobbyrpc.GobbyReply) error
	Get(args *gobbyrpc.GetArgs, reply *gobbyrpc.GobbyReply) error
	Acquire(args *gobbyrpc.AcquireArgs, reply *gobbyrpc.GobbyReply) error
	Release(args *gobbyrpc.ReleaseArgs, reply *gobbyrpc.GobbyReply) error
	CheckMaster(args *gobbyrpc.CheckArgs, reply *gobbyrpc.GobbyReply) error
	Watch(args *gobbyrpc.WatchArgs, reply *gobbyrpc.GobbyReply) error
}
