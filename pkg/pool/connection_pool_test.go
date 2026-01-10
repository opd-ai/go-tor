package pool

import (
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/logger"
	"github.com/opd-ai/go-tor/pkg/metrics"
)

func TestConnectionPoolCreation(t *testing.T) {
	log := logger.NewDefault()
	pool := NewConnectionPool(nil, log)

	if pool == nil {
		t.Fatal("Expected non-nil connection pool")
	}

	stats := pool.Stats()
	if stats.Total != 0 {
		t.Errorf("Expected 0 total connections, got %d", stats.Total)
	}
}

func TestConnectionPoolStats(t *testing.T) {
	log := logger.NewDefault()
	pool := NewConnectionPool(nil, log)

	stats := pool.Stats()
	if stats.Total != 0 || stats.InUse != 0 || stats.Idle != 0 {
		t.Errorf("Expected empty pool, got %+v", stats)
	}
}

func TestConnectionPoolClose(t *testing.T) {
	log := logger.NewDefault()
	pool := NewConnectionPool(nil, log)

	if err := pool.Close(); err != nil {
		t.Errorf("Failed to close pool: %v", err)
	}

	stats := pool.Stats()
	if stats.Total != 0 {
		t.Errorf("Expected 0 connections after close, got %d", stats.Total)
	}
}

func TestConnectionPoolCleanupExpired(t *testing.T) {
	log := logger.NewDefault()
	cfg := &ConnectionPoolConfig{
		MaxIdlePerHost: 5,
		MaxLifetime:    100 * time.Millisecond,
	}
	pool := NewConnectionPool(cfg, log)

	// Cleanup should not panic on empty pool
	pool.CleanupExpired()

	stats := pool.Stats()
	if stats.Total != 0 {
		t.Errorf("Expected 0 connections, got %d", stats.Total)
	}
}

func TestConnectionPoolCleanupIdle(t *testing.T) {
	log := logger.NewDefault()
	pool := NewConnectionPool(nil, log)

	// Cleanup should not panic on empty pool
	pool.CleanupIdle(1 * time.Minute)

	stats := pool.Stats()
	if stats.Total != 0 {
		t.Errorf("Expected 0 connections, got %d", stats.Total)
	}
}

func TestConnectionPoolRemove(t *testing.T) {
	log := logger.NewDefault()
	pool := NewConnectionPool(nil, log)

	// Remove non-existent connection should not panic
	pool.Remove("127.0.0.1:9001")

	stats := pool.Stats()
	if stats.Total != 0 {
		t.Errorf("Expected 0 connections, got %d", stats.Total)
	}
}

// Mock connection for testing (without actual network I/O)
func TestConnectionPoolConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		config *ConnectionPoolConfig
	}{
		{
			name:   "nil config",
			config: nil,
		},
		{
			name: "custom config",
			config: &ConnectionPoolConfig{
				MaxIdlePerHost: 10,
				MaxLifetime:    5 * time.Minute,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := logger.NewDefault()
			pool := NewConnectionPool(tt.config, log)
			if pool == nil {
				t.Fatal("Expected non-nil pool")
			}
		})
	}
}

func TestConnectionPoolWithMetrics(t *testing.T) {
	log := logger.NewDefault()
	m := metrics.New()

	cfg := &ConnectionPoolConfig{
		MaxIdlePerHost: 5,
		MaxLifetime:    10 * time.Minute,
		MaxIdleTime:    30 * time.Second,
		Metrics:        m,
	}

	pool := NewConnectionPool(cfg, log)
	if pool == nil {
		t.Fatal("Expected non-nil pool with metrics")
	}

	// Close the pool and verify metrics are updated
	if err := pool.Close(); err != nil {
		t.Errorf("Failed to close pool: %v", err)
	}

	// Pool size should be 0 after close
	snap := m.Snapshot()
	if snap.PoolSize != 0 {
		t.Errorf("Expected pool size 0 after close, got %d", snap.PoolSize)
	}
}

func TestConnectionPoolSetMetrics(t *testing.T) {
	log := logger.NewDefault()
	pool := NewConnectionPool(nil, log)

	// Initially no metrics
	if pool.metrics != nil {
		t.Error("Expected nil metrics initially")
	}

	// Set metrics
	m := metrics.New()
	pool.SetMetrics(m)

	if pool.metrics == nil {
		t.Error("Expected non-nil metrics after SetMetrics")
	}
}

func TestConnectionPoolHealthCheck(t *testing.T) {
	log := logger.NewDefault()
	pool := NewConnectionPool(nil, log)

	// Health check on nil pooledConnection should return false
	if pool.healthCheck(nil) {
		t.Error("healthCheck(nil) = true, want false")
	}

	// Health check on pooledConnection with nil conn should return false
	pc := &pooledConnection{
		conn: nil,
	}
	if pool.healthCheck(pc) {
		t.Error("healthCheck with nil conn = true, want false")
	}
}

func TestConnectionPoolConfigWithMaxIdleTime(t *testing.T) {
	log := logger.NewDefault()
	cfg := &ConnectionPoolConfig{
		MaxIdlePerHost: 5,
		MaxLifetime:    10 * time.Minute,
		MaxIdleTime:    0, // Should be set to default
	}

	pool := NewConnectionPool(cfg, log)
	if pool.maxIdleTime != 30*time.Second {
		t.Errorf("maxIdleTime = %v, want %v", pool.maxIdleTime, 30*time.Second)
	}
}

func TestConnectionPoolMetricsRecording(t *testing.T) {
	m := metrics.New()
	log := logger.NewDefault()

	cfg := &ConnectionPoolConfig{
		MaxIdlePerHost: 5,
		MaxLifetime:    10 * time.Minute,
		Metrics:        m,
	}

	pool := NewConnectionPool(cfg, log)

	// Manually trigger metrics recording
	pool.recordConnectionCreated()
	pool.recordConnectionReused()
	pool.recordConnectionClosed()
	pool.recordHealthCheckFailed()
	pool.updatePoolSize()

	snap := m.Snapshot()
	if snap.PoolConnectionsCreated != 1 {
		t.Errorf("PoolConnectionsCreated = %d, want 1", snap.PoolConnectionsCreated)
	}
	if snap.PoolConnectionsReused != 1 {
		t.Errorf("PoolConnectionsReused = %d, want 1", snap.PoolConnectionsReused)
	}
	if snap.PoolConnectionsClosed != 1 {
		t.Errorf("PoolConnectionsClosed = %d, want 1", snap.PoolConnectionsClosed)
	}
	if snap.PoolHealthCheckFailed != 1 {
		t.Errorf("PoolHealthCheckFailed = %d, want 1", snap.PoolHealthCheckFailed)
	}
}
