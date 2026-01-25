// Package main demonstrates stream backpressure functionality.
//
// This example shows how stream backpressure prevents memory exhaustion
// by pausing writes when buffer utilization exceeds thresholds.
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/opd-ai/go-tor/pkg/config"
	"github.com/opd-ai/go-tor/pkg/metrics"
	"github.com/opd-ai/go-tor/pkg/stream"
)

func main() {
	fmt.Println("=== Stream Backpressure Demo ===\n")

	// Configure backpressure with low thresholds for demonstration
	cfg := config.DefaultConfig()
	cfg.StreamBufferHighWaterMark = 5000  // 5KB high water mark
	cfg.StreamBufferLowWaterMark = 2000   // 2KB low water mark

	fmt.Printf("Configuration:\n")
	fmt.Printf("  High Water Mark: %d bytes\n", cfg.StreamBufferHighWaterMark)
	fmt.Printf("  Low Water Mark:  %d bytes\n", cfg.StreamBufferLowWaterMark)
	fmt.Printf("  Hysteresis Ratio: %.2f:1\n\n", 
		float64(cfg.StreamBufferHighWaterMark)/float64(cfg.StreamBufferLowWaterMark))

	// Create metrics tracker
	m := metrics.New()

	// Create backpressure controller
	bp := stream.NewBackpressureState(cfg, m)

	// Create stream and attach backpressure
	s := stream.NewStream(1, 100, "example.com", 443, nil)
	s.SetBackpressure(bp)
	s.SetState(stream.StateConnected)

	fmt.Println("=== Scenario 1: Normal Operation ===")
	
	// Send small amounts of data - should succeed
	data1 := make([]byte, 1000)
	if err := s.Send(data1); err != nil {
		fmt.Printf("ERROR: Send(1000 bytes) failed: %v\n", err)
	} else {
		fmt.Printf("✓ Sent 1000 bytes\n")
	}
	
	fmt.Printf("  Buffer size: %d bytes\n", s.GetSendBufferSize())
	fmt.Printf("  Backpressure paused: %v\n\n", bp.IsSendPaused())

	// Send more data
	data2 := make([]byte, 1500)
	if err := s.Send(data2); err != nil {
		fmt.Printf("ERROR: Send(1500 bytes) failed: %v\n", err)
	} else {
		fmt.Printf("✓ Sent 1500 bytes\n")
	}
	
	fmt.Printf("  Buffer size: %d bytes\n", s.GetSendBufferSize())
	fmt.Printf("  Backpressure paused: %v\n\n", bp.IsSendPaused())

	fmt.Println("=== Scenario 2: Backpressure Triggered ===")
	
	// Try to send data that would exceed high water mark
	data3 := make([]byte, 3000)
	if err := s.Send(data3); err != nil {
		fmt.Printf("✗ Send(3000 bytes) failed: %v\n", err)
		fmt.Printf("  This is expected - backpressure applied!\n")
	} else {
		fmt.Printf("ERROR: Send should have failed\n")
	}
	
	fmt.Printf("  Buffer size: %d bytes\n", s.GetSendBufferSize())
	fmt.Printf("  Backpressure paused: %v\n", bp.IsSendPaused())
	
	// Check metrics
	snapshot := m.Snapshot()
	fmt.Printf("  Backpressure pauses: %d\n\n", snapshot.BackpressurePauses)

	fmt.Println("=== Scenario 3: Hysteresis Behavior ===")
	
	// Consume some data, but not enough to drop below low water mark
	ctx := context.Background()
	consumed, _ := s.SendData(ctx)
	fmt.Printf("✓ Consumed %d bytes\n", len(consumed))
	fmt.Printf("  Buffer size: %d bytes (still above low water mark)\n", s.GetSendBufferSize())
	fmt.Printf("  Backpressure paused: %v (hysteresis prevents immediate resume)\n\n", bp.IsSendPaused())
	
	// Try to send - should still fail due to hysteresis
	data4 := make([]byte, 1000)
	if err := s.Send(data4); err != nil {
		fmt.Printf("✗ Send(1000 bytes) failed: %v\n", err)
		fmt.Printf("  Hysteresis keeps backpressure active\n\n")
	}

	fmt.Println("=== Scenario 4: Backpressure Released ===")
	
	// Consume more data to drop below low water mark
	consumed, _ = s.SendData(ctx)
	fmt.Printf("✓ Consumed %d bytes\n", len(consumed))
	fmt.Printf("  Buffer size: %d bytes (below low water mark)\n", s.GetSendBufferSize())
	fmt.Printf("  Backpressure paused: %v (released!)\n", bp.IsSendPaused())
	
	// Check metrics
	snapshot = m.Snapshot()
	fmt.Printf("  Backpressure resumes: %d\n\n", snapshot.BackpressureResumes)
	
	// Should be able to send again
	data5 := make([]byte, 1000)
	if err := s.Send(data5); err != nil {
		fmt.Printf("ERROR: Send(1000 bytes) failed: %v\n", err)
	} else {
		fmt.Printf("✓ Sent 1000 bytes (backpressure released)\n")
	}
	
	fmt.Printf("  Buffer size: %d bytes\n\n", s.GetSendBufferSize())

	fmt.Println("=== Scenario 5: Multiple Cycles ===")
	
	// Simulate multiple pause/resume cycles
	for cycle := 1; cycle <= 3; cycle++ {
		fmt.Printf("Cycle %d:\n", cycle)
		
		// Fill buffer to trigger backpressure
		bigData := make([]byte, 4000)
		if err := s.Send(bigData); err != nil {
			fmt.Printf("  ✗ Backpressure applied at %d bytes\n", s.GetSendBufferSize())
		}
		
		// Drain buffer to release backpressure
		for s.GetSendBufferSize() > 0 {
			s.SendData(ctx)
			time.Sleep(10 * time.Millisecond)
		}
		fmt.Printf("  ✓ Backpressure released at %d bytes\n", s.GetSendBufferSize())
	}
	
	// Final metrics
	snapshot = m.Snapshot()
	fmt.Printf("\n=== Final Metrics ===\n")
	fmt.Printf("Total backpressure pauses:  %d\n", snapshot.BackpressurePauses)
	fmt.Printf("Total backpressure resumes: %d\n", snapshot.BackpressureResumes)
	fmt.Printf("Pause/Resume ratio:         %.2f:1\n", 
		float64(snapshot.BackpressurePauses)/float64(snapshot.BackpressureResumes))

	fmt.Println("\n=== Demo Complete ===")
	fmt.Println("\nKey Takeaways:")
	fmt.Println("1. Backpressure prevents memory exhaustion by rejecting writes when buffer is full")
	fmt.Println("2. Hysteresis (high/low water marks) prevents rapid pause/resume oscillation")
	fmt.Println("3. Independent send/receive buffers allow fine-grained control")
	fmt.Println("4. Metrics tracking enables monitoring and tuning")
	fmt.Println("\nSee docs/STREAM_BACKPRESSURE.md for detailed documentation.")
}
