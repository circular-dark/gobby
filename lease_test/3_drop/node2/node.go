package main

import (
	"fmt"
	"github.com/gobby/src/lease"
	"github.com/gobby/src/config"
	"net"
	"github.com/gobby/src/rpc/rpcwrapper"
	"net/http"
	"net/rpc"
)

const (
	nid      = 1
	numNodes = 3
)

func main() {
	rpcwrapper.SetForwardDropRate(20)
	rpcwrapper.SetBackwardDropRate(20)
	fmt.Printf("node %d starts\n", nid)
	_, err := lease.NewLeaseNode(nid, numNodes)
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
