package chubbyrpc

type RemoteChubbyServer interface {
	Put(args *PutArgs, reply *ChubbyReply) error
	Get(args *GetArgs, reply *ChubbyReply) error
	Acquire(args *AcquireArgs, reply *ChubbyReply) error
	Release(args *ReleaseArgs, reply *ChubbyReply) error
    CheckMaster(args *CheckArgs, reply *ChubbyReply) error
	Watch(args *WatchArgs, reply *ChubbyReply) error
}

type ChubbyServer struct {
	RemoteChubbyServer
}

func Wrap(t RemoteChubbyServer) RemoteChubbyServer {
	return &ChubbyServer{t}
}
