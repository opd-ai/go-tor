package trace

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestPropagationNestedSpans(t *testing.T) {
	tracer := NewTracer("test", NewNoopExporter(), AlwaysSample())
	ctx := context.Background()

	ctx, parent := tracer.StartSpan(ctx, "parent", SpanKindClient)
	ctx, child := tracer.StartSpan(ctx, "child", SpanKindInternal)
	_, grandchild := tracer.StartSpan(ctx, "grandchild", SpanKindInternal)

	if child.TraceID != parent.TraceID {
		t.Fatalf("child TraceID mismatch: got %s, want %s", child.TraceID, parent.TraceID)
	}
	if grandchild.TraceID != parent.TraceID {
		t.Fatalf("grandchild TraceID mismatch: got %s, want %s", grandchild.TraceID, parent.TraceID)
	}
	if child.ParentID != parent.SpanID {
		t.Fatalf("child ParentID: got %s, want %s", child.ParentID, parent.SpanID)
	}
	if grandchild.ParentID != child.SpanID {
		t.Fatalf("grandchild ParentID: got %s, want %s", grandchild.ParentID, child.SpanID)
	}
}

func TestPropagationAttributeIsolation(t *testing.T) {
	tracer := NewTracer("test", NewNoopExporter(), AlwaysSample())
	ctx := context.Background()

	ctx, parent := tracer.StartSpan(ctx, "parent", SpanKindClient)
	parent.SetAttribute("parent.key", "parent-value")

	_, child := tracer.StartSpan(ctx, "child", SpanKindInternal)

	if _, ok := child.Attributes["parent.key"]; ok {
		t.Fatal("parent attribute leaked to child span")
	}
	if v := parent.Attributes["parent.key"]; v != "parent-value" {
		t.Fatalf("parent attribute missing: got %v", v)
	}
}

func TestPropagationErrorInWithSpan(t *testing.T) {
	tracer := NewTracer("test", NewNoopExporter(), AlwaysSample())
	ctx := context.Background()
	expected := errors.New("inner error")

	err := WithSpan(ctx, tracer, "outer", SpanKindClient, func(ctx context.Context, _ *Span) error {
		return WithSpan(ctx, tracer, "inner", SpanKindInternal, func(_ context.Context, _ *Span) error {
			return expected
		})
	})

	if !errors.Is(err, expected) {
		t.Fatalf("error not propagated: got %v, want %v", err, expected)
	}
}

func TestPropagationConcurrentChildSpans(t *testing.T) {
	tracer := NewTracer("test", NewNoopExporter(), AlwaysSample())
	ctx := context.Background()
	ctx, parent := tracer.StartSpan(ctx, "parent", SpanKindClient)

	const numGoroutines = 20
	var wg sync.WaitGroup
	spans := make([]*Span, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, s := tracer.StartSpan(ctx, "child", SpanKindInternal)
			s.SetAttribute("idx", idx)
			s.End()
			spans[idx] = s
		}(i)
	}
	wg.Wait()

	for i, s := range spans {
		if s.TraceID != parent.TraceID {
			t.Errorf("span %d: TraceID mismatch", i)
		}
		if s.ParentID != parent.SpanID {
			t.Errorf("span %d: ParentID mismatch", i)
		}
	}
}

func TestPropagationContextCancellation(t *testing.T) {
	tracer := NewTracer("test", NewNoopExporter(), AlwaysSample())
	ctx, cancel := context.WithCancel(context.Background())

	_, span := tracer.StartSpan(ctx, "op", SpanKindInternal)
	cancel()
	span.SetStatus(StatusCancelled, "context cancelled")
	span.End()

	if span.Status != StatusCancelled {
		t.Fatalf("expected cancelled status, got %s", span.Status)
	}
	if span.EndTime.IsZero() {
		t.Fatal("span EndTime should be set after End()")
	}
}

func TestPropagationMultipleExporters(t *testing.T) {
	exp1 := &countingExporter{}
	exp2 := &countingExporter{}
	multi := NewMultiExporter(exp1, exp2)
	tracer := NewTracer("test", multi, AlwaysSample())
	ctx := context.Background()

	err := WithSpan(ctx, tracer, "op", SpanKindClient, func(_ context.Context, _ *Span) error {
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exp1.count != 1 {
		t.Fatalf("exp1 count: got %d, want 1", exp1.count)
	}
	if exp2.count != 1 {
		t.Fatalf("exp2 count: got %d, want 1", exp2.count)
	}
}

func TestPropagationSamplerDecision(t *testing.T) {
	tracer := NewTracer("test", NewNoopExporter(), AlwaysSample())
	ctx := context.Background()

	ctx, parent := tracer.StartSpan(ctx, "parent", SpanKindClient)
	_, child := tracer.StartSpan(ctx, "child", SpanKindInternal)

	if parent == nil {
		t.Fatal("parent should be sampled")
	}
	if child == nil {
		t.Fatal("child should be sampled when parent is sampled")
	}
	if child.TraceID != parent.TraceID {
		t.Fatal("sampled child must share parent TraceID")
	}
}

func TestPropagationFromContextNoSpan(t *testing.T) {
	ctx := context.Background()
	span := FromContext(ctx)

	if span != nil {
		t.Fatal("expected nil span from empty context")
	}

	// Also test with nil context
	span = FromContext(nil)
	if span != nil {
		t.Fatal("expected nil span from nil context")
	}
}

func TestPropagationSpanLifecycleOrdering(t *testing.T) {
	tracer := NewTracer("test", NewNoopExporter(), AlwaysSample())
	ctx := context.Background()

	_, span := tracer.StartSpan(ctx, "lifecycle", SpanKindInternal)
	span.AddEvent("event1", map[string]interface{}{"k": "v"})
	span.SetAttribute("attr", "value")
	span.RecordError(errors.New("test error"))
	span.End()

	if span.Status != StatusError {
		t.Fatalf("expected error status, got %s", span.Status)
	}
	if len(span.Events) < 1 {
		t.Fatal("expected at least one event")
	}
	if span.Attributes["attr"] != "value" {
		t.Fatal("attribute not set")
	}
}

func TestPropagationEndSpanIdempotency(t *testing.T) {
	tracer := NewTracer("test", NewNoopExporter(), AlwaysSample())
	ctx := context.Background()

	_, span := tracer.StartSpan(ctx, "op", SpanKindInternal)
	span.End()
	firstEnd := span.EndTime

	// Calling End() again should not panic
	span.End()
	if span.EndTime.Before(firstEnd) {
		t.Fatal("EndTime should not go backwards")
	}

	// End on nil span should not panic
	var nilSpan *Span
	nilSpan.End()
}

// countingExporter counts exported spans for test assertions.
type countingExporter struct {
	mu    sync.Mutex
	count int
}

func (e *countingExporter) Export(_ *Span) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.count++
	return nil
}

func (e *countingExporter) Close() error {
	return nil
}
