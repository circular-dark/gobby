package paxos

import "github.com/gobby/src/rpc/paxosrpc"
import "github.com/gobby/src/command"
import "net"

type PaxosNode interface {
	//Paxos protocal prepare rpc, called by proposer
	Prepare(args *paxosrpc.PrepareArgs, reply *paxosrpc.PrepareReply) error

	//Paxos protocal accept rpc, called by proposer
	Accept(args *paxosrpc.AcceptArgs, reply *paxosrpc.AcceptReply) error

	//Paxos protocal commit rpc, called by proposer
	Commit(args *paxosrpc.CommitArgs, reply *paxosrpc.CommitReply) error

	//The proposer calles this function, it will not return
	//until it succees, or some fatal error happens
	DoPrepare(args *paxosrpc.PrepareArgs, reply *paxosrpc.PrepareReply) error

	//The leader calles this function, trying to let followers accept value
	//If it returns error, paxos has to restart from DoPrepare again
	DoAccept(args *paxosrpc.AcceptArgs, reply *paxosrpc.AcceptReply) error

	//The leader calles this function, trying to commit value to log
	DoCommit(args *paxosrpc.CommitArgs) error

	//Interface to the application, which tries to replicate command.
	Replicate(command *command.Command) error

	//For testing
	//Pause the Node, it does not receive any rpc calls. But it is still able to
	//call other node. So it is the tester's reposibility to make sure not to call
	//others so that the tester can simulate the network error for this node.
	SetListener(listener *net.Listener)
	Pause() error
	Resume() error

	//Kill the Node, then we can recover the state with the log on disk.
	//But it is a SECOND-TIER objective, or say, we do not use this now.
	Terminate() error

	//Print commited logs
	DumpLog() error
}

type PaxosCallBack func(int, command.Command)
