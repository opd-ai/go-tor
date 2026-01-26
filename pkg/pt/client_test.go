package pt

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewManagedClient(t *testing.T) {
	tests := []struct {
		name    string
		config  TransportConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: TransportConfig{
				BinaryPath: "/usr/bin/obfs4proxy",
				StateDir:   "/tmp/test-pt",
			},
			wantErr: false,
		},
		{
			name: "missing binary path",
			config: TransportConfig{
				StateDir: "/tmp/test-pt",
			},
			wantErr: true,
		},
		{
			name: "auto state dir",
			config: TransportConfig{
				BinaryPath: "/usr/bin/obfs4proxy",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewManagedClient(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewManagedClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && client == nil {
				t.Error("NewManagedClient() returned nil client")
			}
		})
	}
}

func TestManagedClient_Name(t *testing.T) {
	tests := []struct {
		binaryPath string
		wantName   string
	}{
		{"/usr/bin/obfs4proxy", "obfs4proxy"},
		{"/opt/pt/meek-client", "meek-client"},
		{"snowflake-client", "snowflake-client"},
	}

	for _, tt := range tests {
		t.Run(tt.binaryPath, func(t *testing.T) {
			client, _ := NewManagedClient(TransportConfig{
				BinaryPath: tt.binaryPath,
			})
			if got := client.Name(); got != tt.wantName {
				t.Errorf("Name() = %v, want %v", got, tt.wantName)
			}
		})
	}
}

func TestManagedClient_BuildEnvironment(t *testing.T) {
	config := TransportConfig{
		BinaryPath: "/usr/bin/obfs4proxy",
		StateDir:   "/tmp/test-state",
		ProxyURL:   "socks5://127.0.0.1:9050",
		Options: map[string]string{
			"cert": "test-cert",
			"mode": "1",
		},
	}

	client, err := NewManagedClient(config)
	if err != nil {
		t.Fatalf("NewManagedClient() failed: %v", err)
	}

	env := client.buildEnvironment()

	requiredVars := map[string]string{
		"TOR_PT_MANAGED_TRANSPORT_VER": "1",
		"TOR_PT_STATE_LOCATION":        "/tmp/test-state",
		"TOR_PT_CLIENT_TRANSPORTS":     "*",
		"TOR_PT_PROXY":                 "socks5://127.0.0.1:9050",
	}

	for key, expectedValue := range requiredVars {
		found := false
		for _, envVar := range env {
			if strings.HasPrefix(envVar, key+"=") {
				parts := strings.SplitN(envVar, "=", 2)
				if len(parts) == 2 && parts[1] == expectedValue {
					found = true
					break
				}
			}
		}
		if !found {
			t.Errorf("Environment missing or incorrect: %s=%s", key, expectedValue)
		}
	}

	hasOptCert := false
	for _, envVar := range env {
		if strings.HasPrefix(envVar, "TOR_PT_OPT_CERT=") {
			hasOptCert = true
			break
		}
	}
	if !hasOptCert {
		t.Error("Environment missing TOR_PT_OPT_CERT")
	}
}

func TestManagedClient_ParseCMethod(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		wantErr bool
		check   func(*MethodInfo) error
	}{
		{
			name:    "valid socks5",
			line:    "CMETHOD obfs4 socks5 127.0.0.1:1234",
			wantErr: false,
			check: func(m *MethodInfo) error {
				if m.Name != "obfs4" {
					return fmt.Errorf("name = %s, want obfs4", m.Name)
				}
				if m.SOCKSVersion != 5 {
					return fmt.Errorf("SOCKSVersion = %d, want 5", m.SOCKSVersion)
				}
				if m.Address != "127.0.0.1:1234" {
					return fmt.Errorf("Address = %s, want 127.0.0.1:1234", m.Address)
				}
				return nil
			},
		},
		{
			name:    "valid socks4",
			line:    "CMETHOD meek socks4 127.0.0.1:5678",
			wantErr: false,
			check: func(m *MethodInfo) error {
				if m.SOCKSVersion != 4 {
					return fmt.Errorf("SOCKSVersion = %d, want 4", m.SOCKSVersion)
				}
				return nil
			},
		},
		{
			name:    "with args",
			line:    "CMETHOD obfs4 socks5 127.0.0.1:1234 arg1=value1 arg2=value2",
			wantErr: false,
			check: func(m *MethodInfo) error {
				if m.Args["arg1"] != "value1" {
					return fmt.Errorf("Args[arg1] = %s, want value1", m.Args["arg1"])
				}
				if m.Args["arg2"] != "value2" {
					return fmt.Errorf("Args[arg2] = %s, want value2", m.Args["arg2"])
				}
				return nil
			},
		},
		{
			name:    "invalid format",
			line:    "CMETHOD obfs4",
			wantErr: true,
		},
		{
			name:    "invalid SOCKS version",
			line:    "CMETHOD obfs4 socks3 127.0.0.1:1234",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, _ := NewManagedClient(TransportConfig{
				BinaryPath: "/usr/bin/test",
			})

			err := client.parseCMethod(tt.line)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseCMethod() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.check != nil {
				client.mu.RLock()
				var method *MethodInfo
				for _, m := range client.methods {
					method = m
					break
				}
				client.mu.RUnlock()

				if method == nil {
					t.Fatal("method not registered")
				}

				if err := tt.check(method); err != nil {
					t.Error(err)
				}
			}
		})
	}
}

func TestManagedClient_Methods(t *testing.T) {
	client, _ := NewManagedClient(TransportConfig{
		BinaryPath: "/usr/bin/test",
	})

	client.parseCMethod("CMETHOD obfs4 socks5 127.0.0.1:1234")
	client.parseCMethod("CMETHOD meek socks5 127.0.0.1:5678")

	methods := client.Methods()
	if len(methods) != 2 {
		t.Errorf("Methods() returned %d methods, want 2", len(methods))
	}

	methodSet := make(map[string]bool)
	for _, m := range methods {
		methodSet[m] = true
	}

	if !methodSet["obfs4"] || !methodSet["meek"] {
		t.Errorf("Methods() = %v, want [obfs4 meek]", methods)
	}
}

func TestManagedClient_IsRunning(t *testing.T) {
	client, _ := NewManagedClient(TransportConfig{
		BinaryPath: "/usr/bin/test",
	})

	if client.IsRunning() {
		t.Error("IsRunning() = true, want false for new client")
	}

	client.mu.Lock()
	client.running = true
	client.mu.Unlock()

	if !client.IsRunning() {
		t.Error("IsRunning() = false, want true after setting running")
	}
}

func TestManagedClient_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	mockPT := createMockPT(t)
	defer os.Remove(mockPT)

	config := TransportConfig{
		BinaryPath: mockPT,
		StateDir:   t.TempDir(),
	}

	client, err := NewManagedClient(config)
	if err != nil {
		t.Fatalf("NewManagedClient() failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	defer client.Close()

	if !client.IsRunning() {
		t.Error("IsRunning() = false after successful start")
	}

	methods := client.Methods()
	if len(methods) == 0 {
		t.Error("Methods() returned empty after handshake")
	}
}

func createMockPT(t *testing.T) string {
	tmpDir := t.TempDir()
	mockScript := filepath.Join(tmpDir, "mock-pt")

	script := `#!/bin/sh
set -e
trap 'exit 0' TERM INT
echo "VERSION 1"
echo "CMETHOD obfs4 socks5 127.0.0.1:9999"
echo "CMETHODS DONE"
while true; do sleep 0.5; done
`

	if err := os.WriteFile(mockScript, []byte(script), 0o755); err != nil {
		t.Fatalf("Failed to create mock PT: %v", err)
	}

	return mockScript
}

func TestManagedClient_Close(t *testing.T) {
	client, _ := NewManagedClient(TransportConfig{
		BinaryPath: "/usr/bin/test",
	})

	client.mu.Lock()
	client.running = true
	client.cmd = exec.Command("sleep", "1")
	client.cmd.Start()
	client.mu.Unlock()

	if err := client.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	if client.IsRunning() {
		t.Error("IsRunning() = true after Close()")
	}
}

func TestManagedClient_Dial_NotReady(t *testing.T) {
	client, _ := NewManagedClient(TransportConfig{
		BinaryPath: "/usr/bin/test",
	})

	ctx := context.Background()
	_, err := client.Dial(ctx, "example.com:443")
	if err == nil {
		t.Error("Dial() succeeded on non-running client, want error")
	}
}

type mockSOCKS5Server struct {
	listener net.Listener
	t        *testing.T
}

func newMockSOCKS5Server(t *testing.T) *mockSOCKS5Server {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create mock SOCKS5 server: %v", err)
	}

	server := &mockSOCKS5Server{
		listener: listener,
		t:        t,
	}

	go server.serve()
	return server
}

func (s *mockSOCKS5Server) serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handleConnection(conn)
	}
}

func (s *mockSOCKS5Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	buf := make([]byte, 256)
	n, _ := conn.Read(buf)
	if n < 3 || buf[0] != 0x05 {
		return
	}

	conn.Write([]byte{0x05, 0x00})

	n, _ = conn.Read(buf)
	if n < 10 || buf[0] != 0x05 || buf[1] != 0x01 {
		return
	}

	reply := []byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0}
	conn.Write(reply)

	io.Copy(io.Discard, conn)
}

func (s *mockSOCKS5Server) Close() {
	s.listener.Close()
}

func (s *mockSOCKS5Server) Addr() string {
	return s.listener.Addr().String()
}

func TestManagedClient_SOCKS5Handshake(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	mockServer := newMockSOCKS5Server(t)
	defer mockServer.Close()

	client, _ := NewManagedClient(TransportConfig{
		BinaryPath: "/usr/bin/test",
	})

	conn, err := net.Dial("tcp", mockServer.Addr())
	if err != nil {
		t.Fatalf("Failed to connect to mock server: %v", err)
	}
	defer conn.Close()

	if err := client.socks5Handshake(conn, "example.com:443"); err != nil {
		t.Errorf("socks5Handshake() error = %v", err)
	}
}
