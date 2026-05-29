package telemetry

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	stdouttrace "go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Config controls metrics and tracing setup.
type Config struct {
	ServiceName     string
	ServiceVersion  string
	MetricsEnabled  bool
	TracingEnabled  bool
	TracingExporter string
}

// Provider owns telemetry resources that need shutdown.
type Provider struct {
	Registry *prometheus.Registry
	shutdown func(context.Context) error
}

// New initializes M0 telemetry.
func New(ctx context.Context, cfg Config) (*Provider, error) {
	registry := prometheus.NewRegistry()
	info := prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "token_gateway_info",
		Help:        "Token Gateway build information.",
		ConstLabels: prometheus.Labels{"service": cfg.ServiceName, "version": cfg.ServiceVersion},
	})
	info.Set(1)
	if err := registry.Register(info); err != nil {
		return nil, err
	}

	provider := &Provider{
		Registry: registry,
		shutdown: func(context.Context) error {
			return nil
		},
	}
	if !cfg.TracingEnabled {
		return provider, nil
	}

	tp, err := newTracerProvider(ctx, cfg)
	if err != nil {
		return nil, err
	}
	otel.SetTracerProvider(tp)
	provider.shutdown = tp.Shutdown
	return provider, nil
}

// Shutdown flushes tracing resources.
func (p *Provider) Shutdown(ctx context.Context) error {
	if p == nil || p.shutdown == nil {
		return nil
	}
	return p.shutdown(ctx)
}

func newTracerProvider(_ context.Context, cfg Config) (*sdktrace.TracerProvider, error) {
	res := resource.NewSchemaless(
		attribute.String("service.name", cfg.ServiceName),
		attribute.String("service.version", cfg.ServiceVersion),
	)
	switch cfg.TracingExporter {
	case "", "noop":
		return sdktrace.NewTracerProvider(sdktrace.WithResource(res)), nil
	case "stdout":
		exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return nil, err
		}
		return sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(exporter),
			sdktrace.WithResource(res),
		), nil
	default:
		return nil, fmt.Errorf("unsupported tracing exporter %q", cfg.TracingExporter)
	}
}
