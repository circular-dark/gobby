package test

import (
  "testing"
  "github.com/gobby/src/paxos"
  "github.com/gobby/src/command"
)


func TestCreateNode(t *testing.T) {
  callback := make(chan *paxos.IndexCommand)
  n3,err:=paxos.NewPaxosNode("", 3, 9992, 2, callback)
  if n3 == nil {
    LOGE.Println("Cannot start node.\n")
    LOGE.Println(err)
    return
  }
  c := command.Command{"111", "222", command.Put}
  n3.Replicate(&c)
  d, ok :=<-callback
  if ok {
    LOGE.Printf("have commited %d:%s\n", d.Index, d.V.ToString())
  }
  c = command.Command{"333", "444", command.Put}
  n3.Replicate(&c)
  d, ok =<-callback
  if ok {
    LOGE.Printf("have commited %d:%s\n", d.Index, d.V.ToString())
  }
  c = command.Command{"555", "666", command.Put}
  n3.Replicate(&c)
  d, ok =<-callback
  if ok {
    LOGE.Printf("have commited %d:%s\n", d.Index, d.V.ToString())
  }
}

