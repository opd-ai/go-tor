package errors

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestNewCircuitBreaker(t *testing.T) {
	cb := NewCircuitBreaker(nil)

	if cb == nil {
		t.Fatal("NewCircuitBreaker returned nil")
	}
	if cb.State() != StateClosed {
		t.Errorf("Expected initial state Closed, got %v", cb.State())
	}
}

func TestCircuitBreaker_ClosedState(t *testing.T) {
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig())
	ctx := context.Background()

	// Successful requests should keep circuit closed
	for i := 0; i < 5; i++ {
		err := cb.Execute(ctx, func() error {
			return nil
		})
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	}

	if cb.State() != StateClosed {
		t.Errorf("Expected circuit to remain closed, got %v", cb.State())
	}
}

func TestCircuitBreaker_OpenOnMaxFailures(t *testing.T) {
	config := DefaultCircuitBreakerConfig()
	config.MaxFailures = 3
	cb := NewCircuitBreaker(config)
	ctx := context.Background()

	// Generate failures to open circuit
	for i := 0; i < 3; i++ {
		err := cb.Execute(ctx, func() error {
			return fmt.Errorf("failure")
		})
		if err == nil {
			t.Error("Expected error from failing function")
		}
	}

	// Circuit should now be open
	if cb.State() != StateOpen {
		t.Errorf("Expected circuit to be open, got %v", cb.State())
	}

	// Next request should fail fast
	err := cb.Execute(ctx, func() error {
		t.Error("Function should not be called when circuit is open")
		return nil
	})

	if err == nil {
		t.Error("Expected error when circuit is open")
	}
}

func TestCircuitBreaker_OpenOnErrorRate(t *testing.T) {
	config := DefaultCircuitBreakerConfig()
	config.MaxFailures = 100      // High enough to not trigger
	config.FailureThreshold = 0.5 // 50% error rate
	config.MinRequests = 10
	cb := NewCircuitBreaker(config)
	ctx := context.Background()

	// Generate requests with 50% failure rate
	for i := 0; i < 20; i++ {
		cb.Execute(ctx, func() error {
			if i%2 == 0 {
				return fmt.Errorf("failure")
			}
			return nil
		})
	}

	// Circuit should be open due to error rate
	if cb.State() != StateOpen {
		t.Errorf("Expected circuit to be open due to error rate, got %v", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenState(t *testing.T) {
	config := DefaultCircuitBreakerConfig()
	config.MaxFailures = 2
	config.Timeout = 100 * time.Millisecond
	config.HalfOpenMaxRequests = 1
	cb := NewCircuitBreaker(config)
	ctx := context.Background()

	// Open the circuit
	for i := 0; i < 2; i++ {
		cb.Execute(ctx, func() error {
			return fmt.Errorf("failure")
		})
	}

	if cb.State() != StateOpen {
		t.Fatalf("Expected circuit to be open, got %v", cb.State())
	}

	// Wait for timeout
	time.Sleep(150 * time.Millisecond)

	// Next request should transition to half-open
	err := cb.Execute(ctx, func() error {
		return nil // Success
	})

	if err != nil {
		t.Errorf("Expected no error in half-open state, got %v", err)
	}

	// After success, circuit should be closed
	if cb.State() != StateClosed {
		t.Errorf("Expected circuit to be closed after successful half-open request, got %v", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenFailure(t *testing.T) {
	config := DefaultCircuitBreakerConfig()
	config.MaxFailures = 2
	config.Timeout = 100 * time.Millisecond
	config.HalfOpenMaxRequests = 1
	cb := NewCircuitBreaker(config)
	ctx := context.Background()

	// Open the circuit
	for i := 0; i < 2; i++ {
		cb.Execute(ctx, func() error {
			return fmt.Errorf("failure")
		})
	}

	// Wait for timeout
	time.Sleep(150 * time.Millisecond)

	// Next request should transition to half-open, then fail and reopen
	err := cb.Execute(ctx, func() error {
		return fmt.Errorf("failure")
	})

	if err == nil {
		t.Error("Expected error from failing function")
	}

	// Circuit should be open again
	if cb.State() != StateOpen {
		t.Errorf("Expected circuit to reopen after half-open failure, got %v", cb.State())
	}
}

func TestCircuitBreaker_Stats(t *testing.T) {
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig())
	ctx := context.Background()

	// Generate some successful and failed requests
	cb.Execute(ctx, func() error { return nil })
	cb.Execute(ctx, func() error { return fmt.Errorf("failure") })
	cb.Execute(ctx, func() error { return nil })

	stats := cb.Stats()

	if stats.TotalRequests != 3 {
		t.Errorf("Expected 3 total requests, got %d", stats.TotalRequests)
	}
	if stats.Successes != 2 {
		t.Errorf("Expected 2 successes, got %d", stats.Successes)
	}
	if stats.Failures != 1 {
		t.Errorf("Expected 1 failure, got %d", stats.Failures)
	}

	expectedErrorRate := 1.0 / 3.0
	if stats.ErrorRate != expectedErrorRate {
		t.Errorf("Expected error rate %.2f, got %.2f", expectedErrorRate, stats.ErrorRate)
	}
}

func TestCircuitBreaker_Reset(t *testing.T) {
	config := DefaultCircuitBreakerConfig()
	config.MaxFailures = 2
	cb := NewCircuitBreaker(config)
	ctx := context.Background()

	// Open the circuit
	for i := 0; i < 2; i++ {
		cb.Execute(ctx, func() error {
			return fmt.Errorf("failure")
		})
	}

	if cb.State() != StateOpen {
		t.Fatalf("Expected circuit to be open, got %v", cb.State())
	}

	// Reset the circuit
	cb.Reset()

	if cb.State() != StateClosed {
		t.Errorf("Expected circuit to be closed after reset, got %v", cb.State())
	}

	stats := cb.Stats()
	if stats.Failures != 0 || stats.Successes != 0 {
		t.Error("Expected counters to be reset")
	}
}

func TestCircuitBreaker_ForceOpen(t *testing.T) {
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig())

	if cb.State() != StateClosed {
		t.Fatalf("Expected initial state to be closed")
	}

	cb.ForceOpen()

	if cb.State() != StateOpen {
		t.Errorf("Expected circuit to be open after ForceOpen, got %v", cb.State())
	}
}

func TestCircuitBreaker_StateChangeCallback(t *testing.T) {
	var transitions []string
	var mu sync.Mutex
	config := DefaultCircuitBreakerConfig()
	config.MaxFailures = 2
	config.Timeout = 100 * time.Millisecond
	config.OnStateChange = func(from, to CircuitState) {
		mu.Lock()
		transitions = append(transitions, fmt.Sprintf("%s->%s", from, to))
		mu.Unlock()
	}

	cb := NewCircuitBreaker(config)
	ctx := context.Background()

	// Open the circuit
	for i := 0; i < 2; i++ {
		cb.Execute(ctx, func() error {
			return fmt.Errorf("failure")
		})
	}

	// Wait for timeout and transition to half-open
	time.Sleep(150 * time.Millisecond)

	// Success in half-open should close circuit
	cb.Execute(ctx, func() error {
		return nil
	})

	// Give callbacks time to execute
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	numTransitions := len(transitions)
	transitionsCopy := make([]string, len(transitions))
	copy(transitionsCopy, transitions)
	mu.Unlock()

	if numTransitions < 2 {
		t.Errorf("Expected at least 2 state transitions, got %d: %v", numTransitions, transitionsCopy)
	}
}

func TestCircuitBreaker_HalfOpenMaxRequests(t *testing.T) {
	config := DefaultCircuitBreakerConfig()
	config.MaxFailures = 2
	config.Timeout = 50 * time.Millisecond
	config.HalfOpenMaxRequests = 2
	cb := NewCircuitBreaker(config)
	ctx := context.Background()

	// Open the circuit
	for i := 0; i < 2; i++ {
		cb.Execute(ctx, func() error {
			return fmt.Errorf("failure")
		})
	}

	// Wait for timeout
	time.Sleep(100 * time.Millisecond)

	// Should allow HalfOpenMaxRequests
	for i := 0; i < config.HalfOpenMaxRequests; i++ {
		err := cb.Execute(ctx, func() error {
			return nil
		})
		if err != nil {
			t.Errorf("Request %d should succeed in half-open, got error: %v", i, err)
		}
	}

	// Circuit should be closed after successful requests
	if cb.State() != StateClosed {
		t.Errorf("Expected circuit to be closed, got %v", cb.State())
	}
}

func TestCircuitBreaker_ExecuteWithRetry(t *testing.T) {
	config := DefaultCircuitBreakerConfig()
	config.MaxFailures = 5
	cb := NewCircuitBreaker(config)
	ctx := context.Background()

	retryPolicy := &RetryPolicy{
		MaxAttempts:  2,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		RetryableErrors: map[ErrorCategory]bool{
			CategoryNetwork: true,
		},
	}

	attempts := 0
	err := cb.ExecuteWithRetry(ctx, retryPolicy, func() error {
		attempts++
		if attempts < 2 {
			return NetworkError("temporary failure", fmt.Errorf("network error"))
		}
		return nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}

	// Circuit should still be closed
	if cb.State() != StateClosed {
		t.Errorf("Expected circuit to be closed, got %v", cb.State())
	}
}

func TestCircuitState_String(t *testing.T) {
	tests := []struct {
		state CircuitState
		want  string
	}{
		{StateClosed, "closed"},
		{StateOpen, "open"},
		{StateHalfOpen, "half-open"},
		{CircuitState(999), "unknown"},
	}

	for _, tt := range tests {
		got := tt.state.String()
		if got != tt.want {
			t.Errorf("CircuitState(%d).String() = %s, want %s", tt.state, got, tt.want)
		}
	}
}

func TestCircuitBreaker_MinRequests(t *testing.T) {
	config := DefaultCircuitBreakerConfig()
	config.MaxFailures = 100 // High to not trigger
	config.FailureThreshold = 0.5
	config.MinRequests = 10
	cb := NewCircuitBreaker(config)
	ctx := context.Background()

	// Generate 5 requests with 100% failure rate (below MinRequests)
	for i := 0; i < 5; i++ {
		cb.Execute(ctx, func() error {
			return fmt.Errorf("failure")
		})
	}

	// Circuit should still be closed (below MinRequests threshold)
	if cb.State() != StateClosed {
		t.Errorf("Expected circuit to remain closed (below MinRequests), got %v", cb.State())
	}

	// Generate 5 more requests (total 10, at MinRequests)
	for i := 0; i < 5; i++ {
		cb.Execute(ctx, func() error {
			return fmt.Errorf("failure")
		})
	}

	// Now circuit should open (100% failure rate at MinRequests)
	if cb.State() != StateOpen {
		t.Errorf("Expected circuit to open at MinRequests, got %v", cb.State())
	}
}
