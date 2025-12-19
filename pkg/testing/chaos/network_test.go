//go:build integration
// +build integration

// Package chaos provides chaos engineering testing infrastructure.
package chaos

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
	"github.com/opd-ai/go-tor/pkg/testing/integration"
)

// TestChaosEngineBasic tests basic chaos engine functionality.
func TestChaosEngineBasic(t *testing.T) {
	log := logger.NewDefault()
	engine := NewEngine(DefaultConfig(), log)

	// Initially disabled
	if engine.IsActive() {
		t.Error("Engine should be inactive by default")
	}

	// Enable and verify
	engine.Enable()
	if !engine.IsActive() {
		t.Error("Engine should be active after Enable()")
	}

	// Pause and verify
	engine.Pause()
	if engine.IsActive() {
		t.Error("Engine should be inactive while paused")
	}

	// Resume and verify
	engine.Resume()
	if !engine.IsActive() {
		t.Error("Engine should be active after Resume()")
	}

	// Disable and verify
	engine.Disable()
	if engine.IsActive() {
		t.Error("Engine should be inactive after Disable()")
	}
}

// TestChaosFailureInjection tests failure injection.
func TestChaosFailureInjection(t *testing.T) {
	cfg := &Config{
		FailureRate:     1.0, // 100% failure rate for testing
		LatencyMin:      0,
		LatencyMax:      1 * time.Millisecond,
		ConnectionDrops: 0,
	}

	log := logger.NewDefault()
	engine := NewEngine(cfg, log)
	engine.Enable()

	// Should always fail with 100% rate
	err := engine.MaybeInjectFailure()
	if err == nil {
		t.Error("Expected failure to be injected with 100% rate")
	}

	if !errors.Is(err, ErrChaosFailure) {
		t.Errorf("Expected ErrChaosFailure, got %v", err)
	}

	// Check stats
	stats := engine.Stats()
	if stats.FailuresInjected < 1 {
		t.Errorf("Expected at least 1 failure injected, got %d", stats.FailuresInjected)
	}
}

// TestChaosNoInjectionWhenDisabled tests that chaos is not injected when disabled.
func TestChaosNoInjectionWhenDisabled(t *testing.T) {
	cfg := &Config{
		FailureRate:     1.0, // Would always fail if active
		LatencyMin:      0,
		LatencyMax:      1 * time.Millisecond,
		ConnectionDrops: 1.0, // Would always drop if active
	}

	log := logger.NewDefault()
	engine := NewEngine(cfg, log)
	// Don't enable the engine

	// Should not inject failures when disabled
	err := engine.MaybeInjectFailure()
	if err != nil {
		t.Errorf("Expected no failure when disabled, got %v", err)
	}

	// Should not drop connections when disabled
	if engine.MaybeInjectDrop() {
		t.Error("Expected no drop when disabled")
	}

	// Stats should be zero
	stats := engine.Stats()
	if stats.Total() != 0 {
		t.Errorf("Expected 0 total events when disabled, got %d", stats.Total())
	}
}

// TestChaosStatsTracking tests that statistics are properly tracked.
func TestChaosStatsTracking(t *testing.T) {
	cfg := &Config{
		FailureRate:     1.0,
		LatencyMin:      0,
		LatencyMax:      1 * time.Millisecond,
		ConnectionDrops: 1.0,
	}

	log := logger.NewDefault()
	engine := NewEngine(cfg, log)
	engine.Enable()

	// Generate some events
	for i := 0; i < 5; i++ {
		_ = engine.MaybeInjectFailure()
		engine.MaybeInjectDrop()
	}

	stats := engine.Stats()
	if stats.FailuresInjected != 5 {
		t.Errorf("Expected 5 failures, got %d", stats.FailuresInjected)
	}
	if stats.DropsInjected != 5 {
		t.Errorf("Expected 5 drops, got %d", stats.DropsInjected)
	}

	// Reset and verify
	engine.ResetStats()
	stats = engine.Stats()
	if stats.Total() != 0 {
		t.Errorf("Expected 0 total after reset, got %d", stats.Total())
	}
}

// TestNetworkFaultInjector tests the network fault injector.
func TestNetworkFaultInjector(t *testing.T) {
	log := logger.NewDefault()
	injector := NewNetworkFaultInjector(log)

	// Enable and set packet loss
	injector.Enable()
	injector.SetPacketLoss(1.0) // 100% packet loss

	// Should always drop
	if !injector.ShouldDropPacket() {
		t.Error("Expected packet drop with 100% loss rate")
	}

	// Test network partition
	injector.Partition()
	if !injector.IsPartitioned() {
		t.Error("Expected network to be partitioned")
	}

	// Drop while partitioned to count another drop
	if !injector.ShouldDropPacket() {
		t.Error("Expected packet drop while partitioned")
	}

	// Heal partition
	injector.Heal()
	if injector.IsPartitioned() {
		t.Error("Expected network to be healed")
	}

	// Check stats
	stats := injector.Stats()
	if stats.PacketsDropped < 2 {
		t.Errorf("Expected at least 2 packets dropped, got %d", stats.PacketsDropped)
	}
}

// TestBandwidthThrottling tests bandwidth throttling calculation.
func TestBandwidthThrottling(t *testing.T) {
	log := logger.NewDefault()
	injector := NewNetworkFaultInjector(log)
	injector.Enable()

	// Set bandwidth to 1000 bytes per second
	injector.SetBandwidth(1000)

	// 1000 bytes should take about 1 second
	delay := injector.ThrottleDelay(1000)
	if delay < 900*time.Millisecond || delay > 1100*time.Millisecond {
		t.Errorf("Expected ~1s delay for 1000 bytes at 1000 B/s, got %v", delay)
	}

	// 100 bytes should take about 100ms
	delay = injector.ThrottleDelay(100)
	if delay < 90*time.Millisecond || delay > 110*time.Millisecond {
		t.Errorf("Expected ~100ms delay for 100 bytes at 1000 B/s, got %v", delay)
	}

	// No limit should return 0
	injector.SetBandwidth(0)
	delay = injector.ThrottleDelay(10000)
	if delay != 0 {
		t.Errorf("Expected 0 delay with unlimited bandwidth, got %v", delay)
	}
}

// TestRelaySimulator tests the relay simulator.
func TestRelaySimulator(t *testing.T) {
	log := logger.NewDefault()
	relay := NewRelaySimulator(log)

	// Start and verify
	relay.Start()
	if !relay.IsHealthy() {
		t.Error("Relay should be healthy after start")
	}

	// Test connection tracking
	for i := 0; i < 5; i++ {
		if err := relay.AddConnection(); err != nil {
			t.Fatalf("Failed to add connection: %v", err)
		}
	}

	if relay.ActiveConnections() != 5 {
		t.Errorf("Expected 5 active connections, got %d", relay.ActiveConnections())
	}

	// Remove connections
	for i := 0; i < 5; i++ {
		relay.RemoveConnection()
	}

	if relay.ActiveConnections() != 0 {
		t.Errorf("Expected 0 active connections, got %d", relay.ActiveConnections())
	}

	// Test health control
	relay.SetHealthy(false)
	if relay.IsHealthy() {
		t.Error("Relay should be unhealthy")
	}

	relay.Stop()
}

// TestChaosWithCircuitPool tests chaos injection with circuit pool.
func TestChaosWithCircuitPool(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping chaos circuit pool test in short mode")
	}

	log := logger.NewDefault()
	suite := integration.NewSuite()
	if err := suite.Start(); err != nil {
		t.Fatalf("Failed to start suite: %v", err)
	}
	defer suite.Stop()

	// Create chaos engine with moderate failure rate
	chaosCfg := &Config{
		FailureRate:     0.3, // 30% failure rate
		LatencyMin:      5 * time.Millisecond,
		LatencyMax:      20 * time.Millisecond,
		ConnectionDrops: 0.1,
	}
	engine := NewEngine(chaosCfg, log)
	engine.Enable()

	var buildAttempts int32
	var buildFailures int32

	// Circuit builder with chaos injection
	builder := func(ctx context.Context) (*circuit.Circuit, error) {
		atomic.AddInt32(&buildAttempts, 1)

		// Maybe inject chaos failure
		if err := engine.MaybeInjectFailure(); err != nil {
			atomic.AddInt32(&buildFailures, 1)
			return nil, err
		}

		// Maybe add latency
		engine.MaybeInjectLatency()

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

	// Let the pool operate under chaos
	time.Sleep(500 * time.Millisecond)

	attempts := atomic.LoadInt32(&buildAttempts)
	failures := atomic.LoadInt32(&buildFailures)
	stats := circPool.Stats()
	chaosStats := engine.Stats()

	t.Logf("Chaos test results: %d attempts, %d failures, %d open circuits",
		attempts, failures, stats.Open)
	t.Logf("Chaos stats: %d failures injected, %d latencies",
		chaosStats.FailuresInjected, chaosStats.LatencyInjected)

	// Should have made multiple attempts
	if attempts < 3 {
		t.Errorf("Expected at least 3 build attempts, got %d", attempts)
	}

	// Should have some failures with 30% rate
	if attempts > 10 && failures == 0 {
		t.Log("Warning: no failures with 30% rate - possible but unlikely")
	}
}

// TestNetworkPartitionRecovery tests recovery from network partition.
func TestNetworkPartitionRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network partition test in short mode")
	}

	log := logger.NewDefault()
	injector := NewNetworkFaultInjector(log)
	injector.Enable()

	suite := integration.NewSuite()
	if err := suite.Start(); err != nil {
		t.Fatalf("Failed to start suite: %v", err)
	}
	defer suite.Stop()

	var successCount int32
	var failCount int32

	// Circuit builder that respects network partition
	builder := func(ctx context.Context) (*circuit.Circuit, error) {
		if injector.IsPartitioned() {
			atomic.AddInt32(&failCount, 1)
			return nil, errors.New("network partitioned")
		}

		if injector.ShouldDropPacket() {
			atomic.AddInt32(&failCount, 1)
			return nil, errors.New("packet dropped")
		}

		atomic.AddInt32(&successCount, 1)
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

	// Phase 1: Normal operation
	time.Sleep(200 * time.Millisecond)
	phase1Success := atomic.LoadInt32(&successCount)

	// Phase 2: Partition the network
	injector.Partition()
	time.Sleep(200 * time.Millisecond)

	// Phase 3: Heal the network
	injector.Heal()
	time.Sleep(300 * time.Millisecond)

	finalSuccess := atomic.LoadInt32(&successCount)
	finalFail := atomic.LoadInt32(&failCount)

	t.Logf("Partition recovery: phase1=%d, final=%d successes, %d failures",
		phase1Success, finalSuccess, finalFail)

	// Should have more successes after healing
	if finalSuccess <= phase1Success {
		t.Log("Warning: no new circuits built after partition heal")
	}
}

// TestConcurrentChaos tests system behavior under concurrent chaos.
func TestConcurrentChaos(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent chaos test in short mode")
	}

	log := logger.NewDefault()
	engine := NewEngine(AggressiveConfig(), log)
	engine.Enable()

	suite := integration.NewSuite()
	if err := suite.Start(); err != nil {
		t.Fatalf("Failed to start suite: %v", err)
	}
	defer suite.Stop()

	var wg sync.WaitGroup
	const numWorkers = 10
	const operationsPerWorker = 20

	var successes int32
	var failures int32

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for j := 0; j < operationsPerWorker; j++ {
				// Try to build a circuit with chaos
				if err := engine.MaybeInjectFailure(); err != nil {
					atomic.AddInt32(&failures, 1)
					continue
				}

				engine.MaybeInjectLatency()

				ctx := context.Background()
				_, err := suite.CreateMockCircuit(ctx, 3)
				if err != nil {
					atomic.AddInt32(&failures, 1)
				} else {
					atomic.AddInt32(&successes, 1)
				}
			}
		}()
	}

	wg.Wait()

	totalOps := numWorkers * operationsPerWorker
	finalSuccesses := atomic.LoadInt32(&successes)
	finalFailures := atomic.LoadInt32(&failures)
	stats := engine.Stats()

	t.Logf("Concurrent chaos: %d total ops, %d successes, %d failures",
		totalOps, finalSuccesses, finalFailures)
	t.Logf("Chaos events: %d total", stats.Total())

	// Some operations should succeed even under aggressive chaos
	if finalSuccesses == 0 {
		t.Error("Expected at least some successes under chaos")
	}
}

// TestRelayOverload tests behavior when relay becomes overloaded.
func TestRelayOverload(t *testing.T) {
	log := logger.NewDefault()
	relay := NewRelaySimulator(log)
	relay.Start()
	defer relay.Stop()

	// Default threshold is 100
	for i := 0; i < 150; i++ {
		if err := relay.AddConnection(); err != nil {
			t.Fatalf("Failed to add connection %d: %v", i, err)
		}
	}

	if !relay.IsOverloaded() {
		t.Error("Relay should be overloaded with 150 connections")
	}

	// Remove connections to go below threshold/2
	for i := 0; i < 120; i++ {
		relay.RemoveConnection()
	}

	if relay.IsOverloaded() {
		t.Error("Relay should not be overloaded with 30 connections")
	}

	if relay.ActiveConnections() != 30 {
		t.Errorf("Expected 30 active connections, got %d", relay.ActiveConnections())
	}
}

// TestChaosContextTimeout tests context wrapping with timeout.
func TestChaosContextTimeout(t *testing.T) {
	cfg := &Config{
		FailureRate:     0,
		TimeoutDuration: 50 * time.Millisecond,
	}

	log := logger.NewDefault()
	engine := NewEngine(cfg, log)
	engine.Enable()

	ctx := context.Background()
	wrappedCtx, cancel := engine.WrapContext(ctx)
	defer cancel()

	// Wait for context to complete with timeout
	select {
	case <-wrappedCtx.Done():
		// Expected
	case <-time.After(200 * time.Millisecond):
		t.Error("Expected context to be done after timeout")
	}
}

// TestChaosConfigPresets tests configuration presets.
func TestChaosConfigPresets(t *testing.T) {
	defaultCfg := DefaultConfig()
	aggressiveCfg := AggressiveConfig()

	// Aggressive should have higher failure rate
	if aggressiveCfg.FailureRate <= defaultCfg.FailureRate {
		t.Errorf("Aggressive failure rate (%v) should be higher than default (%v)",
			aggressiveCfg.FailureRate, defaultCfg.FailureRate)
	}

	// Aggressive should have higher latency
	if aggressiveCfg.LatencyMax <= defaultCfg.LatencyMax {
		t.Errorf("Aggressive latency (%v) should be higher than default (%v)",
			aggressiveCfg.LatencyMax, defaultCfg.LatencyMax)
	}

	// Aggressive should have shorter timeout
	if aggressiveCfg.TimeoutDuration >= defaultCfg.TimeoutDuration {
		t.Errorf("Aggressive timeout (%v) should be shorter than default (%v)",
			aggressiveCfg.TimeoutDuration, defaultCfg.TimeoutDuration)
	}
}
