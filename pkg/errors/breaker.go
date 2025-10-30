// Package errors provides circuit breaker pattern for fault tolerance
package errors

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// CircuitState represents the state of a circuit breaker
type CircuitState int

const (
	// StateClosed means circuit is operating normally
	StateClosed CircuitState = iota
	// StateOpen means circuit is broken, all requests fail fast
	StateOpen
	// StateHalfOpen means circuit is testing if service recovered
	StateHalfOpen
)

// String returns the string representation of the circuit state
func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreakerConfig defines circuit breaker behavior
type CircuitBreakerConfig struct {
	// MaxFailures is the number of failures before opening the circuit
	MaxFailures int

	// Timeout is how long the circuit stays open before transitioning to half-open
	Timeout time.Duration

	// HalfOpenMaxRequests is the number of requests allowed in half-open state
	HalfOpenMaxRequests int

	// FailureThreshold is the error rate threshold (0.0-1.0) to open the circuit
	// Example: 0.5 means 50% error rate will open the circuit
	FailureThreshold float64

	// MinRequests is the minimum number of requests needed before checking threshold
	MinRequests int

	// OnStateChange is called when circuit state changes (optional)
	OnStateChange func(from, to CircuitState)
}

// DefaultCircuitBreakerConfig returns sensible defaults
func DefaultCircuitBreakerConfig() *CircuitBreakerConfig {
	return &CircuitBreakerConfig{
		MaxFailures:         5,
		Timeout:             30 * time.Second,
		HalfOpenMaxRequests: 1,
		FailureThreshold:    0.5, // 50% error rate
		MinRequests:         10,  // Need at least 10 requests before checking threshold
		OnStateChange:       nil,
	}
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	config *CircuitBreakerConfig
	mu     sync.RWMutex
	state  CircuitState

	// Counters for closed state
	failures        int
	successes       int
	totalRequests   int
	lastFailureTime time.Time

	// Counters for half-open state
	halfOpenRequests int
	halfOpenFailures int

	// Timestamp when circuit was opened
	openedAt time.Time
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config *CircuitBreakerConfig) *CircuitBreaker {
	if config == nil {
		config = DefaultCircuitBreakerConfig()
	}

	return &CircuitBreaker{
		config: config,
		state:  StateClosed,
	}
}

// Execute executes a function with circuit breaker protection
func (cb *CircuitBreaker) Execute(ctx context.Context, fn RetryableFunc) error {
	// Check if circuit allows execution
	if err := cb.beforeRequest(); err != nil {
		return err
	}

	// Execute the function
	err := fn()

	// Record the result
	cb.afterRequest(err)

	return err
}

// beforeRequest checks if the request should be allowed
func (cb *CircuitBreaker) beforeRequest() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		// Allow request in closed state
		return nil

	case StateOpen:
		// Check if timeout has passed
		if time.Since(cb.openedAt) >= cb.config.Timeout {
			// Transition to half-open
			cb.changeState(StateHalfOpen)
			cb.halfOpenRequests = 0
			cb.halfOpenFailures = 0
			return nil
		}
		// Circuit is still open, fail fast
		return &TorError{
			Category:  CategoryInternal,
			Severity:  SeverityMedium,
			Message:   fmt.Sprintf("circuit breaker is open, will retry in %v", cb.config.Timeout-time.Since(cb.openedAt)),
			Retryable: true,
		}

	case StateHalfOpen:
		// Allow limited requests in half-open state
		if cb.halfOpenRequests >= cb.config.HalfOpenMaxRequests {
			return &TorError{
				Category:  CategoryInternal,
				Severity:  SeverityMedium,
				Message:   "circuit breaker is half-open, max requests reached",
				Retryable: true,
			}
		}
		cb.halfOpenRequests++
		return nil

	default:
		return fmt.Errorf("unknown circuit breaker state: %v", cb.state)
	}
}

// afterRequest records the result of a request
func (cb *CircuitBreaker) afterRequest(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		cb.totalRequests++

		if err != nil {
			cb.failures++
			cb.lastFailureTime = time.Now()

			// Check if we should open the circuit
			if cb.shouldOpen() {
				cb.changeState(StateOpen)
				cb.openedAt = time.Now()
			}
		} else {
			cb.successes++
		}

	case StateHalfOpen:
		if err != nil {
			cb.halfOpenFailures++
			// Any failure in half-open state reopens the circuit
			cb.changeState(StateOpen)
			cb.openedAt = time.Now()
		} else {
			// Success in half-open state closes the circuit
			cb.changeState(StateClosed)
			cb.reset()
		}
	}
}

// shouldOpen determines if the circuit should be opened
func (cb *CircuitBreaker) shouldOpen() bool {
	// Check if we've exceeded max consecutive failures
	if cb.failures >= cb.config.MaxFailures {
		return true
	}

	// Check error rate threshold
	if cb.totalRequests >= cb.config.MinRequests {
		errorRate := float64(cb.failures) / float64(cb.totalRequests)
		if errorRate >= cb.config.FailureThreshold {
			return true
		}
	}

	return false
}

// changeState transitions the circuit breaker to a new state
func (cb *CircuitBreaker) changeState(newState CircuitState) {
	oldState := cb.state
	cb.state = newState

	if cb.config.OnStateChange != nil {
		// Call callback without holding lock to prevent deadlock
		go cb.config.OnStateChange(oldState, newState)
	}
}

// reset resets the circuit breaker counters
func (cb *CircuitBreaker) reset() {
	cb.failures = 0
	cb.successes = 0
	cb.totalRequests = 0
	cb.halfOpenRequests = 0
	cb.halfOpenFailures = 0
}

// State returns the current state of the circuit breaker
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Stats returns current statistics about the circuit breaker
func (cb *CircuitBreaker) Stats() CircuitBreakerStats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	var errorRate float64
	if cb.totalRequests > 0 {
		errorRate = float64(cb.failures) / float64(cb.totalRequests)
	}

	return CircuitBreakerStats{
		State:            cb.state,
		Failures:         cb.failures,
		Successes:        cb.successes,
		TotalRequests:    cb.totalRequests,
		ErrorRate:        errorRate,
		HalfOpenRequests: cb.halfOpenRequests,
		HalfOpenFailures: cb.halfOpenFailures,
		LastFailureTime:  cb.lastFailureTime,
		OpenedAt:         cb.openedAt,
	}
}

// CircuitBreakerStats holds circuit breaker statistics
type CircuitBreakerStats struct {
	State            CircuitState
	Failures         int
	Successes        int
	TotalRequests    int
	ErrorRate        float64
	HalfOpenRequests int
	HalfOpenFailures int
	LastFailureTime  time.Time
	OpenedAt         time.Time
}

// Reset manually resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	oldState := cb.state
	cb.state = StateClosed
	cb.reset()

	if oldState != StateClosed && cb.config.OnStateChange != nil {
		go cb.config.OnStateChange(oldState, StateClosed)
	}
}

// ForceOpen manually opens the circuit breaker
func (cb *CircuitBreaker) ForceOpen() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	oldState := cb.state
	if oldState != StateOpen {
		cb.state = StateOpen
		cb.openedAt = time.Now()

		if cb.config.OnStateChange != nil {
			go cb.config.OnStateChange(oldState, StateOpen)
		}
	}
}

// ExecuteWithRetry combines circuit breaker and retry logic
func (cb *CircuitBreaker) ExecuteWithRetry(ctx context.Context, policy *RetryPolicy, fn RetryableFunc) error {
	return RetryWithPolicy(ctx, policy, func() error {
		return cb.Execute(ctx, fn)
	})
}
