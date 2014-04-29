package main

import (
	"fmt"
	"github.com/gobby/src/chubbyclient"
"time"
)

func main() {
	//TODO:need modify NewClient parameter, now is the numNodes
	client, err := chubbyclient.NewClient(0, 1)
	if err != nil {
		fmt.Println("wrong")
	}
	time.Sleep(3)
	fmt.Println("Client2 Aquire test")
	ts, err := client.Acquire("test")
	for err != nil {
		time.Sleep(time.Second)
		fmt.Println("Client2 Aquire test")
		ts, err = client.Acquire("test")
	}
	fmt.Println("Client2 Aquires lock " + ts)
	time.Sleep(3)
	fmt.Println("Client2 Releases lock " + ts)
	client.Release("test", ts)
}
