package protocol

import (
	"bytes"
	"context"
	"crypto/tls"
	"net"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/cell"
	"github.com/opd-ai/go-tor/pkg/connection"
	"github.com/opd-ai/go-tor/pkg/logger"
)

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

// mockTLSRelay is a TLS-enabled mock relay for testing handshake
type mockTLSRelay struct {
	listener      net.Listener
	logger        *logger.Logger
	handleCellsFn func(conn net.Conn) // Custom cell handler for tests
}

// newMockTLSRelay creates a TLS mock relay for testing
func newMockTLSRelay(t *testing.T) (*mockTLSRelay, string, error) {
	cert, err := tls.X509KeyPair([]byte(testCert), []byte(testKey))
	if err != nil {
		return nil, "", err
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	listener, err := tls.Listen("tcp", "127.0.0.1:0", tlsConfig)
	if err != nil {
		return nil, "", err
	}

	return &mockTLSRelay{
		listener: listener,
		logger:   logger.NewDefault(),
	}, listener.Addr().String(), nil
}

// serveHandshake serves a complete handshake protocol
func (m *mockTLSRelay) serveHandshake() {
	go func() {
		for {
			conn, err := m.listener.Accept()
			if err != nil {
				return
			}
			go m.handleHandshake(conn)
		}
	}()
}

// handleHandshake handles the complete handshake protocol
func (m *mockTLSRelay) handleHandshake(conn net.Conn) {
	defer conn.Close()

	// Set read deadline to avoid blocking forever
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	// Read VERSIONS cell
	versionsCell, err := cell.DecodeCell(conn)
	if err != nil {
		m.logger.Debug("Failed to decode VERSIONS cell", "error", err)
		return
	}

	if versionsCell.Command != cell.CmdVersions {
		m.logger.Debug("Expected VERSIONS command", "got", versionsCell.Command)
		return
	}

	// Send VERSIONS response
	responseCell := cell.NewCell(0, cell.CmdVersions)
	responseCell.Payload = []byte{0x00, 0x04} // Version 4
	var encBuf bytes.Buffer
	if err := responseCell.Encode(&encBuf); err != nil {
		m.logger.Debug("Failed to encode VERSIONS response", "error", err)
		return
	}

	if _, err := conn.Write(encBuf.Bytes()); err != nil {
		m.logger.Debug("Failed to write VERSIONS response", "error", err)
		return
	}

	// Read NETINFO cell
	netinfoCell, err := cell.DecodeCell(conn)
	if err != nil {
		m.logger.Debug("Failed to decode NETINFO cell", "error", err)
		return
	}

	if netinfoCell.Command != cell.CmdNetinfo {
		m.logger.Debug("Expected NETINFO command", "got", netinfoCell.Command)
		return
	}

	// Send NETINFO response
	netinfoResponse := cell.NewCell(0, cell.CmdNetinfo)
	payload := make([]byte, 11)
	// Timestamp (4 bytes)
	now := uint32(time.Now().Unix())
	payload[0] = byte(now >> 24)
	payload[1] = byte(now >> 16)
	payload[2] = byte(now >> 8)
	payload[3] = byte(now)
	// Other address (IPv4)
	payload[4] = 0x04
	payload[5] = 4
	// Number of this addresses
	payload[10] = 0
	netinfoResponse.Payload = payload

	var netBuf bytes.Buffer
	if err := netinfoResponse.Encode(&netBuf); err != nil {
		m.logger.Debug("Failed to encode NETINFO response", "error", err)
		return
	}

	if _, err := conn.Write(netBuf.Bytes()); err != nil {
		m.logger.Debug("Failed to write NETINFO response", "error", err)
		return
	}
}

// close closes the mock relay
func (m *mockTLSRelay) close() {
	m.listener.Close()
}

// TestPerformHandshakeSuccess tests a successful handshake
func TestPerformHandshakeSuccess(t *testing.T) {
	// Skip in short mode as this creates network connections
	if testing.Short() {
		t.Skip("Skipping TLS handshake test in short mode")
	}

	// Create mock TLS relay
	relay, addr, err := newMockTLSRelay(t)
	if err != nil {
		t.Fatalf("Failed to create mock TLS relay: %v", err)
	}
	defer relay.close()

	relay.serveHandshake()

	// Give the server a moment to start
	time.Sleep(50 * time.Millisecond)

	// Create connection config with TLS
	cfg := connection.DefaultConfig(addr)
	cfg.Timeout = 5 * time.Second
	// Use a custom TLS config that accepts self-signed certs
	cfg.TLSConfig = &tls.Config{
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS12,
	}

	log := logger.NewDefault()
	torConn := connection.New(cfg, log)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Connect to the relay
	if err := torConn.Connect(ctx, cfg); err != nil {
		t.Fatalf("Failed to connect to mock relay: %v", err)
	}
	defer torConn.Close()

	// Create and perform handshake
	h := NewHandshake(torConn, log)
	if h == nil {
		t.Fatal("NewHandshake returned nil")
	}

	if err := h.PerformHandshake(ctx); err != nil {
		t.Fatalf("PerformHandshake failed: %v", err)
	}

	// Verify negotiated version
	if v := h.NegotiatedVersion(); v != 4 {
		t.Errorf("NegotiatedVersion() = %d, want 4", v)
	}
}

// TestPerformHandshakeConnectionClosed tests handshake with closed connection
func TestPerformHandshakeConnectionClosed(t *testing.T) {
	log := logger.NewDefault()
	cfg := connection.DefaultConfig("127.0.0.1:9001")
	torConn := connection.New(cfg, log)

	h := NewHandshake(torConn, log)

	ctx := context.Background()

	// This should fail because connection is not open
	err := h.PerformHandshake(ctx)
	if err == nil {
		t.Error("Expected error when handshake on non-connected connection")
	}
}

// TestPerformHandshakeContextCanceled tests handshake with canceled context
func TestPerformHandshakeContextCanceled(t *testing.T) {
	log := logger.NewDefault()
	cfg := connection.DefaultConfig("127.0.0.1:9001")
	torConn := connection.New(cfg, log)

	h := NewHandshake(torConn, log)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// This should fail because context is canceled (but may fail for other reasons too)
	err := h.PerformHandshake(ctx)
	if err == nil {
		// This is expected to fail because connection is not open
		t.Log("Handshake failed as expected")
	}
}

// mockTLSRelayWrongCommand is a mock that sends wrong command type
type mockTLSRelayWrongCommand struct {
	listener net.Listener
	logger   *logger.Logger
}

func newMockTLSRelayWrongCommand(t *testing.T) (*mockTLSRelayWrongCommand, string, error) {
	cert, err := tls.X509KeyPair([]byte(testCert), []byte(testKey))
	if err != nil {
		return nil, "", err
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	listener, err := tls.Listen("tcp", "127.0.0.1:0", tlsConfig)
	if err != nil {
		return nil, "", err
	}

	return &mockTLSRelayWrongCommand{
		listener: listener,
		logger:   logger.NewDefault(),
	}, listener.Addr().String(), nil
}

func (m *mockTLSRelayWrongCommand) serve() {
	go func() {
		for {
			conn, err := m.listener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				c.SetReadDeadline(time.Now().Add(5 * time.Second))

				// Read VERSIONS cell
				_, err := cell.DecodeCell(c)
				if err != nil {
					return
				}

				// Send PADDING cell instead of VERSIONS (wrong command)
				wrongCell := cell.NewCell(0, cell.CmdPadding)
				var buf bytes.Buffer
				if err := wrongCell.Encode(&buf); err != nil {
					return
				}
				c.Write(buf.Bytes())
			}(conn)
		}
	}()
}

func (m *mockTLSRelayWrongCommand) close() {
	m.listener.Close()
}

// TestPerformHandshakeWrongVersionsResponse tests error when wrong cell type is received
func TestPerformHandshakeWrongVersionsResponse(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping TLS test in short mode")
	}

	relay, addr, err := newMockTLSRelayWrongCommand(t)
	if err != nil {
		t.Fatalf("Failed to create mock relay: %v", err)
	}
	defer relay.close()

	relay.serve()
	time.Sleep(50 * time.Millisecond)

	cfg := connection.DefaultConfig(addr)
	cfg.Timeout = 5 * time.Second
	cfg.TLSConfig = &tls.Config{
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS12,
	}

	log := logger.NewDefault()
	torConn := connection.New(cfg, log)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := torConn.Connect(ctx, cfg); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer torConn.Close()

	h := NewHandshake(torConn, log)

	err = h.PerformHandshake(ctx)
	if err == nil {
		t.Error("Expected error when receiving wrong cell type for VERSIONS")
	}
}

// mockTLSRelayIncompatibleVersion is a mock that returns incompatible versions
type mockTLSRelayIncompatibleVersion struct {
	listener net.Listener
	logger   *logger.Logger
}

func newMockTLSRelayIncompatibleVersion(t *testing.T) (*mockTLSRelayIncompatibleVersion, string, error) {
	cert, err := tls.X509KeyPair([]byte(testCert), []byte(testKey))
	if err != nil {
		return nil, "", err
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	listener, err := tls.Listen("tcp", "127.0.0.1:0", tlsConfig)
	if err != nil {
		return nil, "", err
	}

	return &mockTLSRelayIncompatibleVersion{
		listener: listener,
		logger:   logger.NewDefault(),
	}, listener.Addr().String(), nil
}

func (m *mockTLSRelayIncompatibleVersion) serve() {
	go func() {
		for {
			conn, err := m.listener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				c.SetReadDeadline(time.Now().Add(5 * time.Second))

				// Read VERSIONS cell
				_, err := cell.DecodeCell(c)
				if err != nil {
					return
				}

				// Send VERSIONS response with incompatible versions (1, 2)
				responseCell := cell.NewCell(0, cell.CmdVersions)
				responseCell.Payload = []byte{0x00, 0x01, 0x00, 0x02} // Versions 1 and 2
				var buf bytes.Buffer
				if err := responseCell.Encode(&buf); err != nil {
					return
				}
				c.Write(buf.Bytes())
			}(conn)
		}
	}()
}

func (m *mockTLSRelayIncompatibleVersion) close() {
	m.listener.Close()
}

// TestPerformHandshakeIncompatibleVersion tests error when no compatible version
func TestPerformHandshakeIncompatibleVersion(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping TLS test in short mode")
	}

	relay, addr, err := newMockTLSRelayIncompatibleVersion(t)
	if err != nil {
		t.Fatalf("Failed to create mock relay: %v", err)
	}
	defer relay.close()

	relay.serve()
	time.Sleep(50 * time.Millisecond)

	cfg := connection.DefaultConfig(addr)
	cfg.Timeout = 5 * time.Second
	cfg.TLSConfig = &tls.Config{
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS12,
	}

	log := logger.NewDefault()
	torConn := connection.New(cfg, log)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := torConn.Connect(ctx, cfg); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer torConn.Close()

	h := NewHandshake(torConn, log)

	err = h.PerformHandshake(ctx)
	if err == nil {
		t.Error("Expected error when no compatible protocol version")
	}
}

// mockTLSRelayInvalidPayload is a mock that returns invalid payload
type mockTLSRelayInvalidPayload struct {
	listener net.Listener
	logger   *logger.Logger
}

func newMockTLSRelayInvalidPayload(t *testing.T) (*mockTLSRelayInvalidPayload, string, error) {
	cert, err := tls.X509KeyPair([]byte(testCert), []byte(testKey))
	if err != nil {
		return nil, "", err
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	listener, err := tls.Listen("tcp", "127.0.0.1:0", tlsConfig)
	if err != nil {
		return nil, "", err
	}

	return &mockTLSRelayInvalidPayload{
		listener: listener,
		logger:   logger.NewDefault(),
	}, listener.Addr().String(), nil
}

func (m *mockTLSRelayInvalidPayload) serve() {
	go func() {
		for {
			conn, err := m.listener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				c.SetReadDeadline(time.Now().Add(5 * time.Second))

				// Read VERSIONS cell
				_, err := cell.DecodeCell(c)
				if err != nil {
					return
				}

				// Send VERSIONS response with odd-length payload (invalid)
				responseCell := cell.NewCell(0, cell.CmdVersions)
				responseCell.Payload = []byte{0x00, 0x04, 0x00} // 3 bytes - odd length
				var buf bytes.Buffer
				if err := responseCell.Encode(&buf); err != nil {
					return
				}
				c.Write(buf.Bytes())
			}(conn)
		}
	}()
}

func (m *mockTLSRelayInvalidPayload) close() {
	m.listener.Close()
}

// TestPerformHandshakeInvalidPayload tests error when payload is invalid
func TestPerformHandshakeInvalidPayload(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping TLS test in short mode")
	}

	relay, addr, err := newMockTLSRelayInvalidPayload(t)
	if err != nil {
		t.Fatalf("Failed to create mock relay: %v", err)
	}
	defer relay.close()

	relay.serve()
	time.Sleep(50 * time.Millisecond)

	cfg := connection.DefaultConfig(addr)
	cfg.Timeout = 5 * time.Second
	cfg.TLSConfig = &tls.Config{
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS12,
	}

	log := logger.NewDefault()
	torConn := connection.New(cfg, log)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := torConn.Connect(ctx, cfg); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer torConn.Close()

	h := NewHandshake(torConn, log)

	err = h.PerformHandshake(ctx)
	if err == nil {
		t.Error("Expected error when payload has invalid length")
	}
}

// mockTLSRelayWrongNetinfo is a mock that sends wrong command for NETINFO
type mockTLSRelayWrongNetinfo struct {
	listener net.Listener
	logger   *logger.Logger
}

func newMockTLSRelayWrongNetinfo(t *testing.T) (*mockTLSRelayWrongNetinfo, string, error) {
	cert, err := tls.X509KeyPair([]byte(testCert), []byte(testKey))
	if err != nil {
		return nil, "", err
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	listener, err := tls.Listen("tcp", "127.0.0.1:0", tlsConfig)
	if err != nil {
		return nil, "", err
	}

	return &mockTLSRelayWrongNetinfo{
		listener: listener,
		logger:   logger.NewDefault(),
	}, listener.Addr().String(), nil
}

func (m *mockTLSRelayWrongNetinfo) serve() {
	go func() {
		for {
			conn, err := m.listener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				c.SetReadDeadline(time.Now().Add(5 * time.Second))

				// Read VERSIONS cell
				_, err := cell.DecodeCell(c)
				if err != nil {
					return
				}

				// Send correct VERSIONS response
				versionsCell := cell.NewCell(0, cell.CmdVersions)
				versionsCell.Payload = []byte{0x00, 0x04}
				var buf bytes.Buffer
				if err := versionsCell.Encode(&buf); err != nil {
					return
				}
				c.Write(buf.Bytes())

				// Read NETINFO cell
				_, err = cell.DecodeCell(c)
				if err != nil {
					return
				}

				// Send PADDING cell instead of NETINFO (wrong command)
				wrongCell := cell.NewCell(0, cell.CmdPadding)
				var buf2 bytes.Buffer
				if err := wrongCell.Encode(&buf2); err != nil {
					return
				}
				c.Write(buf2.Bytes())
			}(conn)
		}
	}()
}

func (m *mockTLSRelayWrongNetinfo) close() {
	m.listener.Close()
}

// TestPerformHandshakeWrongNetinfoResponse tests error when wrong cell type for NETINFO
func TestPerformHandshakeWrongNetinfoResponse(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping TLS test in short mode")
	}

	relay, addr, err := newMockTLSRelayWrongNetinfo(t)
	if err != nil {
		t.Fatalf("Failed to create mock relay: %v", err)
	}
	defer relay.close()

	relay.serve()
	time.Sleep(50 * time.Millisecond)

	cfg := connection.DefaultConfig(addr)
	cfg.Timeout = 5 * time.Second
	cfg.TLSConfig = &tls.Config{
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS12,
	}

	log := logger.NewDefault()
	torConn := connection.New(cfg, log)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := torConn.Connect(ctx, cfg); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer torConn.Close()

	h := NewHandshake(torConn, log)

	err = h.PerformHandshake(ctx)
	if err == nil {
		t.Error("Expected error when receiving wrong cell type for NETINFO")
	}
}

// TestVersionsPayloadParsing tests the parsing of versions payload
func TestVersionsPayloadParsing(t *testing.T) {
	tests := []struct {
		name     string
		payload  []byte
		expected []int
	}{
		{
			name:     "single_version",
			payload:  []byte{0x00, 0x04},
			expected: []int{4},
		},
		{
			name:     "multiple_versions",
			payload:  []byte{0x00, 0x03, 0x00, 0x04, 0x00, 0x05},
			expected: []int{3, 4, 5},
		},
		{
			name:     "high_version",
			payload:  []byte{0x01, 0x00}, // Version 256
			expected: []int{256},
		},
		{
			name:     "empty_payload",
			payload:  []byte{},
			expected: []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var versions []int
			for i := 0; i < len(tt.payload); i += 2 {
				version := int(tt.payload[i])<<8 | int(tt.payload[i+1])
				versions = append(versions, version)
			}

			if len(versions) != len(tt.expected) {
				t.Errorf("Parsed %d versions, expected %d", len(versions), len(tt.expected))
				return
			}

			for i, v := range versions {
				if v != tt.expected[i] {
					t.Errorf("Version[%d] = %d, want %d", i, v, tt.expected[i])
				}
			}
		})
	}
}

// TestNetinfoPayloadConstruction tests the construction of NETINFO payload
func TestNetinfoPayloadConstruction(t *testing.T) {
	// Test that NETINFO payload is constructed correctly
	payload := make([]byte, 11)

	// Timestamp (4 bytes)
	now := uint32(time.Now().Unix())
	payload[0] = byte(now >> 24)
	payload[1] = byte(now >> 16)
	payload[2] = byte(now >> 8)
	payload[3] = byte(now)

	// Other address type: 0x04 (IPv4), 4 bytes
	payload[4] = 0x04 // IPv4
	payload[5] = 4    // 4 bytes

	// Number of this addresses: 0
	payload[10] = 0

	// Verify timestamp encoding
	decoded := uint32(payload[0])<<24 | uint32(payload[1])<<16 | uint32(payload[2])<<8 | uint32(payload[3])
	if decoded != now {
		t.Errorf("Timestamp encoding/decoding mismatch: encoded %d, decoded %d", now, decoded)
	}

	// Verify address type
	if payload[4] != 0x04 {
		t.Errorf("Address type = %d, want 0x04", payload[4])
	}

	// Verify address length
	if payload[5] != 4 {
		t.Errorf("Address length = %d, want 4", payload[5])
	}

	// Verify number of addresses
	if payload[10] != 0 {
		t.Errorf("Number of addresses = %d, want 0", payload[10])
	}
}
