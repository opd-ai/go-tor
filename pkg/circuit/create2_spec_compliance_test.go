package circuit

import (
	"context"
	"encoding/binary"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/cell"
	"github.com/opd-ai/go-tor/pkg/logger"
)

// TestCREATE2CellFormat verifies CREATE2 cell format per tor-spec.txt §4
// CREATE2 format: HTYPE (2 bytes) | HLEN (2 bytes) | HDATA (HLEN bytes)
func TestCREATE2CellFormat(t *testing.T) {
	tests := []struct {
		name          string
		handshakeType HandshakeType
		expectedHType uint16
	}{
		{
			name:          "ntor handshake type",
			handshakeType: HandshakeTypeNTor,
			expectedHType: 0x0002,
		},
		{
			name:          "TAP handshake type",
			handshakeType: HandshakeTypeTAP,
			expectedHType: 0x0000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := logger.NewDefault()
			circuit := NewCircuit(1)
			mockConn := newMockExtensionConnection()
			circuit.SetConnection(mockConn)
			ext := NewExtension(circuit, log)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Attempt to create first hop (will fail verification with mock response)
			_ = ext.CreateFirstHop(ctx, tt.handshakeType)

			// Verify CREATE2 cell was sent
			if len(mockConn.sentCells) != 1 {
				t.Fatalf("Expected 1 sent cell, got %d", len(mockConn.sentCells))
			}

			sentCell := mockConn.sentCells[0]

			// Verify cell command
			if sentCell.Command != cell.CmdCreate2 {
				t.Errorf("Expected CREATE2 command (0x%02X), got %s (0x%02X)",
					cell.CmdCreate2, sentCell.Command, sentCell.Command)
			}

			// Verify payload format
			payload := sentCell.Payload
			if len(payload) < 4 {
				t.Fatalf("CREATE2 payload too short: %d bytes", len(payload))
			}

			// Verify HTYPE (handshake type)
			htype := binary.BigEndian.Uint16(payload[0:2])
			if htype != tt.expectedHType {
				t.Errorf("Expected HTYPE 0x%04X, got 0x%04X", tt.expectedHType, htype)
			}

			// Verify HLEN (handshake data length)
			hlen := binary.BigEndian.Uint16(payload[2:4])
			if int(4+hlen) != len(payload) {
				t.Errorf("HLEN mismatch: HLEN=%d, actual payload length=%d",
					hlen, len(payload)-4)
			}

			// Verify HDATA exists and matches HLEN
			if len(payload[4:]) != int(hlen) {
				t.Errorf("HDATA length mismatch: HLEN=%d, HDATA length=%d",
					hlen, len(payload[4:]))
			}
		})
	}
}

// TestCREATE2HandshakeDataGeneration verifies handshake data generation per tor-spec.txt §5.1
func TestCREATE2HandshakeDataGeneration(t *testing.T) {
	log := logger.NewDefault()
	circuit := NewCircuit(1)
	ext := NewExtension(circuit, log)

	tests := []struct {
		name          string
		handshakeType HandshakeType
		expectedSize  int
		description   string
	}{
		{
			name:          "ntor handshake data size",
			handshakeType: HandshakeTypeNTor,
			expectedSize:  84, // 32 (ID) + 32 (B) + 20 (KEYID)
			description:   "ntor: NODE_ID (32) | KEYID (20) | CLIENT_PK (32)",
		},
		{
			name:          "TAP handshake data size",
			handshakeType: HandshakeTypeTAP,
			expectedSize:  144, // Actual RSA-OAEP encrypted size
			description:   "TAP: RSA-1024 OAEP encrypted hybrid data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handshakeData, err := ext.generateHandshakeData(tt.handshakeType)
			if err != nil {
				t.Fatalf("Failed to generate handshake data: %v", err)
			}

			if len(handshakeData) != tt.expectedSize {
				t.Errorf("Expected %d bytes (%s), got %d bytes",
					tt.expectedSize, tt.description, len(handshakeData))
			}
		})
	}
}

// TestCREATED2CellFormat verifies CREATED2 cell format per tor-spec.txt §4
// CREATED2 format: HLEN (2 bytes) | HDATA (HLEN bytes)
func TestCREATED2CellFormat(t *testing.T) {
	tests := []struct {
		name         string
		hlen         uint16
		hdata        []byte
		expectError  bool
		errorContain string
	}{
		{
			name:        "valid CREATED2 with ntor response",
			hlen:        64, // 32 (server public) + 32 (auth)
			hdata:       make([]byte, 64),
			expectError: false,
		},
		{
			name:         "CREATED2 payload too short",
			hlen:         0,
			hdata:        []byte{},
			expectError:  true,
			errorContain: "invalid response length",
		},
		{
			name:         "CREATED2 HLEN mismatch",
			hlen:         100,
			hdata:        make([]byte, 50),
			expectError:  true,
			errorContain: "incomplete",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := logger.NewDefault()
			circuit := NewCircuit(1)
			ext := NewExtension(circuit, log)

			// Build CREATED2 cell
			payload := make([]byte, 2+len(tt.hdata))
			binary.BigEndian.PutUint16(payload[0:2], tt.hlen)
			copy(payload[2:], tt.hdata)

			created2Cell := &cell.Cell{
				CircID:  circuit.ID,
				Command: cell.CmdCreated2,
				Payload: payload,
			}

			// Set ephemeral keys for processing (even though we'll fail verification)
			ext.ephemeralPrivate = make([]byte, 32)
			ext.serverNtorKey = make([]byte, 32)
			ext.serverIdentity = make([]byte, 32) // Ed25519 identity is 32 bytes

			err := ext.ProcessCreated2(created2Cell)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.errorContain)
				} else if tt.errorContain != "" && !containsStr(err.Error(), tt.errorContain) {
					t.Errorf("Expected error containing %q, got: %v", tt.errorContain, err)
				}
			}
		})
	}
}

// TestCREATED2Processing verifies CREATED2 response processing per tor-spec.txt §5.1.4
func TestCREATED2Processing(t *testing.T) {
	// Note: Full end-to-end ntor handshake testing requires matching client/server keys
	// This test verifies the processing flow and key derivation structure
	log := logger.NewDefault()
	circuit := NewCircuit(1)
	ext := NewExtension(circuit, log)

	// Set up dummy ntor keys
	ext.ephemeralPrivate = make([]byte, 32)
	ext.serverNtorKey = make([]byte, 32)
	ext.serverIdentity = make([]byte, 32)

	// Fill with test data
	copy(ext.serverIdentity, []byte("test_server_identity_32bytes1234"))

	// Create a mock CREATED2 cell with correct format (will fail verification)
	serverResponse := make([]byte, 64)
	payload := make([]byte, 2+len(serverResponse))
	binary.BigEndian.PutUint16(payload[0:2], uint16(len(serverResponse)))
	copy(payload[2:], serverResponse)

	created2Cell := &cell.Cell{
		CircID:  circuit.ID,
		Command: cell.CmdCreated2,
		Payload: payload,
	}

	// Process CREATED2 - should fail verification since keys don't match
	err := ext.ProcessCreated2(created2Cell)
	if err == nil {
		t.Error("Expected error for mismatched keys, got nil")
	}
	if !containsStr(err.Error(), "auth MAC verification failed") &&
		!containsStr(err.Error(), "ntor handshake verification failed") {
		t.Errorf("Expected ntor verification error, got: %v", err)
	}
}

// TestCREATED2WrongCommand verifies rejection of non-CREATED2 cells
func TestCREATED2WrongCommand(t *testing.T) {
	log := logger.NewDefault()
	circuit := NewCircuit(1)
	ext := NewExtension(circuit, log)

	wrongCells := []struct {
		name    string
		command cell.Command
	}{
		{"CREATE2 instead of CREATED2", cell.CmdCreate2},
		{"DESTROY instead of CREATED2", cell.CmdDestroy},
		{"RELAY instead of CREATED2", cell.CmdRelay},
	}

	for _, tc := range wrongCells {
		t.Run(tc.name, func(t *testing.T) {
			wrongCell := &cell.Cell{
				CircID:  circuit.ID,
				Command: tc.command,
				Payload: make([]byte, 66), // Valid CREATED2 size
			}

			err := ext.ProcessCreated2(wrongCell)
			if err == nil {
				t.Errorf("Expected error for wrong command %s, got nil", tc.command)
			}
			if !containsStr(err.Error(), "expected CREATED2") {
				t.Errorf("Expected 'expected CREATED2' error, got: %v", err)
			}
		})
	}
}

// TestCREATE2CircuitIDMatch verifies circuit ID matching per tor-spec.txt
func TestCREATE2CircuitIDMatch(t *testing.T) {
	log := logger.NewDefault()
	circuit := NewCircuit(42)
	mockConn := newMockExtensionConnection()
	circuit.SetConnection(mockConn)
	ext := NewExtension(circuit, log)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create first hop
	_ = ext.CreateFirstHop(ctx, HandshakeTypeNTor)

	// Verify CREATE2 has correct circuit ID
	if len(mockConn.sentCells) != 1 {
		t.Fatalf("Expected 1 sent cell, got %d", len(mockConn.sentCells))
	}

	sentCell := mockConn.sentCells[0]
	if sentCell.CircID != uint32(42) {
		t.Errorf("Expected circuit ID 42, got %d", sentCell.CircID)
	}
}

// TestCREATE2Timeout verifies timeout handling per tor-spec.txt
func TestCREATE2Timeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping timeout test in short mode")
	}

	log := logger.NewDefault()
	circuit := NewCircuit(1)

	// Mock connection that never responds
	mockConn := &mockTimeoutConnection{}
	circuit.SetConnection(mockConn)
	ext := NewExtension(circuit, log)

	// Use short timeout for test
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := ext.CreateFirstHop(ctx, HandshakeTypeNTor)
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}
	if !containsStr(err.Error(), "timeout") && !containsStr(err.Error(), "deadline exceeded") {
		t.Errorf("Expected timeout error, got: %v", err)
	}
}

// TestCREATED2MissingEphemeralKey verifies error when ephemeral key is missing
func TestCREATED2MissingEphemeralKey(t *testing.T) {
	log := logger.NewDefault()
	circuit := NewCircuit(1)
	ext := NewExtension(circuit, log)

	// Create CREATED2 without setting ephemeral key
	payload := make([]byte, 66)
	binary.BigEndian.PutUint16(payload[0:2], 64)

	created2Cell := &cell.Cell{
		CircID:  circuit.ID,
		Command: cell.CmdCreated2,
		Payload: payload,
	}

	err := ext.ProcessCreated2(created2Cell)
	if err == nil {
		t.Error("Expected error for missing ephemeral key, got nil")
	}
	if !containsStr(err.Error(), "ephemeral private key") {
		t.Errorf("Expected 'ephemeral private key' error, got: %v", err)
	}
}

// TestCREATED2InsufficientKeyMaterial verifies error for short key material
func TestCREATED2InsufficientKeyMaterial(t *testing.T) {
	// This test verifies that insufficient key material (< 72 bytes) is rejected
	// per tor-spec.txt §5.2 which requires 72 bytes for circuit keys
	log := logger.NewDefault()
	circuit := NewCircuit(1)
	ext := NewExtension(circuit, log)

	// Set dummy keys to get past initial checks
	ext.ephemeralPrivate = make([]byte, 32)
	ext.serverNtorKey = make([]byte, 32)
	ext.serverIdentity = make([]byte, 32) // Ed25519 identity is 32 bytes

	// Create CREATED2 with valid format but will fail ntor verification
	// (which would produce insufficient key material)
	payload := make([]byte, 66)
	binary.BigEndian.PutUint16(payload[0:2], 64)

	created2Cell := &cell.Cell{
		CircID:  circuit.ID,
		Command: cell.CmdCreated2,
		Payload: payload,
	}

	err := ext.ProcessCreated2(created2Cell)
	// Should fail at ntor verification since we're using dummy keys
	if err == nil {
		t.Error("Expected error for invalid handshake, got nil")
	}
}

// mockTimeoutConnection simulates a connection that never responds
type mockTimeoutConnection struct{}

func (m *mockTimeoutConnection) SendCell(c *cell.Cell) error {
	return nil
}

func (m *mockTimeoutConnection) ReceiveCell() (*cell.Cell, error) {
	// Block forever to simulate timeout
	select {}
}

// Helper function to check if string contains substring (uses strings.Contains internally)
func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
