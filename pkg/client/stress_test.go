package client

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/config"
	"github.com/opd-ai/go-tor/pkg/logger"
)

// TestConcurrentBandwidthRecording tests concurrent bandwidth recording under load
func TestConcurrentBandwidthRecording(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Stop()

	const (
		numGoroutines = 100
		numIterations = 1000
	)

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 2) // *2 for both read and write goroutines

	// Concurrent read recording
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				client.RecordBytesRead(1)
			}
		}()
	}

	// Concurrent write recording
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				client.RecordBytesWritten(1)
			}
		}()
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Verify no panics occurred (test passes if we reach here)
	t.Log("Concurrent bandwidth recording completed successfully")
}

// TestMultipleStartStop tests rapid start/stop cycles
func TestMultipleStartStop(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	cfg.SocksPort = 19080
	cfg.ControlPort = 19081
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Multiple stop calls should be safe
	for i := 0; i < 5; i++ {
		err := client.Stop()
		if err != nil {
			t.Logf("Stop %d returned: %v", i+1, err)
		}
	}

	t.Log("Multiple stop calls completed successfully")
}

// TestStatsUnderLoad tests GetStats under concurrent access
func TestStatsUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Stop()

	const (
		numGoroutines = 50
		duration      = 2 * time.Second
	)

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Concurrent GetStats calls
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					stats := client.GetStats()
					_ = stats // Use the stats to prevent optimization
				}
			}
		}()
	}

	wg.Wait()
	t.Log("Concurrent GetStats completed successfully")
}

// TestClientLifecycleStress tests full client lifecycle under stress
func TestClientLifecycleStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	// Create and destroy multiple clients rapidly
	for i := 0; i < 10; i++ {
		cfg := config.DefaultConfig()
		cfg.DataDirectory = t.TempDir()
		cfg.SocksPort = 19082 + i
		cfg.ControlPort = 19092 + i
		log := logger.NewDefault()

		client, err := New(cfg, log)
		if err != nil {
			t.Fatalf("Failed to create client %d: %v", i, err)
		}

		// Immediately stop without starting
		err = client.Stop()
		if err != nil {
			t.Logf("Stop for client %d returned: %v", i, err)
		}
	}

	t.Log("Client lifecycle stress test completed successfully")
}

// TestContextCancellationRace tests context cancellation races
func TestContextCancellationRace(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	cfg.SocksPort = 19110
	cfg.ControlPort = 19111
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Start with a context that will be cancelled immediately
	ctx, cancel := context.WithCancel(context.Background())

	// Start in goroutine
	go func() {
		_ = client.Start(ctx)
	}()

	// Cancel immediately
	cancel()

	// Give it a moment then stop
	time.Sleep(100 * time.Millisecond)
	_ = client.Stop()

	t.Log("Context cancellation race test completed")
}

// TestMetricsUnderLoad tests metrics recording under concurrent load
func TestMetricsUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Stop()

	const (
		numGoroutines = 100
		numIterations = 500
	)

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 3) // Bandwidth + GetStats + metrics access

	// Concurrent bandwidth recording
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				client.RecordBytesRead(10)
				client.RecordBytesWritten(20)
			}
		}()
	}

	// Concurrent stats reading
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				_ = client.GetStats()
			}
		}()
	}

	// Concurrent metrics access
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				if client.metrics != nil {
					_ = client.metrics.CircuitBuilds.Value()
					_ = client.metrics.ActiveCircuits.Value()
				}
			}
		}()
	}

	wg.Wait()
	t.Log("Metrics under load test completed successfully")
}

// BenchmarkBandwidthRecording benchmarks bandwidth recording performance
func BenchmarkBandwidthRecording(b *testing.B) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = b.TempDir()
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		b.Fatalf("Failed to create client: %v", err)
	}
	defer client.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.RecordBytesRead(1024)
		client.RecordBytesWritten(2048)
	}
}

// BenchmarkGetStats benchmarks GetStats performance
func BenchmarkGetStats(b *testing.B) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = b.TempDir()
	log := logger.NewDefault()

	client, err := New(cfg, log)
	if err != nil {
		b.Fatalf("Failed to create client: %v", err)
	}
	defer client.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.GetStats()
	}
}

// BenchmarkClientCreation benchmarks client creation performance
func BenchmarkClientCreation(b *testing.B) {
	log := logger.NewDefault()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cfg := config.DefaultConfig()
		cfg.DataDirectory = b.TempDir()

		client, err := New(cfg, log)
		if err != nil {
			b.Fatalf("Failed to create client: %v", err)
		}
		_ = client.Stop()
	}
}
