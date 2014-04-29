package main

import (
	"github.com/gobby/src/chubbyclient"
	"fmt"
)

func main() {
  client,err := chubbyclient.NewClient(-1, 1)
  if err!=nil{
    fmt.Println("wrong")
  }
  fmt.Println("Client Put test:1")
  client.Put("test", "1")
  fmt.Println("Client Get test")
  s,err := client.Get("test")
  if err!=nil{
    fmt.Println("wrong get")
    fmt.Println(err)
  }
  fmt.Println(s)
  fmt.Println("Client Acquire test")
  ts, err := client.Acquire("test")
  fmt.Printf("Client Acquire test, get %s\n", ts)
  fmt.Println("Client Release test")
  client.Release("test")
}

