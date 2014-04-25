package command

import (
    "strconv"
)

type commandType int

const (
    Put commandType = iota
    Get
    Acquire
    Release
    NOP
)

type Command struct {
	Key string
    Value string
    Type commandType
    ID int
}

func (c Command) ToString() string {
    var s string
    switch (c.Type) {
    case Put:
        s = "[Put"
    case Get:
        s = "[Get"
    case Acquire:
        s = "[Acquire"
    case Release:
        s = "[Release"
    case NOP:
        s = "[NOP"
    }
    s += ", ID " + strconv.Itoa(c.ID) + ", " + c.Key + ", " + c.Value + "]"
    return s
}
