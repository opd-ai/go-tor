// Package client integration tests
//go:build integration
// +build integration

package client

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/config"
	"github.com/opd-ai/go-tor/pkg/logger"
)

// TestIntegrationClientLifecycle tests the complete client lifecycle
// Run with: go test -tags=integration -v ./pkg/client
func TestIntegrationClientLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create a test client configuration
	cfg := config.DefaultConfig()
	cfg.SocksPort = 19050 // Use non-standard port to avoid conflicts
	cfg.ControlPort = 19051
	cfg.LogLevel = "info"

	log := logger.NewDefault()

	// Create client
	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Start client in background
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	startErr := make(chan error, 1)
	go func() {
		startErr <- client.Start(ctx)
	}()

	// Wait for client to be ready
	time.Sleep(30 * time.Second)

	// Verify client is running
	stats := client.GetStats()
	if stats.SocksPort != cfg.SocksPort {
		t.Errorf("Expected SOCKS port %d, got %d", cfg.SocksPort, stats.SocksPort)
	}

	// Stop client
	if err := client.Stop(); err != nil {
		t.Errorf("Failed to stop client: %v", err)
	}

	// Wait for start goroutine to complete
	select {
	case err := <-startErr:
		if err != nil && ctx.Err() == nil {
			t.Errorf("Start returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Log("Start goroutine did not complete in time")
	}
}

// TestIntegrationSimpleClient tests the simplified client API
func TestIntegrationSimpleClient(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create client with default settings
	client, err := Connect()
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	// Wait for readiness
	err = client.WaitUntilReady(90 * time.Second)
	if err != nil {
		t.Fatalf("Client did not become ready: %v", err)
	}

	// Verify proxy URL format
	proxyURL := client.ProxyURL()
	if proxyURL == "" {
		t.Error("ProxyURL() returned empty string")
	}
	if proxyURL[:8] != "socks5://" {
		t.Errorf("ProxyURL() should start with 'socks5://', got: %s", proxyURL)
	}

	// Verify proxy address format
	proxyAddr := client.ProxyAddr()
	if proxyAddr == "" {
		t.Error("ProxyAddr() returned empty string")
	}

	// Check stats
	stats := client.Stats()
	if stats.ActiveCircuits == 0 {
		t.Error("Expected at least one active circuit")
	}

	t.Logf("Client ready with %d active circuits", stats.ActiveCircuits)
}

// TestIntegrationHTTPProxy tests HTTP proxy functionality
func TestIntegrationHTTPProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create Tor client
	torClient, err := Connect()
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer torClient.Close()

	// Wait for readiness
	err = torClient.WaitUntilReady(90 * time.Second)
	if err != nil {
		t.Fatalf("Client did not become ready: %v", err)
	}

	// Create HTTP client with proxy
	proxyURL, err := url.Parse(torClient.ProxyURL())
	if err != nil {
		t.Fatalf("Failed to parse proxy URL: %v", err)
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
		Timeout: 30 * time.Second,
	}

	// Test connectivity through Tor
	resp, err := httpClient.Get("https://check.torproject.org")
	if err != nil {
		t.Fatalf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Read a bit of the response to verify connectivity
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	if len(body) == 0 {
		t.Error("Response body is empty")
	}

	t.Logf("Successfully connected through Tor, response length: %d bytes", len(body))
}

// TestIntegrationMultipleClients tests running multiple clients simultaneously
func TestIntegrationMultipleClients(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	const numClients = 2

	clients := make([]*SimpleClient, numClients)
	var err error

	// Create multiple clients with different ports
	for i := 0; i < numClients; i++ {
		opts := &Options{
			SocksPort:   20050 + i,
			ControlPort: 20150 + i,
			LogLevel:    "warn",
		}

		clients[i], err = ConnectWithOptions(opts)
		if err != nil {
			t.Fatalf("Failed to create client %d: %v", i, err)
		}
		defer clients[i].Close()
	}

	// Wait for all clients to be ready
	for i, client := range clients {
		err := client.WaitUntilReady(90 * time.Second)
		if err != nil {
			t.Errorf("Client %d did not become ready: %v", i, err)
		}
	}

	// Verify all clients are independent
	for i, client := range clients {
		stats := client.Stats()
		if stats.ActiveCircuits == 0 {
			t.Errorf("Client %d has no active circuits", i)
		}
		t.Logf("Client %d: %d active circuits", i, stats.ActiveCircuits)
	}
}

// TestIntegrationClientRestart tests stopping and restarting a client
func TestIntegrationClientRestart(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := config.DefaultConfig()
	cfg.SocksPort = 21050
	cfg.ControlPort = 21051

	log := logger.NewDefault()

	// First run
	client1, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create first client: %v", err)
	}

	ctx1, cancel1 := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel1()

	go func() {
		_ = client1.Start(ctx1)
	}()

	time.Sleep(30 * time.Second)

	stats1 := client1.GetStats()
	if stats1.ActiveCircuits == 0 {
		t.Error("First client has no active circuits")
	}

	// Stop first client
	if err := client1.Stop(); err != nil {
		t.Errorf("Failed to stop first client: %v", err)
	}

	time.Sleep(2 * time.Second)

	// Second run with same configuration
	client2, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create second client: %v", err)
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel2()

	go func() {
		_ = client2.Start(ctx2)
	}()

	time.Sleep(30 * time.Second)

	stats2 := client2.GetStats()
	if stats2.ActiveCircuits == 0 {
		t.Error("Second client has no active circuits")
	}

	// Stop second client
	if err := client2.Stop(); err != nil {
		t.Errorf("Failed to stop second client: %v", err)
	}

	t.Logf("First run: %d circuits, Second run: %d circuits",
		stats1.ActiveCircuits, stats2.ActiveCircuits)
}

// TestIntegrationProxyConnection tests actual SOCKS proxy functionality
func TestIntegrationProxyConnection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create and start client
	client, err := Connect()
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	// Wait for readiness
	err = client.WaitUntilReady(90 * time.Second)
	if err != nil {
		t.Fatalf("Client did not become ready: %v", err)
	}

	// Create HTTP client with SOCKS proxy
	proxyURL, err := url.Parse(client.ProxyURL())
	if err != nil {
		t.Fatalf("Failed to parse proxy URL: %v", err)
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
		Timeout: 30 * time.Second,
	}

	// Make request through proxy
	resp, err := httpClient.Get("https://check.torproject.org")
	if err != nil {
		t.Fatalf("Failed to make request through proxy: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	if len(body) == 0 {
		t.Error("Response body is empty")
	}

	t.Logf("Successfully proxied request, response length: %d bytes", len(body))
}

// TestIntegrationContextCancellation tests context cancellation handling
func TestIntegrationContextCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := config.DefaultConfig()
	cfg.SocksPort = 22050
	cfg.ControlPort = 22051

	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Create a context that we'll cancel
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)

	// Start client
	startErr := make(chan error, 1)
	go func() {
		startErr <- client.Start(ctx)
	}()

	// Let it start
	time.Sleep(10 * time.Second)

	// Cancel context
	cancel()

	// Wait for start to complete
	select {
	case err := <-startErr:
		if err != nil && err != context.Canceled {
			t.Logf("Start returned error (expected): %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Error("Start did not respond to context cancellation")
	}

	// Clean stop
	if err := client.Stop(); err != nil {
		t.Logf("Stop returned error: %v", err)
	}
}

// TestIntegrationClientStats tests statistics gathering
func TestIntegrationClientStats(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client, err := Connect()
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	// Wait for readiness
	err = client.WaitUntilReady(90 * time.Second)
	if err != nil {
		t.Fatalf("Client did not become ready: %v", err)
	}

	// Get stats
	stats := client.Stats()

	// Validate stats
	if stats.SocksPort == 0 {
		t.Error("SocksPort should not be 0")
	}

	if stats.ControlPort == 0 {
		t.Error("ControlPort should not be 0")
	}

	if stats.ActiveCircuits == 0 {
		t.Error("ActiveCircuits should not be 0")
	}

	t.Logf("Stats: SOCKS=%d, Control=%d, Active=%d, Builds=%d",
		stats.SocksPort, stats.ControlPort, stats.ActiveCircuits,
		stats.CircuitBuilds)
}

// BenchmarkClientStartup benchmarks client startup time
func BenchmarkClientStartup(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark in short mode")
	}

	cfg := config.DefaultConfig()
	cfg.LogLevel = "error" // Reduce log noise

	log := logger.NewDefault()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Use unique ports for each iteration
		cfg.SocksPort = 30000 + i
		cfg.ControlPort = 30100 + i

		client, err := New(cfg, log)
		if err != nil {
			b.Fatalf("Failed to create client: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)

		go func() {
			_ = client.Start(ctx)
		}()

		// Wait for at least one circuit
		startTime := time.Now()
		for {
			stats := client.GetStats()
			if stats.ActiveCircuits > 0 {
				break
			}
			if time.Since(startTime) > 90*time.Second {
				b.Fatalf("Client did not start in time")
			}
			time.Sleep(1 * time.Second)
		}

		client.Stop()
		cancel()

		// Wait a bit before next iteration
		time.Sleep(2 * time.Second)
	}
}

// TestIntegrationHealthCheck tests health monitoring
func TestIntegrationHealthCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := config.DefaultConfig()
	cfg.SocksPort = 23050
	cfg.ControlPort = 23051
	cfg.EnableMetrics = true
	cfg.MetricsPort = 23052

	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	go func() {
		_ = client.Start(ctx)
	}()

	time.Sleep(30 * time.Second)

	// Check health status
	health := client.healthMonitor.GetLastCheck()

	if health.Status != "healthy" && health.Status != "degraded" {
		t.Errorf("Unexpected health status: %s", health.Status)
	}

	// Should have some component checks
	if len(health.Components) == 0 {
		t.Log("No component health checks found yet (expected during startup)")
	} else {
		for name, component := range health.Components {
			t.Logf("Component %s: %s", name, component.Status)
		}
	}

	client.Stop()
}

// TestIntegrationOptionsValidation tests options validation
func TestIntegrationOptionsValidation(t *testing.T) {
	tests := []struct {
		name    string
		opts    *Options
		wantErr bool
	}{
		{
			name: "valid_custom_ports",
			opts: &Options{
				SocksPort:   24050,
				ControlPort: 24051,
				LogLevel:    "debug",
			},
			wantErr: false,
		},
		{
			name: "valid_custom_data_dir",
			opts: &Options{
				SocksPort:     24052,
				ControlPort:   24053,
				DataDirectory: "/tmp/tor-test-data",
			},
			wantErr: false,
		},
		{
			name: "zero_ports_use_defaults",
			opts: &Options{
				LogLevel: "info",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if testing.Short() {
				t.Skip("Skipping integration test in short mode")
			}

			client, err := ConnectWithOptions(tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConnectWithOptions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				stats := client.Stats()
				if stats.SocksPort == 0 {
					t.Error("SOCKS port should be set")
				}
				client.Close()
			}
		})
	}
}
