package raft

import (
	"context"

	partipb "github.com/super-flat/parti/pb/parti/v1"
)

type RPCServer struct {
	node *Node
}

var _ partipb.RaftServer = &RPCServer{}

func NewRPCServer(node *Node) *RPCServer {
	return &RPCServer{node: node}
}

func (r RPCServer) GetPeerDetails(ctx context.Context, req *partipb.GetPeerDetailsRequest) (*partipb.GetPeerDetailsResponse, error) {
	return &partipb.GetPeerDetailsResponse{
		ServerId: r.node.ID,
	}, nil
}

func (r RPCServer) Bootstrap(ctx context.Context, req *partipb.BootstrapRequest) (*partipb.BootstrapResponse, error) {
	resp := &partipb.BootstrapResponse{
		PeerId:    r.node.ID,
		InCluster: r.node.isBootstrapped,
	}
	return resp, nil
}
