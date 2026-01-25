package cell

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

// TestCellSizeCompliance verifies cell encoding matches tor-spec.txt §0.2
// "Cells are 512 bytes long... On a v3 or higher connection, variable-length cells
// are indicated by a command byte equal to 7 or greater than 128."
func TestCellSizeCompliance(t *testing.T) {
	t.Run("FixedSize514Bytes", func(t *testing.T) {
		// All fixed-size cells must be exactly 514 bytes per spec
		fixedCmds := []Command{
			CmdPadding, CmdCreate, CmdCreated, CmdRelay, CmdDestroy,
			CmdCreateFast, CmdCreatedFast, CmdNetinfo, CmdRelayEarly,
			CmdCreate2, CmdCreated2,
		}

		for _, cmd := range fixedCmds {
			cell := &Cell{
				CircID:  1,
				Command: cmd,
				Payload: []byte{1, 2, 3}, // Small payload
			}

			var buf bytes.Buffer
			if err := cell.Encode(&buf); err != nil {
				t.Fatalf("cmd %s: Encode failed: %v", cmd, err)
			}

			if buf.Len() != 514 {
				t.Errorf("cmd %s: encoded size = %d bytes, want 514 (tor-spec.txt §0.2)", cmd, buf.Len())
			}
		}
	})

	t.Run("FixedSizeFullPayload", func(t *testing.T) {
		// Test with maximum payload (509 bytes)
		cell := &Cell{
			CircID:  1,
			Command: CmdRelay,
			Payload: make([]byte, PayloadLen),
		}

		var buf bytes.Buffer
		if err := cell.Encode(&buf); err != nil {
			t.Fatalf("Encode failed: %v", err)
		}

		if buf.Len() != 514 {
			t.Errorf("full payload: encoded size = %d bytes, want 514", buf.Len())
		}
	})

	t.Run("VariableLengthFormat", func(t *testing.T) {
		// Variable-length cells: CircID(4) + Cmd(1) + Length(2) + Payload
		varCmds := []Command{
			CmdVPadding, CmdCerts, CmdAuthChallenge, CmdAuthenticate, CmdAuthorize,
		}

		for _, cmd := range varCmds {
			payloadSize := 100
			cell := &Cell{
				CircID:  1,
				Command: cmd,
				Payload: make([]byte, payloadSize),
			}

			var buf bytes.Buffer
			if err := cell.Encode(&buf); err != nil {
				t.Fatalf("cmd %s: Encode failed: %v", cmd, err)
			}

			expectedSize := 4 + 1 + 2 + payloadSize // CircID + Cmd + Len + Payload
			if buf.Len() != expectedSize {
				t.Errorf("cmd %s: encoded size = %d bytes, want %d", cmd, buf.Len(), expectedSize)
			}
		}
	})

	t.Run("VersionsCellIsVariableLength", func(t *testing.T) {
		// VERSIONS (cmd 7) is special: always variable-length per spec
		cell := &Cell{
			CircID:  0, // VERSIONS uses CircID=0
			Command: CmdVersions,
			Payload: []byte{0, 3, 0, 4, 0, 5}, // Versions 3, 4, 5
		}

		var buf bytes.Buffer
		if err := cell.Encode(&buf); err != nil {
			t.Fatalf("Encode failed: %v", err)
		}

		// Should be: CircID(4) + Cmd(1) + Len(2) + Payload(6) = 13 bytes
		expectedSize := 4 + 1 + 2 + 6
		if buf.Len() != expectedSize {
			t.Errorf("VERSIONS: encoded size = %d bytes, want %d (variable-length)", buf.Len(), expectedSize)
		}
	})
}

// TestCircuitIDEncoding verifies CircID encoding per tor-spec.txt §0.2
// "Circuit IDs are 4 bytes long on a v4 or higher connection"
func TestCircuitIDEncoding(t *testing.T) {
	t.Run("4BytesBigEndian", func(t *testing.T) {
		testCases := []uint32{
			0x00000000,
			0x00000001,
			0x7FFFFFFF, // Max positive int32
			0x80000000, // High bit set
			0xFFFFFFFF, // All bits set
		}

		for _, circID := range testCases {
			cell := &Cell{
				CircID:  circID,
				Command: CmdCreate,
				Payload: []byte{1, 2, 3},
			}

			var buf bytes.Buffer
			if err := cell.Encode(&buf); err != nil {
				t.Fatalf("CircID %#x: Encode failed: %v", circID, err)
			}

			// Read first 4 bytes as big-endian uint32
			var decoded uint32
			if err := binary.Read(&buf, binary.BigEndian, &decoded); err != nil {
				t.Fatalf("CircID %#x: Read failed: %v", circID, err)
			}

			if decoded != circID {
				t.Errorf("CircID encoding: got %#x, want %#x", decoded, circID)
			}
		}
	})

	t.Run("RoundTripAllValues", func(t *testing.T) {
		// Test multiple circuit IDs round-trip correctly
		circIDs := []uint32{0, 1, 42, 0x1000, 0xABCDEF, 0xFFFFFFFF}

		for _, circID := range circIDs {
			cell := &Cell{
				CircID:  circID,
				Command: CmdPadding,
				Payload: []byte{},
			}

			var buf bytes.Buffer
			if err := cell.Encode(&buf); err != nil {
				t.Fatalf("CircID %d: Encode failed: %v", circID, err)
			}

			decoded, err := DecodeCell(&buf)
			if err != nil {
				t.Fatalf("CircID %d: Decode failed: %v", circID, err)
			}

			if decoded.CircID != circID {
				t.Errorf("CircID round-trip: got %d, want %d", decoded.CircID, circID)
			}
		}
	})
}

// TestCommandTypeCompliance verifies all command types per tor-spec.txt §0.3
func TestCommandTypeCompliance(t *testing.T) {
	t.Run("AllFixedSizeCommands", func(t *testing.T) {
		// Commands 0-127 (except VERSIONS=7) are fixed-size
		fixedCmds := map[Command]string{
			0:  "PADDING",
			1:  "CREATE",
			2:  "CREATED",
			3:  "RELAY",
			4:  "DESTROY",
			5:  "CREATE_FAST",
			6:  "CREATED_FAST",
			8:  "NETINFO",
			9:  "RELAY_EARLY",
			10: "CREATE2",
			11: "CREATED2",
		}

		for cmd, name := range fixedCmds {
			if cmd.IsVariableLength() {
				t.Errorf("cmd %d (%s): should be fixed-size, got variable-length", cmd, name)
			}
		}
	})

	t.Run("AllVariableLengthCommands", func(t *testing.T) {
		// Commands >= 128 are variable-length
		varCmds := map[Command]string{
			128: "VPADDING",
			129: "CERTS",
			130: "AUTH_CHALLENGE",
			131: "AUTHENTICATE",
			132: "AUTHORIZE",
		}

		for cmd, name := range varCmds {
			if !cmd.IsVariableLength() {
				t.Errorf("cmd %d (%s): should be variable-length, got fixed-size", cmd, name)
			}
		}
	})

	t.Run("VersionsSpecialCase", func(t *testing.T) {
		// VERSIONS (7) is the only fixed-size command number that's variable-length
		if !CmdVersions.IsVariableLength() {
			t.Error("VERSIONS (cmd 7) must be variable-length per spec")
		}
		if CmdVersions != 7 {
			t.Errorf("VERSIONS command value = %d, want 7", CmdVersions)
		}
	})

	t.Run("CommandStringRepresentation", func(t *testing.T) {
		// All defined commands should have proper string representations
		tests := []struct {
			cmd  Command
			want string
		}{
			{CmdPadding, "PADDING"},
			{CmdCreate, "CREATE"},
			{CmdCreated, "CREATED"},
			{CmdRelay, "RELAY"},
			{CmdDestroy, "DESTROY"},
			{CmdCreateFast, "CREATE_FAST"},
			{CmdCreatedFast, "CREATED_FAST"},
			{CmdVersions, "VERSIONS"},
			{CmdNetinfo, "NETINFO"},
			{CmdRelayEarly, "RELAY_EARLY"},
			{CmdCreate2, "CREATE2"},
			{CmdCreated2, "CREATED2"},
			{CmdVPadding, "VPADDING"},
			{CmdCerts, "CERTS"},
			{CmdAuthChallenge, "AUTH_CHALLENGE"},
			{CmdAuthenticate, "AUTHENTICATE"},
			{CmdAuthorize, "AUTHORIZE"},
		}

		for _, tt := range tests {
			if got := tt.cmd.String(); got != tt.want {
				t.Errorf("Command(%d).String() = %q, want %q", tt.cmd, got, tt.want)
			}
		}
	})

	t.Run("UnknownCommandFormat", func(t *testing.T) {
		// Unknown commands should return UNKNOWN(n)
		unknownCmd := Command(250)
		str := unknownCmd.String()
		if !strings.HasPrefix(str, "UNKNOWN(") {
			t.Errorf("Unknown command string = %q, want UNKNOWN(...)", str)
		}
	})
}

// TestCellPayloadCompliance verifies payload handling per tor-spec.txt §0.2
func TestCellPayloadCompliance(t *testing.T) {
	t.Run("FixedCellPayloadAlways509Bytes", func(t *testing.T) {
		// Fixed-size cells always have exactly 509 bytes of payload space
		testSizes := []int{0, 1, 100, 509}

		for _, size := range testSizes {
			cell := &Cell{
				CircID:  1,
				Command: CmdCreate,
				Payload: make([]byte, size),
			}

			var buf bytes.Buffer
			if err := cell.Encode(&buf); err != nil {
				t.Fatalf("size %d: Encode failed: %v", size, err)
			}

			decoded, err := DecodeCell(&buf)
			if err != nil {
				t.Fatalf("size %d: Decode failed: %v", size, err)
			}

			if len(decoded.Payload) != PayloadLen {
				t.Errorf("size %d: decoded payload length = %d, want %d", size, len(decoded.Payload), PayloadLen)
			}
		}
	})

	t.Run("VariableCellPayloadPreserved", func(t *testing.T) {
		// Variable-length cells preserve exact payload size
		testSizes := []int{0, 1, 100, 1000, 10000}

		for _, size := range testSizes {
			payload := make([]byte, size)
			for i := range payload {
				payload[i] = byte(i % 256)
			}

			cell := &Cell{
				CircID:  1,
				Command: CmdCerts,
				Payload: payload,
			}

			var buf bytes.Buffer
			if err := cell.Encode(&buf); err != nil {
				t.Fatalf("size %d: Encode failed: %v", size, err)
			}

			decoded, err := DecodeCell(&buf)
			if err != nil {
				t.Fatalf("size %d: Decode failed: %v", size, err)
			}

			if len(decoded.Payload) != size {
				t.Errorf("size %d: decoded payload length = %d, want %d", size, len(decoded.Payload), size)
			}

			if !bytes.Equal(decoded.Payload, payload) {
				t.Errorf("size %d: payload corrupted during encode/decode", size)
			}
		}
	})

	t.Run("PaddingIsZeroBytes", func(t *testing.T) {
		// Fixed-size cells pad with zero bytes
		cell := &Cell{
			CircID:  1,
			Command: CmdCreate,
			Payload: []byte{0xAA, 0xBB, 0xCC}, // 3 bytes
		}

		var buf bytes.Buffer
		if err := cell.Encode(&buf); err != nil {
			t.Fatalf("Encode failed: %v", err)
		}

		// Skip CircID(4) + Cmd(1) + Payload(3) = 8 bytes
		buf.Next(8)

		// Remaining bytes should be padding (506 zero bytes)
		paddingBytes := buf.Bytes()
		if len(paddingBytes) != 506 {
			t.Fatalf("padding length = %d, want 506", len(paddingBytes))
		}

		for i, b := range paddingBytes {
			if b != 0 {
				t.Errorf("padding[%d] = %#x, want 0x00", i, b)
				break
			}
		}
	})
}

// TestCellEncodingErrorCases verifies error handling
func TestCellEncodingErrorCases(t *testing.T) {
	t.Run("EncodeVariableCellTooLarge", func(t *testing.T) {
		// Variable-length cells have max payload of 65535 bytes (uint16)
		cell := &Cell{
			CircID:  1,
			Command: CmdCerts,
			Payload: make([]byte, 65536), // One byte too large
		}

		var buf bytes.Buffer
		err := cell.Encode(&buf)
		if err == nil {
			t.Error("Encode should fail for payload > 65535 bytes")
		}
	})

	t.Run("DecodePartialCell", func(t *testing.T) {
		// Decoding truncated fixed-size cell should fail
		var buf bytes.Buffer
		binary.Write(&buf, binary.BigEndian, uint32(1)) // CircID
		binary.Write(&buf, binary.BigEndian, CmdCreate) // Command
		buf.Write(make([]byte, 100))                    // Only 100 bytes instead of 509

		_, err := DecodeCell(&buf)
		if err == nil {
			t.Error("DecodeCell should fail for truncated cell")
		}
	})

	t.Run("DecodePartialVariableCell", func(t *testing.T) {
		// Decoding truncated variable-length cell should fail
		var buf bytes.Buffer
		binary.Write(&buf, binary.BigEndian, uint32(1))  // CircID
		binary.Write(&buf, binary.BigEndian, CmdCerts)   // Command
		binary.Write(&buf, binary.BigEndian, uint16(50)) // Length = 50
		buf.Write(make([]byte, 25))                      // Only 25 bytes instead of 50

		_, err := DecodeCell(&buf)
		if err == nil {
			t.Error("DecodeCell should fail for truncated variable-length payload")
		}
	})

	t.Run("DecodeEmptyReader", func(t *testing.T) {
		var buf bytes.Buffer
		_, err := DecodeCell(&buf)
		if err == nil {
			t.Error("DecodeCell should fail for empty reader")
		}
		// Error is wrapped, so check with errors.Is or look for EOF in message
		if err != nil && !strings.Contains(err.Error(), "EOF") {
			t.Errorf("DecodeCell error = %v, want EOF-related error", err)
		}
	})
}

// TestDestroyReasonCompliance verifies DESTROY reason codes per tor-spec.txt §5.4
func TestDestroyReasonCompliance(t *testing.T) {
	t.Run("AllDefinedReasons", func(t *testing.T) {
		// Verify all destroy reasons are defined as per spec
		reasons := map[byte]string{
			DestroyReasonNone:          "NONE",
			DestroyReasonProtocol:      "PROTOCOL",
			DestroyReasonInternal:      "INTERNAL",
			DestroyReasonRequested:     "REQUESTED",
			DestroyReasonHibernating:   "HIBERNATING",
			DestroyReasonResourceLimit: "RESOURCELIMIT",
			DestroyReasonConnectFailed: "CONNECTFAILED",
			DestroyReasonNoRoute:       "NOROUTE",
			DestroyReasonTimeout:       "TIMEOUT",
			DestroyReasonDestroyed:     "DESTROYED",
			DestroyReasonNosuchservice: "NOSUCHSERVICE",
		}

		// Verify values match spec
		expectedValues := map[byte]byte{
			DestroyReasonNone:          0,
			DestroyReasonProtocol:      1,
			DestroyReasonInternal:      2,
			DestroyReasonRequested:     3,
			DestroyReasonHibernating:   4,
			DestroyReasonResourceLimit: 5,
			DestroyReasonConnectFailed: 6,
			DestroyReasonNoRoute:       7,
			DestroyReasonTimeout:       8,
			DestroyReasonDestroyed:     9,
			DestroyReasonNosuchservice: 10,
		}

		for reason, expectedValue := range expectedValues {
			if reason != expectedValue {
				name := reasons[reason]
				t.Errorf("DestroyReason%s = %d, want %d", name, reason, expectedValue)
			}
		}
	})

	t.Run("DestroyCell", func(t *testing.T) {
		// DESTROY cell should encode/decode properly with reason
		cell := &Cell{
			CircID:  42,
			Command: CmdDestroy,
			Payload: []byte{DestroyReasonRequested},
		}

		var buf bytes.Buffer
		if err := cell.Encode(&buf); err != nil {
			t.Fatalf("Encode failed: %v", err)
		}

		decoded, err := DecodeCell(&buf)
		if err != nil {
			t.Fatalf("Decode failed: %v", err)
		}

		if len(decoded.Payload) < 1 {
			t.Fatal("DESTROY cell payload is empty")
		}

		if decoded.Payload[0] != DestroyReasonRequested {
			t.Errorf("DESTROY reason = %d, want %d", decoded.Payload[0], DestroyReasonRequested)
		}
	})
}
