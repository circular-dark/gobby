package paxosnode

import "rpc/paxosrpc"
import "command"

type PaxosNode interface {
	//Paxos protocal prepare rpc, called by proposer
	Prepare(args *paxosrpc.PrepareArgs, reply *paxosrpc.PrepareReply) error

	//Paxos protocal accept rpc, called by proposer
	Accept(args *paxosrpc.AcceptArgs, reply *paxosrpc.AcceptReply) error

	//Paxos protocal commit rpc, called by proposer
	CommitAndReply(args *paxosrpc.CommitArgs, reply *paxosrpc.CommitReply) error
	Commit(args *paxosrpc.CommitArgs) error

	//The proposer calles this function, it will not return
	//until it succees, or some fatal error happens
	DoPrepare(args *paxosrpc.PrepareArgs, reply *paxosrpc.PrepareReply) error

	//The leader calles this function, trying to let followers accept value
	//If it returns error, paxos has to restart from DoPrepare again
	DoAccept(args *paxosrpc.AcceptArgs, reply *paxosrpc.PrepareReply) error

	//The leader calles this function, trying to commit value to log
	DoCommit(args *paxosrpc.CommitArgs) error

	//Interface to the application, which tries to replicate command.
	Replicate(command *command.Command) error

}
