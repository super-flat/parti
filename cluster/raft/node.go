package raft

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	transport "github.com/Jille/raft-grpc-transport"
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/raft"
	"github.com/super-flat/parti/cluster/raft/discovery"
	"github.com/super-flat/parti/cluster/raft/serializer"
	partipb "github.com/super-flat/parti/partipb/parti/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const nodeIDCharacters = "abcdefghijklmnopqrstuvwxyz"

type Node struct {
	ID               string
	RaftPort         int
	DiscoveryPort    int
	address          string
	Raft             *raft.Raft
	GrpcServer       *grpc.Server
	DiscoveryMethod  discovery.IDiscovery
	TransportManager *transport.Manager
	Serializer       serializer.Serializer
	mList            *memberlist.Memberlist
	discoveryConfig  *memberlist.Config
	stopped          *uint32
	logger           *log.Logger
	stoppedCh        chan interface{}
	snapshotEnabled  bool
}

// NewNode returns an EasyRaft node
func NewNode(raftPort, discoveryPort int, raftFsm raft.FSM, serializer serializer.Serializer, discoveryMethod discovery.IDiscovery) (*Node, error) {
	// default raft config
	addr := fmt.Sprintf("%s:%d", "0.0.0.0", raftPort)
	nodeId := newNodeID(6)

	raftConf := raft.DefaultConfig()
	raftConf.LocalID = raft.ServerID(nodeId)
	raftLogCacheSize := 512
	raftConf.LogLevel = "Info"

	// create a stable store
	// TODO: see why hasicorp advises against use in prod, maybe
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
	var snapshotStore raft.SnapshotStore = raft.NewInmemSnapshotStore()
	// snapshotStore = raft.NewInmemSnapshotStore()
	// snapshotStore = raft.NewDiscardSnapshotStore()

	// grpc transport
	grpcTransport := transport.New(
		raft.ServerAddress(addr),
		[]grpc.DialOption{
			// TODO: is this needed, or is insecure default?
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		},
	)

	// memberlist config
	mlConfig := memberlist.DefaultWANConfig()
	mlConfig.BindPort = discoveryPort
	mlConfig.Name = fmt.Sprintf("%s:%d", nodeId, raftPort)

	// raft server
	raftServer, err := raft.NewRaft(raftConf, raftFsm, logStore, stableStore, snapshotStore, grpcTransport.Transport())
	if err != nil {
		return nil, err
	}

	// logging
	logger := log.Default()
	logger.SetPrefix("[EasyRaft] ")

	grpcServer := grpc.NewServer()

	// initial stopped flag
	var stopped uint32

	return &Node{
		ID:               nodeId,
		RaftPort:         raftPort,
		address:          addr,
		Raft:             raftServer,
		TransportManager: grpcTransport,
		Serializer:       serializer,
		DiscoveryPort:    discoveryPort,
		DiscoveryMethod:  discoveryMethod,
		discoveryConfig:  mlConfig,
		logger:           logger,
		stopped:          &stopped,
		GrpcServer:       grpcServer,
	}, nil
}

// Start starts the Node and returns a channel that indicates, that the node has been stopped properly
func (n *Node) Start() (chan interface{}, error) {
	n.logger.Printf("Starting Raft Node, ID=%s", n.ID)
	// set stopped as false
	if atomic.LoadUint32(n.stopped) == 1 {
		atomic.StoreUint32(n.stopped, 0)
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

	// memberlist discovery
	n.discoveryConfig.Events = n
	list, err := memberlist.Create(n.discoveryConfig)
	if err != nil {
		return nil, err
	}
	n.mList = list

	// register management services
	n.TransportManager.Register(n.GrpcServer)

	// register custom raft RPC
	raftRpcServer := NewRaftRpcServer(n)
	partipb.RegisterRaftServer(n.GrpcServer, raftRpcServer)

	// discovery method
	discoveryChan, err := n.DiscoveryMethod.Start(n.ID)
	if err != nil {
		return nil, err
	}
	go n.handleDiscoveredNodes(discoveryChan)

	// serve grpc
	// grpc server
	grpcListen, err := net.Listen("tcp", n.address)
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		if err := n.GrpcServer.Serve(grpcListen); err != nil {
			n.logger.Fatal(err)
		}
	}()

	// handle interruption
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGABRT, syscall.SIGKILL)
	go func() {
		_ = <-sigs
		n.Stop()
	}()

	n.logger.Printf("Node started on port %d and discovery port %d\n", n.RaftPort, n.DiscoveryPort)
	n.stoppedCh = make(chan interface{})

	return n.stoppedCh, nil
}

// Stop stops the node and notifies on stopped channel returned in Start
func (n *Node) Stop() {
	if atomic.LoadUint32(n.stopped) == 0 {
		atomic.StoreUint32(n.stopped, 1)
		if n.snapshotEnabled {
			n.logger.Println("Creating snapshot...")
			err := n.Raft.Snapshot().Error()
			if err != nil {
				n.logger.Println("Failed to create snapshot!")
			}
		}
		n.logger.Println("Stopping Node...")
		n.DiscoveryMethod.Stop()
		err := n.mList.Leave(10 * time.Second)
		if err != nil {
			n.logger.Printf("Failed to leave from discovery: %q\n", err.Error())
		}
		err = n.mList.Shutdown()
		if err != nil {
			n.logger.Printf("Failed to shutdown discovery: %q\n", err.Error())
		}
		n.logger.Println("Discovery stopped")
		err = n.Raft.Shutdown().Error()
		if err != nil {
			n.logger.Printf("Failed to shutdown Raft: %q\n", err.Error())
		}
		n.logger.Println("Raft stopped")
		n.GrpcServer.GracefulStop()
		n.logger.Println("Raft Server stopped")
		n.logger.Println("Node Stopped!")
		n.stoppedCh <- true
	}
}

// handleDiscoveredNodes handles the discovered Node additions
func (n *Node) handleDiscoveredNodes(discoveryChan chan string) {
	for peerAddr := range discoveryChan {
		detailsResp, err := n.getPeerDetails(peerAddr)
		if err == nil {
			serverId := detailsResp.ServerId
			needToAddNode := true
			for _, server := range n.Raft.GetConfiguration().Configuration().Servers {
				if server.ID == raft.ServerID(serverId) || string(server.Address) == peerAddr {
					needToAddNode = false
					break
				}
			}
			if needToAddNode {
				peerHost := strings.Split(peerAddr, ":")[0]
				peerDiscoveryAddr := fmt.Sprintf("%s:%d", peerHost, detailsResp.DiscoveryPort)
				_, err = n.mList.Join([]string{peerDiscoveryAddr})
				if err != nil {
					log.Printf("failed to join to cluster using discovery address: %s\n", peerDiscoveryAddr)
				}
			}
		}
	}
}

func (n *Node) getPeerDetails(peerAddress string) (*partipb.GetPeerDetailsResponse, error) {
	var opt grpc.DialOption = grpc.EmptyDialOption{}
	conn, err := grpc.Dial(peerAddress, grpc.WithInsecure(), grpc.WithBlock(), opt)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	client := partipb.NewRaftClient(conn)

	response, err := client.GetPeerDetails(context.Background(), &partipb.GetPeerDetailsRequest{})
	if err != nil {
		return nil, err
	}

	return response, nil
}

// NotifyJoin triggered when a new Node has been joined to the cluster (discovery only)
// and capable of joining the Node to the raft cluster
func (n *Node) NotifyJoin(node *memberlist.Node) {
	nameParts := strings.Split(node.Name, ":")
	nodeId, nodePort := nameParts[0], nameParts[1]
	nodeAddr := fmt.Sprintf("%s:%s", node.Addr, nodePort)
	if err := n.Raft.VerifyLeader().Error(); err == nil {
		result := n.Raft.AddVoter(raft.ServerID(nodeId), raft.ServerAddress(nodeAddr), 0, 0)
		if result.Error() != nil {
			log.Println(result.Error().Error())
		}
	}
}

// NotifyLeave triggered when a Node becomes unavailable after a period of time
// it will remove the unavailable Node from the Raft cluster
func (n *Node) NotifyLeave(node *memberlist.Node) {
	if n.DiscoveryMethod.SupportsNodeAutoRemoval() {
		nodeId := strings.Split(node.Name, ":")[0]
		if err := n.Raft.VerifyLeader().Error(); err == nil {
			result := n.Raft.RemoveServer(raft.ServerID(nodeId), 0, 0)
			if result.Error() != nil {
				log.Println(result.Error().Error())
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
	var opt grpc.DialOption = grpc.EmptyDialOption{}
	conn, err := grpc.Dial(string(n.Raft.Leader()), grpc.WithInsecure(), grpc.WithBlock(), opt)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	client := partipb.NewRaftClient(conn)

	response, err := client.ApplyLog(context.Background(), &partipb.ApplyLogRequest{Request: payload})
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
	return n.Raft.Leader() != ""
}

// newNodeID returns a random node ID of length `size`
func newNodeID(size int) string {
	// TODO, do we need to do this anymore?
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, size)
	for i := range b {
		b[i] = nodeIDCharacters[rand.Intn(len(nodeIDCharacters))]
	}
	return string(b)
}
