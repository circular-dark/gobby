package chubbyserver

import (
    "github.com/gobby/src/rpc/chubbyrpc"
)

type Chubbyserver interface {
	Put(args *chubbyrpc.PutArgs, reply *chubbyrpc.ChubbyReply) error
	Get(args *chubbyrpc.GetArgs, reply *chubbyrpc.ChubbyReply) error
	Aquire(args *chubbyrpc.AquireArgs, reply *chubbyrpc.ChubbyReply) error
	Release(args *chubbyrpc.ReleaseArgs, reply *chubbyrpc.ChubbyReply) error
}
