package raftwrapper

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	transport "github.com/Jille/raft-grpc-transport"
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/raft"
	"github.com/super-flat/parti/cluster/log"
	"github.com/super-flat/parti/cluster/raftwrapper/discovery"
	"github.com/super-flat/parti/cluster/raftwrapper/serializer"
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
	DiscoveryPort    int
	address          string
	Raft             *raft.Raft
	GrpcServer       *grpc.Server
	DiscoveryMethod  discovery.Discovery
	TransportManager *transport.Manager
	Serializer       serializer.Serializer
	mList            *memberlist.Memberlist
	stopped          *uint32
	logger           log.Logger
	stoppedCh        chan interface{}
	mtx              *sync.Mutex
	isStarted        bool
}

// NewNode returns an raft node
func NewNode(raftPort, discoveryPort int, raftFsm raft.FSM, serializer serializer.Serializer, discoveryMethod discovery.Discovery, logger log.Logger) (*Node, error) {
	// default raft config
	addr := fmt.Sprintf("%s:%d", "0.0.0.0", raftPort)
	nodeID := newNodeID(6)

	raftConf := raft.DefaultConfig()
	raftConf.LocalID = raft.ServerID(nodeID)
	raftLogCacheSize := 512
	raftConf.LogLevel = "Info"

	// create a stable store
	// TODO: see why hashicorp advises against use in prod, maybe
	// implement custom one locally... assuming they dont like that
	// it's in memory, but our nodes are ephemeral and keys are low
	// cardinality, so should be OK.
	stableStore := raft.NewInmemStore()

	logStore, err := raft.NewLogCache(raftLogCacheSize, stableStore)
	if err != nil {
		return nil, err
	}

	// snapshot store that discards everything
	// TODO: see if there's any reason not to use this inmem snapshot store
	// var snapshotStore raft.SnapshotStore = raft.NewInmemSnapshotStore()
	var snapshotStore raft.SnapshotStore = raft.NewDiscardSnapshotStore()

	// grpc transport
	grpcTransport := transport.New(
		raft.ServerAddress(addr),
		[]grpc.DialOption{
			// TODO: is this needed, or is insecure default?
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		},
	)

	// raft server
	raftServer, err := raft.NewRaft(raftConf, raftFsm, logStore, stableStore, snapshotStore, grpcTransport.Transport())
	if err != nil {
		return nil, err
	}

	grpcServer := grpc.NewServer()

	// initial stopped flag
	var stopped uint32

	return &Node{
		ID:               nodeID,
		RaftPort:         raftPort,
		address:          addr,
		Raft:             raftServer,
		TransportManager: grpcTransport,
		Serializer:       serializer,
		DiscoveryPort:    discoveryPort,
		DiscoveryMethod:  discoveryMethod,
		logger:           logger,
		stopped:          &stopped,
		GrpcServer:       grpcServer,
		isStarted:        false,
		mtx:              &sync.Mutex{},
	}, nil
}

// Set custom logger
func (n *Node) WithLogger(logger log.Logger) {
	n.logger = logger
}

// Start starts the Node and returns a channel that indicates, that the node has been stopped properly
func (n *Node) Start() (chan interface{}, error) {
	n.logger.Infof("Starting Raft Node, ID=%s", n.ID)

	n.mtx.Lock()
	defer n.mtx.Unlock()

	if n.isStarted {
		return nil, errors.New("already started")
	}

	// raft server
	configuration := raft.Configuration{
		Servers: []raft.Server{
			{
				ID:      raft.ServerID(n.ID),
				Address: n.TransportManager.Transport().LocalAddr(),
			},
		},
	}
	f := n.Raft.BootstrapCluster(configuration)
	err := f.Error()
	if err != nil {
		return nil, err
	}

	// members list discovery
	mlConfig := memberlist.DefaultWANConfig()
	mlConfig.BindPort = n.DiscoveryPort
	mlConfig.Name = fmt.Sprintf("%s:%d", n.ID, n.RaftPort)
	mlConfig.Events = n
	list, err := memberlist.Create(mlConfig)
	if err != nil {
		return nil, err
	}
	n.mList = list

	// register management services
	n.TransportManager.Register(n.GrpcServer)

	// register custom raft RPC
	raftRPCServer := NewRaftRPCServer(n)
	partipb.RegisterRaftServer(n.GrpcServer, raftRPCServer)

	// discovery method
	discoveryChan, err := n.DiscoveryMethod.Start()
	if err != nil {
		return nil, err
	}
	go n.handleDiscoveredNodes(discoveryChan)

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

	// handle interruption
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGABRT, syscall.SIGTERM)
	go func() {
		<-sigs
		n.Stop()
	}()

	n.logger.Infof("Node started on port %d and discovery port %d\n", n.RaftPort, n.DiscoveryPort)
	n.stoppedCh = make(chan interface{})

	n.isStarted = true

	return n.stoppedCh, nil
}

// Stop stops the node and notifies on stopped channel returned in Start
func (n *Node) Stop() {
	n.mtx.Lock()
	defer n.mtx.Unlock()
	if n.isStarted {
		// snapshot state
		// if err := n.Raft.Snapshot().Error(); err != nil {
		// 	n.logger.Println("Failed to create snapshot!")
		// }
		// shut down discovery method
		n.DiscoveryMethod.Stop()
		if err := n.mList.Leave(10 * time.Second); err != nil {
			n.logger.Infof("Failed to leave from discovery: %q\n", err.Error())
		}
		if err := n.mList.Shutdown(); err != nil {
			n.logger.Infof("Failed to shutdown discovery: %q\n", err.Error())
		}
		// shut down raft
		if err := n.Raft.Shutdown().Error(); err != nil {
			n.logger.Infof("Failed to shutdown Raft: %q\n", err.Error())
		}
		// shut down gRPC
		n.logger.Info("Raft stopped")
		n.GrpcServer.GracefulStop()
		n.isStarted = false
		n.stoppedCh <- true
	}
}

func (n *Node) AwaitShutdown() {
	<-n.stoppedCh
}

// handleDiscoveredNodes handles the discovered Node additions
func (n *Node) handleDiscoveredNodes(discoveryChan chan string) {
	for peerAddr := range discoveryChan {
		detailsResp, err := n.getPeerDetails(peerAddr)
		if err == nil {
			serverID := detailsResp.ServerId
			needToAddNode := true
			for _, server := range n.Raft.GetConfiguration().Configuration().Servers {
				if server.ID == raft.ServerID(serverID) || string(server.Address) == peerAddr {
					needToAddNode = false
					break
				}
			}
			if needToAddNode {
				peerHost := strings.Split(peerAddr, ":")[0]
				peerDiscoveryAddr := fmt.Sprintf("%s:%d", peerHost, detailsResp.DiscoveryPort)
				_, err = n.mList.Join([]string{peerDiscoveryAddr})
				if err != nil {
					n.logger.Infof("failed to join to cluster using discovery address: %s\n", peerDiscoveryAddr)
				}
			}
		}
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

// NotifyJoin triggered when a new Node has been joined to the cluster (discovery only)
// and capable of joining the Node to the raft cluster
func (n *Node) NotifyJoin(node *memberlist.Node) {
	nameParts := strings.Split(node.Name, ":")
	nodeID, nodeRaftPort := nameParts[0], nameParts[1]
	nodeRaftAddr := fmt.Sprintf("%s:%s", node.Addr, nodeRaftPort)
	if n.IsLeader() {
		result := n.Raft.AddVoter(raft.ServerID(nodeID), raft.ServerAddress(nodeRaftAddr), 0, 0)
		if result.Error() != nil {
			n.logger.Info(result.Error().Error())
		}
	}
}

// NotifyLeave triggered when a Node becomes unavailable after a period of time
// it will remove the unavailable Node from the Raft cluster
func (n *Node) NotifyLeave(node *memberlist.Node) {
	if n.DiscoveryMethod.SupportsNodeAutoRemoval() {
		nodeID := strings.Split(node.Name, ":")[0]
		if n.IsLeader() {
			result := n.Raft.RemoveServer(raft.ServerID(nodeID), 0, 0)
			if result.Error() != nil {
				n.logger.Info(result.Error().Error())
			}
		}
	}
}

func (n *Node) NotifyUpdate(_ *memberlist.Node) {
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

func RaftApplyDelete(n *Node, group string, key string) error {
	request := &partipb.FsmRemoveRequest{Group: group, Key: key}
	_, err := n.RaftApply(request, time.Second)
	return err
}

func RaftApplyPut(n *Node, group string, key string, value proto.Message) error {
	anyVal, err := anypb.New(value)
	if err != nil {
		return err
	}
	request := &partipb.FsmPutRequest{Group: group, Key: key, Value: anyVal}
	_, err = n.RaftApply(request, time.Second)
	return err
}

func RaftApplyGet[T any](n *Node, group, key string) (T, error) {
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
