package command

type commandType int

const (
    Put commandType = iota + 1
    Get
    Acquire
    Release
)

type Command struct {
	Key string
    Value string
    Type commandType
}
