package server

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/super-flat/parti/cluster"
)

func Run() {
	log.Print("starting example server")
	ctx := context.Background()
	var cancelCtx context.CancelFunc
	ctx, cancelCtx = context.WithCancel(ctx)
	cfg, err := NewConfigFromEnv()
	if err != nil {
		panic(err)
	}
	// define a handler for our clustered app
	handler := &ExampleHandler{}
	// run the raft node
	numPartitions := uint32(10)
	partiNode := cluster.NewNode(
		cfg.RaftPort,
		cfg.GrpcPort,
		cfg.DiscoveryPort,
		cfg.RaftDir,
		handler,
		numPartitions,
	)
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
	log.Print("exiting example server")
}
