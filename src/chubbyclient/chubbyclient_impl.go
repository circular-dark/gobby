package chubbyclient

import (
	"errors"
	"fmt"
	"github.com/gobby/src/config"
	"github.com/gobby/src/rpc/chubbyrpc"
	"math/rand"
	"net"
	"net/rpc"
	"strconv"
    "fmt"
)

type chubbyclient struct {
	masterHostPort string
	masterConn     *rpc.Client
    numNodes       int
    nodeID         int
	Sock           *net.UDPConn
}

func NewClient(numNodes int, nodeID int) (Chubbyclient, error) {
	client := &chubbyclient {
        masterHostPort: "",
        masterConn: nil,
        numNodes: numNodes,
        nodeID: nodeID,
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

func (client *chubbyclient) Put(key, value string) error {
	args := new(chubbyrpc.PutArgs)
	args.Key = key
	args.Value = value
	reply := new(chubbyrpc.ChubbyReply)
    for client.masterConn == nil {
        client.findMaster()
    }
    curtry_args := new(chubbyrpc.CheckArgs)
    curtry_reply := new(chubbyrpc.ChubbyReply)
    if err := client.masterConn.Call("ChubbyServer.CheckMaster", curtry_args, curtry_reply); err != nil || curtry_reply.Status != chubbyrpc.OK {
        client.findMaster()
    }
	if err := client.masterConn.Call("ChubbyServer.Put", args, reply); err == nil {
		if reply.Status == chubbyrpc.OK {
			return nil
		} else {
			return errors.New("Put error")
		}
	} else {
        client.findMaster()
		return err
	}
}

func (client *chubbyclient) Get(key string) (string, error) {
	args := new(chubbyrpc.GetArgs)
	args.Key = key
	reply := new(chubbyrpc.ChubbyReply)
    for client.masterConn == nil {
        client.findMaster()
    }
    curtry_args := new(chubbyrpc.CheckArgs)
    curtry_reply := new(chubbyrpc.ChubbyReply)
    if err := client.masterConn.Call("ChubbyServer.CheckMaster", curtry_args, curtry_reply); err != nil || curtry_reply.Status != chubbyrpc.OK {
        client.findMaster()
    }
	if err := client.masterConn.Call("ChubbyServer.Get", args, reply); err == nil {
		if reply.Status == chubbyrpc.OK {
			return reply.Value, nil
		} else {
			return "", errors.New("Get error")
		}
	} else {
        client.findMaster()
		return "", err
	}
}

func (client *chubbyclient) Acquire(key string) (string, error) {
	args := new(chubbyrpc.AcquireArgs)
	args.Key = key
	reply := new(chubbyrpc.ChubbyReply)
    for client.masterConn == nil {
        client.findMaster()
    }
    curtry_args := new(chubbyrpc.CheckArgs)
    curtry_reply := new(chubbyrpc.ChubbyReply)
    if err := client.masterConn.Call("ChubbyServer.CheckMaster", curtry_args, curtry_reply); err != nil || curtry_reply.Status != chubbyrpc.OK {
        client.findMaster()
    }
	if err := client.masterConn.Call("ChubbyServer.Acquire", args, reply); err == nil {
		if reply.Status == chubbyrpc.OK {
			return reply.Value, nil
		} else {
			return reply.Value, errors.New("Acquire error")
		}
	} else {
        client.findMaster()
		return "", err
	}
}

func (client *chubbyclient) Release(key, lockstamp string) error {
	args := new(chubbyrpc.ReleaseArgs)
	args.Key = key
	args.Lockstamp = lockstamp
	reply := new(chubbyrpc.ChubbyReply)
    for client.masterConn == nil {
        client.findMaster()
    }
    curtry_args := new(chubbyrpc.CheckArgs)
    curtry_reply := new(chubbyrpc.ChubbyReply)
    if err := client.masterConn.Call("ChubbyServer.CheckMaster", curtry_args, curtry_reply); err != nil || curtry_reply.Status != chubbyrpc.OK {
        client.findMaster()
    }
	if err := client.masterConn.Call("ChubbyServer.Release", args, reply); err == nil {
		if reply.Status == chubbyrpc.OK {
			return nil
		} else {
			return errors.New("Release error")
		}
	} else {
        client.findMaster()
		return err
	}
}

func (client *chubbyclient) findMaster() {
    for i := 0; i < client.numNodes; i++ {
        curtry_hostport := config.Nodes[i].Address + ":" + strconv.Itoa(config.Nodes[i].Port)
        if curtry_conn, err := rpc.DialHTTP("tcp", curtry_hostport); err == nil {
            curtry_args := new(chubbyrpc.CheckArgs)
            curtry_reply := new(chubbyrpc.ChubbyReply)
            if err := curtry_conn.Call("ChubbyServer.CheckMaster", curtry_args, curtry_reply); err == nil {
                if curtry_reply.Status == chubbyrpc.OK {
                    client.masterHostPort = curtry_hostport
                    client.masterConn = curtry_conn
                    fmt.Printf("client %d finds that %d is master\n", client.nodeID, i)
                    return
                }
            }
        }
    }
}

func (client *chubbyclient) Watch(key string) error {
	args := new(chubbyrpc.WatchArgs)
	args.Key = key
	args.HostAddr = client.Sock.LocalAddr().String()
	reply := new(chubbyrpc.ChubbyReply)
	if err := client.masterConn.Call("ChubbyServer.Watch", args, reply); err == nil {
		if reply.Status == chubbyrpc.OK {
			return nil
		} else {
			return errors.New("Release error")
		}
	} else {
		return err
	}
}
