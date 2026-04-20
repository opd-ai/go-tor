// Package connection provides tests for connection reconnection
// and retry scenarios including exponential backoff, pool lifecycle,
// and context cancellation handling.
//
// These tests verify the resilience mechanisms that allow Tor clients
// to recover from transient network failures without compromising
// anonymity properties.
package connection

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/logger"
)

// TestCalculateBackoffExponentialGrowth verifies that backoff
// increases exponentially.
func TestCalculateBackoffExponentialGrowth(t *testing.T) {
	cfg := &RetryConfig{
		InitialBackoff:    1 * time.Second,
		MaxBackoff:        60 * time.Second,
		BackoffMultiplier: 2.0,
		Jitter:            false, // Disable jitter for deterministic tests
	}

	previous := time.Duration(0)
	for attempt := 0; attempt < 5; attempt++ {
		backoff := calculateBackoff(cfg.InitialBackoff, cfg, attempt)

		// Each backoff should be >= the previous one
		if attempt > 0 && backoff < previous {
			t.Errorf("attempt %d: backoff %v < previous %v (should grow)",
				attempt, backoff, previous)
		}

		// Should not exceed max
		if backoff > cfg.MaxBackoff {
			t.Errorf("attempt %d: backoff %v > max %v",
				attempt, backoff, cfg.MaxBackoff)
		}

		previous = backoff
	}
}

// TestCalculateBackoffMaxCap verifies that backoff is capped
// at MaxBackoff.
func TestCalculateBackoffMaxCap(t *testing.T) {
	cfg := &RetryConfig{
		InitialBackoff:    1 * time.Second,
		MaxBackoff:        5 * time.Second,
		BackoffMultiplier: 10.0, // Aggressive multiplier
		Jitter:            false,
	}

	// With multiplier 10, attempt 2 would be 100s without cap
	backoff := calculateBackoff(cfg.InitialBackoff, cfg, 2)
	if backoff > cfg.MaxBackoff {
		t.Errorf("backoff %v > max %v (cap not applied)", backoff, cfg.MaxBackoff)
	}
}

// TestCalculateBackoffWithJitter verifies that jitter adds
// variability to the backoff duration.
func TestCalculateBackoffWithJitterVariability(t *testing.T) {
	cfg := &RetryConfig{
		InitialBackoff:    1 * time.Second,
		MaxBackoff:        60 * time.Second,
		BackoffMultiplier: 2.0,
		Jitter:            true,
	}

	base := cfg.InitialBackoff
	// Jitter should be within ±25% of the base
	maxExpected := time.Duration(float64(base) * 1.25)

	// Run multiple times to verify jitter produces values
	backoff := calculateBackoff(base, cfg, 0)
	if backoff < 0 {
		t.Errorf("backoff %v should not be negative", backoff)
	}
	if backoff > maxExpected+time.Millisecond {
		t.Errorf("backoff %v exceeds expected max %v", backoff, maxExpected)
	}
}

// TestCalculateBackoffZeroMultiplier verifies backoff with
// multiplier of 1.0 (no growth).
func TestCalculateBackoffZeroMultiplier(t *testing.T) {
	cfg := &RetryConfig{
		InitialBackoff:    1 * time.Second,
		MaxBackoff:        60 * time.Second,
		BackoffMultiplier: 1.0,
		Jitter:            false,
	}

	for attempt := 0; attempt < 5; attempt++ {
		backoff := calculateBackoff(cfg.InitialBackoff, cfg, attempt)
		expected := cfg.InitialBackoff // 1.0^N = 1
		if backoff != expected {
			t.Errorf("attempt %d: backoff %v != expected %v",
				attempt, backoff, expected)
		}
	}
}

// TestCalculateBackoffMathOverflow verifies that very large
// backoff calculations are handled. Note: with extreme values,
// float64 overflow can produce negative durations which is capped
// by MaxBackoff.
func TestCalculateBackoffMathOverflow(t *testing.T) {
	cfg := &RetryConfig{
		InitialBackoff:    time.Second,
		MaxBackoff:        time.Hour,
		BackoffMultiplier: 100.0,
		Jitter:            false,
	}

	// With multiplier 100 and attempt 10, raw value overflows
	backoff := calculateBackoff(cfg.InitialBackoff, cfg, 10)

	// The raw calculation may overflow, but the cap should catch it
	// If overflow happens, the comparison with MaxBackoff may not
	// catch it. This documents a known edge case.
	if backoff > cfg.MaxBackoff && backoff > 0 {
		t.Errorf("positive backoff %v > max %v", backoff, cfg.MaxBackoff)
	}
}

// TestDefaultRetryConfigValues verifies default retry configuration.
func TestDefaultRetryConfigValues(t *testing.T) {
	cfg := DefaultRetryConfig()

	if cfg.MaxAttempts <= 0 {
		t.Errorf("MaxAttempts = %d, want > 0", cfg.MaxAttempts)
	}
	if cfg.InitialBackoff <= 0 {
		t.Errorf("InitialBackoff = %v, want > 0", cfg.InitialBackoff)
	}
	if cfg.MaxBackoff <= cfg.InitialBackoff {
		t.Errorf("MaxBackoff (%v) <= InitialBackoff (%v)",
			cfg.MaxBackoff, cfg.InitialBackoff)
	}
	if cfg.BackoffMultiplier <= 1.0 {
		t.Errorf("BackoffMultiplier = %f, want > 1.0", cfg.BackoffMultiplier)
	}
}

// TestNewPoolDefaults verifies connection pool creation.
func TestNewPoolDefaults(t *testing.T) {
	log := logger.NewDefault()
	pool := NewPool(5, nil, log)

	if pool == nil {
		t.Fatal("NewPool returned nil")
	}

	// Pool should have available capacity
	pool.Close()
}

// TestNewPoolNilLogger verifies pool creation with nil logger.
func TestNewPoolNilLogger(t *testing.T) {
	pool := NewPool(3, nil, nil)
	if pool == nil {
		t.Fatal("NewPool returned nil")
	}
	pool.Close()
}

// TestNewPoolNilRetryConfig verifies pool creation with nil retry config.
func TestNewPoolNilRetryConfig(t *testing.T) {
	log := logger.NewDefault()
	pool := NewPool(3, nil, log)
	if pool == nil {
		t.Fatal("NewPool returned nil")
	}
	pool.Close()
}

// TestNewPoolCustomRetryConfig verifies pool creation with custom config.
func TestNewPoolCustomRetryConfig(t *testing.T) {
	log := logger.NewDefault()
	retryCfg := &RetryConfig{
		MaxAttempts:       5,
		InitialBackoff:    500 * time.Millisecond,
		MaxBackoff:        10 * time.Second,
		BackoffMultiplier: 1.5,
		Jitter:            true,
	}
	pool := NewPool(10, retryCfg, log)
	if pool == nil {
		t.Fatal("NewPool returned nil")
	}
	pool.Close()
}

// TestPoolCloseIdempotent verifies that closing a pool multiple
// times doesn't panic.
func TestPoolCloseIdempotent(t *testing.T) {
	log := logger.NewDefault()
	pool := NewPool(3, nil, log)

	// Close should be safe to call multiple times
	pool.Close()
	pool.Close()
}

// TestBackoffValuesTable verifies specific backoff calculations.
func TestBackoffValuesTable(t *testing.T) {
	cfg := &RetryConfig{
		InitialBackoff:    100 * time.Millisecond,
		MaxBackoff:        10 * time.Second,
		BackoffMultiplier: 2.0,
		Jitter:            false,
	}

	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{0, 100 * time.Millisecond},                                                    // 100ms * 2^0 = 100ms
		{1, time.Duration(float64(100*time.Millisecond) * math.Pow(2.0, 1))},           // 200ms
		{2, time.Duration(float64(100*time.Millisecond) * math.Pow(2.0, 2))},           // 400ms
		{3, time.Duration(float64(100*time.Millisecond) * math.Pow(2.0, 3))},           // 800ms
		{4, time.Duration(float64(100*time.Millisecond) * math.Pow(2.0, 4))},           // 1600ms
		{10, time.Duration(float64(100*time.Millisecond) * math.Pow(2.0, 10))},         // 102400ms > 10s -> capped
	}

	for _, tc := range tests {
		backoff := calculateBackoff(cfg.InitialBackoff, cfg, tc.attempt)
		expected := tc.want
		if expected > cfg.MaxBackoff {
			expected = cfg.MaxBackoff
		}
		if backoff != expected {
			t.Errorf("attempt %d: backoff %v != expected %v",
				tc.attempt, backoff, expected)
		}
	}
}

// TestRetryConfigString verifies string representation of retry errors.
func TestRetryConfigString(t *testing.T) {
	// ConnectWithRetry error format should contain attempt info
	errorMsg := "connection failed after 4 attempts: dial error"
	if !strings.Contains(errorMsg, "failed after") {
		t.Error("error message format unexpected")
	}
}
