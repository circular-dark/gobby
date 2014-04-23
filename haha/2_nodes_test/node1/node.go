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
	time.Sleep(5 * time.Second)
	if err != nil {
		fmt.Println("Cannot start node.\n")
		fmt.Println(err)
		return
	}
	go func() {
		for i := 0; i < 2; i++ {
			c := command.Command{"111", "222", command.Put}
			n3.Replicate(&c)
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
	}()

	res := 0
	for res < 6 {
		_, ok := <-done
		if ok {
			res++
		} else {
			break
		}
	}

	if res == 6 {
		fmt.Printf("\n%d receive all commands\n", nid)
	} else {
		fmt.Printf("%d Just break!!!!!\n", res)
	}
}
