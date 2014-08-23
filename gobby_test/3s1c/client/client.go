package main

import (
	"fmt"
	"github.com/gobby/src/gobbyclient"
)

func main() {
	//TODO:need modify NewClient parameter, now is the numNodes
	client, err := gobbyclient.NewClient(-1, 1)
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
	fmt.Println("Client Aquire test")
	ts, err := client.Acquire("test")
	fmt.Println("Client Aquire lock " + ts)
	fmt.Println("Client Release test")
	client.Release("test", ts)
}
