// Package errors provides structured error types and recovery mechanisms
package errors

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"
)

// RetryPolicy defines how retry attempts should be executed
type RetryPolicy struct {
	// MaxAttempts is the maximum number of retry attempts (0 = no retries)
	MaxAttempts int

	// InitialDelay is the delay before the first retry
	InitialDelay time.Duration

	// MaxDelay is the maximum delay between retries
	MaxDelay time.Duration

	// Multiplier is the factor to multiply the delay by after each attempt
	Multiplier float64

	// Jitter adds randomness to the delay to prevent thundering herd
	// Value should be between 0.0 and 1.0
	// 0.0 = no jitter, 1.0 = full jitter (delay can be 0 to 2x calculated delay)
	Jitter float64

	// RetryableErrors defines which error categories should be retried
	// If nil, only errors marked as Retryable will be retried
	RetryableErrors map[ErrorCategory]bool
}

// DefaultRetryPolicy returns a sensible default retry policy
func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		Jitter:       0.1,
		RetryableErrors: map[ErrorCategory]bool{
			CategoryConnection: true,
			CategoryCircuit:    true,
			CategoryDirectory:  true,
			CategoryTimeout:    true,
			CategoryNetwork:    true,
		},
	}
}

// AggressiveRetryPolicy returns a more aggressive retry policy
// Use this for critical operations that must succeed
func AggressiveRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts:  5,
		InitialDelay: 500 * time.Millisecond,
		MaxDelay:     60 * time.Second,
		Multiplier:   2.5,
		Jitter:       0.2,
		RetryableErrors: map[ErrorCategory]bool{
			CategoryConnection: true,
			CategoryCircuit:    true,
			CategoryDirectory:  true,
			CategoryTimeout:    true,
			CategoryNetwork:    true,
		},
	}
}

// ConservativeRetryPolicy returns a more conservative retry policy
// Use this for operations where retries are expensive
func ConservativeRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts:  2,
		InitialDelay: 2 * time.Second,
		MaxDelay:     15 * time.Second,
		Multiplier:   1.5,
		Jitter:       0.05,
		RetryableErrors: map[ErrorCategory]bool{
			CategoryConnection: true,
			CategoryTimeout:    true,
			CategoryNetwork:    true,
		},
	}
}

// RetryableFunc is a function that can be retried
type RetryableFunc func() error

// RetryWithPolicy executes a function with retry logic based on the policy
// Returns the last error if all attempts fail
func RetryWithPolicy(ctx context.Context, policy *RetryPolicy, fn RetryableFunc) error {
	if policy == nil {
		policy = DefaultRetryPolicy()
	}

	var lastErr error
	for attempt := 0; attempt <= policy.MaxAttempts; attempt++ {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled: %w", ctx.Err())
		default:
		}

		// Execute the function
		err := fn()
		if err == nil {
			return nil // Success!
		}

		lastErr = err

		// Check if this error is retryable
		if !policy.shouldRetry(err) {
			return err // Not retryable, return immediately
		}

		// If this was the last attempt, return the error
		if attempt >= policy.MaxAttempts {
			return fmt.Errorf("max retry attempts (%d) exceeded: %w", policy.MaxAttempts, err)
		}

		// Calculate delay with exponential backoff
		delay := policy.calculateDelay(attempt)

		// Wait before next attempt
		select {
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled during backoff: %w", ctx.Err())
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	return lastErr
}

// Retry executes a function with default retry policy
func Retry(ctx context.Context, fn RetryableFunc) error {
	return RetryWithPolicy(ctx, DefaultRetryPolicy(), fn)
}

// shouldRetry determines if an error should be retried based on the policy
func (p *RetryPolicy) shouldRetry(err error) bool {
	// First check if error is marked as retryable
	if IsRetryable(err) {
		return true
	}

	// Then check if error category is in our retryable list
	if p.RetryableErrors != nil {
		category := GetCategory(err)
		return p.RetryableErrors[category]
	}

	return false
}

// calculateDelay calculates the delay for a given attempt with exponential backoff and jitter
func (p *RetryPolicy) calculateDelay(attempt int) time.Duration {
	// Calculate base delay with exponential backoff
	delay := float64(p.InitialDelay) * math.Pow(p.Multiplier, float64(attempt))

	// Apply max delay cap
	if delay > float64(p.MaxDelay) {
		delay = float64(p.MaxDelay)
	}

	// Apply jitter if configured
	if p.Jitter > 0 {
		// Add random jitter: delay * (1 ± jitter)
		jitterAmount := delay * p.Jitter
		delay = delay + (rand.Float64()*2-1)*jitterAmount

		// Ensure delay is not negative
		if delay < 0 {
			delay = 0
		}
	}

	return time.Duration(delay)
}

// RetryStats tracks retry attempt statistics
type RetryStats struct {
	TotalAttempts   int
	SuccessfulRetry bool
	FinalError      error
	TotalDuration   time.Duration
}

// RetryWithStats executes a function with retry logic and returns statistics
func RetryWithStats(ctx context.Context, policy *RetryPolicy, fn RetryableFunc) (*RetryStats, error) {
	startTime := time.Now()
	stats := &RetryStats{}

	if policy == nil {
		policy = DefaultRetryPolicy()
	}

	var lastErr error
	for attempt := 0; attempt <= policy.MaxAttempts; attempt++ {
		stats.TotalAttempts++

		// Check if context is cancelled
		select {
		case <-ctx.Done():
			stats.FinalError = fmt.Errorf("retry cancelled: %w", ctx.Err())
			stats.TotalDuration = time.Since(startTime)
			return stats, stats.FinalError
		default:
		}

		// Execute the function
		err := fn()
		if err == nil {
			stats.SuccessfulRetry = (attempt > 0)
			stats.TotalDuration = time.Since(startTime)
			return stats, nil // Success!
		}

		lastErr = err

		// Check if this error is retryable
		if !policy.shouldRetry(err) {
			stats.FinalError = err
			stats.TotalDuration = time.Since(startTime)
			return stats, err // Not retryable
		}

		// If this was the last attempt, return the error
		if attempt >= policy.MaxAttempts {
			stats.FinalError = fmt.Errorf("max retry attempts (%d) exceeded: %w", policy.MaxAttempts, err)
			stats.TotalDuration = time.Since(startTime)
			return stats, stats.FinalError
		}

		// Calculate delay with exponential backoff
		delay := policy.calculateDelay(attempt)

		// Wait before next attempt
		select {
		case <-ctx.Done():
			stats.FinalError = fmt.Errorf("retry cancelled during backoff: %w", ctx.Err())
			stats.TotalDuration = time.Since(startTime)
			return stats, stats.FinalError
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	stats.FinalError = lastErr
	stats.TotalDuration = time.Since(startTime)
	return stats, lastErr
}

// RetryWithCallback executes a function with retry logic and calls a callback after each attempt
type RetryCallback func(attempt int, err error, willRetry bool)

// RetryWithCallbackFunc executes a function with retry and callback for monitoring
func RetryWithCallbackFunc(ctx context.Context, policy *RetryPolicy, fn RetryableFunc, callback RetryCallback) error {
	if policy == nil {
		policy = DefaultRetryPolicy()
	}

	var lastErr error
	for attempt := 0; attempt <= policy.MaxAttempts; attempt++ {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled: %w", ctx.Err())
		default:
		}

		// Execute the function
		err := fn()
		if err == nil {
			if callback != nil {
				callback(attempt, nil, false)
			}
			return nil // Success!
		}

		lastErr = err

		// Check if this error is retryable
		willRetry := policy.shouldRetry(err) && attempt < policy.MaxAttempts

		// Call callback to notify about the attempt
		if callback != nil {
			callback(attempt, err, willRetry)
		}

		if !willRetry {
			return err
		}

		// Calculate delay with exponential backoff
		delay := policy.calculateDelay(attempt)

		// Wait before next attempt
		select {
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled during backoff: %w", ctx.Err())
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	return lastErr
}
