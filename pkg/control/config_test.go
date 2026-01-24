package control

import (
	"bufio"
	"net"
	"strings"
	"testing"

	"github.com/opd-ai/go-tor/pkg/logger"
)

func TestGetConfRequiresAuth(t *testing.T) {
	mockClient := &mockClientGetter{
		activeCircuits: 3,
		socksPort:      9050,
		controlPort:    9051,
		config: map[string]string{
			"SocksPort": "9050",
		},
	}

	server := NewServerWithPassword("127.0.0.1:0", mockClient, "test-password", logger.NewDefault())
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	addr := server.listener.Addr().String()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	
	// Consume greeting
	reader.ReadString('\n')

	// Try GETCONF without authentication
	conn.Write([]byte("GETCONF SocksPort\r\n"))

	response, _ := reader.ReadString('\n')

	if !strings.HasPrefix(response, "514") {
		t.Errorf("Expected 514 error for unauthenticated GETCONF, got: %s", response)
	}
}

func TestGetConfSingleKey(t *testing.T) {
	mockClient := &mockClientGetter{
		activeCircuits: 3,
		socksPort:      9050,
		controlPort:    9051,
		config: map[string]string{
			"SocksPort":    "9050",
			"ControlPort":  "9051",
			"LogLevel":     "info",
			"MetricsPort":  "9090",
		},
	}

	server := NewServer("127.0.0.1:0", mockClient, logger.NewDefault())
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	addr := server.listener.Addr().String()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)

	// Consume greeting
	reader.ReadString('\n')

	// Authenticate
	conn.Write([]byte("AUTHENTICATE\r\n"))
	reader.ReadString('\n')

	// Get single config key
	conn.Write([]byte("GETCONF SocksPort\r\n"))

	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(response)

	expected := "250 SocksPort=9050"
	if response != expected {
		t.Errorf("Expected %q, got %q", expected, response)
	}
}

func TestGetConfMultipleKeys(t *testing.T) {
	mockClient := &mockClientGetter{
		activeCircuits: 3,
		socksPort:      9050,
		controlPort:    9051,
		config: map[string]string{
			"SocksPort":    "9050",
			"ControlPort":  "9051",
			"LogLevel":     "debug",
			"MetricsPort":  "9090",
		},
	}

	server := NewServer("127.0.0.1:0", mockClient, logger.NewDefault())
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	addr := server.listener.Addr().String()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)

	// Consume greeting
	reader.ReadString('\n')

	// Authenticate
	conn.Write([]byte("AUTHENTICATE\r\n"))
	reader.ReadString('\n')

	// Get multiple config keys
	conn.Write([]byte("GETCONF SocksPort ControlPort LogLevel\r\n"))

	var responses []string
	for i := 0; i < 3; i++ {
		response, _ := reader.ReadString('\n')
		responses = append(responses, strings.TrimSpace(response))
	}

	// Check all responses
	if !strings.Contains(responses[0], "SocksPort=9050") {
		t.Errorf("Expected SocksPort=9050 in response, got: %s", responses[0])
	}
	if !strings.Contains(responses[1], "ControlPort=9051") {
		t.Errorf("Expected ControlPort=9051 in response, got: %s", responses[1])
	}
	if !strings.Contains(responses[2], "LogLevel=debug") {
		t.Errorf("Expected LogLevel=debug in response, got: %s", responses[2])
	}

	// Last response should have 250 (not 250-)
	if !strings.HasPrefix(responses[2], "250 ") {
		t.Errorf("Last response should have '250 ' prefix, got: %s", responses[2])
	}
}

func TestGetConfUnknownKey(t *testing.T) {
	mockClient := &mockClientGetter{
		activeCircuits: 3,
		socksPort:      9050,
		controlPort:    9051,
		config: map[string]string{
			"SocksPort": "9050",
		},
	}

	server := NewServer("127.0.0.1:0", mockClient, logger.NewDefault())
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	addr := server.listener.Addr().String()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)

	// Consume greeting
	reader.ReadString('\n')

	// Authenticate
	conn.Write([]byte("AUTHENTICATE\r\n"))
	reader.ReadString('\n')

	// Get unknown config key
	conn.Write([]byte("GETCONF UnknownKey\r\n"))

	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(response)

	// Unknown keys should return empty value per control-spec.txt
	expected := "250 UnknownKey="
	if response != expected {
		t.Errorf("Expected %q for unknown key, got %q", expected, response)
	}
}

func TestGetConfNoConfig(t *testing.T) {
	mockClient := &mockClientGetter{
		activeCircuits: 3,
		socksPort:      9050,
		controlPort:    9051,
		config:         nil, // No config available
	}

	server := NewServer("127.0.0.1:0", mockClient, logger.NewDefault())
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	addr := server.listener.Addr().String()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)

	// Consume greeting
	reader.ReadString('\n')

	// Authenticate
	conn.Write([]byte("AUTHENTICATE\r\n"))
	reader.ReadString('\n')

	// Get config when not available
	conn.Write([]byte("GETCONF SocksPort\r\n"))

	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(response)

	// Should return empty value when config is not available
	expected := "250 SocksPort="
	if response != expected {
		t.Errorf("Expected %q when config unavailable, got %q", expected, response)
	}
}

func TestSetConfRequiresAuth(t *testing.T) {
	mockClient := &mockClientGetter{
		activeCircuits: 3,
		socksPort:      9050,
		controlPort:    9051,
		config: map[string]string{
			"LogLevel": "info",
		},
	}

	server := NewServerWithPassword("127.0.0.1:0", mockClient, "test-password", logger.NewDefault())
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	addr := server.listener.Addr().String()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	
	// Consume greeting
	reader.ReadString('\n')

	// Try SETCONF without authentication
	conn.Write([]byte("SETCONF LogLevel=debug\r\n"))

	response, _ := reader.ReadString('\n')

	if !strings.HasPrefix(response, "514") {
		t.Errorf("Expected 514 error for unauthenticated SETCONF, got: %s", response)
	}
}

func TestSetConfSuccess(t *testing.T) {
	config := make(map[string]string)
	config["LogLevel"] = "info"

	mockClient := &mockClientGetter{
		activeCircuits: 3,
		socksPort:      9050,
		controlPort:    9051,
		config:         config,
	}

	server := NewServer("127.0.0.1:0", mockClient, logger.NewDefault())
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	addr := server.listener.Addr().String()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)

	// Consume greeting
	reader.ReadString('\n')

	// Authenticate
	conn.Write([]byte("AUTHENTICATE\r\n"))
	reader.ReadString('\n')

	// Set config value
	conn.Write([]byte("SETCONF LogLevel=debug\r\n"))

	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(response)

	if !strings.HasPrefix(response, "250") {
		t.Errorf("Expected 250 OK, got: %s", response)
	}

	// Verify value was changed
	if config["LogLevel"] != "debug" {
		t.Errorf("Expected LogLevel to be 'debug', got: %s", config["LogLevel"])
	}
}

func TestSetConfInvalidKey(t *testing.T) {
	config := make(map[string]string)
	config["LogLevel"] = "info"

	mockClient := &mockClientGetter{
		activeCircuits: 3,
		socksPort:      9050,
		controlPort:    9051,
		config:         config,
	}

	server := NewServer("127.0.0.1:0", mockClient, logger.NewDefault())
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	addr := server.listener.Addr().String()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)

	// Consume greeting
	reader.ReadString('\n')

	// Authenticate
	conn.Write([]byte("AUTHENTICATE\r\n"))
	reader.ReadString('\n')

	// Try to set read-only config value
	conn.Write([]byte("SETCONF SocksPort=9999\r\n"))

	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(response)

	if !strings.HasPrefix(response, "553") {
		t.Errorf("Expected 553 error for read-only key, got: %s", response)
	}
}

func TestSetConfNoConfig(t *testing.T) {
	mockClient := &mockClientGetter{
		activeCircuits: 3,
		socksPort:      9050,
		controlPort:    9051,
		config:         nil, // No config available
	}

	server := NewServer("127.0.0.1:0", mockClient, logger.NewDefault())
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	addr := server.listener.Addr().String()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)

	// Consume greeting
	reader.ReadString('\n')

	// Authenticate
	conn.Write([]byte("AUTHENTICATE\r\n"))
	reader.ReadString('\n')

	// Try to set config when not available
	conn.Write([]byte("SETCONF LogLevel=debug\r\n"))

	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(response)

	// Should acknowledge even when config is not available
	if !strings.HasPrefix(response, "250") {
		t.Errorf("Expected 250 OK when config unavailable, got: %s", response)
	}
}

func TestSetConfMissingArgument(t *testing.T) {
	mockClient := &mockClientGetter{
		activeCircuits: 3,
		socksPort:      9050,
		controlPort:    9051,
		config: map[string]string{
			"LogLevel": "info",
		},
	}

	server := NewServer("127.0.0.1:0", mockClient, logger.NewDefault())
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	addr := server.listener.Addr().String()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)

	// Consume greeting
	reader.ReadString('\n')

	// Authenticate
	conn.Write([]byte("AUTHENTICATE\r\n"))
	reader.ReadString('\n')

	// Send SETCONF with missing argument
	conn.Write([]byte("SETCONF\r\n"))

	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(response)

	if !strings.HasPrefix(response, "552") {
		t.Errorf("Expected 552 error for missing argument, got: %s", response)
	}
}

func TestGetConfMissingArgument(t *testing.T) {
	mockClient := &mockClientGetter{
		activeCircuits: 3,
		socksPort:      9050,
		controlPort:    9051,
		config: map[string]string{
			"SocksPort": "9050",
		},
	}

	server := NewServer("127.0.0.1:0", mockClient, logger.NewDefault())
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	addr := server.listener.Addr().String()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)

	// Consume greeting
	reader.ReadString('\n')

	// Authenticate
	conn.Write([]byte("AUTHENTICATE\r\n"))
	reader.ReadString('\n')

	// Send GETCONF with missing argument
	conn.Write([]byte("GETCONF\r\n"))

	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(response)

	if !strings.HasPrefix(response, "552") {
		t.Errorf("Expected 552 error for missing argument, got: %s", response)
	}
}
