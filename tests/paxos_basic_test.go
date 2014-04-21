package test

import (
  "testing"
  "github.com/gobby/src/paxos"
  "github.com/gobby/src/command"
)


func TestCreateNode(t *testing.T) {
  n3,err:=paxos.NewPaxosNode("", 3, 9992, 2, callback)
  if n3 == nil {
    LOGE.Println("Cannot start node.\n")
    LOGE.Println(err)
    return
  }
  c := command.Command{"111", "222", command.Put}
  n3.Replicate(&c)
  if ok {
    LOGE.Printf("have commited %d:%s\n", d.Index, d.V.ToString())
  }
}

