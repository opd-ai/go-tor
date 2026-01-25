package relay

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/cell"
	"github.com/opd-ai/go-tor/pkg/logger"
)

// mockConn is a mock connection for testing
type mockConn struct {
	readData  []byte
	readPos   int
	writeData []byte
	closed    bool
}

func newMockConn() *mockConn {
	return &mockConn{
		readData: make([]byte, 0),
	}
}

func (m *mockConn) Read(b []byte) (n int, err error) {
	if m.readPos >= len(m.readData) {
		return 0, io.EOF
	}
	n = copy(b, m.readData[m.readPos:])
	m.readPos += n
	return n, nil
}

func (m *mockConn) Write(b []byte) (n int, err error) {
	m.writeData = append(m.writeData, b...)
	return len(b), nil
}

func (m *mockConn) Close() error {
	m.closed = true
	return nil
}

func (m *mockConn) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 9001}
}

func (m *mockConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("192.168.1.100"), Port: 54321}
}

func (m *mockConn) SetDeadline(t time.Time) error      { return nil }
func (m *mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *mockConn) SetWriteDeadline(t time.Time) error { return nil }

func (m *mockConn) addReadData(data []byte) {
	m.readData = append(m.readData, data...)
}

func (m *mockConn) getWrittenCells() ([]*cell.Cell, error) {
	var cells []*cell.Cell
	pos := 0

	for pos < len(m.writeData) {
		if pos+5 > len(m.writeData) {
			break
		}

		// Read header
		circID := binary.BigEndian.Uint32(m.writeData[pos : pos+4])
		command := cell.Command(m.writeData[pos+4])
		pos += 5

		// Determine payload length
		var payloadLen int
		if command.IsVariableLength() {
			if pos+2 > len(m.writeData) {
				break
			}
			payloadLen = int(binary.BigEndian.Uint16(m.writeData[pos : pos+2]))
			pos += 2
		} else {
			payloadLen = cell.PayloadLen
		}

		// Read payload
		if pos+payloadLen > len(m.writeData) {
			break
		}
		payload := make([]byte, payloadLen)
		copy(payload, m.writeData[pos:pos+payloadLen])
		pos += payloadLen

		cells = append(cells, &cell.Cell{
			CircID:  circID,
			Command: command,
			Payload: payload,
		})
	}

	return cells, nil
}

func TestNewLinkProtocolHandler(t *testing.T) {
	keys := generateTestRelayKeys(t)
	log := logger.NewDefault()

	handler := NewLinkProtocolHandler(keys, log)
	if handler == nil {
		t.Fatal("NewLinkProtocolHandler returned nil")
	}
	if handler.keys != keys {
		t.Error("Keys not set correctly")
	}
	if handler.logger == nil {
		t.Error("Logger should not be nil")
	}

	// Test with nil logger (should create default)
	handler2 := NewLinkProtocolHandler(keys, nil)
	if handler2.logger == nil {
		t.Error("Handler should create default logger if nil provided")
	}
}

func TestSelectVersion(t *testing.T) {
	keys := generateTestRelayKeys(t)
	handler := NewLinkProtocolHandler(keys, nil)

	tests := []struct {
		name            string
		clientVersions  []int
		expectedVersion int
	}{
		{
			name:            "Client supports v5",
			clientVersions:  []int{3, 4, 5},
			expectedVersion: 5,
		},
		{
			name:            "Client supports only v4",
			clientVersions:  []int{4},
			expectedVersion: 4,
		},
		{
			name:            "Client supports only v3",
			clientVersions:  []int{3},
			expectedVersion: 3,
		},
		{
			name:            "Client supports v2 and v3",
			clientVersions:  []int{2, 3},
			expectedVersion: 3,
		},
		{
			name:            "No compatible version",
			clientVersions:  []int{1, 2},
			expectedVersion: 0,
		},
		{
			name:            "Client supports higher version",
			clientVersions:  []int{5, 6, 7},
			expectedVersion: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version := handler.selectVersion(tt.clientVersions)
			if version != tt.expectedVersion {
				t.Errorf("selectVersion(%v) = %d, want %d",
					tt.clientVersions, version, tt.expectedVersion)
			}
		})
	}
}

func TestReceiveVersions(t *testing.T) {
	keys := generateTestRelayKeys(t)
	handler := NewLinkProtocolHandler(keys, nil)

	// Build VERSIONS cell
	versions := []uint16{3, 4, 5}
	payload := make([]byte, len(versions)*2)
	for i, v := range versions {
		binary.BigEndian.PutUint16(payload[i*2:], v)
	}

	versionsCell := cell.NewCell(0, cell.CmdVersions)
	versionsCell.Payload = payload
	var buf bytes.Buffer
	err := versionsCell.Encode(&buf)
	if err != nil {
		t.Fatalf("Failed to encode VERSIONS cell: %v", err)
	}
	cellData := buf.Bytes()

	// Create mock connection with VERSIONS cell
	conn := newMockConn()
	conn.addReadData(cellData)

	ctx := context.Background()
	receivedVersions, err := handler.receiveVersions(ctx, conn)
	if err != nil {
		t.Fatalf("receiveVersions failed: %v", err)
	}

	if len(receivedVersions) != 3 {
		t.Errorf("Expected 3 versions, got %d", len(receivedVersions))
	}
	expectedVersions := []int{3, 4, 5}
	for i, v := range receivedVersions {
		if v != expectedVersions[i] {
			t.Errorf("Version[%d] = %d, want %d", i, v, expectedVersions[i])
		}
	}
}

func TestReceiveVersionsInvalidCell(t *testing.T) {
	keys := generateTestRelayKeys(t)
	handler := NewLinkProtocolHandler(keys, nil)

	// Build NETINFO cell instead of VERSIONS
	netinfoCell := cell.NewCell(0, cell.CmdNetinfo)
	netinfoCell.Payload = make([]byte, 20)
	var buf bytes.Buffer
	err := netinfoCell.Encode(&buf)
	if err != nil {
		t.Fatalf("Failed to encode NETINFO cell: %v", err)
	}
	cellData := buf.Bytes()

	conn := newMockConn()
	conn.addReadData(cellData)

	ctx := context.Background()
	_, err = handler.receiveVersions(ctx, conn)
	if err == nil {
		t.Error("receiveVersions should fail with non-VERSIONS cell")
	}
}

func TestSendVersions(t *testing.T) {
	keys := generateTestRelayKeys(t)
	handler := NewLinkProtocolHandler(keys, nil)

	conn := newMockConn()
	err := handler.sendVersions(conn)
	if err != nil {
		t.Fatalf("sendVersions failed: %v", err)
	}

	// Parse written cell
	cells, err := conn.getWrittenCells()
	if err != nil {
		t.Fatalf("Failed to parse written cells: %v", err)
	}

	if len(cells) != 1 {
		t.Fatalf("Expected 1 cell, got %d", len(cells))
	}

	c := cells[0]
	if c.Command != cell.CmdVersions {
		t.Errorf("Expected VERSIONS cell, got %s", c.Command)
	}

	// Parse versions
	if len(c.Payload)%2 != 0 {
		t.Fatalf("Invalid payload length: %d", len(c.Payload))
	}

	var versions []int
	for i := 0; i < len(c.Payload); i += 2 {
		version := int(binary.BigEndian.Uint16(c.Payload[i : i+2]))
		versions = append(versions, version)
	}

	expectedVersions := []int{3, 4, 5}
	if len(versions) != len(expectedVersions) {
		t.Errorf("Expected %d versions, got %d", len(expectedVersions), len(versions))
	}
	for i, v := range versions {
		if v != expectedVersions[i] {
			t.Errorf("Version[%d] = %d, want %d", i, v, expectedVersions[i])
		}
	}
}

func TestSendCerts(t *testing.T) {
	keys := generateTestRelayKeys(t)
	handler := NewLinkProtocolHandler(keys, nil)

	conn := newMockConn()
	err := handler.sendCerts(conn)
	if err != nil {
		t.Fatalf("sendCerts failed: %v", err)
	}

	// Parse written cell
	cells, err := conn.getWrittenCells()
	if err != nil {
		t.Fatalf("Failed to parse written cells: %v", err)
	}

	if len(cells) != 1 {
		t.Fatalf("Expected 1 cell, got %d", len(cells))
	}

	c := cells[0]
	if c.Command != cell.CmdCerts {
		t.Errorf("Expected CERTS cell, got %s", c.Command)
	}

	// Parse CERTS payload
	if len(c.Payload) < 1 {
		t.Fatal("CERTS payload too short")
	}

	numCerts := int(c.Payload[0])
	if numCerts < 1 {
		t.Error("Expected at least 1 certificate")
	}

	t.Logf("CERTS cell contains %d certificates", numCerts)
}

func TestSendNetinfo(t *testing.T) {
	keys := generateTestRelayKeys(t)
	handler := NewLinkProtocolHandler(keys, nil)

	conn := newMockConn()
	err := handler.sendNetinfo(conn)
	if err != nil {
		t.Fatalf("sendNetinfo failed: %v", err)
	}

	// Parse written cell
	cells, err := conn.getWrittenCells()
	if err != nil {
		t.Fatalf("Failed to parse written cells: %v", err)
	}

	if len(cells) != 1 {
		t.Fatalf("Expected 1 cell, got %d", len(cells))
	}

	c := cells[0]
	if c.Command != cell.CmdNetinfo {
		t.Errorf("Expected NETINFO cell, got %s", c.Command)
	}

	// Validate payload structure
	if len(c.Payload) < 10 {
		t.Errorf("NETINFO payload too short: %d bytes", len(c.Payload))
	}

	// Check timestamp (first 4 bytes)
	timestamp := binary.BigEndian.Uint32(c.Payload[0:4])
	if timestamp == 0 {
		t.Log("Warning: timestamp is 0 (may be expected in some cases)")
	}
}

func TestReceiveNetinfo(t *testing.T) {
	keys := generateTestRelayKeys(t)
	handler := NewLinkProtocolHandler(keys, nil)

	// Build NETINFO cell
	var payload []byte
	// Timestamp
	timestamp := uint32(time.Now().Unix())
	payload = append(payload,
		byte(timestamp>>24), byte(timestamp>>16),
		byte(timestamp>>8), byte(timestamp))
	// OtherAddress (IPv4)
	payload = append(payload, 0x04, 4, 127, 0, 0, 1)
	// NumAddresses
	payload = append(payload, 0)

	netinfoCell := cell.NewCell(0, cell.CmdNetinfo)
	netinfoCell.Payload = payload
	var buf bytes.Buffer
	err := netinfoCell.Encode(&buf)
	if err != nil {
		t.Fatalf("Failed to encode NETINFO cell: %v", err)
	}
	cellData := buf.Bytes()

	conn := newMockConn()
	conn.addReadData(cellData)

	ctx := context.Background()
	err = handler.receiveNetinfo(ctx, conn)
	if err != nil {
		t.Fatalf("receiveNetinfo failed: %v", err)
	}
}

func TestBuildEd25519SigningCert(t *testing.T) {
	keys := generateTestRelayKeys(t)
	handler := NewLinkProtocolHandler(keys, nil)

	cert, err := handler.buildEd25519SigningCert()
	if err != nil {
		t.Fatalf("buildEd25519SigningCert failed: %v", err)
	}

	// Validate cert structure
	// Version (1) + CertType (1) + Expiration (4) + CertKeyType (1) +
	// CertifiedKey (32) + NumExtensions (1) + Signature (64) = 104 bytes minimum
	if len(cert) < 104 {
		t.Errorf("Expected cert length >= 104, got %d", len(cert))
	}

	// Validate version
	if cert[0] != 0x01 {
		t.Errorf("Expected version 1, got %d", cert[0])
	}

	// Validate cert type
	if cert[1] != 0x04 {
		t.Errorf("Expected cert type 4, got %d", cert[1])
	}

	// Validate number of extensions (at offset 38: 1+1+4+1+32-1 = 38)
	numExtensionsOffset := 1 + 1 + 4 + 1 + 32 // version + certType + expiration + certKeyType + certifiedKey
	numExtensions := cert[numExtensionsOffset]
	if numExtensions != 0x00 {
		t.Errorf("Expected 0 extensions, got %d", numExtensions)
	}

	// Validate signature (last 64 bytes after extensions section)
	signatureOffset := numExtensionsOffset + 1 // after numExtensions byte
	signature := cert[signatureOffset : signatureOffset+64]
	certBody := cert[0:signatureOffset]

	if !ed25519.Verify(keys.Ed25519Public, certBody, signature) {
		t.Error("Ed25519 certificate signature verification failed")
	}
}

func generateTestRelayKeys(t *testing.T) *RelayKeys {
	t.Helper()

	// Generate Ed25519 keys
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate Ed25519 key: %v", err)
	}

	// Create minimal keys
	keys := &RelayKeys{
		Ed25519Public:  pub,
		Ed25519Private: priv,
		TLSCert:        make([]byte, 100), // Dummy cert
	}

	return keys
}

func TestHandleConnectionTimeout(t *testing.T) {
	keys := generateTestRelayKeys(t)
	handler := NewLinkProtocolHandler(keys, nil)

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Create mock connection that doesn't send data
	conn := newMockConn()

	// This should timeout
	_, err := handler.HandleConnection(ctx, conn)
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}
}
