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
	"github.com/ksrichard/easyraft/fsm"
	"github.com/ksrichard/easyraft/serializer"
	"github.com/troop-dev/go-kit/pkg/grpcclient"
	"github.com/troop-dev/go-kit/pkg/grpcserver"
	"github.com/troop-dev/go-kit/pkg/logging"

	"github.com/super-flat/parti/gen/localpb"
	"github.com/super-flat/parti/node/raftwrapper"
	"github.com/super-flat/parti/node/rebalance"
)

const (
	partitionsGroupName = "partitions"
	portsGroupName      = "ports"
)

type Node struct {
	partitionCount uint32

	node     *raftwrapper.Node
	nodeData *fsm.InMemoryMapService

	mtx       *sync.RWMutex
	isStarted bool

	stoppedCh chan interface{}

	grpcPort   uint16
	grpcServer grpcserver.Server

	peerObservations <-chan hraft.Observation

	msgHandler Handler
}

func NewNode(raftPort uint16, grpcPort uint16, discoveryPort uint16, dataDir string, msgHandler Handler, partitionCount uint32) *Node {
	// EasyRaft Node
	fsmService := newInMemoryMapService()
	discoveryService := discovery.NewMDNSDiscovery()
	// discoveryService := discovery.NewStaticDiscovery([]string{})
	node, err := raftwrapper.NewNode(
		int(raftPort),
		int(discoveryPort),
		dataDir,
		[]fsm.FSMService{fsmService},
		serializer.NewMsgPackSerializer(),
		discoveryService,
	)
	if err != nil {
		panic(err)
	}

	return &Node{
		node:           node,
		nodeData:       fsmService,
		mtx:            &sync.RWMutex{},
		isStarted:      false,
		grpcPort:       grpcPort,
		msgHandler:     msgHandler,
		partitionCount: partitionCount,
	}
}

func (n *Node) GetNodeID() string {
	return n.node.ID
}

func (n *Node) Stop(ctx context.Context) {
	n.mtx.Lock()
	defer n.mtx.Unlock()
	if n.isStarted {
		log.Printf("Shutting down node")
		n.grpcServer.Stop(ctx)
		go n.node.Stop()
		// waits for easy raft shutdown
		// TODO: sometimes this never receives, so disabling this for now
		// <-n.stoppedCh
		// log.Printf("Completed node shutdown")
	}
	n.isStarted = false
}

func (n *Node) Start(ctx context.Context) error {
	// acquire lock to ensure node is only started once
	n.mtx.Lock()
	defer n.mtx.Unlock()
	if n.isStarted {
		return nil
	}
	stoppedCh, err := n.node.Start()
	if err != nil {
		return err
	}
	n.stoppedCh = stoppedCh

	// create the grpc server
	// grpc server
	clusteringServer := NewClusteringService(n)
	grpcServer, err := grpcserver.
		NewServerBuilder().
		WithReflection(false).
		// WithDefaultUnaryInterceptors().
		// WithDefaultStreamInterceptors().
		WithTracingEnabled(false).
		// WithTraceURL("").
		WithServiceName("easyraft").
		WithMetricsEnabled(false).
		// WithMetricsPort(0).
		WithPort(int(n.grpcPort)).
		WithService(clusteringServer).
		Build()

	if err != nil {
		log.Panic(fmt.Errorf("could not build server, %v", err))
	}
	n.grpcServer = grpcServer
	n.grpcServer.Start(ctx)

	n.registerPeerObserver()
	// handle peer observations
	go n.handlePeerObservations()
	// do node things
	go n.handleShareGrpcPort()
	// do leader things
	go n.leaderRebalance()
	n.isStarted = true
	return nil
}

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

// handleShareGrpcPort ensures the raft cluster knows this node's gRPC port
func (n *Node) handleShareGrpcPort() {
	for {
		time.Sleep(time.Second * 1)
		if n.HasLeader() {
			// first try to read port data locally
			port, err := RaftGetLocally[uint16](n, portsGroupName, n.GetNodeID())
			if err != nil || port == 0 {
				// if not found, get from leader
				port, err = RaftGetFromLeader[uint16](n, portsGroupName, n.GetNodeID())
				if err == nil && port == 0 {
					// if not found, send to leader
					err := RaftPutValue(n, portsGroupName, n.node.ID, uint16(n.grpcPort))
					if err != nil {
						logging.Errorf("failed to share gRPC port, %v", err)
					}
				}
			}
		}
	}
}

func (n *Node) handlePeerObservations() {
	for observation := range n.peerObservations {
		peerObservation, ok := observation.Data.(hraft.PeerObservation)
		if ok && n.IsLeader() {
			if peerObservation.Removed {
				// remove from the list of ports
				RaftDeleteValue(n, portsGroupName, string(peerObservation.Peer.ID))
				// remove from the list of partitions
				RaftDeleteValue(n, partitionsGroupName, string(peerObservation.Peer.ID))
			}
		}
	}
}

func (n *Node) leaderRebalance() {
	for {
		time.Sleep(time.Second * 3)
		if n.IsLeader() {
			// get current partitions
			currentPartitions := make(map[uint32]string, n.partitionCount)
			for partition := uint32(0); partition < n.partitionCount; partition++ {
				owner, err := n.getPartitionNode(uint32(partition))
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

			for partitionID, newPeerID := range rebalancedOutput {
				currentPeerID, isMapped := currentPartitions[partitionID]
				if !isMapped || currentPeerID != newPeerID {
					n.setPartition(partitionID, newPeerID)
				}
			}
		}
	}
}

func (n *Node) getPartitionNode(partitionID uint32) (string, error) {
	key := strconv.FormatUint(uint64(partitionID), 10)
	return RaftGetLocally[string](n, partitionsGroupName, key)
}

func (n *Node) setPartition(id uint32, nodeID string) error {
	fmt.Printf("assigning partition %d to node %s\n", id, nodeID)
	key := strconv.FormatUint(uint64(id), 10)
	return RaftPutValue(n, partitionsGroupName, key, nodeID)
}

func (n *Node) IsLeader() bool {
	return n.node.Raft.VerifyLeader().Error() == nil
}

func (n *Node) HasLeader() bool {
	return n.node.Raft.Leader() != ""
}

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

func (n *Node) getPeerSelf() *Peer {
	// todo: make this smarter for self
	raftAddr := fmt.Sprintf("0.0.0.0:%d", n.node.RaftPort)
	return NewPeer(n.node.ID, raftAddr, n.grpcPort)
}

// getPeers returns all nodes in raft cluster (including self)
func (n *Node) getPeers() []*Peer {
	peers := make(map[string]*Peer, 3)

	if cfg := n.node.Raft.GetConfiguration(); cfg.Error() == nil {
		for _, server := range cfg.Configuration().Servers {
			peerID := string(server.ID)
			grpcPort, _ := RaftGetLocally[uint16](n, portsGroupName, peerID)
			peer := NewPeer(string(server.ID), string(server.Address), grpcPort)
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
	grpcPort, err := RaftGetLocally[uint16](n, portsGroupName, peerID)
	if err != nil {
		return nil, err
	}
	return NewPeer(peerID, raftAddr, grpcPort), nil
}

func RaftDeleteValue(n *Node, group string, key string) error {
	request := fsm.MapRemoveRequest{MapName: group, Key: key}
	_, err := n.node.RaftApply(request, time.Second)
	return err
}

func RaftPutValue(n *Node, group string, key string, value interface{}) error {
	request := fsm.MapPutRequest{MapName: group, Key: key, Value: value}
	_, err := n.node.RaftApply(request, time.Second)
	return err
}

func RaftGetFromLeader[T any](n *Node, group, key string) (T, error) {
	result, err := n.node.RaftApply(
		fsm.MapGetRequest{MapName: group, Key: key},
		time.Second,
	)
	var output T
	if err != nil || result == nil {
		return output, err
	}
	resultTyped, ok := result.(T)
	if !ok {
		return output, fmt.Errorf("bad value type, '%v'", resultTyped)
	}
	return resultTyped, nil
}

func RaftGetLocally[T any](n *Node, group string, key string) (T, error) {
	var output T
	value := n.nodeData.Get(group, key)
	if value == nil {
		return output, nil
	}
	outputTyped, ok := value.(T)
	if !ok {
		return output, fmt.Errorf("could not deserialize value '%v'", value)
	}
	return outputTyped, nil
}

func raftGetIntLocally(n *Node, group string, key string) (int, error) {
	value := n.nodeData.Get(group, key)
	if value == nil {
		return 0, nil
	}
	outputTyped, ok := value.(int)
	if !ok {
		return 0, fmt.Errorf("could not deserialize value '%v'", value)
	}
	return outputTyped, nil
}

func raftGetStringLocally(n *Node, group string, key string) (string, error) {
	value := n.nodeData.Get(group, key)
	if value == nil {
		return "", nil
	}
	outputTyped, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("could not deserialize value '%v'", value)
	}
	return outputTyped, nil
}

// newInMemoryMapService copies the built in constructor but explicitly
// returns the struct instead of the interface...
func newInMemoryMapService() *fsm.InMemoryMapService {
	return &fsm.InMemoryMapService{Maps: map[string]*fsm.Map{}}
}

type Peer struct {
	ID       string
	Host     string
	RaftPort uint16
	GrpcPort uint16
}

func NewPeer(id string, raftAddr string, grpcPort uint16) *Peer {

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
		GrpcPort: grpcPort,
	}
}

func (p Peer) IsReady() bool {
	return p.GrpcPort != 0
}

func (p Peer) GetClient(ctx context.Context) localpb.ClusteringClient {
	grpcAddr := fmt.Sprintf("%s:%d", p.Host, p.GrpcPort)
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
