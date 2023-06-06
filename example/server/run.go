package server

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/super-flat/parti/cluster"
	"github.com/super-flat/parti/discovery"
	"github.com/super-flat/parti/log"
)

func Run() {
	// define the logger
	logger := log.DefaultLogger

	logger.Info("starting example server")
	ctx := context.Background()
	var cancelCtx context.CancelFunc
	ctx, cancelCtx = context.WithCancel(ctx)
	cfg, err := NewConfigFromEnv()
	if err != nil {
		panic(err)
	}
	// define a handler for our clustered app
	handler := &ExampleHandler{logger: logger}
	// run the raft node
	numPartitions := uint32(10)
	// define discovery
	namespace := "default"
	podLabels := map[string]string{"app": "parti"}
	portName := "parti"
	members := discovery.NewKubernetes(namespace, podLabels, portName, logger)
	// configure a cluster
	partiNode := cluster.NewCluster(
		ctx,
		cfg.RaftPort,
		handler,
		members,
		cluster.WithPartitionCount(numPartitions),
		cluster.WithLogger(logger),
		cluster.WithLogLevel(log.DebugLevel),
	)
	// start the node
	if err := partiNode.Start(ctx); err != nil {
		panic(err)
	}
	// run an http server
	web := NewWebServer(partiNode, uint16(cfg.HTTPPort))
	web.Start()
	// wait for interruption/termination
	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	// wait for a shutdown signal, and then shutdown
	go func() {
		<-sigs
		web.Stop(ctx)
		partiNode.Stop(ctx)
		cancelCtx()
		done <- true
	}()
	<-done
	logger.Info("exiting example server")
}
