package test

import (
  "testing"
  "github.com/gobby/src/paxos"
)

func TestCreateNode(t *testing.T) {
  n1,err := paxos.NewPaxosNode("", 3, 9990, 0)
  if err!= nil {
    LOGE.Println(err)
  }
  n2,err := paxos.NewPaxosNode("", 3, 9991, 1)
  if err!= nil {
    LOGE.Println(err)
  }
  n3,err := paxos.NewPaxosNode("", 3, 9992, 2)
  if err!= nil {
    LOGE.Println(err)
  }

  if n1!= nil && n2 != nil && n3 != nil {
    LOGE.Println("Create 3 nodes.")
  n1.Terminate()
  n2.Terminate()
  n3.Terminate()
  }
  /*if n1!=nil {
    n1.Terminate()
  }*/

}

