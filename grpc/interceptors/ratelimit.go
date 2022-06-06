package interceptors

import (
	"context"
	"time"

	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Limiter defines the interface to perform request rate limiting.
// If Check function return true, the request will be rejected.
// Otherwise, the request will pass.
type Limiter interface {
	Check(ctx context.Context) bool
}

// RateLimiter implements Limiter interface.
type RateLimiter struct {
	ratelimiter *rate.Limiter // nolint
}

// Check applies the rate limit
func (l *RateLimiter) Check(ctx context.Context) bool {
	// This is a blocking call. Honors the rate limit
	if err := l.ratelimiter.Wait(ctx); err != nil {
		// rate limit reached
		return true
	}
	return false
}

// NewRateLimiter return new go-grpc Limiter, specified the number of requests you want to limit as well as the limit period.
func NewRateLimiter(requestCount int, limitPeriod time.Duration) *RateLimiter {
	return &RateLimiter{
		ratelimiter: rate.NewLimiter(rate.Every(limitPeriod), requestCount),
	}
}

// NewRateLimitUnaryServerInterceptor returns a new unary server interceptors that performs request rate limiting.
func NewRateLimitUnaryServerInterceptor(rateLimiter Limiter) grpc.UnaryServerInterceptor {
	// handle the rpc request
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// check for rate limit and block the request for being executed
		if rateLimiter.Check(ctx) {
			return nil, status.Errorf(codes.ResourceExhausted, "%s have been rejected by rate limiting.", info.FullMethod)
		}
		// allow the request processing when no rate limit occurs
		return handler(ctx, req)
	}
}

// NewRateLimitStreamServerInterceptor returns a new stream server interceptors that performs request rate limiting.
func NewRateLimitStreamServerInterceptor(rateLimiter Limiter) grpc.StreamServerInterceptor {
	// handle the rpc request
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		// check for rate limit and block the request for being executed
		if rateLimiter.Check(stream.Context()) {
			return status.Errorf(codes.ResourceExhausted, "%s have been rejected by rate limiting.", info.FullMethod)
		}
		// allow the request processing when no rate limit occurs
		return handler(srv, stream)
	}
}

// NewRateLimitUnaryClientInterceptor return client unary interceptor that limit requests.
func NewRateLimitUnaryClientInterceptor(rateLimiter Limiter) grpc.UnaryClientInterceptor {
	// handle the rpc request
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		// check for rate limit and block the request for being executed
		if rateLimiter.Check(ctx) {
			return status.Errorf(codes.ResourceExhausted, "%s have been rejected by rate limiting.", method)
		}
		// allow the request processing when no rate limit occurs
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// NewRateLimitStreamClientInterceptor return stream client unary interceptor that limit requests.
func NewRateLimitStreamClientInterceptor(rateLimiter Limiter) grpc.StreamClientInterceptor {
	// handle the rpc request
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		// check for rate limit and block the request for being executed
		if rateLimiter.Check(ctx) {
			return nil, status.Errorf(codes.ResourceExhausted, "%s have been rejected by rate limiting.", method)
		}
		// allow the request processing when no rate limit occurs
		return streamer(ctx, desc, cc, method, opts...)
	}
}
