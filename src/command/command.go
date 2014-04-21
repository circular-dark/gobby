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
