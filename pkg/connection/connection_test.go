package connection

import (
	"context"
	"crypto/tls"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/cell"
	"github.com/opd-ai/go-tor/pkg/logger"
)

func TestStateString(t *testing.T) {
	tests := []struct {
		state    State
		expected string
	}{
		{StateConnecting, "CONNECTING"},
		{StateHandshaking, "HANDSHAKING"},
		{StateOpen, "OPEN"},
		{StateClosed, "CLOSED"},
		{StateFailed, "FAILED"},
		{State(99), "UNKNOWN(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.state.String(); got != tt.expected {
				t.Errorf("String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	address := "127.0.0.1:9001"
	cfg := DefaultConfig(address)

	if cfg.Address != address {
		t.Errorf("Address = %v, want %v", cfg.Address, address)
	}
	if cfg.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want %v", cfg.Timeout, 30*time.Second)
	}
	if cfg.TLSConfig == nil {
		t.Error("TLSConfig is nil")
	}
	if !cfg.LinkProtocolV4 {
		t.Error("LinkProtocolV4 = false, want true")
	}
}

func TestNew(t *testing.T) {
	cfg := DefaultConfig("127.0.0.1:9001")
	log := logger.NewDefault()

	conn := New(cfg, log)

	if conn == nil {
		t.Fatal("New() returned nil")
	}
	if conn.address != cfg.Address {
		t.Errorf("address = %v, want %v", conn.address, cfg.Address)
	}
	if conn.getState() != StateConnecting {
		t.Errorf("state = %v, want %v", conn.getState(), StateConnecting)
	}
}

func TestNewWithNilLogger(t *testing.T) {
	cfg := DefaultConfig("127.0.0.1:9001")
	conn := New(cfg, nil)

	if conn == nil {
		t.Fatal("New() with nil logger returned nil")
	}
	if conn.logger == nil {
		t.Error("logger is nil, expected default logger")
	}
}

func TestConnectionSetGetState(t *testing.T) {
	cfg := DefaultConfig("127.0.0.1:9001")
	conn := New(cfg, logger.NewDefault())

	states := []State{StateConnecting, StateHandshaking, StateOpen, StateClosed, StateFailed}

	for _, state := range states {
		conn.setState(state)
		if got := conn.getState(); got != state {
			t.Errorf("getState() = %v, want %v", got, state)
		}
		if got := conn.GetState(); got != state {
			t.Errorf("GetState() = %v, want %v", got, state)
		}
	}
}

func TestConnectionIsOpen(t *testing.T) {
	cfg := DefaultConfig("127.0.0.1:9001")
	conn := New(cfg, logger.NewDefault())

	if conn.IsOpen() {
		t.Error("IsOpen() = true for connecting connection, want false")
	}

	conn.setState(StateOpen)
	if !conn.IsOpen() {
		t.Error("IsOpen() = false for open connection, want true")
	}

	conn.setState(StateClosed)
	if conn.IsOpen() {
		t.Error("IsOpen() = true for closed connection, want false")
	}
}

func TestConnectionAddress(t *testing.T) {
	address := "127.0.0.1:9001"
	cfg := DefaultConfig(address)
	conn := New(cfg, logger.NewDefault())

	if conn.Address() != address {
		t.Errorf("Address() = %v, want %v", conn.Address(), address)
	}
}

func TestConnectionClose(t *testing.T) {
	cfg := DefaultConfig("127.0.0.1:9001")
	conn := New(cfg, logger.NewDefault())

	// Close without establishing connection
	err := conn.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	if conn.getState() != StateClosed {
		t.Errorf("state = %v, want %v", conn.getState(), StateClosed)
	}

	// Close again should be idempotent
	err = conn.Close()
	if err != nil {
		t.Errorf("Second Close() error = %v", err)
	}
}

func TestConnectionSendCellNotOpen(t *testing.T) {
	cfg := DefaultConfig("127.0.0.1:9001")
	conn := New(cfg, logger.NewDefault())

	testCell := cell.NewCell(1, cell.CmdPadding)

	err := conn.SendCell(testCell)
	if err == nil {
		t.Error("SendCell() on non-open connection should return error")
	}
}

func TestConnectionReceiveCellNotOpen(t *testing.T) {
	cfg := DefaultConfig("127.0.0.1:9001")
	conn := New(cfg, logger.NewDefault())

	_, err := conn.ReceiveCell()
	if err == nil {
		t.Error("ReceiveCell() on non-open connection should return error")
	}
}

// Mock TLS server for testing
func setupMockTLSServer(t *testing.T) (string, func()) {
	// Create a self-signed certificate for testing
	cert, err := tls.X509KeyPair([]byte(testCert), []byte(testKey))
	if err != nil {
		t.Fatalf("Failed to create test certificate: %v", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	listener, err := tls.Listen("tcp", "127.0.0.1:0", tlsConfig)
	if err != nil {
		t.Fatalf("Failed to create test listener: %v", err)
	}

	// Start accepting connections in background
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			// Just accept and close for basic tests
			conn.Close()
		}
	}()

	return listener.Addr().String(), func() {
		listener.Close()
	}
}

func TestConnectionConnect(t *testing.T) {
	address, cleanup := setupMockTLSServer(t)
	defer cleanup()

	cfg := DefaultConfig(address)
	// Use default TLS config which now has proper certificate verification
	conn := New(cfg, logger.NewDefault())

	ctx := context.Background()
	err := conn.Connect(ctx, cfg)

	// Connection should fail because mock server closes immediately
	// but we test the connection attempt
	if err == nil {
		defer conn.Close()
	}

	// The state should not be StateConnecting anymore
	if conn.getState() == StateConnecting {
		t.Error("state should change from StateConnecting after Connect()")
	}
}

func TestConnectionConnectTimeout(t *testing.T) {
	// Use a non-routable IP to trigger timeout
	cfg := DefaultConfig("192.0.2.1:9001") // TEST-NET-1, guaranteed to timeout
	cfg.Timeout = 100 * time.Millisecond
	conn := New(cfg, logger.NewDefault())

	ctx := context.Background()
	err := conn.Connect(ctx, cfg)

	if err == nil {
		t.Error("Connect() to non-routable address should timeout")
	}

	if conn.getState() != StateFailed {
		t.Errorf("state = %v, want %v after failed connect", conn.getState(), StateFailed)
	}
}

func TestConnectionConnectContextCanceled(t *testing.T) {
	cfg := DefaultConfig("192.0.2.1:9001")
	cfg.Timeout = 5 * time.Second
	conn := New(cfg, logger.NewDefault())

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := conn.Connect(ctx, cfg)

	if err == nil {
		t.Error("Connect() with canceled context should fail")
	}

	if conn.getState() != StateFailed {
		t.Errorf("state = %v, want %v after canceled connect", conn.getState(), StateFailed)
	}
}

// Test certificate and key for mock TLS server
const testCert = `-----BEGIN CERTIFICATE-----
MIIBhTCCASugAwIBAgIQIRi6zePL6mKjOipn+dNuaTAKBggqhkjOPQQDAjASMRAw
DgYDVQQKEwdBY21lIENvMB4XDTE3MTAyMDE5NDMwNloXDTE4MTAyMDE5NDMwNlow
EjEQMA4GA1UEChMHQWNtZSBDbzBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABD0d
7VNhbWvZLWPuj/RtHFjvtJBEwOkhbN/BnnE8rnZR8+sbwnc/KhCk3FhnpHZnQz7B
5aETbbIgmuvewdjvSBSjYzBhMA4GA1UdDwEB/wQEAwICpDATBgNVHSUEDDAKBggr
BgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MCkGA1UdEQQiMCCCDmxvY2FsaG9zdDo1
NDUzgg4xMjcuMC4wLjE6NTQ1MzAKBggqhkjOPQQDAgNIADBFAiEA2zpJEPQyz6/l
Wf86aX6PepsntZv2GYlA5UpabfT2EZICICpJ5h/iI+i341gBmLiAFQOyTDT+/wQc
6MF9+Yw1Yy0t
-----END CERTIFICATE-----`

const testKey = `-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIIrYSSNQFaA2Hwf1duRSxKtLYX5CB04fSeQ6tF1aY/PuoAoGCCqGSM49
AwEHoUQDQgAEPR3tU2Fta9ktY+6P9G0cWO+0kETA6SFs38GecTyudlHz6xvCdz8q
EKTcWGekdmdDPsHloRNtsiCa697B2O9IFA==
-----END EC PRIVATE KEY-----`

func TestVerifyTorRelayCertificate(t *testing.T) {
	// Test with nil certificates
	err := verifyTorRelayCertificate(nil, nil)
	if err == nil {
		t.Error("Expected error for nil certificates")
	}

	// Test with empty certificates
	err = verifyTorRelayCertificate([][]byte{}, nil)
	if err == nil {
		t.Error("Expected error for empty certificates")
	}

	// Test with invalid certificate data
	err = verifyTorRelayCertificate([][]byte{{0x00, 0x01, 0x02}}, nil)
	if err == nil {
		t.Error("Expected error for invalid certificate")
	}
}
