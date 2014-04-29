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

type GetArgs struct {
	Key string
}

type AcquireArgs struct {
	Key string
}

type ReleaseArgs struct {
	Key string
    Lockstamp string
}

type ChubbyReply struct {
	Status Status
    Value string
}
