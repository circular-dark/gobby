package main

import (
	"fmt"
	"github.com/gobby/src/gobbyclient"
	"time"
)

func main() {
	//TODO:need modify NewClient parameter, now is the numNodes
	client, err := gobbyclient.NewClient(1, 1)
	if err != nil {
		fmt.Println("wrong")
		return
	}
	fmt.Println("Client2 Aquire test")
	ts, err := client.Acquire("test")
	for err != nil {
		time.Sleep(time.Second)
		fmt.Println("Client2 Aquire test")
		ts, err = client.Acquire("test")
	}
	fmt.Println("Client2 Aquires lock " + ts)
	time.Sleep(10 * time.Second)
	fmt.Println("Client2 Releases lock " + ts)
	client.Release("test", ts)
}
