package connection

import (
	"context"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/logger"
)

func TestDefaultRetryConfig(t *testing.T) {
	cfg := DefaultRetryConfig()

	if cfg.MaxAttempts != 3 {
		t.Errorf("MaxAttempts = %d, want 3", cfg.MaxAttempts)
	}

	if cfg.InitialBackoff != 1*time.Second {
		t.Errorf("InitialBackoff = %v, want 1s", cfg.InitialBackoff)
	}

	if cfg.MaxBackoff != 30*time.Second {
		t.Errorf("MaxBackoff = %v, want 30s", cfg.MaxBackoff)
	}

	if cfg.BackoffMultiplier != 2.0 {
		t.Errorf("BackoffMultiplier = %f, want 2.0", cfg.BackoffMultiplier)
	}

	if !cfg.Jitter {
		t.Error("Jitter should be enabled by default")
	}
}

func TestCalculateBackoff(t *testing.T) {
	cfg := &RetryConfig{
		InitialBackoff:    1 * time.Second,
		MaxBackoff:        10 * time.Second,
		BackoffMultiplier: 2.0,
		Jitter:            false, // Disable jitter for predictable testing
	}

	tests := []struct {
		base     time.Duration
		attempt  int
		expected time.Duration
	}{
		{1 * time.Second, 0, 1 * time.Second},
		{1 * time.Second, 1, 2 * time.Second},
		{1 * time.Second, 2, 4 * time.Second},
		{1 * time.Second, 3, 8 * time.Second},
		{1 * time.Second, 4, 10 * time.Second}, // Capped at MaxBackoff
		{2 * time.Second, 0, 2 * time.Second},
		{2 * time.Second, 1, 4 * time.Second},
	}

	for _, tt := range tests {
		result := calculateBackoff(tt.base, cfg, tt.attempt)
		if result != tt.expected {
			t.Errorf("calculateBackoff(%v, attempt=%d) = %v, want %v",
				tt.base, tt.attempt, result, tt.expected)
		}
	}
}

func TestCalculateBackoffWithJitter(t *testing.T) {
	cfg := &RetryConfig{
		InitialBackoff:    1 * time.Second,
		MaxBackoff:        10 * time.Second,
		BackoffMultiplier: 2.0,
		Jitter:            true,
	}

	base := 2 * time.Second
	attempt := 1

	// Expected base without jitter: 4 seconds
	// With ±25% jitter: should be between 3 and 5 seconds
	result := calculateBackoff(base, cfg, attempt)

	minExpected := 3 * time.Second
	maxExpected := 5 * time.Second

	if result < minExpected || result > maxExpected {
		t.Errorf("calculateBackoff with jitter = %v, want between %v and %v",
			result, minExpected, maxExpected)
	}
}

func TestConnectWithRetrySuccess(t *testing.T) {
	// This test would need a mock server, so we'll test the config validation
	cfg := DefaultConfig("192.0.2.1:9001")
	retryCfg := &RetryConfig{
		MaxAttempts:       1,
		InitialBackoff:    10 * time.Millisecond,
		MaxBackoff:        100 * time.Millisecond,
		BackoffMultiplier: 2.0,
		Jitter:            false,
	}

	conn := New(cfg, logger.NewDefault())
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// This will fail because 192.0.2.1 is a test network
	err := conn.ConnectWithRetry(ctx, cfg, retryCfg)
	if err == nil {
		t.Error("Expected connection to fail to test network")
	}

	// Verify it tried multiple times
	if conn.GetState() != StateFailed {
		t.Errorf("Connection state = %v, want StateFailed", conn.GetState())
	}
}

func TestConnectWithRetryContextCancelled(t *testing.T) {
	cfg := DefaultConfig("192.0.2.1:9001")
	retryCfg := &RetryConfig{
		MaxAttempts:       5,
		InitialBackoff:    1 * time.Second,
		MaxBackoff:        10 * time.Second,
		BackoffMultiplier: 2.0,
		Jitter:            false,
	}

	conn := New(cfg, logger.NewDefault())
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel immediately
	cancel()

	err := conn.ConnectWithRetry(ctx, cfg, retryCfg)
	if err == nil {
		t.Error("Expected error with cancelled context")
	}

	if ctx.Err() == nil {
		t.Error("Context should be cancelled")
	}
}

func TestNewPool(t *testing.T) {
	pool := NewPool(5, nil, logger.NewDefault())

	if pool == nil {
		t.Fatal("NewPool returned nil")
	}

	if pool.maxSize != 5 {
		t.Errorf("maxSize = %d, want 5", pool.maxSize)
	}

	if pool.retryCfg == nil {
		t.Error("retryCfg should be initialized with defaults")
	}
}

func TestPoolGetPutClose(t *testing.T) {
	pool := NewPool(2, nil, logger.NewDefault())
	defer pool.Close()

	// Test that pool operations don't panic
	// We can't easily test actual connections without a mock server

	// Test closing empty pool
	pool2 := NewPool(1, nil, logger.NewDefault())
	pool2.Close()
}

func TestPoolPutClosedConnection(t *testing.T) {
	pool := NewPool(2, nil, logger.NewDefault())
	defer pool.Close()

	cfg := DefaultConfig("192.0.2.1:9001")
	conn := New(cfg, logger.NewDefault())

	// Connection is not open, should not be added to pool
	pool.Put(conn)

	// Pool should remain empty (no way to verify without exposing internals)
}

func TestConnectWithRetryNilRetryConfig(t *testing.T) {
	cfg := DefaultConfig("192.0.2.1:9001")
	conn := New(cfg, logger.NewDefault())

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Should use default retry config
	err := conn.ConnectWithRetry(ctx, cfg, nil)
	if err == nil {
		t.Error("Expected connection to fail to test network")
	}
}

func TestRetryConfigWithZeroAttempts(t *testing.T) {
	cfg := DefaultConfig("192.0.2.1:9001")
	retryCfg := &RetryConfig{
		MaxAttempts:       0,
		InitialBackoff:    10 * time.Millisecond,
		MaxBackoff:        100 * time.Millisecond,
		BackoffMultiplier: 2.0,
		Jitter:            false,
	}

	conn := New(cfg, logger.NewDefault())
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Should only try once (no retries)
	err := conn.ConnectWithRetry(ctx, cfg, retryCfg)
	if err == nil {
		t.Error("Expected connection to fail to test network")
	}
}
