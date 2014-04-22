package main

import (
    "fmt"
    "time"
    "github.com/gobby/src/paxos"
    "github.com/gobby/src/config"
    "github.com/gobby/src/command"
)

const (
    nid = 2
)

func fakecallback(index int, c command.Command) {
    fmt.Printf("\n%d's index %d is %s\n", nid, index, c.ToString())
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
    for i := 0; i < 50; i++ {
        c := command.Command{"111", "222", command.Put}
        n3.Replicate(&c)
    }
    time.Sleep(60 * time.Second)
}
