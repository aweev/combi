package trace

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Provider struct {
	tp *sdktrace.TracerProvider
}

type ProviderConfig struct {
	ServiceName string
	Environment string
	Endpoint    string
	Disabled    bool
}

func NewProvider(cfg ProviderConfig) (*Provider, error) {
	if cfg.Disabled {
		provider := sdktrace.NewTracerProvider()
		otel.SetTracerProvider(provider)
		return &Provider{tp: provider}, nil
	}

	var exporter sdktrace.SpanExporter
	var err error

	if cfg.Environment == "development" {
		exporter, err = newStdoutExporter()
	} else {
		exporter, err = newOtlpExporter(context.Background(), cfg.Endpoint)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	// Create resource without semconv to avoid version conflicts
	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			attribute.String("service.name", cfg.ServiceName),
			attribute.String("service.version", "v0.1.0"),
			attribute.String("deployment.environment", cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create otel resource: %w", err)
	}

	sampler := sdktrace.AlwaysSample()
	if cfg.Environment != "development" {
		sampler = sdktrace.ParentBased(sdktrace.TraceIDRatioBased(0.1))
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	return &Provider{tp: tp}, nil
}

func (p *Provider) Shutdown(ctx context.Context) error {
	return p.tp.Shutdown(ctx)
}

func newStdoutExporter() (sdktrace.SpanExporter, error) {
	return stdouttrace.New(stdouttrace.WithPrettyPrint())
}

func newOtlpExporter(ctx context.Context, endpoint string) (sdktrace.SpanExporter, error) {
	if endpoint == "" {
		return nil, fmt.Errorf("OTLP exporter endpoint is required for non-development environments")
	}

	// FIX: Replaced Deprecated grpc.DialContext with grpc.NewClient
	// WithBlock is also deprecated/removed in NewClient as connection happens in background.
	conn, err := grpc.NewClient(endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection to OTLP collector at %s: %w", endpoint, err)
	}

	return otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
}

func (p *Provider) TracerProvider() *sdktrace.TracerProvider {
	return p.tp
}
