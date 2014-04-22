package paxosrpc

import "github.com/gobby/src/command"

type Status int

const (
	OK    Status = iota + 1
	Reject
    Accepted
    Committed
)

type PrepareArgs struct {
	SlotIdx int
	N       int
}

type PrepareReply struct {
	Status Status
	N     int
	V     command.Command
}

type AcceptArgs struct {
	SlotIdx int
	N int
	V command.Command
}

type AcceptReply struct {
	Status Status
	N int
	V command.Command
}

type CommitArgs struct {
	SlotIdx int
	N int
	V command.Command
}

type CommitReply struct {
	Status Status
}
