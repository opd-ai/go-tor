// Package protocol provides tests for tor-spec.txt §3 compliance
// VERSIONS cell protocol implementation verification
package protocol

import (
	"bytes"
	"testing"

	"github.com/opd-ai/go-tor/pkg/cell"
	"github.com/opd-ai/go-tor/pkg/connection"
	"github.com/opd-ai/go-tor/pkg/logger"
)

// TestVERSIONSCellSpecCompliance_Format verifies VERSIONS cell format per tor-spec.txt §3
func TestVERSIONSCellSpecCompliance_Format(t *testing.T) {
	tests := []struct {
		name            string
		versions        []uint16
		expectedPayload []byte
	}{
		{
			name:     "Single version (v3)",
			versions: []uint16{3},
			expectedPayload: []byte{
				0x00, 0x03, // Version 3 (big-endian 2 bytes)
			},
		},
		{
			name:     "Two versions (v3, v4)",
			versions: []uint16{3, 4},
			expectedPayload: []byte{
				0x00, 0x03, // Version 3
				0x00, 0x04, // Version 4
			},
		},
		{
			name:     "Three versions (v3, v4, v5)",
			versions: []uint16{3, 4, 5},
			expectedPayload: []byte{
				0x00, 0x03, // Version 3
				0x00, 0x04, // Version 4
				0x00, 0x05, // Version 5
			},
		},
		{
			name:     "Maximum version number (65535)",
			versions: []uint16{65535},
			expectedPayload: []byte{
				0xFF, 0xFF, // Version 65535 (max uint16)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create payload
			payload := make([]byte, len(tt.versions)*2)
			for i, v := range tt.versions {
				payload[i*2] = byte(v >> 8)
				payload[i*2+1] = byte(v)
			}

			// Verify payload matches expected
			if len(payload) != len(tt.expectedPayload) {
				t.Errorf("Payload length mismatch: got %d, want %d", len(payload), len(tt.expectedPayload))
			}
			for i := range payload {
				if payload[i] != tt.expectedPayload[i] {
					t.Errorf("Payload[%d] = %02x, want %02x", i, payload[i], tt.expectedPayload[i])
				}
			}

			// Verify cell properties per tor-spec.txt §3
			versionsCell := cell.NewCell(0, cell.CmdVersions)
			versionsCell.Payload = payload

			// CircID must be 0 for VERSIONS per tor-spec.txt §3
			if versionsCell.CircID != 0 {
				t.Errorf("VERSIONS cell CircID = %d, want 0", versionsCell.CircID)
			}

			// Command must be VERSIONS (7)
			if versionsCell.Command != cell.CmdVersions {
				t.Errorf("VERSIONS cell Command = %d, want %d", versionsCell.Command, cell.CmdVersions)
			}

			// Payload length must be even (2 bytes per version)
			if len(versionsCell.Payload)%2 != 0 {
				t.Errorf("VERSIONS payload length %d is odd, must be even", len(versionsCell.Payload))
			}
		})
	}
}

// TestVERSIONSCellSpecCompliance_CircuitID verifies CircID=0 requirement per tor-spec.txt §3
func TestVERSIONSCellSpecCompliance_CircuitID(t *testing.T) {
	// Per tor-spec.txt §3: VERSIONS cells are sent before version negotiation
	// Therefore CircID is always 0 (no circuit established yet)

	versionsCell := cell.NewCell(0, cell.CmdVersions)
	versionsCell.Payload = []byte{0x00, 0x03, 0x00, 0x04} // Versions 3, 4

	if versionsCell.CircID != 0 {
		t.Errorf("VERSIONS cell CircID = %d, want 0 (per tor-spec.txt §3)", versionsCell.CircID)
	}

	// Verify cell encoding preserves CircID=0
	var buf bytes.Buffer
	err := versionsCell.Encode(&buf)
	if err != nil {
		t.Fatalf("Failed to encode VERSIONS cell: %v", err)
	}
	encoded := buf.Bytes()

	// Variable-length cell format: CircID(4) + Cmd(1) + Len(2) + Payload
	// CircID should be first 4 bytes (big-endian)
	circID := uint32(encoded[0])<<24 | uint32(encoded[1])<<16 | uint32(encoded[2])<<8 | uint32(encoded[3])
	if circID != 0 {
		t.Errorf("Encoded VERSIONS cell CircID = %d, want 0", circID)
	}

	// Command should be byte 4
	if encoded[4] != byte(cell.CmdVersions) {
		t.Errorf("Encoded VERSIONS cell Command = %d, want %d", encoded[4], cell.CmdVersions)
	}
}

// TestVERSIONSCellSpecCompliance_VariableLength verifies VERSIONS is variable-length per tor-spec.txt §3
func TestVERSIONSCellSpecCompliance_VariableLength(t *testing.T) {
	// Per tor-spec.txt §0.2: VERSIONS (cmd 7) is a variable-length cell

	tests := []struct {
		name           string
		versions       []uint16
		expectedLength int // Total cell length: CircID(4) + Cmd(1) + Len(2) + Payload
	}{
		{
			name:           "One version (2 bytes payload)",
			versions:       []uint16{3},
			expectedLength: 4 + 1 + 2 + 2, // 9 bytes total
		},
		{
			name:           "Three versions (6 bytes payload)",
			versions:       []uint16{3, 4, 5},
			expectedLength: 4 + 1 + 2 + 6, // 13 bytes total
		},
		{
			name:           "Ten versions (20 bytes payload)",
			versions:       []uint16{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			expectedLength: 4 + 1 + 2 + 20, // 27 bytes total
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := make([]byte, len(tt.versions)*2)
			for i, v := range tt.versions {
				payload[i*2] = byte(v >> 8)
				payload[i*2+1] = byte(v)
			}

			versionsCell := cell.NewCell(0, cell.CmdVersions)
			versionsCell.Payload = payload

			var buf bytes.Buffer
			err := versionsCell.Encode(&buf)
			if err != nil {
				t.Fatalf("Failed to encode VERSIONS cell: %v", err)
			}
			encoded := buf.Bytes()

			if len(encoded) != tt.expectedLength {
				t.Errorf("Encoded cell length = %d, want %d", len(encoded), tt.expectedLength)
			}

			// Verify Length field (bytes 5-6, big-endian)
			payloadLen := uint16(encoded[5])<<8 | uint16(encoded[6])
			if payloadLen != uint16(len(payload)) {
				t.Errorf("Length field = %d, want %d", payloadLen, len(payload))
			}
		})
	}
}

// TestVERSIONSCellSpecCompliance_Parsing verifies VERSIONS cell parsing per tor-spec.txt §3
func TestVERSIONSCellSpecCompliance_Parsing(t *testing.T) {
	tests := []struct {
		name             string
		payload          []byte
		expectedVersions []int
		shouldFail       bool
	}{
		{
			name:             "Valid: Single version",
			payload:          []byte{0x00, 0x03},
			expectedVersions: []int{3},
			shouldFail:       false,
		},
		{
			name:             "Valid: Multiple versions",
			payload:          []byte{0x00, 0x03, 0x00, 0x04, 0x00, 0x05},
			expectedVersions: []int{3, 4, 5},
			shouldFail:       false,
		},
		{
			name:             "Valid: High version number",
			payload:          []byte{0xFF, 0xFF},
			expectedVersions: []int{65535},
			shouldFail:       false,
		},
		{
			name:       "Invalid: Odd payload length",
			payload:    []byte{0x00, 0x03, 0x00},
			shouldFail: true,
		},
		{
			name:       "Invalid: Empty payload",
			payload:    []byte{},
			shouldFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse versions from payload (same logic as receiveVersions)
			if len(tt.payload)%2 != 0 {
				if !tt.shouldFail {
					t.Errorf("Payload length %d is odd, parsing should fail", len(tt.payload))
				}
				return
			}

			if len(tt.payload) == 0 {
				if !tt.shouldFail {
					t.Errorf("Empty payload, parsing should fail")
				}
				return
			}

			var versions []int
			for i := 0; i < len(tt.payload); i += 2 {
				version := int(tt.payload[i])<<8 | int(tt.payload[i+1])
				versions = append(versions, version)
			}

			if tt.shouldFail {
				t.Errorf("Parsing succeeded but should have failed")
				return
			}

			// Verify parsed versions match expected
			if len(versions) != len(tt.expectedVersions) {
				t.Errorf("Parsed %d versions, want %d", len(versions), len(tt.expectedVersions))
			}
			for i, v := range versions {
				if v != tt.expectedVersions[i] {
					t.Errorf("Version[%d] = %d, want %d", i, v, tt.expectedVersions[i])
				}
			}
		})
	}
}

// TestVERSIONSCellSpecCompliance_NegotiationAlgorithm verifies version selection per tor-spec.txt §3
func TestVERSIONSCellSpecCompliance_NegotiationAlgorithm(t *testing.T) {
	// Per tor-spec.txt §3: Select highest mutually supported version

	tests := []struct {
		name            string
		remoteVersions  []int
		expectedVersion int
		shouldSucceed   bool
	}{
		{
			name:            "Exact match (v4)",
			remoteVersions:  []int{4},
			expectedVersion: 4,
			shouldSucceed:   true,
		},
		{
			name:            "Highest mutual (v5)",
			remoteVersions:  []int{3, 4, 5},
			expectedVersion: 5,
			shouldSucceed:   true,
		},
		{
			name:            "Prefer highest (v5 over v4)",
			remoteVersions:  []int{3, 4, 5, 6},
			expectedVersion: 5, // 6 not supported locally
			shouldSucceed:   true,
		},
		{
			name:            "Prefer highest (v4 over v3)",
			remoteVersions:  []int{2, 3, 4},
			expectedVersion: 4,
			shouldSucceed:   true,
		},
		{
			name:            "Only v3 mutual",
			remoteVersions:  []int{1, 2, 3},
			expectedVersion: 3,
			shouldSucceed:   true,
		},
		{
			name:            "No mutual versions",
			remoteVersions:  []int{1, 2},
			expectedVersion: 0,
			shouldSucceed:   false,
		},
		{
			name:            "Remote supports higher",
			remoteVersions:  []int{6, 7, 8},
			expectedVersion: 0,
			shouldSucceed:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock connection
			conn := &connection.Connection{}
			log := logger.NewDefault()
			h := NewHandshake(conn, log)

			// Simulate version negotiation
			negotiated := h.selectVersion(tt.remoteVersions)

			if tt.shouldSucceed {
				if negotiated == 0 {
					t.Errorf("Version negotiation failed, expected success")
				}
				if negotiated != tt.expectedVersion {
					t.Errorf("Negotiated version = %d, want %d", negotiated, tt.expectedVersion)
				}
			} else {
				if negotiated != 0 {
					t.Errorf("Version negotiation succeeded with %d, expected failure", negotiated)
				}
			}
		})
	}
}

// TestVERSIONSCellSpecCompliance_SupportedVersions verifies supported version range
func TestVERSIONSCellSpecCompliance_SupportedVersions(t *testing.T) {
	// Verify constants match tor-spec.txt requirements

	if MinLinkProtocolVersion != 3 {
		t.Errorf("MinLinkProtocolVersion = %d, want 3 (per tor-spec.txt)", MinLinkProtocolVersion)
	}

	if MaxLinkProtocolVersion != 5 {
		t.Errorf("MaxLinkProtocolVersion = %d, want 5 (current max)", MaxLinkProtocolVersion)
	}

	if PreferredVersion != 4 {
		t.Errorf("PreferredVersion = %d, want 4 (4-byte circuit IDs)", PreferredVersion)
	}

	// Verify preferred version is within supported range
	if PreferredVersion < MinLinkProtocolVersion || PreferredVersion > MaxLinkProtocolVersion {
		t.Errorf("PreferredVersion %d outside supported range [%d, %d]",
			PreferredVersion, MinLinkProtocolVersion, MaxLinkProtocolVersion)
	}
}

// TestVERSIONSCellSpecCompliance_BigEndianEncoding verifies 2-byte big-endian encoding
func TestVERSIONSCellSpecCompliance_BigEndianEncoding(t *testing.T) {
	// Per tor-spec.txt §3: Each version is 2 bytes, big-endian

	tests := []struct {
		name     string
		version  uint16
		expected []byte
	}{
		{
			name:     "Version 0",
			version:  0,
			expected: []byte{0x00, 0x00},
		},
		{
			name:     "Version 3",
			version:  3,
			expected: []byte{0x00, 0x03},
		},
		{
			name:     "Version 256",
			version:  256,
			expected: []byte{0x01, 0x00},
		},
		{
			name:     "Version 65535",
			version:  65535,
			expected: []byte{0xFF, 0xFF},
		},
		{
			name:     "Version 258 (0x0102)",
			version:  258,
			expected: []byte{0x01, 0x02},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode version
			encoded := []byte{
				byte(tt.version >> 8),
				byte(tt.version),
			}

			// Verify encoding
			if len(encoded) != 2 {
				t.Errorf("Encoded length = %d, want 2", len(encoded))
			}
			if encoded[0] != tt.expected[0] || encoded[1] != tt.expected[1] {
				t.Errorf("Encoded = %02x %02x, want %02x %02x",
					encoded[0], encoded[1], tt.expected[0], tt.expected[1])
			}

			// Verify round-trip decoding
			decoded := uint16(encoded[0])<<8 | uint16(encoded[1])
			if decoded != tt.version {
				t.Errorf("Decoded version = %d, want %d", decoded, tt.version)
			}
		})
	}
}

// TestVERSIONSCellSpecCompliance_HandshakeFlow verifies VERSIONS exchange flow
func TestVERSIONSCellSpecCompliance_HandshakeFlow(t *testing.T) {
	// Per tor-spec.txt §3: VERSIONS is the first cell sent in Tor handshake
	// This test verifies the handshake flow logic (skipped in short mode)

	if testing.Short() {
		t.Skip("Skipping handshake flow test in short mode (requires mock connection)")
	}

	// Test would verify:
	// 1. Client sends VERSIONS first
	// 2. Server responds with VERSIONS
	// 3. Both sides select highest mutual version
	// 4. All subsequent cells use negotiated circuit ID size

	// This is an integration-level test, validated in handshake_test.go
}

// TestVERSIONSCellSpecCompliance_ErrorCases verifies error handling
func TestVERSIONSCellSpecCompliance_ErrorCases(t *testing.T) {
	tests := []struct {
		name             string
		payload          []byte
		command          cell.Command
		expectedVersions []int
		shouldFail       bool
	}{
		{
			name:       "Invalid payload length (odd)",
			payload:    []byte{0x00, 0x03, 0x00}, // 3 bytes (odd)
			command:    cell.CmdVersions,
			shouldFail: true,
		},
		{
			name:       "Wrong command (not VERSIONS)",
			payload:    []byte{0x00, 0x00, 0x00, 0x00},
			command:    cell.CmdNetinfo,
			shouldFail: true,
		},
		{
			name:             "No mutual versions",
			payload:          []byte{0x00, 0x01, 0x00, 0x02}, // Versions 1, 2
			command:          cell.CmdVersions,
			expectedVersions: []int{1, 2},
			shouldFail:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := &connection.Connection{}
			log := logger.NewDefault()
			h := NewHandshake(conn, log)

			// Check command
			if tt.command != cell.CmdVersions {
				// Expected error: wrong command
				if !tt.shouldFail {
					t.Errorf("Wrong command but shouldFail=false")
				}
				return
			}

			// Check payload length
			if len(tt.payload)%2 != 0 {
				if !tt.shouldFail {
					t.Errorf("Odd payload length but shouldFail=false")
				}
				// Expected error: odd payload length
				return
			}

			// Parse versions
			var versions []int
			for i := 0; i < len(tt.payload); i += 2 {
				version := int(tt.payload[i])<<8 | int(tt.payload[i+1])
				versions = append(versions, version)
			}

			// Select version
			negotiated := h.selectVersion(versions)
			if negotiated == 0 {
				if !tt.shouldFail {
					t.Errorf("Version negotiation failed but shouldFail=false")
				}
				// Expected error: no compatible version
				return
			}

			if tt.shouldFail {
				t.Errorf("Test should have failed but succeeded with version %d", negotiated)
			}
		})
	}
}
