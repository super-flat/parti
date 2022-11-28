package raftwrapper

import (
	"context"
	"time"

	partipb "github.com/super-flat/parti/pb/parti/v1"
)

type RaftRPC struct {
	node *Node
}

var _ partipb.RaftServer = &RaftRPC{}

func NewRaftRPCServer(node *Node) *RaftRPC {
	return &RaftRPC{node: node}
}

func (r RaftRPC) GetPeerDetails(context.Context, *partipb.GetPeerDetailsRequest) (*partipb.GetPeerDetailsResponse, error) {
	return &partipb.GetPeerDetailsResponse{
		ServerId: r.node.ID,
	}, nil
}

func (r RaftRPC) ApplyLog(ctx context.Context, request *partipb.ApplyLogRequest) (*partipb.ApplyLogResponse, error) {
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
