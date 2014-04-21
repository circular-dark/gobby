package main

import "github.com/gobby/src/paxos"

func main() {
  n1, err := paxos.NewPaxosNode("", 3, 9990,0)
}
