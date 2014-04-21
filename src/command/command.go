package command

type commandType int

const (
    Put commandType = iota + 1
    Get
    Acquire
    Release
    Bemaster
)

type Command struct {
	Key string
    Value string
    Type commandType
}

func (c Command) ToString() string {
    var s string
    switch (c.Type) {
    case Put:
        s = "Put"
    case Get:
        s = "Get"
    case Acquire:
        s = "Acquire"
    case Release:
        s = "Release"
    case Bemaster:
        s = "Bemaster"
    }
    s += " " + c.Key + " " + c.Value
    return s
}
