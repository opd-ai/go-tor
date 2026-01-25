// Package stream provides stream backpressure implementation.
//
// Backpressure prevents memory exhaustion by pausing reads/writes when
// buffer utilization exceeds high water mark and resuming when it drops
// below low water mark. This implements a hysteresis mechanism to avoid
// oscillation.
package stream

import (
	"sync/atomic"

	"github.com/opd-ai/go-tor/pkg/config"
	"github.com/opd-ai/go-tor/pkg/metrics"
)

// BackpressureState tracks the current backpressure state
type BackpressureState struct {
	sendPaused    atomic.Bool // True when send is paused
	recvPaused    atomic.Bool // True when receive is paused
	highWaterMark int         // Threshold to apply backpressure
	lowWaterMark  int         // Threshold to release backpressure
	metrics       *metrics.Metrics
}

// NewBackpressureState creates a new backpressure state tracker
func NewBackpressureState(cfg *config.Config, m *metrics.Metrics) *BackpressureState {
	highWater := 65536 // Default from config
	lowWater := 16384  // Default from config

	if cfg != nil {
		if cfg.StreamBufferHighWaterMark > 0 {
			highWater = cfg.StreamBufferHighWaterMark
		}
		if cfg.StreamBufferLowWaterMark > 0 {
			lowWater = cfg.StreamBufferLowWaterMark
		}
	}

	return &BackpressureState{
		highWaterMark: highWater,
		lowWaterMark:  lowWater,
		metrics:       m,
	}
}

// CheckSendBuffer evaluates send buffer size and applies/releases backpressure
// Returns true if sending should be paused
func (b *BackpressureState) CheckSendBuffer(bufferSize int) bool {
	isPaused := b.sendPaused.Load()

	if !isPaused && bufferSize >= b.highWaterMark {
		// Apply backpressure
		b.sendPaused.Store(true)
		if b.metrics != nil {
			b.metrics.RecordBackpressure(true)
		}
		return true
	}

	if isPaused && bufferSize <= b.lowWaterMark {
		// Release backpressure
		b.sendPaused.Store(false)
		if b.metrics != nil {
			b.metrics.RecordBackpressure(false)
		}
		return false
	}

	return isPaused
}

// CheckRecvBuffer evaluates receive buffer size and applies/releases backpressure
// Returns true if receiving should be paused
func (b *BackpressureState) CheckRecvBuffer(bufferSize int) bool {
	isPaused := b.recvPaused.Load()

	if !isPaused && bufferSize >= b.highWaterMark {
		// Apply backpressure
		b.recvPaused.Store(true)
		if b.metrics != nil {
			b.metrics.RecordBackpressure(true)
		}
		return true
	}

	if isPaused && bufferSize <= b.lowWaterMark {
		// Release backpressure
		b.recvPaused.Store(false)
		if b.metrics != nil {
			b.metrics.RecordBackpressure(false)
		}
		return false
	}

	return isPaused
}

// IsSendPaused returns true if send is currently paused
func (b *BackpressureState) IsSendPaused() bool {
	return b.sendPaused.Load()
}

// IsRecvPaused returns true if receive is currently paused
func (b *BackpressureState) IsRecvPaused() bool {
	return b.recvPaused.Load()
}

// Reset clears the backpressure state
func (b *BackpressureState) Reset() {
	b.sendPaused.Store(false)
	b.recvPaused.Store(false)
}

// GetHighWaterMark returns the high water mark threshold
func (b *BackpressureState) GetHighWaterMark() int {
	return b.highWaterMark
}

// GetLowWaterMark returns the low water mark threshold
func (b *BackpressureState) GetLowWaterMark() int {
	return b.lowWaterMark
}
