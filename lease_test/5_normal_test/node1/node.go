package main

import (
	"fmt"
	"github.com/gobby/src/lease"
)

const (
	nid      = 0
	numNodes = 5
)

var done = make(chan int)

func main() {
	fmt.Printf("node %d starts\n", nid)
	_, err := lease.NewLeaseNode(nid, numNodes)
	if err != nil {
		fmt.Println("Cannot start node.\n")
		fmt.Println(err)
		return
	}
	<-done
	fmt.Printf("node %d closes\n", nid)
}
