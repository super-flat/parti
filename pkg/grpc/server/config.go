package server

// GrpcConfig holds the grpc service common
// settings used across service
type GrpcConfig struct {
	ServiceName      string // ServiceName is the name given that will show in the traces
	GrpcPort         int    // GrpcPort is the gRPC port used to received and handle gRPC requests
	MetricsEnabled   bool   // MetricsEnabled checks whether metrics should be enabled or not
	MetricsPort      int    // MetricsPort is used to send gRPC server metrics to the prometheus server
	TraceEnabled     bool   // TraceEnabled checks whether tracing should be enabled or not
	TraceURL         string // TraceURL is the OTLP collector url.
	EnableReflection bool   // EnableReflection this is useful or local dev testing
}
