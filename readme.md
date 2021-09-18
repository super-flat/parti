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
