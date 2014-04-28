package rpcwrapper

//Borrow a lot of ideas and some code piece from p1 lspnet package

import (
	"errors"
	"log"
	"math/rand"
	"net/rpc"
	//"os"
	"io/ioutil"
	"sync/atomic"
	"time"
)

var (
	forwardDropRate  uint32 = 0    //The drop rate of sending request
	backwardDropRate uint32 = 0    //The drop rate of recieving request
	callDelayTime    uint32 = 1100 //The delay time of the rpc call, this num should be no less than 1000 (1s)
	callDelayRate    uint32 = 0    //The delay possibility
	//LOGV                    = log.New(os.Stdout, "VERBOSE", log.Lmicroseconds)
	LOGV = log.New(ioutil.Discard, "VERBOSE", log.Lmicroseconds)
)

type Client struct {
	c *rpc.Client
}

func DialHTTP(t, hostport string) (*Client, error) {
	if dropIt(forwardDropRate) {
		LOGV.Println("Drop request!")
		return nil, errors.New("Network error")
	}

	c := new(Client)
	cli, err := rpc.DialHTTP(t, hostport)
	c.c = cli
	return c, err
}

func (c *Client) Go(name string, args interface{}, reply interface{}, done chan *rpc.Call) *rpc.Call {
	var rv *rpc.Call = nil
	if delayIt(callDelayRate) {
		LOGV.Println("Delay request!")
		//it will always timeout in the paxos layer, so we just return a nil channel
		rv = new(rpc.Call)
		go func() {
			time.Sleep(time.Duration(callDelayTime) * time.Millisecond) //delay the call
			c.c.Call(name, args, reply)
		}()
		return rv
	} else {
		rv = c.c.Go(name, args, reply, done)
		if dropIt(backwardDropRate) {
			rv = new(rpc.Call)
			LOGV.Println("Drop reply!")
		}
		return rv
	}
}

func (c *Client) Close() {
	c.c.Close()
}

//Set forwardDropRate
func SetForwardDropRate(p int) {
	if 0 <= p && p <= 100 {
		atomic.StoreUint32(&forwardDropRate, uint32(p))
	}
}

func SetBackwardDropRate(p int) {
	if 0 <= p && p <= 100 {
		atomic.StoreUint32(&backwardDropRate, uint32(p))
	}
}

func ResetDropRate() {
	SetForwardDropRate(0)
	SetBackwardDropRate(0)
}

func dropIt(dropRate uint32) bool {
	return uint32(rand.Intn(100)) < dropRate
}

func delayIt(delayRate uint32) bool {
	return uint32(rand.Intn(100)) < delayRate
}

func SetCallDelayTime(delayTime int) {
	if delayTime > 1000 {
		atomic.StoreUint32(&callDelayTime, uint32(delayTime))
	}
}

func SetCallDelayRate(p int) {
	if 0 <= p && p <= 100 {
		atomic.StoreUint32(&callDelayRate, uint32(p))
	}
}
