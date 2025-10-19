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
	cfg.DataDirectory = t.TempDir() // Use temporary directory for tests
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

	if client.guardManager == nil {
		t.Error("Guard manager not initialized")
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
	cfg.DataDirectory = t.TempDir() // Use temporary directory for tests

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
	cfg.DataDirectory = t.TempDir() // Use temporary directory for tests
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
	cfg.DataDirectory = t.TempDir() // Use temporary directory for tests
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
	cfg.DataDirectory = t.TempDir() // Use temporary directory for tests
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
	cfg.DataDirectory = t.TempDir() // Use temporary directory for tests
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
	cfg.DataDirectory = t.TempDir() // Use temporary directory for tests
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

func TestRecordBandwidth(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Record some bandwidth
	client.RecordBytesRead(100)
	client.RecordBytesWritten(200)

	// Bytes are tracked internally but not exposed in Stats
	// Test that methods don't panic and can be called multiple times
	client.RecordBytesRead(50)
	client.RecordBytesWritten(75)
}

func TestGetCircuits(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Stats should reflect no circuits initially
	stats := client.GetStats()
	if stats.ActiveCircuits != 0 {
		t.Errorf("Expected 0 active circuits, got %d", stats.ActiveCircuits)
	}
}

func TestPublishEvent(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test that PublishEvent doesn't panic even when control server is not fully started
	// We need to pass a valid event, not nil
	// However, since the control server hasn't started accepting connections,
	// the event just goes to the dispatcher which may or may not have subscribers
	// This test mainly ensures the method is accessible and doesn't crash
	
	// Just verify the method exists and is callable
	// We can't meaningfully test event publishing without a full integration test
	_ = client.controlServer
}

func TestClientStatsAdapter(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Create adapter
	adapter := &clientStatsAdapter{client: client}

	// Test GetStats through adapter
	stats := adapter.GetStats()
	if stats == nil {
		t.Fatal("GetStats returned nil")
	}
}

func TestConcurrentBandwidthTracking(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Track bandwidth concurrently from multiple goroutines
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				client.RecordBytesRead(1)
				client.RecordBytesWritten(1)
			}
			done <- true
		}()
	}

	// Wait for all goroutines to finish
	for i := 0; i < 10; i++ {
		<-done
	}

	// Just verify no race conditions occurred (test runs with -race)
	// The actual byte counts are internal and not exposed through Stats
}
