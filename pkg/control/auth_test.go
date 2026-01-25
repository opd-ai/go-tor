package control

import (
	"bufio"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/logger"
)

// TestAuthenticationNoPassword tests that authentication works without a password
func TestAuthenticationNoPassword(t *testing.T) {
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
	defer server.Stop()

	// Connect to server
	addr := server.listener.Addr().String()
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// Read greeting
	greeting, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("Failed to read greeting: %v", err)
	}
	if !strings.HasPrefix(greeting, "250") {
		t.Errorf("Expected 250 greeting, got: %s", greeting)
	}

	// Authenticate without password
	_, err = writer.WriteString("AUTHENTICATE\r\n")
	if err != nil {
		t.Fatalf("Failed to write command: %v", err)
	}
	writer.Flush()

	response, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}
	if !strings.HasPrefix(response, "250") {
		t.Errorf("Expected 250 OK, got: %s", response)
	}
}

// TestAuthenticationWithCorrectPassword tests successful password authentication
func TestAuthenticationWithCorrectPassword(t *testing.T) {
	mockClient := &mockClientGetter{
		activeCircuits: 3,
		socksPort:      9050,
		controlPort:    9051,
	}

	log := logger.NewDefault()
	password := "test-password-123"
	server := NewServerWithPassword("127.0.0.1:0", mockClient, password, log)

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Connect to server
	addr := server.listener.Addr().String()
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// Read greeting
	_, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("Failed to read greeting: %v", err)
	}

	// Authenticate with correct password
	_, err = writer.WriteString("AUTHENTICATE " + password + "\r\n")
	if err != nil {
		t.Fatalf("Failed to write command: %v", err)
	}
	writer.Flush()

	response, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}
	if !strings.HasPrefix(response, "250") {
		t.Errorf("Expected 250 OK, got: %s", response)
	}

	// Verify we can now use authenticated commands
	_, err = writer.WriteString("GETINFO version\r\n")
	if err != nil {
		t.Fatalf("Failed to write GETINFO: %v", err)
	}
	writer.Flush()

	response, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("Failed to read GETINFO response: %v", err)
	}
	if !strings.HasPrefix(response, "250") {
		t.Errorf("Expected GETINFO to work after auth, got: %s", response)
	}
}

// TestAuthenticationWithIncorrectPassword tests that wrong passwords are rejected
func TestAuthenticationWithIncorrectPassword(t *testing.T) {
	mockClient := &mockClientGetter{
		activeCircuits: 3,
		socksPort:      9050,
		controlPort:    9051,
	}

	log := logger.NewDefault()
	password := "correct-password"
	server := NewServerWithPassword("127.0.0.1:0", mockClient, password, log)

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Connect to server
	addr := server.listener.Addr().String()
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// Read greeting
	_, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("Failed to read greeting: %v", err)
	}

	// Authenticate with wrong password
	_, err = writer.WriteString("AUTHENTICATE wrong-password\r\n")
	if err != nil {
		t.Fatalf("Failed to write command: %v", err)
	}
	writer.Flush()

	response, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}
	if !strings.HasPrefix(response, "515") {
		t.Errorf("Expected 515 authentication failed, got: %s", response)
	}
	if !strings.Contains(response, "incorrect password") {
		t.Errorf("Expected 'incorrect password' message, got: %s", response)
	}
}

// TestAuthenticationRequiredForCommands tests that commands require auth when password is set
func TestAuthenticationRequiredForCommands(t *testing.T) {
	mockClient := &mockClientGetter{
		activeCircuits: 3,
		socksPort:      9050,
		controlPort:    9051,
	}

	log := logger.NewDefault()
	password := "secret-password"
	server := NewServerWithPassword("127.0.0.1:0", mockClient, password, log)

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Connect to server
	addr := server.listener.Addr().String()
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// Read greeting
	_, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("Failed to read greeting: %v", err)
	}

	// Try GETINFO without authentication
	_, err = writer.WriteString("GETINFO version\r\n")
	if err != nil {
		t.Fatalf("Failed to write command: %v", err)
	}
	writer.Flush()

	response, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}
	if !strings.HasPrefix(response, "514") {
		t.Errorf("Expected 514 authentication required, got: %s", response)
	}
}

// TestAuthenticationNoPasswordProvided tests that auth fails when password required but not provided
func TestAuthenticationNoPasswordProvided(t *testing.T) {
	mockClient := &mockClientGetter{
		activeCircuits: 3,
		socksPort:      9050,
		controlPort:    9051,
	}

	log := logger.NewDefault()
	password := "required-password"
	server := NewServerWithPassword("127.0.0.1:0", mockClient, password, log)

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Connect to server
	addr := server.listener.Addr().String()
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// Read greeting
	_, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("Failed to read greeting: %v", err)
	}

	// Authenticate without providing password
	_, err = writer.WriteString("AUTHENTICATE\r\n")
	if err != nil {
		t.Fatalf("Failed to write command: %v", err)
	}
	writer.Flush()

	response, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}
	if !strings.HasPrefix(response, "515") {
		t.Errorf("Expected 515 authentication failed, got: %s", response)
	}
	if !strings.Contains(response, "password required") {
		t.Errorf("Expected 'password required' message, got: %s", response)
	}
}

// TestProtocolInfoAuthMethods tests that PROTOCOLINFO reports correct auth methods
func TestProtocolInfoAuthMethods(t *testing.T) {
	tests := []struct {
		name           string
		password       string
		expectedMethod string
	}{
		{
			name:           "No password - NULL auth",
			password:       "",
			expectedMethod: "NULL",
		},
		{
			name:           "With password - HASHEDPASSWORD auth",
			password:       "test-password",
			expectedMethod: "HASHEDPASSWORD",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockClientGetter{
				activeCircuits: 3,
				socksPort:      9050,
				controlPort:    9051,
			}

			log := logger.NewDefault()
			var server *Server
			if tt.password == "" {
				server = NewServer("127.0.0.1:0", mockClient, log)
			} else {
				server = NewServerWithPassword("127.0.0.1:0", mockClient, tt.password, log)
			}

			if err := server.Start(); err != nil {
				t.Fatalf("Failed to start server: %v", err)
			}
			defer server.Stop()

			// Connect to server
			addr := server.listener.Addr().String()
			conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
			if err != nil {
				t.Fatalf("Failed to connect: %v", err)
			}
			defer conn.Close()

			reader := bufio.NewReader(conn)
			writer := bufio.NewWriter(conn)

			// Read greeting
			_, err = reader.ReadString('\n')
			if err != nil {
				t.Fatalf("Failed to read greeting: %v", err)
			}

			// Request PROTOCOLINFO
			_, err = writer.WriteString("PROTOCOLINFO\r\n")
			if err != nil {
				t.Fatalf("Failed to write command: %v", err)
			}
			writer.Flush()

			// Read multiline response
			var responses []string
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					t.Fatalf("Failed to read response: %v", err)
				}
				responses = append(responses, line)
				if strings.HasPrefix(line, "250 ") {
					break
				}
			}

			// Find AUTH METHODS line
			found := false
			for _, line := range responses {
				if strings.Contains(line, "AUTH METHODS=") {
					if !strings.Contains(line, tt.expectedMethod) {
						t.Errorf("Expected auth method %s, got: %s", tt.expectedMethod, line)
					}
					found = true
					break
				}
			}
			if !found {
				t.Errorf("AUTH METHODS line not found in PROTOCOLINFO response")
			}
		})
	}
}

// TestAuthenticationWithQuotedPassword tests password with quotes
func TestAuthenticationWithQuotedPassword(t *testing.T) {
	mockClient := &mockClientGetter{
		activeCircuits: 3,
		socksPort:      9050,
		controlPort:    9051,
	}

	log := logger.NewDefault()
	password := "quoted password"
	server := NewServerWithPassword("127.0.0.1:0", mockClient, password, log)

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Connect to server
	addr := server.listener.Addr().String()
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// Read greeting
	_, err = reader.ReadString('\n')
	if err != nil {
		t.Fatalf("Failed to read greeting: %v", err)
	}

	// Authenticate with quoted password
	_, err = writer.WriteString("AUTHENTICATE \"quoted password\"\r\n")
	if err != nil {
		t.Fatalf("Failed to write command: %v", err)
	}
	writer.Flush()

	response, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}
	if !strings.HasPrefix(response, "250") {
		t.Errorf("Expected 250 OK with quoted password, got: %s", response)
	}
}
