package pt

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	config := DefaultManagerConfig()
	m := NewManager(config)

	if m == nil {
		t.Fatal("NewManager returned nil")
	}

	if m.config.StateDir != config.StateDir {
		t.Errorf("StateDir = %s, want %s", m.config.StateDir, config.StateDir)
	}

	if m.config.RestartDelay != config.RestartDelay {
		t.Errorf("RestartDelay = %v, want %v", m.config.RestartDelay, config.RestartDelay)
	}

	if m.clients == nil {
		t.Error("clients map is nil")
	}

	if m.servers == nil {
		t.Error("servers map is nil")
	}
}

func TestManagerDefaults(t *testing.T) {
	config := ManagerConfig{}
	m := NewManager(config)

	if m.config.StateDir == "" {
		t.Error("StateDir should have default value")
	}

	if m.config.RestartDelay == 0 {
		t.Error("RestartDelay should have default value")
	}
}

func TestAddClient(t *testing.T) {
	tempDir := t.TempDir()
	m := NewManager(ManagerConfig{StateDir: tempDir})

	mockBinary := filepath.Join(tempDir, "mock-pt")
	if err := os.WriteFile(mockBinary, []byte("#!/bin/sh\necho 'mock PT'\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	config := TransportConfig{
		BinaryPath: mockBinary,
		StateDir:   "",
	}

	if err := m.AddClient("test-pt", config); err != nil {
		t.Fatalf("AddClient failed: %v", err)
	}

	if len(m.clients) != 1 {
		t.Errorf("clients count = %d, want 1", len(m.clients))
	}

	client, err := m.GetClient("test-pt")
	if err != nil {
		t.Fatalf("GetClient failed: %v", err)
	}

	if client == nil {
		t.Fatal("GetClient returned nil")
	}

	expectedStateDir := filepath.Join(tempDir, "client", "test-pt")
	if client.config.StateDir != expectedStateDir {
		t.Errorf("client StateDir = %s, want %s", client.config.StateDir, expectedStateDir)
	}
}

func TestAddClientDuplicate(t *testing.T) {
	tempDir := t.TempDir()
	m := NewManager(ManagerConfig{StateDir: tempDir})

	mockBinary := filepath.Join(tempDir, "mock-pt")
	if err := os.WriteFile(mockBinary, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	config := TransportConfig{BinaryPath: mockBinary}

	if err := m.AddClient("test-pt", config); err != nil {
		t.Fatalf("First AddClient failed: %v", err)
	}

	if err := m.AddClient("test-pt", config); err == nil {
		t.Error("AddClient should fail for duplicate name")
	}
}

func TestAddServer(t *testing.T) {
	tempDir := t.TempDir()
	m := NewManager(ManagerConfig{StateDir: tempDir})

	mockBinary := filepath.Join(tempDir, "mock-pt")
	if err := os.WriteFile(mockBinary, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	config := TransportConfig{
		BinaryPath: mockBinary,
		StateDir:   "",
	}

	if err := m.AddServer("test-pt", config); err != nil {
		t.Fatalf("AddServer failed: %v", err)
	}

	if len(m.servers) != 1 {
		t.Errorf("servers count = %d, want 1", len(m.servers))
	}

	server, err := m.GetServer("test-pt")
	if err != nil {
		t.Fatalf("GetServer failed: %v", err)
	}

	if server == nil {
		t.Fatal("GetServer returned nil")
	}

	expectedStateDir := filepath.Join(tempDir, "server", "test-pt")
	if server.config.StateDir != expectedStateDir {
		t.Errorf("server StateDir = %s, want %s", server.config.StateDir, expectedStateDir)
	}
}

func TestClientsAndServers(t *testing.T) {
	tempDir := t.TempDir()
	m := NewManager(ManagerConfig{StateDir: tempDir})

	mockBinary := filepath.Join(tempDir, "mock-pt")
	if err := os.WriteFile(mockBinary, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	config := TransportConfig{BinaryPath: mockBinary}

	m.AddClient("client1", config)
	m.AddClient("client2", config)
	m.AddServer("server1", config)

	clients := m.Clients()
	if len(clients) != 2 {
		t.Errorf("Clients() returned %d items, want 2", len(clients))
	}

	servers := m.Servers()
	if len(servers) != 1 {
		t.Errorf("Servers() returned %d items, want 1", len(servers))
	}
}

func TestGetClientNotFound(t *testing.T) {
	m := NewManager(DefaultManagerConfig())

	_, err := m.GetClient("nonexistent")
	if err == nil {
		t.Error("GetClient should fail for nonexistent PT")
	}
}

func TestGetServerNotFound(t *testing.T) {
	m := NewManager(DefaultManagerConfig())

	_, err := m.GetServer("nonexistent")
	if err == nil {
		t.Error("GetServer should fail for nonexistent PT")
	}
}

func TestClose(t *testing.T) {
	tempDir := t.TempDir()
	m := NewManager(ManagerConfig{StateDir: tempDir, AutoRestart: false})

	mockBinary := filepath.Join(tempDir, "mock-pt")
	if err := os.WriteFile(mockBinary, []byte("#!/bin/sh\necho 'mock'\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	config := TransportConfig{BinaryPath: mockBinary}
	m.AddClient("test-client", config)
	m.AddServer("test-server", config)

	if err := m.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestStartAllWithoutAutoRestart(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping PT start test in short mode")
	}

	tempDir := t.TempDir()
	m := NewManager(ManagerConfig{
		StateDir:    tempDir,
		AutoRestart: false,
	})

	mockScript := filepath.Join(tempDir, "mock-pt")
	script := `#!/bin/sh
echo "VERSION 1"
echo "CMETHODS DONE"
`
	if err := os.WriteFile(mockScript, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	config := TransportConfig{BinaryPath: mockScript}
	if err := m.AddClient("test-pt", config); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := m.StartAll(ctx); err != nil {
		t.Logf("StartAll returned error (expected in test environment): %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	if err := m.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestMonitoringDisabledWhenAutoRestartOff(t *testing.T) {
	tempDir := t.TempDir()
	m := NewManager(ManagerConfig{
		StateDir:    tempDir,
		AutoRestart: false,
	})

	mockBinary := filepath.Join(tempDir, "mock-pt")
	if err := os.WriteFile(mockBinary, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	config := TransportConfig{BinaryPath: mockBinary}
	m.AddClient("test-pt", config)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	m.StartAll(ctx)

	time.Sleep(100 * time.Millisecond)

	select {
	case <-m.ctx.Done():
		t.Error("Manager context should not be cancelled")
	default:
	}

	m.Close()
}

func TestDefaultManagerConfig(t *testing.T) {
	config := DefaultManagerConfig()

	if config.StateDir == "" {
		t.Error("StateDir should not be empty")
	}

	if !config.AutoRestart {
		t.Error("AutoRestart should be true by default")
	}

	if config.RestartDelay != 5*time.Second {
		t.Errorf("RestartDelay = %v, want 5s", config.RestartDelay)
	}

	if config.MaxRestarts != 0 {
		t.Errorf("MaxRestarts = %d, want 0 (unlimited)", config.MaxRestarts)
	}
}
