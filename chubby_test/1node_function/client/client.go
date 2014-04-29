package main

import (
	"github.com/gobby/src/chubbyclient"
	"fmt"
)

func main() {
  client,err := chubbyclient.NewClient(1)
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
  fmt.Println("Client Aquire test")
  client.Aquire("test")
  fmt.Println("Client Release test")
  client.Release("test")
}

