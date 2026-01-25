// Package onion - Stream Handling Tests
package onion

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/cell"
	"github.com/opd-ai/go-tor/pkg/logger"
)

// mockCircuit implements CircuitInterface for testing
type mockCircuit struct {
	id               uint32
	sentCells        []*cell.RelayCell
	receiveCellsChan chan *cell.RelayCell
}

func newMockCircuit(id uint32) *mockCircuit {
	return &mockCircuit{
		id:               id,
		sentCells:        make([]*cell.RelayCell, 0),
		receiveCellsChan: make(chan *cell.RelayCell, 10),
	}
}

func (m *mockCircuit) SendRelayCell(relayCell *cell.RelayCell) error {
	m.sentCells = append(m.sentCells, relayCell)
	return nil
}

func (m *mockCircuit) ReceiveRelayCell(ctx context.Context) (*cell.RelayCell, error) {
	select {
	case c := <-m.receiveCellsChan:
		return c, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (m *mockCircuit) GetID() uint32 {
	return m.id
}

func (m *mockCircuit) getSentCell(index int) *cell.RelayCell {
	if index < 0 || index >= len(m.sentCells) {
		return nil
	}
	return m.sentCells[index]
}

func TestParseAddrPort(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantHost    string
		wantPort    int
		expectError bool
	}{
		{
			name:     "host and port",
			input:    "example.com:80",
			wantHost: "example.com",
			wantPort: 80,
		},
		{
			name:     "IP and port",
			input:    "127.0.0.1:8080",
			wantHost: "127.0.0.1",
			wantPort: 8080,
		},
		{
			name:     "IPv6 and port",
			input:    "[::1]:9050",
			wantHost: "::1",
			wantPort: 9050,
		},
		{
			name:     "host only",
			input:    "example.com",
			wantHost: "example.com",
			wantPort: 0,
		},
		{
			name:        "invalid port",
			input:       "example.com:99999",
			expectError: true,
		},
		{
			name:        "invalid port characters",
			input:       "example.com:abc",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, port, err := parseAddrPort(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if host != tt.wantHost {
				t.Errorf("Host mismatch: got %q, want %q", host, tt.wantHost)
			}

			if port != tt.wantPort {
				t.Errorf("Port mismatch: got %d, want %d", port, tt.wantPort)
			}
		})
	}
}

func TestServiceStreamManager_HandleRelayBegin(t *testing.T) {
	log := logger.NewDefault()

	// Start a test TCP server on localhost
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start test server: %v", err)
	}
	defer listener.Close()

	testServerAddr := listener.Addr().String()
	t.Logf("Test server listening on %s", testServerAddr)

	// Accept connections in background
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			// Echo server
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 1024)
				for {
					n, err := c.Read(buf)
					if err != nil {
						return
					}
					c.Write(buf[:n])
				}
			}(conn)
		}
	}()

	// Create service with port mapping
	config := &ServiceConfig{
		Ports: map[int]string{
			80: testServerAddr,
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	service := &Service{
		config: config,
		ctx:    ctx,
		logger: log,
	}

	streamManager := NewServiceStreamManager(service, log)
	circuit := newMockCircuit(1)

	// Test successful RELAY_BEGIN
	t.Run("successful begin", func(t *testing.T) {
		// Build RELAY_BEGIN payload: "example.com:80\x00"
		addrPort := "example.com:80"
		payload := append([]byte(addrPort), 0) // Add null terminator

		err := streamManager.HandleRelayBegin(circuit.id, 1, payload, circuit)
		if err != nil {
			t.Fatalf("HandleRelayBegin failed: %v", err)
		}

		// Verify RELAY_CONNECTED was sent
		if len(circuit.sentCells) != 1 {
			t.Fatalf("Expected 1 sent cell, got %d", len(circuit.sentCells))
		}

		connectedCell := circuit.getSentCell(0)
		if connectedCell.Command != cell.RelayConnected {
			t.Errorf("Expected RELAY_CONNECTED, got command %d", connectedCell.Command)
		}

		if connectedCell.StreamID != 1 {
			t.Errorf("Expected stream ID 1, got %d", connectedCell.StreamID)
		}

		// Verify stream was created
		if streamManager.GetActiveStreamCount() != 1 {
			t.Errorf("Expected 1 active stream, got %d", streamManager.GetActiveStreamCount())
		}
	})

	// Test RELAY_BEGIN with no null terminator
	t.Run("no null terminator", func(t *testing.T) {
		circuit2 := newMockCircuit(2)
		payload := []byte("example.com:80") // No null terminator

		err := streamManager.HandleRelayBegin(circuit2.id, 2, payload, circuit2)
		if err != nil {
			t.Logf("HandleRelayBegin returned error (expected): %v", err)
		}

		// Should send RELAY_END
		if len(circuit2.sentCells) != 1 {
			t.Fatalf("Expected 1 sent cell (RELAY_END), got %d", len(circuit2.sentCells))
		}

		endCell := circuit2.getSentCell(0)
		if endCell.Command != cell.RelayEnd {
			t.Errorf("Expected RELAY_END, got command %d", endCell.Command)
		}
	})

	// Test RELAY_BEGIN with unmapped port
	t.Run("unmapped port", func(t *testing.T) {
		circuit3 := newMockCircuit(3)
		payload := append([]byte("example.com:443"), 0) // Port 443 not in config

		err := streamManager.HandleRelayBegin(circuit3.id, 3, payload, circuit3)
		if err != nil {
			t.Logf("HandleRelayBegin returned error (expected): %v", err)
		}

		// Should send RELAY_END with EndReasonExitPolicy
		if len(circuit3.sentCells) != 1 {
			t.Fatalf("Expected 1 sent cell (RELAY_END), got %d", len(circuit3.sentCells))
		}

		endCell := circuit3.getSentCell(0)
		if endCell.Command != cell.RelayEnd {
			t.Errorf("Expected RELAY_END, got command %d", endCell.Command)
		}

		if len(endCell.Data) > 0 && endCell.Data[0] != cell.EndReasonExitPolicy {
			t.Errorf("Expected EndReasonExitPolicy (%d), got %d",
				cell.EndReasonExitPolicy, endCell.Data[0])
		}
	})
}

func TestServiceStreamManager_HandleRelayData(t *testing.T) {
	log := logger.NewDefault()

	// Start a test TCP server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start test server: %v", err)
	}
	defer listener.Close()

	testServerAddr := listener.Addr().String()
	receivedData := make(chan []byte, 1)

	// Accept and read from connection
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		buf := make([]byte, 1024)
		n, _ := conn.Read(buf)
		if n > 0 {
			receivedData <- buf[:n]
		}
	}()

	// Create service and stream manager
	config := &ServiceConfig{
		Ports: map[int]string{
			80: testServerAddr,
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	service := &Service{
		config: config,
		ctx:    ctx,
		logger: log,
	}

	streamManager := NewServiceStreamManager(service, log)
	circuit := newMockCircuit(1)

	// Establish stream first
	payload := append([]byte("example.com:80"), 0)
	err = streamManager.HandleRelayBegin(circuit.id, 1, payload, circuit)
	if err != nil {
		t.Fatalf("HandleRelayBegin failed: %v", err)
	}

	// Wait a bit for connection to establish
	time.Sleep(100 * time.Millisecond)

	// Send data through stream
	testData := []byte("Hello, service!")
	err = streamManager.HandleRelayData(1, testData)
	if err != nil {
		t.Fatalf("HandleRelayData failed: %v", err)
	}

	// Verify data was forwarded to backend
	select {
	case data := <-receivedData:
		if string(data) != string(testData) {
			t.Errorf("Data mismatch: got %q, want %q", string(data), string(testData))
		}
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for data from backend")
	}
}

func TestServiceStreamManager_HandleRelayEnd(t *testing.T) {
	log := logger.NewDefault()

	// Create service
	config := &ServiceConfig{
		Ports: map[int]string{
			80: "localhost:8080",
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	service := &Service{
		config: config,
		ctx:    ctx,
		logger: log,
	}

	streamManager := NewServiceStreamManager(service, log)

	// Add a mock stream directly
	mockStream := &ServiceStream{
		StreamID:  1,
		CircuitID: 1,
		logger:    log,
	}
	streamManager.streams[1] = mockStream

	// Handle RELAY_END
	err := streamManager.HandleRelayEnd(1)
	if err != nil {
		t.Fatalf("HandleRelayEnd failed: %v", err)
	}

	// Verify stream was removed
	if streamManager.GetActiveStreamCount() != 0 {
		t.Errorf("Expected 0 active streams after RELAY_END, got %d",
			streamManager.GetActiveStreamCount())
	}

	// Handling unknown stream should not error
	err = streamManager.HandleRelayEnd(999)
	if err != nil {
		t.Errorf("HandleRelayEnd for unknown stream should not error, got: %v", err)
	}
}

func TestParseRelayBeginCell(t *testing.T) {
	tests := []struct {
		name         string
		payload      []byte
		wantAddrPort string
		wantFlags    uint32
		expectError  bool
	}{
		{
			name:         "simple address",
			payload:      append([]byte("example.com:80"), 0, 0, 0, 0, 0),
			wantAddrPort: "example.com:80",
			wantFlags:    0,
		},
		{
			name:         "address with flags",
			payload:      append([]byte("example.com:80"), 0, 0x00, 0x00, 0x00, 0x01),
			wantAddrPort: "example.com:80",
			wantFlags:    1,
		},
		{
			name:        "no null terminator",
			payload:     []byte("example.com:80"),
			expectError: true,
		},
		{
			name:         "minimal",
			payload:      append([]byte("a"), 0),
			wantAddrPort: "a",
			wantFlags:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addrPort, flags, err := ParseRelayBeginCell(tt.payload)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if addrPort != tt.wantAddrPort {
				t.Errorf("AddrPort mismatch: got %q, want %q", addrPort, tt.wantAddrPort)
			}

			if flags != tt.wantFlags {
				t.Errorf("Flags mismatch: got 0x%08x, want 0x%08x", flags, tt.wantFlags)
			}
		})
	}
}

func TestServiceStreamManager_CloseAll(t *testing.T) {
	log := logger.NewDefault()

	config := &ServiceConfig{
		Ports: map[int]string{},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	service := &Service{
		config: config,
		ctx:    ctx,
		logger: log,
	}

	streamManager := NewServiceStreamManager(service, log)

	// Add multiple mock streams
	for i := uint16(1); i <= 5; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		streamManager.streams[i] = &ServiceStream{
			StreamID:  i,
			CircuitID: 1,
			logger:    log,
			ctx:       ctx,
			cancel:    cancel,
		}
	}

	if streamManager.GetActiveStreamCount() != 5 {
		t.Fatalf("Expected 5 active streams, got %d", streamManager.GetActiveStreamCount())
	}

	// Close all streams
	streamManager.CloseAll()

	// Verify all streams were removed
	if streamManager.GetActiveStreamCount() != 0 {
		t.Errorf("Expected 0 active streams after CloseAll, got %d",
			streamManager.GetActiveStreamCount())
	}
}

func TestServiceStream_Bidirectional(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping bidirectional test in short mode")
	}

	log := logger.NewDefault()

	// Start echo server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start test server: %v", err)
	}
	defer listener.Close()

	testServerAddr := listener.Addr().String()

	// Echo server
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 1024)
				for {
					n, err := c.Read(buf)
					if err != nil {
						return
					}
					c.Write(buf[:n])
				}
			}(conn)
		}
	}()

	// Create service
	config := &ServiceConfig{
		Ports: map[int]string{
			80: testServerAddr,
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	service := &Service{
		config: config,
		ctx:    ctx,
		logger: log,
	}

	streamManager := NewServiceStreamManager(service, log)
	circuit := newMockCircuit(1)

	// Establish stream
	payload := append([]byte(fmt.Sprintf("localhost:%d", 80)), 0)
	err = streamManager.HandleRelayBegin(circuit.id, 1, payload, circuit)
	if err != nil {
		t.Fatalf("HandleRelayBegin failed: %v", err)
	}

	// Wait for RELAY_CONNECTED
	time.Sleep(100 * time.Millisecond)

	// Send data to service
	testMsg := []byte("ping")
	err = streamManager.HandleRelayData(1, testMsg)
	if err != nil {
		t.Fatalf("HandleRelayData failed: %v", err)
	}

	// The echo should come back as RELAY_DATA
	// Wait for stream to forward response
	time.Sleep(200 * time.Millisecond)

	// Check for RELAY_DATA cells sent back
	foundEcho := false
	for _, sentCell := range circuit.sentCells {
		if sentCell.Command == cell.RelayData && string(sentCell.Data) == string(testMsg) {
			foundEcho = true
			break
		}
	}

	if !foundEcho {
		t.Errorf("Expected echo response via RELAY_DATA, but didn't find it")
	}
}
