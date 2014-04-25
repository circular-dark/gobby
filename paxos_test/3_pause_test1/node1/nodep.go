package main

import (
	"fmt"
	"github.com/gobby/src/command"
	"github.com/gobby/src/config"
	"github.com/gobby/src/paxos"
	"time"
)

const (
	nid = 2
)

var done = make(chan struct{})

func fakecallback(index int, c command.Command) {
	fmt.Printf("\n%d's index %d is %s\n", nid, index, c.ToString())
	done <- struct{}{}
}

func main() {
	n3, err := paxos.NewPaxosNode(config.Nodes[nid].Address,
		config.Nodes[nid].Port,
		config.Nodes[nid].NodeID,
		fakecallback)
	if err != nil {
		fmt.Println("Cannot start node.\n")
		fmt.Println(err)
		return
	}
	for i := 0; i < 2; i++ {
		err = n3.Pause()
		if err != nil {
			fmt.Println("Cannot Pause node.\n")
			fmt.Println(err)
			return
		}
		time.Sleep(1 * time.Second)
		err = n3.Resume()
		if err != nil {
			fmt.Println("Cannot Resume node.\n")
			fmt.Println(err)
			return
		}
	}

}
