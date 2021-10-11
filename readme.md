# raft-poc

TODO:
- define gRPC service for raft methods & implement them
    - https://github.com/Jille/raft-grpc-example/blob/master/main.go
    - https://github.com/Jille/raft-grpc-transport/blob/master/transport.go
- define a "shard lookup" state and distribute with raft
- make leader node assign shards on boot
- define/implement a gRPC service for messages that forward to the right node
    - requires a "cluster lookup" internal interface
- design/implement rebalancing
    - add a node
    - remove a node
    - (optional) based on traffic skew among shards
- design how new nodes discover siblings to join the cluster
    - eventually, this is an interface that we implement wrapping k8s API

```sh
# AUTOJOIN NODES
# node 1 (autojoins)
WEB_PORT=11001 \
NODE_ID=1 \
go run ./app run

# node 2 (autojoins)
WEB_PORT=11002 \
NODE_ID=2 \
go run ./app run

# node 3 (autojoins)
WEB_PORT=11003 \
NODE_ID=3 \
go run ./app run

# MANUALLY JOINING NODES

# first start the node
WEB_PORT=11004 \
NODE_ID=4 \
ADDR=localhost:63004 \
JOIN=true \
go run ./app run

# then instruct another joined node to let you in
curl -X POST localhost:11001/cluster/addnode/4/localhost:63004

# SET VALUES
curl -X POST localhost:11004/db/someKey/someValue

# GET VALUES
curl -X GET localhost:11004/db/someKey
```
