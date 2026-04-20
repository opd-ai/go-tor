package client

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/config"
	"github.com/opd-ai/go-tor/pkg/control"
	"github.com/opd-ai/go-tor/pkg/logger"
)

// TestRaceConcurrentStopCalls verifies multiple goroutines calling Stop() simultaneously won't panic.
func TestRaceConcurrentStopCalls(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping race condition stress test in short mode")
	}

	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	log := logger.NewDefault()

	c, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_ = c.Stop()
		}()
	}

	wg.Wait()
}

// TestRaceStopBeforeStart verifies Stop() on a never-started client is safe.
func TestRaceStopBeforeStart(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	log := logger.NewDefault()

	c, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	if err := c.Stop(); err != nil {
		t.Logf("Stop returned: %v", err)
	}
}

// TestRaceGetStatsDuringShutdown verifies concurrent GetStats() during Stop() is race-free.
func TestRaceGetStatsDuringShutdown(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping race condition stress test in short mode")
	}

	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	log := logger.NewDefault()

	c, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(11)

	// One goroutine calls Stop
	go func() {
		defer wg.Done()
		time.Sleep(5 * time.Millisecond)
		_ = c.Stop()
	}()

	// Ten goroutines call GetStats concurrently
	for i := 0; i < 10; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = c.GetStats()
			}
		}()
	}

	wg.Wait()
}

// TestRaceRecordBandwidthDuringShutdown verifies RecordBytesRead/Written during Stop().
func TestRaceRecordBandwidthDuringShutdown(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping race condition stress test in short mode")
	}

	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	log := logger.NewDefault()

	c, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(21)

	// One goroutine triggers shutdown
	go func() {
		defer wg.Done()
		time.Sleep(2 * time.Millisecond)
		_ = c.Stop()
	}()

	// Ten goroutines record bytes read
	for i := 0; i < 10; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				c.RecordBytesRead(1024)
			}
		}()
	}

	// Ten goroutines record bytes written
	for i := 0; i < 10; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				c.RecordBytesWritten(512)
			}
		}()
	}

	wg.Wait()
}

// TestRaceSimpleClientCloseIdempotent verifies Close() can be called multiple times.
func TestRaceSimpleClientCloseIdempotent(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	log := logger.NewDefault()

	c, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	sc := &SimpleClient{client: c, logger: log}

	for i := 0; i < 5; i++ {
		if err := sc.Close(); err != nil {
			t.Logf("Close() call %d: %v", i, err)
		}
	}
}

// TestRaceConcurrentNewCalls verifies creating multiple clients concurrently is safe.
func TestRaceConcurrentNewCalls(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping race condition stress test in short mode")
	}

	const goroutines = 10
	var wg sync.WaitGroup
	wg.Add(goroutines)

	log := logger.NewDefault()

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			cfg := config.DefaultConfig()
			cfg.DataDirectory = t.TempDir()
			c, err := New(cfg, log)
			if err != nil {
				t.Errorf("New() failed: %v", err)
				return
			}
			_ = c.Stop()
		}()
	}

	wg.Wait()
}

// TestRaceGetCircuitDuringStop verifies GetCircuit() during Stop() is race-free.
func TestRaceGetCircuitDuringStop(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping race condition stress test in short mode")
	}

	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	log := logger.NewDefault()

	c, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(11)

	// One goroutine calls Stop
	go func() {
		defer wg.Done()
		time.Sleep(3 * time.Millisecond)
		_ = c.Stop()
	}()

	// Ten goroutines call GetCircuit
	for i := 0; i < 10; i++ {
		go func() {
			defer wg.Done()
			ctx := context.Background()
			for j := 0; j < 50; j++ {
				_, _ = c.GetCircuit(ctx)
			}
		}()
	}

	wg.Wait()
}

// TestRacePublishEventDuringStop verifies PublishEvent() during Stop() is race-free.
func TestRacePublishEventDuringStop(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping race condition stress test in short mode")
	}

	cfg := config.DefaultConfig()
	cfg.DataDirectory = t.TempDir()
	log := logger.NewDefault()

	c, err := New(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(11)

	// One goroutine calls Stop
	go func() {
		defer wg.Done()
		time.Sleep(2 * time.Millisecond)
		_ = c.Stop()
	}()

	// Ten goroutines publish events
	for i := 0; i < 10; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				c.PublishEvent(&control.BWEvent{
					BytesRead:    uint64(j * 100),
					BytesWritten: uint64(j * 50),
				})
			}
		}()
	}

	wg.Wait()
}

// TestRaceRapidCreateCloseCycles verifies rapid create-close cycles are race-free.
func TestRaceRapidCreateCloseCycles(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping race condition stress test in short mode")
	}

	log := logger.NewDefault()

	for i := 0; i < 20; i++ {
		cfg := config.DefaultConfig()
		cfg.DataDirectory = t.TempDir()

		c, err := New(cfg, log)
		if err != nil {
			t.Fatalf("Cycle %d: New() failed: %v", i, err)
		}
		_ = c.Stop()
	}
}

// TestRaceContextCancelDuringNew verifies early context cancellation is safe.
func TestRaceContextCancelDuringNew(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping race condition stress test in short mode")
	}

	log := logger.NewDefault()

	for i := 0; i < 10; i++ {
		cfg := config.DefaultConfig()
		cfg.DataDirectory = t.TempDir()

		c, err := New(cfg, log)
		if err != nil {
			t.Fatalf("Iteration %d: New() failed: %v", i, err)
		}

		// Create an already-cancelled context and try Start
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_ = c.Start(ctx)
		_ = c.Stop()
	}
}
