package main

import (
    "fmt"
    "time"
    "strconv"
    "github.com/gobby/src/paxos"
    "github.com/gobby/src/command"
)

const (
    nid = 0
    numNodes = 3
)

var done = make(chan struct{}, 150)

func fakecallback(index int, c command.Command) {
    fmt.Printf("\n%d's index %d is %s\n", nid, index, c.ToString())
	done<-struct{}{}
}

func main() {
    fmt.Printf("node %d starts\n", nid)
    node, err := paxos.NewPaxosNode(nid, numNodes, fakecallback)
    if err != nil {
        fmt.Println("Cannot start node.\n")
        fmt.Println(err)
        return
    }
    time.Sleep(5 * time.Second)
    for i := 0; i < 50; i++ {
        c := command.Command{strconv.Itoa(nid), strconv.Itoa(i), command.Put, i}
        node.Replicate(&c)
    }
	for res := 0; res < 150; res++ {
		_, ok := <-done
		if !ok {
	        break
		}
	}
    node.DumpLog()
    fmt.Printf("node %d closes\n", nid)
}
