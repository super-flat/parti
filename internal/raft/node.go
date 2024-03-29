package raft

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"os"
	"sync"
	"time"

	transport "github.com/Jille/raft-grpc-transport"
	"github.com/hashicorp/raft"
	hraft "github.com/hashicorp/raft"
	"github.com/pkg/errors"
	"github.com/super-flat/parti/internal/raft/serializer"
	partilog "github.com/super-flat/parti/log"
	partipb "github.com/super-flat/parti/pb/parti/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

const nodeIDCharacters = "abcdefghijklmnopqrstuvwxyz"

// Node encapsulate information about a cluster Node
type Node struct {
	ID               string
	RaftPort         int
	address          string
	Raft             *hraft.Raft
	GrpcServer       *grpc.Server
	TransportManager *transport.Manager
	Serializer       serializer.Serializer
	stopped          *uint32
	logger           partilog.Logger
	logLevel         partilog.Level
	mtx              *sync.Mutex
	isStarted        bool
	isBootstrapped   bool
}

// NewNode returns an raft Node
func NewNode(nodeID string, raftPort int, raftFsm hraft.FSM, serializer serializer.Serializer, logLevel partilog.Level, logger partilog.Logger, grpcServer *grpc.Server) (*Node, error) {
	// default raft config
	addr := fmt.Sprintf("%s:%d", "0.0.0.0", raftPort)

	// create the raft logger
	raftLogger := newLog(logLevel, logger)

	raftConf := hraft.DefaultConfig()
	raftConf.LocalID = hraft.ServerID(nodeID)
	raftLogCacheSize := 512
	raftConf.LogLevel = raftLogger.GetLevel().String()
	raftConf.Logger = raftLogger

	// create a stable store
	// TODO: see why hashicorp advises against use in prod, maybe
	// implement custom one locally... assuming they don't like that
	// it's in memory, but our nodes are ephemeral and keys are low
	// cardinality, so should be OK.
	stableStore := hraft.NewInmemStore()

	logStore, err := hraft.NewLogCache(raftLogCacheSize, stableStore)
	if err != nil {
		return nil, err
	}

	// snapshot store that discards everything
	// TODO: see if there's any reason not to use this in-mem snapshot store
	var snapshotStore raft.SnapshotStore = raft.NewInmemSnapshotStore()
	// var snapshotStore hraft.SnapshotStore = hraft.NewDiscardSnapshotStore()

	// grpc transport
	grpcTransport := transport.New(
		hraft.ServerAddress(addr),
		[]grpc.DialOption{
			// TODO: is this needed, or is insecure default?
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		},
	)

	// raft server
	raftServer, err := hraft.NewRaft(raftConf, raftFsm, logStore, stableStore, snapshotStore, grpcTransport.Transport())
	if err != nil {
		return nil, err
	}

	// initial stopped flag
	var stopped uint32

	return &Node{
		ID:               nodeID,
		RaftPort:         raftPort,
		address:          addr,
		Raft:             raftServer,
		TransportManager: grpcTransport,
		Serializer:       serializer,
		logger:           logger,
		stopped:          &stopped,
		GrpcServer:       grpcServer,
		isStarted:        false,
		mtx:              &sync.Mutex{},
	}, nil
}

// Start starts the node
func (n *Node) Start(ctx context.Context) error { // nolint
	n.logger.Infof("Starting Raft Node, ID=%s", n.ID)

	n.mtx.Lock()
	defer n.mtx.Unlock()

	if n.isStarted {
		return errors.New("already started")
	}

	// register management services
	n.TransportManager.Register(n.GrpcServer)
	// register custom raft RPC
	raftRPCServer := NewRPCServer(n)
	partipb.RegisterRaftServer(n.GrpcServer, raftRPCServer)

	// set up the tpc listener
	listener, err := net.Listen("tcp", n.address)
	// handle the error
	if err != nil {
		// enrich the error message
		enriched := errors.Wrapf(err, "failed to listen on address=%s", n.address)
		n.logger.Error(enriched.Error())
		return enriched
	}
	// serve grpc
	go func() {
		if err := n.GrpcServer.Serve(listener); err != nil {
			// here we panic because it is fundamental that the server start listening
			panic(err)
		}
	}()

	n.logger.Infof("Node started on port %d\n", n.RaftPort)

	n.isStarted = true
	go n.waitForBootstrap()

	return nil
}

func (n *Node) waitForBootstrap() {
	for {
		if !n.isStarted {
			return
		}
		numPeers := 0
		cfg := n.Raft.GetConfiguration()
		if err := cfg.Error(); err != nil {
			n.logger.Error(errors.Wrap(err, "couldn't read config").Error())
		} else {
			numPeers = len(cfg.Configuration().Servers)
		}
		if numPeers > 0 {
			n.mtx.Lock()
			n.isBootstrapped = true
			n.mtx.Unlock()
			return
		}

		// add some debug log here
		n.logger.Debugf("waiting for raft peers, current state %s", n.Raft.State().String())
		time.Sleep(time.Second)
	}
}

func (n *Node) IsBootstrapped() bool {
	return n.isBootstrapped
}

func (n *Node) Bootstrap() error {
	podIP := os.Getenv("POD_IP")
	if podIP == "" {
		return errors.New("missing POD_IP")
	}
	addr := fmt.Sprintf("%s:%d", podIP, n.RaftPort)

	// raft server
	configuration := hraft.Configuration{
		Servers: []hraft.Server{
			{
				ID:      hraft.ServerID(n.ID),
				Address: hraft.ServerAddress(addr),
			},
		},
	}
	f := n.Raft.BootstrapCluster(configuration)
	err := f.Error()
	if err != nil {
		return err
	}
	return nil
}

// Stop stops the node and notifies on stopped channel returned in Start
func (n *Node) Stop() {
	n.mtx.Lock()
	defer n.mtx.Unlock()
	if n.isStarted {
		// snapshot state
		if err := n.Raft.Snapshot().Error(); err != nil {
			n.logger.Error(errors.Wrap(err, "failed to create snapshot"))
		}
		// shut down raft
		if err := n.Raft.Shutdown().Error(); err != nil {
			n.logger.Error(errors.Wrapf(err, "failed to shutdown raft").Error())
		}
		// shut down gRPC
		n.logger.Info("Raft stopped")
		n.GrpcServer.GracefulStop()
		n.isStarted = false
	}
}

func (n *Node) getPeerDetails(peerAddress string) (*partipb.GetPeerDetailsResponse, error) {
	var opt grpc.DialOption = grpc.EmptyDialOption{}
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx,
		peerAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(), opt)
	if err != nil {
		return nil, err
	}
	defer conn.Close() // nolint
	client := partipb.NewRaftClient(conn)

	response, err := client.GetPeerDetails(ctx, &partipb.GetPeerDetailsRequest{})
	if err != nil {
		return nil, err
	}

	return response, nil
}

func (n *Node) AddPeer(nodeID string, host string, port uint16) error {
	if !n.IsLeader() {
		return nil
	}
	// skip self
	if nodeID == n.ID {
		return nil
	}
	// compute peer addr
	peerAddr := fmt.Sprintf("%s:%d", host, port)

	// ensure not duplicate peer
	for _, server := range n.Raft.GetConfiguration().Configuration().Servers {
		if string(server.ID) == nodeID {
			n.logger.Debugf("skipping duplicate peer add %s", nodeID)
			return nil
		}
	}

	// contact remote peer
	detailsResp, err := n.getPeerDetails(peerAddr)
	if err != nil {
		n.logger.Error(err.Error())
		return err
	}
	// confirm node ID match
	if detailsResp.GetServerId() != nodeID {
		return fmt.Errorf(
			"remove server nodeID '%s' mismatch '%s'",
			detailsResp.GetServerId(),
			nodeID,
		)
	}
	// add voter to raft
	res := n.Raft.AddVoter(hraft.ServerID(nodeID), hraft.ServerAddress(peerAddr), 0, 0)
	return res.Error()
}

func (n *Node) RemovePeer(nodeID string) error {
	if !n.IsLeader() {
		return nil
	}

	for _, server := range n.Raft.GetConfiguration().Configuration().Servers {
		if string(server.ID) == nodeID {
			res := n.Raft.RemoveServer(server.ID, 0, 0)
			return res.Error()
		}
	}

	n.logger.Infof("did not find peer to remove Peer=%s", nodeID)
	return nil
}

// RaftApply is used to apply any new logs to the raft cluster
// this method does automatic forwarding to Leader node
func (n *Node) RaftApply(request interface{}, timeout time.Duration) (interface{}, error) {
	// serialize the payload for use in raft
	payload, err := n.Serializer.Serialize(request)
	if err != nil {
		return nil, err
	}
	return n.raftApplyLocalLeader(payload, timeout)
}

func (n *Node) raftApplyLocalLeader(payload []byte, timeout time.Duration) (interface{}, error) {
	if !n.IsLeader() {
		return nil, errors.New("must be leader")
	}
	result := n.Raft.Apply(payload, timeout)
	if result.Error() != nil {
		return nil, result.Error()
	}
	switch result.Response().(type) {
	case error:
		return nil, result.Response().(error)
	default:
		return result.Response(), nil
	}
}

// IsLeader returns true if the current node is the cluster leader
func (n *Node) IsLeader() bool {
	// _, id := n.Raft.LeaderWithID()
	// return n.ID == string(id)
	return n.Raft.VerifyLeader().Error() == nil
}

// HasLeader returns true if the current node is aware of a cluster leader
// including itself
func (n *Node) HasLeader() bool {
	leaderAddr, _ := n.Raft.LeaderWithID()
	return string(leaderAddr) != ""
}

// GetPeerSelf returns a Peer for the current node
func (n *Node) GetPeerSelf() *Peer {
	// todo: make this smarter for self
	raftAddr := fmt.Sprintf("0.0.0.0:%d", n.RaftPort)
	n.logger.Debugf("returning peer for self %s @ %s", n.ID, raftAddr)
	return NewPeer(n.ID, raftAddr)
}

// GetPeers returns all nodes in raft cluster (including self)
func (n *Node) GetPeers() []*Peer {
	peers := make(map[string]*Peer, 3)

	if cfg := n.Raft.GetConfiguration(); cfg.Error() == nil {
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

// GetPeer returns a specific peer given an ID
func (n *Node) GetPeer(peerID string) (*Peer, error) {
	if peerID == n.ID {
		return n.GetPeerSelf(), nil
	}
	cfg := n.Raft.GetConfiguration()
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
		return nil, fmt.Errorf("could not find raft peer (%s)", peerID)
	}
	n.logger.Debugf("retrieved peer %s @ %s", peerID, raftAddr)
	return NewPeer(peerID, raftAddr), nil
}

// newNodeID returns a random Node ID of length `size`
// nolint:unused
func newNodeID(size int) string {
	// TODO, do we need to do this anymore?
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, size)
	for i := range b {
		b[i] = nodeIDCharacters[rand.Intn(len(nodeIDCharacters))] // nolint
	}
	return string(b)
}

// Delete a key from the cluster state
// nolint:unused
func Delete(n *Node, group string, key string) error {
	request := &partipb.FsmRemoveRequest{Group: group, Key: key}
	_, err := n.RaftApply(request, time.Second)
	return err
}

// Put a key and value in cluster state
func Put(n *Node, group string, key string, value proto.Message) error {
	anyVal, err := anypb.New(value)
	if err != nil {
		return err
	}
	request := &partipb.FsmPutRequest{Group: group, Key: key, Value: anyVal}
	_, err = n.RaftApply(request, time.Second)
	return err
}

// Get the value for a key in cluster state
func Get[T any](n *Node, group, key string) (T, error) {
	request := &partipb.FsmGetRequest{Group: group, Key: key}
	result, err := n.RaftApply(request, time.Second)
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
