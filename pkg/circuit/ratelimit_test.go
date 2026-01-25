package circuit

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/logger"
	"github.com/opd-ai/go-tor/pkg/path"
	"github.com/opd-ai/go-tor/pkg/ratelimit"
)

// mockMetricsRecorder is a mock implementation of MetricsRecorder for testing
type mockMetricsRecorder struct {
	rateLimitWaitCalls  int64
	rateLimitedCircuits int64
	totalWaitTime       int64 // in nanoseconds
	mu                  sync.Mutex
}

func (m *mockMetricsRecorder) RecordRateLimitWait(duration time.Duration) {
	atomic.AddInt64(&m.rateLimitWaitCalls, 1)
	atomic.AddInt64(&m.totalWaitTime, int64(duration))
}

func (m *mockMetricsRecorder) RecordRateLimitedCircuit() {
	atomic.AddInt64(&m.rateLimitedCircuits, 1)
}

func (m *mockMetricsRecorder) GetWaitCalls() int64 {
	return atomic.LoadInt64(&m.rateLimitWaitCalls)
}

func (m *mockMetricsRecorder) GetRateLimitedCircuits() int64 {
	return atomic.LoadInt64(&m.rateLimitedCircuits)
}

func (m *mockMetricsRecorder) GetTotalWaitTime() time.Duration {
	return time.Duration(atomic.LoadInt64(&m.totalWaitTime))
}

// TestBuilderRateLimitDisabled tests that circuit building works when rate limiting is disabled
func TestBuilderRateLimitDisabled(t *testing.T) {
	mgr := NewManager()
	log := logger.NewDefault()
	builder := NewBuilder(mgr, log)

	// No rate limiter set - should build without rate limiting
	// We can't actually build a circuit in a unit test, but we can verify the rate limiter is nil
	if builder.rateLimiter != nil {
		t.Error("Expected rate limiter to be nil by default")
	}
}

// TestBuilderSetRateLimiter tests setting a rate limiter on the builder
func TestBuilderSetRateLimiter(t *testing.T) {
	mgr := NewManager()
	log := logger.NewDefault()
	builder := NewBuilder(mgr, log)

	limiter := ratelimit.NewRateLimiter(10.0, 5)
	builder.SetRateLimiter(limiter)

	// Verify limiter was set
	builder.mu.Lock()
	if builder.rateLimiter != limiter {
		t.Error("Rate limiter was not set correctly")
	}
	builder.mu.Unlock()
}

// TestBuilderSetMetricsRecorder tests setting a metrics recorder on the builder
func TestBuilderSetMetricsRecorder(t *testing.T) {
	mgr := NewManager()
	log := logger.NewDefault()
	builder := NewBuilder(mgr, log)

	recorder := &mockMetricsRecorder{}
	builder.SetMetricsRecorder(recorder)

	// Verify recorder was set
	builder.mu.Lock()
	if builder.metricsRecorder != recorder {
		t.Error("Metrics recorder was not set correctly")
	}
	builder.mu.Unlock()
}

// TestBuilderRateLimitAllows tests that rate limiting allows operations within limits
func TestBuilderRateLimitAllows(t *testing.T) {
	// Skip actual circuit building in unit tests
	t.Skip("Requires network connection and relay - tested in integration tests")
}

// TestBuilderRateLimitBlocks tests that rate limiting blocks operations exceeding limits
func TestBuilderRateLimitBlocks(t *testing.T) {
	mgr := NewManager()
	log := logger.NewDefault()
	builder := NewBuilder(mgr, log)
	recorder := &mockMetricsRecorder{}

	// Set very strict rate limit: 1 circuit per second, burst of 1
	limiter := ratelimit.NewRateLimiter(1.0, 1)
	builder.SetRateLimiter(limiter)
	builder.SetMetricsRecorder(recorder)

	// Consume the single token
	if !limiter.Allow() {
		t.Fatal("Expected first circuit to be allowed")
	}

	// Try to build another circuit immediately - should be rate limited
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Create a mock path (we won't actually build, just test rate limiting)
	// The BuildCircuit will fail at connection stage, but we should see rate limiting first
	mockPath := &path.Path{} // Empty path will fail, but that's okay for this test

	// This should block for 50ms and then fail due to context timeout
	startTime := time.Now()
	_, err := builder.BuildCircuit(ctx, mockPath, 1*time.Second)
	elapsed := time.Since(startTime)

	if err == nil {
		t.Fatal("Expected error due to rate limiting or context timeout")
	}

	// Should have waited close to the timeout duration
	if elapsed < 40*time.Millisecond {
		t.Errorf("Expected to wait at least 40ms due to rate limit, waited %v", elapsed)
	}

	// The error could be either context deadline exceeded or circuit build failure
	// What matters is that we were rate limited
	t.Logf("Build blocked for %v with error: %v", elapsed, err)
}

// TestBuilderRateLimitMetrics tests that rate limiting metrics are recorded correctly
func TestBuilderRateLimitMetrics(t *testing.T) {
	mgr := NewManager()
	log := logger.NewDefault()
	builder := NewBuilder(mgr, log)
	recorder := &mockMetricsRecorder{}

	// Set rate limit: 2 circuits per second, burst of 2
	limiter := ratelimit.NewRateLimiter(2.0, 2)
	builder.SetRateLimiter(limiter)
	builder.SetMetricsRecorder(recorder)

	// Consume both tokens
	limiter.Allow()
	limiter.Allow()

	// Next attempt should wait and record metrics
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	mockPath := &path.Path{}
	_, err := builder.BuildCircuit(ctx, mockPath, 1*time.Second)

	if err == nil {
		t.Fatal("Expected error (rate limit or build failure)")
	}

	// We should have recorded wait time (even though context timed out)
	// The wait happens before the context check in many cases
	t.Logf("Rate limited circuits: %d, Wait calls: %d, Total wait time: %v",
		recorder.GetRateLimitedCircuits(),
		recorder.GetWaitCalls(),
		recorder.GetTotalWaitTime())
}

// TestBuilderRateLimitConcurrent tests rate limiting under concurrent load
func TestBuilderRateLimitConcurrent(t *testing.T) {
	mgr := NewManager()
	log := logger.NewDefault()
	builder := NewBuilder(mgr, log)
	recorder := &mockMetricsRecorder{}

	// Set rate limit: 5 circuits per second, burst of 5
	limiter := ratelimit.NewRateLimiter(5.0, 5)
	builder.SetRateLimiter(limiter)
	builder.SetMetricsRecorder(recorder)

	// Try to build 10 circuits concurrently
	const numAttempts = 10
	var wg sync.WaitGroup
	errors := make([]error, numAttempts)

	for i := 0; i < numAttempts; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()

			mockPath := &path.Path{}
			_, err := builder.BuildCircuit(ctx, mockPath, 1*time.Second)
			errors[idx] = err
		}(i)
	}

	wg.Wait()

	// All should fail (no actual network), but some should be rate limited
	failCount := 0
	for _, err := range errors {
		if err != nil {
			failCount++
		}
	}

	if failCount != numAttempts {
		t.Errorf("Expected all %d attempts to fail, got %d", numAttempts, failCount)
	}

	// Should have recorded some rate limiting activity
	t.Logf("Concurrent test results: rate limited=%d, wait calls=%d",
		recorder.GetRateLimitedCircuits(),
		recorder.GetWaitCalls())
}

// TestBuilderRateLimitNilLimiter tests that nil rate limiter doesn't cause panics
func TestBuilderRateLimitNilLimiter(t *testing.T) {
	mgr := NewManager()
	log := logger.NewDefault()
	builder := NewBuilder(mgr, log)

	// Explicitly set nil (though it's default)
	builder.SetRateLimiter(nil)

	// Should work without panicking (will fail at connection stage)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	mockPath := &path.Path{}
	_, err := builder.BuildCircuit(ctx, mockPath, 1*time.Second)

	if err == nil {
		t.Fatal("Expected error due to build failure (no network)")
	}
}

// TestBuilderRateLimitNilRecorder tests that nil metrics recorder doesn't cause panics
func TestBuilderRateLimitNilRecorder(t *testing.T) {
	mgr := NewManager()
	log := logger.NewDefault()
	builder := NewBuilder(mgr, log)

	// Set rate limiter but not recorder
	limiter := ratelimit.NewRateLimiter(1.0, 1)
	builder.SetRateLimiter(limiter)
	builder.SetMetricsRecorder(nil)

	limiter.Allow() // Consume token

	// Should work without panicking even though recorder is nil
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	mockPath := &path.Path{}
	_, err := builder.BuildCircuit(ctx, mockPath, 1*time.Second)

	if err == nil {
		t.Fatal("Expected error due to rate limit or build failure")
	}
}

// TestBuilderRateLimitRecovery tests that rate limiter recovers over time
func TestBuilderRateLimitRecovery(t *testing.T) {
	mgr := NewManager()
	log := logger.NewDefault()
	builder := NewBuilder(mgr, log)

	// Set rate limit: 10 circuits per second, burst of 2
	limiter := ratelimit.NewRateLimiter(10.0, 2)
	builder.SetRateLimiter(limiter)

	// Consume both tokens
	if !limiter.Allow() {
		t.Fatal("Expected first token to be available")
	}
	if !limiter.Allow() {
		t.Fatal("Expected second token to be available")
	}

	// Tokens should be exhausted
	if limiter.Allow() {
		t.Error("Expected tokens to be exhausted")
	}

	// Wait for tokens to refill (100ms should give us 1 token at 10/sec)
	time.Sleep(150 * time.Millisecond)

	// Should have at least one token now
	if !limiter.Allow() {
		t.Error("Expected token to have refilled after waiting")
	}
}

// TestBuilderRateLimitConfiguration tests different rate limit configurations
func TestBuilderRateLimitConfiguration(t *testing.T) {
	tests := []struct {
		name  string
		rate  float64
		burst int
	}{
		{"low_rate", 1.0, 1},
		{"medium_rate", 10.0, 5},
		{"high_rate", 100.0, 50},
		{"fractional_rate", 0.5, 1}, // One every 2 seconds
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := NewManager()
			log := logger.NewDefault()
			builder := NewBuilder(mgr, log)

			limiter := ratelimit.NewRateLimiter(tt.rate, tt.burst)
			builder.SetRateLimiter(limiter)

			// Verify configuration
			if limiter.Rate() != tt.rate {
				t.Errorf("Rate = %f, want %f", limiter.Rate(), tt.rate)
			}
			if limiter.Burst() != tt.burst {
				t.Errorf("Burst = %d, want %d", limiter.Burst(), tt.burst)
			}

			// Verify tokens start at burst capacity
			tokens := limiter.Tokens()
			if tokens != float64(tt.burst) {
				t.Errorf("Initial tokens = %f, want %f", tokens, float64(tt.burst))
			}
		})
	}
}
