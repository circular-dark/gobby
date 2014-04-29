package main

import (
	"fmt"
	"github.com/gobby/src/chubbyclient"
	"time"
)

func main() {
	//TODO:need modify NewClient parameter, now is the numNodes
	client, err := chubbyclient.NewClient(0, 0)
	if err != nil {
		fmt.Println("wrong")
	}
	fmt.Println("Client Put test:1")

	client.Put("test", "1")
	fmt.Println("Client Get test")
	s, err := client.Get("test")
	if err != nil {
		fmt.Println("wrong get")
		fmt.Println(err)
	}
	fmt.Println(s)

	fmt.Println("Client Acquire test")
	ts, err := client.Acquire("test")
	fmt.Println("Client Acquire lock " + ts)
	time.Sleep(15 * time.Second)
	fmt.Println("Client Release test")
	client.Release("test", ts)
}
