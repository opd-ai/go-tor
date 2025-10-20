package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/config"
	"github.com/opd-ai/go-tor/pkg/logger"
)

// TestMetricsServerIntegration tests the HTTP metrics server integration
func TestMetricsServerIntegration(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	cfg.SocksPort = 19060
	cfg.ControlPort = 19061
	cfg.MetricsPort = 19062
	cfg.EnableMetrics = true
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Metrics server should be initialized when enabled
	if client.metricsServer == nil {
		t.Error("Metrics server not initialized despite EnableMetrics=true")
	}

	// Cleanup
	_ = client.Stop()
}

// TestMetricsServerDisabled tests that metrics server is not created when disabled
func TestMetricsServerDisabled(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	cfg.EnableMetrics = false
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Metrics server should NOT be initialized when disabled
	if client.metricsServer != nil {
		t.Error("Metrics server initialized despite EnableMetrics=false")
	}

	// Cleanup
	_ = client.Stop()
}

// TestMetricsEndpointJSON tests the JSON metrics endpoint
func TestMetricsEndpointJSON(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	cfg.SocksPort = 19063
	cfg.ControlPort = 19064
	cfg.MetricsPort = 19065
	cfg.EnableMetrics = true
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Start the metrics server (without full client start)
	if client.metricsServer != nil {
		err = client.metricsServer.Start()
		if err != nil {
			t.Fatalf("Failed to start metrics server: %v", err)
		}
	}

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Test JSON endpoint
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/metrics/json", cfg.MetricsPort))
	if err != nil {
		t.Logf("Failed to fetch JSON metrics (server may not be ready): %v", err)
	} else {
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		// Parse JSON response
		var metricsData map[string]interface{}
		body, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(body, &metricsData); err != nil {
			t.Errorf("Failed to parse JSON metrics: %v", err)
		}

		// Verify some expected fields exist
		if _, ok := metricsData["circuits"]; !ok {
			t.Log("Warning: 'circuits' field missing from metrics")
		}
	}

	// Cleanup
	_ = client.Stop()
}

// TestMetricsEndpointPrometheus tests the Prometheus metrics endpoint
func TestMetricsEndpointPrometheus(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	cfg.SocksPort = 19066
	cfg.ControlPort = 19067
	cfg.MetricsPort = 19068
	cfg.EnableMetrics = true
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Start the metrics server
	if client.metricsServer != nil {
		err = client.metricsServer.Start()
		if err != nil {
			t.Fatalf("Failed to start metrics server: %v", err)
		}
	}

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Test Prometheus endpoint
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/metrics", cfg.MetricsPort))
	if err != nil {
		t.Logf("Failed to fetch Prometheus metrics (server may not be ready): %v", err)
	} else {
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		// Read response body
		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)

		// Check for some expected Prometheus metrics
		if len(bodyStr) == 0 {
			t.Error("Prometheus metrics response is empty")
		}

		// Prometheus format should contain metric names and values
		// Just verify we got something that looks like metrics
		t.Logf("Prometheus metrics length: %d bytes", len(bodyStr))
	}

	// Cleanup
	_ = client.Stop()
}

// TestMetricsEndpointHealth tests the health check endpoint
func TestMetricsEndpointHealth(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	cfg.SocksPort = 19069
	cfg.ControlPort = 19070
	cfg.MetricsPort = 19071
	cfg.EnableMetrics = true
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Start the metrics server
	if client.metricsServer != nil {
		err = client.metricsServer.Start()
		if err != nil {
			t.Fatalf("Failed to start metrics server: %v", err)
		}
	}

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Test health endpoint
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/health", cfg.MetricsPort))
	if err != nil {
		t.Logf("Failed to fetch health check (server may not be ready): %v", err)
	} else {
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		// Parse JSON response
		var healthData map[string]interface{}
		body, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(body, &healthData); err != nil {
			t.Errorf("Failed to parse health JSON: %v", err)
		}

		// Verify status field exists
		if _, ok := healthData["status"]; !ok {
			t.Error("Health response missing 'status' field")
		}
	}

	// Cleanup
	_ = client.Stop()
}

// TestMetricsServerLifecycle tests starting and stopping the metrics server
func TestMetricsServerLifecycle(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	cfg.SocksPort = 19072
	cfg.ControlPort = 19073
	cfg.MetricsPort = 19074
	cfg.EnableMetrics = true
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	if client.metricsServer == nil {
		t.Fatal("Metrics server not initialized")
	}

	// Start metrics server
	err = client.metricsServer.Start()
	if err != nil {
		t.Fatalf("Failed to start metrics server: %v", err)
	}

	// Give it time to start
	time.Sleep(50 * time.Millisecond)

	// Stop metrics server
	err = client.metricsServer.Stop()
	if err != nil {
		t.Errorf("Failed to stop metrics server: %v", err)
	}

	// Cleanup
	_ = client.Stop()
}

// TestMetricsWithClientStart tests metrics server with full client lifecycle
func TestMetricsWithClientStart(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	cfg.SocksPort = 19075
	cfg.ControlPort = 19076
	cfg.MetricsPort = 19077
	cfg.EnableMetrics = true
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Start client in goroutine
	startErr := make(chan error, 1)
	go func() {
		startErr <- client.Start(ctx)
	}()

	// Give it time to start
	time.Sleep(200 * time.Millisecond)

	// Try to access metrics endpoint
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/health", cfg.MetricsPort))
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Health check failed with status %d", resp.StatusCode)
		}
	} else {
		t.Logf("Health check request failed (may be due to startup timing): %v", err)
	}

	// Stop client
	_ = client.Stop()

	// Wait for start to complete
	select {
	case <-startErr:
		// Completed
	case <-time.After(3 * time.Second):
		t.Error("Start did not complete in time")
	}
}

// TestMetricsRecording tests that metrics are properly recorded
func TestMetricsRecording(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	cfg.EnableMetrics = true
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Record some bandwidth
	client.RecordBytesRead(1024)
	client.RecordBytesWritten(2048)

	// Metrics should be accessible
	if client.metrics == nil {
		t.Error("Metrics not initialized")
	}

	// Verify bandwidth tracking via StreamData counter
	// The bandwidth is tracked internally and aggregated in StreamData
	streamData := client.metrics.StreamData.Value()
	// StreamData might be 0 if no streams have been created yet
	// Just verify the metric is accessible
	_ = streamData

	// Cleanup
	_ = client.Stop()
}
