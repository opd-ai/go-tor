// Package trace provides distributed tracing and observability for the Tor client.
// This file adds OpenTelemetry SDK integration for exporting traces to standard
// backends like Jaeger, Zipkin, and OTLP-compatible collectors.
package trace

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// TracingConfig holds configuration for OpenTelemetry tracing.
type TracingConfig struct {
	// Enabled enables or disables tracing
	Enabled bool
	// ServiceName is the name of the service for trace attribution
	ServiceName string
	// Endpoint is the collector endpoint (e.g., "localhost:4317" for OTLP/gRPC)
	Endpoint string
	// SampleRate is the sampling rate (0.0 to 1.0)
	SampleRate float64
	// Exporter is the exporter type: "otlp", "stdout", or "noop"
	Exporter string
	// Insecure disables TLS for the OTLP exporter
	Insecure bool
	// Timeout is the export timeout duration
	Timeout time.Duration
}

// DefaultTracingConfig returns a TracingConfig with sensible defaults.
func DefaultTracingConfig() TracingConfig {
	return TracingConfig{
		Enabled:     false,
		ServiceName: "go-tor",
		Endpoint:    "localhost:4317",
		SampleRate:  1.0,
		Exporter:    "noop",
		Insecure:    false,
		Timeout:     10 * time.Second,
	}
}

// OTelProvider wraps an OpenTelemetry TracerProvider for lifecycle management.
type OTelProvider struct {
	provider *sdktrace.TracerProvider
	tracer   oteltrace.Tracer
}

// InitOTelTracer initializes OpenTelemetry tracing with the given configuration.
// It returns an OTelProvider that must be closed when the application shuts down.
func InitOTelTracer(cfg TracingConfig) (*OTelProvider, error) {
	if !cfg.Enabled {
		// Return a noop provider when tracing is disabled
		return &OTelProvider{
			provider: nil,
			tracer:   oteltrace.NewNoopTracerProvider().Tracer(cfg.ServiceName),
		}, nil
	}

	ctx := context.Background()

	// Create resource with service information
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion("0.9.12"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create exporter based on configuration
	var exporter sdktrace.SpanExporter
	switch cfg.Exporter {
	case "otlp":
		opts := []otlptracegrpc.Option{
			otlptracegrpc.WithEndpoint(cfg.Endpoint),
			otlptracegrpc.WithTimeout(cfg.Timeout),
		}
		if cfg.Insecure {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
		exporter, err = otlptracegrpc.New(ctx, opts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
		}
	case "stdout":
		exporter, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return nil, fmt.Errorf("failed to create stdout exporter: %w", err)
		}
	case "noop":
		// Use a noop exporter that discards all spans
		exporter = &noopSpanExporter{}
	default:
		return nil, fmt.Errorf("unknown exporter type: %s", cfg.Exporter)
	}

	// Create sampler based on sample rate
	var sampler sdktrace.Sampler
	switch {
	case cfg.SampleRate <= 0:
		sampler = sdktrace.NeverSample()
	case cfg.SampleRate >= 1:
		sampler = sdktrace.AlwaysSample()
	default:
		sampler = sdktrace.TraceIDRatioBased(cfg.SampleRate)
	}

	// Create TracerProvider
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	// Set as global provider
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return &OTelProvider{
		provider: provider,
		tracer:   provider.Tracer(cfg.ServiceName),
	}, nil
}

// Tracer returns the OpenTelemetry tracer for creating spans.
func (p *OTelProvider) Tracer() oteltrace.Tracer {
	return p.tracer
}

// Shutdown gracefully shuts down the tracer provider, flushing any remaining spans.
func (p *OTelProvider) Shutdown(ctx context.Context) error {
	if p.provider == nil {
		return nil
	}
	return p.provider.Shutdown(ctx)
}

// StartSpan creates a new span using OpenTelemetry.
// This is a convenience method that wraps the underlying OTel tracer.
func (p *OTelProvider) StartSpan(ctx context.Context, name string, opts ...oteltrace.SpanStartOption) (context.Context, oteltrace.Span) {
	return p.tracer.Start(ctx, name, opts...)
}

// noopSpanExporter implements sdktrace.SpanExporter but discards all spans.
type noopSpanExporter struct{}

func (e *noopSpanExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	return nil
}

func (e *noopSpanExporter) Shutdown(ctx context.Context) error {
	return nil
}

// OTelExporter wraps an OpenTelemetry TracerProvider to implement our Exporter interface.
// This allows using the existing trace infrastructure with OpenTelemetry backends.
type OTelExporter struct {
	tracer   oteltrace.Tracer
	provider *OTelProvider
}

// NewOTelExporter creates a new OTelExporter that bridges our Span type to OpenTelemetry.
func NewOTelExporter(provider *OTelProvider) *OTelExporter {
	return &OTelExporter{
		tracer:   provider.tracer,
		provider: provider,
	}
}

// Export converts our Span to an OpenTelemetry span and exports it.
// Note: This creates a synthetic span for compatibility. For best performance,
// use the OTelProvider.StartSpan method directly with OpenTelemetry's native API.
func (e *OTelExporter) Export(span *Span) error {
	if span == nil {
		return nil
	}

	ctx := context.Background()

	// Create a synthetic span with the recorded timing
	_, otelSpan := e.tracer.Start(ctx, span.Name,
		oteltrace.WithTimestamp(span.StartTime),
	)

	// Set attributes
	attrs := make([]attribute.KeyValue, 0, len(span.Attributes))
	for k, v := range span.Attributes {
		switch val := v.(type) {
		case string:
			attrs = append(attrs, attribute.String(k, val))
		case int:
			attrs = append(attrs, attribute.Int(k, val))
		case int64:
			attrs = append(attrs, attribute.Int64(k, val))
		case float64:
			attrs = append(attrs, attribute.Float64(k, val))
		case bool:
			attrs = append(attrs, attribute.Bool(k, val))
		default:
			attrs = append(attrs, attribute.String(k, fmt.Sprintf("%v", v)))
		}
	}
	otelSpan.SetAttributes(attrs...)

	// Add events
	for _, event := range span.Events {
		eventAttrs := make([]attribute.KeyValue, 0, len(event.Attributes))
		for k, v := range event.Attributes {
			eventAttrs = append(eventAttrs, attribute.String(k, fmt.Sprintf("%v", v)))
		}
		otelSpan.AddEvent(event.Name,
			oteltrace.WithTimestamp(event.Timestamp),
			oteltrace.WithAttributes(eventAttrs...),
		)
	}

	// Set status
	if span.Status == StatusError {
		otelSpan.SetStatus(2, "") // 2 = Error in OTel codes
	}

	// End the span
	otelSpan.End(oteltrace.WithTimestamp(span.EndTime))

	return nil
}

// Close shuts down the underlying provider.
func (e *OTelExporter) Close() error {
	return e.provider.Shutdown(context.Background())
}
