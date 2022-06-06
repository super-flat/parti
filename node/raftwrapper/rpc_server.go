package raftwrapper

import (
	"context"
	"time"

	"github.com/super-flat/parti/gen/localpb"
)

type RaftRpc struct {
	node *Node
}

func NewRaftRpcServer(node *Node) *RaftRpc {
	return &RaftRpc{node: node}
}

func (r RaftRpc) GetPeerDetails(context.Context, *localpb.GetPeerDetailsRequest) (*localpb.GetPeerDetailsResponse, error) {
	return &localpb.GetPeerDetailsResponse{
		ServerId:      r.node.ID,
		DiscoveryPort: uint32(r.node.DiscoveryPort),
	}, nil
}

func (r RaftRpc) ApplyLog(ctx context.Context, request *localpb.ApplyLogRequest) (*localpb.ApplyLogResponse, error) {
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
	return &localpb.ApplyLogResponse{Response: respPayload}, nil
}

var _ localpb.RaftServer = &RaftRpc{}
