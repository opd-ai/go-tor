package trace

import (
	"bytes"
	"context"
	"testing"
	"time"
)

func TestDefaultTracingConfig(t *testing.T) {
	cfg := DefaultTracingConfig()

	if cfg.Enabled {
		t.Error("Expected Enabled to be false by default")
	}
	if cfg.ServiceName != "go-tor" {
		t.Errorf("Expected ServiceName 'go-tor', got '%s'", cfg.ServiceName)
	}
	if cfg.Endpoint != "localhost:4317" {
		t.Errorf("Expected Endpoint 'localhost:4317', got '%s'", cfg.Endpoint)
	}
	if cfg.SampleRate != 1.0 {
		t.Errorf("Expected SampleRate 1.0, got %f", cfg.SampleRate)
	}
	if cfg.Exporter != "noop" {
		t.Errorf("Expected Exporter 'noop', got '%s'", cfg.Exporter)
	}
	if cfg.Insecure {
		t.Error("Expected Insecure to be false by default")
	}
	if cfg.Timeout != 10*time.Second {
		t.Errorf("Expected Timeout 10s, got %v", cfg.Timeout)
	}
}

func TestInitOTelTracerDisabled(t *testing.T) {
	cfg := DefaultTracingConfig()
	cfg.Enabled = false

	provider, err := InitOTelTracer(cfg)
	if err != nil {
		t.Fatalf("InitOTelTracer failed: %v", err)
	}

	if provider == nil {
		t.Fatal("Expected non-nil provider")
	}

	if provider.provider != nil {
		t.Error("Expected nil internal provider when disabled")
	}

	// Tracer should still work (noop)
	tracer := provider.Tracer()
	if tracer == nil {
		t.Error("Expected non-nil tracer")
	}

	// Shutdown should succeed
	err = provider.Shutdown(context.Background())
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
}

func TestInitOTelTracerNoop(t *testing.T) {
	cfg := DefaultTracingConfig()
	cfg.Enabled = true
	cfg.Exporter = "noop"

	provider, err := InitOTelTracer(cfg)
	if err != nil {
		t.Fatalf("InitOTelTracer failed: %v", err)
	}
	defer func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Errorf("Shutdown failed: %v", err)
		}
	}()

	if provider == nil {
		t.Fatal("Expected non-nil provider")
	}

	if provider.provider == nil {
		t.Error("Expected non-nil internal provider when enabled")
	}

	// Test creating a span
	ctx := context.Background()
	ctx, span := provider.StartSpan(ctx, "test-operation")
	if span == nil {
		t.Error("Expected non-nil span")
	}
	span.End()
}

func TestInitOTelTracerStdout(t *testing.T) {
	cfg := DefaultTracingConfig()
	cfg.Enabled = true
	cfg.Exporter = "stdout"

	provider, err := InitOTelTracer(cfg)
	if err != nil {
		t.Fatalf("InitOTelTracer failed: %v", err)
	}
	defer func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Errorf("Shutdown failed: %v", err)
		}
	}()

	if provider == nil {
		t.Fatal("Expected non-nil provider")
	}

	// Create and end a span
	ctx := context.Background()
	ctx, span := provider.StartSpan(ctx, "stdout-test-operation")
	span.End()
}

func TestInitOTelTracerUnknownExporter(t *testing.T) {
	cfg := DefaultTracingConfig()
	cfg.Enabled = true
	cfg.Exporter = "unknown"

	_, err := InitOTelTracer(cfg)
	if err == nil {
		t.Error("Expected error for unknown exporter")
	}
}

func TestInitOTelTracerSamplerConfigurations(t *testing.T) {
	tests := []struct {
		name       string
		sampleRate float64
	}{
		{"never_sample", 0.0},
		{"always_sample", 1.0},
		{"half_sample", 0.5},
		{"negative_rate", -0.1},
		{"over_one_rate", 1.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultTracingConfig()
			cfg.Enabled = true
			cfg.Exporter = "noop"
			cfg.SampleRate = tt.sampleRate

			provider, err := InitOTelTracer(cfg)
			if err != nil {
				t.Fatalf("InitOTelTracer failed: %v", err)
			}
			defer provider.Shutdown(context.Background())

			if provider == nil {
				t.Fatal("Expected non-nil provider")
			}
		})
	}
}

func TestOTelProviderTracer(t *testing.T) {
	cfg := DefaultTracingConfig()
	cfg.Enabled = true
	cfg.Exporter = "noop"

	provider, err := InitOTelTracer(cfg)
	if err != nil {
		t.Fatalf("InitOTelTracer failed: %v", err)
	}
	defer provider.Shutdown(context.Background())

	tracer := provider.Tracer()
	if tracer == nil {
		t.Error("Expected non-nil tracer")
	}

	// Use the tracer to create nested spans
	ctx := context.Background()
	ctx, parentSpan := tracer.Start(ctx, "parent-operation")
	_, childSpan := tracer.Start(ctx, "child-operation")

	childSpan.End()
	parentSpan.End()
}

func TestNoopSpanExporter(t *testing.T) {
	exporter := &noopSpanExporter{}

	err := exporter.ExportSpans(context.Background(), nil)
	if err != nil {
		t.Errorf("ExportSpans should not error: %v", err)
	}

	err = exporter.Shutdown(context.Background())
	if err != nil {
		t.Errorf("Shutdown should not error: %v", err)
	}
}

func TestNewOTelExporter(t *testing.T) {
	cfg := DefaultTracingConfig()
	cfg.Enabled = true
	cfg.Exporter = "noop"

	provider, err := InitOTelTracer(cfg)
	if err != nil {
		t.Fatalf("InitOTelTracer failed: %v", err)
	}
	defer provider.Shutdown(context.Background())

	exporter := NewOTelExporter(provider)
	if exporter == nil {
		t.Fatal("Expected non-nil exporter")
	}
}

func TestOTelExporterExport(t *testing.T) {
	cfg := DefaultTracingConfig()
	cfg.Enabled = true
	cfg.Exporter = "noop"

	provider, err := InitOTelTracer(cfg)
	if err != nil {
		t.Fatalf("InitOTelTracer failed: %v", err)
	}
	defer provider.Shutdown(context.Background())

	exporter := NewOTelExporter(provider)

	// Create a span using our internal Span type
	span := &Span{
		TraceID:   "trace-123",
		SpanID:    "span-123",
		ParentID:  "",
		Name:      "test-operation",
		Kind:      SpanKindInternal,
		StartTime: time.Now(),
		Status:    StatusOK,
		Attributes: map[string]interface{}{
			"string-attr":  "value",
			"int-attr":     42,
			"int64-attr":   int64(123),
			"float-attr":   3.14,
			"bool-attr":    true,
			"unknown-attr": bytes.NewBuffer(nil), // Should fallback to string
		},
		Events: []Event{
			{
				Timestamp: time.Now(),
				Name:      "test-event",
				Attributes: map[string]interface{}{
					"event-key": "event-value",
				},
			},
		},
	}
	span.EndTime = span.StartTime.Add(100 * time.Millisecond)

	err = exporter.Export(span)
	if err != nil {
		t.Errorf("Export should not error: %v", err)
	}
}

func TestOTelExporterExportWithError(t *testing.T) {
	cfg := DefaultTracingConfig()
	cfg.Enabled = true
	cfg.Exporter = "noop"

	provider, err := InitOTelTracer(cfg)
	if err != nil {
		t.Fatalf("InitOTelTracer failed: %v", err)
	}
	defer provider.Shutdown(context.Background())

	exporter := NewOTelExporter(provider)

	// Create a span with error status
	span := &Span{
		TraceID:    "trace-123",
		SpanID:     "span-123",
		Name:       "error-operation",
		Kind:       SpanKindInternal,
		StartTime:  time.Now(),
		Status:     StatusError,
		Attributes: map[string]interface{}{},
		Events:     []Event{},
	}
	span.EndTime = span.StartTime.Add(50 * time.Millisecond)

	err = exporter.Export(span)
	if err != nil {
		t.Errorf("Export should not error: %v", err)
	}
}

func TestOTelExporterExportNilSpan(t *testing.T) {
	cfg := DefaultTracingConfig()
	cfg.Enabled = true
	cfg.Exporter = "noop"

	provider, err := InitOTelTracer(cfg)
	if err != nil {
		t.Fatalf("InitOTelTracer failed: %v", err)
	}
	defer provider.Shutdown(context.Background())

	exporter := NewOTelExporter(provider)

	err = exporter.Export(nil)
	if err != nil {
		t.Errorf("Export nil span should not error: %v", err)
	}
}

func TestOTelExporterClose(t *testing.T) {
	cfg := DefaultTracingConfig()
	cfg.Enabled = true
	cfg.Exporter = "noop"

	provider, err := InitOTelTracer(cfg)
	if err != nil {
		t.Fatalf("InitOTelTracer failed: %v", err)
	}

	exporter := NewOTelExporter(provider)

	err = exporter.Close()
	if err != nil {
		t.Errorf("Close should not error: %v", err)
	}
}

func TestOTelProviderShutdownNilProvider(t *testing.T) {
	cfg := DefaultTracingConfig()
	cfg.Enabled = false

	provider, err := InitOTelTracer(cfg)
	if err != nil {
		t.Fatalf("InitOTelTracer failed: %v", err)
	}

	// Provider is nil when disabled
	if provider.provider != nil {
		t.Error("Expected nil internal provider")
	}

	// Shutdown should succeed even with nil provider
	err = provider.Shutdown(context.Background())
	if err != nil {
		t.Errorf("Shutdown should not error with nil provider: %v", err)
	}
}

func TestOTelExporterExportWithCancelledStatus(t *testing.T) {
	cfg := DefaultTracingConfig()
	cfg.Enabled = true
	cfg.Exporter = "noop"
	cfg.SetGlobalProvider = false

	provider, err := InitOTelTracer(cfg)
	if err != nil {
		t.Fatalf("InitOTelTracer failed: %v", err)
	}
	defer provider.Shutdown(context.Background())

	exporter := NewOTelExporter(provider)

	// Create a span with cancelled status
	span := &Span{
		TraceID:    "trace-123",
		SpanID:     "span-123",
		Name:       "cancelled-operation",
		Kind:       SpanKindInternal,
		StartTime:  time.Now(),
		Status:     StatusCancelled,
		Attributes: map[string]interface{}{},
		Events:     []Event{},
	}
	span.EndTime = span.StartTime.Add(50 * time.Millisecond)

	err = exporter.Export(span)
	if err != nil {
		t.Errorf("Export should not error: %v", err)
	}
}

func TestOTelExporterExportSpanKinds(t *testing.T) {
	cfg := DefaultTracingConfig()
	cfg.Enabled = true
	cfg.Exporter = "noop"
	cfg.SetGlobalProvider = false

	provider, err := InitOTelTracer(cfg)
	if err != nil {
		t.Fatalf("InitOTelTracer failed: %v", err)
	}
	defer provider.Shutdown(context.Background())

	exporter := NewOTelExporter(provider)

	testCases := []struct {
		name string
		kind SpanKind
	}{
		{"client", SpanKindClient},
		{"server", SpanKindServer},
		{"internal", SpanKindInternal},
		{"empty", ""},
		{"unknown", SpanKind("unknown")},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			span := &Span{
				TraceID:    "trace-123",
				SpanID:     "span-123",
				Name:       "kind-test-operation",
				Kind:       tc.kind,
				StartTime:  time.Now(),
				Status:     StatusOK,
				Attributes: map[string]interface{}{},
				Events:     []Event{},
			}
			span.EndTime = span.StartTime.Add(50 * time.Millisecond)

			err := exporter.Export(span)
			if err != nil {
				t.Errorf("Export should not error for kind %s: %v", tc.name, err)
			}
		})
	}
}

func TestInitOTelTracerWithContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg := DefaultTracingConfig()
	cfg.Enabled = true
	cfg.Exporter = "noop"
	cfg.SetGlobalProvider = false

	provider, err := InitOTelTracerWithContext(ctx, cfg)
	if err != nil {
		t.Fatalf("InitOTelTracerWithContext failed: %v", err)
	}
	defer provider.Shutdown(context.Background())

	if provider == nil {
		t.Fatal("Expected non-nil provider")
	}

	if provider.provider == nil {
		t.Error("Expected non-nil internal provider when enabled")
	}
}

func TestTracingConfigSetGlobalProvider(t *testing.T) {
	// Test with SetGlobalProvider = false
	cfg := DefaultTracingConfig()
	cfg.Enabled = true
	cfg.Exporter = "noop"
	cfg.SetGlobalProvider = false

	provider, err := InitOTelTracer(cfg)
	if err != nil {
		t.Fatalf("InitOTelTracer failed: %v", err)
	}
	defer provider.Shutdown(context.Background())

	if provider == nil {
		t.Fatal("Expected non-nil provider")
	}

	// The provider should still be functional
	ctx := context.Background()
	ctx, span := provider.StartSpan(ctx, "test-operation")
	if span == nil {
		t.Error("Expected non-nil span")
	}
	span.End()
}

func TestDefaultTracingConfigSetGlobalProvider(t *testing.T) {
	cfg := DefaultTracingConfig()

	if !cfg.SetGlobalProvider {
		t.Error("Expected SetGlobalProvider to be true by default")
	}
}
