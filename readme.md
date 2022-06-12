# parti

[![build](https://github.com/super-flat/parti/actions/workflows/build.yml/badge.svg)](https://github.com/super-flat/parti/actions/workflows/main.yml)

Parti provides cluster sharding on top of hashicorp raft. Send a message for a given partition to any node in the cluster, and the message will be forwarded to the node that owns that partition. The cluster will automatically rebalance as nodes are added and removed.

### SAMPLE
```sh
# run node 1
make run1
# run node 2 (in another terminal)
make run2
# run node 3 (in another terminal)
make run3

# observe the partitions by asking node 1
go run ./client stats --addr 0.0.0.0:50101

# ping a partition from a given node
go run ./client ping --addr 0.0.0.0:50101 --partition 0
go run ./client ping --addr 0.0.0.0:50101 --partition 9

# send a message to various partitions and observe it forward
# curl 'localhost:50001/send?partition=9&message=msg2'
go run ./example send --partition 0 --addr 0.0.0.0:50001 hello world
# curl 'localhost:50001/send?partition=1&message=msg1'
go run ./example send --partition 9 --addr 0.0.0.0:50001 hello world
```

### TODO
- (optional) open long-lived streams between nodes to reduce number of connections between nodes (right now, each forwarded message gets its own gRPC connection)
