package security

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestRateLimiterConcurrent tests that RateLimiter is thread-safe
// This addresses the race condition identified in AUDIT.md line 185-203
func TestRateLimiterConcurrent(t *testing.T) {
	limiter := newRateLimiter(100, time.Second)
	
	// Run concurrent Allow() calls
	const goroutines = 10
	const iterations = 100
	
	var wg sync.WaitGroup
	allowed := atomic.Int32{}
	
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				if limiter.Allow() {
					allowed.Add(1)
				}
			}
		}()
	}
	
	wg.Wait()
	
	// Should have allowed exactly 100 operations (maxTokens)
	// since we haven't waited for refill
	if got := allowed.Load(); got != 100 {
		t.Errorf("Expected 100 allowed operations, got %d", got)
	}
}

// TestRateLimiterRaceDetector explicitly tests for race conditions
func TestRateLimiterRaceDetector(t *testing.T) {
	limiter := newRateLimiter(50, 100*time.Millisecond)
	
	var wg sync.WaitGroup
	const workers = 5
	
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				_ = limiter.Allow()
				time.Sleep(time.Millisecond)
			}
		}()
	}
	
	wg.Wait()
	// Test passes if no race is detected by go test -race
}

// TestRateLimiterRefill tests that rate limiter refills tokens correctly
func TestRateLimiterRefill(t *testing.T) {
	limiter := newRateLimiter(5, 50*time.Millisecond)
	
	// Exhaust all tokens
	for i := 0; i < 5; i++ {
		if !limiter.Allow() {
			t.Errorf("Expected token %d to be available", i)
		}
	}
	
	// Should be exhausted
	if limiter.Allow() {
		t.Error("Expected tokens to be exhausted")
	}
	
	// Wait for refill
	time.Sleep(60 * time.Millisecond)
	
	// Tokens should be refilled
	if !limiter.Allow() {
		t.Error("Expected tokens to be refilled after interval")
	}
}

// TestRateLimiterZeroTokens tests rate limiter with zero initial tokens
func TestRateLimiterZeroTokens(t *testing.T) {
	limiter := newRateLimiter(0, time.Second)
	
	if limiter.Allow() {
		t.Error("Expected zero tokens to deny operation")
	}
}

// TestRateLimiterSequential tests basic sequential behavior
func TestRateLimiterSequential(t *testing.T) {
	limiter := newRateLimiter(3, time.Hour) // Long interval to avoid refill
	
	allowed := 0
	for i := 0; i < 10; i++ {
		if limiter.Allow() {
			allowed++
		}
	}
	
	if allowed != 3 {
		t.Errorf("Expected 3 allowed operations, got %d", allowed)
	}
}
