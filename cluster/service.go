package cluster

import (
	"context"

	"github.com/super-flat/parti/log"

	partipb "github.com/super-flat/parti/pb/parti/v1"
)

type ClusteringService struct {
	partipb.UnimplementedClusteringServer
	cluster *Cluster
	logger  log.Logger
}

// ensure implements complete interface
var _ partipb.ClusteringServer = &ClusteringService{}

func NewClusteringService(cluster *Cluster) *ClusteringService {
	return &ClusteringService{cluster: cluster, logger: cluster.logger}
}

// Ping method is for testing node liveness
func (c *ClusteringService) Ping(ctx context.Context, request *partipb.PingRequest) (*partipb.PingResponse, error) {
	c.logger.Infof("received ping: %d", request.GetPartitionId())
	return c.cluster.Ping(ctx, request)
}

func (c *ClusteringService) Stats(context.Context, *partipb.StatsRequest) (*partipb.StatsResponse, error) {
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

func (c *ClusteringService) Send(ctx context.Context, request *partipb.SendRequest) (*partipb.SendResponse, error) {
	c.logger.Infof("received send, msgID=%s, partition=%d", request.GetMessageId(), request.GetPartitionId())
	return c.cluster.Send(ctx, request)
}

func (c *ClusteringService) StartPartition(ctx context.Context, request *partipb.StartPartitionRequest) (*partipb.StartPartitionResponse, error) {
	return c.cluster.StartPartition(ctx, request)
}

func (c *ClusteringService) ShutdownPartition(ctx context.Context, request *partipb.ShutdownPartitionRequest) (*partipb.ShutdownPartitionResponse, error) {
	return c.cluster.ShutdownPartition(ctx, request)
}
