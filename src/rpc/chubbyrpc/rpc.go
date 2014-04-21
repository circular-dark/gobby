package chubbyrpc

type RemoteChubbyServer interface {
	Put(args *PutArgs, reply *PutReply) error
	Get(args *GetArgs, reply *GetReply) error
	Aquire(args *AquireArgs, reply *AquireReply) (bool, error)
	Release(args *ReleaseArgs, reply *ReleaseReply) error
    GetMasterHostport(args *GetMasterArgs, reply *GetMasterReply) error
}

type ChubbyServer struct {
	RemoteChubbyServer
}

func Warp(t RemoteChubbyServer) RemoteChubbyServer {
	return &ChubbyServer{t}
}
