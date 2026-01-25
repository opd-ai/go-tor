package onion

import (
	"bytes"
	"crypto/rand"
	"errors"
	"testing"

	"github.com/opd-ai/go-tor/pkg/cell"
	"github.com/opd-ai/go-tor/pkg/crypto"
	"golang.org/x/crypto/curve25519"
)

// MockCircuit implements CircuitInterface for testing
type MockCircuit struct {
	id          uint32
	sentCells   []*cell.RelayCell
	sendError   error
}

func (m *MockCircuit) SendRelayCell(c *cell.RelayCell) error {
	if m.sendError != nil {
		return m.sendError
	}
	m.sentCells = append(m.sentCells, c)
	return nil
}

// TestBuildRendezvous1CellV2 tests RENDEZVOUS1 cell construction
func TestBuildRendezvous1CellV2(t *testing.T) {
	// Generate server keys
	serverNtor, err := crypto.GenerateNtorKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate server ntor key: %v", err)
	}

	serverIdentity := make([]byte, 32)
	if _, err := rand.Read(serverIdentity); err != nil {
		t.Fatalf("Failed to generate server identity: %v", err)
	}

	// Generate rendezvous cookie
	rendezvousCookie := make([]byte, 20)
	if _, err := rand.Read(rendezvousCookie); err != nil {
		t.Fatalf("Failed to generate rendezvous cookie: %v", err)
	}

	// Generate client handshake
	var serverPublic [32]byte
	curve25519.ScalarBaseMult(&serverPublic, &serverNtor.Private)

	clientHandshake, clientPrivate, err := crypto.NtorClientHandshake(serverIdentity, serverPublic[:])
	if err != nil {
		t.Fatalf("Failed to generate client handshake: %v", err)
	}

	// Build RENDEZVOUS1 cell
	circuitID := uint32(12345)
	streamID := uint16(0)

	rendezvous1, keyMaterial, err := BuildRendezvous1Cell(
		rendezvousCookie,
		clientHandshake,
		serverNtor.Private[:],
		serverIdentity,
		circuitID,
		streamID,
	)
	if err != nil {
		t.Fatalf("Failed to build RENDEZVOUS1 cell: %v", err)
	}

	// Validate cell structure
	if rendezvous1.Command != cell.RelayRendezvous1 {
		t.Errorf("Wrong cell command: got %d, want %d", rendezvous1.Command, cell.RelayRendezvous1)
	}

	if rendezvous1.StreamID != streamID {
		t.Errorf("Wrong stream ID: got %d, want %d", rendezvous1.StreamID, streamID)
	}

	// Validate payload: COOKIE (20) || HANDSHAKE (64)
	if len(rendezvous1.Data) != 84 {
		t.Fatalf("Invalid payload length: got %d, want 84", len(rendezvous1.Data))
	}

	// Check cookie matches
	if !bytes.Equal(rendezvous1.Data[0:20], rendezvousCookie) {
		t.Error("Rendezvous cookie mismatch in cell payload")
	}

	// Check handshake response format: Y (32) || AUTH (32)
	handshakeResponse := rendezvous1.Data[20:84]
	if len(handshakeResponse) != 64 {
		t.Errorf("Invalid handshake response length: %d", len(handshakeResponse))
	}

	// Validate key material
	if len(keyMaterial) != 72 {
		t.Errorf("Invalid key material length: got %d, want 72", len(keyMaterial))
	}

	// Verify client can process the handshake response
	clientKeyMaterial, err := crypto.NtorProcessResponse(handshakeResponse, clientPrivate, serverPublic[:], serverIdentity)
	if err != nil {
		t.Fatalf("Client failed to process handshake response: %v", err)
	}

	// Verify key material matches
	if !bytes.Equal(clientKeyMaterial, keyMaterial) {
		t.Error("Client and server key material mismatch")
	}
}

// TestBuildRendezvous1CellInvalidInput tests error handling
func TestBuildRendezvous1CellInvalidInput(t *testing.T) {
	tests := []struct {
		name              string
		cookie            []byte
		clientHandshake   []byte
		serverNtorKey     []byte
		serverIdentity    []byte
		expectedErrPrefix string
	}{
		{
			name:              "Invalid cookie length",
			cookie:            make([]byte, 10),
			clientHandshake:   make([]byte, 84),
			serverNtorKey:     make([]byte, 32),
			serverIdentity:    make([]byte, 32),
			expectedErrPrefix: "invalid rendezvous cookie length",
		},
		{
			name:              "Invalid client handshake length",
			cookie:            make([]byte, 20),
			clientHandshake:   make([]byte, 50),
			serverNtorKey:     make([]byte, 32),
			serverIdentity:    make([]byte, 32),
			expectedErrPrefix: "invalid client handshake length",
		},
		{
			name:              "Invalid server ntor key length",
			cookie:            make([]byte, 20),
			clientHandshake:   make([]byte, 84),
			serverNtorKey:     make([]byte, 16),
			serverIdentity:    make([]byte, 32),
			expectedErrPrefix: "invalid server ntor key length",
		},
		{
			name:              "Invalid server identity length",
			cookie:            make([]byte, 20),
			clientHandshake:   make([]byte, 84),
			serverNtorKey:     make([]byte, 32),
			serverIdentity:    make([]byte, 16),
			expectedErrPrefix: "invalid server identity length",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := BuildRendezvous1Cell(tt.cookie, tt.clientHandshake, tt.serverNtorKey, tt.serverIdentity, 1, 0)
			if err == nil {
				t.Error("Expected error, got nil")
			}
			if !bytes.Contains([]byte(err.Error()), []byte(tt.expectedErrPrefix)) {
				t.Errorf("Expected error containing %q, got %q", tt.expectedErrPrefix, err.Error())
			}
		})
	}
}

// TestSendRendezvous1V2 tests sending RENDEZVOUS1 on a circuit
func TestSendRendezvous1V2(t *testing.T) {
	// Generate server keys
	serverNtor, err := crypto.GenerateNtorKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate server ntor key: %v", err)
	}

	serverIdentity := make([]byte, 32)
	if _, err := rand.Read(serverIdentity); err != nil {
		t.Fatalf("Failed to generate server identity: %v", err)
	}

	// Generate rendezvous cookie
	rendezvousCookie := make([]byte, 20)
	if _, err := rand.Read(rendezvousCookie); err != nil {
		t.Fatalf("Failed to generate rendezvous cookie: %v", err)
	}

	// Generate client handshake
	var serverPublic [32]byte
	curve25519.ScalarBaseMult(&serverPublic, &serverNtor.Private)

	clientHandshake, _, err := crypto.NtorClientHandshake(serverIdentity, serverPublic[:])
	if err != nil {
		t.Fatalf("Failed to generate client handshake: %v", err)
	}

	// Create mock circuit
	mockCircuit := &MockCircuit{
		id:        uint32(9999),
		sentCells: make([]*cell.RelayCell, 0),
	}

	// Send RENDEZVOUS1
	keyMaterial, err := SendRendezvous1(
		mockCircuit,
		mockCircuit.id,
		rendezvousCookie,
		clientHandshake,
		serverNtor.Private[:],
		serverIdentity,
	)
	if err != nil {
		t.Fatalf("Failed to send RENDEZVOUS1: %v", err)
	}

	// Verify key material
	if len(keyMaterial) != 72 {
		t.Errorf("Invalid key material length: got %d, want 72", len(keyMaterial))
	}

	// Verify cell was sent
	if len(mockCircuit.sentCells) != 1 {
		t.Fatalf("Expected 1 sent cell, got %d", len(mockCircuit.sentCells))
	}

	sentCell := mockCircuit.sentCells[0]
	if sentCell.Command != cell.RelayRendezvous1 {
		t.Errorf("Wrong cell command: got %d, want %d", sentCell.Command, cell.RelayRendezvous1)
	}

	// Verify payload contains cookie
	if !bytes.Equal(sentCell.Data[0:20], rendezvousCookie) {
		t.Error("Cookie mismatch in sent cell")
	}
}

// TestSendRendezvous1NilCircuit tests error handling with nil circuit
func TestSendRendezvous1NilCircuit(t *testing.T) {
	_, err := SendRendezvous1(nil, 1, make([]byte, 20), make([]byte, 84), make([]byte, 32), make([]byte, 32))
	if err == nil {
		t.Error("Expected error for nil circuit, got nil")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("circuit is nil")) {
		t.Errorf("Expected 'circuit is nil' error, got %q", err.Error())
	}
}

// TestSendRendezvous1CircuitError tests error handling when circuit send fails
func TestSendRendezvous1CircuitError(t *testing.T) {
	// Generate minimal valid inputs
	serverNtor, err := crypto.GenerateNtorKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate server ntor key: %v", err)
	}

	serverIdentity := make([]byte, 32)
	if _, err := rand.Read(serverIdentity); err != nil {
		t.Fatalf("Failed to generate server identity: %v", err)
	}

	rendezvousCookie := make([]byte, 20)
	if _, err := rand.Read(rendezvousCookie); err != nil {
		t.Fatalf("Failed to generate rendezvous cookie: %v", err)
	}

	var serverPublic [32]byte
	curve25519.ScalarBaseMult(&serverPublic, &serverNtor.Private)

	clientHandshake, _, err := crypto.NtorClientHandshake(serverIdentity, serverPublic[:])
	if err != nil {
		t.Fatalf("Failed to generate client handshake: %v", err)
	}

	// Create mock circuit that returns error on send
	mockCircuit := &MockCircuit{
		id:        uint32(1234),
		sendError: errors.New("circuit send failed"),
	}

	// Attempt to send
	_, err = SendRendezvous1(mockCircuit, mockCircuit.id, rendezvousCookie, clientHandshake, serverNtor.Private[:], serverIdentity)
	if err == nil {
		t.Error("Expected error when circuit send fails, got nil")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("failed to send RENDEZVOUS1 cell")) {
		t.Errorf("Expected send error, got %q", err.Error())
	}
}

// TestRendezvous1EndToEnd tests complete workflow
func TestRendezvous1EndToEnd(t *testing.T) {
	// Simulate complete rendezvous establishment:
	// 1. Client generates handshake
	// 2. Server builds RENDEZVOUS1
	// 3. Client processes response
	// 4. Both derive same keys

	// Setup: Generate server keys
	serverNtor, err := crypto.GenerateNtorKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate server ntor key: %v", err)
	}

	serverIdentity := make([]byte, 32)
	if _, err := rand.Read(serverIdentity); err != nil {
		t.Fatalf("Failed to generate server identity: %v", err)
	}

	var serverPublic [32]byte
	curve25519.ScalarBaseMult(&serverPublic, &serverNtor.Private)

	rendezvousCookie := make([]byte, 20)
	if _, err := rand.Read(rendezvousCookie); err != nil {
		t.Fatalf("Failed to generate rendezvous cookie: %v", err)
	}

	// Step 1: Client generates handshake (in INTRODUCE2)
	clientHandshake, clientPrivate, err := crypto.NtorClientHandshake(serverIdentity, serverPublic[:])
	if err != nil {
		t.Fatalf("Client handshake generation failed: %v", err)
	}

	// Step 2: Server builds RENDEZVOUS1
	rendezvous1Cell, serverKeyMaterial, err := BuildRendezvous1Cell(
		rendezvousCookie,
		clientHandshake,
		serverNtor.Private[:],
		serverIdentity,
		12345,
		0,
	)
	if err != nil {
		t.Fatalf("Server RENDEZVOUS1 build failed: %v", err)
	}

	// Step 3: Client receives and processes RENDEZVOUS1
	// Extract handshake response from cell: skip cookie (20 bytes)
	handshakeResponse := rendezvous1Cell.Data[20:84]

	clientKeyMaterial, err := crypto.NtorProcessResponse(handshakeResponse, clientPrivate, serverPublic[:], serverIdentity)
	if err != nil {
		t.Fatalf("Client failed to process RENDEZVOUS1 response: %v", err)
	}

	// Step 4: Verify both have same key material
	if !bytes.Equal(clientKeyMaterial, serverKeyMaterial) {
		t.Error("Key material mismatch between client and server!")
		t.Logf("Client key material: %x", clientKeyMaterial[:16])
		t.Logf("Server key material: %x", serverKeyMaterial[:16])
	}

	// Verify cookie matches
	cookieInCell := rendezvous1Cell.Data[0:20]
	if !bytes.Equal(cookieInCell, rendezvousCookie) {
		t.Error("Rendezvous cookie mismatch in RENDEZVOUS1 cell")
	}
}

// TestRendezvous1KeyMaterialFormat tests key material structure
func TestRendezvous1KeyMaterialFormat(t *testing.T) {
	// Generate server keys
	serverNtor, err := crypto.GenerateNtorKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate server ntor key: %v", err)
	}

	serverIdentity := make([]byte, 32)
	if _, err := rand.Read(serverIdentity); err != nil {
		t.Fatalf("Failed to generate server identity: %v", err)
	}

	var serverPublic [32]byte
	curve25519.ScalarBaseMult(&serverPublic, &serverNtor.Private)

	rendezvousCookie := make([]byte, 20)
	if _, err := rand.Read(rendezvousCookie); err != nil {
		t.Fatalf("Failed to generate rendezvous cookie: %v", err)
	}

	clientHandshake, _, err := crypto.NtorClientHandshake(serverIdentity, serverPublic[:])
	if err != nil {
		t.Fatalf("Failed to generate client handshake: %v", err)
	}

	// Build RENDEZVOUS1
	_, keyMaterial, err := BuildRendezvous1Cell(
		rendezvousCookie,
		clientHandshake,
		serverNtor.Private[:],
		serverIdentity,
		1,
		0,
	)
	if err != nil {
		t.Fatalf("Failed to build RENDEZVOUS1: %v", err)
	}

	// Verify key material structure (72 bytes total)
	if len(keyMaterial) != 72 {
		t.Fatalf("Invalid key material length: %d", len(keyMaterial))
	}

	// Extract key components per tor-spec.txt
	Df := keyMaterial[0:20]   // Forward digest
	Db := keyMaterial[20:40]  // Backward digest
	Kf := keyMaterial[40:56]  // Forward cipher key
	Kb := keyMaterial[56:72]  // Backward cipher key

	// Verify all components are non-zero
	components := []struct {
		name string
		data []byte
	}{
		{"Df (forward digest)", Df},
		{"Db (backward digest)", Db},
		{"Kf (forward cipher)", Kf},
		{"Kb (backward cipher)", Kb},
	}

	for _, comp := range components {
		allZero := true
		for _, b := range comp.data {
			if b != 0 {
				allZero = false
				break
			}
		}
		if allZero {
			t.Errorf("Key component %s is all zeros", comp.name)
		}
	}
}
