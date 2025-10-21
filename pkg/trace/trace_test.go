package trace

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewTracer(t *testing.T) {
	exporter := NewNoopExporter()
	sampler := AlwaysSample()

	tracer := NewTracer("test-service", exporter, sampler)
	if tracer == nil {
		t.Fatal("Expected tracer to be created")
	}

	if tracer.serviceName != "test-service" {
		t.Errorf("Expected service name 'test-service', got '%s'", tracer.serviceName)
	}
}

func TestStartSpan(t *testing.T) {
	tracer := NewTracer("test-service", NewNoopExporter(), AlwaysSample())
	ctx := context.Background()

	ctx, span := tracer.StartSpan(ctx, "test-operation", SpanKindInternal)
	if span == nil {
		t.Fatal("Expected span to be created")
	}

	if span.Name != "test-operation" {
		t.Errorf("Expected span name 'test-operation', got '%s'", span.Name)
	}

	if span.Kind != SpanKindInternal {
		t.Errorf("Expected span kind 'internal', got '%s'", span.Kind)
	}

	if span.TraceID == "" {
		t.Error("Expected trace ID to be set")
	}

	if span.SpanID == "" {
		t.Error("Expected span ID to be set")
	}

	serviceName, ok := span.Attributes["service.name"]
	if !ok || serviceName != "test-service" {
		t.Error("Expected service.name attribute to be set")
	}
}

func TestSpanHierarchy(t *testing.T) {
	tracer := NewTracer("test-service", NewNoopExporter(), AlwaysSample())
	ctx := context.Background()

	// Create parent span
	ctx, parentSpan := tracer.StartSpan(ctx, "parent-operation", SpanKindInternal)
	if parentSpan == nil {
		t.Fatal("Expected parent span to be created")
	}

	// Create child span
	ctx, childSpan := tracer.StartSpan(ctx, "child-operation", SpanKindInternal)
	if childSpan == nil {
		t.Fatal("Expected child span to be created")
	}

	// Verify hierarchy
	if childSpan.ParentID != parentSpan.SpanID {
		t.Errorf("Expected child parent ID to match parent span ID")
	}

	if childSpan.TraceID != parentSpan.TraceID {
		t.Errorf("Expected child trace ID to match parent trace ID")
	}
}

func TestSpanEnd(t *testing.T) {
	tracer := NewTracer("test-service", NewNoopExporter(), AlwaysSample())
	ctx := context.Background()

	ctx, span := tracer.StartSpan(ctx, "test-operation", SpanKindInternal)
	if span == nil {
		t.Fatal("Expected span to be created")
	}

	startTime := span.StartTime

	// Wait a bit to ensure duration is measurable
	time.Sleep(10 * time.Millisecond)

	span.End()

	if span.EndTime.IsZero() {
		t.Error("Expected end time to be set")
	}

	if span.Duration == 0 {
		t.Error("Expected duration to be non-zero")
	}

	if !span.EndTime.After(startTime) {
		t.Error("Expected end time to be after start time")
	}
}

func TestSetStatus(t *testing.T) {
	tracer := NewTracer("test-service", NewNoopExporter(), AlwaysSample())
	ctx := context.Background()

	ctx, span := tracer.StartSpan(ctx, "test-operation", SpanKindInternal)
	if span == nil {
		t.Fatal("Expected span to be created")
	}

	span.SetStatus(StatusError, "something went wrong")

	if span.Status != StatusError {
		t.Errorf("Expected status 'error', got '%s'", span.Status)
	}

	desc, ok := span.Attributes["status.description"]
	if !ok || desc != "something went wrong" {
		t.Error("Expected status description to be set")
	}
}

func TestSetAttribute(t *testing.T) {
	tracer := NewTracer("test-service", NewNoopExporter(), AlwaysSample())
	ctx := context.Background()

	ctx, span := tracer.StartSpan(ctx, "test-operation", SpanKindInternal)
	if span == nil {
		t.Fatal("Expected span to be created")
	}

	span.SetAttribute("test.key", "test.value")

	value, ok := span.Attributes["test.key"]
	if !ok || value != "test.value" {
		t.Error("Expected attribute to be set")
	}
}

func TestSetAttributes(t *testing.T) {
	tracer := NewTracer("test-service", NewNoopExporter(), AlwaysSample())
	ctx := context.Background()

	ctx, span := tracer.StartSpan(ctx, "test-operation", SpanKindInternal)
	if span == nil {
		t.Fatal("Expected span to be created")
	}

	attrs := map[string]interface{}{
		"key1": "value1",
		"key2": 42,
		"key3": true,
	}

	span.SetAttributes(attrs)

	for key, expectedValue := range attrs {
		value, ok := span.Attributes[key]
		if !ok {
			t.Errorf("Expected attribute '%s' to be set", key)
		}
		if value != expectedValue {
			t.Errorf("Expected attribute '%s' to have value '%v', got '%v'", key, expectedValue, value)
		}
	}
}

func TestAddEvent(t *testing.T) {
	tracer := NewTracer("test-service", NewNoopExporter(), AlwaysSample())
	ctx := context.Background()

	ctx, span := tracer.StartSpan(ctx, "test-operation", SpanKindInternal)
	if span == nil {
		t.Fatal("Expected span to be created")
	}

	span.AddEvent("test-event", map[string]interface{}{
		"event.key": "event.value",
	})

	if len(span.Events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(span.Events))
	}

	event := span.Events[0]
	if event.Name != "test-event" {
		t.Errorf("Expected event name 'test-event', got '%s'", event.Name)
	}

	value, ok := event.Attributes["event.key"]
	if !ok || value != "event.value" {
		t.Error("Expected event attribute to be set")
	}
}

func TestRecordError(t *testing.T) {
	tracer := NewTracer("test-service", NewNoopExporter(), AlwaysSample())
	ctx := context.Background()

	ctx, span := tracer.StartSpan(ctx, "test-operation", SpanKindInternal)
	if span == nil {
		t.Fatal("Expected span to be created")
	}

	testErr := errors.New("test error")
	span.RecordError(testErr)

	if span.Status != StatusError {
		t.Errorf("Expected status 'error', got '%s'", span.Status)
	}

	if len(span.Events) != 1 {
		t.Fatalf("Expected 1 error event, got %d", len(span.Events))
	}

	event := span.Events[0]
	if event.Name != "error" {
		t.Errorf("Expected event name 'error', got '%s'", event.Name)
	}

	errMsg, ok := event.Attributes["error.message"]
	if !ok || errMsg != "test error" {
		t.Error("Expected error message to be recorded")
	}
}

func TestFromContext(t *testing.T) {
	tracer := NewTracer("test-service", NewNoopExporter(), AlwaysSample())
	ctx := context.Background()

	ctx, span := tracer.StartSpan(ctx, "test-operation", SpanKindInternal)
	if span == nil {
		t.Fatal("Expected span to be created")
	}

	retrievedSpan := FromContext(ctx)
	if retrievedSpan == nil {
		t.Fatal("Expected to retrieve span from context")
	}

	if retrievedSpan.SpanID != span.SpanID {
		t.Error("Retrieved span does not match original span")
	}
}

func TestFromContextNil(t *testing.T) {
	span := FromContext(nil)
	if span != nil {
		t.Error("Expected nil span from nil context")
	}

	ctx := context.Background()
	span = FromContext(ctx)
	if span != nil {
		t.Error("Expected nil span from context without span")
	}
}

func TestSpanNilSafety(t *testing.T) {
	// Test that all span methods are nil-safe
	var span *Span

	// Should not panic
	span.End()
	span.SetStatus(StatusError, "test")
	span.SetAttribute("key", "value")
	span.SetAttributes(map[string]interface{}{"key": "value"})
	span.AddEvent("event", nil)
	span.RecordError(errors.New("test"))
}

func TestWithSpan(t *testing.T) {
	tracer := NewTracer("test-service", NewNoopExporter(), AlwaysSample())
	ctx := context.Background()

	called := false
	err := WithSpan(ctx, tracer, "test-operation", SpanKindInternal, func(ctx context.Context, span *Span) error {
		called = true
		if span == nil {
			t.Error("Expected span to be passed to function")
		}
		span.SetAttribute("test.key", "test.value")
		return nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !called {
		t.Error("Expected function to be called")
	}
}

func TestWithSpanError(t *testing.T) {
	tracer := NewTracer("test-service", NewNoopExporter(), AlwaysSample())
	ctx := context.Background()

	testErr := errors.New("test error")
	err := WithSpan(ctx, tracer, "test-operation", SpanKindInternal, func(ctx context.Context, span *Span) error {
		return testErr
	})

	if err != testErr {
		t.Errorf("Expected error to be returned")
	}
}

func TestEndSpanHelper(t *testing.T) {
	tracer := NewTracer("test-service", NewNoopExporter(), AlwaysSample())
	ctx := context.Background()

	ctx, span := tracer.StartSpan(ctx, "test-operation", SpanKindInternal)
	if span == nil {
		t.Fatal("Expected span to be created")
	}

	testErr := errors.New("test error")
	EndSpan(span, testErr, tracer.exporter)

	if span.Status != StatusError {
		t.Errorf("Expected status 'error', got '%s'", span.Status)
	}

	if span.EndTime.IsZero() {
		t.Error("Expected end time to be set")
	}
}

func TestEndSpanHelperNil(t *testing.T) {
	// Should not panic
	EndSpan(nil, nil, nil)
	EndSpan(nil, errors.New("test"), nil)
}
