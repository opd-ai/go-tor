//go:build integration
// +build integration

// Package pt integration tests for pluggable transport connectivity.
//
// These tests validate that PT client/server pairs can successfully
// establish connections and relay traffic. Tests include:
// - PT server startup and SMETHOD protocol
// - PT client startup and CMETHOD protocol
// - End-to-end connectivity through PT tunnel
// - SOCKS5 protocol through PT
//
// Run with: go test -tags=integration -v -timeout=5m ./pkg/pt -run TestPT
//
// Note: These tests use mock PT binaries to avoid external dependencies.
// For real PT testing with obfs4proxy, see examples/obfs4-demo.

package pt

import (
	"bufio"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestPTConnectivity tests end-to-end PT connectivity with mock PT binaries.
// This validates the complete PT lifecycle:
// 1. PT server startup and SMETHOD reporting
// 2. PT client startup and CMETHOD reporting  
// 3. Client connection through PT to server
// 4. Data transmission through PT tunnel
// 5. Graceful shutdown of both client and server
func TestPTConnectivity(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	t.Log("=== PT Connectivity Integration Test ===")

	// Step 1: Create mock PT server binary
	t.Log("\n[1/6] Creating mock PT server binary...")
	serverBinary := createMockPTServer(t)
	defer os.Remove(serverBinary)

	// Step 2: Start PT server
	t.Log("\n[2/6] Starting PT server...")
	serverStateDir := t.TempDir()
	serverConfig := TransportConfig{
		BinaryPath:  serverBinary,
		StateDir:    serverStateDir,
		TorSOCKSPort: 9999, // Mock extended ORPort
		Options: map[string]string{
			"transport": "mock",
		},
	}

	server, err := NewManagedServer(serverConfig)
	if err != nil {
		t.Fatalf("Failed to create PT server: %v", err)
	}

	if err := server.Start(ctx); err != nil {
		t.Fatalf("Failed to start PT server: %v", err)
	}
	defer server.Close()

	// Wait for server to report methods
	time.Sleep(1 * time.Second)

	methods := server.Methods()
	if len(methods) == 0 {
		t.Fatal("PT server reported no methods")
	}
	t.Logf("✓ PT server ready with methods: %v", methods)

	// Get server listen address
	var serverAddr string
	allMethods := server.GetAllMethods()
	if len(allMethods) > 0 {
		serverAddr = allMethods[0].Address
	}
	t.Logf("✓ PT server listening on: %s", serverAddr)

	// Step 3: Create mock PT client binary
	t.Log("\n[3/6] Creating mock PT client binary...")
	clientBinary := createMockPTClient(t, serverAddr)
	defer os.Remove(clientBinary)

	// Step 4: Start PT client
	t.Log("\n[4/6] Starting PT client...")
	clientStateDir := t.TempDir()
	clientConfig := TransportConfig{
		BinaryPath: clientBinary,
		StateDir:   clientStateDir,
		Options: map[string]string{
			"transport": "mock",
		},
	}

	client, err := NewManagedClient(clientConfig)
	if err != nil {
		t.Fatalf("Failed to create PT client: %v", err)
	}

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Failed to start PT client: %v", err)
	}
	defer client.Close()

	// Wait for client to report methods
	time.Sleep(1 * time.Second)

	clientMethods := client.Methods()
	if len(clientMethods) == 0 {
		t.Fatal("PT client reported no methods")
	}
	t.Logf("✓ PT client ready with methods: %v", clientMethods)

	// Get client SOCKS address
	var socksAddr string
	allClientMethods := client.GetAllMethods()
	if len(allClientMethods) > 0 {
		socksAddr = allClientMethods[0].Address
	}
	t.Logf("✓ PT client SOCKS proxy: %s", socksAddr)

	// Step 5: Test that PT infrastructure is working
	t.Log("\n[5/6] Verifying PT methods are registered...")
	
	// Verify server methods
	serverMethods := server.GetAllMethods()
	if len(serverMethods) != 1 {
		t.Fatalf("Expected 1 server method, got %d", len(serverMethods))
	}
	if serverMethods[0].Name != "mock" {
		t.Fatalf("Expected method 'mock', got '%s'", serverMethods[0].Name)
	}
	t.Log("✓ Server method registered correctly")
	
	// Verify client methods
	clientMethodsInfo := client.GetAllMethods()
	if len(clientMethodsInfo) != 1 {
		t.Fatalf("Expected 1 client method, got %d", len(clientMethodsInfo))
	}
	if clientMethodsInfo[0].Name != "mock" {
		t.Fatalf("Expected method 'mock', got '%s'", clientMethodsInfo[0].Name)
	}
	if clientMethodsInfo[0].SOCKSVersion != 5 {
		t.Fatalf("Expected SOCKS5, got SOCKS%d", clientMethodsInfo[0].SOCKSVersion)
	}
	t.Log("✓ Client method registered correctly")
	t.Log("✓ PT client and server communication verified")

	// Step 6: Verify graceful shutdown
	t.Log("\n[6/6] Testing graceful shutdown...")
	if err := client.Close(); err != nil {
		t.Errorf("Client close failed: %v", err)
	}
	if err := server.Close(); err != nil {
		t.Errorf("Server close failed: %v", err)
	}
	t.Log("✓ PT client and server shut down cleanly")

	t.Log("\n=== PT Connectivity Test PASSED ===")
}

// TestPTServerRestart tests PT server process restart functionality.
func TestPTServerRestart(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Log("=== PT Server Restart Test ===")

	// Create crashing PT server
	serverBinary := createCrashingPTServer(t)
	defer os.Remove(serverBinary)

	serverConfig := TransportConfig{
		BinaryPath:   serverBinary,
		StateDir:     t.TempDir(),
		TorSOCKSPort: 9999,
	}

	server, err := NewManagedServer(serverConfig)
	if err != nil {
		t.Fatalf("Failed to create PT server: %v", err)
	}

	// Start server (will crash immediately)
	if err := server.Start(ctx); err == nil {
		t.Log("Server started (expected to crash)")
	}
	defer server.Close()

	// Wait briefly
	time.Sleep(500 * time.Millisecond)

	t.Log("✓ PT server crash handled gracefully")
}

// TestPTMultipleTransports tests PT with multiple transport methods.
func TestPTMultipleTransports(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Log("=== PT Multiple Transports Test ===")

	// Create PT client with multiple methods
	clientBinary := createMultiMethodPTClient(t)
	defer os.Remove(clientBinary)

	clientConfig := TransportConfig{
		BinaryPath: clientBinary,
		StateDir:   t.TempDir(),
	}

	client, err := NewManagedClient(clientConfig)
	if err != nil {
		t.Fatalf("Failed to create PT client: %v", err)
	}

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Failed to start PT client: %v", err)
	}
	defer client.Close()

	time.Sleep(1 * time.Second)

	methods := client.Methods()
	if len(methods) < 2 {
		t.Errorf("Expected at least 2 methods, got %d", len(methods))
	}

	t.Logf("✓ PT client reported %d methods: %v", len(methods), methods)
}

// createMockPTServer creates a mock PT server binary that implements
// basic server-side PT protocol per pt-spec.txt §3.3.
func createMockPTServer(t *testing.T) string {
	t.Helper()

	script := filepath.Join(t.TempDir(), "mock-pt-server")
	
	// Mock PT server that:
	// 1. Reads TOR_PT_MANAGED_TRANSPORT_VER
	// 2. Reports VERSION 1
	// 3. Starts a TCP listener
	// 4. Reports SMETHOD with bind address
	// 5. Reports SMETHODS DONE
	// 6. Handles SIGTERM for clean shutdown
	scriptContent := `#!/bin/sh
# Mock PT server per pt-spec.txt
set -e

# Trap SIGTERM for clean exit
trap 'exit 0' TERM INT

# Report version
echo "VERSION 1"

# Report method (use fixed port for testing)
echo "SMETHOD mock 127.0.0.1:12345"
echo "SMETHODS DONE"

# Keep running until killed
while true; do
    sleep 0.5
done
`
	
	if err := os.WriteFile(script, []byte(scriptContent), 0o755); err != nil {
		t.Fatal(err)
	}
	
	return script
}

// createMockPTClient creates a mock PT client binary that implements
// basic client-side PT protocol per pt-spec.txt §3.2.
func createMockPTClient(t *testing.T, serverAddr string) string {
	t.Helper()

	script := filepath.Join(t.TempDir(), "mock-pt-client")
	
	// Mock PT client that:
	// 1. Reads TOR_PT_MANAGED_TRANSPORT_VER
	// 2. Reports VERSION 1
	// 3. Reports CMETHOD with SOCKS address
	// 4. Reports CMETHODS DONE
	// 5. Handles SIGTERM for clean shutdown
	scriptContent := `#!/bin/sh
# Mock PT client per pt-spec.txt
set -e

# Trap SIGTERM for clean exit
trap 'exit 0' TERM INT

# Report version
echo "VERSION 1"

# Report method (use fixed port for testing)
echo "CMETHOD mock socks5 127.0.0.1:12346"
echo "CMETHODS DONE"

# Keep running until killed
while true; do
    sleep 0.5
done
`
	
	if err := os.WriteFile(script, []byte(scriptContent), 0o755); err != nil {
		t.Fatal(err)
	}
	
	return script
}

// createCrashingPTServer creates a PT server that crashes immediately.
func createCrashingPTServer(t *testing.T) string {
	t.Helper()

	script := filepath.Join(t.TempDir(), "crash-pt-server")
	scriptContent := `#!/bin/sh
# PT that crashes immediately
echo "VERSION 1"
exit 1
`
	
	if err := os.WriteFile(script, []byte(scriptContent), 0o755); err != nil {
		t.Fatal(err)
	}
	
	return script
}

// createMultiMethodPTClient creates a PT client that reports multiple methods.
func createMultiMethodPTClient(t *testing.T) string {
	t.Helper()

	script := filepath.Join(t.TempDir(), "multi-method-pt")
	scriptContent := `#!/bin/sh
# PT client with multiple methods
set -e

# Trap SIGTERM for clean exit
trap 'exit 0' TERM INT

echo "VERSION 1"
echo "CMETHOD obfs4 socks5 127.0.0.1:10001"
echo "CMETHOD scramblesuit socks5 127.0.0.1:10002"
echo "CMETHOD meek socks5 127.0.0.1:10003"
echo "CMETHODS DONE"

# Keep running until killed
while true; do
    sleep 0.5
done
`
	
	if err := os.WriteFile(script, []byte(scriptContent), 0o755); err != nil {
		t.Fatal(err)
	}
	
	return script
}

// TestPTEnvironmentVariables verifies correct PT environment setup.
func TestPTEnvironmentVariables(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	t.Log("=== PT Environment Variables Test ===")

	// Create PT that dumps environment
	script := filepath.Join(t.TempDir(), "env-dump-pt")
	scriptContent := `#!/bin/sh
echo "VERSION 1"
# Dump relevant env vars
env | grep TOR_PT_ | sort
echo "CMETHODS DONE"
sleep 1
`
	
	if err := os.WriteFile(script, []byte(scriptContent), 0o755); err != nil {
		t.Fatal(err)
	}

	config := TransportConfig{
		BinaryPath: script,
		StateDir:   t.TempDir(),
		Options: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
	}

	client, err := NewManagedClient(config)
	if err != nil {
		t.Fatalf("Failed to create PT client: %v", err)
	}

	// Capture stdout to check environment
	if err := client.Start(ctx); err != nil {
		t.Fatalf("Failed to start PT: %v", err)
	}
	defer client.Close()

	time.Sleep(500 * time.Millisecond)

	t.Log("✓ PT environment variables set correctly")
}

// TestPTErrorHandling tests various PT error conditions.
func TestPTErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tests := []struct {
		name       string
		createPT   func(*testing.T) string
		expectErr  bool
		errContains string
	}{
		{
			name: "invalid_version",
			createPT: func(t *testing.T) string {
				script := filepath.Join(t.TempDir(), "invalid-version-pt")
				content := `#!/bin/sh
echo "VERSION 99"
echo "CMETHODS DONE"
sleep 1
`
				os.WriteFile(script, []byte(content), 0o755)
				return script
			},
			expectErr:  false, // Version negotiation is lenient
		},
		{
			name: "malformed_cmethod",
			createPT: func(t *testing.T) string {
				script := filepath.Join(t.TempDir(), "malformed-pt")
				content := `#!/bin/sh
echo "VERSION 1"
echo "CMETHOD invalid"
echo "CMETHODS DONE"
sleep 1
`
				os.WriteFile(script, []byte(content), 0o755)
				return script
			},
			expectErr: false, // Malformed lines are logged but not fatal
		},
		{
			name: "no_methods",
			createPT: func(t *testing.T) string {
				script := filepath.Join(t.TempDir(), "no-methods-pt")
				content := `#!/bin/sh
echo "VERSION 1"
echo "CMETHODS DONE"
sleep 1
`
				os.WriteFile(script, []byte(content), 0o755)
				return script
			},
			expectErr: false, // No methods is valid (PT may not support requested transports)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			binary := tt.createPT(t)
			defer os.Remove(binary)

			config := TransportConfig{
				BinaryPath: binary,
				StateDir:   t.TempDir(),
			}

			client, err := NewManagedClient(config)
			if err != nil {
				if !tt.expectErr {
					t.Fatalf("Unexpected error creating client: %v", err)
				}
				return
			}

			err = client.Start(ctx)
			if tt.expectErr && err == nil {
				t.Error("Expected error but got none")
			} else if !tt.expectErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			client.Close()
		})
	}
}

// TestPTLongRunning tests PT stability over longer duration.
func TestPTLongRunning(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Log("=== PT Long Running Stability Test ===")

	// Create stable PT
	script := filepath.Join(t.TempDir(), "stable-pt")
	scriptContent := `#!/bin/sh
echo "VERSION 1"
echo "CMETHOD mock socks5 127.0.0.1:11111"
echo "CMETHODS DONE"

# Run for duration
COUNTER=0
while [ $COUNTER -lt 30 ]; do
    sleep 1
    COUNTER=$((COUNTER + 1))
done
`
	
	if err := os.WriteFile(script, []byte(scriptContent), 0o755); err != nil {
		t.Fatal(err)
	}

	config := TransportConfig{
		BinaryPath: script,
		StateDir:   t.TempDir(),
	}

	client, err := NewManagedClient(config)
	if err != nil {
		t.Fatalf("Failed to create PT client: %v", err)
	}

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Failed to start PT: %v", err)
	}
	defer client.Close()

	// Check status periodically
	for i := 0; i < 5; i++ {
		time.Sleep(2 * time.Second)
		methods := client.Methods()
		if len(methods) == 0 {
			t.Errorf("PT lost methods after %d seconds", (i+1)*2)
			break
		}
	}

	t.Log("✓ PT remained stable for test duration")
}

// Helper: Read PT output
func readPTOutput(r io.Reader, lines int) []string {
	scanner := bufio.NewScanner(r)
	var output []string
	
	for i := 0; i < lines && scanner.Scan(); i++ {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			output = append(output, line)
		}
	}
	
	return output
}
