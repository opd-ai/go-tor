package client

import (
	"context"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/config"
	"github.com/opd-ai/go-tor/pkg/logger"
)

func TestNew(t *testing.T) {
	cfg := config.DefaultConfig()
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	if client == nil {
		t.Fatal("Client is nil")
	}

	if client.config != cfg {
		t.Error("Config not set correctly")
	}

	if client.logger == nil {
		t.Error("Logger not initialized")
	}

	if client.directory == nil {
		t.Error("Directory client not initialized")
	}

	if client.circuitMgr == nil {
		t.Error("Circuit manager not initialized")
	}

	if client.socksServer == nil {
		t.Error("SOCKS server not initialized")
	}
}

func TestNewWithNilConfig(t *testing.T) {
	log := logger.NewDefault()

	_, err := New(nil, log)
	if err == nil {
		t.Fatal("Expected error with nil config")
	}
}

func TestNewWithNilLogger(t *testing.T) {
	cfg := config.DefaultConfig()

	client, err := New(cfg, nil)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	if client.logger == nil {
		t.Error("Logger should be initialized with default")
	}
}

func TestGetStats(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.SocksPort = 9999
	cfg.ControlPort = 9998
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	stats := client.GetStats()
	if stats.SocksPort != 9999 {
		t.Errorf("Expected SocksPort 9999, got %d", stats.SocksPort)
	}

	if stats.ControlPort != 9998 {
		t.Errorf("Expected ControlPort 9998, got %d", stats.ControlPort)
	}

	if stats.ActiveCircuits != 0 {
		t.Errorf("Expected 0 active circuits, got %d", stats.ActiveCircuits)
	}
}

func TestStopWithoutStart(t *testing.T) {
	cfg := config.DefaultConfig()
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Should not panic
	err = client.Stop()
	if err != nil {
		t.Errorf("Stop returned error: %v", err)
	}
}

func TestStopMultipleTimes(t *testing.T) {
	cfg := config.DefaultConfig()
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// First stop
	err = client.Stop()
	if err != nil {
		t.Errorf("First stop returned error: %v", err)
	}

	// Second stop should be no-op
	err = client.Stop()
	if err != nil {
		t.Errorf("Second stop returned error: %v", err)
	}
}

func TestMergeContexts(t *testing.T) {
	cfg := config.DefaultConfig()
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test parent context cancellation
	parentCtx, parentCancel := context.WithCancel(context.Background())
	childCtx, childCancel := context.WithCancel(context.Background())
	defer childCancel()

	merged := client.mergeContexts(parentCtx, childCtx)

	// Cancel parent
	parentCancel()

	// Merged should be cancelled
	select {
	case <-merged.Done():
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Error("Merged context should be cancelled when parent is cancelled")
	}
}

func TestMergeContextsChildCancel(t *testing.T) {
	cfg := config.DefaultConfig()
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test child context cancellation
	parentCtx, parentCancel := context.WithCancel(context.Background())
	defer parentCancel()
	childCtx, childCancel := context.WithCancel(context.Background())

	merged := client.mergeContexts(parentCtx, childCtx)

	// Cancel child
	childCancel()

	// Merged should be cancelled
	select {
	case <-merged.Done():
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Error("Merged context should be cancelled when child is cancelled")
	}
}
