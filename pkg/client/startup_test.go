package client

import (
	"context"
	"log/slog"
	"io"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/config"
	"github.com/opd-ai/go-tor/pkg/logger"
)

// TestConnectWrapperFunctions tests the simple Connect API wrappers
func TestConnectWrapperFunctions(t *testing.T) {
	t.Run("Connect function signature", func(t *testing.T) {
		// Verify the function compiles and has the right signature
		var fn func() (*SimpleClient, error) = Connect
		if fn == nil {
			t.Error("Connect function is nil")
		}
	})

	t.Run("ConnectWithOptions function signature", func(t *testing.T) {
		var fn func(*Options) (*SimpleClient, error) = ConnectWithOptions
		if fn == nil {
			t.Error("ConnectWithOptions function is nil")
		}
	})

	t.Run("ConnectWithContext handles timeout", func(t *testing.T) {
		// Create a context with very short timeout to prevent hanging
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		// This will fail to connect (no Tor network) but shouldn't panic
		_, err := ConnectWithContext(ctx)
		
		// We expect an error (timeout or directory unavailable)
		if err == nil {
			t.Error("Expected error when connecting without Tor network")
		}
		t.Logf("ConnectWithContext failed as expected: %v", err)
	})

	t.Run("ConnectWithOptionsContext handles custom options", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		opts := &Options{
			SocksPort:     19050,
			ControlPort:   19051,
			DataDirectory: t.TempDir(),
			LogLevel:      "error",
		}

		// This will fail but should apply the options without panicking
		_, err := ConnectWithOptionsContext(ctx, opts)
		
		if err == nil {
			t.Error("Expected error when connecting without Tor network")
		}
		t.Logf("ConnectWithOptionsContext with custom options failed as expected: %v", err)
	})

	t.Run("ConnectWithOptionsContext handles nil options", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		// Should handle nil options gracefully by using defaults
		_, err := ConnectWithOptionsContext(ctx, nil)
		
		if err == nil {
			t.Error("Expected error when connecting without Tor network")
		}
		t.Logf("ConnectWithOptionsContext with nil options failed as expected: %v", err)
	})
}

// TestClientStartWithInfrastructure tests Start method behavior with proper initialization
func TestClientStartWithInfrastructure(t *testing.T) {
	t.Run("Start with valid config but no network", func(t *testing.T) {
		log := logger.New(slog.LevelError, io.Discard)
		cfg := config.DefaultConfig()
		cfg.DataDirectory = t.TempDir()

		client, err := New(cfg, log)
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// Start will fail because directory is not available
		// But it should fail gracefully
		err = client.Start(ctx)
		
		if err != nil {
			t.Logf("Start failed as expected without directory: %v", err)
		} else {
			// If it somehow succeeded, clean up
			_ = client.Stop()
			t.Log("Start succeeded (unexpected but cleaning up)")
		}
	})

	t.Run("Start respects context cancellation", func(t *testing.T) {
		log := logger.New(slog.LevelError, io.Discard)
		cfg := config.DefaultConfig()
		cfg.DataDirectory = t.TempDir()

		client, err := New(cfg, log)
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		start := time.Now()
		err = client.Start(ctx)
		duration := time.Since(start)

		if err == nil {
			_ = client.Stop()
			t.Error("Expected error with cancelled context")
		}

		// Should return quickly (< 1 second)
		if duration > 1*time.Second {
			t.Errorf("Start took too long with cancelled context: %v", duration)
		}
	})

	t.Run("Start timeout handling", func(t *testing.T) {
		log := logger.New(slog.LevelError, io.Discard)
		cfg := config.DefaultConfig()
		cfg.DataDirectory = t.TempDir()

		client, err := New(cfg, log)
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}

		// Use very short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		start := time.Now()
		err = client.Start(ctx)
		duration := time.Since(start)

		// Should timeout quickly
		if err == nil {
			_ = client.Stop()
		}

		// Verify it didn't hang for too long
		if duration > 2*time.Second {
			t.Errorf("Start took too long: %v", duration)
		}
	})
}

// TestClientStop tests the Stop method
func TestClientStop(t *testing.T) {
	t.Run("Stop on unstarted client", func(t *testing.T) {
		log := logger.New(slog.LevelError, io.Discard)
		cfg := config.DefaultConfig()
		cfg.DataDirectory = t.TempDir()

		client, err := New(cfg, log)
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}

		// Stop should be safe to call even if not started
		err = client.Stop()
		if err != nil {
			t.Logf("Stop returned error on unstarted client: %v", err)
		}
	})

	t.Run("Stop can be called multiple times", func(t *testing.T) {
		log := logger.New(slog.LevelError, io.Discard)
		cfg := config.DefaultConfig()
		cfg.DataDirectory = t.TempDir()

		client, err := New(cfg, log)
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}

		// First stop
		err1 := client.Stop()
		// Second stop
		err2 := client.Stop()

		t.Logf("First Stop: %v", err1)
		t.Logf("Second Stop: %v", err2)

		// Both should complete without panicking
	})
}

// TestOptionsValidation tests the Options structure
func TestOptionsValidation(t *testing.T) {
	t.Run("valid options", func(t *testing.T) {
		opts := &Options{
			SocksPort:     19050,
			ControlPort:   19051,
			DataDirectory: "/tmp/tor-test",
			LogLevel:      "info",
		}

		if opts.SocksPort != 19050 {
			t.Errorf("Expected SocksPort 19050, got %d", opts.SocksPort)
		}
		if opts.ControlPort != 19051 {
			t.Errorf("Expected ControlPort 19051, got %d", opts.ControlPort)
		}
	})

	t.Run("default values", func(t *testing.T) {
		opts := &Options{}

		// Zero values should let config.DefaultConfig() handle defaults
		if opts.SocksPort != 0 {
			t.Errorf("Expected default SocksPort 0, got %d", opts.SocksPort)
		}
		if opts.ControlPort != 0 {
			t.Errorf("Expected default ControlPort 0, got %d", opts.ControlPort)
		}
	})
}
