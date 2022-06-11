package cluster

import (
	"context"

	partipb "github.com/super-flat/parti/partipb/parti/v1"
	"github.com/troop-dev/go-kit/pkg/logging"
	"google.golang.org/grpc"
)

type ClusteringService struct {
	cluster *Cluster
}

func NewClusteringService(cluster *Cluster) *ClusteringService {
	return &ClusteringService{cluster: cluster}
}

// Ping method is for testing node liveness
func (c ClusteringService) Ping(ctx context.Context, request *partipb.PingRequest) (*partipb.PingResponse, error) {
	logging.Debugf("received ping %d", request.GetPartitionId())
	return c.cluster.Ping(ctx, request)
}

func (c ClusteringService) Stats(context.Context, *partipb.StatsRequest) (*partipb.StatsResponse, error) {
	resp := &partipb.StatsResponse{
		NodeId:          c.cluster.node.ID,
		IsLeader:        c.cluster.node.IsLeader(),
		PartitionOwners: make(map[uint32]string, 0),
		PeerPorts:       make(map[string]uint32),
	}

	for partition, owner := range c.cluster.PartitionMappings() {
		resp.PartitionOwners[partition] = owner
	}

	for _, peer := range c.cluster.node.GetPeers() {
		resp.PeerPorts[peer.ID] = uint32(peer.RaftPort)
	}

	return resp, nil
}

func (c ClusteringService) Send(ctx context.Context, request *partipb.SendRequest) (*partipb.SendResponse, error) {
	logging.Debugf("received send, msgID=%s, partition=%d", request.GetMessageId(), request.GetPartitionId())
	return c.cluster.Send(ctx, request)
}

func (c *ClusteringService) RegisterService(server *grpc.Server) {
	partipb.RegisterClusteringServer(server, c)
}

// ensure implements complete interface
var _ partipb.ClusteringServer = ClusteringService{}
