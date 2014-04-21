package main

import "github.com/gobby/src/paxos"

func main() {
  callback := make(chan *paxos.IndexCommand)
  _, err := paxos.NewPaxosNode("", 3, 9990,0,callback)
  if err!=nil{
    paxos.LOGE.Println(err)
  } else {
    _=<-callback
  }
}
