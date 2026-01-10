// Package pool provides resource pooling for performance optimization.
package pool

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/opd-ai/go-tor/pkg/connection"
	"github.com/opd-ai/go-tor/pkg/logger"
	"github.com/opd-ai/go-tor/pkg/metrics"
)

// ConnectionPool manages a pool of reusable connections to Tor relays
type ConnectionPool struct {
	mu          sync.RWMutex
	connections map[string]*pooledConnection
	maxIdle     int
	maxLifetime time.Duration
	maxIdleTime time.Duration
	logger      *logger.Logger
	metrics     *metrics.Metrics
}

type pooledConnection struct {
	conn      *connection.Connection
	inUse     bool
	lastUsed  time.Time
	createdAt time.Time
}

// ConnectionPoolConfig holds configuration for the connection pool
type ConnectionPoolConfig struct {
	MaxIdlePerHost int              // Maximum idle connections per host
	MaxLifetime    time.Duration    // Maximum lifetime of a connection
	MaxIdleTime    time.Duration    // Maximum idle time before health check (default: 30s)
	Metrics        *metrics.Metrics // Optional metrics collector
}

// DefaultConnectionPoolConfig returns sensible defaults for connection pooling
func DefaultConnectionPoolConfig() *ConnectionPoolConfig {
	return &ConnectionPoolConfig{
		MaxIdlePerHost: 5,
		MaxLifetime:    10 * time.Minute,
		MaxIdleTime:    30 * time.Second,
		Metrics:        nil,
	}
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool(cfg *ConnectionPoolConfig, log *logger.Logger) *ConnectionPool {
	if cfg == nil {
		cfg = DefaultConnectionPoolConfig()
	}
	if log == nil {
		log = logger.NewDefault()
	}
	if cfg.MaxIdleTime == 0 {
		cfg.MaxIdleTime = 30 * time.Second
	}

	return &ConnectionPool{
		connections: make(map[string]*pooledConnection),
		maxIdle:     cfg.MaxIdlePerHost,
		maxLifetime: cfg.MaxLifetime,
		maxIdleTime: cfg.MaxIdleTime,
		logger:      log.Component("conn-pool"),
		metrics:     cfg.Metrics,
	}
}

// Get retrieves a connection from the pool or creates a new one
func (p *ConnectionPool) Get(ctx context.Context, address string, cfg *connection.Config) (*connection.Connection, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	key := address

	// Try to reuse an existing connection
	if pc, ok := p.connections[key]; ok {
		// Check if connection is still valid
		if !pc.inUse && pc.conn.GetState() == connection.StateOpen {
			// Check connection age
			if time.Since(pc.createdAt) < p.maxLifetime {
				// Perform health check if connection has been idle for a while
				if time.Since(pc.lastUsed) > p.maxIdleTime {
					if !p.healthCheck(pc) {
						p.logger.Debug("Health check failed for idle connection", "address", address)
						p.recordHealthCheckFailed()
						if err := pc.conn.Close(); err != nil {
							p.logger.Error("Failed to close unhealthy connection", "function", "Get", "address", address, "error", err)
						}
						delete(p.connections, key)
						p.recordConnectionClosed()
						p.updatePoolSize()
						// Fall through to create a new connection
						goto createNew
					}
				}
				pc.inUse = true
				pc.lastUsed = time.Now()
				p.logger.Debug("Reusing pooled connection", "address", address)
				p.recordConnectionReused()
				return pc.conn, nil
			}
			// Connection too old, close it
			p.logger.Debug("Closing old pooled connection", "address", address, "age", time.Since(pc.createdAt))
			if err := pc.conn.Close(); err != nil {
				p.logger.Error("Failed to close old pooled connection", "function", "Get", "address", address, "error", err)
			}
			delete(p.connections, key)
			p.recordConnectionClosed()
			p.updatePoolSize()
		}
	}

createNew:
	// Create a new connection
	p.logger.Debug("Creating new pooled connection", "address", address)
	conn := connection.New(cfg, p.logger)

	if err := conn.Connect(ctx, cfg); err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	// Store in pool
	p.connections[key] = &pooledConnection{
		conn:      conn,
		inUse:     true,
		lastUsed:  time.Now(),
		createdAt: time.Now(),
	}
	p.recordConnectionCreated()
	p.updatePoolSize()

	return conn, nil
}

// Put returns a connection to the pool
func (p *ConnectionPool) Put(address string, conn *connection.Connection) {
	p.mu.Lock()
	defer p.mu.Unlock()

	key := address

	if pc, ok := p.connections[key]; ok && pc.conn == conn {
		pc.inUse = false
		pc.lastUsed = time.Now()
		p.logger.Debug("Returned connection to pool", "address", address)
	}
}

// Remove removes a connection from the pool
func (p *ConnectionPool) Remove(address string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	key := address

	if pc, ok := p.connections[key]; ok {
		if err := pc.conn.Close(); err != nil {
			p.logger.Error("Failed to close connection during removal", "function", "Remove", "address", address, "error", err)
		}
		delete(p.connections, key)
		p.recordConnectionClosed()
		p.updatePoolSize()
		p.logger.Debug("Removed connection from pool", "address", address)
	}
}

// CleanupIdle closes idle connections that haven't been used recently
func (p *ConnectionPool) CleanupIdle(maxIdleTime time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	closedCount := 0
	for key, pc := range p.connections {
		if !pc.inUse && now.Sub(pc.lastUsed) > maxIdleTime {
			p.logger.Debug("Closing idle connection", "address", key, "idle_time", now.Sub(pc.lastUsed))
			if err := pc.conn.Close(); err != nil {
				p.logger.Error("Failed to close idle connection", "function", "CleanupIdle", "address", key, "error", err)
			}
			delete(p.connections, key)
			closedCount++
		}
	}
	if closedCount > 0 {
		for i := 0; i < closedCount; i++ {
			p.recordConnectionClosed()
		}
		p.updatePoolSize()
	}
}

// CleanupExpired closes connections that have exceeded their maximum lifetime
func (p *ConnectionPool) CleanupExpired() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	closedCount := 0
	for key, pc := range p.connections {
		if now.Sub(pc.createdAt) > p.maxLifetime {
			p.logger.Debug("Closing expired connection", "address", key, "age", now.Sub(pc.createdAt))
			if err := pc.conn.Close(); err != nil {
				p.logger.Error("Failed to close expired connection", "function", "CleanupExpired", "address", key, "error", err)
			}
			delete(p.connections, key)
			closedCount++
		}
	}
	if closedCount > 0 {
		for i := 0; i < closedCount; i++ {
			p.recordConnectionClosed()
		}
		p.updatePoolSize()
	}
}

// Close closes all connections in the pool
func (p *ConnectionPool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var lastErr error
	closedCount := 0
	for key, pc := range p.connections {
		p.logger.Debug("Closing pooled connection", "address", key)
		if err := pc.conn.Close(); err != nil {
			p.logger.Error("Failed to close pooled connection", "function", "Close", "address", key, "error", err)
			lastErr = err
		}
		closedCount++
	}
	p.connections = make(map[string]*pooledConnection)
	for i := 0; i < closedCount; i++ {
		p.recordConnectionClosed()
	}
	p.updatePoolSize()

	return lastErr
}

// Stats returns statistics about the connection pool
func (p *ConnectionPool) Stats() PoolStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := PoolStats{
		Total: len(p.connections),
	}

	for _, pc := range p.connections {
		if pc.inUse {
			stats.InUse++
		} else {
			stats.Idle++
		}
	}

	return stats
}

// PoolStats holds statistics about the connection pool
type PoolStats struct {
	Total int
	InUse int
	Idle  int
}

// healthCheck validates that a pooled connection is still usable.
// It uses the connection's Ping() method to verify the connection is alive.
func (p *ConnectionPool) healthCheck(pc *pooledConnection) bool {
	if pc == nil || pc.conn == nil {
		return false
	}
	return pc.conn.Ping()
}

// recordConnectionCreated records a new connection creation in metrics
func (p *ConnectionPool) recordConnectionCreated() {
	if p.metrics != nil {
		p.metrics.RecordPoolConnectionCreated()
	}
}

// recordConnectionReused records a connection reuse in metrics
func (p *ConnectionPool) recordConnectionReused() {
	if p.metrics != nil {
		p.metrics.RecordPoolConnectionReused()
	}
}

// recordConnectionClosed records a connection closure in metrics
func (p *ConnectionPool) recordConnectionClosed() {
	if p.metrics != nil {
		p.metrics.RecordPoolConnectionClosed()
	}
}

// recordHealthCheckFailed records a failed health check in metrics
func (p *ConnectionPool) recordHealthCheckFailed() {
	if p.metrics != nil {
		p.metrics.RecordPoolHealthCheckFailed()
	}
}

// updatePoolSize updates the pool size gauge in metrics
func (p *ConnectionPool) updatePoolSize() {
	if p.metrics != nil {
		p.metrics.SetPoolSize(int64(len(p.connections)))
	}
}

// SetMetrics sets the metrics collector for the connection pool.
// This can be used to attach metrics after pool creation.
func (p *ConnectionPool) SetMetrics(m *metrics.Metrics) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.metrics = m
	p.updatePoolSize()
}
