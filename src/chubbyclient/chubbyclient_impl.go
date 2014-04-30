package chubbyclient

import (
	"errors"
	"github.com/gobby/src/config"
	"github.com/gobby/src/rpc/chubbyrpc"
	"net/rpc"
	"strconv"
    "fmt"
)

type chubbyclient struct {
	masterHostPort string
	masterConn     *rpc.Client
    numNodes       int
}

func NewClient(numNodes int, idx int) (Chubbyclient, error) {
	client := &chubbyclient {
        masterHostPort: "",
        masterConn: nil,
        numNodes: numNodes,
    }
    client.findMaster()
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
                    fmt.Printf("client find that %d is master\n", i)
                    return
                }
            }
        }
    }
}
