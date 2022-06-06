package node

import (
	"context"

	"github.com/super-flat/parti/gen/localpb"
	"github.com/troop-dev/go-kit/pkg/logging"
	"google.golang.org/grpc"
)

type ClusteringService struct {
	node *Node
}

func NewClusteringService(nw *Node) *ClusteringService {
	return &ClusteringService{node: nw}
}

// Ping method is for testing node liveness
func (c ClusteringService) Ping(ctx context.Context, request *localpb.PingRequest) (*localpb.PingResponse, error) {
	logging.Debugf("received ping %d", request.GetPartitionId())
	return c.node.Ping(ctx, request)
}

func (c ClusteringService) Stats(context.Context, *localpb.StatsRequest) (*localpb.StatsResponse, error) {
	resp := &localpb.StatsResponse{
		NodeId:          c.node.GetNodeID(),
		IsLeader:        c.node.IsLeader(),
		PartitionOwners: make(map[uint32]string, 0),
		PeerPorts:       make(map[string]uint32),
	}

	for partition, owner := range c.node.PartitionMappings() {
		resp.PartitionOwners[partition] = owner
	}

	for _, peer := range c.node.getPeers() {
		resp.PeerPorts[peer.ID] = uint32(peer.GrpcPort)
	}

	return resp, nil
}

func (c ClusteringService) Send(ctx context.Context, request *localpb.SendRequest) (*localpb.SendResponse, error) {
	logging.Debugf("received send, msgID=%s, partition=%d", request.GetMessageId(), request.GetPartitionId())
	return c.node.Send(ctx, request)
}

func (c ClusteringService) GetPeerDetails(context.Context, *localpb.GetPeerDetailsRequest) (*localpb.GetPeerDetailsResponse, error) {
	return &localpb.GetPeerDetailsResponse{
		ServerId:      c.node.GetNodeID(),
		DiscoveryPort: uint32(c.node.GetDiscoveryPort()),
	}, nil
}

func (c ClusteringService) ApplyLog(ctx context.Context, request *localpb.ApplyLogRequest) (*localpb.ApplyLogResponse, error) {
	result, err := c.node.node.RaftApply(request.GetRequest(), 0)
	if result.Error() != nil {
		return nil, result.Error()
	}
	respPayload, err := s.Node.Serializer.Serialize(result.Response())
	if err != nil {
		return nil, err
	}
	return &rgrpc.ApplyResponse{Response: respPayload}, nil
}

func (c *ClusteringService) RegisterService(server *grpc.Server) {
	localpb.RegisterClusteringServer(server, c)
}

// ensure implements complete interface
var _ localpb.ClusteringServer = ClusteringService{}
