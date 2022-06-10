package raft

import (
	"context"
	"time"

	partipb "github.com/super-flat/parti/partipb/parti/v1"
)

type RaftRpc struct {
	node *Node
}

func NewRaftRpcServer(node *Node) *RaftRpc {
	return &RaftRpc{node: node}
}

var _ partipb.RaftServer = &RaftRpc{}

func (r RaftRpc) GetPeerDetails(context.Context, *partipb.GetPeerDetailsRequest) (*partipb.GetPeerDetailsResponse, error) {
	return &partipb.GetPeerDetailsResponse{
		ServerId:      r.node.ID,
		DiscoveryPort: uint32(r.node.DiscoveryPort),
	}, nil
}

func (r RaftRpc) ApplyLog(ctx context.Context, request *partipb.ApplyLogRequest) (*partipb.ApplyLogResponse, error) {
	// TODO: pass this in?
	timeout := time.Second
	result := r.node.Raft.Apply(request.GetRequest(), timeout)
	if result.Error() != nil {
		return nil, result.Error()
	}
	respPayload, err := r.node.Serializer.Serialize(result.Response())
	if err != nil {
		return nil, err
	}
	return &partipb.ApplyLogResponse{Response: respPayload}, nil
}
