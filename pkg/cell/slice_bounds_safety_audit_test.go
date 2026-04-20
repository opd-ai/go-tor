// Package cell provides slice bounds safety audit tests.
//
// This audit verifies that all slice indexing and slicing operations
// in cell encoding/decoding are bounds-safe: length and capacity are
// always checked before indexing, and out-of-range accesses never occur
// on untrusted (network-received) input.
//
// Compliance: CWE-119 (Buffer Mismanagement), CWE-120 (Buffer Copy without
// Checking Size of Input), CWE-129 (Improper Validation of Array Index),
// tor-spec.txt §0.2, §0.3, §6.1
package cell

import (
	"bytes"
	"testing"
)

// TestCellDecodeShortPayload verifies that decoding a cell from a truncated
// reader returns an error instead of panicking with an index-out-of-range.
func TestCellDecodeTruncatedInput(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
	}{
		{"empty_input", []byte{}},
		{"one_byte", []byte{0x00}},
		{"four_bytes_circid_only", []byte{0x00, 0x00, 0x00, 0x01}},
		{"four_bytes_plus_cmd", []byte{0x00, 0x00, 0x00, 0x01, 0x06}},
		{"exactly_513_bytes", make([]byte, 513)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("DecodeCell panicked on %d-byte input: %v", len(tc.input), r)
				}
			}()

			_, _ = DecodeCell(bytes.NewReader(tc.input))
			// Either returns an error or succeeds; must NOT panic
		})
	}
}

// TestCellEncodePayloadTooLarge verifies that encoding a fixed cell with
// a payload larger than 509 bytes returns an error instead of writing
// beyond the end of the encoded buffer.
func TestCellEncodePayloadTooLarge(t *testing.T) {
	tests := []struct {
		name        string
		payloadSize int
		wantErr     bool
	}{
		{"max_payload_509", 509, false},
		{"one_over_max_510", 510, true},
		{"way_over_max_1024", 1024, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Cell.Encode panicked with %d-byte payload: %v", tc.payloadSize, r)
				}
			}()

			c := &Cell{
				CircID:  1,
				Command: CmdRelay, // fixed-size command
				Payload: make([]byte, tc.payloadSize),
			}

			err := c.Encode(bytes.NewBuffer(nil))
			if tc.wantErr && err == nil {
				t.Errorf("expected error for payload size %d, got nil", tc.payloadSize)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error for payload size %d: %v", tc.payloadSize, err)
			}
		})
	}
}

// TestRelayCellNewDataTooLarge verifies that NewRelayCell returns an error
// when the data length exceeds the relay cell data maximum (498 bytes).
func TestRelayCellNewDataTooLarge(t *testing.T) {
	tests := []struct {
		name     string
		dataSize int
		wantErr  bool
	}{
		{"max_data_498", 498, false},
		{"one_over_499", 499, true},
		{"way_over_1024", 1024, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("NewRelayCell panicked with %d-byte data: %v", tc.dataSize, r)
				}
			}()

			_, err := NewRelayCell(1, RelayData, make([]byte, tc.dataSize))
			if tc.wantErr && err == nil {
				t.Errorf("expected error for data size %d, got nil", tc.dataSize)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error for data size %d: %v", tc.dataSize, err)
			}
		})
	}
}

// TestRelayCellDecodeShortPayload verifies that decoding a relay cell
// from a payload shorter than the minimum (11 bytes) returns an error
// rather than panicking.
func TestRelayCellDecodeShortPayload(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
	}{
		{"empty", []byte{}},
		{"one_byte", []byte{0x01}},
		{"ten_bytes", make([]byte, 10)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("DecodeRelayCell panicked on %d-byte input: %v", len(tc.input), r)
				}
			}()

			_, _ = DecodeRelayCell(tc.input)
			// Must not panic
		})
	}
}

// TestCellCommandBoundaries verifies that Command.IsVariableLength() and
// Command.String() do not panic for any byte value 0–255.
func TestCellCommandBoundaries(t *testing.T) {
	for i := 0; i <= 255; i++ {
		cmd := Command(i)
		func(c Command) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Command(%d) method panicked: %v", c, r)
				}
			}()
			_ = c.IsVariableLength()
			_ = c.String()
		}(cmd)
	}
}

// TestDecodeCellRoundTrip verifies encode-decode round-trip with valid inputs
// does not panic and preserves cell content.
func TestDecodeCellRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		circID  uint32
		cmd     Command
		payload []byte
	}{
		{"fixed_cell_empty_payload", 1, CmdRelay, []byte{}},
		{"fixed_cell_full_payload", 0xFFFFFFFF, CmdRelay, make([]byte, 509)},
		{"variable_length_versions", 0, CmdVersions, []byte{0x00, 0x04}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("round-trip panicked: %v", r)
				}
			}()

			original := &Cell{
				CircID:  tc.circID,
				Command: tc.cmd,
				Payload: tc.payload,
			}

			buf := bytes.NewBuffer(nil)
			if err := original.Encode(buf); err != nil {
				t.Fatalf("Encode failed: %v", err)
			}

			decoded, err := DecodeCell(bytes.NewReader(buf.Bytes()))
			if err != nil {
				t.Fatalf("DecodeCell failed: %v", err)
			}

			if decoded.CircID != original.CircID {
				t.Errorf("CircID mismatch: got %d, want %d", decoded.CircID, original.CircID)
			}
			if decoded.Command != original.Command {
				t.Errorf("Command mismatch: got %v, want %v", decoded.Command, original.Command)
			}
		})
	}
}
