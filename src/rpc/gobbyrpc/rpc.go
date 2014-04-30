package gobbyrpc

type RemotegobbyServer interface {
	Put(args *PutArgs, reply *GobbyReply) error
	Get(args *GetArgs, reply *GobbyReply) error
	Acquire(args *AcquireArgs, reply *GobbyReply) error
	Release(args *ReleaseArgs, reply *GobbyReply) error
	CheckMaster(args *CheckArgs, reply *GobbyReply) error
	Watch(args *WatchArgs, reply *GobbyReply) error
}

type gobbyServer struct {
	RemotegobbyServer
}

func Wrap(t RemotegobbyServer) RemotegobbyServer {
	return &gobbyServer{t}
}
