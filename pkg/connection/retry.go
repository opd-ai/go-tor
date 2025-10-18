// Package connection provides connection retry logic with exponential backoff.
package connection

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/opd-ai/go-tor/pkg/logger"
)

// RetryConfig defines retry behavior for connections
type RetryConfig struct {
	// MaxAttempts is the maximum number of retry attempts (0 = no retries)
	MaxAttempts int
	// InitialBackoff is the initial backoff duration
	InitialBackoff time.Duration
	// MaxBackoff is the maximum backoff duration
	MaxBackoff time.Duration
	// BackoffMultiplier is the multiplier for exponential backoff
	BackoffMultiplier float64
	// Jitter adds randomness to backoff to prevent thundering herd
	Jitter bool
}

// DefaultRetryConfig returns a retry config with sensible defaults
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:       3,
		InitialBackoff:    1 * time.Second,
		MaxBackoff:        30 * time.Second,
		BackoffMultiplier: 2.0,
		Jitter:            true,
	}
}

// ConnectWithRetry attempts to connect with exponential backoff retry logic
func (c *Connection) ConnectWithRetry(ctx context.Context, cfg *Config, retryCfg *RetryConfig) error {
	if retryCfg == nil {
		retryCfg = DefaultRetryConfig()
	}

	var lastErr error
	backoff := retryCfg.InitialBackoff

	for attempt := 0; attempt <= retryCfg.MaxAttempts; attempt++ {
		// Check if context is cancelled before attempting
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled before connection attempt: %w", ctx.Err())
		default:
		}

		// Attempt connection
		if attempt == 0 {
			c.logger.Debug("Attempting connection", "address", cfg.Address)
		} else {
			c.logger.Info("Retrying connection",
				"attempt", attempt,
				"max_attempts", retryCfg.MaxAttempts,
				"backoff", backoff)
		}

		err := c.Connect(ctx, cfg)
		if err == nil {
			if attempt > 0 {
				c.logger.Info("Connection successful after retry",
					"attempts", attempt+1)
			}
			return nil
		}

		lastErr = err
		c.logger.Warn("Connection attempt failed",
			"attempt", attempt+1,
			"error", err)

		// Don't sleep after the last attempt
		if attempt >= retryCfg.MaxAttempts {
			break
		}

		// Calculate backoff with exponential increase
		currentBackoff := calculateBackoff(backoff, retryCfg, attempt)

		// Sleep with context awareness
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled during backoff: %w", ctx.Err())
		case <-time.After(currentBackoff):
			// Continue to next attempt
		}

		// Increase backoff for next iteration
		backoff = time.Duration(float64(backoff) * retryCfg.BackoffMultiplier)
		if backoff > retryCfg.MaxBackoff {
			backoff = retryCfg.MaxBackoff
		}
	}

	return fmt.Errorf("connection failed after %d attempts: %w", retryCfg.MaxAttempts+1, lastErr)
}

// calculateBackoff calculates the backoff duration with optional jitter
func calculateBackoff(base time.Duration, cfg *RetryConfig, attempt int) time.Duration {
	// Calculate exponential backoff
	backoff := time.Duration(float64(base) * math.Pow(cfg.BackoffMultiplier, float64(attempt)))

	// Cap at max backoff
	if backoff > cfg.MaxBackoff {
		backoff = cfg.MaxBackoff
	}

	// Add jitter if enabled (±25% randomness)
	if cfg.Jitter {
		jitterRange := float64(backoff) * 0.25
		// Simple jitter using time as pseudo-random source
		jitterValue := float64(time.Now().UnixNano()%1000) / 1000.0 // 0.0 to 1.0
		jitter := time.Duration((jitterValue - 0.5) * 2 * jitterRange)
		backoff += jitter
	}

	return backoff
}

// Pool manages a pool of connections with retry logic
type Pool struct {
	logger    *logger.Logger
	maxSize   int
	conns     []*Connection
	retryCfg  *RetryConfig
	available chan *Connection
}

// NewPool creates a new connection pool
func NewPool(maxSize int, retryCfg *RetryConfig, log *logger.Logger) *Pool {
	if log == nil {
		log = logger.NewDefault()
	}
	if retryCfg == nil {
		retryCfg = DefaultRetryConfig()
	}

	return &Pool{
		logger:    log.Component("connpool"),
		maxSize:   maxSize,
		conns:     make([]*Connection, 0, maxSize),
		retryCfg:  retryCfg,
		available: make(chan *Connection, maxSize),
	}
}

// Get retrieves an available connection from the pool or creates a new one
func (p *Pool) Get(ctx context.Context, cfg *Config) (*Connection, error) {
	select {
	case conn := <-p.available:
		if conn.IsOpen() {
			p.logger.Debug("Reusing connection from pool", "address", conn.Address())
			return conn, nil
		}
		p.logger.Debug("Connection from pool is closed, creating new one")
		// Connection is closed, create a new one
	case <-ctx.Done():
		return nil, fmt.Errorf("context cancelled while getting connection: %w", ctx.Err())
	default:
		// No available connection, create new one
	}

	// Create new connection
	conn := New(cfg, p.logger)
	if err := conn.ConnectWithRetry(ctx, cfg, p.retryCfg); err != nil {
		return nil, err
	}

	return conn, nil
}

// Put returns a connection to the pool
func (p *Pool) Put(conn *Connection) {
	if !conn.IsOpen() {
		p.logger.Debug("Not returning closed connection to pool")
		return
	}

	select {
	case p.available <- conn:
		p.logger.Debug("Returned connection to pool", "address", conn.Address())
	default:
		// Pool is full, close connection
		p.logger.Debug("Pool full, closing connection", "address", conn.Address())
		conn.Close()
	}
}

// Close closes all connections in the pool
func (p *Pool) Close() {
	p.logger.Info("Closing connection pool")
	close(p.available)

	// Drain available channel and close connections
	for conn := range p.available {
		conn.Close()
	}

	p.logger.Info("Connection pool closed")
}
