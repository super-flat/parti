package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/lni/dragonboat/v3"
	"github.com/lni/dragonboat/v3/config"
	"github.com/lni/dragonboat/v3/logger"
	"github.com/super-flat/raft-poc/app/fsm"
)

const (
	exampleClusterID uint64 = 128
)

func Run(cfg *Config) {
	// https://github.com/golang/go/issues/17393
	if runtime.GOOS == "darwin" {
		signal.Ignore(syscall.Signal(0xd))
	}

	webServer := NewWebServer(exampleClusterID, uint64(cfg.RaftNodeID), cfg.RaftPort, cfg.ApiPort)
	webServer.Run()

	foundCluster := false
	initialMembers := make(map[uint64]string)

	peers, err := queryPeers(cfg.Peers)
	if err != nil {
		fmt.Printf("failure, %s\n", err.Error())
	} else {
		for _, n := range peers {
			if n.Joinable {
				foundCluster = true
				initialMembers = make(map[uint64]string)
				break
			}
			initialMembers[n.RaftNodeID] = n.RaftAddr
			fmt.Printf("found peer, id=%d, addr=%s\n", n.RaftNodeID, n.RaftAddr)
		}
	}

	fmt.Printf("found cluster: %v\n", foundCluster)

	nodeAddr := fmt.Sprintf("localhost:%s", cfg.RaftPort)
	fmt.Fprintf(os.Stdout, "node address: %s\n", nodeAddr)
	// change the log verbosity
	logger.GetLogger("raft").SetLevel(logger.ERROR)
	logger.GetLogger("rsm").SetLevel(logger.WARNING)
	logger.GetLogger("transport").SetLevel(logger.WARNING)
	logger.GetLogger("grpc").SetLevel(logger.WARNING)
	// config for raft node
	// See GoDoc for all available options
	rc := config.Config{
		NodeID:             uint64(cfg.RaftNodeID),
		ClusterID:          exampleClusterID,
		ElectionRTT:        10,
		HeartbeatRTT:       1,
		CheckQuorum:        true,
		SnapshotEntries:    10,
		CompactionOverhead: 5,
	}
	datadir := filepath.Join(
		".db",
		fmt.Sprintf("node=%d", cfg.RaftNodeID),
	)
	// config for the nodehost
	// See GoDoc for all available options
	nhc := config.NodeHostConfig{
		WALDir:         datadir,
		NodeHostDir:    datadir,
		RTTMillisecond: 200,
		RaftAddress:    nodeAddr,
	}
	nh, err := dragonboat.NewNodeHost(nhc)
	if err != nil {
		panic(err)
	}
	if err := nh.StartCluster(initialMembers, foundCluster, fsm.NewExampleStateMachine, rc); err != nil {
		fmt.Fprintf(os.Stderr, "failed to add cluster, %v\n", err)
		os.Exit(1)
	}
	webServer.SetNodeHost(nh)

	if foundCluster {
		for _, peer := range peers {
			if peer.Joinable {
				err := joinCluster(peer, cfg.RaftNodeID, nodeAddr)
				if err != nil {
					log.Fatal(err)
				}
				break
			}
		}
	}

	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 5 seconds.
	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)
	<-quit
	log.Println("Shutdown Server ...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	webServer.Stop(ctx)
	log.Println("Server exiting")
}

type PeerNode struct {
	RaftClusterID uint64
	RaftNodeID    uint64
	RaftAddr      string
	ApiAddr       string
	Joinable      bool
}

func queryPeers(peerAddresses []string) ([]*PeerNode, error) {
	output := make([]*PeerNode, 0, len(peerAddresses))

	for _, addr := range peerAddresses {
		r, err := http.Get(fmt.Sprintf("http://%s/info", addr))
		if err != nil {
			return nil, err
		}
		var b BootstrapInfo
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
			return nil, err
		}

		host := strings.Split(addr, ":")[0]
		raftAddr := fmt.Sprintf("%s:%s", host, b.RaftPort)

		peer := &PeerNode{
			ApiAddr:       addr,
			RaftClusterID: b.RaftClusterID,
			RaftNodeID:    b.RaftNodeID,
			RaftAddr:      raftAddr,
			Joinable:      b.Joinable,
		}

		output = append(output, peer)
	}

	return output, nil
}

func joinCluster(peer *PeerNode, myRaftID uint64, myRaftAddr string) error {
	addr := fmt.Sprintf("http://%s/cluster/addnode/%d/%s", peer.ApiAddr, myRaftID, myRaftAddr)
	res, err := http.Post(addr, "application/json", bytes.NewBuffer([]byte{}))
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		return errors.New("failed to join cluster")
	}
	return nil
}
