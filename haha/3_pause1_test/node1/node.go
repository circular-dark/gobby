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

func fakecallback(index int, c command.Command) {
	fmt.Printf("\n%d's index %d is %s\n", nid, index, c.ToString())
}

func main() {
	done := make(chan struct{})
	n3, err := paxos.NewPaxosNode(config.Nodes[nid].Address,
		config.Nodes[nid].Port,
		config.Nodes[nid].NodeID,
		fakecallback)
	time.Sleep(5 * time.Second)
	if err != nil {
		fmt.Println("Cannot start node.\n")
		fmt.Println(err)
		return
	}
	go func() {
		for i := 0; i < 50; i++ {
			c := command.Command{"111", "222", command.Put}
			n3.Replicate(&c)
		}
	}()

	res := 0
	for res < 150 {
		_, ok := <-done
		if ok {
			res++
		} else {
			break
		}
	}

	if res == 150 {
		fmt.Printf("\n%d receive all commands\n", nid)
	}
}
