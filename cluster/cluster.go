package cluster

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	hraft "github.com/hashicorp/raft"
	"github.com/super-flat/parti/cluster/raftwrapper"
	"github.com/super-flat/parti/cluster/raftwrapper/discovery"
	"github.com/super-flat/parti/cluster/raftwrapper/fsm"
	"github.com/super-flat/parti/cluster/raftwrapper/serializer"
	"github.com/super-flat/parti/cluster/rebalance"
	partipb "github.com/super-flat/parti/pb/parti/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const (
	partitionsGroupName = "partitions"
	portsGroupName      = "ports"
)

type Cluster struct {
	partitionCount uint32

	node     *raftwrapper.Node
	nodeData *fsm.ProtoFsm

	mtx       *sync.RWMutex
	isStarted bool

	grpcServer *grpc.Server

	peerObservations <-chan hraft.Observation

	msgHandler Handler
}

func NewCluster(raftPort uint16, discoveryPort uint16, msgHandler Handler, partitionCount uint32) *Cluster {
	// raft fsm
	raftFsm := fsm.NewProtoFsm()

	// select discovery method
	// TODO: make configurable (k8s, docker, etc)
	discoveryService := discovery.NewMDNSDiscovery(int(raftPort))
	// discoveryService := discovery.NewHashicorpDiscovery(int(raftPort))

	ser := serializer.NewProtoSerializer()

	// instantiate the raft node
	node, err := raftwrapper.NewNode(
		int(raftPort),
		int(discoveryPort),
		raftFsm,
		ser,
		discoveryService,
	)
	if err != nil {
		panic(err)
	}

	return &Cluster{
		node:           node,
		nodeData:       raftFsm,
		mtx:            &sync.RWMutex{},
		isStarted:      false,
		msgHandler:     msgHandler,
		partitionCount: partitionCount,
	}
}

// Stop shuts down this node
func (n *Cluster) Stop(context.Context) {
	n.mtx.Lock()
	defer n.mtx.Unlock()
	if n.isStarted {
		log.Println("Shutting down node")
		if n.grpcServer != nil {
			n.grpcServer.GracefulStop()
		}
		go n.node.Stop()
		// waits for easy raft shutdown
		// TODO: sometimes this never receives, so disabling this for now
		n.node.AwaitShutdown()
		log.Println("Completed node shutdown")
	}
	n.isStarted = false
}

// Start the cluster node
func (n *Cluster) Start(context.Context) error {
	// acquire lock to ensure node is only started once
	n.mtx.Lock()
	defer n.mtx.Unlock()
	if n.isStarted {
		return nil
	}
	// register the clustering gRPC service on the node's grpc server
	// so that they share a single gRPC port
	clusteringServer := NewClusteringService(n)
	partipb.RegisterClusteringServer(n.node.GrpcServer, clusteringServer)
	// start the underlying raft node
	_, err := n.node.Start()
	if err != nil {
		return err
	}
	// handle peer observations
	n.registerPeerObserver()
	go n.handlePeerObservations()
	// do leader things
	go n.leaderRebalance()
	// complete startup
	n.isStarted = true
	return nil
}

// registerPeerObserver adds an `observer` to the raft cluster that receives
// any PeerObservation changes and forwards them to a channel
func (n *Cluster) registerPeerObserver() {
	if n.peerObservations != nil {
		return
	}
	observerCh := make(chan hraft.Observation, 100)
	filterfn := func(o *hraft.Observation) bool {
		switch o.Data.(type) {
		case hraft.PeerObservation:
			return true
		default:
			return false
		}
	}
	observer := hraft.NewObserver(observerCh, true, hraft.FilterFn(filterfn))
	n.node.Raft.RegisterObserver(observer)
	n.peerObservations = observerCh
}

// handlePeerObservations handles inbound peer observations from raft
func (n *Cluster) handlePeerObservations() {
	for observation := range n.peerObservations {
		peerObservation, ok := observation.Data.(hraft.PeerObservation)
		if ok && n.node.IsLeader() {
			if peerObservation.Removed {
				// remove from the list of ports
				if err := raftwrapper.RaftApplyDelete(n.node, portsGroupName, string(peerObservation.Peer.ID)); err != nil {
					// TODO whether to exit the system
					log.Println(err.Error())
				}
				// remove from the list of partitions
				if err := raftwrapper.RaftApplyDelete(n.node, partitionsGroupName, string(peerObservation.Peer.ID)); err != nil {
					// TODO whether to exit the system
					log.Println(err.Error())
				}
			}
		}
	}
}

// leaderRebalance allows the leader node to delegate partitions to its
// cluster peers, considering nodes that have left and new nodes that
// have joined, with a goal of evenly distributing the work.
func (n *Cluster) leaderRebalance() {
	for {
		time.Sleep(time.Second * 3)
		if n.node.IsLeader() {
			// get current partitions
			currentPartitions := make(map[uint32]string, n.partitionCount)
			for partition := uint32(0); partition < n.partitionCount; partition++ {
				owner, err := n.getPartitionNode(partition)
				if err != nil {
					log.Printf("failed to get owner, partition=%d, %v", partition, err)
				}
				if owner != "" {
					currentPartitions[partition] = owner
				}
			}
			// get active peers
			peerMap := map[string]*raftwrapper.Peer{}
			activePeerIDs := make([]string, 0)
			for _, peer := range n.node.GetPeers() {
				if peer.IsReady() {
					peerMap[peer.ID] = peer
					activePeerIDs = append(activePeerIDs, peer.ID)
				}
			}
			// compute rebalance
			rebalancedOutput := rebalance.ComputeRebalance(n.partitionCount, currentPartitions, activePeerIDs)
			// apply any rebalance changes to the cluster
			for partitionID, newPeerID := range rebalancedOutput {
				currentPeerID, isMapped := currentPartitions[partitionID]
				if !isMapped || currentPeerID != newPeerID {
					if err := n.setPartition(partitionID, newPeerID); err != nil {
						// TODO decide whether to panic or not
						log.Println(err.Error())
					}
				}
			}
		}
	}
}

// getPartitionNode returns the node that owns a partition
func (n *Cluster) getPartitionNode(partitionID uint32) (string, error) {
	key := strconv.FormatUint(uint64(partitionID), 10)
	val, err := raftGetLocally[*wrapperspb.StringValue](n, partitionsGroupName, key)
	return val.GetValue(), err
}

// setPartition assigns a partition to a node
func (n *Cluster) setPartition(partitionID uint32, nodeID string) error {
	log.Printf("assigning partition (%d) to node (%s)", partitionID, nodeID)
	key := strconv.FormatUint(uint64(partitionID), 10)
	value := wrapperspb.String(nodeID)
	return raftwrapper.RaftApplyPut(n.node, partitionsGroupName, key, value)
}

// PartitionMappings returns a map of partition to node ID
func (n *Cluster) PartitionMappings() map[uint32]string {
	output := map[uint32]string{}
	var partition uint32
	for partition = 0; partition < n.partitionCount; partition++ {
		owner, err := n.getPartitionNode(partition)
		if err == nil && owner != "" {
			output[partition] = owner
		}
	}
	return output
}

// Send a message to the node that owns a partition
func (n *Cluster) Send(ctx context.Context, request *partipb.SendRequest) (*partipb.SendResponse, error) {
	partitionID := request.GetPartitionId()
	ownerNodeID, err := n.getPartitionNode(partitionID)
	if err != nil {
		return nil, err
	}
	// if partition owned by this node, answer locally
	if ownerNodeID == n.node.ID {
		log.Printf("received local send, partition=%d, id=%s", partitionID, request.GetMessageId())
		handlerResp, err := n.msgHandler.Handle(ctx, partitionID, request.GetMessage())
		if err != nil {
			return nil, err
		}
		resp := &partipb.SendResponse{
			NodeId:      n.node.ID,
			PartitionId: request.GetPartitionId(),
			MessageId:   request.GetMessageId(),
			Response:    handlerResp,
		}
		return resp, nil
	}
	peer, err := n.node.GetPeer(ownerNodeID)
	if err != nil {
		return nil, err
	}
	if !peer.IsReady() {
		return nil, errors.New("peer not ready for messages")
	}
	log.Printf("forwarding send, node=%s, messageID=%s, partition=%d", peer.ID, request.GetMessageId(), partitionID)
	return getClient(ctx, peer).Send(ctx, request)
}

// Ping a partition and receive a response from the node that owns it
func (n *Cluster) Ping(ctx context.Context, request *partipb.PingRequest) (*partipb.PingResponse, error) {
	partitionID := request.GetPartitionId()
	ownerNodeID, err := n.getPartitionNode(partitionID)
	if err != nil {
		return nil, err
	}
	if ownerNodeID == n.node.ID {
		log.Printf("received ping, answering locally, partition=%d", partitionID)
		resp := &partipb.PingResponse{
			NodeId: n.node.ID,
			Hops:   request.GetHops() + 1,
		}
		return resp, nil
	}
	peer, err := n.node.GetPeer(ownerNodeID)
	if err != nil {
		return nil, err
	}
	if !peer.IsReady() {
		return nil, errors.New("peer not ready for messages")
	}
	log.Printf("forwarding ping, node=%s, partition=%d", peer.ID, partitionID)
	resp, err := getClient(ctx, peer).Ping(ctx, &partipb.PingRequest{
		PartitionId: partitionID,
		Hops:        request.GetHops() + 1,
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func raftGetLocally[T any](n *Cluster, group string, key string) (T, error) {
	var output T
	value, err := n.nodeData.Get(group, key)
	if err != nil || value == nil {
		return output, err
	}
	outputTyped, ok := value.(T)
	if !ok {
		return output, fmt.Errorf("could not deserialize value '%v'", value)
	}
	return outputTyped, nil
}

func getClient(ctx context.Context, p *raftwrapper.Peer) partipb.ClusteringClient {
	// make the grpc client address
	grpcAddr := fmt.Sprintf("%s:%d", p.Host, p.RaftPort)
	// set up the grpc client connection
	conn, err := grpc.DialContext(ctx,
		grpcAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.EmptyDialOption{},
	)
	// handle the error of the connection
	if err != nil {
		// todo: don't panic here
		panic(err)
	}
	// create the client connection and return it
	client := partipb.NewClusteringClient(conn)
	return client
}