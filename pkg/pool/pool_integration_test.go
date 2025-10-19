package pool

import (
	"context"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/circuit"
	"github.com/opd-ai/go-tor/pkg/logger"
)

// TestBufferPoolGetIntegration tests getting and putting buffers
func TestBufferPoolGetIntegration(t *testing.T) {
	pool := NewBufferPool(512)

	// Get a buffer
	buf := pool.Get()
	if buf == nil {
		t.Fatal("Expected non-nil buffer")
	}

	if len(buf) != 512 {
		t.Errorf("Expected buffer length 512, got %d", len(buf))
	}

	// Put the buffer back
	pool.Put(buf)

	// Get it again - should be reused
	buf2 := pool.Get()
	if buf2 == nil {
		t.Fatal("Expected non-nil buffer on second get")
	}
}

// TestBufferPoolMultipleBuffers tests getting multiple buffers
func TestBufferPoolMultipleBuffers(t *testing.T) {
	pool := NewBufferPool(256)

	buffers := make([][]byte, 10)
	for i := 0; i < 10; i++ {
		buffers[i] = pool.Get()
		if buffers[i] == nil {
			t.Fatalf("Expected non-nil buffer at index %d", i)
		}
	}

	// Put them all back
	for _, buf := range buffers {
		pool.Put(buf)
	}

	// Get them again
	for i := 0; i < 10; i++ {
		buf := pool.Get()
		if buf == nil {
			t.Fatalf("Expected non-nil buffer on reuse at index %d", i)
		}
	}
}

// TestBufferPoolPutSmallBuffer tests putting smaller buffer
func TestBufferPoolPutSmallBuffer(t *testing.T) {
	pool := NewBufferPool(1024)

	// Create a small buffer
	smallBuf := make([]byte, 512)

	// Should not panic (won't be pooled but that's ok)
	pool.Put(smallBuf)

	// Get should still work
	buf := pool.Get()
	if buf == nil {
		t.Fatal("Expected non-nil buffer after putting small buffer")
	}
}

// TestPreConfiguredBufferPools tests pre-configured buffer pools
func TestPreConfiguredBufferPools(t *testing.T) {
	// Test CellBufferPool
	cellBuf := CellBufferPool.Get()
	if len(cellBuf) != 514 {
		t.Errorf("Expected CellBufferPool buffer length 514, got %d", len(cellBuf))
	}
	CellBufferPool.Put(cellBuf)

	// Test PayloadBufferPool
	payloadBuf := PayloadBufferPool.Get()
	if len(payloadBuf) != 509 {
		t.Errorf("Expected PayloadBufferPool buffer length 509, got %d", len(payloadBuf))
	}
	PayloadBufferPool.Put(payloadBuf)

	// Test CryptoBufferPool
	cryptoBuf := CryptoBufferPool.Get()
	if len(cryptoBuf) != 1024 {
		t.Errorf("Expected CryptoBufferPool buffer length 1024, got %d", len(cryptoBuf))
	}
	CryptoBufferPool.Put(cryptoBuf)

	// Test LargeCryptoBufferPool
	largeBuf := LargeCryptoBufferPool.Get()
	if len(largeBuf) != 8192 {
		t.Errorf("Expected LargeCryptoBufferPool buffer length 8192, got %d", len(largeBuf))
	}
	LargeCryptoBufferPool.Put(largeBuf)
}

// TestCircuitPoolPrebuilding tests circuit prebuilding functionality
func TestCircuitPoolPrebuilding(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping circuit pool integration test in short mode")
	}

	log := logger.NewDefault()

	// Mock circuit builder that creates circuits
	buildCount := 0
	builder := func(ctx context.Context) (*circuit.Circuit, error) {
		buildCount++
		time.Sleep(10 * time.Millisecond) // Simulate build time
		circ := &circuit.Circuit{
			ID: uint32(buildCount),
		}
		circ.SetState(circuit.StateOpen)
		return circ, nil
	}

	cfg := &CircuitPoolConfig{
		MinCircuits:     3,
		MaxCircuits:     5,
		PrebuildEnabled: true,
		RebuildInterval: 100 * time.Millisecond,
	}

	pool := NewCircuitPool(cfg, builder, log)
	if pool == nil {
		t.Fatal("Expected non-nil circuit pool")
	}
	defer pool.Close()

	// Wait for circuits to be built
	time.Sleep(400 * time.Millisecond)

	// Check stats
	stats := pool.Stats()
	if stats.Open < 1 {
		t.Errorf("Expected at least 1 open circuit, got %d", stats.Open)
	}
}

// TestCircuitPoolGetAndPut tests getting and putting circuits
func TestCircuitPoolGetAndPut(t *testing.T) {
	log := logger.NewDefault()

	builder := func(ctx context.Context) (*circuit.Circuit, error) {
		circ := &circuit.Circuit{
			ID: 1,
		}
		circ.SetState(circuit.StateOpen)
		return circ, nil
	}

	pool := NewCircuitPool(nil, builder, log)
	defer pool.Close()

	ctx := context.Background()

	// Get a circuit
	circ, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("Failed to get circuit: %v", err)
	}
	if circ == nil {
		t.Fatal("Expected non-nil circuit")
	}

	// Put the circuit back
	pool.Put(circ)

	// Stats should show it's back
	stats := pool.Stats()
	if stats.Open != 1 {
		t.Errorf("Expected 1 open circuit after put, got %d", stats.Open)
	}
}

// TestCircuitPoolIntegrationClose tests closing the pool
func TestCircuitPoolIntegrationClose(t *testing.T) {
	log := logger.NewDefault()

	builder := func(ctx context.Context) (*circuit.Circuit, error) {
		circ := &circuit.Circuit{ID: 1}
		circ.SetState(circuit.StateOpen)
		return circ, nil
	}

	pool := NewCircuitPool(nil, builder, log)

	// Add a circuit
	ctx := context.Background()
	circ, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("Failed to get circuit: %v", err)
	}
	pool.Put(circ)

	// Close the pool
	if err := pool.Close(); err != nil {
		t.Errorf("Failed to close pool: %v", err)
	}

	// Stats should show no circuits
	stats := pool.Stats()
	if stats.Total != 0 {
		t.Errorf("Expected 0 circuits after close, got %d", stats.Total)
	}
}

// TestCircuitPoolStatsAccuracy tests that stats are accurate
func TestCircuitPoolStatsAccuracy(t *testing.T) {
	log := logger.NewDefault()

	circuitCount := 0
	builder := func(ctx context.Context) (*circuit.Circuit, error) {
		circuitCount++
		circ := &circuit.Circuit{
			ID: uint32(circuitCount),
		}
		circ.SetState(circuit.StateOpen)
		return circ, nil
	}

	cfg := &CircuitPoolConfig{
		MinCircuits:     0,
		MaxCircuits:     10,
		PrebuildEnabled: false,
	}

	pool := NewCircuitPool(cfg, builder, log)
	defer pool.Close()

	ctx := context.Background()

	// Initially empty
	stats := pool.Stats()
	if stats.Total != 0 || stats.Open != 0 {
		t.Errorf("Expected empty pool stats, got %+v", stats)
	}

	// Get 3 circuits and put them back immediately
	circs := make([]*circuit.Circuit, 3)
	for i := 0; i < 3; i++ {
		var err error
		circs[i], err = pool.Get(ctx)
		if err != nil {
			t.Fatalf("Failed to get circuit %d: %v", i, err)
		}
	}

	// Put them back to the pool
	for _, circ := range circs {
		pool.Put(circ)
	}

	// Check stats - should show 3 circuits in pool
	stats = pool.Stats()
	if stats.Open != 3 {
		t.Errorf("Expected 3 open circuits, got %d", stats.Open)
	}
	if stats.Total != 3 {
		t.Errorf("Expected 3 total circuits, got %d", stats.Total)
	}
}

// TestConnectionPoolDefaultConfig tests default configuration
func TestConnectionPoolDefaultConfig(t *testing.T) {
	cfg := DefaultConnectionPoolConfig()

	if cfg == nil {
		t.Fatal("Expected non-nil default config")
	}

	if cfg.MaxIdlePerHost <= 0 {
		t.Errorf("Expected positive MaxIdlePerHost, got %d", cfg.MaxIdlePerHost)
	}

	if cfg.MaxLifetime <= 0 {
		t.Errorf("Expected positive MaxLifetime, got %v", cfg.MaxLifetime)
	}
}

// TestConnectionPoolNilLogger tests pool creation with nil logger
func TestConnectionPoolNilLogger(t *testing.T) {
	pool := NewConnectionPool(nil, nil)

	if pool == nil {
		t.Fatal("Expected non-nil pool with nil logger")
	}

	if pool.logger == nil {
		t.Error("Expected default logger to be initialized")
	}
}

// TestConnectionPoolRemoveNonExistent tests removing non-existent connection
func TestConnectionPoolRemoveNonExistent(t *testing.T) {
	log := logger.NewDefault()
	pool := NewConnectionPool(nil, log)
	defer pool.Close()

	// Should not panic when removing non-existent connection
	pool.Remove("192.168.1.1:9001")

	stats := pool.Stats()
	if stats.Total != 0 {
		t.Errorf("Expected 0 connections, got %d", stats.Total)
	}
}

// TestConnectionPoolCleanupExpiredEmpty tests cleanup on empty pool
func TestConnectionPoolCleanupExpiredEmpty(t *testing.T) {
	log := logger.NewDefault()
	pool := NewConnectionPool(nil, log)
	defer pool.Close()

	// Should not panic on empty pool
	pool.CleanupExpired()

	stats := pool.Stats()
	if stats.Total != 0 {
		t.Errorf("Expected 0 connections after cleanup, got %d", stats.Total)
	}
}

// TestCircuitPoolPutClosedCircuit tests putting a closed circuit
func TestCircuitPoolPutClosedCircuit(t *testing.T) {
	log := logger.NewDefault()

	builder := func(ctx context.Context) (*circuit.Circuit, error) {
		circ := &circuit.Circuit{ID: 1}
		circ.SetState(circuit.StateOpen)
		return circ, nil
	}

	cfg := &CircuitPoolConfig{
		MinCircuits:     0,
		MaxCircuits:     5,
		PrebuildEnabled: false,
	}

	pool := NewCircuitPool(cfg, builder, log)
	defer pool.Close()

	// Get a circuit
	ctx := context.Background()
	circ, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("Failed to get circuit: %v", err)
	}

	// Close the circuit
	circ.SetState(circuit.StateClosed)

	// Try to put it back - should not be added
	pool.Put(circ)

	// Pool should be empty
	stats := pool.Stats()
	if stats.Total != 0 {
		t.Errorf("Expected 0 circuits (closed circuit not added), got %d", stats.Total)
	}
}
