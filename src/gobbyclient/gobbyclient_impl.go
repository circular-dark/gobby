package gobbyclient

import (
	"errors"
	"fmt"
	"github.com/gobby/src/config"
	"github.com/gobby/src/rpc/gobbyrpc"
	"net"
	"net/rpc"
	"strconv"
)

type gobbyclient struct {
	masterHostPort string
	masterConn     *rpc.Client
	numNodes       int
	nodeID         int
	Sock           *net.UDPConn
}

func NewClient(numNodes int, nodeID int) (Gobbyclient, error) {
	client := &gobbyclient{
		masterHostPort: "",
		masterConn:     nil,
		numNodes:       numNodes,
		nodeID:         nodeID,
	}
	client.findMaster()

	for _, n := range config.Clients {
		addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", n.Port))
		sock, err := net.ListenUDP("udp", addr)
		if err == nil {
			client.Sock = sock
			break
		}
	}
	go func(c *net.UDPConn) {
		var buf []byte = make([]byte, 1500)
		for {
			blen, err := c.Read(buf)
			if err == nil {
				fmt.Println(string(buf[:blen]))
			} else {
				fmt.Println(err)
			}
		}
	}(client.Sock)

	return client, nil
}

func (client *gobbyclient) Put(key, value string) error {
	args := new(gobbyrpc.PutArgs)
	args.Key = key
	args.Value = value
	reply := new(gobbyrpc.GobbyReply)
	for client.masterConn == nil {
		client.findMaster()
	}
	curtry_args := new(gobbyrpc.CheckArgs)
	curtry_reply := new(gobbyrpc.GobbyReply)
	if err := client.masterConn.Call("gobbyServer.CheckMaster", curtry_args, curtry_reply); err != nil || curtry_reply.Status != gobbyrpc.OK {
		client.findMaster()
	}
	if err := client.masterConn.Call("gobbyServer.Put", args, reply); err == nil {
		if reply.Status == gobbyrpc.OK {
			return nil
		} else {
			return errors.New("Put error")
		}
	} else {
		client.findMaster()
		return err
	}
}

func (client *gobbyclient) Get(key string) (string, error) {
	args := new(gobbyrpc.GetArgs)
	args.Key = key
	reply := new(gobbyrpc.GobbyReply)
	for client.masterConn == nil {
		client.findMaster()
	}
	curtry_args := new(gobbyrpc.CheckArgs)
	curtry_reply := new(gobbyrpc.GobbyReply)
	if err := client.masterConn.Call("gobbyServer.CheckMaster", curtry_args, curtry_reply); err != nil || curtry_reply.Status != gobbyrpc.OK {
		client.findMaster()
	}
	if err := client.masterConn.Call("gobbyServer.Get", args, reply); err == nil {
		if reply.Status == gobbyrpc.OK {
			return reply.Value, nil
		} else {
			return "", errors.New("Get error")
		}
	} else {
		client.findMaster()
		return "", err
	}
}

func (client *gobbyclient) Acquire(key string) (string, error) {
	args := new(gobbyrpc.AcquireArgs)
	args.Key = key
	reply := new(gobbyrpc.GobbyReply)
	for client.masterConn == nil {
		client.findMaster()
	}
	curtry_args := new(gobbyrpc.CheckArgs)
	curtry_reply := new(gobbyrpc.GobbyReply)
	if err := client.masterConn.Call("gobbyServer.CheckMaster", curtry_args, curtry_reply); err != nil || curtry_reply.Status != gobbyrpc.OK {
		client.findMaster()
	}
	if err := client.masterConn.Call("gobbyServer.Acquire", args, reply); err == nil {
		if reply.Status == gobbyrpc.OK {
			return reply.Value, nil
		} else {
			return reply.Value, errors.New("Acquire error")
		}
	} else {
		client.findMaster()
		return "", err
	}
}

func (client *gobbyclient) Release(key, lockstamp string) error {
	args := new(gobbyrpc.ReleaseArgs)
	args.Key = key
	args.Lockstamp = lockstamp
	reply := new(gobbyrpc.GobbyReply)
	for client.masterConn == nil {
		client.findMaster()
	}
	curtry_args := new(gobbyrpc.CheckArgs)
	curtry_reply := new(gobbyrpc.GobbyReply)
	if err := client.masterConn.Call("gobbyServer.CheckMaster", curtry_args, curtry_reply); err != nil || curtry_reply.Status != gobbyrpc.OK {
		client.findMaster()
	}
	if err := client.masterConn.Call("gobbyServer.Release", args, reply); err == nil {
		if reply.Status == gobbyrpc.OK {
			return nil
		} else {
			return errors.New("Release error")
		}
	} else {
		client.findMaster()
		return err
	}
}

func (client *gobbyclient) findMaster() {
	for i := 0; i < client.numNodes; i++ {
		curtry_hostport := config.Nodes[i].Address + ":" + strconv.Itoa(config.Nodes[i].Port)
		if curtry_conn, err := rpc.DialHTTP("tcp", curtry_hostport); err == nil {
			curtry_args := new(gobbyrpc.CheckArgs)
			curtry_reply := new(gobbyrpc.GobbyReply)
			if err := curtry_conn.Call("gobbyServer.CheckMaster", curtry_args, curtry_reply); err == nil {
				if curtry_reply.Status == gobbyrpc.OK {
					client.masterHostPort = curtry_hostport
					client.masterConn = curtry_conn
					fmt.Printf("client %d finds that %d is master\n", client.nodeID, i)
					return
				}
			}
		}
	}
}

func (client *gobbyclient) Watch(key string) error {
	args := new(gobbyrpc.WatchArgs)
	args.Key = key
	args.HostAddr = client.Sock.LocalAddr().String()
	reply := new(gobbyrpc.GobbyReply)
	if err := client.masterConn.Call("gobbyServer.Watch", args, reply); err == nil {
		if reply.Status == gobbyrpc.OK {
			return nil
		} else {
			return errors.New("Release error")
		}
	} else {
		return err
	}
}
