// Package benchmark provides comprehensive end-to-end performance benchmarks
// for the go-tor Tor client implementation.
//
// This package validates the performance targets stated in the README:
// - Circuit build time: < 5 seconds (95th percentile)
// - Memory usage: < 50MB RSS in steady state
// - Concurrent streams: 100+ on typical hardware
package benchmark

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/opd-ai/go-tor/pkg/client"
	"github.com/opd-ai/go-tor/pkg/config"
	"github.com/opd-ai/go-tor/pkg/logger"
)

// Result holds the results of a benchmark run
type Result struct {
	Name              string
	Duration          time.Duration
	MemoryAllocated   uint64 // Bytes allocated during benchmark
	MemoryInUse       uint64 // Bytes in use at end of benchmark
	OperationsPerSec  float64
	TotalOperations   int64
	P50Latency        time.Duration
	P95Latency        time.Duration
	P99Latency        time.Duration
	MaxLatency        time.Duration
	Success           bool
	Error             error
	AdditionalMetrics map[string]interface{}
}

// Suite provides a comprehensive benchmark suite
type Suite struct {
	log     *logger.Logger
	results []Result
}

// NewSuite creates a new benchmark suite
func NewSuite(log *logger.Logger) *Suite {
	if log == nil {
		log = logger.NewDefault()
	}
	return &Suite{
		log:     log,
		results: make([]Result, 0),
	}
}

// MemorySnapshot captures current memory statistics
type MemorySnapshot struct {
	Timestamp   time.Time
	Alloc       uint64 // Bytes allocated and in use
	TotalAlloc  uint64 // Bytes allocated (cumulative)
	Sys         uint64 // Bytes from system
	NumGC       uint32 // Number of GC runs
	HeapAlloc   uint64 // Bytes in heap
	HeapSys     uint64 // Bytes from system for heap
	HeapObjects uint64 // Number of objects in heap
}

// GetMemorySnapshot returns current memory statistics
func GetMemorySnapshot() MemorySnapshot {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return MemorySnapshot{
		Timestamp:   time.Now(),
		Alloc:       m.Alloc,
		TotalAlloc:  m.TotalAlloc,
		Sys:         m.Sys,
		NumGC:       m.NumGC,
		HeapAlloc:   m.HeapAlloc,
		HeapSys:     m.HeapSys,
		HeapObjects: m.HeapObjects,
	}
}

// FormatBytes formats bytes as human-readable string
func FormatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// LatencyTracker tracks operation latencies for percentile calculation
type LatencyTracker struct {
	mu        sync.Mutex
	latencies []time.Duration
}

// NewLatencyTracker creates a new latency tracker
func NewLatencyTracker(capacity int) *LatencyTracker {
	return &LatencyTracker{
		latencies: make([]time.Duration, 0, capacity),
	}
}

// Record records a latency measurement
// This method is thread-safe and can be called concurrently.
func (lt *LatencyTracker) Record(latency time.Duration) {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	lt.latencies = append(lt.latencies, latency)
}

// Percentile calculates the specified percentile (0.0 to 1.0)
// This method is thread-safe and can be called concurrently.
func (lt *LatencyTracker) Percentile(p float64) time.Duration {
	lt.mu.Lock()
	defer lt.mu.Unlock()

	if len(lt.latencies) == 0 {
		return 0
	}

	// Sort latencies (simple insertion sort for small datasets)
	sorted := make([]time.Duration, len(lt.latencies))
	copy(sorted, lt.latencies)

	// Quick sort implementation
	quickSort(sorted, 0, len(sorted)-1)

	// Calculate percentile index
	index := int(float64(len(sorted)-1) * p)
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}

	return sorted[index]
}

// Max returns the maximum latency
// This method is thread-safe and can be called concurrently.
func (lt *LatencyTracker) Max() time.Duration {
	lt.mu.Lock()
	defer lt.mu.Unlock()

	if len(lt.latencies) == 0 {
		return 0
	}
	max := lt.latencies[0]
	for _, l := range lt.latencies[1:] {
		if l > max {
			max = l
		}
	}
	return max
}

// Count returns the number of recorded latencies
// This method is thread-safe and can be called concurrently.
func (lt *LatencyTracker) Count() int {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	return len(lt.latencies)
}

// quickSort implements quick sort for time.Duration slices
func quickSort(arr []time.Duration, low, high int) {
	if low < high {
		pi := partition(arr, low, high)
		quickSort(arr, low, pi-1)
		quickSort(arr, pi+1, high)
	}
}

func partition(arr []time.Duration, low, high int) int {
	pivot := arr[high]
	i := low - 1
	for j := low; j < high; j++ {
		if arr[j] < pivot {
			i++
			arr[i], arr[j] = arr[j], arr[i]
		}
	}
	arr[i+1], arr[high] = arr[high], arr[i+1]
	return i + 1
}

// CreateTestClient creates a client suitable for benchmarking
// This uses a temporary directory and minimal configuration
func CreateTestClient() (*client.Client, error) {
	cfg := config.DefaultConfig()
	cfg.LogLevel = "error" // Reduce noise during benchmarks
	cfg.SocksPort = 0      // Use random port
	cfg.ControlPort = 0    // Disable control port for benchmarks
	cfg.MetricsPort = 0    // Disable metrics for benchmarks

	log := logger.NewDefault()
	return client.New(cfg, log)
}

// Results returns all benchmark results
func (s *Suite) Results() []Result {
	return s.results
}

// AddResult adds a result to the suite
func (s *Suite) addResult(r Result) {
	s.results = append(s.results, r)
}

// PrintSummary prints a summary of all benchmark results
func (s *Suite) PrintSummary() {
	separator := "================================================================================"
	fmt.Println("\n" + separator)
	fmt.Println("BENCHMARK RESULTS SUMMARY")
	fmt.Println(separator)

	for _, r := range s.results {
		fmt.Printf("\n%s\n", r.Name)
		fmt.Printf("  Duration: %v\n", r.Duration)
		if r.TotalOperations > 0 {
			fmt.Printf("  Operations: %d (%.2f ops/sec)\n", r.TotalOperations, r.OperationsPerSec)
		}
		if r.P50Latency > 0 {
			fmt.Printf("  Latency (p50/p95/p99/max): %v / %v / %v / %v\n",
				r.P50Latency, r.P95Latency, r.P99Latency, r.MaxLatency)
		}
		if r.MemoryInUse > 0 {
			fmt.Printf("  Memory: %s in use, %s allocated\n",
				FormatBytes(r.MemoryInUse), FormatBytes(r.MemoryAllocated))
		}
		if r.Error != nil {
			fmt.Printf("  Error: %v\n", r.Error)
		} else {
			fmt.Printf("  Status: ✓ PASS\n")
		}

		// Print additional metrics
		if len(r.AdditionalMetrics) > 0 {
			fmt.Println("  Additional Metrics:")
			for k, v := range r.AdditionalMetrics {
				fmt.Printf("    %s: %v\n", k, v)
			}
		}
	}

	fmt.Println("\n" + separator)
}

// RunAll runs all benchmark suites
func (s *Suite) RunAll(ctx context.Context) error {
	s.log.Info("Starting comprehensive benchmark suite")

	// Run each benchmark category
	if err := s.BenchmarkCircuitBuild(ctx); err != nil {
		s.log.Warn("Circuit build benchmark failed", "error", err)
	}

	if err := s.BenchmarkMemoryUsage(ctx); err != nil {
		s.log.Warn("Memory usage benchmark failed", "error", err)
	}

	if err := s.BenchmarkConcurrentStreams(ctx); err != nil {
		s.log.Warn("Concurrent streams benchmark failed", "error", err)
	}

	s.log.Info("Benchmark suite complete", "total_tests", len(s.results))
	return nil
}
