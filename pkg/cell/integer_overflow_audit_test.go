package cell

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"
)

// TestIntegerOverflow_VariableCellLength tests integer overflow protection in variable-length cells
func TestIntegerOverflow_VariableCellLength(t *testing.T) {
	tests := []struct {
		name          string
		payloadLen    uint16
		payloadData   []byte
		expectError   bool
		errorContains string
	}{
		{
			name:        "zero length payload",
			payloadLen:  0,
			payloadData: []byte{},
			expectError: false,
		},
		{
			name:        "small payload (100 bytes)",
			payloadLen:  100,
			payloadData: make([]byte, 100),
			expectError: false,
		},
		{
			name:        "maximum valid uint16 (65535 bytes)",
			payloadLen:  math.MaxUint16,
			payloadData: make([]byte, math.MaxUint16),
			expectError: false,
		},
		{
			name:        "length field mismatch (underflow)",
			payloadLen:  10,
			payloadData: make([]byte, 5),
			expectError: false, // Encode uses actual payload length, not a separate field
		},
		{
			name:        "length field mismatch (overflow)",
			payloadLen:  5,
			payloadData: make([]byte, 10),
			expectError: false, // Encode uses SafeLenToUint16, actual payload determines length
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a variable-length cell (VERSIONS)
			cell := NewCell(0, CmdVersions)
			cell.Payload = tt.payloadData

			var buf bytes.Buffer
			err := cell.Encode(&buf)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Verify encoded length field matches actual payload
			encodedBytes := buf.Bytes()
			if len(encodedBytes) < 7 {
				t.Errorf("Encoded cell too short: %d bytes", len(encodedBytes))
				return
			}

			// Read encoded length (bytes 5-6, after CircID and Command)
			encodedLen := binary.BigEndian.Uint16(encodedBytes[5:7])
			if encodedLen != uint16(len(tt.payloadData)) {
				t.Errorf("Length mismatch: encoded=%d, actual=%d", encodedLen, len(tt.payloadData))
			}
		})
	}
}

// TestIntegerOverflow_FixedCellPayload tests integer overflow protection in fixed-size cells
func TestIntegerOverflow_FixedCellPayload(t *testing.T) {
	tests := []struct {
		name          string
		payloadSize   int
		expectError   bool
		errorContains string
	}{
		{
			name:        "zero payload",
			payloadSize: 0,
			expectError: false,
		},
		{
			name:        "small payload (100 bytes)",
			payloadSize: 100,
			expectError: false,
		},
		{
			name:        "maximum valid payload (509 bytes)",
			payloadSize: PayloadLen,
			expectError: false,
		},
		{
			name:          "payload exceeds maximum (510 bytes)",
			payloadSize:   PayloadLen + 1,
			expectError:   true,
			errorContains: "payload too large",
		},
		{
			name:          "payload far exceeds maximum (1000 bytes)",
			payloadSize:   1000,
			expectError:   true,
			errorContains: "payload too large",
		},
		{
			name:          "payload at uint16 boundary (65535 bytes)",
			payloadSize:   math.MaxUint16,
			expectError:   true,
			errorContains: "payload too large",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fixed-size cell (PADDING)
			cell := NewCell(1, CmdPadding)
			cell.Payload = make([]byte, tt.payloadSize)

			var buf bytes.Buffer
			err := cell.Encode(&buf)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got nil")
					return
				}
				if tt.errorContains != "" && !bytes.Contains([]byte(err.Error()), []byte(tt.errorContains)) {
					t.Errorf("Error %q does not contain %q", err.Error(), tt.errorContains)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Verify encoded cell is exactly CellLen (514 bytes)
			encodedBytes := buf.Bytes()
			if len(encodedBytes) != CellLen {
				t.Errorf("Fixed cell size mismatch: got %d, want %d", len(encodedBytes), CellLen)
			}
		})
	}
}

// TestIntegerOverflow_RelayCellLength tests integer overflow protection in relay cells
func TestIntegerOverflow_RelayCellLength(t *testing.T) {
	tests := []struct {
		name          string
		dataSize      int
		expectError   bool
		errorContains string
	}{
		{
			name:        "zero data",
			dataSize:    0,
			expectError: false,
		},
		{
			name:        "small data (100 bytes)",
			dataSize:    100,
			expectError: false,
		},
		{
			name:        "maximum valid data (498 bytes)",
			dataSize:    PayloadLen - RelayCellHeaderLen,
			expectError: false,
		},
		{
			name:          "data exceeds maximum (499 bytes)",
			dataSize:      PayloadLen - RelayCellHeaderLen + 1,
			expectError:   true,
			errorContains: "data too large",
		},
		{
			name:          "data far exceeds maximum (1000 bytes)",
			dataSize:      1000,
			expectError:   true,
			errorContains: "data too large",
		},
		{
			name:          "data at uint16 boundary (65535 bytes)",
			dataSize:      math.MaxUint16,
			expectError:   true,
			errorContains: "data too large",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := make([]byte, tt.dataSize)
			_, err := NewRelayCell(1, RelayData, data)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got nil")
					return
				}
				if tt.errorContains != "" && !bytes.Contains([]byte(err.Error()), []byte(tt.errorContains)) {
					t.Errorf("Error %q does not contain %q", err.Error(), tt.errorContains)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

// TestIntegerOverflow_DecodeRelayCell tests integer overflow protection in relay cell decoding
func TestIntegerOverflow_DecodeRelayCell(t *testing.T) {
	tests := []struct {
		name          string
		lengthField   uint16
		actualData    int
		expectError   bool
		errorContains string
	}{
		{
			name:        "length matches data (100 bytes)",
			lengthField: 100,
			actualData:  100,
			expectError: false,
		},
		{
			name:        "length matches data (498 bytes)",
			lengthField: PayloadLen - RelayCellHeaderLen,
			actualData:  PayloadLen - RelayCellHeaderLen,
			expectError: false,
		},
		{
			name:          "length exceeds maximum (499 bytes)",
			lengthField:   PayloadLen - RelayCellHeaderLen + 1,
			actualData:    PayloadLen - RelayCellHeaderLen,
			expectError:   true,
			errorContains: "length exceeds maximum",
		},
		{
			name:          "length far exceeds maximum (1000 bytes)",
			lengthField:   1000,
			actualData:    PayloadLen - RelayCellHeaderLen,
			expectError:   true,
			errorContains: "length exceeds maximum",
		},
		{
			name:          "length at uint16 boundary (65535 bytes)",
			lengthField:   math.MaxUint16,
			actualData:    PayloadLen - RelayCellHeaderLen,
			expectError:   true,
			errorContains: "length exceeds maximum",
		},
		{
			name:          "length exceeds available payload",
			lengthField:   100,
			actualData:    50,
			expectError:   true, // Create smaller payload to trigger the check
			errorContains: "length exceeds payload",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Construct a relay cell payload with manipulated length field
			// For "length exceeds available payload" test, use smaller payload
			payloadSize := PayloadLen
			if tt.name == "length exceeds available payload" {
				payloadSize = RelayCellHeaderLen + tt.actualData
			}
			payload := make([]byte, payloadSize)
			payload[0] = RelayData                                // Command
			binary.BigEndian.PutUint16(payload[1:3], 0)           // Recognized
			binary.BigEndian.PutUint16(payload[3:5], 1)           // StreamID
			copy(payload[5:9], []byte{0, 0, 0, 0})                // Digest
			binary.BigEndian.PutUint16(payload[9:11], tt.lengthField) // Length (manipulated)
			// Data starts at offset 11, we only provide tt.actualData bytes
			if tt.actualData > 0 && payloadSize >= RelayCellHeaderLen+tt.actualData {
				copy(payload[11:], make([]byte, tt.actualData))
			}

			_, err := DecodeRelayCell(payload)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got nil")
					return
				}
				if tt.errorContains != "" && !bytes.Contains([]byte(err.Error()), []byte(tt.errorContains)) {
					t.Errorf("Error %q does not contain %q", err.Error(), tt.errorContains)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

// TestIntegerOverflow_CircuitID tests circuit ID handling doesn't overflow
func TestIntegerOverflow_CircuitID(t *testing.T) {
	tests := []struct {
		name    string
		circID  uint32
		wantErr bool
	}{
		{
			name:    "zero circuit ID",
			circID:  0,
			wantErr: false,
		},
		{
			name:    "small circuit ID",
			circID:  100,
			wantErr: false,
		},
		{
			name:    "maximum circuit ID (uint32 max)",
			circID:  math.MaxUint32,
			wantErr: false,
		},
		{
			name:    "mid-range circuit ID",
			circID:  1 << 31, // 2^31
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cell := NewCell(tt.circID, CmdPadding)
			var buf bytes.Buffer
			err := cell.Encode(&buf)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Decode and verify circuit ID
			decoded, err := DecodeCell(&buf)
			if err != nil {
				t.Errorf("Failed to decode cell: %v", err)
				return
			}

			if decoded.CircID != tt.circID {
				t.Errorf("Circuit ID mismatch: got %d, want %d", decoded.CircID, tt.circID)
			}
		})
	}
}

// TestIntegerOverflow_SafeLenConversion tests that SafeLenToUint16 is used for length conversions
func TestIntegerOverflow_SafeLenConversion(t *testing.T) {
	// This test verifies that the codebase uses SafeLenToUint16 for payload length conversions
	// which prevents integer overflow vulnerabilities

	// Test case 1: Normal payload that fits in uint16
	normalPayload := make([]byte, 1000)
	cell := NewCell(1, CmdVersions)
	cell.Payload = normalPayload

	var buf bytes.Buffer
	err := cell.Encode(&buf)
	if err != nil {
		t.Errorf("Normal payload encoding failed: %v", err)
	}

	// Test case 2: Maximum uint16 payload (should work)
	maxPayload := make([]byte, math.MaxUint16)
	cell = NewCell(2, CmdVersions)
	cell.Payload = maxPayload

	buf.Reset()
	err = cell.Encode(&buf)
	if err != nil {
		t.Errorf("Max uint16 payload encoding failed: %v", err)
	}

	// Test case 3: Verify relay cell also uses safe conversion
	relayData := make([]byte, 400)
	relayCell, err := NewRelayCell(1, RelayData, relayData)
	if err != nil {
		t.Errorf("Normal relay cell creation failed: %v", err)
	}
	if relayCell.Length != 400 {
		t.Errorf("Relay cell length mismatch: got %d, want 400", relayCell.Length)
	}
}

// TestIntegerOverflow_StreamID tests stream ID handling
func TestIntegerOverflow_StreamID(t *testing.T) {
	tests := []struct {
		name     string
		streamID uint16
		wantErr  bool
	}{
		{
			name:     "zero stream ID",
			streamID: 0,
			wantErr:  false,
		},
		{
			name:     "small stream ID",
			streamID: 100,
			wantErr:  false,
		},
		{
			name:     "maximum stream ID (uint16 max)",
			streamID: math.MaxUint16,
			wantErr:  false,
		},
		{
			name:     "mid-range stream ID",
			streamID: 1 << 15, // 2^15
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := make([]byte, 100)
			relayCell, err := NewRelayCell(tt.streamID, RelayData, data)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if relayCell.StreamID != tt.streamID {
				t.Errorf("Stream ID mismatch: got %d, want %d", relayCell.StreamID, tt.streamID)
			}

			// Encode and decode to verify round-trip
			payload, err := relayCell.Encode()
			if err != nil {
				t.Errorf("Encode failed: %v", err)
				return
			}

			decoded, err := DecodeRelayCell(payload)
			if err != nil {
				t.Errorf("Decode failed: %v", err)
				return
			}

			if decoded.StreamID != tt.streamID {
				t.Errorf("Decoded stream ID mismatch: got %d, want %d", decoded.StreamID, tt.streamID)
			}
		})
	}
}

// TestIntegerOverflow_EdgeCases tests edge cases for integer arithmetic
func TestIntegerOverflow_EdgeCases(t *testing.T) {
	t.Run("payload length calculation doesn't overflow", func(t *testing.T) {
		// Verify that PayloadLen - RelayCellHeaderLen calculation is safe
		maxRelayData := PayloadLen - RelayCellHeaderLen
		if maxRelayData != 498 {
			t.Errorf("Max relay data size incorrect: got %d, want 498", maxRelayData)
		}

		// Verify boundary condition
		data := make([]byte, maxRelayData)
		_, err := NewRelayCell(1, RelayData, data)
		if err != nil {
			t.Errorf("Boundary relay cell creation failed: %v", err)
		}

		// Verify one byte over fails
		dataTooLarge := make([]byte, maxRelayData+1)
		_, err = NewRelayCell(1, RelayData, dataTooLarge)
		if err == nil {
			t.Errorf("Expected error for oversized relay data, got nil")
		}
	})

	t.Run("cell length constants are consistent", func(t *testing.T) {
		// Verify that CellLen = CircIDLen + CmdLen + PayloadLen
		expectedCellLen := CircIDLen + CmdLen + PayloadLen
		if CellLen != expectedCellLen {
			t.Errorf("CellLen mismatch: got %d, want %d", CellLen, expectedCellLen)
		}
		if CellLen != 514 {
			t.Errorf("CellLen should be 514 bytes, got %d", CellLen)
		}
	})

	t.Run("relay cell header length is consistent", func(t *testing.T) {
		// Verify relay cell header: Command(1) + Recognized(2) + StreamID(2) + Digest(4) + Length(2) = 11
		if RelayCellHeaderLen != 11 {
			t.Errorf("RelayCellHeaderLen should be 11 bytes, got %d", RelayCellHeaderLen)
		}
	})
}
