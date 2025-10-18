package socks

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/circuit"
	"github.com/opd-ai/go-tor/pkg/logger"
)

func TestNewServer(t *testing.T) {
	manager := circuit.NewManager()
	log := logger.NewDefault()

	server := NewServer("127.0.0.1:0", manager, log)

	if server == nil {
		t.Fatal("NewServer returned nil")
	}

	if server.logger == nil {
		t.Error("Server logger is nil")
	}

	if server.circuitMgr == nil {
		t.Error("Server circuit manager is nil")
	}

	// Test with nil logger
	server2 := NewServer("127.0.0.1:0", manager, nil)
	if server2.logger == nil {
		t.Error("Server should create default logger when nil is passed")
	}
}

func TestServerStartShutdown(t *testing.T) {
	manager := circuit.NewManager()
	log := logger.NewDefault()

	server := NewServer("127.0.0.1:0", manager, log)

	ctx, cancel := context.WithCancel(context.Background())

	// Start server in background
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe(ctx)
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Cancel context to stop server
	cancel()

	// Wait for server to stop
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("Server returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Server did not stop in time")
	}
}

func TestSOCKS5Handshake(t *testing.T) {
	manager := circuit.NewManager()
	log := logger.NewDefault()

	server := NewServer("127.0.0.1:0", manager, log)

	// Start server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go server.ListenAndServe(ctx)
	time.Sleep(100 * time.Millisecond)

	// Get actual listening address
	if server.listener == nil {
		t.Fatal("Server listener is nil")
	}
	addr := server.listener.Addr().String()

	// Connect to server
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	// Send SOCKS5 handshake (version 5, 1 method: no auth)
	handshake := []byte{0x05, 0x01, 0x00}
	if _, err := conn.Write(handshake); err != nil {
		t.Fatalf("Failed to write handshake: %v", err)
	}

	// Read response
	response := make([]byte, 2)
	if _, err := io.ReadFull(conn, response); err != nil {
		t.Fatalf("Failed to read handshake response: %v", err)
	}

	// Check response
	if response[0] != 0x05 {
		t.Errorf("Expected SOCKS version 5, got %d", response[0])
	}

	if response[1] != 0x00 {
		t.Errorf("Expected no auth method, got %d", response[1])
	}
}

func TestSOCKS5ConnectRequest(t *testing.T) {
	manager := circuit.NewManager()
	log := logger.NewDefault()

	server := NewServer("127.0.0.1:0", manager, log)

	// Start server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go server.ListenAndServe(ctx)
	time.Sleep(100 * time.Millisecond)

	addr := server.listener.Addr().String()

	// Connect to server
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	// Handshake
	handshake := []byte{0x05, 0x01, 0x00}
	if _, err := conn.Write(handshake); err != nil {
		t.Fatalf("Failed to write handshake: %v", err)
	}

	response := make([]byte, 2)
	if _, err := io.ReadFull(conn, response); err != nil {
		t.Fatalf("Failed to read handshake response: %v", err)
	}

	// Send CONNECT request (IPv4: 1.2.3.4:80)
	request := []byte{
		0x05,       // Version
		0x01,       // CONNECT command
		0x00,       // Reserved
		0x01,       // IPv4 address type
		1, 2, 3, 4, // IP address
		0x00, 0x50, // Port 80
	}

	if _, err := conn.Write(request); err != nil {
		t.Fatalf("Failed to write request: %v", err)
	}

	// Read reply
	reply := make([]byte, 10) // Max size for IPv4 reply
	if _, err := io.ReadFull(conn, reply); err != nil {
		t.Fatalf("Failed to read reply: %v", err)
	}

	// Check reply
	if reply[0] != 0x05 {
		t.Errorf("Expected SOCKS version 5, got %d", reply[0])
	}

	if reply[1] != 0x00 {
		t.Errorf("Expected success reply, got %d", reply[1])
	}
}

func TestSOCKS5DomainRequest(t *testing.T) {
	manager := circuit.NewManager()
	log := logger.NewDefault()

	server := NewServer("127.0.0.1:0", manager, log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go server.ListenAndServe(ctx)
	time.Sleep(100 * time.Millisecond)

	addr := server.listener.Addr().String()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	// Handshake
	handshake := []byte{0x05, 0x01, 0x00}
	conn.Write(handshake)
	response := make([]byte, 2)
	io.ReadFull(conn, response)

	// Send CONNECT request with domain
	domain := "example.com"
	request := bytes.NewBuffer([]byte{
		0x05,              // Version
		0x01,              // CONNECT command
		0x00,              // Reserved
		0x03,              // Domain address type
		byte(len(domain)), // Domain length
	})
	request.WriteString(domain)
	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, 80)
	request.Write(portBytes)

	if _, err := conn.Write(request.Bytes()); err != nil {
		t.Fatalf("Failed to write request: %v", err)
	}

	// Read reply
	reply := make([]byte, 10)
	if _, err := io.ReadFull(conn, reply); err != nil {
		t.Fatalf("Failed to read reply: %v", err)
	}

	// Check reply
	if reply[1] != 0x00 {
		t.Errorf("Expected success reply, got %d", reply[1])
	}
}

func TestSOCKS5UnsupportedVersion(t *testing.T) {
	manager := circuit.NewManager()
	log := logger.NewDefault()

	server := NewServer("127.0.0.1:0", manager, log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go server.ListenAndServe(ctx)
	time.Sleep(100 * time.Millisecond)

	addr := server.listener.Addr().String()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	// Send SOCKS4 handshake (should be rejected)
	handshake := []byte{0x04, 0x01, 0x00}
	conn.Write(handshake)

	// Server should close connection
	time.Sleep(100 * time.Millisecond)

	// Try to read - should get EOF or error
	buf := make([]byte, 10)
	_, err = conn.Read(buf)
	if err == nil {
		t.Error("Expected error or connection close for unsupported version")
	}
}

func TestSOCKS5ConcurrentConnections(t *testing.T) {
	manager := circuit.NewManager()
	log := logger.NewDefault()

	server := NewServer("127.0.0.1:0", manager, log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go server.ListenAndServe(ctx)
	time.Sleep(100 * time.Millisecond)

	addr := server.listener.Addr().String()

	// Make multiple concurrent connections
	done := make(chan bool)
	numConns := 5

	for i := 0; i < numConns; i++ {
		go func() {
			conn, err := net.Dial("tcp", addr)
			if err != nil {
				t.Errorf("Failed to connect: %v", err)
				done <- false
				return
			}
			defer conn.Close()

			// Handshake
			handshake := []byte{0x05, 0x01, 0x00}
			conn.Write(handshake)
			response := make([]byte, 2)
			io.ReadFull(conn, response)

			if response[0] != 0x05 || response[1] != 0x00 {
				t.Error("Handshake failed")
				done <- false
				return
			}

			done <- true
		}()
	}

	// Wait for all connections
	timeout := time.After(5 * time.Second)
	for i := 0; i < numConns; i++ {
		select {
		case <-done:
			// Success
		case <-timeout:
			t.Fatal("Test timed out")
		}
	}
}

func TestServerShutdownWithActiveConnections(t *testing.T) {
	manager := circuit.NewManager()
	log := logger.NewDefault()

	server := NewServer("127.0.0.1:0", manager, log)

	ctx, cancel := context.WithCancel(context.Background())

	go server.ListenAndServe(ctx)
	time.Sleep(100 * time.Millisecond)

	addr := server.listener.Addr().String()

	// Create a connection
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Shutdown server while connection is active
	cancel()

	// Wait for shutdown
	time.Sleep(500 * time.Millisecond)

	// Connection should be closed
	buf := make([]byte, 10)
	conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	_, err = conn.Read(buf)
	if err == nil {
		t.Error("Expected connection to be closed")
	}
}
