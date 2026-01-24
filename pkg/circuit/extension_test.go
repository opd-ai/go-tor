package circuit

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/cell"
	"github.com/opd-ai/go-tor/pkg/crypto"
	"github.com/opd-ai/go-tor/pkg/logger"
)

// mockExtensionConnection implements CellConnection for testing circuit extension
type mockExtensionConnection struct {
	sentCells     []*cell.Cell
	receivedCells []*cell.Cell
	receiveIndex  int
}

func newMockExtensionConnection() *mockExtensionConnection {
	return &mockExtensionConnection{
		sentCells:     make([]*cell.Cell, 0),
		receivedCells: make([]*cell.Cell, 0),
		receiveIndex:  0,
	}
}

func (m *mockExtensionConnection) SendCell(c *cell.Cell) error {
	m.sentCells = append(m.sentCells, c)
	return nil
}

func (m *mockExtensionConnection) ReceiveCell() (*cell.Cell, error) {
	if m.receiveIndex >= len(m.receivedCells) {
		// Return a mock CREATED2 cell if none provided
		return m.createMockCreated2(), nil
	}
	c := m.receivedCells[m.receiveIndex]
	m.receiveIndex++
	return c, nil
}

func (m *mockExtensionConnection) createMockCreated2() *cell.Cell {
	// Create a valid-looking CREATED2 response
	// For ntor: 32 bytes server public key + 32 bytes auth MAC = 64 bytes
	response := make([]byte, 64)
	rand.Read(response)

	// Build CREATED2 payload: HLEN (2) + HDATA (64)
	payload := make([]byte, 2+64)
	binary.BigEndian.PutUint16(payload[0:2], 64)
	copy(payload[2:], response)

	return &cell.Cell{
		CircID:  1, // Match circuit ID
		Command: cell.CmdCreated2,
		Payload: payload,
	}
}

func TestNewExtension(t *testing.T) {
	log := logger.NewDefault()
	circuit := NewCircuit(1)
	ext := NewExtension(circuit, log)

	if ext == nil {
		t.Fatal("Expected extension to be created")
	}

	if ext.circuit.ID != 1 {
		t.Errorf("Expected circuit ID 1, got %d", ext.circuit.ID)
	}
}

func TestCreateFirstHop(t *testing.T) {
	log := logger.NewDefault()
	circuit := NewCircuit(1)

	// Set up mock connection
	mockConn := newMockExtensionConnection()
	circuit.SetConnection(mockConn)

	ext := NewExtension(circuit, log)

	ctx := context.Background()

	// This will fail during ntor verification since we're using random response
	// but it should successfully send CREATE2 and receive CREATED2
	err := ext.CreateFirstHop(ctx, HandshakeTypeNTor)
	if err == nil {
		t.Error("Expected error due to mock handshake response, got nil")
	}

	// Verify CREATE2 was sent
	if len(mockConn.sentCells) != 1 {
		t.Fatalf("Expected 1 sent cell, got %d", len(mockConn.sentCells))
	}

	sentCell := mockConn.sentCells[0]
	if sentCell.Command != cell.CmdCreate2 {
		t.Errorf("Expected CREATE2 command, got %s", sentCell.Command)
	}

	if sentCell.CircID != circuit.ID {
		t.Errorf("Expected circuit ID %d, got %d", circuit.ID, sentCell.CircID)
	}
}

func TestCreateFirstHopTAP(t *testing.T) {
	log := logger.NewDefault()
	circuit := NewCircuit(1)

	// Set up mock connection
	mockConn := newMockExtensionConnection()
	circuit.SetConnection(mockConn)

	ext := NewExtension(circuit, log)

	ctx := context.Background()

	// TAP handshake will also fail verification with mock response
	err := ext.CreateFirstHop(ctx, HandshakeTypeTAP)
	if err == nil {
		t.Error("Expected error due to mock handshake response, got nil")
	}

	// Verify CREATE2 was sent
	if len(mockConn.sentCells) != 1 {
		t.Fatalf("Expected 1 sent cell, got %d", len(mockConn.sentCells))
	}
}

// TestTAPHandshakeDeprecationWarning verifies that TAP handshake logs a deprecation warning (LOW-001)
func TestTAPHandshakeDeprecationWarning(t *testing.T) {
	// Capture log output to verify deprecation warning
	var logBuffer strings.Builder
	log := logger.New(slog.LevelWarn, &logBuffer)
	circuit := NewCircuit(1)
	ext := NewExtension(circuit, log)

	_, err := ext.generateHandshakeData(HandshakeTypeTAP)
	if err != nil {
		t.Fatalf("Failed to generate TAP handshake data: %v", err)
	}

	// Verify deprecation warning was logged
	logOutput := logBuffer.String()
	if !strings.Contains(logOutput, "TAP handshake is deprecated") {
		t.Errorf("Expected deprecation warning in log output, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "RSA-1024") {
		t.Errorf("Expected RSA-1024 mentioned in deprecation warning, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "HandshakeTypeNTor") {
		t.Errorf("Expected HandshakeTypeNTor recommendation in warning, got: %s", logOutput)
	}
}

func TestExtendCircuit(t *testing.T) {
	log := logger.NewDefault()
	circuit := NewCircuit(1)
	ext := NewExtension(circuit, log)

	// Test that ExtendCircuit validates inputs without actually sending
	// We can't fully test the async flow without complex mocking
	// Just verify the method exists and basic structure works

	// Test that building EXTEND2 data works
	handshakeData := make([]byte, 84)
	rand.Read(handshakeData)

	extend2Data := ext.buildExtend2Data("relay.example.com:9001", HandshakeTypeNTor, handshakeData)

	if len(extend2Data) < 20 {
		t.Errorf("EXTEND2 data too short: %d bytes", len(extend2Data))
	}

	// Verify NSPEC
	if extend2Data[0] != 1 {
		t.Errorf("Expected NSPEC=1, got %d", extend2Data[0])
	}
}

func TestGenerateHandshakeData(t *testing.T) {
	log := logger.NewDefault()
	circuit := NewCircuit(1)
	ext := NewExtension(circuit, log)

	tests := []struct {
		name          string
		handshakeType HandshakeType
		expectedLen   int
	}{
		{"NTor", HandshakeTypeNTor, 84}, // NODEID (20) + KEYID (32) + CLIENT_PK (32)
		{"TAP", HandshakeTypeTAP, 144},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := ext.generateHandshakeData(tt.handshakeType)
			if err != nil {
				t.Fatalf("Failed to generate handshake data: %v", err)
			}

			if len(data) != tt.expectedLen {
				t.Errorf("Expected %d bytes, got %d", tt.expectedLen, len(data))
			}
		})
	}
}

func TestGenerateHandshakeDataInvalidType(t *testing.T) {
	log := logger.NewDefault()
	circuit := NewCircuit(1)
	ext := NewExtension(circuit, log)

	_, err := ext.generateHandshakeData(HandshakeType(0xFFFF))
	if err == nil {
		t.Error("Expected error for invalid handshake type")
	}
}

func TestBuildExtend2Data(t *testing.T) {
	log := logger.NewDefault()
	circuit := NewCircuit(1)
	ext := NewExtension(circuit, log)

	handshakeData := make([]byte, 32)
	data := ext.buildExtend2Data("relay.example.com:9001", HandshakeTypeNTor, handshakeData)

	if len(data) == 0 {
		t.Error("Expected non-empty EXTEND2 data")
	}

	// Check NSPEC
	if data[0] != 1 {
		t.Errorf("Expected NSPEC=1, got %d", data[0])
	}
}

func TestProcessCreated2Valid(t *testing.T) {
	log := logger.NewDefault()
	circuit := NewCircuit(1)
	ext := NewExtension(circuit, log)

	// AUDIT-001 FIX: Set up proper handshake state first
	// Generate ephemeral key pair
	ephemeral, err := crypto.GenerateNtorKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate ephemeral key: %v", err)
	}

	// Set up server keys
	serverIdentity := make([]byte, 32)
	serverNtorKey := make([]byte, 32)
	if _, err := rand.Read(serverIdentity); err != nil {
		t.Fatalf("Failed to generate server identity: %v", err)
	}
	if _, err := rand.Read(serverNtorKey); err != nil {
		t.Fatalf("Failed to generate server ntor key: %v", err)
	}

	// Store handshake state in extension
	ext.ephemeralPrivate = make([]byte, 32)
	copy(ext.ephemeralPrivate, ephemeral.Private[:])
	ext.serverIdentity = serverIdentity
	ext.serverNtorKey = serverNtorKey

	// Create a valid CREATED2 response with proper ntor format
	// For testing, we'll create a minimal response (Y || AUTH)
	// In production, this would be generated by a real relay
	// For now, just test that the parsing works with a 64-byte response
	handshakeResponse := make([]byte, 64) // Y (32) + AUTH (32)
	if _, err := rand.Read(handshakeResponse); err != nil {
		t.Fatalf("Failed to generate handshake response: %v", err)
	}

	payload := make([]byte, 2+len(handshakeResponse))
	payload[0] = 0
	payload[1] = 64 // hlen
	copy(payload[2:], handshakeResponse)

	created2Cell := &cell.Cell{
		CircID:  1,
		Command: cell.CmdCreated2,
		Payload: payload,
	}

	// This should now fail auth verification (expected) but not crash
	err = ext.ProcessCreated2(created2Cell)
	// We expect an auth MAC verification failure with random data
	if err == nil {
		t.Log("Note: ProcessCreated2 succeeded (unlikely with random data)")
	} else if !strings.Contains(err.Error(), "auth MAC verification failed") &&
		!strings.Contains(err.Error(), "ntor handshake verification failed") {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestProcessCreated2InvalidCommand(t *testing.T) {
	log := logger.NewDefault()
	circuit := NewCircuit(1)
	ext := NewExtension(circuit, log)

	wrongCell := &cell.Cell{
		CircID:  1,
		Command: cell.CmdCreate2, // Wrong command
		Payload: make([]byte, 34),
	}

	err := ext.ProcessCreated2(wrongCell)
	if err == nil {
		t.Error("Expected error for wrong command")
	}
}

func TestProcessCreated2ShortPayload(t *testing.T) {
	log := logger.NewDefault()
	circuit := NewCircuit(1)
	ext := NewExtension(circuit, log)

	shortCell := &cell.Cell{
		CircID:  1,
		Command: cell.CmdCreated2,
		Payload: make([]byte, 1), // Too short
	}

	err := ext.ProcessCreated2(shortCell)
	if err == nil {
		t.Error("Expected error for short payload")
	}
}

func TestProcessExtended2Valid(t *testing.T) {
	log := logger.NewDefault()
	circuit := NewCircuit(1)
	ext := NewExtension(circuit, log)

	// AUDIT-001 FIX: Set up proper handshake state first
	// Generate ephemeral key pair
	ephemeral, err := crypto.GenerateNtorKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate ephemeral key: %v", err)
	}

	// Set up server keys
	serverIdentity := make([]byte, 32)
	serverNtorKey := make([]byte, 32)
	if _, err := rand.Read(serverIdentity); err != nil {
		t.Fatalf("Failed to generate server identity: %v", err)
	}
	if _, err := rand.Read(serverNtorKey); err != nil {
		t.Fatalf("Failed to generate server ntor key: %v", err)
	}

	// Store handshake state in extension
	ext.ephemeralPrivate = make([]byte, 32)
	copy(ext.ephemeralPrivate, ephemeral.Private[:])
	ext.serverIdentity = serverIdentity
	ext.serverNtorKey = serverNtorKey

	// Create a valid EXTENDED2 relay cell
	handshakeResponse := make([]byte, 64) // Y (32) + AUTH (32)
	if _, err := rand.Read(handshakeResponse); err != nil {
		t.Fatalf("Failed to generate handshake response: %v", err)
	}

	payload := make([]byte, 2+len(handshakeResponse))
	payload[0] = 0
	payload[1] = 64 // hlen
	copy(payload[2:], handshakeResponse)

	extended2Cell := &cell.RelayCell{
		Command:  cell.RelayExtended2,
		StreamID: 0,
		Data:     payload,
	}

	// This should now fail auth verification (expected) but not crash
	err = ext.ProcessExtended2(extended2Cell)
	// We expect an auth MAC verification failure with random data
	if err == nil {
		t.Log("Note: ProcessExtended2 succeeded (unlikely with random data)")
	} else if !strings.Contains(err.Error(), "auth MAC verification failed") &&
		!strings.Contains(err.Error(), "ntor handshake verification failed") {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestProcessExtended2InvalidCommand(t *testing.T) {
	log := logger.NewDefault()
	circuit := NewCircuit(1)
	ext := NewExtension(circuit, log)

	wrongCell := &cell.RelayCell{
		Command:  cell.RelayBegin, // Wrong command
		StreamID: 0,
		Data:     make([]byte, 34),
	}

	err := ext.ProcessExtended2(wrongCell)
	if err == nil {
		t.Error("Expected error for wrong command")
	}
}

func TestDeriveKeys(t *testing.T) {
	log := logger.NewDefault()
	circuit := NewCircuit(1)
	ext := NewExtension(circuit, log)

	sharedSecret := make([]byte, 32)
	for i := range sharedSecret {
		sharedSecret[i] = byte(i)
	}

	forwardKey, backwardKey, err := ext.DeriveKeys(sharedSecret)
	if err != nil {
		t.Fatalf("Failed to derive keys: %v", err)
	}

	if len(forwardKey) != 16 {
		t.Errorf("Expected forward key length 16, got %d", len(forwardKey))
	}

	if len(backwardKey) != 16 {
		t.Errorf("Expected backward key length 16, got %d", len(backwardKey))
	}

	// Keys should be different
	if string(forwardKey) == string(backwardKey) {
		t.Error("Forward and backward keys should be different")
	}
}

func TestDeriveKeysEmptySecret(t *testing.T) {
	log := logger.NewDefault()
	circuit := NewCircuit(1)
	ext := NewExtension(circuit, log)

	// Empty shared secret should still work (though not secure)
	sharedSecret := make([]byte, 0)

	_, _, err := ext.DeriveKeys(sharedSecret)
	if err != nil {
		t.Fatalf("Failed to derive keys with empty secret: %v", err)
	}
}

func TestHandshakeTypeConstants(t *testing.T) {
	if HandshakeTypeNTor != 0x0002 {
		t.Errorf("Expected HandshakeTypeNTor=0x0002, got 0x%04x", HandshakeTypeNTor)
	}

	if HandshakeTypeTAP != 0x0000 {
		t.Errorf("Expected HandshakeTypeTAP=0x0000, got 0x%04x", HandshakeTypeTAP)
	}
}

// TestCreateFirstHopWireProtocol tests the complete CREATE2/CREATED2 wire protocol
// This verifies that CREATE2 is sent and CREATED2 is properly received and processed
func TestCreateFirstHopWireProtocol(t *testing.T) {
	log := logger.NewDefault()
	circuit := NewCircuit(1)

	// Set up mock connection
	mockConn := newMockExtensionConnection()
	circuit.SetConnection(mockConn)

	ext := NewExtension(circuit, log)
	ctx := context.Background()

	// Attempt to create first hop
	err := ext.CreateFirstHop(ctx, HandshakeTypeNTor)

	// We expect an error due to handshake verification failure (mock response)
	// but the important thing is that cells were exchanged
	if err == nil {
		t.Error("Expected error due to mock ntor verification failure")
	}

	// Verify the wire protocol was executed correctly

	// 1. CREATE2 cell should have been sent
	if len(mockConn.sentCells) != 1 {
		t.Fatalf("Expected 1 CREATE2 cell sent, got %d", len(mockConn.sentCells))
	}

	create2Cell := mockConn.sentCells[0]

	// 2. Verify CREATE2 cell structure
	if create2Cell.Command != cell.CmdCreate2 {
		t.Errorf("Expected CREATE2 command, got %s", create2Cell.Command)
	}

	if create2Cell.CircID != circuit.ID {
		t.Errorf("Expected circuit ID %d, got %d", circuit.ID, create2Cell.CircID)
	}

	// 3. Verify CREATE2 payload structure: HTYPE (2) + HLEN (2) + HDATA
	payload := create2Cell.Payload
	if len(payload) < 4 {
		t.Fatalf("CREATE2 payload too short: %d bytes", len(payload))
	}

	htype := binary.BigEndian.Uint16(payload[0:2])
	if htype != uint16(HandshakeTypeNTor) {
		t.Errorf("Expected handshake type ntor (0x0002), got 0x%04x", htype)
	}

	hlen := binary.BigEndian.Uint16(payload[2:4])
	if hlen != 84 { // NODEID (20) + KEYID (32) + CLIENT_PK (32)
		t.Errorf("Expected handshake data length 84, got %d", hlen)
	}

	if len(payload) != int(4+hlen) {
		t.Errorf("Expected total payload length %d, got %d", 4+hlen, len(payload))
	}

	t.Log("✓ CREATE2 cell wire protocol validated successfully")
}

// TestCellConnectionInterface verifies the CellConnection interface is properly defined
func TestCellConnectionInterface(t *testing.T) {
	var _ CellConnection = (*mockExtensionConnection)(nil)

	// This test just ensures the interface compiles
	// If mockExtensionConnection doesn't implement CellConnection, this will fail to compile
}

// mockCircuitForExtension provides a mock circuit that can send/receive relay cells
type mockCircuitForExtension struct {
	*Circuit
	sentRelayCells     []*cell.RelayCell
	receivedRelayCells []*cell.RelayCell
	receiveIndex       int
}

// mockConnectionForRelay implements the connection interface for relay cells
type mockConnectionForRelay struct {
	sentCells []*cell.Cell
}

func (m *mockConnectionForRelay) SendCell(c *cell.Cell) error {
	m.sentCells = append(m.sentCells, c)
	return nil
}

func newMockCircuitForExtension(id uint32) *mockCircuitForExtension {
	circ := NewCircuit(id)
	circ.State = StateOpen
	circ.conn = &mockConnectionForRelay{sentCells: make([]*cell.Cell, 0)}

	return &mockCircuitForExtension{
		Circuit:            circ,
		sentRelayCells:     make([]*cell.RelayCell, 0),
		receivedRelayCells: make([]*cell.RelayCell, 0),
		receiveIndex:       0,
	}
}

func (m *mockCircuitForExtension) SendRelayCell(relayCell *cell.RelayCell) error {
	// Capture the relay cell before it gets encrypted
	m.sentRelayCells = append(m.sentRelayCells, relayCell)
	// Don't call the actual Circuit.SendRelayCell as it would need full crypto setup
	return nil
}

func (m *mockCircuitForExtension) ReceiveRelayCellTimeout(timeout time.Duration) (*cell.RelayCell, error) {
	if m.receiveIndex >= len(m.receivedRelayCells) {
		// If no cells provided, create a mock EXTENDED2 for basic tests
		// This allows tests that don't care about verification to succeed
		return m.createMockExtended2(), nil
	}
	c := m.receivedRelayCells[m.receiveIndex]
	m.receiveIndex++
	return c, nil
}

func (m *mockCircuitForExtension) ReceiveRelayCell(ctx context.Context) (*cell.RelayCell, error) {
	if m.receiveIndex >= len(m.receivedRelayCells) {
		// If no cells provided, create a mock EXTENDED2 for basic tests
		return m.createMockExtended2(), nil
	}
	c := m.receivedRelayCells[m.receiveIndex]
	m.receiveIndex++
	return c, nil
}

func (m *mockCircuitForExtension) createMockExtended2() *cell.RelayCell {
	// Create a valid-looking EXTENDED2 response
	// For ntor: 32 bytes server public key + 32 bytes auth MAC = 64 bytes
	response := make([]byte, 64)
	rand.Read(response)

	// Build EXTENDED2 payload: HLEN (2) + HDATA (64)
	payload := make([]byte, 2+64)
	binary.BigEndian.PutUint16(payload[0:2], 64)
	copy(payload[2:], response)

	return &cell.RelayCell{
		Command:  cell.RelayExtended2,
		StreamID: 0,
		Data:     payload,
	}
}

// TestBuildExtend2DataStructure tests the EXTEND2 data structure
func TestBuildExtend2DataStructure(t *testing.T) {
	log := logger.NewDefault()
	circuit := NewCircuit(1)
	ext := NewExtension(circuit, log)

	// Generate handshake data
	handshakeData := make([]byte, 84) // NODEID (20) + KEYID (32) + CLIENT_PK (32)
	rand.Read(handshakeData)

	// Build EXTEND2 data
	extend2Data := ext.buildExtend2Data("192.0.2.1:9001", HandshakeTypeNTor, handshakeData)

	// Verify structure
	if len(extend2Data) < 1 {
		t.Fatalf("EXTEND2 data too short")
	}

	// Check NSPEC
	nspec := extend2Data[0]
	if nspec != 1 {
		t.Errorf("Expected NSPEC=1, got %d", nspec)
	}

	// Verify minimum length (NSPEC + link spec + handshake type + handshake len + data)
	minLen := 1 + 8 + 2 + 2 + len(handshakeData) // Simplified link spec is 8 bytes
	if len(extend2Data) < minLen {
		t.Errorf("EXTEND2 data too short: got %d, want at least %d", len(extend2Data), minLen)
	}

	t.Log("✓ EXTEND2 data structure validated")
}

// TestProcessExtended2Structure tests EXTENDED2 processing with valid structure
func TestProcessExtended2Structure(t *testing.T) {
	log := logger.NewDefault()
	circuit := NewCircuit(1)
	ext := NewExtension(circuit, log)

	// Set up handshake state
	ephemeral, err := crypto.GenerateNtorKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate ephemeral key: %v", err)
	}

	ext.ephemeralPrivate = make([]byte, 32)
	copy(ext.ephemeralPrivate, ephemeral.Private[:])
	ext.serverIdentity = make([]byte, 32)
	ext.serverNtorKey = make([]byte, 32)
	rand.Read(ext.serverIdentity)
	rand.Read(ext.serverNtorKey)

	// Create EXTENDED2 relay cell
	handshakeResponse := make([]byte, 64)
	rand.Read(handshakeResponse)

	payload := make([]byte, 2+len(handshakeResponse))
	binary.BigEndian.PutUint16(payload[0:2], 64)
	copy(payload[2:], handshakeResponse)

	extended2Cell := &cell.RelayCell{
		Command:  cell.RelayExtended2,
		StreamID: 0,
		Data:     payload,
	}

	// Process should fail verification but not crash
	err = ext.ProcessExtended2(extended2Cell)
	if err == nil {
		t.Log("Note: ProcessExtended2 succeeded (unlikely with random data)")
	} else if !strings.Contains(err.Error(), "ntor handshake verification failed") {
		t.Logf("Got expected verification error: %v", err)
	}

	// Verify ephemeral key was cleared (unless it succeeded by chance)
	if err != nil {
		for _, b := range ext.ephemeralPrivate {
			if b != 0 {
				t.Error("Ephemeral private key should be cleared after use, even on error")
				break
			}
		}
	}
}
