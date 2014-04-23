package main

import (
    "fmt"
    "time"
    "github.com/gobby/src/paxos"
    "github.com/gobby/src/config"
    "github.com/gobby/src/command"
)

const (
    nid = 0
)

func fakecallback(index int, c command.Command) {
    fmt.Printf("\n%d's index %d is %s\n", nid, index, c.ToString())
}

func main() {
    n1, err := paxos.NewPaxosNode(config.Nodes[nid].Address,
                                  config.Nodes[nid].Port,
                                  config.Nodes[nid].NodeID,
                                  fakecallback)
    time.Sleep(5 * time.Second)
    if n1 == nil {
        fmt.Println("Cannot start node.\n")
        fmt.Println(err)
        return
    }
    for i := 0; i < 50; i++ {
        c := command.Command{"333", "4444", command.Put}
        n1.Replicate(&c)
    }
    time.Sleep(60 * time.Second)
    n1.DumpLog()
}
