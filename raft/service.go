package raft

import (
	"context"

	"github.com/super-flat/parti/logging"

	partipb "github.com/super-flat/parti/gen/parti"
	"google.golang.org/grpc"
)

// Service implements the partiv1.RaftService
type Service struct {
	Node *Node
}

func NewClientGrpcService(node *Node) *Service {
	return &Service{
		Node: node,
	}
}

// RegisterService registers the gRPC service
func (s *Service) RegisterService(sv *grpc.Server) {
	partipb.RegisterRaftServiceServer(sv, s)
}

func (s *Service) ApplyLog(ctx context.Context, request *partipb.ApplyLogRequest) (*partipb.ApplyLogResponse, error) {
	// get logger from context
	logger := logging.WithContext(ctx)
	// add some debug logging
	logger.Debug().Msg("applying raft log")
	// apply the raft log
	result := s.Node.Raft.Apply(request.GetRequest(), 0)
	// handle the error
	if result.Error() != nil {
		return nil, result.Error()
	}
	// serialize the response
	respPayload, err := s.Node.Serializer.Serialize(result.Response())
	// handle the error
	if err != nil {
		return nil, err
	}
	// return the response
	return &partipb.ApplyLogResponse{Response: respPayload}, nil
}

func (s *Service) GetDetails(ctx context.Context, request *partipb.GetDetailsRequest) (*partipb.GetDetailsResponse, error) {
	return &partipb.GetDetailsResponse{
		ServerId:      s.Node.ID,
		DiscoveryPort: int32(s.Node.DiscoveryPort),
	}, nil
}
