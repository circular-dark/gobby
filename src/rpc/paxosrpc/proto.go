package paxosrpc

import "command"

type Status int

const (
	OK      Status = iota + 1 //The RPC was a success
	Reject                    //The RPC was a rejection
	Existed                   //The RPC was Prepare-OK, but we have commited a value in the slot
)

type PrepareArgs struct {
	SlotIdx int //Command Slot Index
	N       int
	V	command.Command
}

type PrepareReply struct {
	Status Status
	Na     int
	Va     command.Command
}

type AcceptArgs struct {
	SlotIdx int //Command Slot Index
	N int
	V command.Command
}

type AcceptReply struct {
	Status Status
}

type CommitArgs struct {
	SlotIdx int //Command Slot Index
	N int
	V command.Command
}

type CommitReply struct {
	Status Status
}
