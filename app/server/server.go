package server

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
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

var (
	// initial nodes count is fixed to three, their addresses are also fixed
	// these are the initial member nodes of the Raft cluster.
	addresses = []string{
		"localhost:63001",
		"localhost:63002",
		"localhost:63003",
	}
)

func Run(cfg *Config) {
	flag.Parse()
	if len(cfg.Address) == 0 && cfg.NodeID != 1 && cfg.NodeID != 2 && cfg.NodeID != 3 {
		fmt.Fprintf(os.Stderr, "node id must be 1, 2 or 3 when address is not specified\n")
		os.Exit(1)
	}
	// https://github.com/golang/go/issues/17393
	if runtime.GOOS == "darwin" {
		signal.Ignore(syscall.Signal(0xd))
	}
	initialMembers := make(map[uint64]string)
	// when joining a new node which is not an initial members, the initialMembers
	// map should be empty.
	// when restarting a node that is not a member of the initial nodes, you can
	// leave the initialMembers to be empty. we still populate the initialMembers
	// here for simplicity.
	if !cfg.Join {
		for idx, v := range addresses {
			// key is the NodeID, NodeID is not allowed to be 0
			// value is the raft address
			initialMembers[uint64(idx+1)] = v
		}
	}
	var nodeAddr string
	// for simplicity, in this example program, addresses of all those 3 initial
	// raft members are hard coded. when address is not specified on the command
	// line, we assume the node being launched is an initial raft member.
	if len(cfg.Address) != 0 {
		nodeAddr = cfg.Address
	} else {
		nodeAddr = initialMembers[uint64(cfg.NodeID)]
	}
	fmt.Fprintf(os.Stdout, "node address: %s\n", nodeAddr)
	// change the log verbosity
	logger.GetLogger("raft").SetLevel(logger.ERROR)
	logger.GetLogger("rsm").SetLevel(logger.WARNING)
	logger.GetLogger("transport").SetLevel(logger.WARNING)
	logger.GetLogger("grpc").SetLevel(logger.WARNING)
	// config for raft node
	// See GoDoc for all available options
	rc := config.Config{
		NodeID:             uint64(cfg.NodeID),
		ClusterID:          exampleClusterID,
		ElectionRTT:        10,
		HeartbeatRTT:       1,
		CheckQuorum:        true,
		SnapshotEntries:    10,
		CompactionOverhead: 5,
	}
	datadir := filepath.Join(
		".db",
		fmt.Sprintf("node=%d/%v/", cfg.NodeID, time.Now().Unix()),
	)
	// config for the nodehost
	// See GoDoc for all available options
	nhc := config.NodeHostConfig{
		NodeHostDir:    datadir,
		RTTMillisecond: 200,
		RaftAddress:    nodeAddr,
	}
	nh, err := dragonboat.NewNodeHost(nhc)
	if err != nil {
		panic(err)
	}
	if err := nh.StartCluster(initialMembers, cfg.Join, fsm.NewExampleStateMachine, rc); err != nil {
		fmt.Fprintf(os.Stderr, "failed to add cluster, %v\n", err)
		os.Exit(1)
	}

	// make webserver
	webServer := GetWebServer(exampleClusterID, nh)
	webServerAddr := fmt.Sprintf(":%s", cfg.WebPort)
	webServer.Run(webServerAddr)
}
