package errors

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestDefaultRetryPolicy(t *testing.T) {
	policy := DefaultRetryPolicy()

	if policy.MaxAttempts != 3 {
		t.Errorf("Expected MaxAttempts=3, got %d", policy.MaxAttempts)
	}
	if policy.InitialDelay != 1*time.Second {
		t.Errorf("Expected InitialDelay=1s, got %v", policy.InitialDelay)
	}
	if policy.MaxDelay != 30*time.Second {
		t.Errorf("Expected MaxDelay=30s, got %v", policy.MaxDelay)
	}
	if policy.Multiplier != 2.0 {
		t.Errorf("Expected Multiplier=2.0, got %f", policy.Multiplier)
	}
}

func TestRetry_Success(t *testing.T) {
	ctx := context.Background()
	attempts := 0

	err := Retry(ctx, func() error {
		attempts++
		return nil // Success on first attempt
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if attempts != 1 {
		t.Errorf("Expected 1 attempt, got %d", attempts)
	}
}

func TestRetry_SuccessAfterRetries(t *testing.T) {
	ctx := context.Background()
	attempts := 0

	err := Retry(ctx, func() error {
		attempts++
		if attempts < 3 {
			return NetworkError("temporary failure", fmt.Errorf("network error"))
		}
		return nil // Success on third attempt
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestRetry_MaxAttemptsExceeded(t *testing.T) {
	ctx := context.Background()
	policy := &RetryPolicy{
		MaxAttempts:  2,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		Jitter:       0.0,
		RetryableErrors: map[ErrorCategory]bool{
			CategoryNetwork: true,
		},
	}

	attempts := 0
	err := RetryWithPolicy(ctx, policy, func() error {
		attempts++
		return NetworkError("persistent failure", fmt.Errorf("network error"))
	})

	if err == nil {
		t.Fatal("Expected error after max attempts")
	}
	// Should attempt initial + 2 retries = 3 total
	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestRetry_NonRetryableError(t *testing.T) {
	ctx := context.Background()
	attempts := 0

	err := Retry(ctx, func() error {
		attempts++
		return ProtocolError("protocol error", fmt.Errorf("bad protocol"))
	})

	if err == nil {
		t.Fatal("Expected error")
	}
	// Should not retry protocol errors (not retryable)
	if attempts != 1 {
		t.Errorf("Expected 1 attempt (no retries for non-retryable), got %d", attempts)
	}
}

func TestRetry_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	attempts := 0

	// Cancel context after first attempt
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	policy := &RetryPolicy{
		MaxAttempts:  10,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
		RetryableErrors: map[ErrorCategory]bool{
			CategoryNetwork: true,
		},
	}

	err := RetryWithPolicy(ctx, policy, func() error {
		attempts++
		return NetworkError("failure", fmt.Errorf("network error"))
	})

	if err == nil {
		t.Fatal("Expected error from context cancellation")
	}
	if attempts > 2 {
		t.Errorf("Expected few attempts before cancellation, got %d", attempts)
	}
}

func TestRetryWithStats(t *testing.T) {
	ctx := context.Background()
	attempts := 0

	policy := &RetryPolicy{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		Jitter:       0.0,
		RetryableErrors: map[ErrorCategory]bool{
			CategoryNetwork: true,
		},
	}

	stats, err := RetryWithStats(ctx, policy, func() error {
		attempts++
		if attempts < 3 {
			return NetworkError("temporary failure", fmt.Errorf("network error"))
		}
		return nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if stats.TotalAttempts != 3 {
		t.Errorf("Expected 3 total attempts, got %d", stats.TotalAttempts)
	}
	if !stats.SuccessfulRetry {
		t.Error("Expected SuccessfulRetry to be true")
	}
	if stats.TotalDuration == 0 {
		t.Error("Expected non-zero TotalDuration")
	}
}

func TestRetryWithCallback(t *testing.T) {
	ctx := context.Background()
	callbackCount := 0
	attempts := 0

	policy := &RetryPolicy{
		MaxAttempts:  2,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		RetryableErrors: map[ErrorCategory]bool{
			CategoryNetwork: true,
		},
	}

	callback := func(attempt int, err error, willRetry bool) {
		callbackCount++
	}

	err := RetryWithCallbackFunc(ctx, policy, func() error {
		attempts++
		if attempts < 2 {
			return NetworkError("temporary failure", fmt.Errorf("network error"))
		}
		return nil
	}, callback)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if callbackCount != 2 {
		t.Errorf("Expected 2 callback calls, got %d", callbackCount)
	}
}

func TestRetryPolicy_CalculateDelay(t *testing.T) {
	policy := &RetryPolicy{
		InitialDelay: 1 * time.Second,
		MaxDelay:     10 * time.Second,
		Multiplier:   2.0,
		Jitter:       0.0, // No jitter for predictable testing
	}

	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{0, 1 * time.Second},
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},
		{4, 10 * time.Second}, // Capped at MaxDelay
		{5, 10 * time.Second}, // Still capped
	}

	for _, tt := range tests {
		got := policy.calculateDelay(tt.attempt)
		if got != tt.want {
			t.Errorf("calculateDelay(%d) = %v, want %v", tt.attempt, got, tt.want)
		}
	}
}

func TestRetryPolicy_WithJitter(t *testing.T) {
	policy := &RetryPolicy{
		InitialDelay: 1 * time.Second,
		MaxDelay:     10 * time.Second,
		Multiplier:   2.0,
		Jitter:       0.5, // 50% jitter
	}

	// Calculate delay multiple times and verify jitter adds randomness
	delays := make(map[time.Duration]bool)
	for i := 0; i < 10; i++ {
		delay := policy.calculateDelay(1)
		delays[delay] = true

		// Verify delay is in reasonable range
		expectedBase := 2 * time.Second
		minDelay := time.Duration(float64(expectedBase) * 0.5)
		maxDelay := time.Duration(float64(expectedBase) * 1.5)

		if delay < minDelay || delay > maxDelay {
			t.Errorf("Delay %v out of expected range [%v, %v]", delay, minDelay, maxDelay)
		}
	}

	// With jitter, we should see different delays (probabilistic test)
	if len(delays) < 2 {
		t.Error("Expected jitter to produce varying delays")
	}
}

func TestAggressiveRetryPolicy(t *testing.T) {
	policy := AggressiveRetryPolicy()

	if policy.MaxAttempts != 5 {
		t.Errorf("Expected MaxAttempts=5, got %d", policy.MaxAttempts)
	}
	if policy.InitialDelay != 500*time.Millisecond {
		t.Errorf("Expected InitialDelay=500ms, got %v", policy.InitialDelay)
	}
}

func TestConservativeRetryPolicy(t *testing.T) {
	policy := ConservativeRetryPolicy()

	if policy.MaxAttempts != 2 {
		t.Errorf("Expected MaxAttempts=2, got %d", policy.MaxAttempts)
	}
	if policy.InitialDelay != 2*time.Second {
		t.Errorf("Expected InitialDelay=2s, got %v", policy.InitialDelay)
	}
}

func TestRetry_RetryableErrorMarking(t *testing.T) {
	ctx := context.Background()
	attempts := 0

	policy := &RetryPolicy{
		MaxAttempts:     2,
		InitialDelay:    10 * time.Millisecond,
		MaxDelay:        100 * time.Millisecond,
		Multiplier:      2.0,
		RetryableErrors: nil, // Only respect Retryable flag
	}

	err := RetryWithPolicy(ctx, policy, func() error {
		attempts++
		if attempts < 2 {
			// Create a retryable error explicitly
			return NewRetryable(CategoryNetwork, SeverityMedium, "retryable error")
		}
		return nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}
}
