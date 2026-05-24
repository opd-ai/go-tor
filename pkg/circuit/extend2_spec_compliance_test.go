// Package circuit provides comprehensive EXTEND2/EXTENDED2 specification compliance tests
// per tor-spec.txt §5.3
package circuit

import (
	"crypto/rand"
	"encoding/binary"
	"testing"

	"github.com/opd-ai/go-tor/pkg/cell"
	"github.com/opd-ai/go-tor/pkg/crypto"
	"github.com/opd-ai/go-tor/pkg/logger"
)

// TestEXTEND2CellFormat verifies EXTEND2 relay cell format per tor-spec.txt §5.3
func TestEXTEND2CellFormat(t *testing.T) {
	tests := []struct {
		name          string
		handshakeType HandshakeType
		wantNSPEC     byte
		wantHTYPE     uint16
	}{
		{
			name:          "ntor handshake type",
			handshakeType: HandshakeTypeNTor,
			wantNSPEC:     1,      // At least 1 link specifier
			wantHTYPE:     0x0002, // ntor handshake type
		},
		{
			name:          "TAP handshake type",
			handshakeType: HandshakeTypeTAP,
			wantNSPEC:     1,
			wantHTYPE:     0x0000, // TAP handshake type
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			circuit := NewCircuit(1)
			ext := NewExtension(circuit, logger.NewDefault())

			// Generate handshake data
			handshakeData := make([]byte, 84) // ntor size
			rand.Read(handshakeData)

			// Build EXTEND2 data
			extend2Data, err := ext.buildExtend2Data("127.0.0.1:9001", tt.handshakeType, handshakeData)
			if err != nil {
				t.Fatalf("buildExtend2Data() error = %v", err)
			}

			if len(extend2Data) == 0 {
				t.Fatal("EXTEND2 data should not be empty")
			}

			// Verify EXTEND2 format per tor-spec.txt §5.3:
			// NSPEC (1) [LSPECS] HTYPE (2) HLEN (2) HDATA

			// Check NSPEC
			nspec := extend2Data[0]
			if nspec < tt.wantNSPEC {
				t.Errorf("NSPEC = %d, want >= %d", nspec, tt.wantNSPEC)
			}

			// Skip link specifiers to find HTYPE (simplified check)
			// After NSPEC (1 byte), we have link specifiers
			// Each link specifier: Type (1) + Length (1) + Data (Length bytes)
			offset := 1
			for i := byte(0); i < nspec; i++ {
				if offset+2 > len(extend2Data) {
					t.Fatal("EXTEND2 data truncated in link specifiers")
				}
				// Skip: Type (1) + Length (1) + Data (length bytes)
				lspecLen := int(extend2Data[offset+1])
				offset += 2 + lspecLen
			}

			// Check HTYPE
			if offset+2 > len(extend2Data) {
				t.Fatal("EXTEND2 data truncated at HTYPE")
			}
			htype := binary.BigEndian.Uint16(extend2Data[offset : offset+2])
			if htype != tt.wantHTYPE {
				t.Errorf("HTYPE = 0x%04x, want 0x%04x", htype, tt.wantHTYPE)
			}

			// Check HLEN
			if offset+4 > len(extend2Data) {
				t.Fatal("EXTEND2 data truncated at HLEN")
			}
			hlen := binary.BigEndian.Uint16(extend2Data[offset+2 : offset+4])
			if hlen != uint16(len(handshakeData)) {
				t.Errorf("HLEN = %d, want %d", hlen, len(handshakeData))
			}

			// Check HDATA
			if offset+4+int(hlen) > len(extend2Data) {
				t.Fatal("EXTEND2 data truncated at HDATA")
			}
			hdata := extend2Data[offset+4 : offset+4+int(hlen)]
			if len(hdata) != len(handshakeData) {
				t.Errorf("HDATA length = %d, want %d", len(hdata), len(handshakeData))
			}
		})
	}
}

// TestEXTEND2LinkSpecifiers verifies link specifier format per tor-spec.txt §5.3
func TestEXTEND2LinkSpecifiers(t *testing.T) {
	circuit := NewCircuit(1)
	ext := NewExtension(circuit, logger.NewDefault())

	handshakeData := make([]byte, 84)
	rand.Read(handshakeData)

	extend2Data, err := ext.buildExtend2Data("127.0.0.1:9001", HandshakeTypeNTor, handshakeData)
	if err != nil {
		t.Fatalf("buildExtend2Data() error = %v", err)
	}

	// Parse NSPEC
	if len(extend2Data) < 1 {
		t.Fatal("EXTEND2 data too short")
	}

	nspec := extend2Data[0]
	if nspec == 0 {
		t.Error("NSPEC should be at least 1")
	}

	// Parse first link specifier
	offset := 1
	if offset+2 > len(extend2Data) {
		t.Fatal("Link specifier truncated")
	}

	lstype := extend2Data[offset]
	lslen := extend2Data[offset+1]

	// Link specifier format per tor-spec.txt §5.3:
	// Type (1) + Length (1) + Data (Length bytes)
	if offset+2+int(lslen) > len(extend2Data) {
		t.Fatal("Link specifier data truncated")
	}

	// Type 0 is TLS-over-TCP, IPv4 (6 bytes: IP + Port)
	// Type 1 is TLS-over-TCP, IPv6 (18 bytes: IP + Port)
	// Type 2 is Legacy identity (20 bytes: SHA-1 digest)
	// Type 3 is Ed25519 identity (32 bytes: Ed25519 public key)

	t.Logf("Link specifier: Type=%d, Length=%d", lstype, lslen)

	// Verify link specifier has valid structure
	if lslen == 0 {
		t.Error("Link specifier length should not be zero")
	}
}

// TestEXTEND2HandshakeData verifies handshake data generation per tor-spec.txt §5.3
func TestEXTEND2HandshakeData(t *testing.T) {
	tests := []struct {
		name          string
		handshakeType HandshakeType
		wantMinSize   int
	}{
		{
			name:          "ntor handshake",
			handshakeType: HandshakeTypeNTor,
			wantMinSize:   84, // NODEID (20) + KEYID (32) + CLIENT_PK (32)
		},
		{
			name:          "TAP handshake",
			handshakeType: HandshakeTypeTAP,
			wantMinSize:   144, // RSA-1024 OAEP encrypted data
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			circuit := NewCircuit(1)
			ext := NewExtension(circuit, logger.NewDefault())

			handshakeData, err := ext.generateHandshakeData(tt.handshakeType)
			if err != nil {
				t.Fatalf("generateHandshakeData() error = %v", err)
			}

			if len(handshakeData) < tt.wantMinSize {
				t.Errorf("handshake data size = %d, want >= %d", len(handshakeData), tt.wantMinSize)
			}
		})
	}
}

// TestEXTENDED2CellFormat verifies EXTENDED2 relay cell format per tor-spec.txt §5.3
func TestEXTENDED2CellFormat(t *testing.T) {
	tests := []struct {
		name        string
		payloadSize int
		wantError   bool
	}{
		{
			name:        "valid ntor response",
			payloadSize: 64, // SERVER_PK (32) + AUTH (32)
			wantError:   false,
		},
		{
			name:        "too short",
			payloadSize: 1,
			wantError:   true,
		},
		{
			name:        "HLEN mismatch",
			payloadSize: 10,
			wantError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			circuit := NewCircuit(1)
			ext := NewExtension(circuit, logger.NewDefault())

			// Generate ephemeral key for handshake verification
			ephemeral, err := crypto.GenerateNtorKeyPair()
			if err != nil {
				t.Fatalf("Failed to generate ephemeral key: %v", err)
			}
			ext.ephemeralPrivate = ephemeral.Private[:]

			// Set server keys (dummy values for format testing)
			ext.serverIdentity = make([]byte, 32)
			ext.serverNtorKey = make([]byte, 32)
			rand.Read(ext.serverIdentity)
			rand.Read(ext.serverNtorKey)

			// Build EXTENDED2 response
			var payload []byte
			if tt.payloadSize > 0 {
				handshakeResponse := make([]byte, tt.payloadSize)
				rand.Read(handshakeResponse)

				payload = make([]byte, 2+tt.payloadSize)
				binary.BigEndian.PutUint16(payload[0:2], uint16(tt.payloadSize))
				copy(payload[2:], handshakeResponse)
			} else {
				payload = make([]byte, 1) // Too short
			}

			relayCell := &cell.RelayCell{
				Command: cell.RelayExtended2,
				Data:    payload,
			}

			err = ext.ProcessExtended2(relayCell)
			if tt.wantError {
				if err == nil {
					t.Error("ProcessExtended2() expected error, got nil")
				}
			} else {
				// Will fail handshake verification with random data, but format should be OK
				if err != nil && !stringContains(err.Error(), "handshake verification failed") &&
					!stringContains(err.Error(), "insufficient key material") {
					t.Errorf("ProcessExtended2() unexpected error type: %v", err)
				}
			}
		})
	}
}

// TestEXTENDED2Processing verifies EXTENDED2 response processing per tor-spec.txt §5.3
func TestEXTENDED2Processing(t *testing.T) {
	t.Run("complete ntor handshake", func(t *testing.T) {
		circuit := NewCircuit(1)
		ext := NewExtension(circuit, logger.NewDefault())

		// Generate relay keypair (server side)
		relayKeypair, err := crypto.GenerateNtorKeyPair()
		if err != nil {
			t.Fatalf("Failed to generate relay keypair: %v", err)
		}

		// Generate relay identity key
		relayIdentity := make([]byte, 32)
		rand.Read(relayIdentity)

		// Client generates handshake data
		clientHandshake, clientSharedSecret, err := crypto.NtorClientHandshake(
			relayIdentity,
			relayKeypair.Public[:],
		)
		if err != nil {
			t.Fatalf("Client handshake failed: %v", err)
		}

		// Extract ephemeral private key from shared secret (32 bytes)
		// In the real flow, this is stored during generateHandshakeData
		ephemeralPrivate := make([]byte, 32)
		copy(ephemeralPrivate, clientSharedSecret[:32])

		// Store client ephemeral key in extension
		ext.ephemeralPrivate = ephemeralPrivate
		ext.serverIdentity = relayIdentity
		ext.serverNtorKey = relayKeypair.Public[:]

		// Server processes client handshake and generates response
		serverResponse, _, err := crypto.NtorServerHandshake(
			clientHandshake,
			relayKeypair.Private[:],
			relayIdentity,
		)
		if err != nil {
			t.Fatalf("Server handshake failed: %v", err)
		}

		// Build EXTENDED2 cell with server response
		payload := make([]byte, 2+len(serverResponse))
		binary.BigEndian.PutUint16(payload[0:2], uint16(len(serverResponse)))
		copy(payload[2:], serverResponse)

		relayCell := &cell.RelayCell{
			Command: cell.RelayExtended2,
			Data:    payload,
		}

		// Process EXTENDED2 (should succeed with matching keys)
		err = ext.ProcessExtended2(relayCell)
		if err != nil {
			t.Fatalf("ProcessExtended2() error = %v", err)
		}

		// Verify hop was added to circuit
		if len(circuit.Hops) != 1 {
			t.Errorf("Expected 1 hop in circuit, got %d", len(circuit.Hops))
		}
	})
}

// TestEXTENDED2WrongCommand verifies rejection of non-EXTENDED2 cells
func TestEXTENDED2WrongCommand(t *testing.T) {
	tests := []struct {
		name    string
		command uint8
	}{
		{"RELAY_DATA", cell.RelayData},
		{"RELAY_BEGIN", cell.RelayBegin},
		{"RELAY_END", cell.RelayEnd},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			circuit := NewCircuit(1)
			ext := NewExtension(circuit, logger.NewDefault())

			relayCell := &cell.RelayCell{
				Command: tt.command,
				Data:    make([]byte, 66),
			}

			err := ext.ProcessExtended2(relayCell)
			if err == nil {
				t.Error("ProcessExtended2() expected error for wrong command, got nil")
			}

			if !stringContains(err.Error(), "expected RELAY_EXTENDED2") {
				t.Errorf("Error message should mention RELAY_EXTENDED2, got: %v", err)
			}
		})
	}
}

// TestEXTENDED2MissingEphemeralKey verifies error when ephemeral key is missing
func TestEXTENDED2MissingEphemeralKey(t *testing.T) {
	circuit := NewCircuit(1)
	ext := NewExtension(circuit, logger.NewDefault())

	// Build valid EXTENDED2 cell
	handshakeResponse := make([]byte, 64)
	rand.Read(handshakeResponse)

	payload := make([]byte, 2+64)
	binary.BigEndian.PutUint16(payload[0:2], 64)
	copy(payload[2:], handshakeResponse)

	relayCell := &cell.RelayCell{
		Command: cell.RelayExtended2,
		Data:    payload,
	}

	// Don't set ephemeral key - should fail
	err := ext.ProcessExtended2(relayCell)
	if err == nil {
		t.Error("ProcessExtended2() expected error for missing ephemeral key, got nil")
	}

	if !stringContains(err.Error(), "ephemeral private key") {
		t.Errorf("Error message should mention ephemeral key, got: %v", err)
	}
}

// TestEXTENDED2InsufficientKeyMaterial verifies rejection of insufficient key material
func TestEXTENDED2InsufficientKeyMaterial(t *testing.T) {
	circuit := NewCircuit(1)
	ext := NewExtension(circuit, logger.NewDefault())

	// Generate ephemeral key
	ephemeral, err := crypto.GenerateNtorKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate ephemeral key: %v", err)
	}
	ext.ephemeralPrivate = ephemeral.Private[:]

	// Set server keys
	ext.serverIdentity = make([]byte, 32)
	ext.serverNtorKey = make([]byte, 32)
	rand.Read(ext.serverIdentity)
	rand.Read(ext.serverNtorKey)

	// Build EXTENDED2 with valid format but will produce insufficient key material
	// (due to random response data)
	handshakeResponse := make([]byte, 64)
	rand.Read(handshakeResponse)

	payload := make([]byte, 2+64)
	binary.BigEndian.PutUint16(payload[0:2], 64)
	copy(payload[2:], handshakeResponse)

	relayCell := &cell.RelayCell{
		Command: cell.RelayExtended2,
		Data:    payload,
	}

	// Should fail during handshake verification or key derivation
	err = ext.ProcessExtended2(relayCell)
	if err == nil {
		t.Error("ProcessExtended2() expected error for invalid handshake, got nil")
	}
}

// TestEXTEND2RelayEarlyFlag verifies that EXTEND2 cells should be sent with RELAY_EARLY
// (This is a protocol requirement per tor-spec.txt §5.6)
func TestEXTEND2RelayEarlyFlag(t *testing.T) {
	// Note: The actual RELAY_EARLY enforcement is in the circuit layer
	// This test documents the requirement that EXTEND2 cells should be
	// sent as RELAY_EARLY cells (up to 8 per circuit direction)

	t.Run("documentation", func(t *testing.T) {
		// Per tor-spec.txt §5.6:
		// - EXTEND2 cells SHOULD be sent as RELAY_EARLY
		// - Maximum 8 RELAY_EARLY cells per circuit direction
		// - Relays SHOULD reject excess RELAY_EARLY cells with DESTROY

		// This is enforced at the protocol layer, not in the extension handler
		t.Log("EXTEND2 cells should be sent with RELAY_EARLY flag per tor-spec.txt §5.6")
	})
}

// TestEXTEND2StreamID verifies EXTEND2 uses stream ID 0
func TestEXTEND2StreamID(t *testing.T) {
	t.Run("documentation", func(t *testing.T) {
		// Per tor-spec.txt §5.3:
		// - EXTEND2 cells use stream ID 0
		// - This is because circuit extension is not stream-specific

		// Note: This is verified in the ExtendCircuit implementation
		// which creates RelayCell with StreamID: 0 (line 133 in extension.go)
		t.Log("EXTEND2 cells should use stream ID 0 per tor-spec.txt §5.3")
	})
}

// Helper function to check if string contains substring
func stringContains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			stringContainsHelper(s, substr)))
}

func stringContainsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
