package chubbyrpc

type Status int

const (
	OK      Status = iota + 1 //The RPC was a success
	FAIL                      //The RPC failed
)

type PutArgs struct {
	Key string
    Value string
}

type PutReply struct {
	Status Status
}

type GetArgs struct {
	Key string
}

type GetReply struct {
	Status Status
    Value string
}

type AquireArgs struct {
	Key string
}

type AquireReply struct {
	Status Status
}

type ReleaseArgs struct {
	Key string
}

type ReleaseReply struct {
	Status Status
}

type GetMasterArgs struct {
}

type GetMasterReply struct {
	Status Status
    Hostport string
}
