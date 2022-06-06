package server

import (
	"crypto/tls"
	"fmt"
	"sync"
	"time"

	interceptors2 "github.com/super-flat/parti/grpc/interceptors"
	"github.com/super-flat/parti/grpc/traces"
	"github.com/super-flat/parti/prometheus"

	grpcMiddleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
)

const (
	// MaxConnectionAge is the duration a connection may exist before shutdown
	MaxConnectionAge = 600 * time.Second
	// MaxConnectionAgeGrace is the maximum duration a
	// connection will be kept alive for outstanding RPCs to complete
	MaxConnectionAgeGrace = 60 * time.Second
	// KeepAliveTime is the period after which a keepalive ping is sent on the
	// transport
	KeepAliveTime = 1200 * time.Second

	// default metrics port
	defaultMetricsPort = 9102
	// default grpc port
	defaultGrpcPort = 50051
)

// ShutdownHook is used to perform some cleaning before stopping
// the long-running grpcServer
type ShutdownHook func()

// Builder helps build a grpc grpcServer
type Builder struct {
	options           []grpc.ServerOption
	services          []serviceRegistry
	enableReflection  bool
	enableHealthCheck bool
	metricsEnabled    bool
	tracingEnabled    bool
	serviceName       string
	metricsPort       int
	grpcPort          int
	traceURL          string

	shutdownHook ShutdownHook
	isBuilt      bool

	rwMutex *sync.RWMutex
}

// NewServerBuilder creates an instance of ServerBuilder
func NewServerBuilder() *Builder {
	return &Builder{
		metricsPort: defaultMetricsPort,
		grpcPort:    defaultGrpcPort,
		isBuilt:     false,
		rwMutex:     &sync.RWMutex{},
	}
}

// WithShutdownHook sets the shutdown hook
func (sb *Builder) WithShutdownHook(fn ShutdownHook) *Builder {
	sb.shutdownHook = fn
	return sb
}

// WithPort sets the grpc service port
func (sb *Builder) WithPort(port int) *Builder {
	sb.grpcPort = port
	return sb
}

// WithMetricsPort sets the metrics service port
func (sb *Builder) WithMetricsPort(port int) *Builder {
	sb.metricsPort = port
	return sb
}

// WithMetricsEnabled enable grpc metrics
func (sb *Builder) WithMetricsEnabled(enabled bool) *Builder {
	sb.metricsEnabled = enabled
	return sb
}

// WithTracingEnabled enables tracing
func (sb *Builder) WithTracingEnabled(enabled bool) *Builder {
	sb.tracingEnabled = enabled
	return sb
}

// WithTraceURL sets the tracing URL
func (sb *Builder) WithTraceURL(traceURL string) *Builder {
	sb.traceURL = traceURL
	return sb
}

// WithOption adds a grpc service option
func (sb *Builder) WithOption(o grpc.ServerOption) *Builder {
	sb.options = append(sb.options, o)
	return sb
}

// WithService registers service with gRPC grpcServer
func (sb *Builder) WithService(service serviceRegistry) *Builder {
	sb.services = append(sb.services, service)
	return sb
}

// WithServiceName sets the service name
func (sb *Builder) WithServiceName(serviceName string) *Builder {
	sb.serviceName = serviceName
	return sb
}

// WithReflection enables the reflection
// gRPC RunnableService Reflection provides information about publicly-accessible gRPC services on a grpcServer,
// and assists clients at runtime to construct RPC requests and responses without precompiled service information.
// It is used by gRPC CLI, which can be used to introspect grpcServer protos and send/receive test RPCs.
//Warning! We should not have this enabled in production
func (sb *Builder) WithReflection(enabled bool) *Builder {
	sb.enableReflection = enabled
	return sb
}

// WithHealthCheck enables the default health check service
func (sb *Builder) WithHealthCheck(enabled bool) *Builder {
	sb.enableHealthCheck = enabled
	return sb
}

// WithKeepAlive is used to set keepalive and max-age parameters on the grpcServer-side.
func (sb *Builder) WithKeepAlive(serverParams keepalive.ServerParameters) *Builder {
	keepAlive := grpc.KeepaliveParams(serverParams)
	sb.WithOption(keepAlive)
	return sb
}

// WithDefaultKeepAlive is used to set the default keep alive parameters on the grpcServer-side
func (sb *Builder) WithDefaultKeepAlive() *Builder {
	return sb.WithKeepAlive(keepalive.ServerParameters{
		MaxConnectionIdle:     0,
		MaxConnectionAge:      MaxConnectionAge,
		MaxConnectionAgeGrace: MaxConnectionAgeGrace,
		Time:                  KeepAliveTime,
		Timeout:               0,
	})
}

// WithStreamInterceptors set a list of interceptors to the Grpc grpcServer for stream connection
// By default, gRPC doesn't allow one to have more than one interceptor either on the client nor on the grpcServer side.
// By using `grpcMiddleware` we are able to provides convenient method to add a list of interceptors
func (sb *Builder) WithStreamInterceptors(interceptors ...grpc.StreamServerInterceptor) *Builder {
	chain := grpc.StreamInterceptor(grpcMiddleware.ChainStreamServer(interceptors...))
	sb.WithOption(chain)
	return sb
}

// WithUnaryInterceptors set a list of interceptors to the Grpc grpcServer for unary connection
// By default, gRPC doesn't allow one to have more than one interceptor either on the client nor on the grpcServer side.
// By using `grpc_middleware` we are able to provides convenient method to add a list of interceptors
func (sb *Builder) WithUnaryInterceptors(interceptors ...grpc.UnaryServerInterceptor) *Builder {
	chain := grpc.UnaryInterceptor(grpcMiddleware.ChainUnaryServer(interceptors...))
	sb.WithOption(chain)
	return sb
}

// WithTLSCert sets credentials for grpcServer connections
func (sb *Builder) WithTLSCert(cert *tls.Certificate) *Builder {
	sb.WithOption(grpc.Creds(credentials.NewServerTLSFromCert(cert)))
	return sb
}

// WithDefaultUnaryInterceptors sets the default unary interceptors for the grpc grpcServer
func (sb *Builder) WithDefaultUnaryInterceptors() *Builder {
	return sb.WithUnaryInterceptors(
		interceptors2.NewRequestIDUnaryServerInterceptor(),
		interceptors2.NewTracingUnaryInterceptor(),
		interceptors2.NewMetricUnaryInterceptor(),
		interceptors2.NewRecoveryUnaryInterceptor(),
	)
}

// WithDefaultStreamInterceptors sets the default stream interceptors for the grpc grpcServer
func (sb *Builder) WithDefaultStreamInterceptors() *Builder {
	return sb.WithStreamInterceptors(
		interceptors2.NewRequestIDStreamServerInterceptor(),
		interceptors2.NewTracingStreamInterceptor(),
		interceptors2.NewMetricStreamInterceptor(),
		interceptors2.NewRecoveryStreamInterceptor(),
	)
}

// Build is responsible for building a GRPC grpcServer
func (sb *Builder) Build() (Server, error) {
	// check whether the builder has already been used
	sb.rwMutex.Lock()
	defer sb.rwMutex.Unlock()
	if sb.isBuilt {
		return nil, errMsgCannotUseSameBuilder
	}

	// create the grpc server
	srv := grpc.NewServer(sb.options...)

	// create the grpc server
	addr := fmt.Sprintf(":%d", sb.grpcPort)
	grpcServer := &grpcServer{
		addr:         addr,
		server:       srv,
		shutdownHook: sb.shutdownHook,
	}

	// register services
	for _, service := range sb.services {
		service.RegisterService(srv)
	}

	// set reflection when enable
	if sb.enableReflection {
		reflection.Register(srv)
	}

	// register health check if enabled
	if sb.enableHealthCheck {
		grpc_health_v1.RegisterHealthServer(srv, health.NewServer())
	}

	// register metrics if enabled
	if sb.metricsEnabled {
		grpcServer.metricsExporter = prometheus.NewPromServer(sb.metricsPort)
	}

	// register tracing if enabled
	if sb.tracingEnabled {
		if sb.traceURL == "" {
			return nil, errMissingTraceURL
		}

		if sb.serviceName == "" {
			return nil, errMissingServiceName
		}
		grpcServer.traceProvider = traces.NewProvider(sb.traceURL, sb.serviceName)
	}

	// set isBuild
	sb.isBuilt = true

	return grpcServer, nil
}

// DefaultServerBuilder loads the grpc standard configuration grpcserver.GrpcConfig and
// returns a grpcserver.ServerBuilder
func DefaultServerBuilder(config *GrpcConfig) *Builder {
	// build the grpc server
	return NewServerBuilder().
		WithReflection(config.EnableReflection).
		WithDefaultUnaryInterceptors().
		WithDefaultStreamInterceptors().
		WithTracingEnabled(config.TraceEnabled).
		WithTraceURL(config.TraceURL).
		WithServiceName(config.ServiceName).
		WithPort(config.GrpcPort).
		WithMetricsEnabled(config.MetricsEnabled).
		WithMetricsPort(config.MetricsPort)
}
