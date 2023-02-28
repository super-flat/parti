package cluster

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/pkg/errors"
	"github.com/super-flat/parti/cluster/membership"
	"github.com/super-flat/parti/cluster/rebalance"
	"github.com/super-flat/parti/internal/raft"
	"github.com/super-flat/parti/internal/raft/fsm"
	"github.com/super-flat/parti/internal/raft/serializer"
	"github.com/super-flat/parti/logging"
	partipb "github.com/super-flat/parti/pb/parti/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	partitionsGroupName = "partitions"
)

type Cluster struct {
	mtx                *sync.RWMutex
	isStarted          bool
	partitionCount     uint32
	membershipProvider membership.Provider
	node               *raft.Node
	nodeData           *fsm.ProtoFsm
	grpcServer         *grpc.Server
	msgHandler         Handler
	logger             logging.Logger
	logLevel           logging.Level
}

// NewCluster creates an instance of Cluster
func NewCluster(ctx context.Context, raftPort uint16, msgHandler Handler, membershipProvider membership.Provider, opts ...Option) *Cluster {
	// create an instance of cluster with some default values
	cluster := &Cluster{
		mtx:                &sync.RWMutex{},
		isStarted:          false,
		partitionCount:     20,
		nodeData:           fsm.NewProtoFsm(),
		grpcServer:         grpc.NewServer(),
		msgHandler:         msgHandler,
		logger:             logging.DefaultLogger,
		logLevel:           logging.InfoLevel,
		membershipProvider: membershipProvider,
	}

	// apply the various options to override the default values
	for _, opt := range opts {
		opt.Apply(cluster)
	}

	// retrieve the node ID from the membership implementation
	nodeID, err := cluster.membershipProvider.GetNodeID(ctx)
	if err != nil {
		panic(err)
	}

	// instantiate the raft node
	node, err := raft.NewNode(
		nodeID,
		int(raftPort),
		cluster.nodeData,
		serializer.NewProtoSerializer(),
		cluster.logLevel,
		cluster.logger,
		cluster.grpcServer,
	)

	// panic in case of failure
	if err != nil {
		panic(err)
	}

	// set the node
	cluster.node = node
	// return the created cluster
	return cluster
}

func (n *Cluster) handleMemberEvents(events <-chan membership.Event) {
	n.logger.Info("begin listening for member changes")
	for event := range events {
		if !n.node.IsLeader() {
			// n.logger.Debugf("skipping event because not leader")
			continue
		}
		n.logger.Debug("received event", "addr", hclog.Fmt("%s:%d", event.Host, event.Port))
		switch event.Change {
		case membership.MemberAdded:
			n.logger.Debug("handling MemberAdded, node", event.ID)
			if err := n.node.AddPeer(event.ID, event.Host, event.Port); err != nil {
				n.logger.Error(errors.Wrap(err, "failed to add peer").Error())
			}

		case membership.MemberRemoved:
			n.logger.Debug("handling MemberRemoved, node", event.ID)
			if err := n.node.RemovePeer(event.ID); err != nil {
				n.logger.Error(errors.Wrap(err, "failed to remove peer").Error())
			}

		case membership.MemberPinged:
			n.logger.Debug("handling MemberPinged, node", event.ID)
			if err := n.node.AddPeer(event.ID, event.Host, event.Port); err != nil {
				n.logger.Error("failed to add pinged peer", err.Error())
			}
		}
	}
	n.logger.Info("stopped listening for member changes")
}

// Stop shuts down this node
func (n *Cluster) Stop(ctx context.Context) {
	n.mtx.Lock()
	defer n.mtx.Unlock()
	if n.isStarted {
		n.logger.Info("Shutting down node")
		n.membershipProvider.Stop(ctx)
		n.node.Stop()
		n.grpcServer.GracefulStop()
		n.logger.Info("Completed node shutdown")
	}
	n.isStarted = false
}

// Start the cluster node
func (n *Cluster) Start(ctx context.Context) error {
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

	// read member events
	memberEvents, err := n.membershipProvider.Listen(ctx)
	if err != nil {
		return err
	}

	if err := n.node.Start(ctx); err != nil {
		return err
	}

	n.bootstrap(memberEvents)
	// handle member change events
	go n.handleMemberEvents(memberEvents)
	// do leader things
	go n.leaderRebalance()
	// complete startup
	n.isStarted = true
	n.logger.Debug("cluster is started")
	return nil
}

func (n *Cluster) bootstrap(memberEvents chan membership.Event) {
	n.logger.Debug("begin bootstrap gossip")
	t := time.Now()
	for {
		select {
		case m := <-memberEvents:
			switch m.Change {
			case membership.MemberAdded, membership.MemberPinged:
				n.logger.Debug("checking peer", m.ID)
				addr := fmt.Sprintf("%s:%d", m.Host, m.Port)
				resp, err := n.callPeerBootstrap(addr)
				if err != nil {
					n.logger.Error(fmt.Errorf("failed to call client %s bootstrap, %v", addr).Error())
				} else {
					// if peer is in cluster or has a higher ID, wait more time
					if resp.GetInCluster() {
						n.logger.Debug("found another active cluster")
						t = time.Now()
					} else if strings.Compare(resp.GetPeerId(), n.node.ID) > 0 {
						n.logger.Debug("found another better leader candidate", resp.GetPeerId())
						t = time.Now()
					} else {
						n.logger.Debug("this node is a better candidate than", m.ID)
					}
				}
			default:
				n.logger.Debug("skipping membership event %v", m.Change)
			}

		case <-time.After(time.Second):
			n.logger.Debug("no new member events received")
			// this case advances the loop if we haven't seen a new peer event
			// in 10 seconds
			// pass
		}

		if n.node.IsBootstrapped() {
			n.logger.Debug("bootstrap complete, joined another cluster")
			return
		}

		if time.Since(t) > time.Second*10 {
			// if we have seen no leader alternatives in 5 seconds, then
			// bootstrap this node as the leader
			n.logger.Debug("hasn't found another leader peer, bootstrapping")
			if err := n.node.Bootstrap(); err != nil {
				n.logger.Error(errors.Wrap(err, "failed to bootstrap node").Error())
			} else {
				return
			}
		}
	}
}

func (n *Cluster) callPeerBootstrap(addr string) (*partipb.BootstrapResponse, error) {
	var opt grpc.DialOption = grpc.EmptyDialOption{}
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx,
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(), opt)
	if err != nil {
		return nil, err
	}
	defer conn.Close() // nolint
	client := partipb.NewRaftClient(conn)
	return client.Bootstrap(ctx, &partipb.BootstrapRequest{
		FromPeerId: n.node.ID,
	})
}

// leaderRebalance allows the leader Node to delegate partitions to its
// cluster peers, considering nodes that have left and new nodes that
// have joined, with a goal of evenly distributing the work.
func (n *Cluster) leaderRebalance() {
	for {
		// wait 3 seconds
		// TODO: make configurable
		time.Sleep(time.Second * 3)
		// exit if not leader
		if !n.node.IsLeader() {
			continue
		}
		n.logger.Debug("start rebalance loop")
		// create a context for each loop
		// TODO: should this be a different context?
		ctx := context.Background()
		// get active peers
		peerMap := map[string]*raft.Peer{}
		activePeerIDs := make([]string, 0)
		for _, peer := range n.node.GetPeers() {
			if peer.IsReady() {
				peerMap[peer.ID] = peer
				activePeerIDs = append(activePeerIDs, peer.ID)
			}
		}
		// get current partitions
		// map partitionID -> peerID that owns the partition
		currentPartitions := make(map[uint32]string, n.partitionCount)
		for partitionID := uint32(0); partitionID < n.partitionCount; partitionID++ {
			partition, err := n.getPartition(partitionID)
			if err != nil {
				n.logger.Error(errors.Wrapf(err, "failed to get owner, partition=%d, %v", partitionID, err).Error())
			}
			if partition.GetOwner() != "" && partition.AcceptingMessages {
				currentPartitions[partitionID] = partition.GetOwner()
			}
		}
		// compute rebalance
		rebalancedOutput := rebalance.ComputeRebalance(n.partitionCount, currentPartitions, activePeerIDs)
		// apply any rebalance changes to the cluster
		for partitionID, newPeerID := range rebalancedOutput {
			currentPeerID, isMapped := currentPartitions[partitionID]
			// if the partition is already on the correct node, continue
			if currentPeerID == newPeerID {
				continue
			}
			// determine if this peer is online
			_, currentPeerIsOnline := peerMap[currentPeerID]
			// if it is mapped and the peer is online, shut down the partition
			if isMapped && currentPeerIsOnline {
				// do 2-phase shutdown
				// first, pause the partition
				err := n.setPartition(partitionID, currentPeerID, false)
				if err != nil {
					n.logger.Error(err.Error())
					continue
				}
				// then, invoke shutdown on owner
				currentPeer, err := n.node.GetPeer(currentPeerID)
				if err != nil {
					n.logger.Error(errors.Wrapf(err, "could not get peer %s, %v", currentPeerID).Error())
					// TODO: make this rollback smarter
					continue
				}
				shutdownRequest := &partipb.ShutdownPartitionRequest{
					PartitionId: partitionID,
				}
				resp, err := getClient(ctx, currentPeer).ShutdownPartition(ctx, shutdownRequest)
				if err != nil {
					// TODO: this means that the shutdown grpc call failed. when a node goes down,
					// this call will definitely fail. think about if there are other reasons this
					// might fail, and perhaps have some kind of retry here?
					n.logger.Warnf("failed to shutdown partition %d, %v", partitionID, err)
					continue
				} else if !resp.GetSuccess() {
					continue
				}
			}
			// now, assign a new owner, but don't accept new messages
			if err := n.setPartition(partitionID, newPeerID, false); err != nil {
				// TODO: make this smarter
				n.logger.Error(err.Error())
				continue
			}
			// then, invoke startup on new owner
			newPeer, err := n.node.GetPeer(newPeerID)
			if err != nil {
				// TODO: make this smarter
				continue
			}
			newPeerClient := getClient(ctx, newPeer)
			// wait until the peer has synced
			for {
				// use the stats endpoint to look up the partition
				// mapping on the new node
				statsResp, err := newPeerClient.Stats(ctx, &partipb.StatsRequest{})
				remoteOwner, exists := statsResp.GetPartitionOwners()[partitionID]
				if err == nil && exists && remoteOwner == newPeerID {
					break
				}
				time.Sleep(time.Millisecond * 100)
			}
			// start the partition
			startupResp, err := newPeerClient.StartPartition(
				ctx,
				&partipb.StartPartitionRequest{PartitionId: partitionID},
			)
			if err != nil {
				n.logger.Warnf("node (%s) failed to start partition (%d), %v", newPeerID, partitionID, err)
				continue
			} else if !startupResp.GetSuccess() {
				n.logger.Warnf("node (%s) failed to start partition (%d)", newPeerID, partitionID)
				continue
			}
			// unpause the partition on new node
			if err := n.setPartition(partitionID, newPeerID, true); err != nil {
				// TODO decide whether to panic or not
				n.logger.Error(err.Error())
				continue
			}
		}
	}
}

// getPartitionNode returns the Node that owns a partition
func (n *Cluster) getPartitionNode(partitionID uint32) (string, error) {
	val, err := n.getPartition(partitionID)
	return val.GetOwner(), err
}

// getPartition returns the current partition mapping from local state
func (n *Cluster) getPartition(partitionID uint32) (*partipb.PartitionOwnership, error) {
	key := strconv.FormatUint(uint64(partitionID), 10)
	val, err := raftGetLocally[*partipb.PartitionOwnership](n, partitionsGroupName, key)
	return val, err
}

// setPartition assigns a partition to a Node
func (n *Cluster) setPartition(partitionID uint32, nodeID string, acceptMessages bool) error {
	n.logger.Info(fmt.Sprintf("assigning node (%s) partition (%d) accepting messages (%v)", nodeID, partitionID, acceptMessages))

	key := strconv.FormatUint(uint64(partitionID), 10)

	value := &partipb.PartitionOwnership{
		PartitionId:       partitionID,
		Owner:             nodeID,
		AcceptingMessages: acceptMessages,
	}

	return raft.Put(n.node, partitionsGroupName, key, value)
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

func (n *Cluster) StartPartition(ctx context.Context, request *partipb.StartPartitionRequest) (*partipb.StartPartitionResponse, error) {
	partitionID := request.GetPartitionId()
	ownerNode, err := n.getPartition(partitionID)
	if err != nil {
		return nil, err
	}
	ownerNodeID := ownerNode.GetOwner()
	// if this node is not the owner, we cannot shut down that partition.
	// TODO: decide if error would be better here
	if ownerNodeID != n.node.ID {
		n.logger.Info(fmt.Sprintf("received partition start command for another node (%s), partition (%d)", ownerNodeID, request.GetPartitionId()))
		return &partipb.StartPartitionResponse{Success: false}, nil
	}
	// attempt to start the partition using the provided handler
	if err := n.msgHandler.StartPartition(ctx, partitionID); err != nil {
		n.logger.Warnf("failed to start partition %d, %v", partitionID, err)
		// TODO, should this return an error instead?
		return &partipb.StartPartitionResponse{Success: false}, nil
	}
	// return success to the caller
	return &partipb.StartPartitionResponse{Success: true}, nil
}

func (n *Cluster) ShutdownPartition(ctx context.Context, request *partipb.ShutdownPartitionRequest) (*partipb.ShutdownPartitionResponse, error) {
	partitionID := request.GetPartitionId()
	ownerNodeID, err := n.getPartitionNode(partitionID)
	if err != nil {
		return nil, err
	}
	// if this node is not the owner, we cannot shut down that partition.
	// TODO: decide if error would be better here
	if ownerNodeID != n.node.ID {
		n.logger.Warnf("received partition shutdown for another node (%s), partition=(%d)", ownerNodeID, request.GetPartitionId())
		return &partipb.ShutdownPartitionResponse{Success: false}, nil
	}
	// attempt to shut down the partition using the provided handler
	if err := n.msgHandler.ShutdownPartition(ctx, partitionID); err != nil {
		n.logger.Warnf("failed to shut down partition %d, %v", partitionID, err)
		// TODO, should this return an error instead?
		return &partipb.ShutdownPartitionResponse{Success: false}, nil
	}
	// return success to the caller
	return &partipb.ShutdownPartitionResponse{Success: true}, nil
}

// Send a message to the node that owns a partition
func (n *Cluster) Send(ctx context.Context, request *partipb.SendRequest) (*partipb.SendResponse, error) {
	partitionID := request.GetPartitionId()
	// try to get the partition, loop if it is paused
	// TODO: introduce some backoff here, perhaps with a max timeout
	var partition *partipb.PartitionOwnership
	var err error
	for {
		partition, err = n.getPartition(partitionID)
		if err != nil {
			return nil, err
		}
		if partition.GetAcceptingMessages() {
			break
		}
		n.logger.Infof("partition (%d) is paused on node (%s), backing off", partitionID, partition.GetOwner())
		time.Sleep(time.Second)
	}
	ownerNodeID := partition.GetOwner()
	// if partition owned by this node, answer locally
	if ownerNodeID == n.node.ID {
		n.logger.Infof("received local send, partition=%d, id=%s", partitionID, request.GetMessageId())
		handlerResp, err := n.msgHandler.Handle(ctx, partitionID, request.GetMessage())
		if err != nil {
			return nil, err
		}
		resp := &partipb.SendResponse{
			NodeId:      n.node.ID,
			PartitionId: request.GetPartitionId(),
			MessageId:   request.GetMessageId(),
			Response:    handlerResp,
			NodeChain:   []string{n.node.ID},
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
	n.logger.Info(fmt.Sprintf("forwarding send, node=%s, messageID=%s, partition=%d", peer.ID, request.GetMessageId(), partitionID))
	remoteResp, err := getClient(ctx, peer).Send(ctx, request)
	if err != nil {
		return nil, err
	}
	remoteResp.NodeChain = append(remoteResp.NodeChain, n.node.ID)
	return remoteResp, nil
}

// Ping a partition and receive a response from the node that owns it
func (n *Cluster) Ping(ctx context.Context, request *partipb.PingRequest) (*partipb.PingResponse, error) {
	partitionID := request.GetPartitionId()
	ownerNodeID, err := n.getPartitionNode(partitionID)
	if err != nil {
		return nil, err
	}
	if ownerNodeID == n.node.ID {
		n.logger.Info(fmt.Sprintf("received ping, answering locally, partition=%d", partitionID))
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
	n.logger.Info(fmt.Sprintf("forwarding ping, node=%s, partition=%d", peer.ID, partitionID))
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

// getClient returns a gRPC client for the given peer
// TODO: if peer is self, return the local implementation instead of forcing
// a gRPC call
func getClient(ctx context.Context, p *raft.Peer) partipb.ClusteringClient {
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
