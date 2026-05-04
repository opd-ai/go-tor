package ratelimit

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// tolerance is the acceptable relative error for time-based measurements.
const tolerance = 0.20

func withinTolerance(got, want float64) bool {
	if want == 0 {
		return got == 0
	}
	diff := got - want
	if diff < 0 {
		diff = -diff
	}
	return diff/want <= tolerance
}

func TestTokenRefillAccuracy(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping time-sensitive test in short mode")
	}

	rl := NewRateLimiter(100.0, 20)
	// Consume all tokens.
	for rl.Allow() {
	}

	// Wait 100ms; expect ~10 tokens refilled (100/s * 0.1s).
	time.Sleep(100 * time.Millisecond)

	tokens := rl.Tokens()
	if !withinTolerance(tokens, 10.0) {
		t.Errorf("tokens refilled = %.2f, want ~10.0 (±20%%)", tokens)
	}
}

func TestBurstCapacityEnforcement(t *testing.T) {
	burst := 5
	rl := NewRateLimiter(1.0, burst) // Very slow refill.

	allowed := 0
	for i := 0; i < burst*3; i++ {
		if rl.Allow() {
			allowed++
		}
	}

	if allowed != burst {
		t.Errorf("allowed = %d, want exactly %d (burst)", allowed, burst)
	}
}

func TestRateEnforcementOverTime(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping time-sensitive test in short mode")
	}

	rate := 10.0
	burst := 10
	rl := NewRateLimiter(rate, burst)

	// Consume burst first.
	for rl.Allow() {
	}

	// Now measure operations allowed over 500ms.
	duration := 500 * time.Millisecond
	deadline := time.Now().Add(duration)
	allowed := 0

	for time.Now().Before(deadline) {
		if rl.Allow() {
			allowed++
		}
		time.Sleep(5 * time.Millisecond)
	}

	expected := rate * duration.Seconds() // 5
	if !withinTolerance(float64(allowed), expected) {
		t.Errorf("allowed = %d, want ~%.0f (±20%%)", allowed, expected)
	}
}

func TestMultiLimiterAtomicity(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping time-sensitive test in short mode")
	}

	fast := NewRateLimiter(1000.0, 100)
	slow := NewRateLimiter(10.0, 5)
	ml := NewMultiLimiter(fast, slow)

	allowed := 0
	for i := 0; i < 50; i++ {
		if ml.Allow() {
			allowed++
		}
	}

	// Slow limiter's burst (5) should be the bottleneck.
	if allowed != 5 {
		t.Errorf("allowed = %d, want 5 (slow limiter burst)", allowed)
	}
}

func TestMultiLimiterEmpty(t *testing.T) {
	ml := NewMultiLimiter()

	for i := 0; i < 100; i++ {
		if !ml.Allow() {
			t.Fatal("empty MultiLimiter should always allow")
		}
	}
}

func TestKeyedRateLimiterPerKeyIsolation(t *testing.T) {
	kl := NewKeyedRateLimiter(1.0, 3)

	// Exhaust key "A".
	for i := 0; i < 3; i++ {
		kl.Allow("A")
	}
	if kl.Allow("A") {
		t.Error("key A should be exhausted")
	}

	// Key "B" must still allow operations.
	for i := 0; i < 3; i++ {
		if !kl.Allow("B") {
			t.Errorf("key B operation %d should succeed", i)
		}
	}
}

func TestKeyedRateLimiterCleanupEffectiveness(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping time-sensitive test in short mode")
	}

	kl := NewKeyedRateLimiter(10.0, 5)
	kl.Allow("stale1")
	kl.Allow("stale2")
	kl.Allow("stale3")

	time.Sleep(60 * time.Millisecond)

	// Touch fresh key after sleep so it won't be stale.
	kl.Allow("fresh")

	kl.Cleanup(50 * time.Millisecond)

	if kl.Size() != 1 {
		t.Errorf("after cleanup Size() = %d, want 1 (only fresh)", kl.Size())
	}
}

func TestWaitWithContextCancellation(t *testing.T) {
	rl := NewRateLimiter(0.1, 1) // Very slow refill.
	rl.Allow()                   // Exhaust.

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err := rl.Wait(ctx)
	elapsed := time.Since(start)

	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	if elapsed > 200*time.Millisecond {
		t.Errorf("Wait returned too slowly: %v", elapsed)
	}
}

func TestWaitNLargeN(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping time-sensitive test in short mode")
	}

	// Rate 100/s, burst 10. Request 8 tokens from empty bucket.
	rl := NewRateLimiter(100.0, 10)
	rl.AllowN(10) // Exhaust burst.

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := rl.WaitN(ctx, 8)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("WaitN(8) should succeed within timeout, got %v", err)
	}
	// Expected wait: 8/100 = 80ms.
	expected := 80 * time.Millisecond
	if !withinTolerance(float64(elapsed), float64(expected)) {
		t.Errorf("wait elapsed = %v, want ~%v (±20%%)", elapsed, expected)
	}
}

func TestReserveDelayAccuracy(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping time-sensitive test in short mode")
	}

	rate := 10.0
	rl := NewRateLimiter(rate, 5)
	rl.AllowN(5) // Exhaust.

	res := rl.Reserve(3)
	if !res.OK() {
		t.Fatal("reservation should be OK")
	}

	// Expected delay: 3 tokens / 10 tokens/sec = 300ms.
	expected := 300 * time.Millisecond
	got := res.Delay()
	if !withinTolerance(float64(got), float64(expected)) {
		t.Errorf("delay = %v, want ~%v (±20%%)", got, expected)
	}
}

func TestConcurrentRateLimiting(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping time-sensitive test in short mode")
	}

	rate := 100.0
	burst := 10
	rl := NewRateLimiter(rate, burst)

	var total atomic.Int64
	var wg sync.WaitGroup
	duration := 200 * time.Millisecond

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			deadline := time.Now().Add(duration)
			for time.Now().Before(deadline) {
				if rl.Allow() {
					total.Add(1)
				}
				time.Sleep(time.Millisecond)
			}
		}()
	}

	wg.Wait()

	// Expected: burst + rate*duration = 10 + 100*0.2 = 30.
	expected := float64(burst) + rate*duration.Seconds()
	got := float64(total.Load())
	if !withinTolerance(got, expected) {
		t.Errorf("total allowed = %.0f, want ~%.0f (±20%%)", got, expected)
	}
}

func TestSetRateSetBurstDynamicUpdates(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping time-sensitive test in short mode")
	}

	rl := NewRateLimiter(10.0, 10)
	rl.AllowN(10) // Exhaust.

	// Change rate to 100/s.
	rl.SetRate(100.0)
	time.Sleep(50 * time.Millisecond)

	// Expect ~5 tokens refilled (100/s * 0.05s).
	tokens := rl.Tokens()
	if !withinTolerance(tokens, 5.0) {
		t.Errorf("after SetRate tokens = %.2f, want ~5.0", tokens)
	}

	// Consume and change burst.
	for rl.Allow() {
	}
	rl.SetBurst(3)
	rl.SetRate(1000.0)
	time.Sleep(50 * time.Millisecond)

	// Tokens should cap at new burst of 3.
	tokens = rl.Tokens()
	if tokens > 3.0 {
		t.Errorf("tokens = %.2f, should be capped at burst 3", tokens)
	}
}
