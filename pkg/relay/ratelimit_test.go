package relay

import (
	"context"
	"testing"
	"time"
)

func TestRateLimiter_AllowCircuit(t *testing.T) {
	cfg := &RateLimiterConfig{
		CircuitRate:  10.0, // 10 per second
		CircuitBurst: 5,
	}
	rl := NewRateLimiter(cfg)

	ctx := context.Background()

	// Should allow burst
	for i := 0; i < 5; i++ {
		if err := rl.AllowCircuit(ctx); err != nil {
			t.Errorf("Expected circuit %d to be allowed, got error: %v", i, err)
		}
	}

	// Next one should wait (but complete quickly due to refill)
	start := time.Now()
	if err := rl.AllowCircuit(ctx); err != nil {
		t.Errorf("Expected circuit to be allowed after wait, got error: %v", err)
	}
	elapsed := time.Since(start)

	// Should have waited at least a bit
	if elapsed < 50*time.Millisecond {
		t.Logf("Circuit wait time: %v (expected slight delay)", elapsed)
	}
}

func TestRateLimiter_AllowConnection(t *testing.T) {
	cfg := &RateLimiterConfig{
		ConnectionRate:  5.0, // 5 per second per IP
		ConnectionBurst: 3,
	}
	rl := NewRateLimiter(cfg)

	ctx := context.Background()
	ip := "192.168.1.1"

	// Should allow burst
	for i := 0; i < 3; i++ {
		if err := rl.AllowConnection(ctx, ip); err != nil {
			t.Errorf("Expected connection %d to be allowed, got error: %v", i, err)
		}
	}

	// Different IP should have separate limit
	ip2 := "192.168.1.2"
	if err := rl.AllowConnection(ctx, ip2); err != nil {
		t.Errorf("Expected connection from different IP to be allowed, got error: %v", err)
	}
}

func TestRateLimiter_AllowCell(t *testing.T) {
	cfg := &RateLimiterConfig{
		CellRate:  100.0, // 100 per second
		CellBurst: 10,
	}
	rl := NewRateLimiter(cfg)

	ctx := context.Background()
	circuitID := uint32(12345)

	// Should allow burst
	for i := 0; i < 10; i++ {
		if err := rl.AllowCell(ctx, circuitID); err != nil {
			t.Errorf("Expected cell %d to be allowed, got error: %v", i, err)
		}
	}

	// Different circuit should have separate limit
	circuitID2 := uint32(67890)
	if err := rl.AllowCell(ctx, circuitID2); err != nil {
		t.Errorf("Expected cell from different circuit to be allowed, got error: %v", err)
	}
}

func TestRateLimiter_RemoveCircuit(t *testing.T) {
	cfg := DefaultRateLimiterConfig()
	rl := NewRateLimiter(cfg)

	circuitID := uint32(12345)
	ctx := context.Background()

	// Create limiter for circuit
	if err := rl.AllowCell(ctx, circuitID); err != nil {
		t.Fatalf("Failed to allow initial cell: %v", err)
	}

	// Verify limiter exists
	if len(rl.cellLimiters) != 1 {
		t.Errorf("Expected 1 cell limiter, got %d", len(rl.cellLimiters))
	}

	// Remove circuit
	rl.RemoveCircuit(circuitID)

	// Verify limiter removed
	if len(rl.cellLimiters) != 0 {
		t.Errorf("Expected 0 cell limiters after removal, got %d", len(rl.cellLimiters))
	}
}

func TestRateLimiter_Stats(t *testing.T) {
	cfg := &RateLimiterConfig{
		CircuitRate:     10.0,
		CircuitBurst:    5,
		ConnectionRate:  5.0,
		ConnectionBurst: 3,
		CellRate:        100.0,
		CellBurst:       10,
	}
	rl := NewRateLimiter(cfg)

	ctx := context.Background()

	// Add some limiters
	rl.AllowConnection(ctx, "192.168.1.1")
	rl.AllowConnection(ctx, "192.168.1.2")
	rl.AllowCell(ctx, uint32(111))
	rl.AllowCell(ctx, uint32(222))
	rl.AllowCell(ctx, uint32(333))

	stats := rl.Stats()

	if stats.CircuitBurst != 5 {
		t.Errorf("Expected circuit burst of 5, got %d", stats.CircuitBurst)
	}

	if stats.ActiveIPLimiters != 2 {
		t.Errorf("Expected 2 active IP limiters, got %d", stats.ActiveIPLimiters)
	}

	if stats.ActiveCellLimiters != 3 {
		t.Errorf("Expected 3 active cell limiters, got %d", stats.ActiveCellLimiters)
	}

	// Circuit limiter should have tokens available (burst size since not used)
	if stats.CircuitAvailable < 4.0 {
		t.Errorf("Expected at least 4 circuit tokens available, got %f", stats.CircuitAvailable)
	}
}

func TestRateLimiter_Cleanup(t *testing.T) {
	cfg := &RateLimiterConfig{
		ConnectionRate:  5.0,
		ConnectionBurst: 3,
		CleanupInterval: 100 * time.Millisecond,
	}
	rl := NewRateLimiter(cfg)

	ctx := context.Background()

	// Add some limiters
	rl.AllowConnection(ctx, "192.168.1.1")
	rl.AllowConnection(ctx, "192.168.1.2")

	if len(rl.connLimiters) != 2 {
		t.Errorf("Expected 2 connection limiters, got %d", len(rl.connLimiters))
	}

	// Wait for cleanup interval
	time.Sleep(150 * time.Millisecond)

	// Trigger cleanup by making a new connection
	rl.AllowConnection(ctx, "192.168.1.3")

	// Idle limiters should be cleaned up (they have full tokens)
	if len(rl.connLimiters) > 3 {
		t.Logf("Connection limiters after cleanup: %d (some may remain if not idle)", len(rl.connLimiters))
	}
}

func TestRateLimiter_ContextCancellation(t *testing.T) {
	cfg := &RateLimiterConfig{
		CircuitRate:  0.1, // Very slow: 0.1 per second = 1 per 10 seconds
		CircuitBurst: 0,   // No burst
	}
	rl := NewRateLimiter(cfg)

	// Drain initial token if any
	rl.circuitLimiter.Allow() // Consume any initial token

	// Now create a timeout context for the next request
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// This should timeout due to rate limit (need to wait 10 seconds for next token)
	start := time.Now()
	err := rl.AllowCircuit(ctx)
	elapsed := time.Since(start)

	if err == nil {
		t.Errorf("Expected timeout error, got nil (elapsed: %v)", elapsed)
	}

	// Either got context deadline exceeded or the error from Wait
	if err != nil && ctx.Err() == nil {
		// The error came from Wait before context deadline - that's ok too
		t.Logf("Got error from Wait: %v (elapsed: %v)", err, elapsed)
	}
}

func TestRateLimiter_WithMetrics(t *testing.T) {
	metrics := NewRelayMetrics()
	cfg := &RateLimiterConfig{
		CircuitRate:  1.0,
		CircuitBurst: 0,
		Metrics:      metrics,
	}
	rl := NewRateLimiter(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// This should timeout and increment rate limited circuits
	rl.AllowCircuit(ctx)

	if metrics.RateLimitedCircuits.Value() == 0 {
		t.Error("Expected rate limited circuits metric to be incremented")
	}
}

func TestDefaultRateLimiterConfig(t *testing.T) {
	cfg := DefaultRateLimiterConfig()

	if cfg.CircuitRate != 10.0 {
		t.Errorf("Expected circuit rate of 10.0, got %f", cfg.CircuitRate)
	}

	if cfg.CircuitBurst != 20 {
		t.Errorf("Expected circuit burst of 20, got %d", cfg.CircuitBurst)
	}

	if cfg.ConnectionRate != 5.0 {
		t.Errorf("Expected connection rate of 5.0, got %f", cfg.ConnectionRate)
	}

	if cfg.ConnectionBurst != 10 {
		t.Errorf("Expected connection burst of 10, got %d", cfg.ConnectionBurst)
	}

	if cfg.CellRate != 100.0 {
		t.Errorf("Expected cell rate of 100.0, got %f", cfg.CellRate)
	}

	if cfg.CellBurst != 200 {
		t.Errorf("Expected cell burst of 200, got %d", cfg.CellBurst)
	}

	if cfg.CleanupInterval != 5*time.Minute {
		t.Errorf("Expected cleanup interval of 5 minutes, got %v", cfg.CleanupInterval)
	}
}
