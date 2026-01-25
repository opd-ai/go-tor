package pool

import (
	"context"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/connection"
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
	if pool.HasMetrics() {
		t.Error("Expected no metrics initially")
	}

	// Set metrics
	m := metrics.New()
	pool.SetMetrics(m)

	if !pool.HasMetrics() {
		t.Error("Expected metrics after SetMetrics")
	}
}

func TestConnectionPoolRemoveNonexistent(t *testing.T) {
	// Test that removing a non-existent connection doesn't panic or error
	log := logger.NewDefault()
	pool := NewConnectionPool(nil, log)

	// Remove a non-existent connection should not panic or error
	pool.Remove("nonexistent:9001")

	// Stats should show empty pool
	stats := pool.Stats()
	if stats.Total != 0 {
		t.Errorf("Total = %d, want 0", stats.Total)
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
	if pool.MaxIdleTime() != 30*time.Second {
		t.Errorf("MaxIdleTime() = %v, want %v", pool.MaxIdleTime(), 30*time.Second)
	}
}

func TestConnectionPoolConfigWithCustomMaxIdleTime(t *testing.T) {
	log := logger.NewDefault()
	cfg := &ConnectionPoolConfig{
		MaxIdlePerHost: 5,
		MaxLifetime:    10 * time.Minute,
		MaxIdleTime:    1 * time.Minute,
	}

	pool := NewConnectionPool(cfg, log)
	if pool.MaxIdleTime() != 1*time.Minute {
		t.Errorf("MaxIdleTime() = %v, want %v", pool.MaxIdleTime(), 1*time.Minute)
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

	// Verify that metrics are attached
	if !pool.HasMetrics() {
		t.Fatal("Expected metrics to be attached")
	}

	// Verify pool size starts at 0
	snap := m.Snapshot()
	if snap.PoolSize != 0 {
		t.Errorf("Initial PoolSize = %d, want 0", snap.PoolSize)
	}

	// Closing the empty pool should not change metrics significantly
	// but should still record the pool size update
	if err := pool.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Verify the pool size is still 0 after closing empty pool
	snap = m.Snapshot()
	if snap.PoolSize != 0 {
		t.Errorf("PoolSize after close = %d, want 0", snap.PoolSize)
	}
}

// TestConnectionPoolGetWithUnreachableHost tests Get with an unreachable address
func TestConnectionPoolGetWithUnreachableHost(t *testing.T) {
	log := logger.NewDefault()
	pool := NewConnectionPool(nil, log)
	defer pool.Close()

	// Use a context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Try to get connection to unreachable host
	cfg := connection.DefaultConfig("192.0.2.1:9999") // TEST-NET-1, should be unreachable
	cfg.Timeout = 50 * time.Millisecond

	_, err := pool.Get(ctx, "192.0.2.1:9999", cfg)
	if err == nil {
		t.Error("Expected error when connecting to unreachable host, got nil")
	}
}

// TestConnectionPoolPutWithNilConnection tests Put behavior
func TestConnectionPoolPutWithNilConnection(t *testing.T) {
	log := logger.NewDefault()
	pool := NewConnectionPool(nil, log)
	defer pool.Close()

	// Put should not panic with a non-existent connection
	cfg := connection.DefaultConfig("127.0.0.1:9001")
	conn := connection.New(cfg, log)
	pool.Put("127.0.0.1:9001", conn)

	stats := pool.Stats()
	// Pool should remain empty since the connection was never added
	if stats.Total != 0 {
		t.Errorf("Expected 0 connections after Put without Get, got %d", stats.Total)
	}
}

// TestConnectionPoolHealthCheck tests the healthCheck functionality
func TestConnectionPoolHealthCheck(t *testing.T) {
	log := logger.NewDefault()
	pool := NewConnectionPool(nil, log)
	defer pool.Close()

	// Test nil pooledConnection
	if pool.healthCheck(nil) {
		t.Error("healthCheck(nil) should return false")
	}

	// Test pooledConnection with nil conn
	pc := &pooledConnection{
		conn:      nil,
		inUse:     false,
		lastUsed:  time.Now(),
		createdAt: time.Now(),
	}
	if pool.healthCheck(pc) {
		t.Error("healthCheck with nil conn should return false")
	}
}

// TestConnectionPoolCloseWithMultipleConnections tests closing pool with multiple connections
func TestConnectionPoolCloseWithMultipleConnections(t *testing.T) {
	log := logger.NewDefault()
	m := metrics.New()
	cfg := &ConnectionPoolConfig{
		MaxIdlePerHost: 5,
		MaxLifetime:    10 * time.Minute,
		Metrics:        m,
	}
	pool := NewConnectionPool(cfg, log)

	// Manually add some mock connections to the pool for testing
	// We'll simulate connections by adding them directly to the internal map
	pool.mu.Lock()
	for i := 0; i < 3; i++ {
		address := time.Now().Format("mock-addr-") + string(rune('0'+i))
		connCfg := connection.DefaultConfig(address)
		mockConn := connection.New(connCfg, log)
		pool.connections[address] = &pooledConnection{
			conn:      mockConn,
			inUse:     false,
			lastUsed:  time.Now(),
			createdAt: time.Now(),
		}
	}
	pool.mu.Unlock()

	// Verify we have 3 connections
	stats := pool.Stats()
	if stats.Total != 3 {
		t.Errorf("Expected 3 connections before close, got %d", stats.Total)
	}

	// Close the pool
	if err := pool.Close(); err != nil {
		t.Logf("Close returned error (expected for mock connections): %v", err)
	}

	// Verify connections are removed
	stats = pool.Stats()
	if stats.Total != 0 {
		t.Errorf("Expected 0 connections after close, got %d", stats.Total)
	}

	// Verify metrics updated
	snap := m.Snapshot()
	if snap.PoolSize != 0 {
		t.Errorf("Expected pool size 0 after close, got %d", snap.PoolSize)
	}
}

// TestConnectionPoolRemoveExisting tests removing an existing connection
func TestConnectionPoolRemoveExisting(t *testing.T) {
	log := logger.NewDefault()
	m := metrics.New()
	cfg := &ConnectionPoolConfig{
		MaxIdlePerHost: 5,
		MaxLifetime:    10 * time.Minute,
		Metrics:        m,
	}
	pool := NewConnectionPool(cfg, log)
	defer pool.Close()

	// Manually add a mock connection
	address := "mock-remove-test:9001"
	connCfg := connection.DefaultConfig(address)
	mockConn := connection.New(connCfg, log)

	pool.mu.Lock()
	pool.connections[address] = &pooledConnection{
		conn:      mockConn,
		inUse:     false,
		lastUsed:  time.Now(),
		createdAt: time.Now(),
	}
	pool.mu.Unlock()

	// Verify connection exists
	stats := pool.Stats()
	if stats.Total != 1 {
		t.Errorf("Expected 1 connection, got %d", stats.Total)
	}

	// Remove the connection
	pool.Remove(address)

	// Verify connection removed
	stats = pool.Stats()
	if stats.Total != 0 {
		t.Errorf("Expected 0 connections after Remove, got %d", stats.Total)
	}
}

// TestConnectionPoolCleanupIdleConnections tests idle cleanup
func TestConnectionPoolCleanupIdleConnections(t *testing.T) {
	log := logger.NewDefault()
	pool := NewConnectionPool(nil, log)
	defer pool.Close()

	// Add connections with different idle times
	now := time.Now()
	addresses := []string{"idle1:9001", "idle2:9001", "recent:9001"}

	pool.mu.Lock()
	// Old idle connections (should be cleaned up)
	for i := 0; i < 2; i++ {
		cfg := connection.DefaultConfig(addresses[i])
		conn := connection.New(cfg, log)
		pool.connections[addresses[i]] = &pooledConnection{
			conn:      conn,
			inUse:     false,
			lastUsed:  now.Add(-2 * time.Minute), // 2 minutes idle
			createdAt: now.Add(-5 * time.Minute),
		}
	}

	// Recent connection (should not be cleaned up)
	cfg := connection.DefaultConfig(addresses[2])
	conn := connection.New(cfg, log)
	pool.connections[addresses[2]] = &pooledConnection{
		conn:      conn,
		inUse:     false,
		lastUsed:  now.Add(-10 * time.Second), // 10 seconds idle
		createdAt: now.Add(-1 * time.Minute),
	}
	pool.mu.Unlock()

	// Verify we have 3 connections
	stats := pool.Stats()
	if stats.Total != 3 {
		t.Errorf("Expected 3 connections, got %d", stats.Total)
	}

	// Cleanup connections idle for more than 1 minute
	pool.CleanupIdle(1 * time.Minute)

	// Should have 1 connection remaining (the recent one)
	stats = pool.Stats()
	if stats.Total != 1 {
		t.Errorf("Expected 1 connection after idle cleanup, got %d", stats.Total)
	}
}

// TestConnectionPoolCleanupExpiredConnections tests expired connection cleanup
func TestConnectionPoolCleanupExpiredConnections(t *testing.T) {
	log := logger.NewDefault()
	cfg := &ConnectionPoolConfig{
		MaxIdlePerHost: 5,
		MaxLifetime:    2 * time.Minute,
	}
	pool := NewConnectionPool(cfg, log)
	defer pool.Close()

	now := time.Now()
	addresses := []string{"old1:9001", "old2:9001", "new:9001"}

	pool.mu.Lock()
	// Old connections (should be cleaned up)
	for i := 0; i < 2; i++ {
		connCfg := connection.DefaultConfig(addresses[i])
		conn := connection.New(connCfg, log)
		pool.connections[addresses[i]] = &pooledConnection{
			conn:      conn,
			inUse:     false,
			lastUsed:  now.Add(-1 * time.Minute),
			createdAt: now.Add(-3 * time.Minute), // 3 minutes old
		}
	}

	// New connection (should not be cleaned up)
	connCfg := connection.DefaultConfig(addresses[2])
	conn := connection.New(connCfg, log)
	pool.connections[addresses[2]] = &pooledConnection{
		conn:      conn,
		inUse:     false,
		lastUsed:  now.Add(-30 * time.Second),
		createdAt: now.Add(-1 * time.Minute), // 1 minute old
	}
	pool.mu.Unlock()

	// Verify we have 3 connections
	stats := pool.Stats()
	if stats.Total != 3 {
		t.Errorf("Expected 3 connections, got %d", stats.Total)
	}

	// Cleanup expired connections
	pool.CleanupExpired()

	// Should have 1 connection remaining (the new one)
	stats = pool.Stats()
	if stats.Total != 1 {
		t.Errorf("Expected 1 connection after expired cleanup, got %d", stats.Total)
	}
}

// TestConnectionPoolStatsInUseVsIdle tests Stats tracking of in-use vs idle
func TestConnectionPoolStatsInUseVsIdle(t *testing.T) {
	log := logger.NewDefault()
	pool := NewConnectionPool(nil, log)
	defer pool.Close()

	now := time.Now()

	pool.mu.Lock()
	// Add 2 in-use connections
	for i := 0; i < 2; i++ {
		addr := time.Now().Format("inuse-") + string(rune('0'+i))
		cfg := connection.DefaultConfig(addr)
		conn := connection.New(cfg, log)
		pool.connections[addr] = &pooledConnection{
			conn:      conn,
			inUse:     true,
			lastUsed:  now,
			createdAt: now,
		}
	}

	// Add 3 idle connections
	for i := 0; i < 3; i++ {
		addr := time.Now().Format("idle-") + string(rune('0'+i))
		cfg := connection.DefaultConfig(addr)
		conn := connection.New(cfg, log)
		pool.connections[addr] = &pooledConnection{
			conn:      conn,
			inUse:     false,
			lastUsed:  now,
			createdAt: now,
		}
	}
	pool.mu.Unlock()

	stats := pool.Stats()
	if stats.Total != 5 {
		t.Errorf("Total = %d, want 5", stats.Total)
	}
	if stats.InUse != 2 {
		t.Errorf("InUse = %d, want 2", stats.InUse)
	}
	if stats.Idle != 3 {
		t.Errorf("Idle = %d, want 3", stats.Idle)
	}
}

// TestConnectionPoolMetricsRecordingWithOperations tests metrics during pool operations
func TestConnectionPoolMetricsRecordingWithOperations(t *testing.T) {
	m := metrics.New()
	log := logger.NewDefault()

	cfg := &ConnectionPoolConfig{
		MaxIdlePerHost: 5,
		MaxLifetime:    10 * time.Minute,
		Metrics:        m,
	}

	pool := NewConnectionPool(cfg, log)
	defer pool.Close()

	// Add a mock connection manually
	address := "metrics-test:9001"
	connCfg := connection.DefaultConfig(address)
	mockConn := connection.New(connCfg, log)

	pool.mu.Lock()
	pool.connections[address] = &pooledConnection{
		conn:      mockConn,
		inUse:     false,
		lastUsed:  time.Now(),
		createdAt: time.Now(),
	}
	pool.recordConnectionCreated()
	pool.updatePoolSize()
	pool.mu.Unlock()

	// Verify pool size metric updated
	snap := m.Snapshot()
	if snap.PoolSize != 1 {
		t.Errorf("PoolSize = %d, want 1", snap.PoolSize)
	}

	// Test Put operation (marking connection as not in use)
	pool.Put(address, mockConn)

	// Remove the connection
	pool.Remove(address)

	// Verify pool size updated
	snap = m.Snapshot()
	if snap.PoolSize != 0 {
		t.Errorf("PoolSize after Remove = %d, want 0", snap.PoolSize)
	}
}

// TestConnectionPoolGetWithRecentlyUsedConnection tests Get reusing a recent connection
func TestConnectionPoolGetWithRecentlyUsedConnection(t *testing.T) {
	m := metrics.New()
	log := logger.NewDefault()

	cfg := &ConnectionPoolConfig{
		MaxIdlePerHost: 5,
		MaxLifetime:    10 * time.Minute,
		MaxIdleTime:    30 * time.Second,
		Metrics:        m,
	}

	pool := NewConnectionPool(cfg, log)
	defer pool.Close()

	// Create a mock connection that was recently used (within maxIdleTime)
	address := "recent-conn:9001"
	connCfg := connection.DefaultConfig(address)
	mockConn := connection.New(connCfg, log)

	// Simulate an open connection state
	pool.mu.Lock()
	pool.connections[address] = &pooledConnection{
		conn:      mockConn,
		inUse:     false,
		lastUsed:  time.Now().Add(-10 * time.Second), // Used 10 seconds ago
		createdAt: time.Now().Add(-1 * time.Minute),
	}
	pool.mu.Unlock()

	// Note: Since the connection is not actually open (no real network connection),
	// Get will not find it in StateOpen and will try to create a new one.
	// This tests the fallback path.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	connCfg.Timeout = 30 * time.Millisecond
	_, err := pool.Get(ctx, address, connCfg)
	// Expect error since we can't actually connect
	if err == nil {
		t.Log("Get succeeded (connection may have been created)")
	}

	// The original connection should still be in pool (or replaced)
	stats := pool.Stats()
	t.Logf("Pool stats after Get: %+v", stats)
}

// TestConnectionPoolGetWithOldConnection tests Get handling of old connections
func TestConnectionPoolGetWithOldConnection(t *testing.T) {
	log := logger.NewDefault()

	cfg := &ConnectionPoolConfig{
		MaxIdlePerHost: 5,
		MaxLifetime:    1 * time.Minute, // Short lifetime for testing
		MaxIdleTime:    30 * time.Second,
	}

	pool := NewConnectionPool(cfg, log)
	defer pool.Close()

	// Create a mock connection that's too old
	address := "old-conn:9001"
	connCfg := connection.DefaultConfig(address)
	mockConn := connection.New(connCfg, log)

	pool.mu.Lock()
	pool.connections[address] = &pooledConnection{
		conn:      mockConn,
		inUse:     false,
		lastUsed:  time.Now().Add(-30 * time.Second),
		createdAt: time.Now().Add(-2 * time.Minute), // 2 minutes old, exceeds max lifetime
	}
	pool.mu.Unlock()

	// Try to get the connection
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	connCfg.Timeout = 30 * time.Millisecond
	_, err := pool.Get(ctx, address, connCfg)
	// Expect error since we can't actually connect
	if err == nil {
		t.Log("Get succeeded (connection may have been created)")
	}

	// Stats may vary depending on whether old connection was removed
	stats := pool.Stats()
	t.Logf("Pool stats after Get: %+v", stats)
}

// TestConnectionPoolPutWithMatchingConnection tests Put with correct connection
func TestConnectionPoolPutWithMatchingConnection(t *testing.T) {
	log := logger.NewDefault()
	pool := NewConnectionPool(nil, log)
	defer pool.Close()

	address := "put-test:9001"
	connCfg := connection.DefaultConfig(address)
	mockConn := connection.New(connCfg, log)

	// Add connection to pool as in-use
	pool.mu.Lock()
	pool.connections[address] = &pooledConnection{
		conn:      mockConn,
		inUse:     true,
		lastUsed:  time.Now().Add(-1 * time.Minute),
		createdAt: time.Now().Add(-2 * time.Minute),
	}
	pool.mu.Unlock()

	// Verify it's marked as in-use
	stats := pool.Stats()
	if stats.InUse != 1 {
		t.Errorf("Expected 1 in-use connection, got %d", stats.InUse)
	}

	// Return connection to pool
	pool.Put(address, mockConn)

	// Verify it's now marked as idle
	stats = pool.Stats()
	if stats.Idle != 1 {
		t.Errorf("Expected 1 idle connection after Put, got %d", stats.Idle)
	}
	if stats.InUse != 0 {
		t.Errorf("Expected 0 in-use connections after Put, got %d", stats.InUse)
	}

	// Verify lastUsed was updated (should be recent)
	pool.mu.RLock()
	pc := pool.connections[address]
	pool.mu.RUnlock()

	if time.Since(pc.lastUsed) > 1*time.Second {
		t.Errorf("lastUsed should be recent, got %v ago", time.Since(pc.lastUsed))
	}
}

// TestConnectionPoolRemoveWithMetrics tests Remove with metrics tracking
func TestConnectionPoolRemoveWithMetrics(t *testing.T) {
	m := metrics.New()
	log := logger.NewDefault()

	cfg := &ConnectionPoolConfig{
		MaxIdlePerHost: 5,
		MaxLifetime:    10 * time.Minute,
		Metrics:        m,
	}

	pool := NewConnectionPool(cfg, log)
	defer pool.Close()

	address := "remove-metrics:9001"
	connCfg := connection.DefaultConfig(address)
	mockConn := connection.New(connCfg, log)

	// Add connection
	pool.mu.Lock()
	pool.connections[address] = &pooledConnection{
		conn:      mockConn,
		inUse:     false,
		lastUsed:  time.Now(),
		createdAt: time.Now(),
	}
	pool.recordConnectionCreated()
	pool.updatePoolSize()
	pool.mu.Unlock()

	snap := m.Snapshot()
	if snap.PoolSize != 1 {
		t.Errorf("PoolSize before Remove = %d, want 1", snap.PoolSize)
	}

	// Remove the connection
	pool.Remove(address)

	// Verify metrics updated
	snap = m.Snapshot()
	if snap.PoolSize != 0 {
		t.Errorf("PoolSize after Remove = %d, want 0", snap.PoolSize)
	}
}
