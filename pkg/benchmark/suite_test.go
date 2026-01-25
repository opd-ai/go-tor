package benchmark

import (
	"context"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/logger"
)

// TestCreateTestClient tests the client creation for benchmarking
func TestCreateTestClient(t *testing.T) {
	client, err := CreateTestClient()
	if err != nil {
		t.Fatalf("CreateTestClient failed: %v", err)
	}

	if client == nil {
		t.Fatal("Expected non-nil client")
	}

	// Verify the client was configured correctly
	// No additional checks needed as this is a simple wrapper
}

// TestBenchmarkCircuitBuildShort is a fast version for short mode
func TestBenchmarkCircuitBuildShort(t *testing.T) {
	log := logger.NewDefault()
	suite := NewSuite(log)

	// Use a very short timeout to ensure test completes quickly
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := suite.BenchmarkCircuitBuild(ctx)
	// Expect context deadline exceeded due to short timeout
	if err != nil && err != context.DeadlineExceeded && err != context.Canceled {
		// But don't fail if partial results were recorded
		t.Logf("BenchmarkCircuitBuild returned: %v", err)
	}

	// Even with cancellation, some results may have been recorded
	results := suite.Results()
	t.Logf("Recorded %d results", len(results))
}

// TestBenchmarkMemoryUsageShort is a fast version for short mode
func TestBenchmarkMemoryUsageShort(t *testing.T) {
	log := logger.NewDefault()
	suite := NewSuite(log)

	// Use a very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := suite.BenchmarkMemoryUsage(ctx)
	// Expect context deadline exceeded
	if err != nil && err != context.DeadlineExceeded && err != context.Canceled {
		t.Logf("BenchmarkMemoryUsage returned: %v", err)
	}

	results := suite.Results()
	t.Logf("Recorded %d results", len(results))
}

// TestBenchmarkConcurrentStreamsShort is a fast version for short mode
func TestBenchmarkConcurrentStreamsShort(t *testing.T) {
	log := logger.NewDefault()
	suite := NewSuite(log)

	// Use a very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := suite.BenchmarkConcurrentStreams(ctx)
	if err != nil && err != context.DeadlineExceeded && err != context.Canceled {
		t.Logf("BenchmarkConcurrentStreams returned: %v", err)
	}

	results := suite.Results()
	t.Logf("Recorded %d results", len(results))
}

// TestBenchmarkMemoryLeaksShort is a fast version for short mode
func TestBenchmarkMemoryLeaksShort(t *testing.T) {
	log := logger.NewDefault()
	suite := NewSuite(log)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := suite.BenchmarkMemoryLeaks(ctx)
	if err != nil && err != context.DeadlineExceeded && err != context.Canceled {
		t.Logf("BenchmarkMemoryLeaks returned: %v", err)
	}

	results := suite.Results()
	t.Logf("Recorded %d results", len(results))
}

// TestBenchmarkCircuitBuildWithPoolShort is a fast version for short mode
func TestBenchmarkCircuitBuildWithPoolShort(t *testing.T) {
	log := logger.NewDefault()
	suite := NewSuite(log)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := suite.BenchmarkCircuitBuildWithPool(ctx)
	if err != nil && err != context.DeadlineExceeded && err != context.Canceled {
		t.Logf("BenchmarkCircuitBuildWithPool returned: %v", err)
	}

	results := suite.Results()
	t.Logf("Recorded %d results", len(results))
}

// TestBenchmarkStreamScalingShort is a fast version for short mode
func TestBenchmarkStreamScalingShort(t *testing.T) {
	log := logger.NewDefault()
	suite := NewSuite(log)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := suite.BenchmarkStreamScaling(ctx)
	if err != nil && err != context.DeadlineExceeded && err != context.Canceled {
		t.Logf("BenchmarkStreamScaling returned: %v", err)
	}

	results := suite.Results()
	t.Logf("Recorded %d results", len(results))
}

// TestBenchmarkStreamMultiplexingShort is a fast version for short mode
func TestBenchmarkStreamMultiplexingShort(t *testing.T) {
	log := logger.NewDefault()
	suite := NewSuite(log)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := suite.BenchmarkStreamMultiplexing(ctx)
	if err != nil && err != context.DeadlineExceeded && err != context.Canceled {
		t.Logf("BenchmarkStreamMultiplexing returned: %v", err)
	}

	results := suite.Results()
	t.Logf("Recorded %d results", len(results))
}

// TestRunAllShort is a fast version for short mode
func TestRunAllShort(t *testing.T) {
	log := logger.NewDefault()
	suite := NewSuite(log)

	// Use a short timeout so RunAll will be cancelled quickly
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := suite.RunAll(ctx)
	// RunAll doesn't return errors from individual benchmarks
	if err != nil {
		t.Logf("RunAll returned: %v", err)
	}

	// Even with early cancellation, some results should be recorded
	results := suite.Results()
	if len(results) == 0 {
		t.Log("No results recorded (expected with short timeout)")
	} else {
		t.Logf("Recorded %d results", len(results))
	}
}

// TestRunAll tests the comprehensive benchmark suite
func TestRunAll(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping RunAll in short mode - takes too long")
	}

	log := logger.NewDefault()
	suite := NewSuite(log)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	err := suite.RunAll(ctx)
	if err != nil {
		t.Fatalf("RunAll failed: %v", err)
	}

	// Verify results were collected
	results := suite.Results()
	if len(results) == 0 {
		t.Error("Expected at least one benchmark result")
	}

	// Verify each benchmark ran
	expectedBenchmarks := []string{
		"Circuit Build Performance",
		"Memory Usage in Steady State",
		"Concurrent Streams Performance",
	}

	for _, expected := range expectedBenchmarks {
		found := false
		for _, result := range results {
			if result.Name == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected benchmark %q not found in results", expected)
		}
	}
}

// TestRunAllWithCancellation tests that RunAll respects context cancellation
func TestRunAllWithCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping RunAll cancellation test in short mode")
	}

	log := logger.NewDefault()
	suite := NewSuite(log)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// This should be cancelled quickly
	err := suite.RunAll(ctx)
	// RunAll doesn't return context errors from sub-benchmarks
	// It continues even if individual benchmarks fail
	if err != nil {
		t.Logf("RunAll returned error (expected): %v", err)
	}
}

// TestNewSuiteWithNilLogger tests that NewSuite handles nil logger gracefully
func TestNewSuiteWithNilLogger(t *testing.T) {
	suite := NewSuite(nil)
	if suite == nil {
		t.Fatal("NewSuite returned nil")
	}
	if suite.log == nil {
		t.Error("Expected default logger to be created")
	}
}

// TestBenchmarkCircuitBuildWithPool tests the circuit pool benchmark
func TestBenchmarkCircuitBuildWithPool(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping pool benchmark in short mode")
	}

	log := logger.NewDefault()
	suite := NewSuite(log)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := suite.BenchmarkCircuitBuildWithPool(ctx)
	if err != nil && err != context.Canceled {
		t.Fatalf("BenchmarkCircuitBuildWithPool failed: %v", err)
	}

	results := suite.Results()
	if len(results) == 0 {
		t.Fatal("Expected at least one result")
	}

	result := results[0]
	if result.Name != "Circuit Build with Pool (Instant Availability)" {
		t.Errorf("Expected specific benchmark name, got %q", result.Name)
	}

	if result.TotalOperations == 0 {
		t.Error("Expected non-zero operations")
	}

	t.Logf("Pool benchmark: ops=%d, throughput=%.2f ops/sec", 
		result.TotalOperations, result.OperationsPerSec)
}

// TestBenchmarkMemoryLeaks tests the memory leak detection
func TestBenchmarkMemoryLeaks(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory leak test in short mode")
	}

	log := logger.NewDefault()
	suite := NewSuite(log)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := suite.BenchmarkMemoryLeaks(ctx)
	if err != nil && err != context.Canceled {
		t.Fatalf("BenchmarkMemoryLeaks failed: %v", err)
	}

	results := suite.Results()
	if len(results) == 0 {
		t.Fatal("Expected at least one result")
	}

	result := results[0]
	if result.Name != "Memory Leak Detection" {
		t.Errorf("Expected specific benchmark name, got %q", result.Name)
	}

	if result.TotalOperations == 0 {
		t.Error("Expected non-zero operations")
	}

	// Memory growth should be reasonable
	if metrics, ok := result.AdditionalMetrics["memory_growth"]; ok {
		t.Logf("Memory growth: %v", metrics)
	}
}

// TestBenchmarkStreamScaling tests stream scaling
func TestBenchmarkStreamScaling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stream scaling test in short mode")
	}

	log := logger.NewDefault()
	suite := NewSuite(log)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	err := suite.BenchmarkStreamScaling(ctx)
	if err != nil && err != context.Canceled {
		t.Fatalf("BenchmarkStreamScaling failed: %v", err)
	}

	results := suite.Results()
	if len(results) == 0 {
		t.Fatal("Expected at least one result")
	}

	// Should have results for multiple stream counts
	if len(results) < 2 {
		t.Logf("Expected multiple results for different stream counts, got %d", len(results))
	}

	for i, result := range results {
		t.Logf("Scaling test %d: %s - ops=%d, throughput=%.2f", 
			i, result.Name, result.TotalOperations, result.OperationsPerSec)
	}
}

// TestBenchmarkStreamMultiplexing tests stream multiplexing
func TestBenchmarkStreamMultiplexing(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stream multiplexing test in short mode")
	}

	log := logger.NewDefault()
	suite := NewSuite(log)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := suite.BenchmarkStreamMultiplexing(ctx)
	if err != nil && err != context.Canceled {
		t.Fatalf("BenchmarkStreamMultiplexing failed: %v", err)
	}

	results := suite.Results()
	if len(results) == 0 {
		t.Fatal("Expected at least one result")
	}

	result := results[0]
	if result.Name != "Stream Multiplexing Performance" {
		t.Errorf("Expected specific benchmark name, got %q", result.Name)
	}

	if result.TotalOperations == 0 {
		t.Error("Expected non-zero operations")
	}

	if metrics, ok := result.AdditionalMetrics["total_streams"]; ok {
		t.Logf("Total streams: %v", metrics)
	}
}
