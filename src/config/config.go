package config

type Node struct {
	Address string
	Port    int
}

var (
	Nodes = [7]Node{
		Node{"127.0.0.1", 54322},
		Node{"127.0.0.1", 54323},
		Node{"127.0.0.1", 54324},
		Node{"127.0.0.1", 54325},
		Node{"127.0.0.1", 54326},
		Node{"127.0.0.1", 54327},
		Node{"127.0.0.1", 54328},
	}
)
