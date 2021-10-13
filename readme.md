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
RAFT_ID=1 \
RAFT_PORT=63001 \
API_PORT=11001 \
PEERS="localhost:11001" \
go run ./app run

# node 2 (autojoins)
RAFT_ID=2 \
RAFT_PORT=63002 \
API_PORT=11002 \
PEERS="localhost:11001,localhost:11002" \
go run ./app run

# node 3 (autojoins)
RAFT_ID=3 \
RAFT_PORT=63003 \
API_PORT=11003 \
PEERS="localhost:11002,localhost:11003" \
go run ./app run

# SET VALUES
curl -X POST :11001/db/someKey/someValue

# GET VALUES
curl -X GET localhost:11002/db/someKey
```
