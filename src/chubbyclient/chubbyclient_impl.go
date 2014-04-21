// TODO: need to deal with corner cases: master failure in several timings

package chubbyclient

import (
    "net/rpc"
    "errors"
    "github.com/gobby/src/config"
    "github.com/gobby/src/rpc/chubbyrpc"
)

type chubbyclient struct {
    masterHostPort string
    masterConn *rpc.Client
}

func NewClient() (Chubbyclient, error) {
    client := new(chubbyclient)
    for _, hostport := range config.Hostports {
        if conn, err := rpc.DialHTTP("tcp", hostport); err == nil {
            args := new(chubbyrpc.GetMasterArgs)
            reply := new(chubbyrpc.GetMasterReply)
            if err = conn.Call("Chubbyserver.GetMasterHostport", args, reply); err == nil {
                client.masterHostPort = reply.Hostport
                break
            }
        }
    }
    if conn, err := rpc.DialHTTP("tcp", client.masterHostPort); err == nil {
        client.masterConn = conn
    }
    return client, nil
}

func (client *chubbyclient) Put(key, value string) error {
    args := new(chubbyrpc.PutArgs)
    args.Key = key
    args.Value = value
    reply := new(chubbyrpc.PutReply)
    if err := client.masterConn.Call("Chubbyserver.Put", args, reply); err == nil {
        if reply.Status == chubbyrpc.OK {
            return nil
        } else {
            return errors.New("Put error")
        }
    } else {
        return err
    }
}

func (client *chubbyclient) Get(key string) (string, error) {
    args := new(chubbyrpc.GetArgs)
    args.Key = key
    reply := new(chubbyrpc.GetReply)
    if err := client.masterConn.Call("Chubbyserver.Get", args, reply); err == nil {
        if reply.Status == chubbyrpc.OK {
            return reply.Value, nil
        } else {
            return "", errors.New("Get error")
        }
    } else {
        return "", err
    }
}

func (client *chubbyclient) Aquire(key string) error {
    args := new(chubbyrpc.AquireArgs)
    args.Key = key
    reply := new(chubbyrpc.AquireReply)
    if err := client.masterConn.Call("Chubbyserver.Aquire", args, reply); err == nil {
        if reply.Status == chubbyrpc.OK {
            return nil
        } else {
            return errors.New("Aquire error")
        }
    } else {
        return err
    }
}

func (client *chubbyclient) Release(key string) error {
    args := new(chubbyrpc.ReleaseArgs)
    args.Key = key
    reply := new(chubbyrpc.ReleaseReply)
    if err := client.masterConn.Call("Chubbyserver.Release", args, reply); err == nil {
        if reply.Status == chubbyrpc.OK {
            return nil
        } else {
            return errors.New("Release error")
        }
    } else {
        return err
    }
}
