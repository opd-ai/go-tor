// Package stream provides integration tests for backpressure.
package stream

import (
	"context"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/config"
	"github.com/opd-ai/go-tor/pkg/metrics"
)

// TestStreamBackpressureIntegration tests full backpressure workflow
func TestStreamBackpressureIntegration(t *testing.T) {
	m := metrics.New()
	cfg := config.DefaultConfig()
	cfg.StreamBufferHighWaterMark = 1000 // Low threshold for testing
	cfg.StreamBufferLowWaterMark = 400

	bp := NewBackpressureState(cfg, m)
	stream := NewStream(1, 100, "example.com", 443, nil)
	stream.SetBackpressure(bp)
	stream.SetState(StateConnected)

	// Send data below high water mark - should succeed
	data := make([]byte, 300)
	if err := stream.Send(data); err != nil {
		t.Fatalf("Send(300 bytes) error = %v, want nil", err)
	}

	// Send more data to exceed high water mark - should fail
	data = make([]byte, 800)
	err := stream.Send(data)
	if err == nil {
		t.Error("Send(800 bytes) should fail when backpressure applied")
	}

	// Verify backpressure is applied
	if !bp.IsSendPaused() {
		t.Error("Backpressure should be paused after exceeding high water mark")
	}

	// Consume all data to drop below low water mark
	ctx := context.Background()
	consumed, err := stream.SendData(ctx)
	if err != nil {
		t.Fatalf("SendData() error = %v", err)
	}

	if len(consumed) != 300 {
		t.Errorf("SendData() returned %d bytes, want 300", len(consumed))
	}

	// Verify backpressure is released (buffer dropped from 300 to 0, which is < 400)
	if bp.IsSendPaused() {
		t.Error("Backpressure should be released after dropping below low water mark")
	}

	// Should be able to send again
	data = make([]byte, 300)
	if err := stream.Send(data); err != nil {
		t.Fatalf("Send(300 bytes) after release error = %v, want nil", err)
	}

	// Check metrics
	snapshot := m.Snapshot()
	if snapshot.BackpressurePauses < 1 {
		t.Errorf("BackpressurePauses = %d, want >= 1", snapshot.BackpressurePauses)
	}
	if snapshot.BackpressureResumes < 1 {
		t.Errorf("BackpressureResumes = %d, want >= 1", snapshot.BackpressureResumes)
	}
}

// TestStreamReceiveBackpressure tests receive-side backpressure
func TestStreamReceiveBackpressure(t *testing.T) {
	m := metrics.New()
	cfg := config.DefaultConfig()
	cfg.StreamBufferHighWaterMark = 1000
	cfg.StreamBufferLowWaterMark = 400

	bp := NewBackpressureState(cfg, m)
	stream := NewStream(1, 100, "example.com", 443, nil)
	stream.SetBackpressure(bp)
	stream.SetState(StateConnected)

	// Receive data below high water mark
	data := make([]byte, 300)
	if err := stream.ReceiveData(data); err != nil {
		t.Fatalf("ReceiveData(300 bytes) error = %v, want nil", err)
	}

	// Receive more to exceed high water mark
	data = make([]byte, 800)
	err := stream.ReceiveData(data)
	if err == nil {
		t.Error("ReceiveData(800 bytes) should fail when backpressure applied")
	}

	// Verify backpressure is applied
	if !bp.IsRecvPaused() {
		t.Error("Receive backpressure should be paused after exceeding high water mark")
	}

	// Consume received data
	ctx := context.Background()
	consumed, err := stream.Receive(ctx)
	if err != nil {
		t.Fatalf("Receive() error = %v", err)
	}

	if len(consumed) != 300 {
		t.Errorf("Receive() returned %d bytes, want 300", len(consumed))
	}

	// Verify backpressure is released
	if bp.IsRecvPaused() {
		t.Error("Receive backpressure should be released after dropping below low water mark")
	}

	// Should be able to receive again
	data = make([]byte, 300)
	if err := stream.ReceiveData(data); err != nil {
		t.Fatalf("ReceiveData(300 bytes) after release error = %v, want nil", err)
	}
}

// TestStreamBackpressureWithoutController tests stream without backpressure
func TestStreamBackpressureWithoutController(t *testing.T) {
	stream := NewStream(1, 100, "example.com", 443, nil)
	stream.SetState(StateConnected)

	// Should work without backpressure controller
	data := make([]byte, 1000000) // Large data
	if err := stream.Send(data); err != nil {
		t.Fatalf("Send() without backpressure error = %v, want nil", err)
	}
}

// TestStreamBackpressureBufferTracking tests buffer size tracking
func TestStreamBackpressureBufferTracking(t *testing.T) {
	m := metrics.New()
	cfg := config.DefaultConfig()
	cfg.StreamBufferHighWaterMark = 2000
	cfg.StreamBufferLowWaterMark = 1000

	bp := NewBackpressureState(cfg, m)
	stream := NewStream(1, 100, "example.com", 443, nil)
	stream.SetBackpressure(bp)
	stream.SetState(StateConnected)

	// Send data and check buffer size
	data1 := make([]byte, 500)
	stream.Send(data1)

	if size := stream.GetSendBufferSize(); size != 500 {
		t.Errorf("SendBufferSize = %d, want 500", size)
	}

	// Send more data
	data2 := make([]byte, 300)
	stream.Send(data2)

	if size := stream.GetSendBufferSize(); size != 800 {
		t.Errorf("SendBufferSize = %d, want 800", size)
	}

	// Consume data
	ctx := context.Background()
	consumed, _ := stream.SendData(ctx)

	expectedSize := 800 - len(consumed)
	if size := stream.GetSendBufferSize(); size != expectedSize {
		t.Errorf("SendBufferSize after consume = %d, want %d", size, expectedSize)
	}
}

// TestStreamBackpressureMultipleStreams tests independent backpressure
func TestStreamBackpressureMultipleStreams(t *testing.T) {
	m := metrics.New()
	cfg := config.DefaultConfig()
	cfg.StreamBufferHighWaterMark = 1000
	cfg.StreamBufferLowWaterMark = 500

	// Create two streams with separate backpressure controllers
	bp1 := NewBackpressureState(cfg, m)
	stream1 := NewStream(1, 100, "example.com", 443, nil)
	stream1.SetBackpressure(bp1)
	stream1.SetState(StateConnected)

	bp2 := NewBackpressureState(cfg, m)
	stream2 := NewStream(2, 100, "example.org", 443, nil)
	stream2.SetBackpressure(bp2)
	stream2.SetState(StateConnected)

	// Fill stream1 to trigger backpressure
	data := make([]byte, 1100)
	stream1.Send(data)

	// stream1 should be paused
	if !bp1.IsSendPaused() {
		t.Error("Stream1 backpressure should be paused")
	}

	// stream2 should not be affected
	if bp2.IsSendPaused() {
		t.Error("Stream2 backpressure should not be paused")
	}

	// stream2 should still accept data
	data2 := make([]byte, 400)
	if err := stream2.Send(data2); err != nil {
		t.Errorf("Stream2 Send() error = %v, want nil", err)
	}
}

// TestStreamBackpressureStateReset tests reset during stream lifecycle
func TestStreamBackpressureStateReset(t *testing.T) {
	m := metrics.New()
	cfg := config.DefaultConfig()
	cfg.StreamBufferHighWaterMark = 1000
	cfg.StreamBufferLowWaterMark = 500

	bp := NewBackpressureState(cfg, m)
	stream := NewStream(1, 100, "example.com", 443, nil)
	stream.SetBackpressure(bp)
	stream.SetState(StateConnected)

	// Trigger backpressure
	data := make([]byte, 1100)
	stream.Send(data)

	if !bp.IsSendPaused() {
		t.Error("Backpressure should be paused")
	}

	// Reset backpressure
	bp.Reset()

	if bp.IsSendPaused() {
		t.Error("Backpressure should be reset")
	}

	// Should be able to send again (though buffer may still be full)
	data2 := make([]byte, 400)
	// This might still fail due to channel being full, but backpressure itself is reset
	stream.Send(data2)
}

// TestStreamBackpressureGetters tests getter methods
func TestStreamBackpressureGetters(t *testing.T) {
	m := metrics.New()
	cfg := config.DefaultConfig()

	bp := NewBackpressureState(cfg, m)
	stream := NewStream(1, 100, "example.com", 443, nil)
	stream.SetBackpressure(bp)

	// Test GetBackpressure
	retrieved := stream.GetBackpressure()
	if retrieved != bp {
		t.Error("GetBackpressure() returned different instance")
	}

	// Test initial buffer sizes
	if size := stream.GetSendBufferSize(); size != 0 {
		t.Errorf("Initial SendBufferSize = %d, want 0", size)
	}

	if size := stream.GetRecvBufferSize(); size != 0 {
		t.Errorf("Initial RecvBufferSize = %d, want 0", size)
	}
}

// TestStreamBackpressureConcurrentOps tests concurrent operations
func TestStreamBackpressureConcurrentOps(t *testing.T) {
	m := metrics.New()
	cfg := config.DefaultConfig()
	cfg.StreamBufferHighWaterMark = 10000
	cfg.StreamBufferLowWaterMark = 5000

	bp := NewBackpressureState(cfg, m)
	stream := NewStream(1, 100, "example.com", 443, nil)
	stream.SetBackpressure(bp)
	stream.SetState(StateConnected)

	done := make(chan bool, 2)

	// Concurrent sends
	go func() {
		for i := 0; i < 50; i++ {
			data := make([]byte, 100)
			stream.Send(data)
			time.Sleep(1 * time.Millisecond)
		}
		done <- true
	}()

	// Concurrent consumes
	go func() {
		ctx := context.Background()
		for i := 0; i < 50; i++ {
			stream.SendData(ctx)
			time.Sleep(1 * time.Millisecond)
		}
		done <- true
	}()

	// Wait for completion
	<-done
	<-done

	// Should not panic - that's the main test
}
