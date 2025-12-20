package ratelimit

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestNewRateLimiter(t *testing.T) {
	tests := []struct {
		name      string
		rate      float64
		burst     int
		wantRate  float64
		wantBurst int
	}{
		{
			name:      "valid rate and burst",
			rate:      10.0,
			burst:     5,
			wantRate:  10.0,
			wantBurst: 5,
		},
		{
			name:      "zero rate defaults to 1",
			rate:      0,
			burst:     5,
			wantRate:  1.0,
			wantBurst: 5,
		},
		{
			name:      "negative rate defaults to 1",
			rate:      -5.0,
			burst:     5,
			wantRate:  1.0,
			wantBurst: 5,
		},
		{
			name:      "zero burst defaults to 1",
			rate:      10.0,
			burst:     0,
			wantRate:  10.0,
			wantBurst: 1,
		},
		{
			name:      "negative burst defaults to 1",
			rate:      10.0,
			burst:     -5,
			wantRate:  10.0,
			wantBurst: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := NewRateLimiter(tt.rate, tt.burst)
			if l.Rate() != tt.wantRate {
				t.Errorf("Rate() = %v, want %v", l.Rate(), tt.wantRate)
			}
			if l.Burst() != tt.wantBurst {
				t.Errorf("Burst() = %v, want %v", l.Burst(), tt.wantBurst)
			}
		})
	}
}

func TestRateLimiterAllow(t *testing.T) {
	// Create a limiter with burst of 3
	l := NewRateLimiter(1.0, 3)

	// Should allow 3 operations immediately (burst)
	for i := 0; i < 3; i++ {
		if !l.Allow() {
			t.Errorf("Allow() should return true for operation %d", i)
		}
	}

	// Fourth operation should fail
	if l.Allow() {
		t.Error("Allow() should return false when tokens exhausted")
	}
}

func TestRateLimiterAllowN(t *testing.T) {
	l := NewRateLimiter(10.0, 5)

	// Request 3 tokens
	if !l.AllowN(3) {
		t.Error("AllowN(3) should succeed with 5 tokens available")
	}

	// Request 3 more (only 2 available)
	if l.AllowN(3) {
		t.Error("AllowN(3) should fail with only 2 tokens available")
	}

	// Request 2 (exactly available)
	if !l.AllowN(2) {
		t.Error("AllowN(2) should succeed with 2 tokens available")
	}
}

func TestRateLimiterRefill(t *testing.T) {
	// 10 tokens per second, burst of 5
	l := NewRateLimiter(10.0, 5)

	// Consume all tokens
	for i := 0; i < 5; i++ {
		l.Allow()
	}

	// Should have no tokens
	if l.Allow() {
		t.Error("Should have no tokens immediately after consuming all")
	}

	// Wait for tokens to refill (100ms = 1 token at 10/s)
	time.Sleep(120 * time.Millisecond)

	// Should have at least 1 token now
	if !l.Allow() {
		t.Error("Should have at least 1 token after 120ms at 10/s rate")
	}
}

func TestRateLimiterWait(t *testing.T) {
	// High rate for fast test
	l := NewRateLimiter(100.0, 2)

	// Consume burst
	l.Allow()
	l.Allow()

	// Wait should block briefly then succeed
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := l.Wait(ctx)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("Wait() returned error: %v", err)
	}

	// Should have waited about 10ms (1/100 second for 1 token)
	if elapsed < 5*time.Millisecond || elapsed > 50*time.Millisecond {
		t.Logf("Wait duration: %v (expected ~10ms)", elapsed)
	}
}

func TestRateLimiterWaitCancellation(t *testing.T) {
	// Very slow rate
	l := NewRateLimiter(0.1, 1)

	// Consume the burst token
	l.Allow()

	// Context that cancels quickly
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := l.Wait(ctx)
	if err != context.DeadlineExceeded {
		t.Errorf("Wait() should return DeadlineExceeded, got: %v", err)
	}
}

func TestRateLimiterWaitN(t *testing.T) {
	l := NewRateLimiter(100.0, 3)

	// Consume burst
	l.AllowN(3)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Wait for 2 tokens
	err := l.WaitN(ctx, 2)
	if err != nil {
		t.Errorf("WaitN(2) returned error: %v", err)
	}
}

func TestRateLimiterReserve(t *testing.T) {
	l := NewRateLimiter(10.0, 3)

	// Reserve when tokens available
	r := l.Reserve(2)
	if !r.OK() {
		t.Error("Reserve should succeed")
	}
	if r.Delay() != 0 {
		t.Errorf("Delay should be 0 when tokens available, got %v", r.Delay())
	}

	// Reserve more than available
	r = l.Reserve(2)
	if !r.OK() {
		t.Error("Reserve should succeed even without immediate tokens")
	}
	if r.Delay() <= 0 {
		t.Error("Delay should be positive when tokens need to refill")
	}
}

func TestRateLimiterTokens(t *testing.T) {
	l := NewRateLimiter(10.0, 5)

	// Full bucket
	tokens := l.Tokens()
	if tokens != 5.0 {
		t.Errorf("Initial tokens = %v, want 5.0", tokens)
	}

	// After consuming 2 (allow small tolerance for time-based refill)
	l.AllowN(2)
	tokens = l.Tokens()
	if tokens < 3.0 || tokens > 3.1 {
		t.Errorf("After consuming 2, tokens = %v, want ~3.0", tokens)
	}
}

func TestRateLimiterSetRate(t *testing.T) {
	l := NewRateLimiter(10.0, 5)

	l.SetRate(20.0)
	if l.Rate() != 20.0 {
		t.Errorf("Rate after SetRate(20) = %v, want 20.0", l.Rate())
	}

	// Invalid rate ignored
	l.SetRate(0)
	if l.Rate() != 20.0 {
		t.Errorf("Rate after SetRate(0) = %v, want 20.0", l.Rate())
	}
}

func TestRateLimiterSetBurst(t *testing.T) {
	l := NewRateLimiter(10.0, 5)

	l.SetBurst(10)
	if l.Burst() != 10 {
		t.Errorf("Burst after SetBurst(10) = %v, want 10", l.Burst())
	}

	// Setting lower burst caps tokens
	l.SetBurst(3)
	if l.Tokens() > 3.0 {
		t.Errorf("Tokens should be capped at burst, got %v", l.Tokens())
	}

	// Invalid burst ignored
	l.SetBurst(0)
	if l.Burst() != 3 {
		t.Errorf("Burst after SetBurst(0) = %v, want 3", l.Burst())
	}
}

func TestMultiLimiter(t *testing.T) {
	l1 := NewRateLimiter(100.0, 3)
	l2 := NewRateLimiter(100.0, 2)

	ml := NewMultiLimiter(l1, l2)

	// Should allow 2 (limited by l2's burst)
	if !ml.Allow() {
		t.Error("First Allow() should succeed")
	}
	if !ml.Allow() {
		t.Error("Second Allow() should succeed")
	}

	// Third should fail (l2 exhausted)
	if ml.Allow() {
		t.Error("Third Allow() should fail")
	}
}

func TestMultiLimiterWait(t *testing.T) {
	l1 := NewRateLimiter(100.0, 1)
	l2 := NewRateLimiter(100.0, 1)

	ml := NewMultiLimiter(l1, l2)

	// Consume tokens
	ml.Allow()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := ml.Wait(ctx)
	if err != nil {
		t.Errorf("Wait() returned error: %v", err)
	}
}

func TestKeyedRateLimiter(t *testing.T) {
	k := NewKeyedRateLimiter(10.0, 2)

	// Different keys have separate limits
	if !k.Allow("client1") {
		t.Error("First Allow for client1 should succeed")
	}
	if !k.Allow("client1") {
		t.Error("Second Allow for client1 should succeed")
	}
	if k.Allow("client1") {
		t.Error("Third Allow for client1 should fail")
	}

	// client2 has its own limit
	if !k.Allow("client2") {
		t.Error("First Allow for client2 should succeed")
	}
	if !k.Allow("client2") {
		t.Error("Second Allow for client2 should succeed")
	}
}

func TestKeyedRateLimiterWait(t *testing.T) {
	k := NewKeyedRateLimiter(100.0, 1)

	k.Allow("key1")

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := k.Wait(ctx, "key1")
	if err != nil {
		t.Errorf("Wait() returned error: %v", err)
	}
}

func TestKeyedRateLimiterCleanup(t *testing.T) {
	k := NewKeyedRateLimiter(10.0, 2)

	// Create some keys
	k.Allow("key1")
	k.Allow("key2")
	k.Allow("key3")

	if k.Size() != 3 {
		t.Errorf("Size() = %d, want 3", k.Size())
	}

	// Wait and cleanup
	time.Sleep(50 * time.Millisecond)
	k.Cleanup(10 * time.Millisecond)

	// All should be cleaned up
	if k.Size() != 0 {
		t.Errorf("After cleanup, Size() = %d, want 0", k.Size())
	}
}

func TestKeyedRateLimiterSize(t *testing.T) {
	k := NewKeyedRateLimiter(10.0, 2)

	if k.Size() != 0 {
		t.Errorf("Initial Size() = %d, want 0", k.Size())
	}

	k.Allow("a")
	k.Allow("b")
	k.Allow("c")

	if k.Size() != 3 {
		t.Errorf("After 3 keys, Size() = %d, want 3", k.Size())
	}
}

func TestRateLimiterConcurrency(t *testing.T) {
	l := NewRateLimiter(1000.0, 100)

	var wg sync.WaitGroup
	var allowed int64
	var mu sync.Mutex

	// 10 goroutines each trying 20 operations
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				if l.Allow() {
					mu.Lock()
					allowed++
					mu.Unlock()
				}
				time.Sleep(time.Millisecond)
			}
		}()
	}

	wg.Wait()

	// Should have allowed roughly 100 (burst) + some refilled
	if allowed < 100 {
		t.Errorf("Expected at least 100 allowed, got %d", allowed)
	}
}

func TestReservationMethods(t *testing.T) {
	r := &Reservation{ok: true, delay: 100 * time.Millisecond}

	if !r.OK() {
		t.Error("OK() should return true")
	}
	if r.Delay() != 100*time.Millisecond {
		t.Errorf("Delay() = %v, want 100ms", r.Delay())
	}

	r2 := &Reservation{ok: false, delay: 0}
	if r2.OK() {
		t.Error("OK() should return false for failed reservation")
	}
}
