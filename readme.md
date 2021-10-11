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
# LEADER
NODE_ADDRESS="localhost:11000" \
RAFT_ADDRESS="localhost:12000" \
RAFT_ID="node-a" \
RAFT_DIR=".local/node-a" \
JOIN_ADDRESS="" \
go run ./app run

# Follower 1
NODE_ADDRESS="localhost:11001" \
RAFT_ADDRESS="localhost:12001" \
RAFT_ID="node-b" \
RAFT_DIR=".local/node-b" \
JOIN_ADDRESS="localhost:11000" \
go run ./app run

# Follower 2
NODE_ADDRESS="localhost:11002" \
RAFT_ADDRESS="localhost:12002" \
RAFT_ID="node-c" \
RAFT_DIR=".local/node-c" \
JOIN_ADDRESS="localhost:11000" \
go run ./app run
```

### Bootstrap algorithm
- node boots and retrieves list of all nodes
    - in k8s, this will be implemented as a Pod List given an app selector
    - in local dev, you can either supply this, or it can use DNS in docker
- node will record the unix time (ms) that it booted
- node will try to communicate with all known siblings
    - node will send the list of known nodes and which one it thinks is the oldest
    - that algorithm needs to be deterministic (age, then node ID alphabetical, etc)


phase 1
- LEADER: NODE_C
- NODE_C: 300

phase 2 (node C asks node b)
- LEADER: NODE_B
- NODE_B: 200
- NODE_C: 300

phase 3 (node C asks node a)
- LEADER: NODE_A
- NODE_A: 100
- NODE_B: 200
- NODE_C: 300


- LEADER: NODE_A
- NODE_A: 100
- NODE_B: 200
- NODE_C: 300

- NODE_D: 2000



```sh
# A
NODE_ADDRESS="localhost:11000" \
RAFT_ADDRESS="localhost:12000" \
RAFT_ID="node-a" \
RAFT_DIR=".local/node-a" \
PEER_ADDRESSES="localhost:11000,localhost:11001,localhost:11002" \
go run ./app run

NODE_ADDRESS="localhost:11001" \
RAFT_ADDRESS="localhost:12001" \
RAFT_ID="node-b" \
RAFT_DIR=".local/node-b" \
PEER_ADDRESSES="localhost:11000,localhost:11001,localhost:11002" \
go run ./app run

NODE_ADDRESS="localhost:11002" \
RAFT_ADDRESS="localhost:12002" \
RAFT_ID="node-a" \
RAFT_DIR=".local/node-c" \
PEER_ADDRESSES="localhost:11000,localhost:11001,localhost:11002" \
go run ./app run

```
