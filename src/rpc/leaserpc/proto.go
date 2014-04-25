package leaserpc

type Status int

const (
	OK      Status = iota + 1 //The RPC was a success
	Reject                    //The RPC was a rejection
)

type PrepareArgs struct {
	N       int
}

type PrepareReply struct {
	Status Status
}

type AcceptArgs struct {
	N int
	Span int // number of periods
}

type AcceptReply struct {
	Status Status
}
