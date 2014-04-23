package config

type Node struct {
	Address string
    Port    int
	NodeID  int
}

var (
    NumNodes = 3
    Nodes = [3]Node{
        Node{"127.0.0.1", 54322, 0},
        Node{"127.0.0.1", 54323, 1},
        Node{"127.0.0.1", 54324, 2},
    }
)
