package client

import (
	"context"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/circuit"
	"github.com/opd-ai/go-tor/pkg/config"
	"github.com/opd-ai/go-tor/pkg/logger"
)

func TestSimpleConnect(t *testing.T) {
	// Note: This test doesn't actually connect to Tor network
	// It tests the API structure, not the network functionality

	// Test that Connect creates a client
	// In real usage this would connect to Tor, but we can't do full integration here
	t.Run("Connect API exists", func(t *testing.T) {
		// Just verify the function signature compiles
		_ = Connect
		_ = ConnectWithContext
		_ = ConnectWithOptions
		_ = ConnectWithOptionsContext
	})
}

func TestSimpleClientOptions(t *testing.T) {
	t.Run("valid options", func(t *testing.T) {
		opts := &Options{
			SocksPort:     19050,
			ControlPort:   19051,
			DataDirectory: t.TempDir(),
			LogLevel:      "info",
		}

		if opts.SocksPort != 19050 {
			t.Errorf("Expected SocksPort 19050, got %d", opts.SocksPort)
		}
	})

	t.Run("default options", func(t *testing.T) {
		opts := &Options{}

		// Default values should be zero, letting config.DefaultConfig() handle them
		if opts.SocksPort != 0 {
			t.Errorf("Expected default SocksPort 0, got %d", opts.SocksPort)
		}
	})
}

func TestSimpleClientContextCancellation(t *testing.T) {
	// Test that context cancellation is handled
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// This should fail quickly due to timeout/cancellation
	// We expect this to fail during network operations
	_, err := ConnectWithContext(ctx)
	if err == nil {
		t.Error("Expected error with short timeout context")
	}
	t.Logf("Got expected error: %v", err)
}

func TestSimpleClientMethodsExist(t *testing.T) {
	// Verify the SimpleClient has all required methods
	// This is a compile-time check more than a runtime test

	var sc *SimpleClient
	if sc == nil {
		// Expected - we're just checking method signatures exist
	}

	// These should compile
	_ = sc.Close
	_ = sc.ProxyURL
	_ = sc.ProxyAddr
	_ = sc.IsReady
	_ = sc.WaitUntilReady
	_ = sc.Stats
}

func TestWaitUntilReadyTimeout(t *testing.T) {
	// Test timeout behavior of WaitUntilReady
	// We'll create a mock SimpleClient that's never ready

	// Create a minimal config
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()

	log := logger.NewDefault()

	// Create a client with proper initialization
	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Stop()

	// Client has no circuits yet, so it's not ready
	sc := &SimpleClient{
		client: client,
		logger: log,
	}

	// This should timeout quickly
	err = sc.WaitUntilReady(100 * time.Millisecond)
	if err == nil {
		t.Error("Expected timeout error")
	}

	if err.Error() != "timeout waiting for Tor client to be ready" {
		t.Errorf("Expected timeout error, got: %v", err)
	}
}

func TestIsReady(t *testing.T) {
	t.Run("not ready with no circuits", func(t *testing.T) {
		cfg := config.DefaultConfig()
		cfg.DataDirectory = t.TempDir()
		log := logger.NewDefault()

		client, err := New(cfg, log)
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}
		defer client.Stop()

		sc := &SimpleClient{
			client: client,
			logger: log,
		}

		if sc.IsReady() {
			t.Error("Expected IsReady() to return false with no circuits")
		}
	})

	t.Run("ready with circuits", func(t *testing.T) {
		cfg := config.DefaultConfig()
		cfg.DataDirectory = t.TempDir()
		log := logger.NewDefault()

		client, err := New(cfg, log)
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}
		defer client.Stop()

		// Add a mock circuit
		circ := &circuit.Circuit{
			ID: 1,
		}
		client.circuitsMu.Lock()
		client.circuits = append(client.circuits, circ)
		client.circuitsMu.Unlock()

		sc := &SimpleClient{
			client: client,
			logger: log,
		}

		if !sc.IsReady() {
			t.Error("Expected IsReady() to return true with circuits")
		}
	})
}

func TestProxyMethods(t *testing.T) {
	// Test proxy URL/address generation
	cfg := config.DefaultConfig()
	cfg.SocksPort = 19050
	cfg.DataDirectory = t.TempDir()

	log := logger.NewDefault()
	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Stop()

	sc := &SimpleClient{
		client: client,
		logger: log,
	}

	proxyURL := sc.ProxyURL()
	expectedURL := "socks5://127.0.0.1:19050"
	if proxyURL != expectedURL {
		t.Errorf("Expected ProxyURL %s, got %s", expectedURL, proxyURL)
	}

	proxyAddr := sc.ProxyAddr()
	expectedAddr := "127.0.0.1:19050"
	if proxyAddr != expectedAddr {
		t.Errorf("Expected ProxyAddr %s, got %s", expectedAddr, proxyAddr)
	}
}
