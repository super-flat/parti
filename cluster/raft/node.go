package raftwrapper

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
	"github.com/hashicorp/memberlist"
	hraft "github.com/hashicorp/raft"
	"github.com/super-flat/parti/cluster/log"
	"github.com/super-flat/parti/cluster/membership"
	"github.com/super-flat/parti/cluster/raft/serializer"
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
	mList            *memberlist.Memberlist
	stopped          *uint32
	logger           log.Logger
	stoppedCh        chan interface{}
	mtx              *sync.Mutex
	isStarted        bool
	members          membership.Provider
}

// NewNode returns an raft node
func NewNode(raftPort int, raftFsm hraft.FSM, serializer serializer.Serializer, members membership.Provider, logger log.Logger) (*Node, error) {
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
		members:          members,
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
	// TODO: accept context in method args
	ctx := context.Background()

	n.logger.Infof("Starting Raft Node, ID=%s", n.ID)

	n.mtx.Lock()
	defer n.mtx.Unlock()

	if n.isStarted {
		return nil, errors.New("already started")
	}

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
		return nil, err
	}

	// register management services
	n.TransportManager.Register(n.GrpcServer)

	// register custom raft RPC
	raftRPCServer := NewRaftRPCServer(n)
	partipb.RegisterRaftServer(n.GrpcServer, raftRPCServer)

	// discovery method
	discoveryChan, err := n.members.Listen(ctx)
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

	n.logger.Infof("Node started on port %d\n", n.RaftPort)
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
func (n *Node) handleDiscoveredNodes(events chan membership.MembershipEvent) {
	n.logger.Info("begin listening for member changes")
	for event := range events {
		n.logger.Infof("received event for addr %s:%d", event.Host, event.Port)
		switch event.Change {
		case membership.MemberAdded:
			peerAddr := fmt.Sprintf("%s:%d", event.Host, event.Port)
			detailsResp, err := n.getPeerDetails(peerAddr)
			if err != nil {
				n.logger.Error(err)
			} else {
				if err := n.handleAddPeerEvent(detailsResp.ServerId, event.Host, event.Port); err != nil {
					n.logger.Errorf("failed to add peer, %v", err)
				}

			}
		case membership.MemberRemoved:
			n.handleRemovePeerEvent(event.Host, event.Port)

		case membership.MemberPinged:
			peerAddr := fmt.Sprintf("%s:%d", event.Host, event.Port)
			detailsResp, err := n.getPeerDetails(peerAddr)
			if err != nil {
				n.logger.Error(err)
			} else {
				if err := n.handleAddPeerEvent(detailsResp.ServerId, event.Host, event.Port); err != nil {
					n.logger.Errorf("failed to add peer, %v", err)
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

func (n *Node) handleAddPeerEvent(ID string, host string, port uint16) error {
	if n.IsLeader() {
		addr := fmt.Sprintf("%s:%d", host, port)
		// skip dups
		for _, server := range n.Raft.GetConfiguration().Configuration().Servers {
			if string(server.Address) == addr || string(server.ID) == ID {
				n.logger.Debugf("skipping duplicate peer add %s", ID)
				return nil
			}
		}
		if err := n.Raft.AddVoter(hraft.ServerID(ID), hraft.ServerAddress(addr), 0, 0).Error(); err != nil {
			return err
		}
	}
	return nil
}

func (n *Node) handleRemovePeerEvent(host string, port uint16) error {
	if n.IsLeader() {
		addr := fmt.Sprintf("%s:%d", host, port)
		// find the server using the host/port
		// TODO: rethink this
		for _, server := range n.Raft.GetConfiguration().Configuration().Servers {
			if string(server.Address) == addr {
				res := n.Raft.RemoveServer(server.ID, 0, 0)
				return res.Error()
			}
		}
	}
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
