// Package cell provides tests for invalid cell command handling
// per tor-spec.txt §3 (Cell Encoding).
//
// These tests verify that cell encoding/decoding correctly handles
// unknown commands, boundary command values, malformed payloads, and
// relay cell edge cases. Cells are the fundamental unit of Tor
// network communication and must be handled defensively.
//
// Compliance: tor-spec.txt §3 (Cell Format), §6.1 (Relay Cells)
package cell

import (
	"bytes"
	"strings"
	"testing"
)

// TestCellCommandString verifies that all known commands produce
// readable strings and unknown commands produce "UNKNOWN(N)".
func TestCellCommandString(t *testing.T) {
	known := map[Command]string{
		CmdPadding:       "PADDING",
		CmdCreate:        "CREATE",
		CmdCreated:       "CREATED",
		CmdRelay:         "RELAY",
		CmdDestroy:       "DESTROY",
		CmdCreateFast:    "CREATE_FAST",
		CmdCreatedFast:   "CREATED_FAST",
		CmdVersions:      "VERSIONS",
		CmdNetinfo:       "NETINFO",
		CmdRelayEarly:    "RELAY_EARLY",
		CmdCreate2:       "CREATE2",
		CmdCreated2:      "CREATED2",
		CmdVPadding:      "VPADDING",
		CmdCerts:         "CERTS",
		CmdAuthChallenge: "AUTH_CHALLENGE",
		CmdAuthenticate:  "AUTHENTICATE",
		CmdAuthorize:     "AUTHORIZE",
	}

	for cmd, expected := range known {
		got := cmd.String()
		if got != expected {
			t.Errorf("Command(%d).String() = %q, want %q", cmd, got, expected)
		}
	}

	// Unknown commands should produce "UNKNOWN(N)"
	unknownCmds := []Command{12, 13, 50, 100, 127, 133, 200, 255}
	for _, cmd := range unknownCmds {
		got := cmd.String()
		if !strings.HasPrefix(got, "UNKNOWN(") {
			t.Errorf("Command(%d).String() = %q, want UNKNOWN(N) format", cmd, got)
		}
	}
}

// TestCellCommandIsVariableLength verifies the variable-length
// classification per tor-spec.txt §3.
func TestCellCommandIsVariableLength(t *testing.T) {
	tests := []struct {
		cmd  Command
		want bool
		desc string
	}{
		{CmdPadding, false, "PADDING is fixed"},
		{CmdCreate, false, "CREATE is fixed"},
		{CmdRelay, false, "RELAY is fixed"},
		{CmdDestroy, false, "DESTROY is fixed"},
		{CmdVersions, true, "VERSIONS is variable (special case)"},
		{CmdNetinfo, false, "NETINFO is fixed"},
		{CmdCreate2, false, "CREATE2 is fixed"},
		{CmdVPadding, true, "VPADDING is variable (>= 128)"},
		{CmdCerts, true, "CERTS is variable (>= 128)"},
		{CmdAuthChallenge, true, "AUTH_CHALLENGE is variable"},
		{CmdAuthenticate, true, "AUTHENTICATE is variable"},
		{Command(127), false, "command 127 is fixed"},
		{Command(128), true, "command 128 is variable (boundary)"},
		{Command(255), true, "command 255 is variable"},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			got := tc.cmd.IsVariableLength()
			if got != tc.want {
				t.Errorf("Command(%d).IsVariableLength() = %v, want %v",
					tc.cmd, got, tc.want)
			}
		})
	}
}

// TestEncodeDecodeFixedCellUnknownCommand verifies that cells with
// unknown fixed-size commands can be encoded and decoded correctly.
func TestEncodeDecodeFixedCellUnknownCommand(t *testing.T) {
	unknownFixed := []Command{12, 50, 100, 127}

	for _, cmd := range unknownFixed {
		t.Run(cmd.String(), func(t *testing.T) {
			original := NewCell(42, cmd)
			original.Payload = []byte("test data for unknown command")

			var buf bytes.Buffer
			if err := original.Encode(&buf); err != nil {
				t.Fatalf("Encode: %v", err)
			}

			decoded, err := DecodeCell(&buf)
			if err != nil {
				t.Fatalf("DecodeCell: %v", err)
			}

			if decoded.CircID != original.CircID {
				t.Errorf("CircID = %d, want %d", decoded.CircID, original.CircID)
			}
			if decoded.Command != original.Command {
				t.Errorf("Command = %d, want %d", decoded.Command, original.Command)
			}
			// Payload should start with original data (rest is zero padding)
			if !bytes.HasPrefix(decoded.Payload, original.Payload) {
				t.Error("decoded payload does not start with original")
			}
		})
	}
}

// TestEncodeDecodeVariableCellUnknownCommand verifies that cells
// with unknown variable-length commands (>= 128) round-trip correctly.
func TestEncodeDecodeVariableCellUnknownCommand(t *testing.T) {
	unknownVariable := []Command{133, 150, 200, 255}

	for _, cmd := range unknownVariable {
		t.Run(cmd.String(), func(t *testing.T) {
			original := NewCell(99, cmd)
			original.Payload = []byte("variable-length unknown command data")

			var buf bytes.Buffer
			if err := original.Encode(&buf); err != nil {
				t.Fatalf("Encode: %v", err)
			}

			decoded, err := DecodeCell(&buf)
			if err != nil {
				t.Fatalf("DecodeCell: %v", err)
			}

			if decoded.Command != cmd {
				t.Errorf("Command = %d, want %d", decoded.Command, cmd)
			}
			if !bytes.Equal(decoded.Payload, original.Payload) {
				t.Error("payload mismatch for variable-length cell")
			}
		})
	}
}

// TestDecodeFixedCellTruncatedPayload verifies that decoding
// a truncated fixed-size cell produces an error.
func TestDecodeFixedCellTruncatedPayload(t *testing.T) {
	// A valid fixed cell is 514 bytes (4 circID + 1 cmd + 509 payload)
	// Truncate at various points
	truncations := []int{0, 1, 4, 5, 100, 513}

	for _, sz := range truncations {
		// Build a partial cell
		fullCell := make([]byte, 514)
		fullCell[4] = byte(CmdPadding) // Fixed-size command

		partial := fullCell[:sz]
		_, err := DecodeCell(bytes.NewReader(partial))
		if err == nil {
			t.Errorf("truncation at %d bytes: expected error", sz)
		}
	}
}

// TestDecodeVariableCellTruncatedLength verifies that decoding
// a variable-length cell with truncated length field fails.
func TestDecodeVariableCellTruncatedLength(t *testing.T) {
	// Variable cell: 4 circID + 1 cmd + 2 length + payload
	// Provide only circID + cmd (no length bytes)
	data := make([]byte, 5)
	data[4] = byte(CmdCerts) // Variable-length command

	_, err := DecodeCell(bytes.NewReader(data))
	if err == nil {
		t.Error("expected error for missing length field")
	}
}

// TestDecodeVariableCellTruncatedPayload verifies that decoding
// a variable-length cell with insufficient payload fails.
func TestDecodeVariableCellTruncatedPayload(t *testing.T) {
	// Variable cell claiming 100 bytes but only providing 10
	data := make([]byte, 7+10)
	data[4] = byte(CmdCerts) // Variable-length command
	data[5] = 0              // Length high byte
	data[6] = 100            // Length low byte (claims 100)
	// Only 10 bytes of "payload" follow

	_, err := DecodeCell(bytes.NewReader(data))
	if err == nil {
		t.Error("expected error for truncated variable payload")
	}
}

// TestFixedCellPayloadTooLarge verifies that encoding a fixed-size
// cell with oversized payload fails.
func TestFixedCellPayloadTooLarge(t *testing.T) {
	c := NewCell(1, CmdRelay)
	c.Payload = make([]byte, PayloadLen+1) // 510 bytes

	var buf bytes.Buffer
	err := c.Encode(&buf)
	if err == nil {
		t.Error("expected error for oversized fixed-cell payload")
	}
}

// TestNewCellDefaults verifies NewCell initializes correctly.
func TestNewCellDefaults(t *testing.T) {
	c := NewCell(0, CmdPadding)
	if c.CircID != 0 {
		t.Errorf("CircID = %d, want 0", c.CircID)
	}
	if c.Command != CmdPadding {
		t.Errorf("Command = %d, want PADDING", c.Command)
	}
	if c.Payload == nil {
		t.Error("Payload is nil, want empty slice")
	}
	if len(c.Payload) != 0 {
		t.Errorf("Payload length = %d, want 0", len(c.Payload))
	}
}

// TestNewCellMaxCircuitID verifies NewCell with maximum circuit ID.
func TestNewCellMaxCircuitID(t *testing.T) {
	c := NewCell(0xFFFFFFFF, CmdCreate2)
	if c.CircID != 0xFFFFFFFF {
		t.Errorf("CircID = %d, want 0xFFFFFFFF", c.CircID)
	}

	var buf bytes.Buffer
	if err := c.Encode(&buf); err != nil {
		t.Fatalf("Encode: %v", err)
	}

	decoded, err := DecodeCell(&buf)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if decoded.CircID != 0xFFFFFFFF {
		t.Errorf("decoded CircID = %d, want 0xFFFFFFFF", decoded.CircID)
	}
}

// TestRelayCellDecodeEdgeCases verifies DecodeRelayCell handles
// malformed relay cell payloads.
func TestRelayCellDecodeEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		payload     []byte
		wantErr     bool
		errContains string
	}{
		{
			name:        "nil payload",
			payload:     nil,
			wantErr:     true,
			errContains: "payload too short",
		},
		{
			name:        "empty payload",
			payload:     []byte{},
			wantErr:     true,
			errContains: "payload too short",
		},
		{
			name:        "payload 10 bytes (one short)",
			payload:     make([]byte, 10),
			wantErr:     true,
			errContains: "payload too short",
		},
		{
			name:    "minimum valid header (11 bytes)",
			payload: make([]byte, 11),
			wantErr: false,
		},
		{
			name:    "full relay cell (509 bytes)",
			payload: make([]byte, 509),
			wantErr: false,
		},
		{
			name: "length claims more data than available",
			payload: func() []byte {
				p := make([]byte, 20)
				// Length field at bytes 9-10, claim 100 bytes
				p[9] = 0
				p[10] = 100
				return p
			}(),
			wantErr:     true,
			errContains: "relay cell data length exceeds payload",
		},
		{
			name: "length claims max (498) with full payload",
			payload: func() []byte {
				p := make([]byte, 509)
				// Length field: 498 (509 - 11 header)
				p[9] = 1
				p[10] = 242 // 498 = 0x01F2
				return p
			}(),
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := DecodeRelayCell(tc.payload)

			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tc.errContains != "" && !strings.Contains(err.Error(), tc.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.errContains)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if result == nil {
					t.Error("result is nil")
				}
			}
		})
	}
}

// TestRelayCellNewEdgeCases verifies NewRelayCell handles edge
// cases in relay cell construction.
func TestRelayCellNewEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		streamID uint16
		cmd      byte
		data     []byte
		wantErr  bool
	}{
		{"empty data", 1, RelayData, []byte{}, false},
		{"nil data", 1, RelayData, nil, false},
		{"small data", 1, RelayData, []byte("hello"), false},
		{"max data (498 bytes)", 1, RelayData, make([]byte, 498), false},
		{"too large (499 bytes)", 1, RelayData, make([]byte, 499), true},
		{"stream ID 0", 0, RelayBegin, []byte("test"), false},
		{"max stream ID", 0xFFFF, RelayEnd, []byte{}, false},
		{"all relay commands", 1, RelayDrop, []byte{}, false},
		{"onion service command", 1, RelayIntroduce1, make([]byte, 100), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rc, err := NewRelayCell(tc.streamID, tc.cmd, tc.data)

			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if rc == nil {
					t.Fatal("result is nil")
				}
				if rc.StreamID != tc.streamID {
					t.Errorf("StreamID = %d, want %d", rc.StreamID, tc.streamID)
				}
				if rc.Command != tc.cmd {
					t.Errorf("Command = %d, want %d", rc.Command, tc.cmd)
				}
			}
		})
	}
}

// TestRelayCellEncodeDecodeRoundTrip verifies that relay cells
// survive encode/decode round-trips.
func TestRelayCellEncodeDecodeRoundTrip(t *testing.T) {
	testData := []struct {
		name     string
		streamID uint16
		cmd      byte
		data     []byte
	}{
		{"empty data", 1, RelayData, []byte{}},
		{"small data", 42, RelayBegin, []byte("host:80")},
		{"max data", 100, RelayData, make([]byte, 498)},
		{"SENDME", 0, RelaySendme, []byte{}},
	}

	for _, td := range testData {
		t.Run(td.name, func(t *testing.T) {
			rc, err := NewRelayCell(td.streamID, td.cmd, td.data)
			if err != nil {
				t.Fatalf("NewRelayCell: %v", err)
			}

			encoded, err := rc.Encode()
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}

			if len(encoded) != PayloadLen {
				t.Errorf("encoded length = %d, want %d", len(encoded), PayloadLen)
			}

			decoded, err := DecodeRelayCell(encoded)
			if err != nil {
				t.Fatalf("DecodeRelayCell: %v", err)
			}

			if decoded.Command != td.cmd {
				t.Errorf("Command = %d, want %d", decoded.Command, td.cmd)
			}
			if decoded.StreamID != td.streamID {
				t.Errorf("StreamID = %d, want %d", decoded.StreamID, td.streamID)
			}
			if !bytes.Equal(decoded.Data, td.data) {
				t.Error("data mismatch after round-trip")
			}
		})
	}
}

// TestRelayCmdStringUnknown verifies human-readable relay command names.
func TestRelayCmdStringUnknown(t *testing.T) {
	known := map[byte]string{
		RelayBegin:     "RELAY_BEGIN",
		RelayData:      "RELAY_DATA",
		RelayEnd:       "RELAY_END",
		RelaySendme:    "RELAY_SENDME",
		RelayExtend2:   "RELAY_EXTEND2",
		RelayExtended2: "RELAY_EXTENDED2",
	}

	for cmd, expected := range known {
		got := RelayCmdString(cmd)
		if got != expected {
			t.Errorf("RelayCmdString(%d) = %q, want %q", cmd, got, expected)
		}
	}

	// Unknown relay commands
	got := RelayCmdString(99)
	if !strings.Contains(got, "UNKNOWN") {
		t.Errorf("RelayCmdString(99) = %q, want string containing UNKNOWN", got)
	}
}
