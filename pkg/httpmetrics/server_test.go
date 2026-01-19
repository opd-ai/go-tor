package httpmetrics

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/health"
	"github.com/opd-ai/go-tor/pkg/logger"
	"github.com/opd-ai/go-tor/pkg/metrics"
)

// Mock metrics provider for testing
type mockMetricsProvider struct {
	snapshot *metrics.Snapshot
}

func (m *mockMetricsProvider) Snapshot() *metrics.Snapshot {
	if m.snapshot == nil {
		return &metrics.Snapshot{
			CircuitBuilds:       100,
			CircuitBuildSuccess: 95,
			CircuitBuildFailure: 5,
			CircuitBuildTimeAvg: 3 * time.Second,
			CircuitBuildTimeP95: 5 * time.Second,
			ActiveCircuits:      3,
			ConnectionAttempts:  50,
			ConnectionSuccess:   48,
			ConnectionFailures:  2,
			ConnectionRetries:   5,
			TLSHandshakeAvg:     500 * time.Millisecond,
			TLSHandshakeP95:     800 * time.Millisecond,
			ActiveConnections:   3,
			StreamsCreated:      200,
			StreamsClosed:       190,
			StreamFailures:      10,
			ActiveStreams:       10,
			StreamData:          1024000,
			GuardsActive:        3,
			GuardsConfirmed:     2,
			SocksConnections:    150,
			SocksRequests:       145,
			SocksErrors:         5,
			UptimeSeconds:       3600,
		}
	}
	return m.snapshot
}

// Mock health provider for testing
type mockHealthProvider struct {
	health health.OverallHealth
}

func (m *mockHealthProvider) Check(ctx context.Context) health.OverallHealth {
	if m.health.Status == "" {
		return health.OverallHealth{
			Status:    health.StatusHealthy,
			Timestamp: time.Now(),
			Uptime:    time.Hour,
			Components: map[string]health.ComponentHealth{
				"circuits": {
					Name:        "circuits",
					Status:      health.StatusHealthy,
					Message:     "All circuits operational",
					LastChecked: time.Now(),
				},
			},
		}
	}
	return m.health
}

// newTestServer creates a server configured for fast tests with zero drain period.
func newTestServer(metricsProvider MetricsProvider, healthProvider HealthProvider) *Server {
	log := logger.NewDefault()
	server := NewServer("127.0.0.1:0", metricsProvider, healthProvider, log)
	server.SetDrainPeriod(0) // Fast shutdown for tests
	return server
}

func TestNewServer(t *testing.T) {
	log := logger.NewDefault()
	metricsProvider := &mockMetricsProvider{}
	healthProvider := &mockHealthProvider{}

	server := NewServer("127.0.0.1:0", metricsProvider, healthProvider, log)
	if server == nil {
		t.Fatal("NewServer returned nil")
	}

	if server.address == "" {
		t.Error("Server address not set")
	}

	if server.metricsProvider == nil {
		t.Error("Metrics provider not set")
	}

	if server.healthProvider == nil {
		t.Error("Health provider not set")
	}
}

func TestServerStartStop(t *testing.T) {
	metricsProvider := &mockMetricsProvider{}
	healthProvider := &mockHealthProvider{}

	server := newTestServer(metricsProvider, healthProvider)

	// Start server
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Verify it's listening
	addr := server.GetAddress()
	if addr == "" {
		t.Error("Server address is empty after start")
	}

	// Stop server
	if err := server.Stop(); err != nil {
		t.Errorf("Failed to stop server: %v", err)
	}
}

func TestPrometheusMetricsEndpoint(t *testing.T) {
	metricsProvider := &mockMetricsProvider{}
	healthProvider := &mockHealthProvider{}

	server := newTestServer(metricsProvider, healthProvider)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Make HTTP request
	url := "http://" + server.GetAddress() + "/metrics"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("Failed to GET /metrics: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/plain") {
		t.Errorf("Expected Content-Type text/plain, got %s", contentType)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	bodyStr := string(body)

	// Check for expected metrics
	expectedMetrics := []string{
		"tor_circuit_builds_total",
		"tor_circuit_build_success_total",
		"tor_active_circuits",
		"tor_connection_attempts_total",
		"tor_active_streams",
		"tor_uptime_seconds",
	}

	for _, metric := range expectedMetrics {
		if !strings.Contains(bodyStr, metric) {
			t.Errorf("Expected metric %s not found in response", metric)
		}
	}

	// Verify HELP and TYPE comments are present
	if !strings.Contains(bodyStr, "# HELP") {
		t.Error("Expected HELP comments in Prometheus format")
	}
	if !strings.Contains(bodyStr, "# TYPE") {
		t.Error("Expected TYPE comments in Prometheus format")
	}
}

func TestJSONMetricsEndpoint(t *testing.T) {
	metricsProvider := &mockMetricsProvider{}
	healthProvider := &mockHealthProvider{}

	server := newTestServer(metricsProvider, healthProvider)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Make HTTP request
	url := "http://" + server.GetAddress() + "/metrics/json"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("Failed to GET /metrics/json: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	var snapshot metrics.Snapshot
	if err := json.NewDecoder(resp.Body).Decode(&snapshot); err != nil {
		t.Fatalf("Failed to decode JSON response: %v", err)
	}

	// Verify metrics values
	if snapshot.CircuitBuilds != 100 {
		t.Errorf("Expected CircuitBuilds=100, got %d", snapshot.CircuitBuilds)
	}
	if snapshot.CircuitBuildSuccess != 95 {
		t.Errorf("Expected CircuitBuildSuccess=95, got %d", snapshot.CircuitBuildSuccess)
	}
	if snapshot.ActiveCircuits != 3 {
		t.Errorf("Expected ActiveCircuits=3, got %d", snapshot.ActiveCircuits)
	}
}

func TestHealthEndpoint(t *testing.T) {
	metricsProvider := &mockMetricsProvider{}
	healthProvider := &mockHealthProvider{}

	server := newTestServer(metricsProvider, healthProvider)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Test healthy status
	url := "http://" + server.GetAddress() + "/health"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("Failed to GET /health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for healthy, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	var healthStatus health.OverallHealth
	if err := json.NewDecoder(resp.Body).Decode(&healthStatus); err != nil {
		t.Fatalf("Failed to decode JSON response: %v", err)
	}

	if healthStatus.Status != health.StatusHealthy {
		t.Errorf("Expected status healthy, got %s", healthStatus.Status)
	}
}

func TestHealthEndpointUnhealthy(t *testing.T) {
	metricsProvider := &mockMetricsProvider{}
	healthProvider := &mockHealthProvider{
		health: health.OverallHealth{
			Status:    health.StatusUnhealthy,
			Timestamp: time.Now(),
			Components: map[string]health.ComponentHealth{
				"circuits": {
					Name:    "circuits",
					Status:  health.StatusUnhealthy,
					Message: "No circuits available",
				},
			},
		},
	}

	server := newTestServer(metricsProvider, healthProvider)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Test unhealthy status
	url := "http://" + server.GetAddress() + "/health"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("Failed to GET /health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503 for unhealthy, got %d", resp.StatusCode)
	}
}

func TestDashboardEndpoint(t *testing.T) {
	metricsProvider := &mockMetricsProvider{}
	healthProvider := &mockHealthProvider{}

	server := newTestServer(metricsProvider, healthProvider)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Make HTTP request
	url := "http://" + server.GetAddress() + "/debug/metrics"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("Failed to GET /debug/metrics: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("Expected Content-Type text/html, got %s", contentType)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	bodyStr := string(body)

	// Check for expected HTML content
	if !strings.Contains(bodyStr, "<!DOCTYPE html>") {
		t.Error("Expected HTML document")
	}
	if !strings.Contains(bodyStr, "go-tor Metrics Dashboard") {
		t.Error("Expected dashboard title")
	}
	if !strings.Contains(bodyStr, "Circuit Metrics") {
		t.Error("Expected circuit metrics section")
	}
}

func TestIndexEndpoint(t *testing.T) {
	metricsProvider := &mockMetricsProvider{}
	healthProvider := &mockHealthProvider{}

	server := newTestServer(metricsProvider, healthProvider)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Make HTTP request
	url := "http://" + server.GetAddress() + "/"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("Failed to GET /: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	bodyStr := string(body)

	// Check for links to all endpoints
	expectedLinks := []string{
		"/metrics",
		"/metrics/json",
		"/health",
		"/debug/metrics",
	}

	for _, link := range expectedLinks {
		if !strings.Contains(bodyStr, link) {
			t.Errorf("Expected link to %s not found", link)
		}
	}
}

func TestMethodNotAllowed(t *testing.T) {
	metricsProvider := &mockMetricsProvider{}
	healthProvider := &mockHealthProvider{}

	server := newTestServer(metricsProvider, healthProvider)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Test POST method on GET-only endpoint
	url := "http://" + server.GetAddress() + "/metrics"
	resp, err := http.Post(url, "text/plain", strings.NewReader("test"))
	if err != nil {
		t.Fatalf("Failed to POST /metrics: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", resp.StatusCode)
	}
}

func TestNotFound(t *testing.T) {
	metricsProvider := &mockMetricsProvider{}
	healthProvider := &mockHealthProvider{}

	server := newTestServer(metricsProvider, healthProvider)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Test non-existent endpoint
	url := "http://" + server.GetAddress() + "/nonexistent"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("Failed to GET /nonexistent: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.StatusCode)
	}
}

func TestShutdownTimeoutConfiguration(t *testing.T) {
	log := logger.NewDefault()
	metricsProvider := &mockMetricsProvider{}
	healthProvider := &mockHealthProvider{}

	server := NewServer("127.0.0.1:0", metricsProvider, healthProvider, log)

	// Verify default values
	if server.GetShutdownTimeout() != DefaultShutdownTimeout {
		t.Errorf("Expected default shutdown timeout %v, got %v", DefaultShutdownTimeout, server.GetShutdownTimeout())
	}
	if server.GetDrainPeriod() != DefaultDrainPeriod {
		t.Errorf("Expected default drain period %v, got %v", DefaultDrainPeriod, server.GetDrainPeriod())
	}

	// Test setting custom values
	customTimeout := 30 * time.Second
	customDrain := 5 * time.Second
	server.SetShutdownTimeout(customTimeout)
	server.SetDrainPeriod(customDrain)

	if server.GetShutdownTimeout() != customTimeout {
		t.Errorf("Expected shutdown timeout %v, got %v", customTimeout, server.GetShutdownTimeout())
	}
	if server.GetDrainPeriod() != customDrain {
		t.Errorf("Expected drain period %v, got %v", customDrain, server.GetDrainPeriod())
	}
}

func TestShutdownTimeoutZeroValue(t *testing.T) {
	log := logger.NewDefault()
	metricsProvider := &mockMetricsProvider{}
	healthProvider := &mockHealthProvider{}

	server := NewServer("127.0.0.1:0", metricsProvider, healthProvider, log)

	// Setting zero should reset to default
	server.SetShutdownTimeout(0)
	if server.GetShutdownTimeout() != DefaultShutdownTimeout {
		t.Errorf("Expected default timeout after setting 0, got %v", server.GetShutdownTimeout())
	}

	// Setting negative should reset to default
	server.SetShutdownTimeout(-5 * time.Second)
	if server.GetShutdownTimeout() != DefaultShutdownTimeout {
		t.Errorf("Expected default timeout after setting negative, got %v", server.GetShutdownTimeout())
	}
}

func TestDrainPeriodNegativeValue(t *testing.T) {
	log := logger.NewDefault()
	metricsProvider := &mockMetricsProvider{}
	healthProvider := &mockHealthProvider{}

	server := NewServer("127.0.0.1:0", metricsProvider, healthProvider, log)

	// Setting negative should reset to default
	server.SetDrainPeriod(-5 * time.Second)
	if server.GetDrainPeriod() != DefaultDrainPeriod {
		t.Errorf("Expected default drain period after setting negative, got %v", server.GetDrainPeriod())
	}

	// Setting zero should be allowed (no drain)
	server.SetDrainPeriod(0)
	if server.GetDrainPeriod() != 0 {
		t.Errorf("Expected zero drain period, got %v", server.GetDrainPeriod())
	}
}

func TestGracefulShutdownWithZeroDrain(t *testing.T) {
	log := logger.NewDefault()
	metricsProvider := &mockMetricsProvider{}
	healthProvider := &mockHealthProvider{}

	server := NewServer("127.0.0.1:0", metricsProvider, healthProvider, log)

	// Set drain period to 0 for fast tests
	server.SetDrainPeriod(0)

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Verify server is working
	url := "http://" + server.GetAddress() + "/health"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("Failed to GET /health: %v", err)
	}
	resp.Body.Close()

	// Stop should be fast with 0 drain period
	start := time.Now()
	if err := server.Stop(); err != nil {
		t.Errorf("Failed to stop server: %v", err)
	}
	elapsed := time.Since(start)

	// Without drain period, shutdown should be very fast (under 100ms)
	if elapsed > 100*time.Millisecond {
		t.Errorf("Shutdown took too long without drain period: %v", elapsed)
	}
}

func TestStopWithoutStart(t *testing.T) {
	metricsProvider := &mockMetricsProvider{}
	healthProvider := &mockHealthProvider{}

	server := newTestServer(metricsProvider, healthProvider)

	// Stop without Start should not panic or cause nil pointer dereference
	// The server.server field is initialized in NewServer, so Shutdown should work
	err := server.Stop()
	if err != nil {
		t.Errorf("Stop without Start returned error: %v", err)
	}
}

func TestStopCalledMultipleTimes(t *testing.T) {
	metricsProvider := &mockMetricsProvider{}
	healthProvider := &mockHealthProvider{}

	server := newTestServer(metricsProvider, healthProvider)

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// First Stop should succeed
	if err := server.Stop(); err != nil {
		t.Errorf("First Stop returned error: %v", err)
	}

	// Second Stop should also complete without panic
	// Note: Calling Stop multiple times is safe because:
	// - cancel() is safe to call multiple times
	// - http.Server.Shutdown() is safe to call on already closed server
	// - sync.WaitGroup.Wait() is safe when counter is already 0
	// Second stop may return an error (server already closed) but should not panic
	_ = server.Stop()
}

// mockProbeProvider implements ProbeProvider for testing
type mockProbeProvider struct {
	livenessResult  health.ProbeResult
	readinessResult health.ProbeResult
}

func (m *mockProbeProvider) CheckLiveness(ctx context.Context) health.ProbeResult {
	return m.livenessResult
}

func (m *mockProbeProvider) CheckReadiness(ctx context.Context) health.ProbeResult {
	return m.readinessResult
}

func TestLivenessEndpoint(t *testing.T) {
	metricsProvider := &mockMetricsProvider{}
	healthProvider := &mockHealthProvider{}

	server := newTestServer(metricsProvider, healthProvider)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Test liveness endpoint (should fall back to health check)
	url := "http://" + server.GetAddress() + "/live"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("Failed to GET /live: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for liveness, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	var result health.ProbeResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode JSON response: %v", err)
	}

	if result.Type != health.ProbeLiveness {
		t.Errorf("Expected liveness probe type, got %s", result.Type)
	}
	if !result.Healthy {
		t.Error("Expected healthy liveness result")
	}
}

func TestLivenessEndpointUnhealthy(t *testing.T) {
	metricsProvider := &mockMetricsProvider{}
	healthProvider := &mockHealthProvider{
		health: health.OverallHealth{
			Status:    health.StatusUnhealthy,
			Timestamp: time.Now(),
			Components: map[string]health.ComponentHealth{
				"test": {
					Name:   "test",
					Status: health.StatusUnhealthy,
				},
			},
		},
	}

	server := newTestServer(metricsProvider, healthProvider)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	url := "http://" + server.GetAddress() + "/live"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("Failed to GET /live: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503 for unhealthy liveness, got %d", resp.StatusCode)
	}
}

func TestReadinessEndpoint(t *testing.T) {
	metricsProvider := &mockMetricsProvider{}
	healthProvider := &mockHealthProvider{}

	server := newTestServer(metricsProvider, healthProvider)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Test readiness endpoint (should fall back to health check)
	url := "http://" + server.GetAddress() + "/ready"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("Failed to GET /ready: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for readiness, got %d", resp.StatusCode)
	}

	var result health.ProbeResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode JSON response: %v", err)
	}

	if result.Type != health.ProbeReadiness {
		t.Errorf("Expected readiness probe type, got %s", result.Type)
	}
	if !result.Healthy {
		t.Error("Expected healthy readiness result")
	}
}

func TestReadinessEndpointUnhealthy(t *testing.T) {
	metricsProvider := &mockMetricsProvider{}
	healthProvider := &mockHealthProvider{
		health: health.OverallHealth{
			Status:    health.StatusUnhealthy,
			Timestamp: time.Now(),
			Components: map[string]health.ComponentHealth{
				"test": {
					Name:   "test",
					Status: health.StatusUnhealthy,
				},
			},
		},
	}

	server := newTestServer(metricsProvider, healthProvider)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	url := "http://" + server.GetAddress() + "/ready"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("Failed to GET /ready: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503 for unhealthy readiness, got %d", resp.StatusCode)
	}
}

func TestReadinessEndpointDegraded(t *testing.T) {
	metricsProvider := &mockMetricsProvider{}
	healthProvider := &mockHealthProvider{
		health: health.OverallHealth{
			Status:    health.StatusDegraded,
			Timestamp: time.Now(),
			Components: map[string]health.ComponentHealth{
				"test": {
					Name:   "test",
					Status: health.StatusDegraded,
				},
			},
		},
	}

	server := newTestServer(metricsProvider, healthProvider)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	url := "http://" + server.GetAddress() + "/ready"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("Failed to GET /ready: %v", err)
	}
	defer resp.Body.Close()

	// Degraded should still be ready (can serve traffic)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for degraded readiness, got %d", resp.StatusCode)
	}
}

func TestLivenessWithProbeProvider(t *testing.T) {
	metricsProvider := &mockMetricsProvider{}
	healthProvider := &mockHealthProvider{}
	probeProvider := &mockProbeProvider{
		livenessResult: health.ProbeResult{
			Type:    health.ProbeLiveness,
			Healthy: true,
			Message: "Custom liveness check passed",
		},
	}

	server := newTestServer(metricsProvider, healthProvider)
	server.SetProbeProvider(probeProvider)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	url := "http://" + server.GetAddress() + "/live"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("Failed to GET /live: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var result health.ProbeResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result.Message != "Custom liveness check passed" {
		t.Errorf("Expected custom message, got: %s", result.Message)
	}
}

func TestReadinessWithProbeProvider(t *testing.T) {
	metricsProvider := &mockMetricsProvider{}
	healthProvider := &mockHealthProvider{}
	probeProvider := &mockProbeProvider{
		readinessResult: health.ProbeResult{
			Type:    health.ProbeReadiness,
			Healthy: false,
			Message: "Not ready - waiting for dependencies",
		},
	}

	server := newTestServer(metricsProvider, healthProvider)
	server.SetProbeProvider(probeProvider)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	url := "http://" + server.GetAddress() + "/ready"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("Failed to GET /ready: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", resp.StatusCode)
	}

	var result health.ProbeResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result.Message != "Not ready - waiting for dependencies" {
		t.Errorf("Expected custom message, got: %s", result.Message)
	}
}

func TestLivenessMethodNotAllowed(t *testing.T) {
	metricsProvider := &mockMetricsProvider{}
	healthProvider := &mockHealthProvider{}

	server := newTestServer(metricsProvider, healthProvider)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	url := "http://" + server.GetAddress() + "/live"
	resp, err := http.Post(url, "text/plain", strings.NewReader("test"))
	if err != nil {
		t.Fatalf("Failed to POST /live: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", resp.StatusCode)
	}
}

func TestReadinessMethodNotAllowed(t *testing.T) {
	metricsProvider := &mockMetricsProvider{}
	healthProvider := &mockHealthProvider{}

	server := newTestServer(metricsProvider, healthProvider)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	url := "http://" + server.GetAddress() + "/ready"
	resp, err := http.Post(url, "text/plain", strings.NewReader("test"))
	if err != nil {
		t.Fatalf("Failed to POST /ready: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", resp.StatusCode)
	}
}

func TestIndexEndpointIncludesProbeLinks(t *testing.T) {
	metricsProvider := &mockMetricsProvider{}
	healthProvider := &mockHealthProvider{}

	server := newTestServer(metricsProvider, healthProvider)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	url := "http://" + server.GetAddress() + "/"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("Failed to GET /: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	bodyStr := string(body)

	// Check for new probe endpoint links
	if !strings.Contains(bodyStr, "/live") {
		t.Error("Expected /live link in index page")
	}
	if !strings.Contains(bodyStr, "/ready") {
		t.Error("Expected /ready link in index page")
	}
}
