package circuit

import (
	"context"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/directory"
	"github.com/opd-ai/go-tor/pkg/logger"
	"github.com/opd-ai/go-tor/pkg/path"
)

func TestNewBuilder(t *testing.T) {
	manager := NewManager()
	log := logger.NewDefault()

	builder := NewBuilder(manager, log)

	if builder == nil {
		t.Fatal("NewBuilder returned nil")
	}

	if builder.logger == nil {
		t.Error("Builder logger is nil")
	}

	if builder.manager == nil {
		t.Error("Builder manager is nil")
	}

	// Test with nil logger
	builder2 := NewBuilder(manager, nil)
	if builder2.logger == nil {
		t.Error("Builder should create default logger when nil is passed")
	}
}

func TestBuildCircuitSimulated(t *testing.T) {
	manager := NewManager()
	log := logger.NewDefault()
	builder := NewBuilder(manager, log)

	// Create a test path
	testPath := &path.Path{
		Guard: &directory.Relay{
			Nickname:    "TestGuard",
			Fingerprint: "GUARD123",
			Address:     "127.0.0.1",
			ORPort:      9001,
		},
		Middle: &directory.Relay{
			Nickname:    "TestMiddle",
			Fingerprint: "MIDDLE123",
			Address:     "127.0.0.1",
			ORPort:      9002,
		},
		Exit: &directory.Relay{
			Nickname:    "TestExit",
			Fingerprint: "EXIT123",
			Address:     "127.0.0.1",
			ORPort:      9003,
		},
	}

	ctx := context.Background()

	// Note: This will fail to connect since we don't have real relays running
	// But we're testing the logic flow
	_, err := builder.BuildCircuit(ctx, testPath, 2*time.Second)

	// We expect an error because there's no relay running
	if err == nil {
		t.Error("Expected error when building circuit without real relays")
	}

	// Verify a circuit was created
	if manager.Count() != 1 {
		t.Errorf("Expected 1 circuit in manager, got %d", manager.Count())
	}

	// Get the circuit and verify it failed
	circuits := manager.ListCircuits()
	if len(circuits) > 0 {
		circuit, _ := manager.GetCircuit(circuits[0])
		if circuit.GetState() != StateFailed {
			t.Errorf("Expected circuit state to be Failed, got %s", circuit.GetState())
		}
	}
}

func TestBuilderConcurrentBuilds(t *testing.T) {
	manager := NewManager()
	log := logger.NewDefault()
	builder := NewBuilder(manager, log)

	testPath := &path.Path{
		Guard: &directory.Relay{
			Nickname:    "TestGuard",
			Fingerprint: "GUARD123",
			Address:     "127.0.0.1",
			ORPort:      9001,
		},
		Middle: &directory.Relay{
			Nickname:    "TestMiddle",
			Fingerprint: "MIDDLE123",
			Address:     "127.0.0.1",
			ORPort:      9002,
		},
		Exit: &directory.Relay{
			Nickname:    "TestExit",
			Fingerprint: "EXIT123",
			Address:     "127.0.0.1",
			ORPort:      9003,
		},
	}

	ctx := context.Background()
	done := make(chan bool)

	// Try to build multiple circuits concurrently
	for i := 0; i < 3; i++ {
		go func() {
			_, _ = builder.BuildCircuit(ctx, testPath, 1*time.Second)
			done <- true
		}()
	}

	// Wait for all builds to complete
	timeout := time.After(5 * time.Second)
	for i := 0; i < 3; i++ {
		select {
		case <-done:
			// Success
		case <-timeout:
			t.Fatal("Test timed out")
		}
	}

	// All builds should have been attempted
	if manager.Count() < 1 {
		t.Error("Expected at least 1 circuit to be created")
	}
}

func TestBuildCircuitTimeout(t *testing.T) {
	manager := NewManager()
	log := logger.NewDefault()
	builder := NewBuilder(manager, log)

	testPath := &path.Path{
		Guard: &directory.Relay{
			Nickname:    "TestGuard",
			Fingerprint: "GUARD123",
			Address:     "192.0.2.1", // TEST-NET-1 (should timeout)
			ORPort:      9001,
		},
		Middle: &directory.Relay{
			Nickname:    "TestMiddle",
			Fingerprint: "MIDDLE123",
			Address:     "192.0.2.2",
			ORPort:      9002,
		},
		Exit: &directory.Relay{
			Nickname:    "TestExit",
			Fingerprint: "EXIT123",
			Address:     "192.0.2.3",
			ORPort:      9003,
		},
	}

	ctx := context.Background()

	// Use very short timeout
	_, err := builder.BuildCircuit(ctx, testPath, 100*time.Millisecond)

	if err == nil {
		t.Error("Expected error when building circuit to unreachable addresses")
	}
}

func TestBuildCircuitContextCancelled(t *testing.T) {
	manager := NewManager()
	log := logger.NewDefault()
	builder := NewBuilder(manager, log)

	testPath := &path.Path{
		Guard: &directory.Relay{
			Nickname:    "TestGuard",
			Fingerprint: "GUARD123",
			Address:     "192.0.2.1",
			ORPort:      9001,
		},
		Middle: &directory.Relay{
			Nickname:    "TestMiddle",
			Fingerprint: "MIDDLE123",
			Address:     "192.0.2.2",
			ORPort:      9002,
		},
		Exit: &directory.Relay{
			Nickname:    "TestExit",
			Fingerprint: "EXIT123",
			Address:     "192.0.2.3",
			ORPort:      9003,
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := builder.BuildCircuit(ctx, testPath, 5*time.Second)

	if err == nil {
		t.Error("Expected error when context is cancelled")
	}
}
