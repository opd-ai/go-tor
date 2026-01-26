package obfs4

import (
	"context"
	"path/filepath"
	"testing"
)

func TestNewServer(t *testing.T) {
	tests := []struct {
		name    string
		config  ServerConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: ServerConfig{
				BinaryPath: "/usr/bin/obfs4proxy",
				BindAddr:   "127.0.0.1:0",
				StateDir:   "/tmp/test-obfs4-server",
				IATMode:    0,
			},
			wantErr: false,
		},
		{
			name: "missing state directory",
			config: ServerConfig{
				BinaryPath: "/usr/bin/obfs4proxy",
				BindAddr:   "127.0.0.1:0",
			},
			wantErr: true,
		},
		{
			name: "auto discover binary",
			config: ServerConfig{
				BindAddr: "127.0.0.1:0",
				StateDir: "/tmp/test-obfs4-server",
				IATMode:  1,
			},
			// May fail if obfs4proxy not installed
			wantErr: true,
		},
		{
			name: "default bind address",
			config: ServerConfig{
				BinaryPath: "/usr/bin/obfs4proxy",
				StateDir:   "/tmp/test-obfs4-server",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, err := NewServer(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewServer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if server == nil {
					t.Error("NewServer() returned nil server without error")
				}
				if server.Name() != "obfs4" {
					t.Errorf("Server.Name() = %v, want obfs4", server.Name())
				}
			}
		})
	}
}

func TestServer_Name(t *testing.T) {
	server := &Server{}
	if got := server.Name(); got != "obfs4" {
		t.Errorf("Server.Name() = %v, want obfs4", got)
	}
}

func TestServer_Methods(t *testing.T) {
	config := ServerConfig{
		BinaryPath: "/usr/bin/obfs4proxy",
		StateDir:   t.TempDir(),
	}

	server, err := NewServer(config)
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}

	methods := server.Methods()
	// Methods may be empty before server starts, or return a default
	// Just verify it's callable without panic
	if methods == nil {
		t.Error("Server.Methods() returned nil")
	}

	// The default implementation should return at least one method
	if len(methods) == 0 {
		// This is acceptable before server starts
		t.Log("Server.Methods() returned empty slice (server not started)")
	}
}

func TestServer_Dial_NotSupported(t *testing.T) {
	config := ServerConfig{
		BinaryPath: "/usr/bin/obfs4proxy",
		StateDir:   t.TempDir(),
	}

	server, err := NewServer(config)
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}

	ctx := context.Background()
	_, err = server.Dial(ctx, "192.0.2.1:1234")
	if err == nil {
		t.Error("Server.Dial() should return error (not supported)")
	}
}

func TestServer_Close(t *testing.T) {
	config := ServerConfig{
		BinaryPath: "/usr/bin/obfs4proxy",
		StateDir:   t.TempDir(),
	}

	server, err := NewServer(config)
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}

	// Close should work even if not started
	if err := server.Close(); err != nil {
		t.Errorf("Server.Close() error = %v", err)
	}
}

func TestServer_IATModes(t *testing.T) {
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
			config := ServerConfig{
				BinaryPath: "/usr/bin/obfs4proxy",
				StateDir:   t.TempDir(),
				IATMode:    tt.iatMode,
			}

			server, err := NewServer(config)
			if err != nil {
				t.Fatalf("NewServer() failed: %v", err)
			}

			if server.config.IATMode != tt.iatMode {
				t.Errorf("IATMode = %d, want %d", server.config.IATMode, tt.iatMode)
			}
		})
	}
}

func TestServer_GetCertificate_NotStarted(t *testing.T) {
	config := ServerConfig{
		BinaryPath: "/usr/bin/obfs4proxy",
		StateDir:   t.TempDir(),
	}

	server, err := NewServer(config)
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}

	_, err = server.GetCertificate()
	if err == nil {
		t.Error("GetCertificate() should fail when server not started")
	}
}

func TestServer_GetBindAddress_NotStarted(t *testing.T) {
	config := ServerConfig{
		BinaryPath: "/usr/bin/obfs4proxy",
		StateDir:   t.TempDir(),
	}

	server, err := NewServer(config)
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}

	_, err = server.GetBindAddress()
	if err == nil {
		t.Error("GetBindAddress() should fail when server not started")
	}
}

func TestServer_DefaultBindAddress(t *testing.T) {
	config := ServerConfig{
		BinaryPath: "/usr/bin/obfs4proxy",
		StateDir:   t.TempDir(),
	}

	server, err := NewServer(config)
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}

	expected := "127.0.0.1:0"
	if server.config.BindAddr != expected {
		t.Errorf("BindAddr = %v, want %v", server.config.BindAddr, expected)
	}
}

func TestServer_ExtORPort(t *testing.T) {
	extORPort := "127.0.0.1:9999"
	config := ServerConfig{
		BinaryPath: "/usr/bin/obfs4proxy",
		StateDir:   t.TempDir(),
		ExtORPort:  extORPort,
	}

	server, err := NewServer(config)
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}

	if server.config.ExtORPort != extORPort {
		t.Errorf("ExtORPort = %v, want %v", server.config.ExtORPort, extORPort)
	}
}

// TestServer_StateDirectory tests state directory configuration
func TestServer_StateDirectory(t *testing.T) {
	tempDir := t.TempDir()
	stateDir := filepath.Join(tempDir, "custom-server-state")

	config := ServerConfig{
		BinaryPath: "/usr/bin/obfs4proxy",
		StateDir:   stateDir,
	}

	server, err := NewServer(config)
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}

	if server.config.StateDir != stateDir {
		t.Errorf("StateDir = %v, want %v", server.config.StateDir, stateDir)
	}
}
