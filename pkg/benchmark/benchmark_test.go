package benchmark

import (
	"context"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/logger"
)

func TestLatencyTracker(t *testing.T) {
	tracker := NewLatencyTracker(10)

	// Record some latencies
	latencies := []time.Duration{
		100 * time.Millisecond,
		200 * time.Millisecond,
		150 * time.Millisecond,
		300 * time.Millisecond,
		250 * time.Millisecond,
	}

	for _, l := range latencies {
		tracker.Record(l)
	}

	// Test Count
	if tracker.Count() != len(latencies) {
		t.Errorf("Expected count %d, got %d", len(latencies), tracker.Count())
	}

	// Test Max
	expectedMax := 300 * time.Millisecond
	if tracker.Max() != expectedMax {
		t.Errorf("Expected max %v, got %v", expectedMax, tracker.Max())
	}

	// Test Percentile
	p50 := tracker.Percentile(0.50)
	if p50 < 100*time.Millisecond || p50 > 300*time.Millisecond {
		t.Errorf("P50 %v out of reasonable range", p50)
	}

	p95 := tracker.Percentile(0.95)
	if p95 < 250*time.Millisecond || p95 > 300*time.Millisecond {
		t.Errorf("P95 %v should be close to max", p95)
	}
}

func TestLatencyTrackerEmpty(t *testing.T) {
	tracker := NewLatencyTracker(10)

	if tracker.Count() != 0 {
		t.Errorf("Expected count 0, got %d", tracker.Count())
	}

	if tracker.Max() != 0 {
		t.Errorf("Expected max 0, got %v", tracker.Max())
	}

	if tracker.Percentile(0.95) != 0 {
		t.Errorf("Expected percentile 0, got %v", tracker.Percentile(0.95))
	}
}

// TestLatencyTrackerConcurrent tests thread-safety of LatencyTracker
// This test ensures the race condition fix works correctly
func TestLatencyTrackerConcurrent(t *testing.T) {
	tracker := NewLatencyTracker(1000)

	// Number of concurrent goroutines
	numGoroutines := 10
	numRecordsPerGoroutine := 100

	// Use a channel to synchronize goroutines
	done := make(chan bool, numGoroutines)

	// Launch concurrent recorders
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < numRecordsPerGoroutine; j++ {
				// Record different latencies from each goroutine
				latency := time.Duration(id*100+j) * time.Microsecond
				tracker.Record(latency)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify all records were captured
	expectedCount := numGoroutines * numRecordsPerGoroutine
	if tracker.Count() != expectedCount {
		t.Errorf("Expected count %d, got %d", expectedCount, tracker.Count())
	}

	// Test that Percentile and Max can be called concurrently with Record
	done2 := make(chan bool, 3)

	go func() {
		tracker.Record(999 * time.Millisecond)
		done2 <- true
	}()

	go func() {
		_ = tracker.Percentile(0.95)
		done2 <- true
	}()

	go func() {
		_ = tracker.Max()
		done2 <- true
	}()

	// Wait for concurrent operations to complete
	for i := 0; i < 3; i++ {
		<-done2
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    uint64
		expected string
	}{
		{0, "0 B"},
		{500, "500 B"},
		{1024, "1.0 KiB"},
		{1536, "1.5 KiB"},
		{1024 * 1024, "1.0 MiB"},
		{50 * 1024 * 1024, "50.0 MiB"},
		{1024 * 1024 * 1024, "1.0 GiB"},
	}

	for _, tt := range tests {
		result := FormatBytes(tt.bytes)
		if result != tt.expected {
			t.Errorf("FormatBytes(%d) = %s, expected %s", tt.bytes, result, tt.expected)
		}
	}
}

func TestGetMemorySnapshot(t *testing.T) {
	snapshot := GetMemorySnapshot()

	if snapshot.Alloc == 0 {
		t.Error("Expected non-zero memory allocation")
	}

	if snapshot.TotalAlloc == 0 {
		t.Error("Expected non-zero total allocation")
	}

	if snapshot.Sys == 0 {
		t.Error("Expected non-zero system memory")
	}
}

func TestSuiteBasic(t *testing.T) {
	log := logger.NewDefault()
	suite := NewSuite(log)

	if suite == nil {
		t.Fatal("NewSuite returned nil")
	}

	if len(suite.Results()) != 0 {
		t.Errorf("Expected 0 results, got %d", len(suite.Results()))
	}

	// Add a result
	suite.addResult(Result{
		Name:     "Test",
		Duration: time.Second,
		Success:  true,
	})

	if len(suite.Results()) != 1 {
		t.Errorf("Expected 1 result, got %d", len(suite.Results()))
	}
}

func TestBenchmarkCircuitBuild(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping benchmark in short mode")
	}

	log := logger.NewDefault()
	suite := NewSuite(log)

	ctx := context.Background()
	err := suite.BenchmarkCircuitBuild(ctx)
	if err != nil {
		t.Fatalf("BenchmarkCircuitBuild failed: %v", err)
	}

	results := suite.Results()
	if len(results) == 0 {
		t.Fatal("Expected at least one result")
	}

	result := results[0]
	if result.Name == "" {
		t.Error("Result name is empty")
	}

	if result.Duration == 0 {
		t.Error("Result duration is zero")
	}

	if result.TotalOperations == 0 {
		t.Error("No operations were performed")
	}

	t.Logf("Circuit build benchmark: p95=%v, ops=%d", result.P95Latency, result.TotalOperations)
}

func TestBenchmarkMemoryUsage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping benchmark in short mode")
	}

	log := logger.NewDefault()
	suite := NewSuite(log)

	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()

	err := suite.BenchmarkMemoryUsage(ctx)
	if err != nil && err != context.Canceled {
		t.Fatalf("BenchmarkMemoryUsage failed: %v", err)
	}

	results := suite.Results()
	if len(results) == 0 {
		t.Fatal("Expected at least one result")
	}

	result := results[0]
	t.Logf("Memory usage: %s (target: 50 MB)", FormatBytes(result.MemoryInUse))
}

func TestBenchmarkConcurrentStreams(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping benchmark in short mode")
	}

	log := logger.NewDefault()
	suite := NewSuite(log)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	err := suite.BenchmarkConcurrentStreams(ctx)
	if err != nil && err != context.Canceled {
		t.Fatalf("BenchmarkConcurrentStreams failed: %v", err)
	}

	results := suite.Results()
	if len(results) == 0 {
		t.Fatal("Expected at least one result")
	}

	result := results[0]
	if result.TotalOperations == 0 {
		t.Error("No operations were performed")
	}

	t.Logf("Concurrent streams: ops=%d, throughput=%.2f ops/sec",
		result.TotalOperations, result.OperationsPerSec)
}

func TestPrintSummary(t *testing.T) {
	log := logger.NewDefault()
	suite := NewSuite(log)

	// Add some test results
	suite.addResult(Result{
		Name:             "Test Benchmark 1",
		Duration:         time.Second,
		MemoryInUse:      50 * 1024 * 1024,
		MemoryAllocated:  100 * 1024 * 1024,
		OperationsPerSec: 1000,
		TotalOperations:  1000,
		P50Latency:       time.Millisecond,
		P95Latency:       5 * time.Millisecond,
		P99Latency:       10 * time.Millisecond,
		MaxLatency:       15 * time.Millisecond,
		Success:          true,
		AdditionalMetrics: map[string]interface{}{
			"test_metric": "value",
		},
	})

	suite.addResult(Result{
		Name:    "Test Benchmark 2",
		Success: false,
		Error:   context.DeadlineExceeded,
	})

	// Should not panic
	suite.PrintSummary()
}

func TestQuickSort(t *testing.T) {
	durations := []time.Duration{
		5 * time.Second,
		2 * time.Second,
		8 * time.Second,
		1 * time.Second,
		4 * time.Second,
	}

	quickSort(durations, 0, len(durations)-1)

	// Verify sorted
	for i := 1; i < len(durations); i++ {
		if durations[i] < durations[i-1] {
			t.Errorf("Array not sorted at index %d: %v > %v", i, durations[i-1], durations[i])
		}
	}

	// Check values
	if durations[0] != 1*time.Second {
		t.Errorf("Expected first element to be 1s, got %v", durations[0])
	}
	if durations[len(durations)-1] != 8*time.Second {
		t.Errorf("Expected last element to be 8s, got %v", durations[len(durations)-1])
	}
}
