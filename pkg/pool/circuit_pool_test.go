package pool

import (
	"context"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/circuit"
	"github.com/opd-ai/go-tor/pkg/logger"
)

// Mock circuit builder for testing
func mockCircuitBuilder(ctx context.Context) (*circuit.Circuit, error) {
	circ := &circuit.Circuit{
		ID: uint32(time.Now().UnixNano() % 65536),
	}
	circ.SetState(circuit.StateOpen)
	return circ, nil
}

func TestCircuitPoolCreation(t *testing.T) {
	log := logger.NewDefault()
	cfg := DefaultCircuitPoolConfig()
	cfg.PrebuildEnabled = false // Disable prebuilding for testing

	pool := NewCircuitPool(cfg, mockCircuitBuilder, log)
	if pool == nil {
		t.Fatal("Expected non-nil circuit pool")
	}

	defer pool.Close()

	stats := pool.Stats()
	if stats.MinCircuits != 2 {
		t.Errorf("Expected min circuits 2, got %d", stats.MinCircuits)
	}
	if stats.MaxCircuits != 10 {
		t.Errorf("Expected max circuits 10, got %d", stats.MaxCircuits)
	}
}

func TestCircuitPoolGetPut(t *testing.T) {
	log := logger.NewDefault()
	cfg := DefaultCircuitPoolConfig()
	cfg.PrebuildEnabled = false

	pool := NewCircuitPool(cfg, mockCircuitBuilder, log)
	defer pool.Close()

	ctx := context.Background()

	// Get a circuit (will build new one)
	circ1, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("Failed to get circuit: %v", err)
	}

	// Return it to pool
	pool.Put(circ1)

	stats := pool.Stats()
	if stats.Total != 1 {
		t.Errorf("Expected 1 circuit in pool, got %d", stats.Total)
	}

	// Get it again (should reuse)
	circ2, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("Failed to get circuit: %v", err)
	}

	if circ2.ID != circ1.ID {
		t.Errorf("Expected to reuse circuit %d, got %d", circ1.ID, circ2.ID)
	}
}

func TestCircuitPoolMaxCapacity(t *testing.T) {
	log := logger.NewDefault()
	cfg := &CircuitPoolConfig{
		MinCircuits:     1,
		MaxCircuits:     3,
		PrebuildEnabled: false,
	}

	pool := NewCircuitPool(cfg, mockCircuitBuilder, log)
	defer pool.Close()

	ctx := context.Background()

	// Build and return circuits up to max capacity
	circuits := make([]*circuit.Circuit, 5)
	for i := 0; i < 5; i++ {
		circ, err := pool.Get(ctx)
		if err != nil {
			t.Fatalf("Failed to get circuit: %v", err)
		}
		circuits[i] = circ
		pool.Put(circ)
	}

	stats := pool.Stats()
	if stats.Total > cfg.MaxCircuits {
		t.Errorf("Expected max %d circuits, got %d", cfg.MaxCircuits, stats.Total)
	}
}

func TestCircuitPoolClosedCircuit(t *testing.T) {
	log := logger.NewDefault()
	cfg := DefaultCircuitPoolConfig()
	cfg.PrebuildEnabled = false

	pool := NewCircuitPool(cfg, mockCircuitBuilder, log)
	defer pool.Close()

	ctx := context.Background()

	// Get a circuit
	circ, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("Failed to get circuit: %v", err)
	}

	// Close it
	circ.SetState(circuit.StateClosed)

	// Try to return it (should be rejected)
	pool.Put(circ)

	stats := pool.Stats()
	if stats.Total != 0 {
		t.Errorf("Expected 0 circuits in pool (closed circuit rejected), got %d", stats.Total)
	}
}

func TestCircuitPoolStats(t *testing.T) {
	log := logger.NewDefault()
	cfg := DefaultCircuitPoolConfig()
	cfg.PrebuildEnabled = false

	pool := NewCircuitPool(cfg, mockCircuitBuilder, log)
	defer pool.Close()

	ctx := context.Background()

	// Add some circuits by getting and immediately returning them
	circuits := make([]*circuit.Circuit, 3)
	for i := 0; i < 3; i++ {
		circ, err := pool.Get(ctx)
		if err != nil {
			t.Fatalf("Failed to get circuit: %v", err)
		}
		circuits[i] = circ
	}

	// Return all circuits to pool
	for _, circ := range circuits {
		pool.Put(circ)
	}

	stats := pool.Stats()
	if stats.Total != 3 {
		t.Errorf("Expected 3 circuits, got %d", stats.Total)
	}
	if stats.Open != 3 {
		t.Errorf("Expected 3 open circuits, got %d", stats.Open)
	}
}

func TestCircuitPoolClose(t *testing.T) {
	log := logger.NewDefault()
	cfg := DefaultCircuitPoolConfig()
	cfg.PrebuildEnabled = false

	pool := NewCircuitPool(cfg, mockCircuitBuilder, log)

	ctx := context.Background()

	// Add some circuits
	for i := 0; i < 2; i++ {
		circ, err := pool.Get(ctx)
		if err != nil {
			t.Fatalf("Failed to get circuit: %v", err)
		}
		pool.Put(circ)
	}

	// Close pool
	if err := pool.Close(); err != nil {
		t.Errorf("Failed to close pool: %v", err)
	}

	stats := pool.Stats()
	if stats.Total != 0 {
		t.Errorf("Expected 0 circuits after close, got %d", stats.Total)
	}
}

func TestCircuitPoolPrebuildDisabled(t *testing.T) {
	log := logger.NewDefault()
	cfg := &CircuitPoolConfig{
		MinCircuits:     5,
		MaxCircuits:     10,
		PrebuildEnabled: false,
		RebuildInterval: 10 * time.Millisecond,
	}

	pool := NewCircuitPool(cfg, mockCircuitBuilder, log)
	defer pool.Close()

	// Wait a bit to ensure prebuild doesn't run
	time.Sleep(50 * time.Millisecond)

	stats := pool.Stats()
	if stats.Total != 0 {
		t.Errorf("Expected 0 circuits (prebuild disabled), got %d", stats.Total)
	}
}
