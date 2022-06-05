package metrics

import (
	grpcPrometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"google.golang.org/grpc"
)

// RegisterGrpcServer takes a gRPC server and pre-initializes all counters to 0. This
// allows for easier monitoring in Prometheus (no missing metrics), and should
// be called *after* all services have been registered with the server. This
// function acts on the DefaultServerMetrics variable.
func RegisterGrpcServer(grpcServer *grpc.Server) {
	grpcPrometheus.Register(grpcServer)
}
