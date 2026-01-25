package relay

import (
	"context"
	"fmt"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter implements token bucket rate limiting for relay operations
type RateLimiter struct {
	// Circuit creation rate limiting
	circuitLimiter *rate.Limiter
	circuitBurst   int

	// Connection rate limiting per IP
	connLimiters   map[string]*rate.Limiter
	connMu         sync.RWMutex
	connRate       rate.Limit
	connBurst      int
	connCleanupTTL time.Duration

	// Cell processing rate limiting per circuit
	cellLimiters   map[uint32]*rate.Limiter
	cellMu         sync.RWMutex
	cellRate       rate.Limit
	cellBurst      int
	cellCleanupTTL time.Duration

	// Cleanup tracking
	lastCleanup time.Time
	cleanupMu   sync.Mutex

	// Metrics
	metrics *RelayMetrics
}

// RateLimiterConfig holds rate limiting configuration
type RateLimiterConfig struct {
	// Circuit creation: circuits per second
	CircuitRate  float64
	CircuitBurst int

	// Connection rate per IP: connections per second
	ConnectionRate  float64
	ConnectionBurst int

	// Cell processing per circuit: cells per second
	CellRate  float64
	CellBurst int

	// Cleanup interval for stale limiters
	CleanupInterval time.Duration

	// Metrics for tracking rate limiting
	Metrics *RelayMetrics
}

// DefaultRateLimiterConfig returns sensible defaults
func DefaultRateLimiterConfig() *RateLimiterConfig {
	return &RateLimiterConfig{
		// Allow 10 circuit creations per second with burst of 20
		CircuitRate:  10.0,
		CircuitBurst: 20,

		// Allow 5 connections per IP per second with burst of 10
		ConnectionRate:  5.0,
		ConnectionBurst: 10,

		// Allow 100 cells per second per circuit with burst of 200
		CellRate:  100.0,
		CellBurst: 200,

		CleanupInterval: 5 * time.Minute,
	}
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(cfg *RateLimiterConfig) *RateLimiter {
	if cfg == nil {
		cfg = DefaultRateLimiterConfig()
	}

	return &RateLimiter{
		circuitLimiter: rate.NewLimiter(rate.Limit(cfg.CircuitRate), cfg.CircuitBurst),
		circuitBurst:   cfg.CircuitBurst,
		connLimiters:   make(map[string]*rate.Limiter),
		connRate:       rate.Limit(cfg.ConnectionRate),
		connBurst:      cfg.ConnectionBurst,
		connCleanupTTL: cfg.CleanupInterval,
		cellLimiters:   make(map[uint32]*rate.Limiter),
		cellRate:       rate.Limit(cfg.CellRate),
		cellBurst:      cfg.CellBurst,
		cellCleanupTTL: cfg.CleanupInterval,
		lastCleanup:    time.Now(),
		metrics:        cfg.Metrics,
	}
}

// AllowCircuit checks if a new circuit creation is allowed
func (rl *RateLimiter) AllowCircuit(ctx context.Context) error {
	if err := rl.circuitLimiter.Wait(ctx); err != nil {
		if rl.metrics != nil {
			rl.metrics.RateLimitedCircuits.Inc()
		}
		return fmt.Errorf("circuit rate limit exceeded: %w", err)
	}
	return nil
}

// AllowConnection checks if a new connection from the given IP is allowed
func (rl *RateLimiter) AllowConnection(ctx context.Context, ip string) error {
	// Periodic cleanup of stale limiters
	rl.maybeCleanup()

	// Get or create limiter for this IP
	rl.connMu.Lock()
	limiter, exists := rl.connLimiters[ip]
	if !exists {
		limiter = rate.NewLimiter(rl.connRate, rl.connBurst)
		rl.connLimiters[ip] = limiter
	}
	rl.connMu.Unlock()

	// Check rate limit
	if err := limiter.Wait(ctx); err != nil {
		if rl.metrics != nil {
			rl.metrics.RateLimitedConnections.Inc()
		}
		return fmt.Errorf("connection rate limit exceeded for IP %s: %w", ip, err)
	}

	return nil
}

// AllowCell checks if processing a cell on the given circuit is allowed
func (rl *RateLimiter) AllowCell(ctx context.Context, circuitID uint32) error {
	// Periodic cleanup of stale limiters
	rl.maybeCleanup()

	// Get or create limiter for this circuit
	rl.cellMu.Lock()
	limiter, exists := rl.cellLimiters[circuitID]
	if !exists {
		limiter = rate.NewLimiter(rl.cellRate, rl.cellBurst)
		rl.cellLimiters[circuitID] = limiter
	}
	rl.cellMu.Unlock()

	// Check rate limit
	if err := limiter.Wait(ctx); err != nil {
		if rl.metrics != nil {
			rl.metrics.RateLimitedCells.Inc()
		}
		return fmt.Errorf("cell rate limit exceeded for circuit %d: %w", circuitID, err)
	}

	return nil
}

// RemoveCircuit removes the rate limiter for a circuit (called when circuit closes)
func (rl *RateLimiter) RemoveCircuit(circuitID uint32) {
	rl.cellMu.Lock()
	delete(rl.cellLimiters, circuitID)
	rl.cellMu.Unlock()
}

// maybeCleanup performs periodic cleanup of stale limiters
func (rl *RateLimiter) maybeCleanup() {
	rl.cleanupMu.Lock()
	defer rl.cleanupMu.Unlock()

	if time.Since(rl.lastCleanup) < rl.connCleanupTTL {
		return
	}

	// Cleanup connection limiters (remove idle ones)
	rl.connMu.Lock()
	for ip, limiter := range rl.connLimiters {
		// If limiter has full tokens (not used recently), remove it
		if limiter.Tokens() >= float64(rl.connBurst) {
			delete(rl.connLimiters, ip)
		}
	}
	rl.connMu.Unlock()

	// Note: Circuit limiters are removed explicitly via RemoveCircuit()

	rl.lastCleanup = time.Now()
}

// Stats returns current rate limiting statistics
func (rl *RateLimiter) Stats() RateLimitStats {
	rl.connMu.RLock()
	connCount := len(rl.connLimiters)
	rl.connMu.RUnlock()

	rl.cellMu.RLock()
	circuitCount := len(rl.cellLimiters)
	rl.cellMu.RUnlock()

	return RateLimitStats{
		CircuitAvailable:   rl.circuitLimiter.Tokens(),
		CircuitBurst:       rl.circuitBurst,
		ActiveIPLimiters:   connCount,
		ActiveCellLimiters: circuitCount,
	}
}

// RateLimitStats contains rate limiting statistics
type RateLimitStats struct {
	CircuitAvailable   float64 // Available tokens for circuit creation
	CircuitBurst       int     // Maximum burst for circuits
	ActiveIPLimiters   int     // Number of active IP limiters
	ActiveCellLimiters int     // Number of active circuit cell limiters
}
