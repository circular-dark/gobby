package main

import (
	"fmt"
	"github.com/gobby/src/command"
	"github.com/gobby/src/config"
	"github.com/gobby/src/paxos"
	"net"
	"net/http"
	"net/rpc"
	"strconv"
	"time"
)

const (
	nid      = 0
	numNodes = 5
)

var done = make(chan struct{}, 15000)

func fakecallback(index int, c command.Command) {
	fmt.Printf("\n%d's index %d is %s\n", nid, index, c.ToString())
	done <- struct{}{}
}

func main() {
	fmt.Printf("node %d starts\n", nid)
	node, err := paxos.NewPaxosNode(nid, numNodes, fakecallback)
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
	node.SetListener(&listener)
	rpc.HandleHTTP()
	go http.Serve(listener, nil)

	time.Sleep(5 * time.Second)
	for i := 0; i < 2000; i++ {
		c := command.Command{strconv.Itoa(nid), strconv.Itoa(i), command.Put, i, ""}
		node.Replicate(&c)
	}
	for res := 0; res < 10000; res++ {
		_, ok := <-done
		if !ok {
			break
		}
	}
	node.DumpLog()
	fmt.Printf("node %d closes\n", nid)
}
