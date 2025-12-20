// Package ratelimit provides rate limiting functionality for the Tor client.
// It implements a token bucket algorithm for controlling the rate of operations
// such as SOCKS connections, circuit creation, and descriptor fetches.
package ratelimit

import (
	"context"
	"sync"
	"time"
)

// RateLimiter implements a token bucket rate limiter.
// It allows a configurable rate of operations with burst capacity.
type RateLimiter struct {
	rate       float64   // tokens per second
	burst      int       // maximum burst size (bucket capacity)
	tokens     float64   // current tokens in bucket
	lastUpdate time.Time // last time tokens were updated
	mu         sync.Mutex
}

// NewRateLimiter creates a new rate limiter with the specified rate and burst size.
// rate: tokens per second
// burst: maximum tokens that can be accumulated (burst capacity)
func NewRateLimiter(rate float64, burst int) *RateLimiter {
	if rate <= 0 {
		rate = 1.0
	}
	if burst <= 0 {
		burst = 1
	}
	return &RateLimiter{
		rate:       rate,
		burst:      burst,
		tokens:     float64(burst), // Start with full bucket
		lastUpdate: time.Now(),
	}
}

// Allow checks if an operation is allowed under the rate limit.
// Returns true if the operation is allowed (token consumed), false otherwise.
// This is a non-blocking check.
func (r *RateLimiter) Allow() bool {
	return r.AllowN(1)
}

// AllowN checks if n operations are allowed under the rate limit.
// Returns true if allowed (tokens consumed), false otherwise.
func (r *RateLimiter) AllowN(n int) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.refillTokens()

	needed := float64(n)
	if r.tokens >= needed {
		r.tokens -= needed
		return true
	}
	return false
}

// Wait blocks until an operation is allowed or the context is cancelled.
// Returns nil if the operation is allowed, or the context error if cancelled.
func (r *RateLimiter) Wait(ctx context.Context) error {
	return r.WaitN(ctx, 1)
}

// WaitN blocks until n operations are allowed or the context is cancelled.
// Returns nil if allowed, or the context error if cancelled.
func (r *RateLimiter) WaitN(ctx context.Context, n int) error {
	needed := float64(n)

	for {
		r.mu.Lock()
		r.refillTokens()

		if r.tokens >= needed {
			r.tokens -= needed
			r.mu.Unlock()
			return nil
		}

		// Calculate wait time for needed tokens
		deficit := needed - r.tokens
		waitTime := time.Duration(deficit / r.rate * float64(time.Second))
		r.mu.Unlock()

		// Wait for tokens to become available or context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
			// Continue loop to recheck tokens
		}
	}
}

// Reserve reserves n tokens for future use.
// Returns a Reservation that indicates when the tokens will be available.
//
// Behavior:
// - If tokens are available immediately, they are consumed and delay is 0.
// - If tokens are not available, delay indicates how long to wait before
//   calling Allow() or Wait() to actually consume the tokens.
func (r *RateLimiter) Reserve(n int) *Reservation {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.refillTokens()

	needed := float64(n)
	if r.tokens >= needed {
		r.tokens -= needed
		return &Reservation{
			ok:    true,
			delay: 0,
		}
	}

	// Calculate delay until tokens would be available
	// Do not deduct tokens that we don't have - the caller must wait
	// for the delay period and then call Allow() or Wait()
	deficit := needed - r.tokens
	delay := time.Duration(deficit / r.rate * float64(time.Second))

	return &Reservation{
		ok:    true,
		delay: delay,
	}
}

// refillTokens adds tokens based on time elapsed since last update.
// Must be called with mutex held.
func (r *RateLimiter) refillTokens() {
	now := time.Now()
	elapsed := now.Sub(r.lastUpdate).Seconds()
	r.lastUpdate = now

	// Add tokens based on elapsed time
	r.tokens += elapsed * r.rate

	// Cap at burst size
	if r.tokens > float64(r.burst) {
		r.tokens = float64(r.burst)
	}
}

// Tokens returns the current number of available tokens.
func (r *RateLimiter) Tokens() float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.refillTokens()
	return r.tokens
}

// Rate returns the rate limit in tokens per second.
func (r *RateLimiter) Rate() float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rate
}

// Burst returns the burst size (maximum tokens).
func (r *RateLimiter) Burst() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.burst
}

// SetRate updates the rate limit.
func (r *RateLimiter) SetRate(rate float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rate > 0 {
		r.rate = rate
	}
}

// SetBurst updates the burst size.
func (r *RateLimiter) SetBurst(burst int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if burst > 0 {
		r.burst = burst
		// Cap current tokens if needed
		if r.tokens > float64(burst) {
			r.tokens = float64(burst)
		}
	}
}

// Reservation represents a reserved operation.
type Reservation struct {
	ok    bool
	delay time.Duration
}

// OK returns true if the reservation was successful.
func (r *Reservation) OK() bool {
	return r.ok
}

// Delay returns the time to wait before the operation can proceed.
func (r *Reservation) Delay() time.Duration {
	return r.delay
}

// MultiLimiter combines multiple rate limiters.
// All limiters must allow an operation for it to proceed.
type MultiLimiter struct {
	limiters []*RateLimiter
}

// NewMultiLimiter creates a new multi-limiter from the given limiters.
func NewMultiLimiter(limiters ...*RateLimiter) *MultiLimiter {
	return &MultiLimiter{
		limiters: limiters,
	}
}

// Allow checks if an operation is allowed by all limiters.
// This is atomic - either all limiters allow and consume a token, or none do.
func (m *MultiLimiter) Allow() bool {
	if len(m.limiters) == 0 {
		return true
	}

	// Acquire all locks first to ensure atomicity
	for _, l := range m.limiters {
		l.mu.Lock()
	}

	// Check all limiters and refill tokens
	allAllowed := true
	for _, l := range m.limiters {
		l.refillTokens()
		if l.tokens < 1 {
			allAllowed = false
			break
		}
	}

	// If all allowed, consume tokens; otherwise release locks and return false
	if allAllowed {
		for _, l := range m.limiters {
			l.tokens--
		}
	}

	// Release all locks
	for _, l := range m.limiters {
		l.mu.Unlock()
	}

	return allAllowed
}

// Wait blocks until all limiters allow the operation.
// This preserves the atomicity guarantee of Allow: either all limiters
// allow and consume a token for this operation, or none do.
func (m *MultiLimiter) Wait(ctx context.Context) error {
	if len(m.limiters) == 0 {
		return nil
	}

	// Poll using the atomic Allow until it succeeds or the context is done.
	// A small sleep avoids busy-waiting while still being responsive.
	const retryDelay = 10 * time.Millisecond

	for {
		if m.Allow() {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(retryDelay):
			// Try again.
		}
	}
}

// KeyedRateLimiter maintains separate rate limiters per key.
// This is useful for per-client or per-circuit rate limiting.
type KeyedRateLimiter struct {
	rate     float64
	burst    int
	limiters map[string]*RateLimiter
	mu       sync.Mutex
}

// NewKeyedRateLimiter creates a new keyed rate limiter.
func NewKeyedRateLimiter(rate float64, burst int) *KeyedRateLimiter {
	return &KeyedRateLimiter{
		rate:     rate,
		burst:    burst,
		limiters: make(map[string]*RateLimiter),
	}
}

// Allow checks if an operation for the given key is allowed.
func (k *KeyedRateLimiter) Allow(key string) bool {
	k.mu.Lock()
	l, exists := k.limiters[key]
	if !exists {
		l = NewRateLimiter(k.rate, k.burst)
		k.limiters[key] = l
	}
	k.mu.Unlock()
	return l.Allow()
}

// Wait blocks until an operation for the given key is allowed.
func (k *KeyedRateLimiter) Wait(ctx context.Context, key string) error {
	k.mu.Lock()
	l, exists := k.limiters[key]
	if !exists {
		l = NewRateLimiter(k.rate, k.burst)
		k.limiters[key] = l
	}
	k.mu.Unlock()
	return l.Wait(ctx)
}

// Cleanup removes limiters that haven't been used recently.
// This should be called periodically to prevent memory leaks.
func (k *KeyedRateLimiter) Cleanup(maxAge time.Duration) {
	// Take a snapshot of the current limiters under k.mu to avoid
	// holding both k.mu and l.mu at the same time, which can lead
	// to lock-ordering deadlocks.
	k.mu.Lock()
	limiters := make([]struct {
		key string
		l   *RateLimiter
	}, 0, len(k.limiters))
	for key, l := range k.limiters {
		limiters = append(limiters, struct {
			key string
			l   *RateLimiter
		}{key: key, l: l})
	}
	k.mu.Unlock()

	now := time.Now()
	for _, item := range limiters {
		item.l.mu.Lock()
		expired := now.Sub(item.l.lastUpdate) > maxAge
		item.l.mu.Unlock()

		if !expired {
			continue
		}

		// Delete expired limiter under k.mu, ensuring it hasn't been
		// replaced concurrently with a different instance.
		k.mu.Lock()
		if cur, ok := k.limiters[item.key]; ok && cur == item.l {
			delete(k.limiters, item.key)
		}
		k.mu.Unlock()
	}
}

// Size returns the number of tracked keys.
func (k *KeyedRateLimiter) Size() int {
	k.mu.Lock()
	defer k.mu.Unlock()
	return len(k.limiters)
}
