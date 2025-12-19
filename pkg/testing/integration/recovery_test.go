//go:build integration
// +build integration

// Package integration provides comprehensive integration testing infrastructure.
package integration

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/circuit"
	"github.com/opd-ai/go-tor/pkg/logger"
	"github.com/opd-ai/go-tor/pkg/pool"
)

// TestCircuitRecoveryOnFailure tests that the system recovers when circuits fail.
func TestCircuitRecoveryOnFailure(t *testing.T) {
	log := logger.NewDefault()
	suite := NewSuite()
	if err := suite.Start(); err != nil {
		t.Fatalf("Failed to start suite: %v", err)
	}
	defer suite.Stop()

	// Track build attempts
	var buildAttempts int32
	var failCount int32

	// Simulate a builder that fails occasionally
	builder := func(ctx context.Context) (*circuit.Circuit, error) {
		attempt := atomic.AddInt32(&buildAttempts, 1)

		// Fail the first 2 attempts
		if attempt <= 2 {
			atomic.AddInt32(&failCount, 1)
			return nil, errors.New("simulated build failure")
		}

		// Successful build
		return suite.CreateMockCircuit(ctx, 3)
	}

	poolConfig := &pool.CircuitPoolConfig{
		MinCircuits:     1,
		MaxCircuits:     5,
		PrebuildEnabled: true,
		RebuildInterval: 50 * time.Millisecond,
	}

	circPool := pool.NewCircuitPool(poolConfig, builder, log)
	if circPool == nil {
		t.Fatal("Failed to create circuit pool")
	}
	defer circPool.Close()

	// Wait for recovery attempts
	time.Sleep(300 * time.Millisecond)

	// Should have made multiple attempts
	attempts := atomic.LoadInt32(&buildAttempts)
	failures := atomic.LoadInt32(&failCount)

	if attempts < 3 {
		t.Errorf("Expected at least 3 build attempts, got %d", attempts)
	}

	if failures != 2 {
		t.Errorf("Expected 2 failures, got %d", failures)
	}

	// Eventually should have a working circuit
	stats := circPool.Stats()
	if stats.Open < 1 {
		t.Errorf("Expected at least 1 open circuit after recovery, got %d", stats.Open)
	}

	t.Logf("Recovery test: %d attempts, %d failures, %d open circuits",
		attempts, failures, stats.Open)
}

// TestCircuitPoolRecoveryFromEmpty tests pool recovery when all circuits are closed.
func TestCircuitPoolRecoveryFromEmpty(t *testing.T) {
	log := logger.NewDefault()
	suite := NewSuite()
	if err := suite.Start(); err != nil {
		t.Fatalf("Failed to start suite: %v", err)
	}
	defer suite.Stop()

	builder := func(ctx context.Context) (*circuit.Circuit, error) {
		return suite.CreateMockCircuit(ctx, 3)
	}

	poolConfig := &pool.CircuitPoolConfig{
		MinCircuits:     2,
		MaxCircuits:     5,
		PrebuildEnabled: true,
		RebuildInterval: 50 * time.Millisecond,
	}

	circPool := pool.NewCircuitPool(poolConfig, builder, log)
	defer circPool.Close()

	// Wait for initial circuits to be built
	time.Sleep(200 * time.Millisecond)

	// Get and close all circuits (consume them without putting back)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		circ, err := circPool.Get(ctx)
		if err != nil {
			break // No more circuits available
		}
		circ.Close() // Close instead of putting back
	}

	// Wait for pool to rebuild, polling until at least one circuit is open or timeout
	maxWait := 2 * time.Second
	deadline := time.Now().Add(maxWait)
	for {
		stats := circPool.Stats()
		if stats.Open >= 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("Pool did not recover to at least 1 open circuit within %v; stats: %+v", maxWait, stats)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// TestConcurrentCircuitFailureAndRecovery tests recovery under concurrent failures.
func TestConcurrentCircuitFailureAndRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent recovery test in short mode")
	}

	log := logger.NewDefault()
	suite := NewSuite()
	if err := suite.Start(); err != nil {
		t.Fatalf("Failed to start suite: %v", err)
	}
	defer suite.Stop()

	var buildCount int32
	builder := func(ctx context.Context) (*circuit.Circuit, error) {
		atomic.AddInt32(&buildCount, 1)
		return suite.CreateMockCircuit(ctx, 3)
	}

	poolConfig := &pool.CircuitPoolConfig{
		MinCircuits:     5,
		MaxCircuits:     20,
		PrebuildEnabled: true,
		RebuildInterval: 20 * time.Millisecond,
	}

	circPool := pool.NewCircuitPool(poolConfig, builder, log)
	defer circPool.Close()

	// Wait for initial pool
	time.Sleep(200 * time.Millisecond)

	// Concurrent circuit usage and failures
	var wg sync.WaitGroup
	const numWorkers = 10
	const iterationsPerWorker = 5

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			ctx := context.Background()

			for j := 0; j < iterationsPerWorker; j++ {
				circ, err := circPool.Get(ctx)
				if err != nil {
					continue
				}

				// Simulate some work
				time.Sleep(5 * time.Millisecond)

				// Half the time, close the circuit (simulating failure)
				// Other half, return it to the pool
				if (workerID+j)%2 == 0 {
					circ.Close()
				} else {
					circPool.Put(circ)
				}
			}
		}(i)
	}

	wg.Wait()

	// Give time for recovery
	time.Sleep(300 * time.Millisecond)

	finalStats := circPool.Stats()
	finalBuilds := atomic.LoadInt32(&buildCount)

	t.Logf("Concurrent failure test: %d total builds, final pool: %d open, %d total",
		finalBuilds, finalStats.Open, finalStats.Total)

	// Should have built more circuits to replace failed ones
	if finalBuilds < int32(numWorkers) {
		t.Errorf("Expected more build attempts, got %d", finalBuilds)
	}
}

// TestCircuitTimeoutRecovery tests recovery from circuit operation timeouts.
func TestCircuitTimeoutRecovery(t *testing.T) {
	log := logger.NewDefault()
	suite := NewSuite()
	if err := suite.Start(); err != nil {
		t.Fatalf("Failed to start suite: %v", err)
	}
	defer suite.Stop()

	var slowBuildCount int32
	builder := func(ctx context.Context) (*circuit.Circuit, error) {
		count := atomic.AddInt32(&slowBuildCount, 1)

		// First few builds are slow
		if count <= 2 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(500 * time.Millisecond):
				return suite.CreateMockCircuit(ctx, 3)
			}
		}

		// Later builds are fast
		return suite.CreateMockCircuit(ctx, 3)
	}

	poolConfig := &pool.CircuitPoolConfig{
		MinCircuits:     1,
		MaxCircuits:     5,
		PrebuildEnabled: true,
		RebuildInterval: 100 * time.Millisecond,
	}

	circPool := pool.NewCircuitPool(poolConfig, builder, log)
	defer circPool.Close()

	// Try to get a circuit with a short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := circPool.Get(ctx)
	if err == nil {
		t.Log("Got circuit quickly")
	} else {
		t.Logf("Initial get timed out (expected): %v", err)
	}

	// Wait longer for pool to have circuits
	time.Sleep(1 * time.Second)

	// Now should be able to get a circuit
	ctx2, cancel2 := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel2()

	circ, err := circPool.Get(ctx2)
	if err != nil {
		t.Logf("Second get attempt: %v", err)
	} else {
		t.Logf("Got circuit on second attempt: ID=%d", circ.ID)
		circPool.Put(circ)
	}
}

// TestSequentialFailureRecovery tests recovery from a sequence of failures.
func TestSequentialFailureRecovery(t *testing.T) {
	log := logger.NewDefault()
	suite := NewSuite()
	if err := suite.Start(); err != nil {
		t.Fatalf("Failed to start suite: %v", err)
	}
	defer suite.Stop()

	// Failure probability decreases over time
	var attemptNum int32
	builder := func(ctx context.Context) (*circuit.Circuit, error) {
		n := atomic.AddInt32(&attemptNum, 1)

		// First 5 attempts fail
		if n <= 5 {
			return nil, errors.New("simulated sequential failure")
		}

		return suite.CreateMockCircuit(ctx, 3)
	}

	poolConfig := &pool.CircuitPoolConfig{
		MinCircuits:     1,
		MaxCircuits:     3,
		PrebuildEnabled: true,
		RebuildInterval: 30 * time.Millisecond,
	}

	circPool := pool.NewCircuitPool(poolConfig, builder, log)
	defer circPool.Close()

	// Wait for recovery
	time.Sleep(500 * time.Millisecond)

	// Should eventually have working circuits
	stats := circPool.Stats()
	attempts := atomic.LoadInt32(&attemptNum)

	t.Logf("Sequential failure recovery: %d attempts, %d open circuits",
		attempts, stats.Open)

	if attempts < 6 {
		t.Errorf("Expected at least 6 attempts for recovery, got %d", attempts)
	}
}

// TestCircuitStateRecovery tests recovery based on circuit state changes.
func TestCircuitStateRecovery(t *testing.T) {
	suite := NewSuite()
	if err := suite.Start(); err != nil {
		t.Fatalf("Failed to start suite: %v", err)
	}
	defer suite.Stop()

	ctx := context.Background()

	// Create a circuit
	circ, err := suite.CreateMockCircuit(ctx, 3)
	if err != nil {
		t.Fatalf("Failed to create circuit: %v", err)
	}

	// Record initial state
	initialState := circ.GetState()
	if initialState != circuit.StateOpen {
		t.Errorf("Expected initial state OPEN, got %v", initialState)
	}

	// Simulate failure
	circ.SetState(circuit.StateFailed)
	if circ.GetState() != circuit.StateFailed {
		t.Errorf("Expected state FAILED after setting, got %v", circ.GetState())
	}

	// Close the failed circuit
	circ.Close()
	if circ.GetState() != circuit.StateClosed {
		t.Errorf("Expected state CLOSED after close, got %v", circ.GetState())
	}

	// Create a replacement circuit
	replacement, err := suite.CreateMockCircuit(ctx, 3)
	if err != nil {
		t.Fatalf("Failed to create replacement circuit: %v", err)
	}

	if replacement.GetState() != circuit.StateOpen {
		t.Errorf("Expected replacement state OPEN, got %v", replacement.GetState())
	}

	t.Log("Circuit state recovery test passed")
}

// TestGracefulDegradation tests system behavior under degraded conditions.
func TestGracefulDegradation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping graceful degradation test in short mode")
	}

	log := logger.NewDefault()
	suite := NewSuite()
	if err := suite.Start(); err != nil {
		t.Fatalf("Failed to start suite: %v", err)
	}
	defer suite.Stop()

	// Failure rate increases over time then decreases
	var buildNum int32
	builder := func(ctx context.Context) (*circuit.Circuit, error) {
		n := atomic.AddInt32(&buildNum, 1)

		// Middle builds have high failure rate
		if n >= 3 && n <= 7 {
			if n%2 == 0 {
				return nil, errors.New("simulated degraded condition")
			}
		}

		return suite.CreateMockCircuit(ctx, 3)
	}

	poolConfig := &pool.CircuitPoolConfig{
		MinCircuits:     3,
		MaxCircuits:     10,
		PrebuildEnabled: true,
		RebuildInterval: 50 * time.Millisecond,
	}

	circPool := pool.NewCircuitPool(poolConfig, builder, log)
	defer circPool.Close()

	// Monitor pool health during degradation using ticker-based collection
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	var samples []int
	for len(samples) < 10 {
		select {
		case <-ctx.Done():
			t.Fatalf("timed out collecting circuit pool stats: collected %d samples", len(samples))
		case <-ticker.C:
			stats := circPool.Stats()
			samples = append(samples, stats.Open)
		}
	}

	// System should maintain some capacity even during degradation
	minObserved := samples[0]
	maxObserved := samples[0]
	for _, s := range samples {
		if s < minObserved {
			minObserved = s
		}
		if s > maxObserved {
			maxObserved = s
		}
	}

	t.Logf("Graceful degradation: builds=%d, min=%d, max=%d circuits",
		atomic.LoadInt32(&buildNum), minObserved, maxObserved)
}
