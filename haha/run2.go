package main

import (
    "fmt"
    "time"
    "github.com/gobby/src/paxos"
    "github.com/gobby/src/config"
    "github.com/gobby/src/command"
)

const (
    nid = 1
)

func fakecallback(index int, c command.Command) {
    fmt.Printf("\n%d's index %d is %s\n", nid, index, c.ToString())
}

func main() {
    n2, err := paxos.NewPaxosNode(config.Nodes[nid].Address,
                                  config.Nodes[nid].Port,
                                  config.Nodes[nid].NodeID,
                                  fakecallback)
    if n2 == nil {
        fmt.Println("Cannot start node.\n")
        fmt.Println(err)
        return
    }
    c := command.Command{"777", "888", command.Put}
    n2.Replicate(&c)
    time.Sleep(15 * time.Second)
}
