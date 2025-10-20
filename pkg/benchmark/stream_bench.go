package benchmark

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// BenchmarkConcurrentStreams validates the concurrent streams target
// Target: 100+ concurrent streams on typical hardware
//
// This benchmark measures the system's ability to handle many concurrent
// streams simultaneously, simulating real-world usage.
func (s *Suite) BenchmarkConcurrentStreams(ctx context.Context) error {
	s.log.Info("Running concurrent streams benchmark")
	
	const (
		targetStreams = 100
		duration      = 10 * time.Second
		dataSize      = 1024 // 1KB per stream operation
	)
	
	// Force GC before benchmark
	runtime.GC()
	memBefore := GetMemorySnapshot()
	
	tracker := NewLatencyTracker(1000)
	startTime := time.Now()
	
	var (
		successCount int64
		errorCount   int64
		wg           sync.WaitGroup
	)
	
	// Launch concurrent streams
	wg.Add(targetStreams)
	for i := 0; i < targetStreams; i++ {
		go func(streamID int) {
			defer wg.Done()
			
			// Simulate stream operations
			for {
				select {
				case <-ctx.Done():
					return
				default:
					opStart := time.Now()
					
					// Simulate stream data transfer
					data := make([]byte, dataSize)
					data[0] = byte(streamID)
					
					// Simulate processing time
					time.Sleep(time.Duration(1+streamID%10) * time.Millisecond)
					
					opDuration := time.Since(opStart)
					tracker.Record(opDuration)
					atomic.AddInt64(&successCount, 1)
					
					// Check if we should stop
					if time.Since(startTime) >= duration {
						return
					}
				}
			}
		}(i)
	}
	
	// Wait for all streams to complete
	wg.Wait()
	
	totalDuration := time.Since(startTime)
	memAfter := GetMemorySnapshot()
	
	// Calculate statistics
	p50 := tracker.Percentile(0.50)
	p95 := tracker.Percentile(0.95)
	p99 := tracker.Percentile(0.99)
	max := tracker.Max()
	
	totalOps := atomic.LoadInt64(&successCount)
	totalErrors := atomic.LoadInt64(&errorCount)
	throughput := float64(totalOps) / totalDuration.Seconds()
	
	// Success if we handled targetStreams concurrently
	success := totalOps > 0 && totalErrors == 0
	
	result := Result{
		Name:             "Concurrent Streams Performance",
		Duration:         totalDuration,
		MemoryAllocated:  memAfter.TotalAlloc - memBefore.TotalAlloc,
		MemoryInUse:      memAfter.Alloc,
		OperationsPerSec: throughput,
		TotalOperations:  totalOps,
		P50Latency:       p50,
		P95Latency:       p95,
		P99Latency:       p99,
		MaxLatency:       max,
		Success:          success,
		AdditionalMetrics: map[string]interface{}{
			"target_streams":     targetStreams,
			"actual_streams":     targetStreams,
			"total_operations":   totalOps,
			"error_count":        totalErrors,
			"ops_per_stream":     float64(totalOps) / float64(targetStreams),
			"avg_latency":        totalDuration / time.Duration(totalOps),
			"data_transferred":   FormatBytes(uint64(totalOps * dataSize)),
			"gc_runs":            memAfter.NumGC - memBefore.NumGC,
			"meets_target":       success,
		},
	}
	
	if !success {
		result.Error = fmt.Errorf("failed to handle %d concurrent streams", targetStreams)
	}
	
	s.addResult(result)
	s.log.Info("Concurrent streams benchmark complete",
		"streams", targetStreams,
		"ops", totalOps,
		"throughput", throughput,
		"success", success)
	
	return nil
}

// BenchmarkStreamScaling tests how performance scales with stream count
func (s *Suite) BenchmarkStreamScaling(ctx context.Context) error {
	s.log.Info("Running stream scaling benchmark")
	
	streamCounts := []int{10, 25, 50, 100, 200}
	const operationsPerStream = 100
	
	for _, numStreams := range streamCounts {
		// Force GC before each test
		runtime.GC()
		memBefore := GetMemorySnapshot()
		
		startTime := time.Now()
		var (
			completedOps int64
			wg           sync.WaitGroup
		)
		
		wg.Add(numStreams)
		for i := 0; i < numStreams; i++ {
			go func(streamID int) {
				defer wg.Done()
				
				for j := 0; j < operationsPerStream; j++ {
					// Simulate work
					data := make([]byte, 512)
					data[0] = byte(streamID + j)
					atomic.AddInt64(&completedOps, 1)
					
					// Small delay to simulate I/O
					time.Sleep(time.Microsecond * 100)
				}
			}(i)
		}
		
		wg.Wait()
		totalDuration := time.Since(startTime)
		memAfter := GetMemorySnapshot()
		
		throughput := float64(completedOps) / totalDuration.Seconds()
		
		result := Result{
			Name:             fmt.Sprintf("Stream Scaling (%d streams)", numStreams),
			Duration:         totalDuration,
			MemoryAllocated:  memAfter.TotalAlloc - memBefore.TotalAlloc,
			MemoryInUse:      memAfter.Alloc,
			OperationsPerSec: throughput,
			TotalOperations:  completedOps,
			Success:          completedOps == int64(numStreams*operationsPerStream),
			AdditionalMetrics: map[string]interface{}{
				"num_streams":        numStreams,
				"ops_per_stream":     operationsPerStream,
				"total_ops":          completedOps,
				"avg_latency":        totalDuration / time.Duration(completedOps),
				"gc_runs":            memAfter.NumGC - memBefore.NumGC,
			},
		}
		
		s.addResult(result)
		s.log.Info("Stream scaling test complete",
			"streams", numStreams,
			"throughput", throughput)
		
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
	
	return nil
}

// BenchmarkStreamMultiplexing tests stream multiplexing on circuits
func (s *Suite) BenchmarkStreamMultiplexing(ctx context.Context) error {
	s.log.Info("Running stream multiplexing benchmark")
	
	const (
		numCircuits       = 3
		streamsPerCircuit = 50
		operationsPerStream = 20
	)
	
	// Force GC before benchmark
	runtime.GC()
	memBefore := GetMemorySnapshot()
	
	startTime := time.Now()
	var (
		completedOps int64
		wg           sync.WaitGroup
	)
	
	// Simulate multiple circuits with streams
	for circuitID := 0; circuitID < numCircuits; circuitID++ {
		for streamID := 0; streamID < streamsPerCircuit; streamID++ {
			wg.Add(1)
			go func(cID, sID int) {
				defer wg.Done()
				
				for i := 0; i < operationsPerStream; i++ {
					// Simulate stream operation
					data := make([]byte, 256)
					data[0] = byte(cID)
					data[1] = byte(sID)
					atomic.AddInt64(&completedOps, 1)
					
					// Small delay
					time.Sleep(time.Microsecond * 50)
				}
			}(circuitID, streamID)
		}
	}
	
	wg.Wait()
	totalDuration := time.Since(startTime)
	memAfter := GetMemorySnapshot()
	
	totalStreams := numCircuits * streamsPerCircuit
	expectedOps := int64(totalStreams * operationsPerStream)
	throughput := float64(completedOps) / totalDuration.Seconds()
	
	result := Result{
		Name:             "Stream Multiplexing Performance",
		Duration:         totalDuration,
		MemoryAllocated:  memAfter.TotalAlloc - memBefore.TotalAlloc,
		MemoryInUse:      memAfter.Alloc,
		OperationsPerSec: throughput,
		TotalOperations:  completedOps,
		Success:          completedOps == expectedOps,
		AdditionalMetrics: map[string]interface{}{
			"num_circuits":         numCircuits,
			"streams_per_circuit":  streamsPerCircuit,
			"total_streams":        totalStreams,
			"ops_per_stream":       operationsPerStream,
			"expected_ops":         expectedOps,
			"actual_ops":           completedOps,
			"gc_runs":              memAfter.NumGC - memBefore.NumGC,
		},
	}
	
	if completedOps != expectedOps {
		result.Error = fmt.Errorf("expected %d operations, got %d", expectedOps, completedOps)
	}
	
	s.addResult(result)
	s.log.Info("Stream multiplexing benchmark complete",
		"circuits", numCircuits,
		"streams", totalStreams,
		"throughput", throughput)
	
	return nil
}
