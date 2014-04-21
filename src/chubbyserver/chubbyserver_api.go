package chubbyserver

import (
    "github.com/gobby/src/rpc/chubbyrpc"
)

type Chubbyserver interface {
	Put(args *chubbyrpc.PutArgs, reply *chubbyrpc.PutReply) error
	Get(args *chubbyrpc.GetArgs, reply *chubbyrpc.GetReply) error
	Aquire(args *chubbyrpc.AquireArgs, reply *chubbyrpc.AquireReply) error
	Release(args *chubbyrpc.ReleaseArgs, reply *chubbyrpc.ReleaseReply) error
    GetMasterHostport(args *chubbyrpc.GetMasterArgs, reply *chubbyrpc.GetMasterReply) error
}
