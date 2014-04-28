package chubbyrpc

type RemoteChubbyServer interface {
	Put(args *PutArgs, reply *ChubbyReply) error
	Get(args *GetArgs, reply *ChubbyReply) error
	Aquire(args *AquireArgs, reply *ChubbyReply) error
	Release(args *ReleaseArgs, reply *ChubbyReply) error
}

type ChubbyServer struct {
	RemoteChubbyServer
}

func Wrap(t RemoteChubbyServer) RemoteChubbyServer {
	return &ChubbyServer{t}
}
