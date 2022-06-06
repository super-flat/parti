package node

import (
	"context"

	"github.com/super-flat/parti/gen/localpb"
	"github.com/troop-dev/go-kit/pkg/logging"
	"google.golang.org/grpc"
)

type ClusteringService struct {
	nodeWrapper *Node
}

func NewClusteringService(nw *Node) *ClusteringService {
	return &ClusteringService{nodeWrapper: nw}
}

// Ping method is for testing node liveness
func (c ClusteringService) Ping(ctx context.Context, request *localpb.PingRequest) (*localpb.PingResponse, error) {
	logging.Debugf("received ping %d", request.GetPartitionId())
	return c.nodeWrapper.Ping(ctx, request)
}

func (c ClusteringService) Stats(context.Context, *localpb.StatsRequest) (*localpb.StatsResponse, error) {
	resp := &localpb.StatsResponse{
		NodeId:          c.nodeWrapper.GetNodeID(),
		IsLeader:        c.nodeWrapper.IsLeader(),
		PartitionOwners: make(map[uint32]string, 0),
		PeerPorts:       make(map[string]uint32),
	}

	for partition, owner := range c.nodeWrapper.PartitionMappings() {
		resp.PartitionOwners[partition] = owner
	}

	for _, peer := range c.nodeWrapper.getPeers() {
		resp.PeerPorts[peer.ID] = uint32(peer.GrpcPort)
	}

	return resp, nil
}

func (c ClusteringService) Send(ctx context.Context, request *localpb.SendRequest) (*localpb.SendResponse, error) {
	logging.Debugf("received send, msgID=%s, partition=%d", request.GetMessageId(), request.GetPartitionId())
	return c.nodeWrapper.Send(ctx, request)
}

func (c *ClusteringService) RegisterService(server *grpc.Server) {
	localpb.RegisterClusteringServer(server, c)
}

// ensure implements complete interface
var _ localpb.ClusteringServer = ClusteringService{}
