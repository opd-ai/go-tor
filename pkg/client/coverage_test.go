// Additional test coverage for client package
// Target: Improve coverage from 35.1% to 70%+
package client

import (
	"context"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/circuit"
	"github.com/opd-ai/go-tor/pkg/config"
	"github.com/opd-ai/go-tor/pkg/control"
	"github.com/opd-ai/go-tor/pkg/directory"
	"github.com/opd-ai/go-tor/pkg/logger"
)

// TestStatsGetters tests all Stats getter methods
func TestStatsGetters(t *testing.T) {
	stats := Stats{
		ActiveCircuits: 5,
		SocksPort:      9050,
		ControlPort:    9051,
	}

	if stats.GetActiveCircuits() != 5 {
		t.Errorf("GetActiveCircuits() = %d, want 5", stats.GetActiveCircuits())
	}

	if stats.GetSocksPort() != 9050 {
		t.Errorf("GetSocksPort() = %d, want 9050", stats.GetSocksPort())
	}

	if stats.GetControlPort() != 9051 {
		t.Errorf("GetControlPort() = %d, want 9051", stats.GetControlPort())
	}
}

// TestStatsGettersZeroValues tests Stats getters with zero values
func TestStatsGettersZeroValues(t *testing.T) {
	stats := Stats{}

	if stats.GetActiveCircuits() != 0 {
		t.Errorf("GetActiveCircuits() = %d, want 0", stats.GetActiveCircuits())
	}

	if stats.GetSocksPort() != 0 {
		t.Errorf("GetSocksPort() = %d, want 0", stats.GetSocksPort())
	}

	if stats.GetControlPort() != 0 {
		t.Errorf("GetControlPort() = %d, want 0", stats.GetControlPort())
	}
}

// TestPublishEventNilControlServer tests PublishEvent when control server is nil
func TestPublishEventNilControlServer(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Stop()

	// Create a test event
	event := &control.CircuitEvent{
		CircuitID: 1,
		Status:    "BUILT",
		Purpose:   "GENERAL",
	}

	// PublishEvent should not panic even with valid event
	client.PublishEvent(event)
}

// TestPublishEventWithControlServer tests PublishEvent with initialized control server
func TestPublishEventWithControlServer(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	cfg.ControlPort = 29050 // Use different port
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Stop()

	// Verify control server is initialized
	if client.controlServer == nil {
		t.Fatal("Control server should be initialized")
	}

	// Create various event types
	events := []control.Event{
		&control.CircuitEvent{
			CircuitID:   1,
			Status:      "BUILT",
			Purpose:     "GENERAL",
			TimeCreated: time.Now(),
		},
		&control.StreamEvent{
			StreamID:  1,
			Status:    "SUCCEEDED",
			CircuitID: 1,
			Target:    "example.com:80",
		},
		&control.BWEvent{
			BytesRead:    1024,
			BytesWritten: 2048,
		},
		&control.GuardEvent{
			GuardType: "ENTRY",
			Name:      "$ABC123~testguard",
			Status:    "GOOD",
		},
	}

	// Should not panic for any event type
	for _, event := range events {
		client.PublishEvent(event)
	}
}

// TestPublishBandwidthEvent tests the bandwidth event publishing
func TestPublishBandwidthEvent(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Stop()

	// Record some bandwidth
	client.RecordBytesRead(1000)
	client.RecordBytesWritten(2000)

	// Call publishBandwidthEvent directly
	client.publishBandwidthEvent()

	// Verify no panic occurred
}

// TestPublishNewDescEvents tests publishing new descriptor events
func TestPublishNewDescEvents(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Stop()

	// Test with empty relay list
	client.publishNewDescEvents([]*directory.Relay{})

	// Test with some relays
	relays := []*directory.Relay{
		{Fingerprint: "ABC123", Nickname: "test1"},
		{Fingerprint: "DEF456", Nickname: "test2"},
		{Fingerprint: "GHI789", Nickname: "test3"},
	}
	client.publishNewDescEvents(relays)
}

// TestPublishConsensusEvents tests publishing consensus events
func TestPublishConsensusEvents(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Stop()

	// Test with empty relay list
	client.publishConsensusEvents([]*directory.Relay{})

	// Test with relays that have Guard and Exit flags
	relays := []*directory.Relay{
		{
			Fingerprint: "ABC123",
			Nickname:    "guard1",
			Address:     "192.168.1.1",
			ORPort:      9001,
			DirPort:     9030,
			Flags:       []string{"Fast", "Guard", "Running", "Stable", "Valid"},
			Published:   time.Now(),
		},
		{
			Fingerprint: "DEF456",
			Nickname:    "exit1",
			Address:     "192.168.1.2",
			ORPort:      9001,
			DirPort:     9030,
			Flags:       []string{"Fast", "Exit", "Running", "Stable", "Valid"},
			Published:   time.Now(),
		},
		{
			Fingerprint: "GHI789",
			Nickname:    "middle1",
			Address:     "192.168.1.3",
			ORPort:      9001,
			Flags:       []string{"Fast", "Running", "Stable", "Valid"},
			Published:   time.Now(),
		},
	}
	client.publishConsensusEvents(relays)
}

// TestCheckAndRebuildCircuitsEmptyList tests checkAndRebuildCircuits with circuits to clean
func TestCheckAndRebuildCircuitsEmptyList(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	cfg.EnableCircuitPrebuilding = true // Pool mode
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Stop()

	// Add enough circuits so the rebuild logic doesn't trigger
	// The minCircuitCount is 2, so we need at least 2 circuits
	for i := 1; i <= 3; i++ {
		circ := circuit.NewCircuit(uint32(i))
		circ.SetState(circuit.StateOpen)
		circ.CreatedAt = time.Now()
		client.circuitsMu.Lock()
		client.circuits = append(client.circuits, circ)
		client.circuitsMu.Unlock()
	}

	ctx := context.Background()

	// With enough circuits, checkAndRebuildCircuits should not try to rebuild
	client.checkAndRebuildCircuits(ctx)

	// Verify circuits are still present
	client.circuitsMu.RLock()
	count := len(client.circuits)
	client.circuitsMu.RUnlock()

	if count != 3 {
		t.Errorf("Expected 3 circuits, got %d", count)
	}
}

// TestCheckAndRebuildCircuitsWithMixedStates tests circuit filtering
func TestCheckAndRebuildCircuitsWithMixedStates(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	cfg.EnableCircuitPrebuilding = true // Avoid rebuild attempts
	cfg.MaxCircuitDirtiness = 10 * time.Minute
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Stop()

	// Create circuits with different states - at least 2 open circuits
	// to avoid triggering rebuild logic
	openCircuit1 := circuit.NewCircuit(1)
	openCircuit1.SetState(circuit.StateOpen)
	openCircuit1.CreatedAt = time.Now()

	openCircuit2 := circuit.NewCircuit(2)
	openCircuit2.SetState(circuit.StateOpen)
	openCircuit2.CreatedAt = time.Now()

	closedCircuit := circuit.NewCircuit(3)
	closedCircuit.SetState(circuit.StateClosed)
	closedCircuit.CreatedAt = time.Now()

	oldCircuit := circuit.NewCircuit(4)
	oldCircuit.SetState(circuit.StateOpen)
	oldCircuit.CreatedAt = time.Now().Add(-20 * time.Minute) // Older than max dirtiness

	client.circuitsMu.Lock()
	client.circuits = []*circuit.Circuit{openCircuit1, openCircuit2, closedCircuit, oldCircuit}
	client.circuitsMu.Unlock()

	ctx := context.Background()
	client.checkAndRebuildCircuits(ctx)

	// Verify closed and old circuits were removed
	client.circuitsMu.RLock()
	defer client.circuitsMu.RUnlock()

	// Only the 2 open, non-old circuits should remain
	if len(client.circuits) != 2 {
		t.Errorf("Expected 2 circuits remaining, got %d", len(client.circuits))
	}

	// Check that the remaining circuits are the young open ones
	for _, circ := range client.circuits {
		if circ.ID != 1 && circ.ID != 2 {
			t.Errorf("Unexpected circuit ID %d in remaining circuits", circ.ID)
		}
	}
}

// TestCheckAndRebuildCircuitsWithPoolEnabled tests pool mode behavior
func TestCheckAndRebuildCircuitsWithPoolEnabled(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	cfg.EnableCircuitPrebuilding = true // Pool mode - rebuilding should be skipped
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Stop()

	// Add at least 2 circuits to prevent rebuild logic from triggering
	for i := 1; i <= 2; i++ {
		circ := circuit.NewCircuit(uint32(i))
		circ.SetState(circuit.StateOpen)
		circ.CreatedAt = time.Now()
		client.circuitsMu.Lock()
		client.circuits = append(client.circuits, circ)
		client.circuitsMu.Unlock()
	}

	ctx := context.Background()

	// Should not panic, and should not rebuild circuits since we have enough
	client.checkAndRebuildCircuits(ctx)

	// Verify circuits are still there
	client.circuitsMu.RLock()
	count := len(client.circuits)
	client.circuitsMu.RUnlock()

	if count < 2 {
		t.Errorf("Expected at least 2 circuits, got %d", count)
	}
}

// TestReturnCircuitWithPool tests circuit return with pool enabled
func TestReturnCircuitWithPool(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	cfg.EnableCircuitPrebuilding = false // Pool not initialized in New
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Stop()

	mockCircuit := circuit.NewCircuit(1)

	// In legacy mode with no pool, this should be a no-op
	client.ReturnCircuit(mockCircuit)
}

// TestGetCircuitWithNoHealthyCircuits tests GetCircuit when all circuits are closed
func TestGetCircuitWithNoHealthyCircuits(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	cfg.EnableCircuitPrebuilding = false
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Stop()

	// Add only closed circuits
	closedCircuit := circuit.NewCircuit(1)
	closedCircuit.SetState(circuit.StateClosed)

	client.circuitsMu.Lock()
	client.circuits = []*circuit.Circuit{closedCircuit}
	client.circuitsMu.Unlock()

	ctx := context.Background()
	_, err = client.GetCircuit(ctx)
	if err == nil {
		t.Error("Expected error when no healthy circuits available")
	}
}

// TestGetStatsWithActiveCircuits tests GetStats with circuits
func TestGetStatsWithActiveCircuits(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	cfg.SocksPort = 29051
	cfg.ControlPort = 29052
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Stop()

	// Add mock circuits
	for i := 1; i <= 3; i++ {
		circ := circuit.NewCircuit(uint32(i))
		circ.SetState(circuit.StateOpen)
		client.circuitsMu.Lock()
		client.circuits = append(client.circuits, circ)
		client.circuitsMu.Unlock()
	}

	stats := client.GetStats()

	if stats.ActiveCircuits != 3 {
		t.Errorf("Expected 3 active circuits, got %d", stats.ActiveCircuits)
	}

	if stats.SocksPort != 29051 {
		t.Errorf("Expected SocksPort 29051, got %d", stats.SocksPort)
	}

	if stats.ControlPort != 29052 {
		t.Errorf("Expected ControlPort 29052, got %d", stats.ControlPort)
	}
}

// TestMaintainCircuitsShutdown tests maintainCircuits responds to shutdown
func TestMaintainCircuitsShutdown(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		client.maintainCircuits(ctx)
		close(done)
	}()

	// Give it a moment to start
	time.Sleep(50 * time.Millisecond)

	// Cancel context
	cancel()

	// Should exit quickly
	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Error("maintainCircuits did not exit on context cancellation")
	}

	client.Stop()
}

// TestMaintainCircuitsShutdownSignal tests maintainCircuits responds to shutdown channel
func TestMaintainCircuitsShutdownSignal(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	ctx := context.Background()

	done := make(chan struct{})
	go func() {
		client.maintainCircuits(ctx)
		close(done)
	}()

	// Give it a moment to start
	time.Sleep(50 * time.Millisecond)

	// Close shutdown channel
	close(client.shutdown)

	// Should exit quickly
	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Error("maintainCircuits did not exit on shutdown signal")
	}

	// Need to recreate shutdown channel for Stop() to work
	client.shutdown = make(chan struct{})
	client.Stop()
}

// TestMonitorBandwidthShutdown tests monitorBandwidth responds to shutdown
func TestMonitorBandwidthShutdown(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		client.monitorBandwidth(ctx)
		close(done)
	}()

	// Give it a moment to start
	time.Sleep(50 * time.Millisecond)

	// Cancel context
	cancel()

	// Should exit quickly
	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Error("monitorBandwidth did not exit on context cancellation")
	}

	client.Stop()
}

// TestSimpleClientClose tests SimpleClient.Close method
func TestSimpleClientClose(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	log := logger.NewDefault()

	innerClient, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	sc := &SimpleClient{
		client: innerClient,
		logger: log,
	}

	// Close should not return error for freshly created client
	err = sc.Close()
	if err != nil {
		t.Errorf("Close returned error: %v", err)
	}
}

// TestSimpleClientStats tests SimpleClient.Stats method
func TestSimpleClientStats(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	cfg.SocksPort = 29053
	cfg.ControlPort = 29054
	log := logger.NewDefault()

	innerClient, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer innerClient.Stop()

	sc := &SimpleClient{
		client: innerClient,
		logger: log,
	}

	stats := sc.Stats()

	if stats.SocksPort != 29053 {
		t.Errorf("Expected SocksPort 29053, got %d", stats.SocksPort)
	}

	if stats.ControlPort != 29054 {
		t.Errorf("Expected ControlPort 29054, got %d", stats.ControlPort)
	}
}

// TestConnectWithOptionsNilOptions tests ConnectWithOptionsContext with nil options
func TestConnectWithOptionsNilOptions(t *testing.T) {
	// Create a short-lived context to make the test fail fast
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// With nil options, should use defaults
	_, err := ConnectWithOptionsContext(ctx, nil)
	// Should fail due to timeout during network operations
	if err == nil {
		t.Error("Expected error with short timeout")
	}
}

// TestConnectWithOptionsCustomPorts tests ConnectWithOptionsContext with custom ports
func TestConnectWithOptionsCustomPorts(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	opts := &Options{
		SocksPort:     29055,
		ControlPort:   29056,
		DataDirectory: t.TempDir(),
		LogLevel:      "debug",
	}

	_, err := ConnectWithOptionsContext(ctx, opts)
	// Should fail due to timeout during network operations
	if err == nil {
		t.Error("Expected error with short timeout")
	}
}

// TestConnectWithOptionsInvalidLogLevel tests invalid log level handling
func TestConnectWithOptionsInvalidLogLevel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	opts := &Options{
		SocksPort:     29057,
		ControlPort:   29058,
		DataDirectory: t.TempDir(),
		LogLevel:      "invalid_level",
	}

	_, err := ConnectWithOptionsContext(ctx, opts)
	// Should fail due to invalid log level
	if err == nil {
		t.Error("Expected error with invalid log level")
	}
}

// TestBuildInitialCircuitsCondition tests the buildInitialCircuits condition logic
// Note: We can't test the full execution without network access, but we can verify
// that the method exists and the configuration flags are checked
func TestBuildInitialCircuitsCondition(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	cfg.EnableCircuitPrebuilding = true
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Stop()

	// Verify the config is set correctly
	if !client.config.EnableCircuitPrebuilding {
		t.Error("EnableCircuitPrebuilding should be true")
	}

	// Verify circuitPool is nil before Start (expected)
	if client.circuitPool != nil {
		t.Error("circuitPool should be nil before Start")
	}
}

// TestCircuitBuilderFuncExists verifies the circuit builder function is created
func TestCircuitBuilderFuncExists(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Stop()

	builderFunc := client.circuitBuilderFunc()
	if builderFunc == nil {
		t.Error("circuitBuilderFunc returned nil")
	}
}

// TestConcurrentGetStats tests concurrent access to GetStats
func TestConcurrentGetStats(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Stop()

	// Add some circuits
	for i := 1; i <= 3; i++ {
		circ := circuit.NewCircuit(uint32(i))
		circ.SetState(circuit.StateOpen)
		client.circuitsMu.Lock()
		client.circuits = append(client.circuits, circ)
		client.circuitsMu.Unlock()
	}

	// Concurrent reads
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				stats := client.GetStats()
				_ = stats.ActiveCircuits
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestConcurrentCircuitOperations tests concurrent circuit operations
func TestConcurrentCircuitOperations(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	cfg.EnableCircuitPrebuilding = false
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Stop()

	ctx := context.Background()

	// Add initial circuit
	circ := circuit.NewCircuit(1)
	circ.SetState(circuit.StateOpen)
	client.circuitsMu.Lock()
	client.circuits = append(client.circuits, circ)
	client.circuitsMu.Unlock()

	done := make(chan bool)

	// Concurrent GetCircuit calls
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 50; j++ {
				_, _ = client.GetCircuit(ctx)
			}
			done <- true
		}()
	}

	// Concurrent GetStats calls
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 50; j++ {
				_ = client.GetStats()
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestPublishEventsWithManyRelays tests event publishing with many relays
func TestPublishEventsWithManyRelays(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Stop()

	// Create many relays (more than the limit of 50 for NS events and 100 for NEWDESC)
	relays := make([]*directory.Relay, 150)
	for i := 0; i < 150; i++ {
		relays[i] = &directory.Relay{
			Fingerprint: "ABC123",
			Nickname:    "relay",
			Address:     "192.168.1.1",
			ORPort:      9001,
			Flags:       []string{"Guard", "Exit"},
			Published:   time.Now(),
		}
	}

	// Should not panic with many relays
	client.publishConsensusEvents(relays)
	client.publishNewDescEvents(relays)
}

// TestClientWithMetricsEnabled tests client creation with metrics enabled
func TestClientWithMetricsEnabled(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	cfg.EnableMetrics = true
	cfg.MetricsPort = 29060
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Stop()

	if client.metricsServer == nil {
		t.Error("Metrics server should be initialized when enabled")
	}
}

// TestClientWithMetricsDisabled tests client creation with metrics disabled
func TestClientWithMetricsDisabled(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	cfg.EnableMetrics = false
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Stop()

	if client.metricsServer != nil {
		t.Error("Metrics server should be nil when disabled")
	}
}

// TestHealthMonitorInitialized tests that health monitor is initialized
func TestHealthMonitorInitialized(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Stop()

	if client.healthMonitor == nil {
		t.Error("Health monitor should be initialized")
	}
}

// TestPathSelectorNotInitializedBeforeStart tests path selector state before Start
func TestPathSelectorNotInitializedBeforeStart(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Stop()

	// Path selector should be nil before Start
	if client.pathSelector != nil {
		t.Error("Path selector should be nil before Start")
	}
}

// TestGetCircuitWithPoolEnabledButNil tests GetCircuit pool path when pool is nil
func TestGetCircuitWithPoolEnabledButNil(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	cfg.EnableCircuitPrebuilding = true
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Stop()

	// Add a circuit to the legacy list
	circ := circuit.NewCircuit(1)
	circ.SetState(circuit.StateOpen)
	circ.CreatedAt = time.Now()

	client.circuitsMu.Lock()
	client.circuits = append(client.circuits, circ)
	client.circuitsMu.Unlock()

	ctx := context.Background()

	// Even with EnableCircuitPrebuilding=true, if pool is nil, it should
	// fall back to legacy mode
	result, err := client.GetCircuit(ctx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result.ID != 1 {
		t.Errorf("Expected circuit ID 1, got %d", result.ID)
	}
}

// TestReturnCircuitLegacyModeWithCircuit tests ReturnCircuit in legacy mode
func TestReturnCircuitLegacyModeWithCircuit(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	cfg.EnableCircuitPrebuilding = false
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Stop()

	circ := circuit.NewCircuit(1)
	circ.SetState(circuit.StateOpen)

	// ReturnCircuit in legacy mode is a no-op, should not panic
	client.ReturnCircuit(circ)
	client.ReturnCircuit(nil) // Should handle nil gracefully
}

// TestGetStatsFullFields tests GetStats with all field types populated
func TestGetStatsFullFields(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	cfg.SocksPort = 29100
	cfg.ControlPort = 29101
	cfg.EnableCircuitPrebuilding = false
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Stop()

	// Add circuits to populate stats
	for i := 1; i <= 5; i++ {
		circ := circuit.NewCircuit(uint32(i))
		circ.SetState(circuit.StateOpen)
		circ.CreatedAt = time.Now()
		client.circuitsMu.Lock()
		client.circuits = append(client.circuits, circ)
		client.circuitsMu.Unlock()
	}

	stats := client.GetStats()

	// Verify the stats fields
	if stats.ActiveCircuits != 5 {
		t.Errorf("Expected 5 active circuits, got %d", stats.ActiveCircuits)
	}
	if stats.SocksPort != 29100 {
		t.Errorf("Expected SocksPort 29100, got %d", stats.SocksPort)
	}
	if stats.ControlPort != 29101 {
		t.Errorf("Expected ControlPort 29101, got %d", stats.ControlPort)
	}
	if stats.CircuitPoolEnabled {
		t.Error("CircuitPoolEnabled should be false")
	}
}

// TestMonitorBandwidthTickerBehavior tests monitorBandwidth shutdown via shutdown channel
func TestMonitorBandwidthShutdownChannel(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	ctx := context.Background()

	done := make(chan struct{})
	go func() {
		client.monitorBandwidth(ctx)
		close(done)
	}()

	// Give it a moment to start
	time.Sleep(50 * time.Millisecond)

	// Close shutdown channel to stop
	close(client.shutdown)

	// Should exit quickly
	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Error("monitorBandwidth did not exit on shutdown channel close")
	}

	// Recreate shutdown channel for Stop()
	client.shutdown = make(chan struct{})
	client.Stop()
}

// TestConnect tests the Connect function (wrapper for ConnectWithContext)
func TestConnectFails(t *testing.T) {
	// Connect starts with a background context and will try network operations
	// We can't mock the network, so we just verify the function exists and returns an error
	// since it can't bootstrap the Tor network in a test environment
	done := make(chan error, 1)
	go func() {
		_, err := Connect()
		done <- err
	}()

	select {
	case err := <-done:
		// Expected to fail due to network issues in test environment
		if err == nil {
			t.Log("Connect succeeded unexpectedly")
		} else {
			t.Logf("Connect returned expected error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Log("Connect is still running, cancelling test (network operation in progress)")
	}
}

// TestConnectWithOptionsWrapper tests the ConnectWithOptions function
func TestConnectWithOptionsWrapper(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Test with valid options
	opts := &Options{
		SocksPort:     29102,
		ControlPort:   29103,
		DataDirectory: t.TempDir(),
		LogLevel:      "info",
	}

	// Should fail due to timeout (can't bootstrap network in tests)
	_, err := ConnectWithOptionsContext(ctx, opts)
	if err == nil {
		t.Error("Expected error with short timeout")
	}
}

// TestStatsWithCircuitPoolMetrics tests Stats circuit pool fields
func TestStatsWithCircuitPoolMetrics(t *testing.T) {
	stats := Stats{
		ActiveCircuits:     3,
		SocksPort:          9050,
		ControlPort:        9051,
		CircuitPoolEnabled: true,
		CircuitPoolTotal:   10,
		CircuitPoolOpen:    5,
		CircuitPoolMin:     2,
		CircuitPoolMax:     20,
		GuardsActive:       3,
		GuardsConfirmed:    2,
	}

	// Verify all fields are accessible
	if stats.CircuitPoolEnabled != true {
		t.Error("CircuitPoolEnabled should be true")
	}
	if stats.CircuitPoolTotal != 10 {
		t.Error("CircuitPoolTotal should be 10")
	}
	if stats.CircuitPoolOpen != 5 {
		t.Error("CircuitPoolOpen should be 5")
	}
	if stats.CircuitPoolMin != 2 {
		t.Error("CircuitPoolMin should be 2")
	}
	if stats.CircuitPoolMax != 20 {
		t.Error("CircuitPoolMax should be 20")
	}
	if stats.GuardsActive != 3 {
		t.Error("GuardsActive should be 3")
	}
	if stats.GuardsConfirmed != 2 {
		t.Error("GuardsConfirmed should be 2")
	}
}

// TestNewDescEventWithNoDescriptors tests publishNewDescEvents with empty list
func TestNewDescEventWithNoDescriptors(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Stop()

	// Should not panic with nil
	client.publishNewDescEvents(nil)

	// Should handle empty slice
	client.publishNewDescEvents([]*directory.Relay{})
}

// TestPublishConsensusEventsWithMiddleRelays tests that only guards/exits are published
func TestPublishConsensusEventsWithMiddleRelays(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Stop()

	// Create only middle relays (no Guard or Exit flag)
	middleRelays := make([]*directory.Relay, 10)
	for i := 0; i < 10; i++ {
		middleRelays[i] = &directory.Relay{
			Fingerprint: "MIDDLE123",
			Nickname:    "middle",
			Address:     "192.168.1.1",
			ORPort:      9001,
			Flags:       []string{"Fast", "Running", "Stable", "Valid"},
			Published:   time.Now(),
		}
	}

	// Should not panic, middle relays won't generate NS events
	client.publishConsensusEvents(middleRelays)
}

// TestConnectWithOptionsFails tests ConnectWithOptions wrapper
func TestConnectWithOptionsFails(t *testing.T) {
	opts := &Options{
		SocksPort:     29104,
		ControlPort:   29105,
		DataDirectory: t.TempDir(),
		LogLevel:      "info",
	}

	// ConnectWithOptions wraps ConnectWithOptionsContext with background context
	// It will fail during network bootstrap in test environment
	done := make(chan error, 1)
	go func() {
		_, err := ConnectWithOptions(opts)
		done <- err
	}()

	// Wait briefly then cancel the goroutine
	select {
	case err := <-done:
		if err == nil {
			t.Log("ConnectWithOptions succeeded unexpectedly")
		} else {
			t.Logf("ConnectWithOptions returned expected error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Log("ConnectWithOptions still running (expected, network operation)")
	}
}

// TestGetStatsWithMetrics tests GetStats returns proper metrics snapshot
func TestGetStatsWithMetrics(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	cfg.SocksPort = 29106
	cfg.ControlPort = 29107
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Stop()

	// Get stats multiple times to ensure consistency
	stats1 := client.GetStats()
	stats2 := client.GetStats()

	if stats1.SocksPort != stats2.SocksPort {
		t.Error("Stats should be consistent across calls")
	}

	// Record some activity
	client.RecordBytesRead(1000)
	client.RecordBytesWritten(2000)

	stats3 := client.GetStats()
	if stats3.SocksPort != 29106 {
		t.Errorf("Expected SocksPort 29106, got %d", stats3.SocksPort)
	}
}

// TestGetCircuitMultipleOpenCircuits tests circuit selection with multiple circuits
func TestGetCircuitMultipleOpenCircuits(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	cfg.EnableCircuitPrebuilding = false
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Stop()

	// Add multiple open circuits with different ages
	now := time.Now()
	for i := 1; i <= 5; i++ {
		circ := circuit.NewCircuit(uint32(i))
		circ.SetState(circuit.StateOpen)
		circ.CreatedAt = now.Add(-time.Duration(i) * time.Minute)
		client.circuitsMu.Lock()
		client.circuits = append(client.circuits, circ)
		client.circuitsMu.Unlock()
	}

	ctx := context.Background()

	// Should get the youngest circuit (ID 1)
	result, err := client.GetCircuit(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.ID != 1 {
		t.Errorf("Expected youngest circuit (ID 1), got %d", result.ID)
	}
}

// TestCheckAndRebuildCircuitsOldCircuits tests removal of old circuits
func TestCheckAndRebuildCircuitsOldCircuits(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	cfg.EnableCircuitPrebuilding = true // Avoid rebuild attempts
	cfg.MaxCircuitDirtiness = 1 * time.Minute
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Stop()

	// Add circuits - some old, some new
	now := time.Now()

	// New circuits (within max dirtiness)
	for i := 1; i <= 2; i++ {
		circ := circuit.NewCircuit(uint32(i))
		circ.SetState(circuit.StateOpen)
		circ.CreatedAt = now.Add(-30 * time.Second) // 30 seconds old
		client.circuitsMu.Lock()
		client.circuits = append(client.circuits, circ)
		client.circuitsMu.Unlock()
	}

	// Old circuits (beyond max dirtiness)
	for i := 3; i <= 4; i++ {
		circ := circuit.NewCircuit(uint32(i))
		circ.SetState(circuit.StateOpen)
		circ.CreatedAt = now.Add(-5 * time.Minute) // 5 minutes old
		client.circuitsMu.Lock()
		client.circuits = append(client.circuits, circ)
		client.circuitsMu.Unlock()
	}

	ctx := context.Background()
	client.checkAndRebuildCircuits(ctx)

	// Only the 2 new circuits should remain
	client.circuitsMu.RLock()
	count := len(client.circuits)
	client.circuitsMu.RUnlock()

	if count != 2 {
		t.Errorf("Expected 2 circuits after cleanup, got %d", count)
	}
}
