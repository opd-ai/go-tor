// Package main demonstrates distributed tracing capabilities in go-tor.
//
// This example shows how to:
// - Create a tracer with different exporters
// - Start and end spans
// - Create span hierarchies (parent-child relationships)
// - Add attributes and events to spans
// - Record errors
// - Use different sampling strategies
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/opd-ai/go-tor/pkg/trace"
)

func main() {
	fmt.Println("=== Distributed Tracing Demo ===")

	// Demo 1: Basic tracing with stdout exporter
	fmt.Println("Demo 1: Basic Tracing")
	fmt.Println("---------------------")
	demoBasicTracing()

	// Demo 2: Span hierarchy (parent-child)
	fmt.Println("\nDemo 2: Span Hierarchy")
	fmt.Println("----------------------")
	demoSpanHierarchy()

	// Demo 3: Error recording
	fmt.Println("\nDemo 3: Error Recording")
	fmt.Println("-----------------------")
	demoErrorRecording()

	// Demo 4: Events and attributes
	fmt.Println("\nDemo 4: Events and Attributes")
	fmt.Println("------------------------------")
	demoEventsAndAttributes()

	// Demo 5: Sampling strategies
	fmt.Println("\nDemo 5: Sampling Strategies")
	fmt.Println("---------------------------")
	demoSampling()

	// Demo 6: File export
	fmt.Println("\nDemo 6: File Export")
	fmt.Println("-------------------")
	demoFileExport()

	// Demo 7: Simulated circuit building
	fmt.Println("\nDemo 7: Simulated Circuit Building")
	fmt.Println("-----------------------------------")
	demoCircuitBuilding()

	fmt.Println("\n✓ All demos completed successfully")
}

func demoBasicTracing() {
	// Create a tracer with stdout exporter
	exporter := trace.NewStdoutExporter(true)
	sampler := trace.AlwaysSample()
	tracer := trace.NewTracer("demo-service", exporter, sampler)

	ctx := context.Background()

	// Start a span
	ctx, span := tracer.StartSpan(ctx, "basic-operation", trace.SpanKindInternal)
	defer func() {
		span.End()
		exporter.Export(span)
	}()

	// Simulate some work
	time.Sleep(50 * time.Millisecond)

	fmt.Printf("Created span: %s (trace: %s)\n", span.SpanID, span.TraceID)
}

func demoSpanHierarchy() {
	exporter := trace.NewNoopExporter()
	tracer := trace.NewTracer("demo-service", exporter, trace.AlwaysSample())

	ctx := context.Background()

	// Create parent span
	ctx, parentSpan := tracer.StartSpan(ctx, "parent-operation", trace.SpanKindInternal)
	defer func() {
		parentSpan.End()
		fmt.Printf("Parent span completed in %v\n", parentSpan.Duration)
	}()

	// Simulate work
	time.Sleep(20 * time.Millisecond)

	// Create child span 1
	childCtx1, childSpan1 := tracer.StartSpan(ctx, "child-operation-1", trace.SpanKindInternal)
	time.Sleep(10 * time.Millisecond)
	childSpan1.End()
	fmt.Printf("  Child span 1 completed in %v (parent: %s)\n", childSpan1.Duration, childSpan1.ParentID)

	// Create child span 2
	_, childSpan2 := tracer.StartSpan(childCtx1, "child-operation-2", trace.SpanKindInternal)
	time.Sleep(15 * time.Millisecond)
	childSpan2.End()
	fmt.Printf("  Child span 2 completed in %v (parent: %s)\n", childSpan2.Duration, childSpan2.ParentID)
}

func demoErrorRecording() {
	exporter := trace.NewNoopExporter()
	tracer := trace.NewTracer("demo-service", exporter, trace.AlwaysSample())

	ctx := context.Background()

	// Create span that encounters an error
	ctx, span := tracer.StartSpan(ctx, "failing-operation", trace.SpanKindInternal)
	defer func() {
		span.End()
	}()

	// Simulate work that fails
	err := simulateFailingWork()
	if err != nil {
		span.RecordError(err)
		fmt.Printf("Recorded error in span: %v\n", err)
		fmt.Printf("Span status: %s\n", span.Status)
		fmt.Printf("Error events: %d\n", len(span.Events))
	}
}

func demoEventsAndAttributes() {
	exporter := trace.NewNoopExporter()
	tracer := trace.NewTracer("demo-service", exporter, trace.AlwaysSample())

	ctx := context.Background()

	ctx, span := tracer.StartSpan(ctx, "operation-with-details", trace.SpanKindInternal)
	defer func() {
		span.End()
	}()

	// Add attributes
	span.SetAttribute("user.id", "12345")
	span.SetAttribute("request.method", "GET")
	span.SetAttribute("request.path", "/api/data")

	fmt.Println("Added attributes:")
	for key, value := range span.Attributes {
		if key != "service.name" {
			fmt.Printf("  %s: %v\n", key, value)
		}
	}

	// Add events
	span.AddEvent("cache-lookup", map[string]interface{}{
		"cache.hit": false,
		"cache.key": "user:12345",
	})

	time.Sleep(10 * time.Millisecond)

	span.AddEvent("database-query", map[string]interface{}{
		"db.query":    "SELECT * FROM users WHERE id = ?",
		"db.duration": "5ms",
	})

	fmt.Printf("\nAdded events: %d\n", len(span.Events))
	for i, event := range span.Events {
		fmt.Printf("  Event %d: %s\n", i+1, event.Name)
	}
}

func demoSampling() {
	// Always sample
	fmt.Println("Always Sample:")
	sampler1 := trace.AlwaysSample()
	for i := 0; i < 3; i++ {
		sampled := sampler1.ShouldSample("test-op")
		fmt.Printf("  Attempt %d: sampled=%v\n", i+1, sampled)
	}

	// Never sample
	fmt.Println("\nNever Sample:")
	sampler2 := trace.NeverSample()
	for i := 0; i < 3; i++ {
		sampled := sampler2.ShouldSample("test-op")
		fmt.Printf("  Attempt %d: sampled=%v\n", i+1, sampled)
	}

	// Probability sample (50%)
	fmt.Println("\nProbability Sample (50%):")
	sampler3 := trace.ProbabilitySample(0.5)
	sampled := 0
	total := 100
	for i := 0; i < total; i++ {
		if sampler3.ShouldSample("test-op") {
			sampled++
		}
	}
	fmt.Printf("  Sampled %d/%d (%.1f%%)\n", sampled, total, float64(sampled)/float64(total)*100)

	// Rate limit sample (10/sec)
	fmt.Println("\nRate Limit Sample (10/sec):")
	sampler4 := trace.RateLimitSample(10)
	sampled = 0
	for i := 0; i < 20; i++ {
		if sampler4.ShouldSample("test-op") {
			sampled++
		}
	}
	fmt.Printf("  Sampled %d/20 in burst\n", sampled)
}

func demoFileExport() {
	// Create temporary file
	tmpfile, err := os.CreateTemp("", "trace-demo-*.json")
	if err != nil {
		fmt.Printf("Failed to create temp file: %v\n", err)
		return
	}
	filename := tmpfile.Name()
	tmpfile.Close()
	defer os.Remove(filename)

	// Create file exporter
	exporter, err := trace.NewFileExporter(filename, true)
	if err != nil {
		fmt.Printf("Failed to create file exporter: %v\n", err)
		return
	}
	defer exporter.Close()

	tracer := trace.NewTracer("demo-service", exporter, trace.AlwaysSample())

	ctx := context.Background()

	// Create and export multiple spans
	for i := 1; i <= 3; i++ {
		_, span := tracer.StartSpan(ctx, fmt.Sprintf("operation-%d", i), trace.SpanKindInternal)
		span.SetAttribute("iteration", i)
		time.Sleep(10 * time.Millisecond)
		span.End()
		exporter.Export(span)
	}

	fmt.Printf("Exported 3 spans to: %s\n", filename)

	// Read and display file size
	info, err := os.Stat(filename)
	if err == nil {
		fmt.Printf("File size: %d bytes\n", info.Size())
	}
}

func demoCircuitBuilding() {
	exporter := trace.NewNoopExporter()
	tracer := trace.NewTracer("tor-client", exporter, trace.AlwaysSample())

	ctx := context.Background()

	// Simulate circuit building with tracing
	err := trace.WithSpan(ctx, tracer, "build-circuit", trace.SpanKindInternal, func(ctx context.Context, span *trace.Span) error {
		span.SetAttribute("circuit.id", 12345)
		span.SetAttribute("circuit.hops", 3)

		// Select guard
		err := trace.WithSpan(ctx, tracer, "select-guard", trace.SpanKindInternal, func(ctx context.Context, span *trace.Span) error {
			time.Sleep(20 * time.Millisecond)
			span.SetAttribute("relay.type", "guard")
			span.SetAttribute("relay.fingerprint", "AAAA1111")
			span.AddEvent("guard-selected", map[string]interface{}{
				"bandwidth": "10 Mbps",
			})
			return nil
		})
		if err != nil {
			return err
		}

		// Select middle
		err = trace.WithSpan(ctx, tracer, "select-middle", trace.SpanKindInternal, func(ctx context.Context, span *trace.Span) error {
			time.Sleep(15 * time.Millisecond)
			span.SetAttribute("relay.type", "middle")
			span.SetAttribute("relay.fingerprint", "BBBB2222")
			return nil
		})
		if err != nil {
			return err
		}

		// Select exit
		err = trace.WithSpan(ctx, tracer, "select-exit", trace.SpanKindInternal, func(ctx context.Context, span *trace.Span) error {
			time.Sleep(18 * time.Millisecond)
			span.SetAttribute("relay.type", "exit")
			span.SetAttribute("relay.fingerprint", "CCCC3333")
			return nil
		})
		if err != nil {
			return err
		}

		// Extend circuit
		err = trace.WithSpan(ctx, tracer, "extend-circuit", trace.SpanKindInternal, func(ctx context.Context, span *trace.Span) error {
			time.Sleep(30 * time.Millisecond)
			span.AddEvent("circuit-extended", map[string]interface{}{
				"hop": 1,
			})
			span.AddEvent("circuit-extended", map[string]interface{}{
				"hop": 2,
			})
			span.AddEvent("circuit-extended", map[string]interface{}{
				"hop": 3,
			})
			return nil
		})

		return err
	})

	if err != nil {
		fmt.Printf("Circuit building failed: %v\n", err)
	} else {
		fmt.Println("Simulated circuit building with distributed tracing")
		fmt.Println("  - Guard selection traced")
		fmt.Println("  - Middle selection traced")
		fmt.Println("  - Exit selection traced")
		fmt.Println("  - Circuit extension traced")
	}
}

func simulateFailingWork() error {
	return errors.New("connection timeout after 30s")
}
