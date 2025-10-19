// Package pool provides resource pooling for performance optimization.
package pool

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/opd-ai/go-tor/pkg/connection"
	"github.com/opd-ai/go-tor/pkg/logger"
)

// ConnectionPool manages a pool of reusable connections to Tor relays
type ConnectionPool struct {
	mu          sync.RWMutex
	connections map[string]*pooledConnection
	maxIdle     int
	maxLifetime time.Duration
	logger      *logger.Logger
}

type pooledConnection struct {
	conn      *connection.Connection
	inUse     bool
	lastUsed  time.Time
	createdAt time.Time
}

// ConnectionPoolConfig holds configuration for the connection pool
type ConnectionPoolConfig struct {
	MaxIdlePerHost int           // Maximum idle connections per host
	MaxLifetime    time.Duration // Maximum lifetime of a connection
}

// DefaultConnectionPoolConfig returns sensible defaults for connection pooling
func DefaultConnectionPoolConfig() *ConnectionPoolConfig {
	return &ConnectionPoolConfig{
		MaxIdlePerHost: 5,
		MaxLifetime:    10 * time.Minute,
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

	return &ConnectionPool{
		connections: make(map[string]*pooledConnection),
		maxIdle:     cfg.MaxIdlePerHost,
		maxLifetime: cfg.MaxLifetime,
		logger:      log.Component("conn-pool"),
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
				pc.inUse = true
				pc.lastUsed = time.Now()
				p.logger.Debug("Reusing pooled connection", "address", address)
				return pc.conn, nil
			}
			// Connection too old, close it
			p.logger.Debug("Closing old pooled connection", "address", address, "age", time.Since(pc.createdAt))
			pc.conn.Close()
			delete(p.connections, key)
		}
	}

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
		pc.conn.Close()
		delete(p.connections, key)
		p.logger.Debug("Removed connection from pool", "address", address)
	}
}

// CleanupIdle closes idle connections that haven't been used recently
func (p *ConnectionPool) CleanupIdle(maxIdleTime time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	for key, pc := range p.connections {
		if !pc.inUse && now.Sub(pc.lastUsed) > maxIdleTime {
			p.logger.Debug("Closing idle connection", "address", key, "idle_time", now.Sub(pc.lastUsed))
			pc.conn.Close()
			delete(p.connections, key)
		}
	}
}

// CleanupExpired closes connections that have exceeded their maximum lifetime
func (p *ConnectionPool) CleanupExpired() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	for key, pc := range p.connections {
		if now.Sub(pc.createdAt) > p.maxLifetime {
			p.logger.Debug("Closing expired connection", "address", key, "age", now.Sub(pc.createdAt))
			pc.conn.Close()
			delete(p.connections, key)
		}
	}
}

// Close closes all connections in the pool
func (p *ConnectionPool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	for key, pc := range p.connections {
		p.logger.Debug("Closing pooled connection", "address", key)
		pc.conn.Close()
	}
	p.connections = make(map[string]*pooledConnection)

	return nil
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
