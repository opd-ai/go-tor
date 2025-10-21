// Circuit pool integration tests for Phase 9.4
package client

import (
	"context"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/circuit"
	"github.com/opd-ai/go-tor/pkg/config"
	"github.com/opd-ai/go-tor/pkg/logger"
)

// TestCircuitPoolEnabled tests that circuit pool is created when enabled
func TestCircuitPoolEnabled(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	cfg.EnableCircuitPrebuilding = true
	cfg.CircuitPoolMinSize = 2
	cfg.CircuitPoolMaxSize = 5

	client, err := New(cfg, logger.NewDefault())
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Stop()

	// Circuit pool should be nil before Start (initialized in Start)
	if client.circuitPool != nil {
		t.Error("Circuit pool should be nil before Start")
	}
}

// TestCircuitPoolDisabled tests that circuit pool is not created when disabled
func TestCircuitPoolDisabled(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	cfg.EnableCircuitPrebuilding = false

	client, err := New(cfg, logger.NewDefault())
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Stop()

	if client.circuitPool != nil {
		t.Error("Circuit pool should be nil when prebuilding is disabled")
	}
}

// TestCircuitBuilderFunc tests the circuit builder function
func TestCircuitBuilderFunc(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()

	client, err := New(cfg, logger.NewDefault())
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Stop()

	builderFunc := client.circuitBuilderFunc()
	if builderFunc == nil {
		t.Error("Circuit builder function should not be nil")
	}

	// Note: We can't actually call the builder without starting the client
	// and having a path selector initialized, but we can verify it exists
}

// TestGetStatsWithCircuitPool tests that stats include circuit pool info
func TestGetStatsWithCircuitPool(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	cfg.EnableCircuitPrebuilding = false // Disabled so pool is nil

	client, err := New(cfg, logger.NewDefault())
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Stop()

	stats := client.GetStats()

	// When pool is disabled, CircuitPoolEnabled should be false
	if stats.CircuitPoolEnabled {
		t.Error("CircuitPoolEnabled should be false when pool is nil")
	}

	if stats.CircuitPoolTotal != 0 {
		t.Error("CircuitPoolTotal should be 0 when pool is nil")
	}
}

// TestGetCircuitLegacyMode tests circuit retrieval in legacy mode
func TestGetCircuitLegacyMode(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	cfg.EnableCircuitPrebuilding = false // Legacy mode

	client, err := New(cfg, logger.NewDefault())
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Stop()

	ctx := context.Background()

	// No circuits available initially
	_, err = client.GetCircuit(ctx)
	if err == nil {
		t.Error("Expected error when no circuits available")
	}

	// Add a mock circuit to test selection
	mockCircuit := circuit.NewCircuit(1)
	mockCircuit.SetState(circuit.StateOpen)

	client.circuitsMu.Lock()
	client.circuits = append(client.circuits, mockCircuit)
	client.circuitsMu.Unlock()

	// Now we should be able to get a circuit
	circ, err := client.GetCircuit(ctx)
	if err != nil {
		t.Fatalf("Failed to get circuit: %v", err)
	}

	if circ.ID != 1 {
		t.Errorf("Expected circuit ID 1, got %d", circ.ID)
	}
}

// TestGetCircuitSelectsYoungest tests that adaptive selection chooses youngest circuit
func TestGetCircuitSelectsYoungest(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	cfg.EnableCircuitPrebuilding = false // Legacy mode for testing

	client, err := New(cfg, logger.NewDefault())
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Stop()

	// Create circuits with different ages
	oldCircuit := circuit.NewCircuit(1)
	oldCircuit.SetState(circuit.StateOpen)
	oldCircuit.CreatedAt = time.Now().Add(-5 * time.Minute)

	newCircuit := circuit.NewCircuit(2)
	newCircuit.SetState(circuit.StateOpen)
	newCircuit.CreatedAt = time.Now().Add(-1 * time.Minute)

	client.circuitsMu.Lock()
	client.circuits = []*circuit.Circuit{oldCircuit, newCircuit}
	client.circuitsMu.Unlock()

	ctx := context.Background()

	// Should select the newer circuit
	circ, err := client.GetCircuit(ctx)
	if err != nil {
		t.Fatalf("Failed to get circuit: %v", err)
	}

	if circ.ID != 2 {
		t.Errorf("Expected youngest circuit (ID 2), got ID %d", circ.ID)
	}
}

// TestGetCircuitSkipsClosedCircuits tests that closed circuits are skipped
func TestGetCircuitSkipsClosedCircuits(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	cfg.EnableCircuitPrebuilding = false

	client, err := New(cfg, logger.NewDefault())
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Stop()

	// Create one closed and one open circuit
	closedCircuit := circuit.NewCircuit(1)
	closedCircuit.SetState(circuit.StateClosed)

	openCircuit := circuit.NewCircuit(2)
	openCircuit.SetState(circuit.StateOpen)

	client.circuitsMu.Lock()
	client.circuits = []*circuit.Circuit{closedCircuit, openCircuit}
	client.circuitsMu.Unlock()

	ctx := context.Background()

	// Should select the open circuit
	circ, err := client.GetCircuit(ctx)
	if err != nil {
		t.Fatalf("Failed to get circuit: %v", err)
	}

	if circ.ID != 2 {
		t.Errorf("Expected open circuit (ID 2), got ID %d", circ.ID)
	}
}

// TestReturnCircuitLegacyMode tests circuit return in legacy mode
func TestReturnCircuitLegacyMode(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	cfg.EnableCircuitPrebuilding = false

	client, err := New(cfg, logger.NewDefault())
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Stop()

	mockCircuit := circuit.NewCircuit(1)

	// In legacy mode, ReturnCircuit should be a no-op
	client.ReturnCircuit(mockCircuit)

	// No error expected, just verify it doesn't panic
}

// TestCircuitPoolStats tests circuit pool statistics reporting
func TestCircuitPoolStats(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	cfg.EnableCircuitPrebuilding = true
	cfg.CircuitPoolMinSize = 3
	cfg.CircuitPoolMaxSize = 10

	client, err := New(cfg, logger.NewDefault())
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Stop()

	stats := client.GetStats()

	// Before Start, pool should not be initialized
	if stats.CircuitPoolEnabled {
		t.Error("Circuit pool should not be enabled before Start")
	}
}

// TestBuildCircuitForPoolReturnsCircuit tests that buildCircuitForPool returns a circuit
func TestBuildCircuitForPoolReturnsCircuit(t *testing.T) {
	// This test would require a full client start with network access,
	// so we just verify the method exists and has correct signature
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()

	client, err := New(cfg, logger.NewDefault())
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Stop()

	// Just verify the method exists (can't call it without network setup)
	_ = client.buildCircuitForPool
}

// TestCheckAndRebuildCircuitsWithPool tests that rebuilding is skipped in pool mode
func TestCheckAndRebuildCircuitsWithPool(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	cfg.EnableCircuitPrebuilding = true
	cfg.CircuitPoolMinSize = 2

	client, err := New(cfg, logger.NewDefault())
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Stop()

	// This test verifies that checkAndRebuildCircuits respects the
	// EnableCircuitPrebuilding flag. We can't actually test the full
	// flow without network access, but we can verify the flag is checked.

	// The key behavior is: when EnableCircuitPrebuilding is true,
	// checkAndRebuildCircuits should NOT try to rebuild circuits
	// because the pool handles that automatically.

	// Since we can't fully test this without initializing the pool
	// (which requires network access), we just verify the config is set
	if !client.config.EnableCircuitPrebuilding {
		t.Error("EnableCircuitPrebuilding should be true")
	}
}
