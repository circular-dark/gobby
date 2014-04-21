package chubbyserver

import (
)

const (
    masterHostPort = "masterHostPort"
    clientReqPrefix = "%"
    lockPrefix = "#"
)

type chubbyserver struct {
    kvstore map[string]string
}
