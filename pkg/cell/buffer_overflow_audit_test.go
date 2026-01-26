// Package cell provides comprehensive buffer overflow auditing for cell parsing
package cell

import (
	"bytes"
	"encoding/binary"
	"io"
	"strings"
	"testing"
)

// TestBufferOverflow_FixedCellPayloadExact verifies fixed cells handle exact PayloadLen
func TestBufferOverflow_FixedCellPayloadExact(t *testing.T) {
	payload := make([]byte, PayloadLen)
	for i := range payload {
		payload[i] = byte(i % 256)
	}

	cell := &Cell{
		CircID:  12345,
		Command: CmdRelay,
		Payload: payload,
	}

	var buf bytes.Buffer
	if err := cell.Encode(&buf); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	if buf.Len() != CellLen {
		t.Errorf("Encoded cell length mismatch: got %d, want %d", buf.Len(), CellLen)
	}

	decoded, err := DecodeCell(&buf)
	if err != nil {
		t.Fatalf("DecodeCell failed: %v", err)
	}

	if decoded.CircID != cell.CircID {
		t.Errorf("CircID mismatch: got %d, want %d", decoded.CircID, cell.CircID)
	}
	if decoded.Command != cell.Command {
		t.Errorf("Command mismatch: got %v, want %v", decoded.Command, cell.Command)
	}
	if !bytes.Equal(decoded.Payload, payload) {
		t.Error("Payload mismatch after round-trip")
	}
}

// TestBufferOverflow_FixedCellPayloadOversized verifies oversized payloads are rejected
func TestBufferOverflow_FixedCellPayloadOversized(t *testing.T) {
	// Create payload larger than allowed
	payload := make([]byte, PayloadLen+100)

	cell := &Cell{
		CircID:  12345,
		Command: CmdRelay,
		Payload: payload,
	}

	var buf bytes.Buffer
	// Encoding should fail for oversized fixed cell payload
	err := cell.Encode(&buf)
	if err == nil {
		t.Fatal("Expected error for oversized fixed cell payload, got nil")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("Expected 'too large' error, got: %v", err)
	}
}

// TestBufferOverflow_VariableCellMaxSize verifies variable cells enforce uint16 max
func TestBufferOverflow_VariableCellMaxSize(t *testing.T) {
	// Maximum payload size for variable cells is 65535 (uint16 max)
	maxPayload := make([]byte, 65535)
	for i := range maxPayload {
		maxPayload[i] = byte(i % 256)
	}

	cell := &Cell{
		CircID:  12345,
		Command: CmdVPadding,
		Payload: maxPayload,
	}

	var buf bytes.Buffer
	if err := cell.Encode(&buf); err != nil {
		t.Fatalf("Encode max-size variable cell failed: %v", err)
	}

	// Expected size: CircIDLen(4) + CmdLen(1) + LengthField(2) + Payload(65535)
	expectedLen := CircIDLen + CmdLen + 2 + 65535
	if buf.Len() != expectedLen {
		t.Errorf("Max-size variable cell length: got %d, want %d", buf.Len(), expectedLen)
	}

	decoded, err := DecodeCell(&buf)
	if err != nil {
		t.Fatalf("DecodeCell max-size failed: %v", err)
	}

	if len(decoded.Payload) != 65535 {
		t.Errorf("Decoded payload length: got %d, want 65535", len(decoded.Payload))
	}
}

// TestBufferOverflow_VariableCellOversized verifies oversized variable cells are rejected
func TestBufferOverflow_VariableCellOversized(t *testing.T) {
	// Try to create a cell with payload larger than uint16 max (65536+)
	oversizedPayload := make([]byte, 70000)

	cell := &Cell{
		CircID:  12345,
		Command: CmdVPadding,
		Payload: oversizedPayload,
	}

	var buf bytes.Buffer
	err := cell.Encode(&buf)
	if err == nil {
		t.Fatal("Expected error for oversized variable cell, got nil")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("Expected 'too large' error, got: %v", err)
	}
}

// TestBufferOverflow_DecodeTruncatedCircID verifies truncated circuit ID is rejected
func TestBufferOverflow_DecodeTruncatedCircID(t *testing.T) {
	// Only 3 bytes instead of 4
	truncated := []byte{0x01, 0x02, 0x03}
	buf := bytes.NewReader(truncated)

	_, err := DecodeCell(buf)
	if err == nil {
		t.Fatal("Expected error for truncated circuit ID, got nil")
	}
}

// TestBufferOverflow_DecodeTruncatedCommand verifies truncated command is rejected
func TestBufferOverflow_DecodeTruncatedCommand(t *testing.T) {
	// CircID (4 bytes) but no command
	truncated := make([]byte, CircIDLen)
	binary.BigEndian.PutUint32(truncated, 12345)

	buf := bytes.NewReader(truncated)
	_, err := DecodeCell(buf)
	if err == nil {
		t.Fatal("Expected error for truncated command, got nil")
	}
}

// TestBufferOverflow_DecodeTruncatedVarLength verifies truncated variable length field
func TestBufferOverflow_DecodeTruncatedVarLength(t *testing.T) {
	// CircID + Command (VPADDING) but no length field
	truncated := make([]byte, CircIDLen+CmdLen)
	binary.BigEndian.PutUint32(truncated[0:4], 12345)
	truncated[4] = byte(CmdVPadding)

	buf := bytes.NewReader(truncated)
	_, err := DecodeCell(buf)
	if err == nil {
		t.Fatal("Expected error for truncated variable length field, got nil")
	}
}

// TestBufferOverflow_DecodeTruncatedFixedPayload verifies truncated fixed payload
func TestBufferOverflow_DecodeTruncatedFixedPayload(t *testing.T) {
	// CircID + Command + partial payload (100 bytes instead of 509)
	truncated := make([]byte, CircIDLen+CmdLen+100)
	binary.BigEndian.PutUint32(truncated[0:4], 12345)
	truncated[4] = byte(CmdRelay)

	buf := bytes.NewReader(truncated)
	_, err := DecodeCell(buf)
	if err == nil {
		t.Fatal("Expected error for truncated fixed payload, got nil")
	}
	if !strings.Contains(err.Error(), "failed to read") {
		t.Errorf("Expected read failure error, got: %v", err)
	}
}

// TestBufferOverflow_DecodeTruncatedVariablePayload verifies truncated variable payload
func TestBufferOverflow_DecodeTruncatedVariablePayload(t *testing.T) {
	// Build header claiming 1000-byte payload but provide only 100
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, uint32(12345))     // CircID
	binary.Write(buf, binary.BigEndian, byte(CmdVPadding)) // Command
	binary.Write(buf, binary.BigEndian, uint16(1000))      // Length field claims 1000
	buf.Write(make([]byte, 100))                           // Only 100 bytes provided

	_, err := DecodeCell(buf)
	if err == nil {
		t.Fatal("Expected error for truncated variable payload, got nil")
	}
}

// TestBufferOverflow_RelayPayloadOversized verifies relay cell data size limits
func TestBufferOverflow_RelayPayloadOversized(t *testing.T) {
	// Maximum relay data is PayloadLen - RelayCellHeaderLen = 498 bytes
	maxRelayData := PayloadLen - RelayCellHeaderLen
	oversizedData := make([]byte, maxRelayData+100)

	_, err := NewRelayCell(1, RelayData, oversizedData)
	if err == nil {
		t.Fatal("Expected error for oversized relay data, got nil")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("Expected 'too large' error, got: %v", err)
	}
}

// TestBufferOverflow_RelayEncodeOversized verifies Encode rejects oversized data
func TestBufferOverflow_RelayEncodeOversized(t *testing.T) {
	// Manually create relay cell with oversized data
	maxRelayData := PayloadLen - RelayCellHeaderLen
	rc := &RelayCell{
		Command:  RelayData,
		StreamID: 1,
		Length:   uint16(maxRelayData + 100),
		Data:     make([]byte, maxRelayData+100),
	}

	_, err := rc.Encode()
	if err == nil {
		t.Fatal("Expected error encoding oversized relay cell, got nil")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("Expected 'too large' error, got: %v", err)
	}
}

// TestBufferOverflow_RelayDecodeTruncated verifies relay decoding rejects short payloads
func TestBufferOverflow_RelayDecodeTruncated(t *testing.T) {
	// Payload shorter than RelayCellHeaderLen
	shortPayload := make([]byte, RelayCellHeaderLen-5)

	_, err := DecodeRelayCell(shortPayload)
	if err == nil {
		t.Fatal("Expected error for short relay payload, got nil")
	}
	if !strings.Contains(err.Error(), "too short") {
		t.Errorf("Expected 'too short' error, got: %v", err)
	}
}

// TestBufferOverflow_RelayLengthExceedsMax verifies length field validation
func TestBufferOverflow_RelayLengthExceedsMax(t *testing.T) {
	// Create payload with length field exceeding maximum
	payload := make([]byte, PayloadLen)
	payload[0] = RelayData                                     // Command
	binary.BigEndian.PutUint16(payload[1:3], 0)                // Recognized
	binary.BigEndian.PutUint16(payload[3:5], 1)                // StreamID
	binary.BigEndian.PutUint16(payload[9:11], PayloadLen+1000) // Invalid length

	_, err := DecodeRelayCell(payload)
	if err == nil {
		t.Fatal("Expected error for relay length exceeding maximum, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds maximum") {
		t.Errorf("Expected 'exceeds maximum' error, got: %v", err)
	}
}

// TestBufferOverflow_RelayLengthExceedsPayload verifies length vs payload validation
func TestBufferOverflow_RelayLengthExceedsPayload(t *testing.T) {
	// Create short payload but claim large length
	payload := make([]byte, 100)
	payload[0] = RelayData                          // Command
	binary.BigEndian.PutUint16(payload[1:3], 0)     // Recognized
	binary.BigEndian.PutUint16(payload[3:5], 1)     // StreamID
	binary.BigEndian.PutUint16(payload[9:11], 1000) // Length exceeds payload

	_, err := DecodeRelayCell(payload)
	if err == nil {
		t.Fatal("Expected error for relay length exceeding payload, got nil")
	}
	// The error caught should be about exceeding maximum first (1000 > 498)
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("Expected 'exceeds' error, got: %v", err)
	}
}

// TestBufferOverflow_ConcurrentDecoding verifies thread-safe decoding
func TestBufferOverflow_ConcurrentDecoding(t *testing.T) {
	// Create valid cell
	payload := make([]byte, PayloadLen)
	cell := &Cell{
		CircID:  12345,
		Command: CmdRelay,
		Payload: payload,
	}

	var buf bytes.Buffer
	if err := cell.Encode(&buf); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	cellBytes := buf.Bytes()

	// Decode concurrently
	done := make(chan bool)
	for i := 0; i < 100; i++ {
		go func() {
			defer func() { done <- true }()
			reader := bytes.NewReader(cellBytes)
			_, err := DecodeCell(reader)
			if err != nil {
				t.Errorf("Concurrent decode failed: %v", err)
			}
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		<-done
	}
}

// TestBufferOverflow_ZeroLengthPayload verifies zero-length payloads are safe
func TestBufferOverflow_ZeroLengthPayload(t *testing.T) {
	cell := &Cell{
		CircID:  12345,
		Command: CmdVPadding,
		Payload: []byte{},
	}

	var buf bytes.Buffer
	if err := cell.Encode(&buf); err != nil {
		t.Fatalf("Encode zero-length payload failed: %v", err)
	}

	decoded, err := DecodeCell(&buf)
	if err != nil {
		t.Fatalf("Decode zero-length payload failed: %v", err)
	}

	if len(decoded.Payload) != 0 {
		t.Errorf("Decoded payload should be zero-length, got %d", len(decoded.Payload))
	}
}

// TestBufferOverflow_RelayZeroLengthData verifies relay cells with zero data
func TestBufferOverflow_RelayZeroLengthData(t *testing.T) {
	rc, err := NewRelayCell(1, RelaySendme, []byte{})
	if err != nil {
		t.Fatalf("NewRelayCell with zero data failed: %v", err)
	}

	payload, err := rc.Encode()
	if err != nil {
		t.Fatalf("Encode zero-data relay cell failed: %v", err)
	}

	decoded, err := DecodeRelayCell(payload)
	if err != nil {
		t.Fatalf("Decode zero-data relay cell failed: %v", err)
	}

	if decoded.Length != 0 {
		t.Errorf("Decoded relay cell length should be 0, got %d", decoded.Length)
	}
	if len(decoded.Data) != 0 {
		t.Errorf("Decoded relay cell data should be empty, got %d bytes", len(decoded.Data))
	}
}

// TestBufferOverflow_MalformedReader verifies error handling for bad readers
func TestBufferOverflow_MalformedReader(t *testing.T) {
	// Reader that returns immediate EOF
	eofReader := bytes.NewReader([]byte{})
	_, err := DecodeCell(eofReader)
	if err == nil {
		t.Fatal("Expected error for EOF reader, got nil")
	}

	// Reader that returns error after partial read
	errorReader := &errorAfterNReader{n: 3, err: io.ErrUnexpectedEOF}
	_, err = DecodeCell(errorReader)
	if err == nil {
		t.Fatal("Expected error for error reader, got nil")
	}
}

// errorAfterNReader returns error after n bytes
type errorAfterNReader struct {
	n   int
	err error
}

func (r *errorAfterNReader) Read(p []byte) (n int, err error) {
	if r.n > 0 {
		if len(p) > r.n {
			p = p[:r.n]
		}
		r.n -= len(p)
		return len(p), nil
	}
	return 0, r.err
}

// TestBufferOverflow_EdgeCases verifies various edge cases
func TestBufferOverflow_EdgeCases(t *testing.T) {
	testCases := []struct {
		name     string
		circID   uint32
		cmd      Command
		payloadLen int
		expectErr bool
	}{
		{"MinCircID", 0, CmdRelay, 10, false},
		{"MaxCircID", 0xFFFFFFFF, CmdRelay, 10, false},
		{"MinPayload", 12345, CmdRelay, 0, false},
		{"MaxFixedPayload", 12345, CmdRelay, PayloadLen, false},
		{"MaxVarPayload", 12345, CmdVPadding, 65535, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			payload := make([]byte, tc.payloadLen)
			cell := &Cell{
				CircID:  tc.circID,
				Command: tc.cmd,
				Payload: payload,
			}

			var buf bytes.Buffer
			err := cell.Encode(&buf)
			if tc.expectErr && err == nil {
				t.Fatal("Expected error, got nil")
			}
			if !tc.expectErr && err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if !tc.expectErr {
				decoded, err := DecodeCell(&buf)
				if err != nil {
					t.Fatalf("Decode failed: %v", err)
				}
				if decoded.CircID != tc.circID {
					t.Errorf("CircID mismatch: got %d, want %d", decoded.CircID, tc.circID)
				}
			}
		})
	}
}

// TestBufferOverflow_RelayCellRoundTrip verifies relay cell integrity
func TestBufferOverflow_RelayCellRoundTrip(t *testing.T) {
	testCases := []struct {
		name     string
		streamID uint16
		cmd      byte
		dataLen  int
	}{
		{"SENDME_NoData", 0, RelaySendme, 0},
		{"DATA_SmallData", 1, RelayData, 10},
		{"DATA_MediumData", 2, RelayData, 250},
		{"DATA_MaxData", 3, RelayData, PayloadLen - RelayCellHeaderLen},
		{"BEGIN_SmallData", 4, RelayBegin, 50},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data := make([]byte, tc.dataLen)
			for i := range data {
				data[i] = byte(i % 256)
			}

			rc, err := NewRelayCell(tc.streamID, tc.cmd, data)
			if err != nil {
				t.Fatalf("NewRelayCell failed: %v", err)
			}

			payload, err := rc.Encode()
			if err != nil {
				t.Fatalf("Encode failed: %v", err)
			}

			decoded, err := DecodeRelayCell(payload)
			if err != nil {
				t.Fatalf("DecodeRelayCell failed: %v", err)
			}

			if decoded.StreamID != tc.streamID {
				t.Errorf("StreamID mismatch: got %d, want %d", decoded.StreamID, tc.streamID)
			}
			if decoded.Command != tc.cmd {
				t.Errorf("Command mismatch: got %d, want %d", decoded.Command, tc.cmd)
			}
			if !bytes.Equal(decoded.Data, data) {
				t.Error("Data mismatch after round-trip")
			}
		})
	}
}
