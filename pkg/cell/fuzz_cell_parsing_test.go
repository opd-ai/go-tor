// Package cell provides fuzzing tests for cell encoding and decoding.
//
// These fuzzing tests verify that cell parsing never panics on arbitrary
// or malformed input, which is critical for a network-facing protocol
// implementation. Any panic in the cell parsing code could be triggered
// by a malicious peer sending crafted cells.
//
// Run the fuzz tests with:
//
//	go test -fuzz=FuzzDecodeCell -fuzztime=30s
//	go test -fuzz=FuzzDecodeRelayCell -fuzztime=30s
//
// The seed corpus is defined inline and covers known edge cases.
// The fuzzer explores additional inputs automatically.
//
// Compliance: CWE-120 (Buffer Copy without Checking Size of Input),
// tor-spec.txt §0.2 (Cell Format)
package cell

import (
	"bytes"
	"testing"
)

// FuzzDecodeCell verifies that DecodeCell never panics on arbitrary input.
// This covers all code paths in cell parsing including fixed-size and
// variable-length cell handling.
func FuzzDecodeCell(f *testing.F) {
	// Seed corpus: valid cells
	// Valid fixed-size cell (514 bytes)
	validFixed := make([]byte, 514)
	validFixed[4] = byte(CmdRelay) // Command: Relay (fixed-length)
	f.Add(validFixed)

	// Valid variable-length cell (VERSIONS: circID=0, cmd=7, len=2, payload=0x00,0x04)
	versions := []byte{0x00, 0x00, 0x00, 0x00, byte(CmdVersions), 0x00, 0x02, 0x00, 0x04}
	f.Add(versions)

	// Valid CERTS cell (variable-length, cmd=129)
	certsHeader := []byte{0x00, 0x00, 0x00, 0x01, byte(CmdCerts), 0x00, 0x01, 0x00}
	f.Add(certsHeader)

	// Seed corpus: adversarial edge cases
	// Empty input
	f.Add([]byte{})

	// Single byte
	f.Add([]byte{0x00})

	// Just circuit ID (4 bytes)
	f.Add([]byte{0x00, 0x00, 0x00, 0x01})

	// Circuit ID + command, no payload
	f.Add([]byte{0x00, 0x00, 0x00, 0x01, 0x03})

	// Variable-length cell with claimed length larger than data
	f.Add([]byte{0x00, 0x00, 0x00, 0x00, byte(CmdVersions), 0xFF, 0xFF})

	// Variable-length cell with zero length
	f.Add([]byte{0x00, 0x00, 0x00, 0x00, byte(CmdVersions), 0x00, 0x00})

	// Fixed-size cell truncated to 100 bytes
	f.Add(make([]byte, 100))

	// Maximum valid variable-length cell
	maxVarCell := make([]byte, 7+65535)
	maxVarCell[4] = byte(CmdVersions)
	maxVarCell[5] = 0xFF
	maxVarCell[6] = 0xFF
	f.Add(maxVarCell)

	f.Fuzz(func(t *testing.T, data []byte) {
		// The fuzz target must not panic for any input
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("DecodeCell panicked on %d bytes: %v", len(data), r)
			}
		}()

		// Attempt to decode the cell — may return an error, must not panic
		_, _ = DecodeCell(bytes.NewReader(data))
	})
}

// FuzzDecodeRelayCell verifies that DecodeRelayCell never panics on
// arbitrary input. Relay cells are the most complex cell type in the
// Tor protocol, with multiple fields that can be crafted maliciously.
func FuzzDecodeRelayCell(f *testing.F) {
	// Seed corpus: valid relay cell payload (509 bytes)
	// Relay cell format: cmd(1) + recognized(2) + streamID(2) + digest(4) + length(2) + data(498)
	validRelay := make([]byte, 509)
	validRelay[0] = byte(RelayData) // RelayData command
	// recognized = 0x0000 (unencrypted, valid)
	// streamID = 0x0001
	validRelay[3] = 0x00
	validRelay[4] = 0x01
	// digest = 0 (4 bytes)
	// length = 0x0004 (4 bytes of data)
	validRelay[9] = 0x00
	validRelay[10] = 0x04
	// data: 4 bytes "test"
	copy(validRelay[11:], []byte("test"))
	f.Add(validRelay)

	// Seed: empty payload
	f.Add([]byte{})

	// Seed: 1 byte
	f.Add([]byte{0x01})

	// Seed: exactly 10 bytes (one short of minimum 11)
	f.Add(make([]byte, 10))

	// Seed: exactly 11 bytes (minimum valid)
	f.Add(make([]byte, 11))

	// Seed: relay cell with length field larger than available data
	tooLong := make([]byte, 11)
	tooLong[0] = byte(RelayData)
	tooLong[9] = 0xFF // length = 0xFF00 > available
	tooLong[10] = 0x00
	f.Add(tooLong)

	// Seed: relay cell with max data length
	maxData := make([]byte, 509)
	maxData[9] = 0x01 // length = 0x01F2 = 498
	maxData[10] = 0xF2
	f.Add(maxData)

	// Seed: relay cell with extended data (padded to 509)
	f.Add(make([]byte, 509))

	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("DecodeRelayCell panicked on %d bytes: %v", len(data), r)
			}
		}()

		_, _ = DecodeRelayCell(data)
	})
}

// FuzzCellEncode verifies that Cell.Encode never panics on arbitrary
// cell structures (including oversized payloads).
func FuzzCellEncode(f *testing.F) {
	// Valid cells
	f.Add(uint32(0), byte(CmdRelay), []byte{})
	f.Add(uint32(1), byte(CmdRelay), make([]byte, 509))
	f.Add(uint32(0xFFFFFFFF), byte(CmdVersions), []byte{0x00, 0x04})

	// Boundary conditions
	f.Add(uint32(0), byte(0), []byte{})                 // Unknown command
	f.Add(uint32(0), byte(255), []byte{})               // Max command byte
	f.Add(uint32(0), byte(CmdRelay), make([]byte, 510)) // Oversized payload

	f.Fuzz(func(t *testing.T, circID uint32, cmdByte byte, payload []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Cell.Encode panicked with circID=%d cmd=%d payload_len=%d: %v",
					circID, cmdByte, len(payload), r)
			}
		}()

		c := &Cell{
			CircID:  circID,
			Command: Command(cmdByte),
			Payload: payload,
		}
		_ = c.Encode(bytes.NewBuffer(nil))
	})
}

// FuzzNewRelayCell verifies that NewRelayCell never panics on arbitrary
// stream IDs, commands, and data.
func FuzzNewRelayCell(f *testing.F) {
	// Valid relay cells
	f.Add(uint16(1), byte(RelayData), []byte("hello"))
	f.Add(uint16(0), byte(RelayBegin), []byte("127.0.0.1:80\x00"))
	f.Add(uint16(1), byte(RelayData), make([]byte, 498))

	// Edge cases
	f.Add(uint16(0xFFFF), byte(255), []byte{})           // Max stream ID, unknown cmd
	f.Add(uint16(0), byte(0), make([]byte, 499))         // Oversized data
	f.Add(uint16(1), byte(RelayData), make([]byte, 499)) // Just over limit

	f.Fuzz(func(t *testing.T, streamID uint16, cmdByte byte, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("NewRelayCell panicked with streamID=%d cmd=%d data_len=%d: %v",
					streamID, cmdByte, len(data), r)
			}
		}()

		_, _ = NewRelayCell(streamID, cmdByte, data)
	})
}
