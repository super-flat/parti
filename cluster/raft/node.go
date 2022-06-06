package raft

import (
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
	"github.com/super-flat/parti/cluster/raft/fsm"
	"github.com/super-flat/parti/cluster/raft/serializer"
	partipb "github.com/super-flat/parti/gen/parti"
	"github.com/zemirco/uid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const nodeIDCharacters = "abcdefghijklmnopqrstuvwxyz"

// Node represents a raft Node
type Node struct {
	ID               string
	RaftPort         int
	DiscoveryPort    int
	address          string
	dataDir          string
	Raft             *raft.Raft
	GrpcServer       *grpc.Server
	Discovery        discovery.Discovery
	TransportManager *transport.Manager
	Serializer       serializer.Serializer
	mList            *memberlist.Memberlist
	discoveryConfig  *memberlist.Config
	stopped          *uint32
	logger           *log.Logger
	stoppedCh        chan interface{}
	snapshotEnabled  bool
}

// NewNode returns a Node instance
func NewNode(raftPort, discoveryPort int, dataDir string, services []fsm.Service, serializer serializer.Serializer, discovery discovery.Discovery, snapshotEnabled bool) (*Node, error) {
	// default raft config
	addr := fmt.Sprintf("%s:%d", "0.0.0.0", raftPort)
	nodeId := uid.New(50)
	raftConf := raft.DefaultConfig()
	raftConf.LocalID = raft.ServerID(nodeId)
	raftLogCacheSize := 512

	// TODO make this configurable
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
	var snapshotStore raft.SnapshotStore
	snapshotStore = raft.NewInmemSnapshotStore()
	// snapshotStore = raft.NewDiscardSnapshotStore()
	// grpc transport
	grpcTransport := transport.New(
		raft.ServerAddress(addr),
		[]grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		},
	)

	// init FSM
	sm := fsm.NewRoutingFSM(services)
	sm.Init(serializer)

	// member list config
	mlConfig := memberlist.DefaultWANConfig()
	mlConfig.BindPort = discoveryPort
	mlConfig.Name = fmt.Sprintf("%s:%d", nodeId, raftPort)

	// raft server
	raftServer, err := raft.NewRaft(raftConf, sm, logStore, stableStore, snapshotStore, grpcTransport.Transport())
	if err != nil {
		return nil, err
	}

	// logging
	// TODO replace this with our logger
	logger := log.Default()
	logger.SetPrefix("[PartiRaft] ")

	// initial stopped flag
	var stopped uint32

	return &Node{
		ID:               nodeId,
		RaftPort:         raftPort,
		address:          addr,
		dataDir:          dataDir,
		Raft:             raftServer,
		TransportManager: grpcTransport,
		Serializer:       serializer,
		DiscoveryPort:    discoveryPort,
		Discovery:        discovery,
		discoveryConfig:  mlConfig,
		logger:           logger,
		stopped:          &stopped,
		snapshotEnabled:  snapshotEnabled,
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

	// member list discovery
	n.discoveryConfig.Events = n
	list, err := memberlist.Create(n.discoveryConfig)
	if err != nil {
		return nil, err
	}
	n.mList = list

	// grpc server
	grpcListen, err := net.Listen("tcp", n.address)
	if err != nil {
		log.Fatal(err)
	}
	grpcServer := grpc.NewServer()
	n.GrpcServer = grpcServer

	// register management services
	n.TransportManager.Register(grpcServer)

	// register client services
	clientGrpcServer := NewClientGrpcService(n)
	partipb.RegisterRaftServiceServer(grpcServer, clientGrpcServer)

	// discovery method
	discoveryChan, err := n.Discovery.Start(n.ID, n.RaftPort)
	if err != nil {
		return nil, err
	}
	go n.handleDiscoveredNodes(discoveryChan)

	// serve grpc
	go func() {
		if err := grpcServer.Serve(grpcListen); err != nil {
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
// TODO expose a logging interface to replace std log. Use something like logr to allow dev to hookup their own logger
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
		n.Discovery.Stop()
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
	for peer := range discoveryChan {
		detailsResp, err := GetPeerDetails(peer)
		if err == nil {
			serverId := detailsResp.ServerId
			needToAddNode := true
			for _, server := range n.Raft.GetConfiguration().Configuration().Servers {
				if server.ID == raft.ServerID(serverId) || string(server.Address) == peer {
					needToAddNode = false
					break
				}
			}
			if needToAddNode {
				peerHost := strings.Split(peer, ":")[0]
				peerDiscoveryAddr := fmt.Sprintf("%s:%d", peerHost, detailsResp.DiscoveryPort)
				_, err = n.mList.Join([]string{peerDiscoveryAddr})
				if err != nil {
					log.Printf("failed to join to cluster using discovery address: %s\n", peerDiscoveryAddr)
				}
			}
		}
	}
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
	if n.Discovery.NodeAutoRemovalEnabled() {
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
	payload, err := n.Serializer.Serialize(request)
	if err != nil {
		return nil, err
	}

	if err := n.Raft.VerifyLeader().Error(); err == nil {
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

	response, err := ApplyOnLeader(n, payload)
	if err != nil {
		return nil, err
	}
	return response, nil
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