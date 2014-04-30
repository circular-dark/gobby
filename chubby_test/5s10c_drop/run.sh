rm -f dumplog_*
go run node1/node.go &
go run node2/node.go &
go run node3/node.go &
go run node4/node.go &
go run node5/node.go &
sleep 3
go run c1/client.go &
go run c2/client.go &
go run c3/client.go &
go run c4/client.go &
go run c5/client.go &
go run c6/client.go &
go run c7/client.go &
go run c8/client.go &
go run c9/client.go &
go run c10/client.go &
