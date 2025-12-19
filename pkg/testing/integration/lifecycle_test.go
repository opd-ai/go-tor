//go:build integration
// +build integration

// Package integration provides comprehensive integration testing infrastructure.
package integration

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/circuit"
	"github.com/opd-ai/go-tor/pkg/config"
	"github.com/opd-ai/go-tor/pkg/logger"
	"github.com/opd-ai/go-tor/pkg/pool"
)

// TestSuiteLifecycle tests the integration test suite lifecycle.
func TestSuiteLifecycle(t *testing.T) {
	suite := NewSuite()

	// Test start
	if err := suite.Start(); err != nil {
		t.Fatalf("Failed to start suite: %v", err)
	}

	// Starting again should fail
	if err := suite.Start(); err == nil {
		t.Error("Expected error when starting already running suite")
	}

	// Test circuit creation
	ctx := context.Background()
	circ, err := suite.CreateMockCircuit(ctx, 3)
	if err != nil {
		t.Fatalf("Failed to create mock circuit: %v", err)
	}

	if circ.ID == 0 {
		t.Error("Expected non-zero circuit ID")
	}

	if circ.GetState() != circuit.StateOpen {
		t.Errorf("Expected circuit state %v, got %v", circuit.StateOpen, circ.GetState())
	}

	if suite.CircuitCount() != 1 {
		t.Errorf("Expected 1 circuit, got %d", suite.CircuitCount())
	}

	// Test stop
	if err := suite.Stop(); err != nil {
		t.Fatalf("Failed to stop suite: %v", err)
	}

	// Circuits should be cleaned up
	if suite.CircuitCount() != 0 {
		t.Errorf("Expected 0 circuits after stop, got %d", suite.CircuitCount())
	}
}

// TestMockServerLifecycle tests mock server creation and shutdown.
func TestMockServerLifecycle(t *testing.T) {
	server, err := NewMockServer()
	if err != nil {
		t.Fatalf("Failed to create mock server: %v", err)
	}

	if server.Address() == "" {
		t.Error("Expected non-empty server address")
	}

	// Stop the server
	if err := server.Stop(); err != nil {
		t.Fatalf("Failed to stop server: %v", err)
	}

	// Stopping again should be safe (idempotent)
	if err := server.Stop(); err != nil {
		t.Logf("Second stop returned: %v", err)
	}
}

// TestCircuitBuildWithContext tests circuit creation respects context cancellation.
func TestCircuitBuildWithContext(t *testing.T) {
	suite := NewSuite()
	if err := suite.Start(); err != nil {
		t.Fatalf("Failed to start suite: %v", err)
	}
	defer suite.Stop()

	// Create context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Wait for context to expire
	time.Sleep(10 * time.Millisecond)

	// Circuit creation should fail due to context cancellation
	_, err := suite.CreateMockCircuit(ctx, 3)
	if err == nil {
		t.Error("Expected error when context is cancelled")
	}
}

// TestMultipleCircuitsLifecycle tests creating and managing multiple circuits.
func TestMultipleCircuitsLifecycle(t *testing.T) {
	suite := NewSuite()
	if err := suite.Start(); err != nil {
		t.Fatalf("Failed to start suite: %v", err)
	}
	defer suite.Stop()

	ctx := context.Background()
	const numCircuits = 10

	// Create multiple circuits
	circuitIDs := make([]uint32, numCircuits)
	for i := 0; i < numCircuits; i++ {
		circ, err := suite.CreateMockCircuit(ctx, 3)
		if err != nil {
			t.Fatalf("Failed to create circuit %d: %v", i, err)
		}
		circuitIDs[i] = circ.ID
	}

	// Verify all circuits exist
	if suite.CircuitCount() != numCircuits {
		t.Errorf("Expected %d circuits, got %d", numCircuits, suite.CircuitCount())
	}

	// Verify each circuit can be retrieved
	for _, id := range circuitIDs {
		circ, ok := suite.GetCircuit(id)
		if !ok {
			t.Errorf("Failed to get circuit %d", id)
			continue
		}
		if circ.GetState() != circuit.StateOpen {
			t.Errorf("Circuit %d expected state %v, got %v", id, circuit.StateOpen, circ.GetState())
		}
	}
}

// TestConcurrentCircuitCreation tests creating circuits concurrently.
func TestConcurrentCircuitCreation(t *testing.T) {
	suite := NewSuite()
	if err := suite.Start(); err != nil {
		t.Fatalf("Failed to start suite: %v", err)
	}
	defer suite.Stop()

	ctx := context.Background()
	const numGoroutines = 20
	const circuitsPerGoroutine = 5

	var wg sync.WaitGroup
	errCh := make(chan error, numGoroutines*circuitsPerGoroutine)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < circuitsPerGoroutine; j++ {
				_, err := suite.CreateMockCircuit(ctx, 3)
				if err != nil {
					errCh <- err
				}
			}
		}()
	}

	wg.Wait()
	close(errCh)

	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		t.Errorf("Got %d errors during concurrent creation: %v", len(errs), errs[0])
	}

	expectedCount := numGoroutines * circuitsPerGoroutine
	if suite.CircuitCount() != expectedCount {
		t.Errorf("Expected %d circuits, got %d", expectedCount, suite.CircuitCount())
	}
}

// TestCircuitPoolIntegration tests circuit pool with integration test framework.
func TestCircuitPoolIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping circuit pool integration test in short mode")
	}

	log := logger.NewDefault()
	suite := NewSuite()
	if err := suite.Start(); err != nil {
		t.Fatalf("Failed to start suite: %v", err)
	}
	defer suite.Stop()

	// Create a circuit builder that uses the suite
	builder := func(ctx context.Context) (*circuit.Circuit, error) {
		return suite.CreateMockCircuit(ctx, 3)
	}

	poolConfig := &pool.CircuitPoolConfig{
		MinCircuits:     2,
		MaxCircuits:     10,
		PrebuildEnabled: true,
		RebuildInterval: 50 * time.Millisecond,
	}

	circPool := pool.NewCircuitPool(poolConfig, builder, log)
	if circPool == nil {
		t.Fatal("Failed to create circuit pool")
	}
	defer circPool.Close()

	// Wait for prebuilding
	time.Sleep(200 * time.Millisecond)

	// Check pool stats
	stats := circPool.Stats()
	if stats.Open < 2 {
		t.Errorf("Expected at least 2 open circuits after prebuilding, got %d", stats.Open)
	}

	// Get a circuit from the pool
	ctx := context.Background()
	circ, err := circPool.Get(ctx)
	if err != nil {
		t.Fatalf("Failed to get circuit from pool: %v", err)
	}

	if circ == nil {
		t.Fatal("Expected non-nil circuit from pool")
	}

	// Put it back
	circPool.Put(circ)

	t.Logf("Circuit pool integration test passed: %d open circuits", stats.Open)
}

// TestClientLifecycleScenarios tests common client lifecycle scenarios.
func TestClientLifecycleScenarios(t *testing.T) {
	scenarios := []struct {
		name     string
		setup    func() *config.Config
		validate func(cfg *config.Config) error
	}{
		{
			name: "default_config",
			setup: func() *config.Config {
				cfg := config.DefaultConfig()
				cfg.DataDirectory = t.TempDir()
				return cfg
			},
			validate: func(cfg *config.Config) error {
				if cfg.SocksPort == 0 {
					return fmt.Errorf("expected non-zero SOCKS port")
				}
				return nil
			},
		},
		{
			name: "custom_ports",
			setup: func() *config.Config {
				cfg := config.DefaultConfig()
				cfg.DataDirectory = t.TempDir()
				cfg.SocksPort = 19999
				cfg.ControlPort = 19998
				return cfg
			},
			validate: func(cfg *config.Config) error {
				if cfg.SocksPort != 19999 {
					return fmt.Errorf("expected SOCKS port 19999, got %d", cfg.SocksPort)
				}
				if cfg.ControlPort != 19998 {
					return fmt.Errorf("expected control port 19998, got %d", cfg.ControlPort)
				}
				return nil
			},
		},
		{
			name: "metrics_enabled",
			setup: func() *config.Config {
				cfg := config.DefaultConfig()
				cfg.DataDirectory = t.TempDir()
				cfg.EnableMetrics = true
				cfg.MetricsPort = 19997
				return cfg
			},
			validate: func(cfg *config.Config) error {
				if !cfg.EnableMetrics {
					return fmt.Errorf("expected metrics enabled")
				}
				return nil
			},
		},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			cfg := sc.setup()
			if err := sc.validate(cfg); err != nil {
				t.Errorf("Validation failed: %v", err)
			}
		})
	}
}

// TestTestResultsTracking tests the TestResults helper.
func TestTestResultsTracking(t *testing.T) {
	results := NewTestResults()

	// Add passing tests
	results.Add(TestResult{Name: "test1", Passed: true, Duration: time.Second})
	results.Add(TestResult{Name: "test2", Passed: true, Duration: 2 * time.Second})

	// Add failing test
	results.Add(TestResult{Name: "test3", Passed: false, Duration: 500 * time.Millisecond, Error: fmt.Errorf("test error")})

	if results.TotalCount() != 3 {
		t.Errorf("Expected 3 total tests, got %d", results.TotalCount())
	}

	if results.PassCount() != 2 {
		t.Errorf("Expected 2 passed tests, got %d", results.PassCount())
	}

	if results.FailCount() != 1 {
		t.Errorf("Expected 1 failed test, got %d", results.FailCount())
	}

	if results.AllPassed() {
		t.Error("Expected AllPassed() to be false")
	}
}

// TestMockServerConnections tests mock server connection handling.
func TestMockServerConnections(t *testing.T) {
	server, err := NewMockServer()
	if err != nil {
		t.Fatalf("Failed to create mock server: %v", err)
	}
	defer server.Stop()

	// Connect to the server
	conn, err := net.Dial("tcp", server.Address())
	if err != nil {
		t.Fatalf("Failed to connect to mock server: %v", err)
	}
	defer conn.Close()

	// Give time for the server to register the connection
	time.Sleep(50 * time.Millisecond)

	if server.ConnectionCount() != 1 {
		t.Errorf("Expected 1 connection, got %d", server.ConnectionCount())
	}
}

// TestCircuitStateTransitions tests circuit state transitions.
func TestCircuitStateTransitions(t *testing.T) {
	suite := NewSuite()
	if err := suite.Start(); err != nil {
		t.Fatalf("Failed to start suite: %v", err)
	}
	defer suite.Stop()

	ctx := context.Background()
	circ, err := suite.CreateMockCircuit(ctx, 3)
	if err != nil {
		t.Fatalf("Failed to create circuit: %v", err)
	}

	// Should start as open
	if circ.GetState() != circuit.StateOpen {
		t.Errorf("Expected initial state %v, got %v", circuit.StateOpen, circ.GetState())
	}

	// Close the circuit
	circ.Close()

	if circ.GetState() != circuit.StateClosed {
		t.Errorf("Expected state %v after close, got %v", circuit.StateClosed, circ.GetState())
	}
}

// TestCircuitHopConfiguration tests circuit hop configuration.
func TestCircuitHopConfiguration(t *testing.T) {
	suite := NewSuite()
	if err := suite.Start(); err != nil {
		t.Fatalf("Failed to start suite: %v", err)
	}
	defer suite.Stop()

	testCases := []struct {
		name     string
		numHops  int
		expected struct {
			guardAt int
			exitAt  int
		}
	}{
		{
			name:    "standard_3_hops",
			numHops: 3,
			expected: struct {
				guardAt int
				exitAt  int
			}{0, 2},
		},
		{
			name:    "minimal_2_hops",
			numHops: 2,
			expected: struct {
				guardAt int
				exitAt  int
			}{0, 1},
		},
		{
			name:    "extended_5_hops",
			numHops: 5,
			expected: struct {
				guardAt int
				exitAt  int
			}{0, 4},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			circ, err := suite.CreateMockCircuit(ctx, tc.numHops)
			if err != nil {
				t.Fatalf("Failed to create circuit with %d hops: %v", tc.numHops, err)
			}

			hops := circ.GetHops()
			if len(hops) != tc.numHops {
				t.Errorf("Expected %d hops, got %d", tc.numHops, len(hops))
				return
			}

			// Check guard position
			if !hops[tc.expected.guardAt].IsGuard {
				t.Errorf("Expected hop %d to be guard", tc.expected.guardAt)
			}

			// Check exit position
			if !hops[tc.expected.exitAt].IsExit {
				t.Errorf("Expected hop %d to be exit", tc.expected.exitAt)
			}

			// Non-guard, non-exit hops should be neither
			for i, hop := range hops {
				if i != tc.expected.guardAt && hop.IsGuard {
					t.Errorf("Hop %d should not be marked as guard", i)
				}
				if i != tc.expected.exitAt && hop.IsExit {
					t.Errorf("Hop %d should not be marked as exit", i)
				}
			}
		})
	}
}
