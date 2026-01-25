package pt

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestManagerMonitoringIntegration tests PT process monitoring and restart.
func TestManagerMonitoringIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tempDir := t.TempDir()

	script := filepath.Join(tempDir, "crash-pt")
	scriptContent := `#!/bin/sh
# PT that crashes immediately
exit 1
`
	if err := os.WriteFile(script, []byte(scriptContent), 0o755); err != nil {
		t.Fatal(err)
	}

	m := NewManager(ManagerConfig{
		StateDir:     tempDir,
		AutoRestart:  true,
		RestartDelay: 100 * time.Millisecond,
		MaxRestarts:  2,
	})

	config := TransportConfig{
		BinaryPath: script,
	}

	if err := m.AddClient("crash-pt", config); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	m.StartAll(ctx)

	time.Sleep(1 * time.Second)

	if err := m.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

// TestManagerMonitoringUnlimitedRestarts tests unlimited restarts.
func TestManagerMonitoringUnlimitedRestarts(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tempDir := t.TempDir()

	script := filepath.Join(tempDir, "crash-pt")
	scriptContent := `#!/bin/sh
exit 1
`
	if err := os.WriteFile(script, []byte(scriptContent), 0o755); err != nil {
		t.Fatal(err)
	}

	m := NewManager(ManagerConfig{
		StateDir:     tempDir,
		AutoRestart:  true,
		RestartDelay: 50 * time.Millisecond,
		MaxRestarts:  0,
	})

	config := TransportConfig{
		BinaryPath: script,
	}

	if err := m.AddClient("crash-pt", config); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	m.StartAll(ctx)

	time.Sleep(300 * time.Millisecond)

	if err := m.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

// TestManagerServerMonitoring tests server PT monitoring.
func TestManagerServerMonitoring(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tempDir := t.TempDir()

	script := filepath.Join(tempDir, "crash-server")
	scriptContent := `#!/bin/sh
exit 1
`
	if err := os.WriteFile(script, []byte(scriptContent), 0o755); err != nil {
		t.Fatal(err)
	}

	m := NewManager(ManagerConfig{
		StateDir:     tempDir,
		AutoRestart:  true,
		RestartDelay: 100 * time.Millisecond,
		MaxRestarts:  1,
	})

	config := TransportConfig{
		BinaryPath: script,
	}

	if err := m.AddServer("crash-server", config); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	m.StartAll(ctx)

	time.Sleep(500 * time.Millisecond)

	if err := m.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

// TestManagerGracefulShutdown tests graceful shutdown of monitoring goroutines.
func TestManagerGracefulShutdown(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tempDir := t.TempDir()

	script := filepath.Join(tempDir, "long-pt")
	scriptContent := `#!/bin/sh
sleep 10
`
	if err := os.WriteFile(script, []byte(scriptContent), 0o755); err != nil {
		t.Fatal(err)
	}

	m := NewManager(ManagerConfig{
		StateDir:     tempDir,
		AutoRestart:  true,
		RestartDelay: 100 * time.Millisecond,
	})

	config := TransportConfig{
		BinaryPath: script,
	}

	if err := m.AddClient("long-pt", config); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	m.StartAll(ctx)

	time.Sleep(200 * time.Millisecond)

	start := time.Now()
	if err := m.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
	elapsed := time.Since(start)

	if elapsed > 2*time.Second {
		t.Errorf("Close took too long: %v", elapsed)
	}
}
