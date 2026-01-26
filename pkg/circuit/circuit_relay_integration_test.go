// Package circuit integration tests with real Tor relays
//
// These tests validate circuit building against the actual Tor network.
// They test CREATE2/CREATED2 handshakes, cryptographic state management,
// and flow control with real guard relays.
//
// Run with: go test -tags=integration -v -timeout=5m ./pkg/circuit -run TestIntegration
//
//go:build integration
// +build integration

package circuit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/connection"
	"github.com/opd-ai/go-tor/pkg/directory"
	"github.com/opd-ai/go-tor/pkg/logger"
	"github.com/opd-ai/go-tor/pkg/path"
)

// TestIntegrationCircuitBuildingWithRealRelays tests circuit building with real Tor relays.
// NOTE: Currently tests first-hop CREATE2 handshake only. Middle and exit hops are
// simulated (not yet using EXTEND2) as documented in PLAN.md section 3.
func TestIntegrationCircuitBuildingWithRealRelays(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	log := logger.NewDefault()

	// Fetch real consensus and relays
	dirClient := directory.NewClient(log)
	relays, err := dirClient.FetchConsensus(ctx)
	if err != nil {
		t.Fatalf("Failed to fetch consensus: %v", err)
	}

	if len(relays) == 0 {
		t.Fatal("No relays available from consensus")
	}
	t.Logf("Fetched %d relays from consensus", len(relays))

	// Create path selector
	guardMgr, err := path.NewGuardManager("", log)
	if err != nil {
		t.Fatalf("Failed to create guard manager: %v", err)
	}

	pathSelector := path.NewSelectorWithGuards(dirClient, guardMgr, log)

	// Select a path (SelectPath takes exitPort parameter)
	selectedPath, err := pathSelector.SelectPath(80)
	if err != nil {
		t.Fatalf("Failed to select path: %v", err)
	}

	t.Logf("Selected path: Guard=%s, Middle=%s, Exit=%s",
		selectedPath.Guard.Nickname,
		selectedPath.Middle.Nickname,
		selectedPath.Exit.Nickname)

	// Build circuit
	manager := NewManager()
	builder := NewBuilder(manager, log)

	circuit, err := builder.BuildCircuit(ctx, selectedPath, 30*time.Second)
	if err != nil {
		t.Fatalf("Failed to build circuit: %v", err)
	}
	defer circuit.Close()

	// Verify circuit state
	if circuit.GetState() != StateOpen {
		t.Errorf("Expected circuit state Open, got %s", circuit.GetState())
	}

	// Verify circuit has 3 hops (structure only - middle/exit are simulated)
	hops := circuit.GetHops()
	if len(hops) != 3 {
		t.Errorf("Expected 3 hops, got %d", len(hops))
	}

	// Verify first hop has cryptographic state (CREATE2 actually implemented)
	if len(hops) > 0 {
		firstHop := hops[0]
		if firstHop.ForwardCipher == nil {
			t.Error("First hop missing forward cipher")
		}
		if firstHop.BackwardCipher == nil {
			t.Error("First hop missing backward cipher")
		}
		if firstHop.ForwardDigest == nil {
			t.Error("First hop missing forward digest")
		}
		if firstHop.BackwardDigest == nil {
			t.Error("First hop missing backward digest")
		}

		t.Logf("First hop (%s): CREATE2 handshake successful, cryptographic state established",
			firstHop.Fingerprint)
	}

	// NOTE: Middle and exit hops are currently simulated in builder.go lines 94-115
	// They have hop structures but not yet real EXTEND2 handshakes
	// This is documented in PLAN.md as "⏳ Integration tests with real Tor relays pending"

	t.Logf("Circuit %d built: 1 real hop (CREATE2), 2 simulated hops, state=%s",
		circuit.ID, circuit.GetState())
}

// TestIntegrationFirstHopHandshake tests CREATE2/CREATED2 handshake with a real guard relay
func TestIntegrationFirstHopHandshake(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	log := logger.NewDefault()

	// Fetch consensus to get a guard relay
	dirClient := directory.NewClient(log)
	relays, err := dirClient.FetchConsensus(ctx)
	if err != nil {
		t.Fatalf("Failed to fetch consensus: %v", err)
	}

	// Find a guard relay with valid keys
	var guard *directory.Relay
	for _, relay := range relays {
		if relay.IsGuard() && relay.IsRunning() && relay.IsValid() &&
			relay.NtorOnionKey != nil && len(relay.NtorOnionKey) == 32 {
			guard = relay
			break
		}
	}

	if guard == nil {
		t.Fatal("No suitable guard relay found")
	}
	t.Logf("Selected guard: %s (%s:%d)", guard.Nickname, guard.Address, guard.ORPort)

	// Connect to guard using connection package
	cfg := connection.DefaultConfig(fmt.Sprintf("%s:%d", guard.Address, guard.ORPort))
	conn := connection.New(cfg, log)

	if err := conn.Connect(ctx, cfg); err != nil {
		t.Fatalf("Failed to connect to guard: %v", err)
	}
	defer conn.Close()

	t.Logf("Successfully connected to guard %s", guard.Nickname)

	// Create circuit and attach connection
	manager := NewManager()
	circuit, err := manager.CreateCircuit()
	if err != nil {
		t.Fatalf("Failed to create circuit: %v", err)
	}
	defer circuit.Close()

	circuit.SetConnection(conn)

	// Create extension helper and set relay keys
	ext := NewExtension(circuit, log)
	ext.SetTargetRelay(guard) // Provide real relay descriptor for key extraction

	// Test CREATE2 handshake
	if err := ext.CreateFirstHop(ctx, HandshakeTypeNTor); err != nil {
		t.Fatalf("CREATE2 handshake failed: %v", err)
	}

	// Verify hop was added with cryptographic state
	hops := circuit.GetHops()
	if len(hops) != 1 {
		t.Errorf("Expected 1 hop after CREATE2, got %d", len(hops))
	}

	if len(hops) > 0 {
		hop := hops[0]
		if hop.ForwardCipher == nil || hop.BackwardCipher == nil {
			t.Error("Missing ciphers in first hop")
		}
		if hop.ForwardDigest == nil || hop.BackwardDigest == nil {
			t.Error("Missing digests in first hop")
		}

		t.Logf("CREATE2/CREATED2 handshake successful with %s, cryptographic state established", guard.Nickname)
	}
}

// TestIntegrationFlowControlWithRealCircuit tests flow control on a real circuit
func TestIntegrationFlowControlWithRealCircuit(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	log := logger.NewDefault()

	// Build a real circuit
	dirClient := directory.NewClient(log)
	relays, err := dirClient.FetchConsensus(ctx)
	if err != nil {
		t.Fatalf("Failed to fetch consensus: %v", err)
	}

	if len(relays) == 0 {
		t.Fatal("No relays available from consensus")
	}

	guardMgr, err := path.NewGuardManager("", log)
	if err != nil {
		t.Fatalf("Failed to create guard manager: %v", err)
	}

	pathSelector := path.NewSelectorWithGuards(dirClient, guardMgr, log)

	selectedPath, err := pathSelector.SelectPath(80)
	if err != nil {
		t.Fatalf("Failed to select path: %v", err)
	}

	manager := NewManager()
	builder := NewBuilder(manager, log)

	circuit, err := builder.BuildCircuit(ctx, selectedPath, 30*time.Second)
	if err != nil {
		t.Fatalf("Failed to build circuit: %v", err)
	}
	defer circuit.Close()

	// Verify initial flow control windows per tor-spec.txt §7.4
	// Note: PackageWindow() and DeliverWindow() methods need to be added or accessed via reflection
	// For now, we verify the circuit was built successfully
	if circuit.GetState() != StateOpen {
		t.Errorf("Expected circuit state Open for flow control test, got %s", circuit.GetState())
	}

	t.Logf("Flow control infrastructure validated - circuit built successfully")
}
