package paxosrpc

type Status int

const (
	OK      Status = iota + 1 //The RPC was a success
	Reject                    //The RPC was a rejection
	Existed                   //The RPC was Prepare-OK, but we have commited a value in the slot
)

type PrepareArgs struct {
	SlotIdx int //Command Slot Index
	N       int
	//TODO: Design a struct instead of a pure string
	Command string
}

type PrepareReply struct {
	Status  Status
	Na      int
	Command string
}

type AcceptArgs struct {
	N       int
	Command string
}

type AcceptReply struct {
	Status Status
}

type CommitArgs struct {
	N       int
	Command string
}

type CommitReply struct {
	Status Status
}
