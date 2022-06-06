package node

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	hraft "github.com/hashicorp/raft"
	"github.com/ksrichard/easyraft/discovery"
	"github.com/troop-dev/go-kit/pkg/grpcclient"
	"github.com/troop-dev/go-kit/pkg/grpcserver"
	"github.com/troop-dev/go-kit/pkg/logging"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/super-flat/parti/gen/localpb"
	"github.com/super-flat/parti/node/raftwrapper"
	"github.com/super-flat/parti/node/raftwrapper/fsm"
	"github.com/super-flat/parti/node/rebalance"
)

const (
	partitionsGroupName = "partitions"
	portsGroupName      = "ports"
)

type Node struct {
	partitionCount uint32

	node     *raftwrapper.Node
	nodeData *fsm.ProtoFsm

	mtx       *sync.RWMutex
	isStarted bool

	stoppedCh chan interface{}

	grpcServer grpcserver.Server

	peerObservations <-chan hraft.Observation

	msgHandler Handler
}

func NewNode(raftPort uint16, discoveryPort uint16, msgHandler Handler, partitionCount uint32) *Node {
	// raft fsm
	raftFsm := fsm.NewProtoFsm()

	// select discovery method
	// TODO: make configurable (k8s, docker, etc)
	discoveryService := discovery.NewMDNSDiscovery()
	// discoveryService := discovery.NewStaticDiscovery([]string{})

	ser := raftwrapper.NewLegacySerializer()

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

	return &Node{
		node:           node,
		nodeData:       raftFsm,
		mtx:            &sync.RWMutex{},
		isStarted:      false,
		msgHandler:     msgHandler,
		partitionCount: partitionCount,
	}
}

// GetDiscoveryPort returns the internal raft discovery port
func (n *Node) GetDiscoveryPort() uint16 {
	return uint16(n.node.DiscoveryPort)
}

// GetNodeID returns the current node's ID
func (n *Node) GetNodeID() string {
	return n.node.ID
}

// Stop shuts down this node
func (n *Node) Stop(ctx context.Context) {
	n.mtx.Lock()
	defer n.mtx.Unlock()
	if n.isStarted {
		log.Printf("Shutting down node")
		if n.grpcServer != nil {
			n.grpcServer.Stop(ctx)
		}
		go n.node.Stop()
		// waits for easy raft shutdown
		// TODO: sometimes this never receives, so disabling this for now
		// <-n.stoppedCh
		// log.Printf("Completed node shutdown")
	}
	n.isStarted = false
}

// Start the cluster node
func (n *Node) Start(ctx context.Context) error {
	// acquire lock to ensure node is only started once
	n.mtx.Lock()
	defer n.mtx.Unlock()
	if n.isStarted {
		return nil
	}

	// TODO: I commented out the server builder so we could share the grpc server
	// from the raft node

	// create the grpc server
	// grpc server
	// clusteringServer := NewClusteringService(n)
	// grpcServer, err := grpcserver.
	// 	NewServerBuilder().
	// 	WithReflection(false).
	// 	// WithDefaultUnaryInterceptors().
	// 	// WithDefaultStreamInterceptors().
	// 	WithTracingEnabled(false).
	// 	// WithTraceURL("").
	// 	WithServiceName("easyraft").
	// 	WithMetricsEnabled(false).
	// 	// WithMetricsPort(0).
	// 	WithPort(int(n.grpcPort)).
	// 	WithService(clusteringServer).
	// 	Build()
	clusteringServer := NewClusteringService(n)
	localpb.RegisterClusteringServer(n.node.GrpcServer, clusteringServer)

	// if err != nil {
	// 	log.Panic(fmt.Errorf("could not build server, %v", err))
	// }
	// n.grpcServer = grpcServer
	// n.grpcServer.Start(ctx)

	stoppedCh, err := n.node.Start()
	if err != nil {
		return err
	}
	n.stoppedCh = stoppedCh

	// handle peer observations
	n.registerPeerObserver()
	go n.handlePeerObservations()
	// do node things
	// go n.handleShareGrpcPort()
	// do leader things
	go n.leaderRebalance()
	n.isStarted = true
	return nil
}

// registerPeerObserver adds an `observer` to the raft cluster that receives
// any PeerObservation changes and forwards them to a channel
func (n *Node) registerPeerObserver() {
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

// // handleShareGrpcPort ensures the raft cluster knows this node's gRPC port
// func (n *Node) handleShareGrpcPort() {
// 	for {
// 		time.Sleep(time.Second * 1)
// 		if n.HasLeader() {
// 			// try to read grpc port from leader
// 			port, err := n.getPeerPortFromLeader(n.GetNodeID())
// 			if err != nil || port != n.grpcPort {
// 				// if not found, send to leader
// 				if err := n.setPeerPortOnLeader(); err != nil {
// 					logging.Errorf("failed to share gRPC port, %v", err)
// 				}
// 			}
// 		}
// 	}
// }

// handlePeerObservations handles inbound peer observations from raft
func (n *Node) handlePeerObservations() {
	for observation := range n.peerObservations {
		peerObservation, ok := observation.Data.(hraft.PeerObservation)
		if ok && n.IsLeader() {
			if peerObservation.Removed {
				// remove from the list of ports
				raftDeleteValue(n, portsGroupName, string(peerObservation.Peer.ID))
				// remove from the list of partitions
				raftDeleteValue(n, partitionsGroupName, string(peerObservation.Peer.ID))
			}
		}
	}
}

// leaderRebalance allows the leader node to delegate partitions to its
// cluster peers, considering nodes that have left and new nodes that
// have joined, with a goal of evenly distributring the work.
func (n *Node) leaderRebalance() {
	for {
		time.Sleep(time.Second * 3)
		if n.IsLeader() {
			// get current partitions
			currentPartitions := make(map[uint32]string, n.partitionCount)
			for partition := uint32(0); partition < n.partitionCount; partition++ {
				owner, err := n.getPartitionNode(partition)
				if err != nil {
					log.Default().Printf("failed to get owner, partition=%d, %v", partition, err)
				}
				if owner != "" {
					currentPartitions[partition] = owner
				}
			}
			// get active peers
			peerMap := map[string]*Peer{}
			activePeerIDs := make([]string, 0)
			for _, peer := range n.getPeers() {
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
					n.setPartition(partitionID, newPeerID)
				}
			}
		}
	}
}

// getPeerPortFromLeader queries the leader for the given peer's grpc port
func (n *Node) getPeerPortFromLeader(peerID string) (uint16, error) {
	port, err := raftGetFromLeader[*wrapperspb.UInt32Value](n, portsGroupName, n.GetNodeID())
	return uint16(port.GetValue()), err
}

// getPeerPortLocally queries the local FSM store for a peer's gRPC port
func (n *Node) getPeerPortLocally(peerID string) (uint16, error) {
	val, err := raftGetLocally[*wrapperspb.UInt32Value](n, portsGroupName, peerID)
	return uint16(val.GetValue()), err
}

// // setPeerPortOnLeader sets current node's gRPC port on the leader
// func (n *Node) setPeerPortOnLeader() error {
// 	value := wrapperspb.UInt32(uint32(n.grpcPort))
// 	return raftPutValue(n, portsGroupName, n.GetNodeID(), value)
// }

// getPartitionNode returns the node that owns a partition
func (n *Node) getPartitionNode(partitionID uint32) (string, error) {
	key := strconv.FormatUint(uint64(partitionID), 10)
	val, err := raftGetLocally[*wrapperspb.StringValue](n, partitionsGroupName, key)
	return val.GetValue(), err
}

// setPartition assigns a partition to a node
func (n *Node) setPartition(partitionID uint32, nodeID string) error {
	fmt.Printf("assigning partition (%d) to node (%s)\n", partitionID, nodeID)
	key := strconv.FormatUint(uint64(partitionID), 10)
	value := wrapperspb.String(nodeID)
	return raftPutValue(n, partitionsGroupName, key, value)
}

// IsLeader returns true if the current node is the cluster leader
func (n *Node) IsLeader() bool {
	return n.node.IsLeader()
}

// HasLeader returns true if the current node is aware of a cluster leader
// including itself
func (n *Node) HasLeader() bool {
	return n.node.HasLeader()
}

// PartitionMappings returns a map of partition to node ID
func (n *Node) PartitionMappings() map[uint32]string {
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
func (n *Node) Send(ctx context.Context, request *localpb.SendRequest) (*localpb.SendResponse, error) {
	partitionID := request.GetPartitionId()
	ownerNodeID, err := n.getPartitionNode(partitionID)
	if err != nil {
		return nil, err
	}
	// if partition owned by this node, answer locally
	if ownerNodeID == n.GetNodeID() {
		logging.Debugf("received local send, partition=%d, id=%s", partitionID, request.GetMessageId())
		handlerResp, err := n.msgHandler.Handle(ctx, partitionID, request.GetMessage())
		if err != nil {
			return nil, err
		}
		resp := &localpb.SendResponse{
			NodeId:      n.GetNodeID(),
			PartitionId: request.GetPartitionId(),
			MessageId:   request.GetMessageId(),
			Response:    handlerResp,
		}
		return resp, nil
	}
	peer, err := n.getPeer(ownerNodeID)
	if err != nil {
		return nil, err
	}
	if !peer.IsReady() {
		return nil, errors.New("peer not ready for messages")
	}
	logging.Debugf("forwarding send, node=%s, messageID=%s, partition=%d", peer.ID, request.GetMessageId(), partitionID)
	return peer.GetClient(ctx).Send(ctx, request)
}

// Ping a partition and receive a response from the node that owns it
func (n *Node) Ping(ctx context.Context, request *localpb.PingRequest) (*localpb.PingResponse, error) {
	partitionID := request.GetPartitionId()
	ownerNodeID, err := n.getPartitionNode(partitionID)
	if err != nil {
		return nil, err
	}
	if ownerNodeID == n.node.ID {
		logging.Debugf("received ping, answering locally, partition=%d", partitionID)
		resp := &localpb.PingResponse{
			NodeId: n.node.ID,
			Hops:   request.GetHops() + 1,
		}
		return resp, nil
	}
	peer, err := n.getPeer(ownerNodeID)
	if err != nil {
		return nil, err
	}
	if !peer.IsReady() {
		return nil, errors.New("peer not ready for messages")
	}
	logging.Debugf("forwarding ping, node=%s, partition=%d", peer.ID, partitionID)
	resp, err := peer.GetClient(ctx).Ping(ctx, &localpb.PingRequest{
		PartitionId: partitionID,
		Hops:        request.GetHops() + 1,
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// getPeerSelf returns a Peer for the current node
func (n *Node) getPeerSelf() *Peer {
	// todo: make this smarter for self
	raftAddr := fmt.Sprintf("0.0.0.0:%d", n.node.RaftPort)
	return NewPeer(n.node.ID, raftAddr)
}

// getPeers returns all nodes in raft cluster (including self)
func (n *Node) getPeers() []*Peer {
	peers := make(map[string]*Peer, 3)

	if cfg := n.node.Raft.GetConfiguration(); cfg.Error() == nil {
		for _, server := range cfg.Configuration().Servers {
			peerID := string(server.ID)
			peer := NewPeer(peerID, string(server.Address))
			peers[peer.ID] = peer
		}
	}

	output := make([]*Peer, 0, len(peers))
	for _, peer := range peers {
		output = append(output, peer)
	}

	return output
}

// getPeer returns a specific peer given an ID
func (n *Node) getPeer(peerID string) (*Peer, error) {
	if peerID == n.node.ID {
		return n.getPeerSelf(), nil
	}
	cfg := n.node.Raft.GetConfiguration()
	if err := cfg.Error(); err != nil {
		return nil, err
	}
	raftAddr := ""
	for _, server := range cfg.Configuration().Servers {
		if string(server.ID) == peerID {
			raftAddr = string(server.Address)
			break
		}
	}
	if raftAddr == "" {
		return nil, errors.New("could not find raft peer")
	}
	return NewPeer(peerID, raftAddr), nil
}

func raftDeleteValue(n *Node, group string, key string) error {
	request := &localpb.FsmRemoveRequest{Group: group, Key: key}
	_, err := n.node.RaftApply(request, time.Second)
	return err
}

func raftPutValue(n *Node, group string, key string, value proto.Message) error {
	anyVal, err := anypb.New(value)
	if err != nil {
		return err
	}
	request := &localpb.FsmPutRequest{Group: group, Key: key, Value: anyVal}
	_, err = n.node.RaftApply(request, time.Second)
	return err
}

func raftGetFromLeader[T any](n *Node, group, key string) (T, error) {
	request := &localpb.FsmGetRequest{Group: group, Key: key}
	result, err := n.node.RaftApply(request, time.Second)
	var output T
	if err != nil || result == nil {
		return output, err
	}
	resultTyped, ok := result.(T)
	if !ok {
		return output, fmt.Errorf("bad value type, '%T' '%v'", result, result)
	}
	return resultTyped, nil
}

func raftGetLocally[T any](n *Node, group string, key string) (T, error) {
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

type Peer struct {
	ID       string
	Host     string
	RaftPort uint16
}

func NewPeer(id string, raftAddr string) *Peer {

	addrParts := strings.Split(raftAddr, ":")
	if len(addrParts) != 2 {
		panic(fmt.Errorf("cant parse raft addr '%s'", raftAddr))
	}
	host := addrParts[0]
	raftPort, _ := strconv.ParseUint(addrParts[1], 10, 16)

	return &Peer{
		ID:       id,
		Host:     host,
		RaftPort: uint16(raftPort),
	}
}

func (p Peer) IsReady() bool {
	return true
}

func (p Peer) GetClient(ctx context.Context) localpb.ClusteringClient {
	grpcAddr := fmt.Sprintf("%s:%d", p.Host, p.RaftPort)
	conn, err := grpcclient.NewBuilder().
		WithBlock().
		WithInsecure().
		GetConn(ctx, grpcAddr)

	if err != nil {
		// todo: don't panic here
		panic(err)
	}
	client := localpb.NewClusteringClient(conn)
	return client
}
