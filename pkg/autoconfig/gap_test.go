// Package autoconfig_test demonstrates the gap in automatic port selection
package autoconfig_test

import (
	"net"
	"testing"

	"github.com/opd-ai/go-tor/pkg/autoconfig"
	"github.com/opd-ai/go-tor/pkg/config"
)

// TestPortSelectionGap demonstrates Gap #2 from IMPLEMENTATION_GAP_AUDIT.md
// This test shows that FindAvailablePort exists but is not used in zero-config mode
func TestPortSelectionGap(t *testing.T) {
	// Reserve the default SOCKS port to simulate it being in use
	listener, err := net.Listen("tcp", "127.0.0.1:9050")
	if err != nil {
		t.Fatalf("Failed to reserve port 9050: %v", err)
	}
	defer listener.Close()

	t.Log("Port 9050 is now in use by test")

	// Test that FindAvailablePort works correctly
	availablePort := autoconfig.FindAvailablePort(9050)
	if availablePort == 9050 {
		t.Error("FindAvailablePort returned 9050 even though it's in use")
	}
	t.Logf("FindAvailablePort correctly found alternative: %d", availablePort)

	// However, DefaultConfig doesn't use FindAvailablePort
	cfg := config.DefaultConfig()
	t.Logf("DefaultConfig SocksPort: %d (hardcoded, doesn't check availability)", cfg.SocksPort)

	// This demonstrates the gap: the config always uses 9050
	// even though 9050 is already in use and FindAvailablePort could find an alternative
	if cfg.SocksPort != 9050 {
		t.Errorf("Expected DefaultConfig to use hardcoded 9050, got %d", cfg.SocksPort)
	}

	// In a true zero-config mode, we'd expect:
	// cfg.SocksPort = autoconfig.FindAvailablePort(9050)
	// which would return 9051 or another available port

	t.Log("Gap confirmed: DefaultConfig doesn't use FindAvailablePort")
	t.Log("See IMPLEMENTATION_GAP_AUDIT.md Gap #2 for details")
}

// TestCircuitTimeoutGap demonstrates Gap #1 from IMPLEMENTATION_GAP_AUDIT.md
// This test shows the configuration timeout is not used in circuit building
func TestCircuitTimeoutGap(t *testing.T) {
	cfg := config.DefaultConfig()
	
	t.Logf("Config CircuitBuildTimeout: %v", cfg.CircuitBuildTimeout)
	
	// The documentation states "< 5 seconds (95th percentile)" 
	// but the default is 60 seconds
	if cfg.CircuitBuildTimeout.Seconds() != 60 {
		t.Errorf("Expected default CircuitBuildTimeout to be 60s, got %v", cfg.CircuitBuildTimeout)
	}

	// Furthermore, pkg/client/client.go:264 hardcodes 30 seconds:
	// circ, err := builder.BuildCircuit(ctx, selectedPath, 30*time.Second)
	// 
	// This means:
	// 1. The documented target is 5 seconds
	// 2. The config default is 60 seconds  
	// 3. The actual implementation uses 30 seconds (hardcoded)
	// 
	// None of these align!

	t.Log("Gap confirmed: Circuit timeout values inconsistent")
	t.Log("  - README target: < 5 seconds (95th percentile)")
	t.Log("  - Config default: 60 seconds")
	t.Log("  - Implementation: 30 seconds (hardcoded)")
	t.Log("See IMPLEMENTATION_GAP_AUDIT.md Gap #1 for details")
}
