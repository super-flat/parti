package server

import (
	"context"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/super-flat/parti/grpc/metrics"
	"github.com/super-flat/parti/grpc/traces"
	"github.com/super-flat/parti/logging"
	"github.com/super-flat/parti/prometheus"

	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

// Server will be implemented by the grpcServer
type Server interface {
	Start(ctx context.Context)
	Stop(ctx context.Context)
	AwaitTermination(ctx context.Context)
	GetListener() net.Listener
	GetServer() *grpc.Server
}

type grpcServer struct {
	addr     string
	server   *grpc.Server
	listener net.Listener

	traceProvider   *traces.Provider
	metricsExporter *prometheus.Server

	shutdownHook ShutdownHook
}

var _ Server = (*grpcServer)(nil)

// GetServer returns the underlying grpc.Server
// This is useful when one want to use the underlying grpc.Server
// for some registration like metrics, traces and so one
func (s grpcServer) GetServer() *grpc.Server {
	return s.server
}

// GetListener returns the underlying tcp listener
func (s grpcServer) GetListener() net.Listener {
	return s.listener
}

// Start the GRPC server and listen to incoming connections.
// It will panic in case of error
func (s *grpcServer) Start(ctx context.Context) {
	// start the metrics
	if s.metricsExporter != nil {
		// let us register the metrics
		metrics.RegisterGrpcServer(s.GetServer())
		// starts the metrics exporter
		s.metricsExporter.Start()
	}

	// let us register the tracer
	if s.traceProvider != nil {
		err := s.traceProvider.Register(ctx)
		if err != nil {
			logging.Fatal(errors.Wrap(err, errMsgTracerRegistrationFailure))
		}
	}

	var err error
	s.listener, err = net.Listen("tcp", s.addr)

	if err != nil {
		logging.Fatal(errors.Wrap(err, "failed to listen"))
	}

	go s.serv()

	logging.Infof("gRPC Server started on %s ", s.addr)
}

// Stop will shut down gracefully the running service.
// This is very useful when one wants to control the shutdown
// without waiting for an OS signal. For a long- running process, kindly use
// AwaitTermination after Start
func (s grpcServer) Stop(ctx context.Context) {
	s.cleanup(ctx)
	if s.shutdownHook != nil {
		s.shutdownHook()
	}
}

// AwaitTermination makes the program wait for the signal termination
// Valid signal termination (SIGINT, SIGTERM). This function should succeed Start.
func (s *grpcServer) AwaitTermination(ctx context.Context) {
	interruptSignal := make(chan os.Signal, 1)
	signal.Notify(interruptSignal, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-interruptSignal
	s.cleanup(ctx)
	if s.shutdownHook != nil {
		s.shutdownHook()
	}
}

// serv makes the grpc listener ready to accept connections
func (s *grpcServer) serv() {
	if err := s.server.Serve(s.listener); err != nil {
		logging.Panic(errors.Wrap(err, errMsgListenerServiceFailure))
	}
}

// cleanup stops the OTLP tracer and the metrics server and gracefully shutdowns the grpc server
// It stops the server from accepting new connections and RPCs and blocks until all the pending RPCs are
// finished and closes the underlying listener.
func (s *grpcServer) cleanup(ctx context.Context) {
	// stop the metrics grpcServer
	if s.metricsExporter != nil {
		s.metricsExporter.Stop()
	}

	// stop the tracing service
	if s.traceProvider != nil {
		err := s.traceProvider.Deregister(ctx)
		if err != nil {
			logging.Error(errors.Wrap(err, errMsgTracerDeregistrationFailure))
		}
	}

	logging.Info("Stopping the server")
	s.server.GracefulStop()

	logging.Info("Server stopped")
}
