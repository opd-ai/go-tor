package httpmetrics

import (
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/metrics"
)

func startStressServer(t *testing.T) (*Server, string) {
	t.Helper()
	server := newTestServer(&mockMetricsProvider{}, &mockHealthProvider{})
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	return server, "http://" + server.GetAddress()
}

func TestStressConcurrentMetricRequests(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}
	server, baseURL := startStressServer(t)
	defer server.Stop()

	runConcurrentRequests(t, baseURL+"/metrics/json", 50)
}

func TestStressConcurrentHealthChecks(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}
	server, baseURL := startStressServer(t)
	defer server.Stop()

	runConcurrentRequests(t, baseURL+"/health", 50)
}

func TestStressMixedEndpoints(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}
	server, baseURL := startStressServer(t)
	defer server.Stop()

	endpoints := []string{"/metrics", "/metrics/json", "/health", "/live", "/ready"}
	var wg sync.WaitGroup
	errCh := make(chan error, 50)

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ep := endpoints[rand.Intn(len(endpoints))]
			if err := doGet(baseURL + ep); err != nil {
				errCh <- fmt.Errorf("goroutine %d on %s: %w", id, ep, err)
			}
		}(i)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}
}

func TestStressRapidStartStopCycles(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}
	for i := 0; i < 10; i++ {
		server := newTestServer(&mockMetricsProvider{}, &mockHealthProvider{})
		if err := server.Start(); err != nil {
			t.Fatalf("cycle %d: start failed: %v", i, err)
		}
		if err := server.Stop(); err != nil {
			t.Fatalf("cycle %d: stop failed: %v", i, err)
		}
	}
}

func TestStressConcurrentStartStop(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}
	server := newTestServer(&mockMetricsProvider{}, &mockHealthProvider{})
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = server.Stop()
		}()
	}
	wg.Wait()
}

func TestStressRequestDuringShutdown(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}
	server := newTestServer(&mockMetricsProvider{}, &mockHealthProvider{})
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	baseURL := "http://" + server.GetAddress()

	// Fire requests while shutting down concurrently
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(5 * time.Millisecond)
		_ = server.Stop()
	}()

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Errors expected during shutdown; we just verify no panic
			client := &http.Client{Timeout: 2 * time.Second}
			resp, err := client.Get(baseURL + "/health")
			if err == nil {
				resp.Body.Close()
			}
		}()
	}
	wg.Wait()
}

func TestStressLargeResponseHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}
	// Create a snapshot with large values to simulate many metrics
	largeSnapshot := &metrics.Snapshot{
		CircuitBuilds:       999999,
		CircuitBuildSuccess: 999990,
		CircuitBuildFailure: 9,
		CircuitBuildTimeAvg: 10 * time.Second,
		CircuitBuildTimeP95: 20 * time.Second,
		ActiveCircuits:      500,
		ConnectionAttempts:  888888,
		ConnectionSuccess:   888880,
		ConnectionFailures:  8,
		ConnectionRetries:   100,
		TLSHandshakeAvg:     2 * time.Second,
		TLSHandshakeP95:     5 * time.Second,
		ActiveConnections:   200,
		StreamsCreated:      777777,
		StreamsClosed:       777000,
		StreamFailures:      777,
		ActiveStreams:       500,
		StreamData:          999999999,
		GuardsActive:        50,
		GuardsConfirmed:     45,
		SocksConnections:    666666,
		SocksRequests:       666000,
		SocksErrors:         666,
		UptimeSeconds:       9999999,
	}
	provider := &mockMetricsProvider{snapshot: largeSnapshot}
	server := newTestServer(provider, &mockHealthProvider{})
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	baseURL := "http://" + server.GetAddress()
	runConcurrentRequests(t, baseURL+"/metrics/json", 50)
}

// runConcurrentRequests fires n concurrent GET requests and reports errors.
func runConcurrentRequests(t *testing.T, url string, n int) {
	t.Helper()
	var wg sync.WaitGroup
	errCh := make(chan error, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			if err := doGet(url); err != nil {
				errCh <- fmt.Errorf("goroutine %d: %w", id, err)
			}
		}(i)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}
}

// doGet performs a GET request and validates a 200 response.
func doGet(url string) error {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d for %s", resp.StatusCode, url)
	}
	return nil
}
