package circuit_test

import (
	"context"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/circuit"
	"github.com/opd-ai/go-tor/pkg/logger"
	"github.com/opd-ai/go-tor/pkg/pool"
)

// TestCircuitIsolation_Integration tests circuit isolation end-to-end
func TestCircuitIsolation_Integration(t *testing.T) {
	log := logger.NewDefault()

	// Create a simple circuit builder for testing
	circuitID := uint32(1)
	builder := func(ctx context.Context) (*circuit.Circuit, error) {
		circ := circuit.NewCircuit(circuitID)
		circuitID++
		circ.SetState(circuit.StateOpen)
		return circ, nil
	}

	cfg := pool.DefaultCircuitPoolConfig()
	cfg.PrebuildEnabled = false // Disable prebuilding for controlled testing

	circuitPool := pool.NewCircuitPool(cfg, builder, log)
	defer circuitPool.Close()

	t.Run("NoIsolation_SharedCircuits", func(t *testing.T) {
		ctx := context.Background()

		// Get two circuits without isolation - should share from pool
		circ1, err := circuitPool.Get(ctx)
		if err != nil {
			t.Fatalf("Failed to get first circuit: %v", err)
		}

		// Return to pool
		circuitPool.Put(circ1)

		// Get another circuit - should reuse the same one
		circ2, err := circuitPool.Get(ctx)
		if err != nil {
			t.Fatalf("Failed to get second circuit: %v", err)
		}

		if circ1.ID != circ2.ID {
			t.Errorf("Expected circuits to share ID without isolation, got %d and %d", circ1.ID, circ2.ID)
		}
	})

	t.Run("DestinationIsolation_SeparateCircuits", func(t *testing.T) {
		ctx := context.Background()

		// Create isolation keys for different destinations
		key1 := circuit.NewIsolationKey(circuit.IsolationDestination).
			WithDestination("example.com:80")
		key2 := circuit.NewIsolationKey(circuit.IsolationDestination).
			WithDestination("example.org:80")

		// Get circuits for different destinations
		circ1, err := circuitPool.GetWithIsolation(ctx, key1)
		if err != nil {
			t.Fatalf("Failed to get circuit for example.com: %v", err)
		}

		circ2, err := circuitPool.GetWithIsolation(ctx, key2)
		if err != nil {
			t.Fatalf("Failed to get circuit for example.org: %v", err)
		}

		// Should be different circuits
		if circ1.ID == circ2.ID {
			t.Errorf("Expected different circuits for different destinations, got same ID %d", circ1.ID)
		}

		// Verify isolation keys are set
		if circ1.GetIsolationKey() == nil || !circ1.GetIsolationKey().Equals(key1) {
			t.Error("Circuit 1 isolation key not set correctly")
		}
		if circ2.GetIsolationKey() == nil || !circ2.GetIsolationKey().Equals(key2) {
			t.Error("Circuit 2 isolation key not set correctly")
		}

		// Return to pool
		circuitPool.Put(circ1)
		circuitPool.Put(circ2)

		// Get again - should reuse from isolated pools
		circ3, err := circuitPool.GetWithIsolation(ctx, key1)
		if err != nil {
			t.Fatalf("Failed to get circuit for example.com (second time): %v", err)
		}

		if circ3.ID != circ1.ID {
			t.Errorf("Expected to reuse circuit from isolated pool, got %d instead of %d", circ3.ID, circ1.ID)
		}
	})

	t.Run("CredentialIsolation_DifferentUsers", func(t *testing.T) {
		ctx := context.Background()

		// Create isolation keys for different users
		keyUser1 := circuit.NewIsolationKey(circuit.IsolationCredential).
			WithCredentials("alice")
		keyUser2 := circuit.NewIsolationKey(circuit.IsolationCredential).
			WithCredentials("bob")

		// Get circuits for different users
		circAlice, err := circuitPool.GetWithIsolation(ctx, keyUser1)
		if err != nil {
			t.Fatalf("Failed to get circuit for alice: %v", err)
		}

		circBob, err := circuitPool.GetWithIsolation(ctx, keyUser2)
		if err != nil {
			t.Fatalf("Failed to get circuit for bob: %v", err)
		}

		// Should be different circuits
		if circAlice.ID == circBob.ID {
			t.Errorf("Expected different circuits for different users, got same ID %d", circAlice.ID)
		}
	})

	t.Run("PortIsolation_DifferentPorts", func(t *testing.T) {
		ctx := context.Background()

		// Create isolation keys for different source ports
		keyPort1 := circuit.NewIsolationKey(circuit.IsolationPort).
			WithSourcePort(12345)
		keyPort2 := circuit.NewIsolationKey(circuit.IsolationPort).
			WithSourcePort(54321)

		// Get circuits for different ports
		circ1, err := circuitPool.GetWithIsolation(ctx, keyPort1)
		if err != nil {
			t.Fatalf("Failed to get circuit for port 12345: %v", err)
		}

		circ2, err := circuitPool.GetWithIsolation(ctx, keyPort2)
		if err != nil {
			t.Fatalf("Failed to get circuit for port 54321: %v", err)
		}

		// Should be different circuits
		if circ1.ID == circ2.ID {
			t.Errorf("Expected different circuits for different ports, got same ID %d", circ1.ID)
		}
	})

	t.Run("SessionIsolation_DifferentTokens", func(t *testing.T) {
		ctx := context.Background()

		// Create isolation keys for different sessions
		keySession1 := circuit.NewIsolationKey(circuit.IsolationSession).
			WithSessionToken("session-alpha")
		keySession2 := circuit.NewIsolationKey(circuit.IsolationSession).
			WithSessionToken("session-beta")

		// Get circuits for different sessions
		circ1, err := circuitPool.GetWithIsolation(ctx, keySession1)
		if err != nil {
			t.Fatalf("Failed to get circuit for session alpha: %v", err)
		}

		circ2, err := circuitPool.GetWithIsolation(ctx, keySession2)
		if err != nil {
			t.Fatalf("Failed to get circuit for session beta: %v", err)
		}

		// Should be different circuits
		if circ1.ID == circ2.ID {
			t.Errorf("Expected different circuits for different sessions, got same ID %d", circ1.ID)
		}
	})

	t.Run("PoolStats_IsolatedCircuits", func(t *testing.T) {
		ctx := context.Background()

		// Create multiple isolated circuits
		keys := []*circuit.IsolationKey{
			circuit.NewIsolationKey(circuit.IsolationDestination).WithDestination("site1.com:80"),
			circuit.NewIsolationKey(circuit.IsolationDestination).WithDestination("site2.com:80"),
			circuit.NewIsolationKey(circuit.IsolationCredential).WithCredentials("user1"),
			circuit.NewIsolationKey(circuit.IsolationPort).WithSourcePort(9999),
		}

		circuits := make([]*circuit.Circuit, len(keys))
		for i, key := range keys {
			circ, err := circuitPool.GetWithIsolation(ctx, key)
			if err != nil {
				t.Fatalf("Failed to get isolated circuit %d: %v", i, err)
			}
			circuits[i] = circ
		}

		// Return all to pool
		for _, circ := range circuits {
			circuitPool.Put(circ)
		}

		// Check stats
		stats := circuitPool.Stats()
		if stats.IsolatedPools < 4 {
			t.Errorf("Expected at least 4 isolated pools, got %d", stats.IsolatedPools)
		}
		if stats.IsolatedCircuits < 4 {
			t.Errorf("Expected at least 4 isolated circuits, got %d", stats.IsolatedCircuits)
		}
	})
}

// TestCircuitIsolation_PoolCapacity tests that isolation respects pool limits
func TestCircuitIsolation_PoolCapacity(t *testing.T) {
	log := logger.NewDefault()

	circuitID := uint32(1)
	builder := func(ctx context.Context) (*circuit.Circuit, error) {
		circ := circuit.NewCircuit(circuitID)
		circuitID++
		circ.SetState(circuit.StateOpen)
		return circ, nil
	}

	cfg := &pool.CircuitPoolConfig{
		MinCircuits:     0,
		MaxCircuits:     2, // Small limit for testing
		PrebuildEnabled: false,
		RebuildInterval: 30 * time.Second,
	}

	circuitPool := pool.NewCircuitPool(cfg, builder, log)
	defer circuitPool.Close()

	ctx := context.Background()
	key := circuit.NewIsolationKey(circuit.IsolationDestination).
		WithDestination("example.com:80")

	// Get circuits up to limit
	circ1, _ := circuitPool.GetWithIsolation(ctx, key)
	circ2, _ := circuitPool.GetWithIsolation(ctx, key)
	circ3, _ := circuitPool.GetWithIsolation(ctx, key)

	// Return all to pool
	circuitPool.Put(circ1)
	circuitPool.Put(circ2)
	circuitPool.Put(circ3) // This should not be added due to capacity

	stats := circuitPool.Stats()
	if stats.IsolatedCircuits > 2 {
		t.Errorf("Pool exceeded max capacity: expected <= 2, got %d", stats.IsolatedCircuits)
	}
}

// TestCircuitIsolation_ClosedCircuits tests that closed circuits are not returned
func TestCircuitIsolation_ClosedCircuits(t *testing.T) {
	log := logger.NewDefault()

	circuitID := uint32(1)
	builder := func(ctx context.Context) (*circuit.Circuit, error) {
		circ := circuit.NewCircuit(circuitID)
		circuitID++
		circ.SetState(circuit.StateOpen)
		return circ, nil
	}

	cfg := pool.DefaultCircuitPoolConfig()
	cfg.PrebuildEnabled = false

	circuitPool := pool.NewCircuitPool(cfg, builder, log)
	defer circuitPool.Close()

	ctx := context.Background()
	key := circuit.NewIsolationKey(circuit.IsolationDestination).
		WithDestination("example.com:80")

	// Get a circuit
	circ1, err := circuitPool.GetWithIsolation(ctx, key)
	if err != nil {
		t.Fatalf("Failed to get circuit: %v", err)
	}

	// Close it and return to pool
	circ1.SetState(circuit.StateClosed)
	circuitPool.Put(circ1)

	// Get another circuit - should build new one, not return closed
	circ2, err := circuitPool.GetWithIsolation(ctx, key)
	if err != nil {
		t.Fatalf("Failed to get second circuit: %v", err)
	}

	if circ1.ID == circ2.ID {
		t.Error("Pool returned a closed circuit")
	}

	if circ2.GetState() != circuit.StateOpen {
		t.Errorf("New circuit should be open, got %s", circ2.GetState())
	}
}
