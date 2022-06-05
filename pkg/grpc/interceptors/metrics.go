package interceptors

import (
	grpcPrometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"google.golang.org/grpc"
)

// NewMetricUnaryInterceptor returns a grpc metric unary interceptor
func NewMetricUnaryInterceptor() grpc.UnaryServerInterceptor {
	// Create some standard server metrics.
	return grpcPrometheus.UnaryServerInterceptor
}

// NewMetricStreamInterceptor returns a grpc metric stream interceptor
func NewMetricStreamInterceptor() grpc.StreamServerInterceptor {
	return grpcPrometheus.StreamServerInterceptor
}

// NewClientMetricUnaryInterceptor creates a grpc client metric unary interceptor
func NewClientMetricUnaryInterceptor() grpc.UnaryClientInterceptor {
	return grpcPrometheus.UnaryClientInterceptor
}

// NewClientMetricStreamInterceptor creates a grpc client metric stream interceptor
func NewClientMetricStreamInterceptor() grpc.StreamClientInterceptor {
	return grpcPrometheus.StreamClientInterceptor
}
