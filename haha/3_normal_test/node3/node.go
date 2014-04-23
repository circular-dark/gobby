package main

import (
    "fmt"
    "time"
    "strconv"
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
    time.Sleep(5 * time.Second)
    if n2 == nil {
        fmt.Println("Cannot start node.\n")
        fmt.Println(err)
        return
    }
    for i := 0; i < 50; i++ {
        c := command.Command{strconv.Itoa(nid), strconv.Itoa(i), command.Put}
        n2.Replicate(&c)
    }
    time.Sleep(10 * time.Second)
    n2.DumpLog()
}
