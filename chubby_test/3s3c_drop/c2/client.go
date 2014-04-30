package main

import (
	"fmt"
	"github.com/gobby/src/chubbyclient"
	"strconv"
	"time"
"math/rand"
)

const (
	cid = 1
	key = "test"
)

func main() {
	client, err := chubbyclient.NewClient(-1, cid)
	if err != nil {
		fmt.Println("can't create chubby client")
		return
	}
	if cid == 0 {
		fmt.Printf("client %d Put %s:1", cid, key)
		client.Put(key, "1")
	}

	time.Sleep(3 * time.Second)

	for i := 0; i < 50; i++ {
		var lstm string
		var err error
		for {
			if lstm, err = client.Acquire(key); err != nil {
                 time.Sleep(time.Duration(rand.Intn(1000)) * time.Millisecond)
				continue
			}
			fmt.Printf("client %d gets the lock\n", cid)
			val, _ := client.Get(key)
			num, _ := strconv.Atoi(val)
			fmt.Printf("the value is now %d\n", num)
			newval := strconv.Itoa(num + 1)
			client.Put(key, newval)
			fmt.Printf("client %d put %s\n", cid, newval)
			if err = client.Release(key, lstm); err == nil {
				fmt.Printf("client %d releases the lock\n", cid)
				break
			} else {
				fmt.Printf("client %d fails to release the lock\n", cid)
				return
			}
		}
	}
}
