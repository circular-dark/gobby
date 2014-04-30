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
	nid      = 3
	numNodes = 5
)

var done = make(chan struct{})

func fakecallback(index int, c command.Command) {
	fmt.Printf("\n%d's index %d is %s\n", nid, index, c.ToString())
	done <- struct{}{}
}

func main() {
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
	go func() {
		for i := 0; i < 5; i++ {
			c := command.Command{strconv.Itoa(nid), strconv.Itoa(i), command.Put, i, ""}
			node.Replicate(&c)
		}
	}()

	res := 0
	for res < 22 {
		_, ok := <-done
		if ok {
			res++
		} else {
			break
		}
	}

	if res == 22 {
		fmt.Printf("\n%d receive all commands\n", nid)
	} else {
		fmt.Printf("%d Just break %d!!!!!\n", nid, res)
	}
	node.DumpLog()
	time.Sleep(15 * time.Second)
}
