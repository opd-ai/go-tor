package pool

import (
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/logger"
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
