// Package stream provides stream backpressure tests.
package stream

import (
	"testing"

	"github.com/opd-ai/go-tor/pkg/config"
	"github.com/opd-ai/go-tor/pkg/metrics"
)

// TestBackpressureStateInitialization tests creation with default values
func TestBackpressureStateInitialization(t *testing.T) {
	m := metrics.New()
	cfg := config.DefaultConfig()

	bp := NewBackpressureState(cfg, m)

	if bp.GetHighWaterMark() != 65536 {
		t.Errorf("HighWaterMark = %d, want 65536", bp.GetHighWaterMark())
	}

	if bp.GetLowWaterMark() != 16384 {
		t.Errorf("LowWaterMark = %d, want 16384", bp.GetLowWaterMark())
	}

	if bp.IsSendPaused() {
		t.Error("IsSendPaused() = true, want false initially")
	}

	if bp.IsRecvPaused() {
		t.Error("IsRecvPaused() = false, want false initially")
	}
}

// TestBackpressureStateCustomConfig tests creation with custom config
func TestBackpressureStateCustomConfig(t *testing.T) {
	m := metrics.New()
	cfg := config.DefaultConfig()
	cfg.StreamBufferHighWaterMark = 100000
	cfg.StreamBufferLowWaterMark = 25000

	bp := NewBackpressureState(cfg, m)

	if bp.GetHighWaterMark() != 100000 {
		t.Errorf("HighWaterMark = %d, want 100000", bp.GetHighWaterMark())
	}

	if bp.GetLowWaterMark() != 25000 {
		t.Errorf("LowWaterMark = %d, want 25000", bp.GetLowWaterMark())
	}
}

// TestBackpressureStateNilConfig tests creation with nil config
func TestBackpressureStateNilConfig(t *testing.T) {
	m := metrics.New()

	bp := NewBackpressureState(nil, m)

	// Should use defaults
	if bp.GetHighWaterMark() != 65536 {
		t.Errorf("HighWaterMark = %d, want 65536", bp.GetHighWaterMark())
	}

	if bp.GetLowWaterMark() != 16384 {
		t.Errorf("LowWaterMark = %d, want 16384", bp.GetLowWaterMark())
	}
}

// TestSendBackpressureApply tests applying send backpressure
func TestSendBackpressureApply(t *testing.T) {
	m := metrics.New()
	cfg := config.DefaultConfig()
	bp := NewBackpressureState(cfg, m)

	// Buffer below high water mark - no backpressure
	paused := bp.CheckSendBuffer(50000)
	if paused {
		t.Error("CheckSendBuffer(50000) = true, want false")
	}

	// Buffer at high water mark - apply backpressure
	paused = bp.CheckSendBuffer(65536)
	if !paused {
		t.Error("CheckSendBuffer(65536) = false, want true")
	}

	if !bp.IsSendPaused() {
		t.Error("IsSendPaused() = false after exceeding high water mark")
	}

	// Check metrics
	snapshot := m.Snapshot()
	if snapshot.BackpressurePauses != 1 {
		t.Errorf("BackpressurePauses = %d, want 1", snapshot.BackpressurePauses)
	}
}

// TestSendBackpressureRelease tests releasing send backpressure
func TestSendBackpressureRelease(t *testing.T) {
	m := metrics.New()
	cfg := config.DefaultConfig()
	bp := NewBackpressureState(cfg, m)

	// Apply backpressure first
	bp.CheckSendBuffer(65536)

	// Buffer still above low water mark - backpressure remains
	paused := bp.CheckSendBuffer(20000)
	if !paused {
		t.Error("CheckSendBuffer(20000) = false, want true (still paused)")
	}

	// Buffer drops to low water mark - release backpressure
	paused = bp.CheckSendBuffer(16384)
	if paused {
		t.Error("CheckSendBuffer(16384) = true, want false (released)")
	}

	if bp.IsSendPaused() {
		t.Error("IsSendPaused() = true after dropping to low water mark")
	}

	// Check metrics
	snapshot := m.Snapshot()
	if snapshot.BackpressurePauses != 1 {
		t.Errorf("BackpressurePauses = %d, want 1", snapshot.BackpressurePauses)
	}
	if snapshot.BackpressureResumes != 1 {
		t.Errorf("BackpressureResumes = %d, want 1", snapshot.BackpressureResumes)
	}
}

// TestRecvBackpressureApply tests applying receive backpressure
func TestRecvBackpressureApply(t *testing.T) {
	m := metrics.New()
	cfg := config.DefaultConfig()
	bp := NewBackpressureState(cfg, m)

	// Buffer below high water mark - no backpressure
	paused := bp.CheckRecvBuffer(50000)
	if paused {
		t.Error("CheckRecvBuffer(50000) = true, want false")
	}

	// Buffer at high water mark - apply backpressure
	paused = bp.CheckRecvBuffer(65536)
	if !paused {
		t.Error("CheckRecvBuffer(65536) = false, want true")
	}

	if !bp.IsRecvPaused() {
		t.Error("IsRecvPaused() = false after exceeding high water mark")
	}
}

// TestRecvBackpressureRelease tests releasing receive backpressure
func TestRecvBackpressureRelease(t *testing.T) {
	m := metrics.New()
	cfg := config.DefaultConfig()
	bp := NewBackpressureState(cfg, m)

	// Apply backpressure first
	bp.CheckRecvBuffer(65536)

	// Buffer still above low water mark - backpressure remains
	paused := bp.CheckRecvBuffer(20000)
	if !paused {
		t.Error("CheckRecvBuffer(20000) = false, want true (still paused)")
	}

	// Buffer drops to low water mark - release backpressure
	paused = bp.CheckRecvBuffer(16384)
	if paused {
		t.Error("CheckRecvBuffer(16384) = true, want false (released)")
	}

	if bp.IsRecvPaused() {
		t.Error("IsRecvPaused() = true after dropping to low water mark")
	}
}

// TestBackpressureHysteresis tests hysteresis behavior
func TestBackpressureHysteresis(t *testing.T) {
	m := metrics.New()
	cfg := config.DefaultConfig()
	bp := NewBackpressureState(cfg, m)

	// Apply backpressure
	bp.CheckSendBuffer(65536)

	// Buffer at intermediate value - should remain paused
	paused := bp.CheckSendBuffer(30000)
	if !paused {
		t.Error("CheckSendBuffer(30000) = false, want true (hysteresis)")
	}

	// Only releases at low water mark
	paused = bp.CheckSendBuffer(16384)
	if paused {
		t.Error("CheckSendBuffer(16384) = true, want false")
	}
}

// TestBackpressureReset tests reset functionality
func TestBackpressureReset(t *testing.T) {
	m := metrics.New()
	cfg := config.DefaultConfig()
	bp := NewBackpressureState(cfg, m)

	// Apply both send and recv backpressure
	bp.CheckSendBuffer(65536)
	bp.CheckRecvBuffer(65536)

	if !bp.IsSendPaused() || !bp.IsRecvPaused() {
		t.Error("Expected both send and recv to be paused")
	}

	// Reset
	bp.Reset()

	if bp.IsSendPaused() {
		t.Error("IsSendPaused() = true after reset, want false")
	}

	if bp.IsRecvPaused() {
		t.Error("IsRecvPaused() = true after reset, want false")
	}
}

// TestBackpressureMultipleCycles tests multiple pause/resume cycles
func TestBackpressureMultipleCycles(t *testing.T) {
	m := metrics.New()
	cfg := config.DefaultConfig()
	bp := NewBackpressureState(cfg, m)

	// Cycle 1: Pause
	bp.CheckSendBuffer(65536)
	if !bp.IsSendPaused() {
		t.Error("Cycle 1: Expected send to be paused")
	}

	// Cycle 1: Resume
	bp.CheckSendBuffer(16384)
	if bp.IsSendPaused() {
		t.Error("Cycle 1: Expected send to be resumed")
	}

	// Cycle 2: Pause
	bp.CheckSendBuffer(70000)
	if !bp.IsSendPaused() {
		t.Error("Cycle 2: Expected send to be paused")
	}

	// Cycle 2: Resume
	bp.CheckSendBuffer(10000)
	if bp.IsSendPaused() {
		t.Error("Cycle 2: Expected send to be resumed")
	}

	// Check metrics
	snapshot := m.Snapshot()
	if snapshot.BackpressurePauses != 2 {
		t.Errorf("BackpressurePauses = %d, want 2", snapshot.BackpressurePauses)
	}
	if snapshot.BackpressureResumes != 2 {
		t.Errorf("BackpressureResumes = %d, want 2", snapshot.BackpressureResumes)
	}
}

// TestBackpressureIndependentSendRecv tests send and recv are independent
func TestBackpressureIndependentSendRecv(t *testing.T) {
	m := metrics.New()
	cfg := config.DefaultConfig()
	bp := NewBackpressureState(cfg, m)

	// Pause send only
	bp.CheckSendBuffer(65536)

	if !bp.IsSendPaused() {
		t.Error("Expected send to be paused")
	}

	if bp.IsRecvPaused() {
		t.Error("Expected recv to remain unpaused")
	}

	// Pause recv only
	bp.CheckRecvBuffer(65536)

	if !bp.IsRecvPaused() {
		t.Error("Expected recv to be paused")
	}

	// Resume send, recv remains paused
	bp.CheckSendBuffer(16384)

	if bp.IsSendPaused() {
		t.Error("Expected send to be resumed")
	}

	if !bp.IsRecvPaused() {
		t.Error("Expected recv to remain paused")
	}
}

// TestBackpressureNilMetrics tests operation with nil metrics
func TestBackpressureNilMetrics(t *testing.T) {
	cfg := config.DefaultConfig()
	bp := NewBackpressureState(cfg, nil)

	// Should not panic with nil metrics
	paused := bp.CheckSendBuffer(65536)
	if !paused {
		t.Error("CheckSendBuffer(65536) = false, want true")
	}

	paused = bp.CheckSendBuffer(16384)
	if paused {
		t.Error("CheckSendBuffer(16384) = true, want false")
	}
}

// TestBackpressureEdgeCases tests edge cases
func TestBackpressureEdgeCases(t *testing.T) {
	m := metrics.New()
	cfg := config.DefaultConfig()
	bp := NewBackpressureState(cfg, m)

	// Exactly at high water mark
	paused := bp.CheckSendBuffer(65536)
	if !paused {
		t.Error("CheckSendBuffer(exactly at high water mark) should pause")
	}

	// Exactly at low water mark
	paused = bp.CheckSendBuffer(16384)
	if paused {
		t.Error("CheckSendBuffer(exactly at low water mark) should resume")
	}

	// Buffer size of 0
	bp.Reset()
	paused = bp.CheckSendBuffer(0)
	if paused {
		t.Error("CheckSendBuffer(0) = true, want false")
	}
}

// TestBackpressureConcurrency tests concurrent access
func TestBackpressureConcurrency(t *testing.T) {
	m := metrics.New()
	cfg := config.DefaultConfig()
	bp := NewBackpressureState(cfg, m)

	done := make(chan bool, 4)

	// Concurrent send checks
	go func() {
		for i := 0; i < 100; i++ {
			bp.CheckSendBuffer(50000)
		}
		done <- true
	}()

	// Concurrent recv checks
	go func() {
		for i := 0; i < 100; i++ {
			bp.CheckRecvBuffer(50000)
		}
		done <- true
	}()

	// Concurrent pause/resume cycles
	go func() {
		for i := 0; i < 50; i++ {
			bp.CheckSendBuffer(65536)
			bp.CheckSendBuffer(16384)
		}
		done <- true
	}()

	// Concurrent reset
	go func() {
		for i := 0; i < 50; i++ {
			bp.Reset()
		}
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 4; i++ {
		<-done
	}

	// Should not panic - that's the main test
}
