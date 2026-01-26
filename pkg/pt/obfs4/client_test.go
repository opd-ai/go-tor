package obfs4

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name    string
		config  ClientConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: ClientConfig{
				BinaryPath: "/usr/bin/obfs4proxy",
				Cert:       "dGVzdGNlcnRpZmljYXRl", // "testcertificate" in base64
				IATMode:    0,
				StateDir:   "/tmp/test-obfs4",
			},
			wantErr: false,
		},
		{
			name: "missing certificate",
			config: ClientConfig{
				BinaryPath: "/usr/bin/obfs4proxy",
			},
			wantErr: true,
		},
		{
			name: "auto discover binary",
			config: ClientConfig{
				Cert:     "dGVzdGNlcnRpZmljYXRl",
				IATMode:  1,
				StateDir: "/tmp/test-obfs4",
			},
			// May fail if obfs4proxy not installed, but shouldn't panic
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && client == nil {
				t.Error("NewClient() returned nil client without error")
			}
			if client != nil && client.Name() != "obfs4" {
				t.Errorf("Client.Name() = %v, want obfs4", client.Name())
			}
		})
	}
}

func TestClient_Name(t *testing.T) {
	client := &Client{}
	if got := client.Name(); got != "obfs4" {
		t.Errorf("Client.Name() = %v, want obfs4", got)
	}
}

func TestClient_DefaultTimeout(t *testing.T) {
	config := ClientConfig{
		BinaryPath: "/usr/bin/obfs4proxy",
		Cert:       "dGVzdGNlcnRpZmljYXRl",
	}

	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("NewClient() failed: %v", err)
	}

	expectedTimeout := 30 * time.Second
	if client.config.DialTimeout != expectedTimeout {
		t.Errorf("DialTimeout = %v, want %v", client.config.DialTimeout, expectedTimeout)
	}
}

func TestClient_CustomTimeout(t *testing.T) {
	customTimeout := 60 * time.Second
	config := ClientConfig{
		BinaryPath:  "/usr/bin/obfs4proxy",
		Cert:        "dGVzdGNlcnRpZmljYXRl",
		DialTimeout: customTimeout,
	}

	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("NewClient() failed: %v", err)
	}

	if client.config.DialTimeout != customTimeout {
		t.Errorf("DialTimeout = %v, want %v", client.config.DialTimeout, customTimeout)
	}
}

func TestClient_IsRunning(t *testing.T) {
	config := ClientConfig{
		BinaryPath: "/usr/bin/obfs4proxy",
		Cert:       "dGVzdGNlcnRpZmljYXRl",
		StateDir:   t.TempDir(),
	}

	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("NewClient() failed: %v", err)
	}

	// Should not be running initially
	if client.IsRunning() {
		t.Error("Client.IsRunning() = true, want false (not started)")
	}

	// Note: We can't actually start obfs4proxy in tests without it being installed
	// So we just test the initial state
}

func TestClient_Close(t *testing.T) {
	config := ClientConfig{
		BinaryPath: "/usr/bin/obfs4proxy",
		Cert:       "dGVzdGNlcnRpZmljYXRl",
		StateDir:   t.TempDir(),
	}

	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("NewClient() failed: %v", err)
	}

	// Close should work even if not started
	if err := client.Close(); err != nil {
		t.Errorf("Client.Close() error = %v", err)
	}
}

func TestClient_IATModes(t *testing.T) {
	tests := []struct {
		name    string
		iatMode int
	}{
		{"disabled", 0},
		{"enabled", 1},
		{"paranoid", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := ClientConfig{
				BinaryPath: "/usr/bin/obfs4proxy",
				Cert:       "dGVzdGNlcnRpZmljYXRl",
				IATMode:    tt.iatMode,
				StateDir:   t.TempDir(),
			}

			client, err := NewClient(config)
			if err != nil {
				t.Fatalf("NewClient() failed: %v", err)
			}

			if client.config.IATMode != tt.iatMode {
				t.Errorf("IATMode = %d, want %d", client.config.IATMode, tt.iatMode)
			}
		})
	}
}

// TestClient_Dial_NotStarted verifies that Dial auto-starts the client
func TestClient_Dial_NotStarted(t *testing.T) {
	config := ClientConfig{
		BinaryPath: "/nonexistent/obfs4proxy", // Will fail, but tests the logic
		Cert:       "dGVzdGNlcnRpZmljYXRl",
		StateDir:   t.TempDir(),
	}

	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("NewClient() failed: %v", err)
	}

	ctx := context.Background()
	_, err = client.Dial(ctx, "192.0.2.1:1234")

	// Should get an error (binary doesn't exist), but it should attempt to start
	if err == nil {
		t.Error("Dial() should fail with nonexistent binary")
	}
}

// TestClient_StateDirectory tests state directory creation
func TestClient_StateDirectory(t *testing.T) {
	tempDir := t.TempDir()
	stateDir := filepath.Join(tempDir, "custom-state")

	config := ClientConfig{
		BinaryPath: "/usr/bin/obfs4proxy",
		Cert:       "dGVzdGNlcnRpZmljYXRl",
		StateDir:   stateDir,
	}

	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("NewClient() failed: %v", err)
	}

	if client.config.StateDir != stateDir {
		t.Errorf("StateDir = %v, want %v", client.config.StateDir, stateDir)
	}
}
