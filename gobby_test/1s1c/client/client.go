package main

import (
	"fmt"
	"github.com/gobby/src/gobbyclient"
	"time"
)

func main() {
	client, err := gobbyclient.NewClient(-1, 0)
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
	fmt.Printf("Client Acquire test, get %s\n", ts)
	fmt.Println("Client Release test")
	client.Release("test", ts)
	client.Watch("test")

	time.Sleep(2 * time.Second)
	go client.Put("test", "hahaha")
	time.Sleep(2 * time.Second)
}
