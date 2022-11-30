package raft

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	transport "github.com/Jille/raft-grpc-transport"
	hraft "github.com/hashicorp/raft"
	"github.com/super-flat/parti/cluster/raft/serializer"
	"github.com/super-flat/parti/log"
	partipb "github.com/super-flat/parti/pb/parti/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

const nodeIDCharacters = "abcdefghijklmnopqrstuvwxyz"

type Node struct {
	ID               string
	RaftPort         int
	address          string
	Raft             *hraft.Raft
	GrpcServer       *grpc.Server
	TransportManager *transport.Manager
	Serializer       serializer.Serializer
	stopped          *uint32
	logger           log.Logger
	mtx              *sync.Mutex
	isStarted        bool
}

// NewNode returns an raft node
func NewNode(raftPort int, raftFsm hraft.FSM, serializer serializer.Serializer, logger log.Logger, grpcServer *grpc.Server) (*Node, error) {
	// default raft config
	addr := fmt.Sprintf("%s:%d", "0.0.0.0", raftPort)
	nodeID := newNodeID(6)

	raftConf := hraft.DefaultConfig()
	raftConf.LocalID = hraft.ServerID(nodeID)
	raftLogCacheSize := 512
	raftConf.LogLevel = "Info"

	// create a stable store
	// TODO: see why hashicorp advises against use in prod, maybe
	// implement custom one locally... assuming they dont like that
	// it's in memory, but our nodes are ephemeral and keys are low
	// cardinality, so should be OK.
	stableStore := hraft.NewInmemStore()

	logStore, err := hraft.NewLogCache(raftLogCacheSize, stableStore)
	if err != nil {
		return nil, err
	}

	// snapshot store that discards everything
	// TODO: see if there's any reason not to use this inmem snapshot store
	// var snapshotStore raft.SnapshotStore = raft.NewInmemSnapshotStore()
	var snapshotStore hraft.SnapshotStore = hraft.NewDiscardSnapshotStore()

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

// WithLogger sets custom log
func (n *Node) WithLogger(logger log.Logger) {
	n.logger = logger
}

// Start starts the Node
func (n *Node) Start(ctx context.Context, bootstrap bool) error {
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
	if err != nil {
		n.logger.Fatal(err)
	}
	// serve grpc
	go func() {
		if err := n.GrpcServer.Serve(listener); err != nil {
			n.logger.Fatal(err)
		}
	}()

	if bootstrap {
		// raft server
		configuration := hraft.Configuration{
			Servers: []hraft.Server{
				{
					ID:      hraft.ServerID(n.ID),
					Address: n.TransportManager.Transport().LocalAddr(),
				},
			},
		}
		f := n.Raft.BootstrapCluster(configuration)
		err := f.Error()
		if err != nil {
			return err
		}
	}

	// handle interruption
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGABRT, syscall.SIGTERM)
	go func() {
		<-sigs
		n.Stop()
	}()

	n.logger.Infof("Node started on port %d\n", n.RaftPort)

	n.isStarted = true

	return nil
}

// Stop stops the node and notifies on stopped channel returned in Start
func (n *Node) Stop() {
	n.mtx.Lock()
	defer n.mtx.Unlock()
	if n.isStarted {
		// snapshot state
		// if err := n.Raft.Snapshot().Error(); err != nil {
		// 	n.log.Println("Failed to create snapshot!")
		// }
		// shut down raft
		if err := n.Raft.Shutdown().Error(); err != nil {
			n.logger.Infof("Failed to shutdown Raft: %q\n", err.Error())
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

func (n *Node) AddPeer(host string, port uint16) error {
	if !n.IsLeader() {
		return nil
	}
	peerAddr := fmt.Sprintf("%s:%d", host, port)
	// ensure no other peer at this address port
	for _, server := range n.Raft.GetConfiguration().Configuration().Servers {
		if string(server.Address) == peerAddr {
			n.logger.Debugf("skipping duplicate peer add %s", peerAddr)
			return nil
		}
	}
	// contact remote peer
	detailsResp, err := n.getPeerDetails(peerAddr)
	if err != nil {
		n.logger.Error(err)
		return err
	}
	// skip self
	if detailsResp.GetServerId() == n.ID {
		return nil
	}
	res := n.Raft.AddVoter(hraft.ServerID(detailsResp.GetServerId()), hraft.ServerAddress(peerAddr), 0, 0)
	return res.Error()
}

func (n *Node) RemovePeer(host string, port uint16) error {
	if !n.IsLeader() {
		return nil
	}
	addr := fmt.Sprintf("%s:%d", host, port)
	// find the server using the host/port
	// TODO: rethink this
	for _, server := range n.Raft.GetConfiguration().Configuration().Servers {
		if string(server.Address) == addr {
			res := n.Raft.RemoveServer(server.ID, 0, 0)
			return res.Error()
		}
	}
	n.logger.Infof("did not find peer to remove at %s", addr)
	return nil
}

// RaftApply is used to apply any new logs to the raft cluster
// this method does automatic forwarding to Leader Node
func (n *Node) RaftApply(request interface{}, timeout time.Duration) (interface{}, error) {
	// serialize the payload for use in raft
	payload, err := n.Serializer.Serialize(request)
	if err != nil {
		return nil, err
	}
	// if leader, do local
	if n.IsLeader() {
		return n.raftApplyLocalLeader(payload, timeout)
	}
	// otherwise, run on remote leader
	// TODO: pass in timeout somehow and serialize in proto?
	return n.raftApplyRemoteLeader(payload)
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

func (n *Node) raftApplyRemoteLeader(payload []byte) (interface{}, error) {
	if !n.HasLeader() {
		return nil, errors.New("unknown leader")
	}
	ctx := context.Background()
	// get the leader address
	leaderAddr, _ := n.Raft.LeaderWithID()
	var opt grpc.DialOption = grpc.EmptyDialOption{}
	conn, err := grpc.DialContext(ctx,
		string(leaderAddr),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		opt)

	if err != nil {
		return nil, err
	}
	defer conn.Close() // nolint
	client := partipb.NewRaftClient(conn)

	response, err := client.ApplyLog(ctx, &partipb.ApplyLogRequest{Request: payload})
	if err != nil {
		return nil, err
	}
	result, err := n.Serializer.Deserialize(response.Response)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// IsLeader returns true if the current node is the cluster leader
func (n *Node) IsLeader() bool {
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
	return NewPeer(peerID, raftAddr), nil
}

// newNodeID returns a random node ID of length `size`
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
