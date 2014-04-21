package command

type Command struct {
	Object string
	Action string
	Args   string
}

func (c Command) ToString() string {
  return c.Object + " " + c.Action + " " + c.Args
}
