package traces

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
)

// Provider is a wrapper around the open telemetry tracer.Provider
// It helps initialize an OTLP exporter, and configures the corresponding trace provider
type Provider struct {
	serviceName      string
	exporterEndpoint string

	tracerProvider *sdktrace.TracerProvider
}

// NewProvider creates a new instance of Provider
func NewProvider(exporterEndPoint, serviceName string) *Provider {
	return &Provider{
		serviceName:      serviceName,
		exporterEndpoint: exporterEndPoint,
	}
}

// Register initializes an OTLP exporter, and configures the corresponding trace provider
func (p *Provider) Register(ctx context.Context) error {
	res, err := resource.New(ctx,
		resource.WithHost(),
		resource.WithProcess(),
		resource.WithTelemetrySDK(),
		resource.WithAttributes(
			// the service name used to display traces in backends
			semconv.ServiceNameKey.String(p.serviceName),
		),
	)
	if err != nil {
		return err
	}

	// Set up a trace exporter
	traceExporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithEndpoint(p.exporterEndpoint),
	)

	if err != nil {
		return err
	}

	// Register the trace exporter with a Provider, using a batch
	// span processor to aggregate spans before export.
	bsp := sdktrace.NewBatchSpanProcessor(traceExporter)
	p.tracerProvider = sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)
	otel.SetTracerProvider(p.tracerProvider)

	// set global propagator to tracecontext (the default is no-op).
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	return nil
}

// Deregister will flush any remaining spans and shut down the exporter.
func (p *Provider) Deregister(ctx context.Context) error {
	return p.tracerProvider.Shutdown(ctx)
}
