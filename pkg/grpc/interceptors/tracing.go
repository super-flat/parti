package interceptors

import (
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
)

// NewTracingUnaryInterceptor helps gather traces and metrics from any grpc unary server
// request. Make sure to start the TracerProvider to connect to an OLTP connector
func NewTracingUnaryInterceptor(opts ...otelgrpc.Option) grpc.UnaryServerInterceptor {
	return otelgrpc.UnaryServerInterceptor(opts...)
}

// NewTracingStreamInterceptor helps gather traces and metrics from any grpc stream server
// request. Make sure to start the TracerProvider to connect to an OLTP connector
func NewTracingStreamInterceptor(opts ...otelgrpc.Option) grpc.StreamServerInterceptor {
	return otelgrpc.StreamServerInterceptor(opts...)
}

// NewTracingClientUnaryInterceptor helps gather traces and metrics from any grpc unary client
// request. Make sure to start the TracerProvider to connect to an OLTP connector
func NewTracingClientUnaryInterceptor(opts ...otelgrpc.Option) grpc.UnaryClientInterceptor {
	return otelgrpc.UnaryClientInterceptor(opts...)
}

// NewTracingClientStreamInterceptor helps gather traces and metrics from any grpc stream client
// request. Make sure to start the TracerProvider to connect to an OLTP connector
func NewTracingClientStreamInterceptor(opts ...otelgrpc.Option) grpc.StreamClientInterceptor {
	return otelgrpc.StreamClientInterceptor(opts...)
}
