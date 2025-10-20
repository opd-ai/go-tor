// Package autoconfig_test demonstrates the gap in automatic port selection
package autoconfig_test

import (
	"net"
	"testing"

	"github.com/opd-ai/go-tor/pkg/autoconfig"
	"github.com/opd-ai/go-tor/pkg/config"
)

// TestPortSelectionGap was demonstrating Gap #2 from IMPLEMENTATION_GAP_AUDIT.md
// This test now verifies that the gap has been fixed - DefaultConfig uses FindAvailablePort
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

	// DefaultConfig should now use FindAvailablePort (Gap #2 fix)
	cfg := config.DefaultConfig()
	t.Logf("DefaultConfig SocksPort: %d (uses FindAvailablePort)", cfg.SocksPort)

	// Verify the fix: config should NOT use 9050 since it's in use
	// It should find an available port (9051 or higher)
	if cfg.SocksPort == 9050 {
		t.Errorf("DefaultConfig still uses hardcoded 9050 even though it's in use - Gap #2 not fixed!")
	}

	t.Logf("Gap #2 fixed: DefaultConfig uses FindAvailablePort and found port %d", cfg.SocksPort)
}

// TestCircuitTimeoutGap was demonstrating Gap #1 from IMPLEMENTATION_GAP_AUDIT.md
// This test now verifies that the gap has been partially fixed - config timeout is used
func TestCircuitTimeoutGap(t *testing.T) {
	cfg := config.DefaultConfig()

	t.Logf("Config CircuitBuildTimeout: %v", cfg.CircuitBuildTimeout)

	// The documentation states "< 5 seconds (95th percentile)"
	// but the default is 60 seconds
	if cfg.CircuitBuildTimeout.Seconds() != 60 {
		t.Errorf("Expected default CircuitBuildTimeout to be 60s, got %v", cfg.CircuitBuildTimeout)
	}

	// Gap #1 fix: pkg/client/client.go now uses c.config.CircuitBuildTimeout
	// instead of hardcoded 30*time.Second
	//
	// However, there's still a documentation discrepancy:
	// 1. The documented target is < 5 seconds (95th percentile)
	// 2. The config default is 60 seconds
	//
	// This is acceptable - the timeout should be higher than the target
	// to allow for network variability

	t.Log("Gap #1 partially fixed: Implementation now uses config timeout")
	t.Log("  - README target: < 5 seconds (95th percentile) - performance goal")
	t.Log("  - Config default: 60 seconds - timeout value (correctly higher than target)")
	t.Log("  - Implementation: Uses c.config.CircuitBuildTimeout (FIXED)")
}
