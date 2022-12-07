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

func (r RPCServer) GetPeerDetails(context.Context, *partipb.GetPeerDetailsRequest) (*partipb.GetPeerDetailsResponse, error) {
	return &partipb.GetPeerDetailsResponse{
		ServerId: r.node.ID,
	}, nil
}
