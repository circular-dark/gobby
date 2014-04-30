package main

import (
	"fmt"
	"github.com/gobby/src/chubbyserver"
	"github.com/gobby/src/config"
	"net"
	"net/http"
	"net/rpc"
	"github.com/gobby/src/rpc/rpcwrapper"
)

const (
	nid      = 2
	numNodes = 3
)

func main() {
	rpcwrapper.SetForwardDropRate(5)
	rpcwrapper.SetBackwardDropRate(5)
	//rpcwrapper.ResetDropRate()
	fmt.Printf("server %d starts\n", nid)
	_, err := chubbyserver.NewChubbyServer(nid, numNodes)
	if err != nil {
		fmt.Println("Cannot start node.\n")
		fmt.Println(err)
		return
	}
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", config.Nodes[nid].Port))
	if err != nil {
		fmt.Printf("node %d cannot listen to port:%s\n", err)
		return
	}
	rpc.HandleHTTP()
	http.Serve(listener, nil)
}
