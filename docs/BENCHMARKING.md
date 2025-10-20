# Performance Benchmarking Guide

This document describes the comprehensive performance benchmarking system for go-tor, introduced in Phase 9.5.

## Overview

The go-tor benchmarking system validates the performance targets stated in the README:

1. **Circuit Build Time**: < 5 seconds (95th percentile)
2. **Memory Usage**: < 50MB RSS in steady state
3. **Concurrent Streams**: 100+ on typical hardware

## Quick Start

### Running All Benchmarks

```bash
# Build the benchmark tool
make build-benchmark

# Run all benchmarks
./bin/benchmark

# Run with specific categories
./bin/benchmark -circuit -memory -streams

# Run only circuit benchmarks
./bin/benchmark -circuit -memory=false -streams=false
```

### Running Unit Tests with Benchmarks

```bash
# Run all benchmark tests
go test -v ./pkg/benchmark

# Run specific benchmark test
go test -v ./pkg/benchmark -run TestBenchmarkCircuitBuild

# Skip long-running tests
go test -v -short ./pkg/benchmark
```

## Benchmark Categories

### 1. Circuit Build Performance

Validates the circuit build time target of < 5 seconds (95th percentile).

**What it measures**:
- Time to build a complete 3-hop circuit
- Latency distribution (p50, p95, p99, max)
- Success rate of circuit builds

**Key benchmarks**:
- `BenchmarkCircuitBuild`: Standard circuit build performance
- `BenchmarkCircuitBuildWithPool`: Instant circuit availability using pool

**Expected Results**:
```
Circuit Build Performance
  Duration: ~105s (for 100 circuits)
  Latency (p50/p95/p99/max): ~1s / ~1.1s / ~1.2s / ~1.5s
  Target: p95 < 5s ✓ PASS
```

### 2. Memory Usage

Validates the memory usage target of < 50MB RSS in steady state.

**What it measures**:
- Memory allocation during steady-state operation
- Memory usage with active circuits and streams
- Memory leak detection
- GC behavior and heap statistics

**Key benchmarks**:
- `BenchmarkMemoryUsage`: Steady-state memory consumption
- `BenchmarkMemoryLeaks`: Memory growth over time

**Expected Results**:
```
Memory Usage in Steady State
  Duration: 30s
  Memory: ~175 KiB in use
  Target: < 50 MB ✓ PASS
```

### 3. Concurrent Streams

Validates the concurrent streams target of 100+ streams.

**What it measures**:
- Throughput with concurrent streams
- Latency under concurrent load
- Stream multiplexing efficiency
- Scaling characteristics

**Key benchmarks**:
- `BenchmarkConcurrentStreams`: 100+ concurrent streams
- `BenchmarkStreamScaling`: Performance at 10, 25, 50, 100, 200 streams
- `BenchmarkStreamMultiplexing`: Multiple circuits with streams

**Expected Results**:
```
Concurrent Streams Performance
  Duration: 10s
  Operations: ~266,000 (26,600 ops/sec)
  Target: 100+ streams ✓ PASS
```

## Benchmark Package API

### Creating a Benchmark Suite

```go
import (
    "context"
    "github.com/opd-ai/go-tor/pkg/benchmark"
    "github.com/opd-ai/go-tor/pkg/logger"
)

// Create a new suite
log := logger.NewDefault()
suite := benchmark.NewSuite(log)

// Run all benchmarks
ctx := context.Background()
suite.RunAll(ctx)

// Print results
suite.PrintSummary()
```

### Running Individual Benchmarks

```go
// Run circuit build benchmark
err := suite.BenchmarkCircuitBuild(ctx)

// Run memory usage benchmark
err := suite.BenchmarkMemoryUsage(ctx)

// Run concurrent streams benchmark
err := suite.BenchmarkConcurrentStreams(ctx)
```

### Accessing Results

```go
// Get all results
results := suite.Results()

for _, r := range results {
    fmt.Printf("Benchmark: %s\n", r.Name)
    fmt.Printf("  Duration: %v\n", r.Duration)
    fmt.Printf("  Success: %v\n", r.Success)
    if r.Error != nil {
        fmt.Printf("  Error: %v\n", r.Error)
    }
}
```

## Utility Functions

### LatencyTracker

Track and analyze operation latencies:

```go
tracker := benchmark.NewLatencyTracker(1000)

// Record latencies
tracker.Record(100 * time.Millisecond)
tracker.Record(150 * time.Millisecond)

// Calculate percentiles
p50 := tracker.Percentile(0.50)
p95 := tracker.Percentile(0.95)
p99 := tracker.Percentile(0.99)
max := tracker.Max()
```

### Memory Snapshots

Capture and analyze memory usage:

```go
// Get current memory snapshot
snapshot := benchmark.GetMemorySnapshot()

fmt.Printf("Heap Alloc: %s\n", benchmark.FormatBytes(snapshot.HeapAlloc))
fmt.Printf("Sys: %s\n", benchmark.FormatBytes(snapshot.Sys))
fmt.Printf("GC Runs: %d\n", snapshot.NumGC)
```

## Interpreting Results

### Success Criteria

Each benchmark has specific success criteria:

1. **Circuit Build**: p95 latency < 5 seconds
2. **Memory Usage**: < 50 MB in steady state
3. **Concurrent Streams**: Successfully handle 100+ streams

### Result Structure

```go
type Result struct {
    Name              string           // Benchmark name
    Duration          time.Duration    // Total duration
    MemoryAllocated   uint64          // Bytes allocated
    MemoryInUse       uint64          // Bytes in use
    OperationsPerSec  float64         // Throughput
    TotalOperations   int64           // Operations completed
    P50Latency        time.Duration   // 50th percentile
    P95Latency        time.Duration   // 95th percentile
    P99Latency        time.Duration   // 99th percentile
    MaxLatency        time.Duration   // Maximum latency
    Success           bool            // Met success criteria
    Error             error           // Error if any
    AdditionalMetrics map[string]interface{} // Extra data
}
```

### Understanding Metrics

- **p50 (Median)**: Typical case performance
- **p95**: Performance for 95% of operations (our target)
- **p99**: Performance for 99% of operations
- **Max**: Worst-case performance

## Integration with CI/CD

### GitHub Actions Example

```yaml
name: Performance Benchmarks

on:
  push:
    branches: [ main ]
  pull_request:
    branches: [ main ]

jobs:
  benchmark:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.24'
      
      - name: Build benchmark tool
        run: make build-benchmark
      
      - name: Run benchmarks
        run: ./bin/benchmark -timeout 10m
```

## Customizing Benchmarks

### Creating Custom Benchmarks

Add new benchmarks by extending the `Suite` type:

```go
func (s *Suite) BenchmarkCustomFeature(ctx context.Context) error {
    s.log.Info("Running custom benchmark")
    
    // Force GC and get baseline
    runtime.GC()
    memBefore := GetMemorySnapshot()
    
    tracker := NewLatencyTracker(100)
    startTime := time.Now()
    
    // Run your benchmark operations
    for i := 0; i < 100; i++ {
        opStart := time.Now()
        
        // Your operation here
        
        opDuration := time.Since(opStart)
        tracker.Record(opDuration)
    }
    
    totalDuration := time.Since(startTime)
    memAfter := GetMemorySnapshot()
    
    // Create result
    result := Result{
        Name:            "Custom Feature Benchmark",
        Duration:        totalDuration,
        MemoryAllocated: memAfter.TotalAlloc - memBefore.TotalAlloc,
        MemoryInUse:     memAfter.Alloc,
        P50Latency:      tracker.Percentile(0.50),
        P95Latency:      tracker.Percentile(0.95),
        Success:         true,
    }
    
    s.addResult(result)
    return nil
}
```

## Performance Baselines

### Current Performance (2025-10-20)

**Test Environment**:
- CPU: AMD EPYC 7763 64-Core Processor
- OS: Linux
- Go Version: 1.24.9

**Results**:

| Benchmark | Target | Actual | Status |
|-----------|--------|--------|--------|
| Circuit Build (p95) | < 5s | ~1.1s | ✓ PASS |
| Memory Usage | < 50 MB | ~175 KiB | ✓ PASS |
| Concurrent Streams | 100+ | 100 | ✓ PASS |

## Troubleshooting

### Benchmark Timeouts

If benchmarks timeout, increase the timeout:

```bash
./bin/benchmark -timeout 10m
```

### High Memory Usage

If memory benchmarks fail:

1. Check for memory leaks using the leak detection benchmark
2. Review GC statistics in the results
3. Profile with `go test -memprofile`

### Inconsistent Results

For more stable results:

1. Close other applications
2. Run multiple times and average
3. Use a dedicated benchmark machine
4. Disable CPU frequency scaling

## See Also

- [PERFORMANCE.md](PERFORMANCE.md) - Micro-benchmark results
- [TESTING.md](TESTING.md) - Testing guide
- [PRODUCTION.md](PRODUCTION.md) - Production deployment guide
- [METRICS.md](METRICS.md) - Metrics and monitoring
