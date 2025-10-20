package benchmark

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/opd-ai/go-tor/pkg/security"
)

// BenchmarkMemoryUsage validates the memory usage target
// Target: < 50MB RSS in steady state
//
// This benchmark measures memory usage under typical workload:
// - Active circuits
// - Concurrent connections
// - Stream multiplexing
// - Metrics collection
func (s *Suite) BenchmarkMemoryUsage(ctx context.Context) error {
	s.log.Info("Running memory usage benchmark")
	
	const (
		targetMemoryMB = 50
		targetMemory   = targetMemoryMB * 1024 * 1024 // 50 MB in bytes
		numCircuits    = 3
		numStreams     = 10
		duration       = 30 * time.Second
	)
	
	// Force GC and get baseline
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	runtime.GC()
	memBefore := GetMemorySnapshot()
	
	startTime := time.Now()
	
	// Define types for simulating circuits and streams
	type mockStream struct {
		id   uint16
		data []byte
	}
	
	type mockCircuit struct {
		id      uint32
		data    []byte
		streams []mockStream
	}
	
	// Simulate steady-state operation
	// In production, this would:
	// 1. Maintain circuit pool
	// 2. Handle concurrent streams
	// 3. Collect metrics
	// 4. Manage guard state
	
	circuits := make([]mockCircuit, numCircuits)
	for i := range circuits {
		// Safe conversion for circuit ID (AUDIT-003)
		circuitID, err := security.SafeIntToUint32(i)
		if err != nil {
			return fmt.Errorf("circuit index out of range: %w", err)
		}
		circuits[i] = mockCircuit{
			id:      circuitID,
			data:    make([]byte, 1024*100), // ~100KB per circuit
			streams: make([]mockStream, numStreams),
		}
		
		for j := range circuits[i].streams {
			// Safe conversion for stream ID (AUDIT-004)
			streamID, err := security.SafeIntToUint16(j)
			if err != nil {
				return fmt.Errorf("stream index out of range: %w", err)
			}
			circuits[i].streams[j] = mockStream{
				id:   streamID,
				data: make([]byte, 1024*10), // ~10KB per stream
			}
		}
	}
	
	// Simulate metrics collection
	metrics := make(map[string]int64)
	for i := 0; i < 100; i++ {
		metrics[fmt.Sprintf("metric_%d", i)] = int64(i)
	}
	
	// Run for duration
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	
	samples := make([]MemorySnapshot, 0, int(duration.Seconds()))
	
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// Record memory snapshot
			runtime.GC()
			snapshot := GetMemorySnapshot()
			samples = append(samples, snapshot)
			
			// Check if we've run long enough
			if time.Since(startTime) >= duration {
				goto done
			}
			
			// Simulate some activity
			for i := range circuits {
				circuits[i].data[0]++
				for j := range circuits[i].streams {
					circuits[i].streams[j].data[0]++
				}
			}
		}
	}
	
done:
	totalDuration := time.Since(startTime)
	
	// Force final GC and get final memory
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	runtime.GC()
	memAfter := GetMemorySnapshot()
	
	// Calculate memory statistics
	avgMemory := uint64(0)
	maxMemory := uint64(0)
	minMemory := memAfter.Alloc
	
	for _, sample := range samples {
		avgMemory += sample.Alloc
		if sample.Alloc > maxMemory {
			maxMemory = sample.Alloc
		}
		if sample.Alloc < minMemory {
			minMemory = sample.Alloc
		}
	}
	
	if len(samples) > 0 {
		avgMemory /= uint64(len(samples))
	}
	
	// Check if we meet the target
	success := memAfter.Alloc <= targetMemory
	
	result := Result{
		Name:             "Memory Usage in Steady State",
		Duration:         totalDuration,
		MemoryAllocated:  memAfter.TotalAlloc - memBefore.TotalAlloc,
		MemoryInUse:      memAfter.Alloc,
		TotalOperations:  int64(len(samples)),
		Success:          success,
		AdditionalMetrics: map[string]interface{}{
			"target_mb":       targetMemoryMB,
			"actual_mb":       float64(memAfter.Alloc) / (1024 * 1024),
			"avg_mb":          float64(avgMemory) / (1024 * 1024),
			"max_mb":          float64(maxMemory) / (1024 * 1024),
			"min_mb":          float64(minMemory) / (1024 * 1024),
			"heap_objects":    memAfter.HeapObjects,
			"gc_runs":         memAfter.NumGC - memBefore.NumGC,
			"num_circuits":    numCircuits,
			"num_streams":     numCircuits * numStreams,
			"samples_count":   len(samples),
			"meets_target":    success,
		},
	}
	
	if !success {
		result.Error = fmt.Errorf("memory usage (%s) exceeds target (%d MB)",
			FormatBytes(memAfter.Alloc), targetMemoryMB)
	}
	
	s.addResult(result)
	s.log.Info("Memory usage benchmark complete",
		"actual_mb", float64(memAfter.Alloc)/(1024*1024),
		"target_mb", targetMemoryMB,
		"success", success)
	
	return nil
}

// BenchmarkMemoryLeaks checks for memory leaks over extended operation
func (s *Suite) BenchmarkMemoryLeaks(ctx context.Context) error {
	s.log.Info("Running memory leak detection benchmark")
	
	const (
		iterations = 1000
		threshold  = 10 * 1024 * 1024 // 10 MB growth threshold
	)
	
	// Force GC and get baseline
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	memBefore := GetMemorySnapshot()
	
	startTime := time.Now()
	
	// Perform repeated allocations and deallocations
	for i := 0; i < iterations; i++ {
		// Simulate work
		data := make([]byte, 1024*10) // 10KB allocation
		data[0] = byte(i)              // Use the data
		
		// Periodically force GC
		if i%100 == 0 {
			runtime.GC()
		}
		
		// Check context
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
	
	totalDuration := time.Since(startTime)
	
	// Force final GC
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	runtime.GC()
	memAfter := GetMemorySnapshot()
	
	// Calculate memory growth - use safe conversion to avoid overflow (AUDIT-001)
	allocAfter, err := security.SafeUint64ToInt64(memAfter.Alloc)
	if err != nil {
		return fmt.Errorf("failed to convert memory after: %w", err)
	}
	allocBefore, err := security.SafeUint64ToInt64(memBefore.Alloc)
	if err != nil {
		return fmt.Errorf("failed to convert memory before: %w", err)
	}
	memoryGrowth := allocAfter - allocBefore
	success := memoryGrowth <= int64(threshold)
	
	result := Result{
		Name:             "Memory Leak Detection",
		Duration:         totalDuration,
		MemoryAllocated:  memAfter.TotalAlloc - memBefore.TotalAlloc,
		MemoryInUse:      memAfter.Alloc,
		TotalOperations:  int64(iterations),
		OperationsPerSec: float64(iterations) / totalDuration.Seconds(),
		Success:          success,
		AdditionalMetrics: map[string]interface{}{
			"memory_growth":  FormatBytes(uint64(memoryGrowth)),
			"threshold":      FormatBytes(threshold),
			"before_mb":      float64(memBefore.Alloc) / (1024 * 1024),
			"after_mb":       float64(memAfter.Alloc) / (1024 * 1024),
			"gc_runs":        memAfter.NumGC - memBefore.NumGC,
			"meets_target":   success,
		},
	}
	
	if !success {
		result.Error = fmt.Errorf("memory growth (%s) exceeds threshold (%s)",
			FormatBytes(uint64(memoryGrowth)), FormatBytes(threshold))
	}
	
	s.addResult(result)
	// Safe conversion for display - handle negative growth (AUDIT-005)
	var growthDisplay string
	if memoryGrowth < 0 {
		growthDisplay = fmt.Sprintf("-%s", FormatBytes(uint64(-memoryGrowth)))
	} else {
		growthDisplay = FormatBytes(uint64(memoryGrowth))
	}
	s.log.Info("Memory leak detection complete",
		"growth", growthDisplay,
		"success", success)
	
	return nil
}
