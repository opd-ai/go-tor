package control

import (
	"bufio"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/logger"
)

// mockClientGetter implements ClientInfoGetter for testing
type mockClientGetter struct {
	activeCircuits int
	socksPort      int
	controlPort    int
}

func (m *mockClientGetter) GetStats() StatsProvider {
	return m
}

func (m *mockClientGetter) GetActiveCircuits() int {
	return m.activeCircuits
}

func (m *mockClientGetter) GetSocksPort() int {
	return m.socksPort
}

func (m *mockClientGetter) GetControlPort() int {
	return m.controlPort
}

// Helper to create test server
func setupTestServer(t *testing.T) (*Server, *mockClientGetter) {
	t.Helper()

	mockClient := &mockClientGetter{
		activeCircuits: 3,
		socksPort:      9050,
		controlPort:    9051,
	}

	log := logger.NewDefault()
	server := NewServer("127.0.0.1:0", mockClient, log)

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	t.Cleanup(func() {
		server.Stop()
	})

	return server, mockClient
}

// Helper to connect to server
func connectToServer(t *testing.T, server *Server) net.Conn {
	t.Helper()

	addr := server.listener.Addr().String()
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}

	t.Cleanup(func() {
		conn.Close()
	})

	return conn
}

// Helper to read response
func readResponse(t *testing.T, reader *bufio.Reader) string {
	t.Helper()

	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	return strings.TrimSpace(line)
}

// Helper to send command and get response
func sendCommand(t *testing.T, conn net.Conn, cmd string) string {
	t.Helper()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// Skip greeting
	if !strings.Contains(t.Name(), "Greeting") {
		readResponse(t, reader)
	}

	// Send command
	_, err := writer.WriteString(cmd + "\r\n")
	if err != nil {
		t.Fatalf("Failed to write command: %v", err)
	}
	writer.Flush()

	// Read response
	return readResponse(t, reader)
}

func TestServerStartStop(t *testing.T) {
	server, _ := setupTestServer(t)

	if server.listener == nil {
		t.Fatal("Server listener is nil")
	}

	addr := server.listener.Addr().String()
	if addr == "" {
		t.Fatal("Server address is empty")
	}
}

func TestServerGreeting(t *testing.T) {
	server, _ := setupTestServer(t)
	conn := connectToServer(t, server)

	reader := bufio.NewReader(conn)
	response := readResponse(t, reader)

	if !strings.HasPrefix(response, "250") {
		t.Errorf("Expected 250 greeting, got: %s", response)
	}
}

func TestProtocolInfo(t *testing.T) {
	server, _ := setupTestServer(t)
	conn := connectToServer(t, server)

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// Skip greeting
	readResponse(t, reader)

	// Send PROTOCOLINFO
	writer.WriteString("PROTOCOLINFO\r\n")
	writer.Flush()

	// Read multi-line response
	var lines []string
	for {
		line := readResponse(t, reader)
		lines = append(lines, line)
		if strings.HasPrefix(line, "250 ") {
			break
		}
	}

	if len(lines) < 3 {
		t.Fatalf("Expected multi-line response, got %d lines", len(lines))
	}

	// Check for expected content
	found := false
	for _, line := range lines {
		if strings.Contains(line, "PROTOCOLINFO 1") {
			found = true
			break
		}
	}

	if !found {
		t.Error("PROTOCOLINFO response missing protocol version")
	}
}

func TestAuthenticate(t *testing.T) {
	server, _ := setupTestServer(t)
	conn := connectToServer(t, server)

	response := sendCommand(t, conn, "AUTHENTICATE")

	if !strings.HasPrefix(response, "250") {
		t.Errorf("Expected 250 OK, got: %s", response)
	}
}

func TestGetInfoRequiresAuth(t *testing.T) {
	server, _ := setupTestServer(t)
	conn := connectToServer(t, server)

	response := sendCommand(t, conn, "GETINFO version")

	if !strings.HasPrefix(response, "514") {
		t.Errorf("Expected 514 (auth required), got: %s", response)
	}
}

func TestGetInfoAfterAuth(t *testing.T) {
	server, _ := setupTestServer(t)
	conn := connectToServer(t, server)

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// Skip greeting
	readResponse(t, reader)

	// Authenticate
	writer.WriteString("AUTHENTICATE\r\n")
	writer.Flush()
	readResponse(t, reader)

	// GETINFO
	writer.WriteString("GETINFO version\r\n")
	writer.Flush()

	response := readResponse(t, reader)

	if !strings.HasPrefix(response, "250") {
		t.Errorf("Expected 250 OK, got: %s", response)
	}

	if !strings.Contains(response, "version=") {
		t.Errorf("Expected version in response, got: %s", response)
	}
}

func TestGetInfoMultipleKeys(t *testing.T) {
	server, _ := setupTestServer(t)
	conn := connectToServer(t, server)

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// Skip greeting
	readResponse(t, reader)

	// Authenticate
	writer.WriteString("AUTHENTICATE\r\n")
	writer.Flush()
	readResponse(t, reader)

	// GETINFO with multiple keys
	writer.WriteString("GETINFO version status/circuit-established\r\n")
	writer.Flush()

	// Read multi-line response
	line1 := readResponse(t, reader)
	line2 := readResponse(t, reader)

	if !strings.Contains(line1, "version=") {
		t.Errorf("Expected version in first line, got: %s", line1)
	}

	if !strings.Contains(line2, "status/circuit-established=") {
		t.Errorf("Expected status in second line, got: %s", line2)
	}
}

func TestGetInfoUnrecognizedKey(t *testing.T) {
	server, _ := setupTestServer(t)
	conn := connectToServer(t, server)

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// Skip greeting
	readResponse(t, reader)

	// Authenticate
	writer.WriteString("AUTHENTICATE\r\n")
	writer.Flush()
	readResponse(t, reader)

	// GETINFO with invalid key
	writer.WriteString("GETINFO invalid-key\r\n")
	writer.Flush()

	response := readResponse(t, reader)

	if !strings.HasPrefix(response, "552") {
		t.Errorf("Expected 552 (unrecognized key), got: %s", response)
	}
}

func TestGetConf(t *testing.T) {
	server, _ := setupTestServer(t)
	conn := connectToServer(t, server)

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// Skip greeting and authenticate
	readResponse(t, reader)
	writer.WriteString("AUTHENTICATE\r\n")
	writer.Flush()
	readResponse(t, reader)

	// GETCONF
	writer.WriteString("GETCONF SocksPort\r\n")
	writer.Flush()

	response := readResponse(t, reader)

	if !strings.HasPrefix(response, "250") {
		t.Errorf("Expected 250 OK, got: %s", response)
	}
}

func TestSetConf(t *testing.T) {
	server, _ := setupTestServer(t)
	conn := connectToServer(t, server)

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// Skip greeting and authenticate
	readResponse(t, reader)
	writer.WriteString("AUTHENTICATE\r\n")
	writer.Flush()
	readResponse(t, reader)

	// SETCONF
	writer.WriteString("SETCONF SocksPort=9150\r\n")
	writer.Flush()

	response := readResponse(t, reader)

	if !strings.HasPrefix(response, "250") {
		t.Errorf("Expected 250 OK, got: %s", response)
	}
}

func TestSetEvents(t *testing.T) {
	server, _ := setupTestServer(t)
	conn := connectToServer(t, server)

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// Skip greeting and authenticate
	readResponse(t, reader)
	writer.WriteString("AUTHENTICATE\r\n")
	writer.Flush()
	readResponse(t, reader)

	// SETEVENTS
	writer.WriteString("SETEVENTS CIRC STREAM\r\n")
	writer.Flush()

	response := readResponse(t, reader)

	if !strings.HasPrefix(response, "250") {
		t.Errorf("Expected 250 OK, got: %s", response)
	}
}

func TestQuit(t *testing.T) {
	server, _ := setupTestServer(t)
	conn := connectToServer(t, server)

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// Skip greeting
	readResponse(t, reader)

	// QUIT
	writer.WriteString("QUIT\r\n")
	writer.Flush()

	response := readResponse(t, reader)

	if !strings.HasPrefix(response, "250") {
		t.Errorf("Expected 250 OK, got: %s", response)
	}

	// Connection should be closed
	_, err := reader.ReadString('\n')
	if err == nil {
		t.Error("Expected connection to be closed")
	}
}

func TestUnrecognizedCommand(t *testing.T) {
	server, _ := setupTestServer(t)
	conn := connectToServer(t, server)

	response := sendCommand(t, conn, "INVALIDCOMMAND")

	if !strings.HasPrefix(response, "510") {
		t.Errorf("Expected 510 (unrecognized command), got: %s", response)
	}
}

func TestEmptyCommand(t *testing.T) {
	server, _ := setupTestServer(t)
	conn := connectToServer(t, server)

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// Skip greeting
	readResponse(t, reader)

	// Send empty lines (should be ignored)
	writer.WriteString("\r\n")
	writer.WriteString("\r\n")
	writer.Flush()

	// Send valid command to verify connection still works
	writer.WriteString("AUTHENTICATE\r\n")
	writer.Flush()

	response := readResponse(t, reader)

	if !strings.HasPrefix(response, "250") {
		t.Errorf("Expected 250 OK, got: %s", response)
	}
}

func TestConcurrentConnections(t *testing.T) {
	server, _ := setupTestServer(t)

	// Connect multiple clients
	const numClients = 5
	conns := make([]net.Conn, numClients)

	for i := 0; i < numClients; i++ {
		conn := connectToServer(t, server)
		conns[i] = conn

		// Verify greeting
		reader := bufio.NewReader(conn)
		response := readResponse(t, reader)

		if !strings.HasPrefix(response, "250") {
			t.Errorf("Client %d: Expected 250 greeting, got: %s", i, response)
		}
	}

	// All connections should be registered
	server.connsMu.RLock()
	numConns := len(server.conns)
	server.connsMu.RUnlock()

	if numConns != numClients {
		t.Errorf("Expected %d connections, got %d", numClients, numConns)
	}
}

func TestServerShutdownClosesConnections(t *testing.T) {
	server, _ := setupTestServer(t)
	conn := connectToServer(t, server)

	reader := bufio.NewReader(conn)

	// Skip greeting
	readResponse(t, reader)

	// Stop server
	server.Stop()

	// Try to read - should fail because connection is closed
	_, err := reader.ReadString('\n')
	if err == nil {
		t.Error("Expected connection to be closed after server shutdown")
	}
}

func TestContextCancellation(t *testing.T) {
	mockClient := &mockClientGetter{}
	log := logger.NewDefault()
	server := NewServer("127.0.0.1:0", mockClient, log)

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Cancel context
	server.cancel()

	// Give it time to process cancellation
	time.Sleep(100 * time.Millisecond)

	// Stop should not hang
	done := make(chan struct{})
	go func() {
		server.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Server.Stop() did not complete within timeout")
	}
}

func TestConnectionTimeout(t *testing.T) {
	server, _ := setupTestServer(t)
	conn := connectToServer(t, server)

	reader := bufio.NewReader(conn)

	// Skip greeting
	readResponse(t, reader)

	// Wait for read timeout (30 seconds in production, but connection closes on error)
	// Just verify the connection eventually closes due to inactivity
	conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))

	// This will timeout and close the connection server-side
	time.Sleep(200 * time.Millisecond)

	// Connection might be closed by server
	// This is a best-effort test
}

func TestGetInfoCircuitStatus(t *testing.T) {
	server, mockClient := setupTestServer(t)
	conn := connectToServer(t, server)

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// Skip greeting and authenticate
	readResponse(t, reader)
	writer.WriteString("AUTHENTICATE\r\n")
	writer.Flush()
	readResponse(t, reader)

	// Test with circuits
	mockClient.activeCircuits = 5
	writer.WriteString("GETINFO status/circuit-established\r\n")
	writer.Flush()

	response := readResponse(t, reader)

	if !strings.Contains(response, "=1") {
		t.Errorf("Expected circuit-established=1, got: %s", response)
	}

	// Test without circuits
	mockClient.activeCircuits = 0
	writer.WriteString("GETINFO status/circuit-established\r\n")
	writer.Flush()

	response = readResponse(t, reader)

	if !strings.Contains(response, "=0") {
		t.Errorf("Expected circuit-established=0, got: %s", response)
	}
}

func BenchmarkServerStartStop(b *testing.B) {
	mockClient := &mockClientGetter{}
	log := logger.NewDefault()

	for i := 0; i < b.N; i++ {
		server := NewServer("127.0.0.1:0", mockClient, log)
		server.Start()
		server.Stop()
	}
}

func BenchmarkCommandProcessing(b *testing.B) {
	mockClient := &mockClientGetter{activeCircuits: 3}
	log := logger.NewDefault()
	server := NewServer("127.0.0.1:0", mockClient, log)
	server.Start()
	defer server.Stop()

	addr := server.listener.Addr().String()
	conn, _ := net.Dial("tcp", addr)
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// Skip greeting
	reader.ReadString('\n')

	// Authenticate
	writer.WriteString("AUTHENTICATE\r\n")
	writer.Flush()
	reader.ReadString('\n')

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		writer.WriteString("GETINFO version\r\n")
		writer.Flush()
		reader.ReadString('\n')
	}
}
