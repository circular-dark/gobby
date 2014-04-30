rm -f dumplog_*
go run node1/node.go &
go run node2/node.go &
go run node3/node.go &
sleep 3
go run c1/client.go &
go run c2/client.go &
go run c3/client.go &
