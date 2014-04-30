rm -f dumplog_*
go run node1/node.go &
go run node2/node.go &
go run node3/node.go &
sleep 5
go run client1/client.go &
sleep 2
go run client2/client.go &
go run client3/client.go &
