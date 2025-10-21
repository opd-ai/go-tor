// Package bine provides unit tests for the bine wrapper.
package bine

import (
	"context"
	"testing"
	"time"
)

func TestConnect(t *testing.T) {
	// This test verifies basic client creation
	// Note: This will actually start a Tor client, so it may be slow
	t.Skip("Skipping integration test - requires network access")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	client, err := ConnectWithOptionsContext(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	if !client.IsReady() {
		t.Error("Client should be ready after connect")
	}

	addr := client.ProxyAddr()
	if addr == "" {
		t.Error("ProxyAddr should not be empty")
	}

	url := client.ProxyURL()
	if url == "" {
		t.Error("ProxyURL should not be empty")
	}
}

func TestConnectWithOptions(t *testing.T) {
	t.Skip("Skipping integration test - requires network access")

	opts := &Options{
		LogLevel:       "info",
		StartupTimeout: 120 * time.Second,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	client, err := ConnectWithOptionsContext(ctx, opts)
	if err != nil {
		t.Fatalf("Failed to connect with options: %v", err)
	}
	defer client.Close()

	if !client.IsReady() {
		t.Error("Client should be ready")
	}
}

func TestHTTPClient(t *testing.T) {
	t.Skip("Skipping integration test - requires network access")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	client, err := ConnectWithOptionsContext(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	httpClient, err := client.HTTPClient()
	if err != nil {
		t.Fatalf("Failed to create HTTP client: %v", err)
	}

	if httpClient == nil {
		t.Error("HTTP client should not be nil")
	}
}

func TestCreateHiddenServiceWithoutBine(t *testing.T) {
	t.Skip("Skipping integration test - requires network access")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Create client without bine enabled
	client, err := ConnectWithOptionsContext(ctx, &Options{EnableBine: false})
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	// Attempting to create hidden service should fail
	_, err = client.CreateHiddenService(ctx, 80)
	if err == nil {
		t.Error("Should fail when bine is not enabled")
	}
}

func TestOptionsDefaults(t *testing.T) {
	opts := &Options{}

	// Test that defaults are applied in ConnectWithOptionsContext
	// We can't actually test the connection without network, but we can test the struct
	if opts.StartupTimeout != 0 {
		t.Error("Default timeout should be 0 before Connect")
	}
}
