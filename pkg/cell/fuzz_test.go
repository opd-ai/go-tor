package cell

import (
	"bytes"
	"testing"
)

// FuzzDecodeCell exercises the cell decoder with arbitrary byte sequences.
// The fuzzer is expected to find inputs that trigger panics, excessive
// allocations, or invalid behaviour without crashing the process.
func FuzzDecodeCell(f *testing.F) {
	// Seed corpus: minimal valid fixed-length cell (4+1+509 bytes).
	fixedCell := make([]byte, 514)
	fixedCell[4] = byte(CmdRelay) // command byte
	f.Add(fixedCell)

	// Seed corpus: minimal valid variable-length cell (VERSIONS cell, cmd=7).
	varCell := []byte{
		0x00, 0x00, 0x00, 0x01, // CircID
		0x07,       // CmdVersions (variable-length)
		0x00, 0x02, // payload len = 2
		0x00, 0x03, // version 3
	}
	f.Add(varCell)

	// Seed corpus: truncated header (should return error, not panic).
	f.Add([]byte{0x00, 0x01})

	// Seed corpus: empty input.
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		r := bytes.NewReader(data)
		// Must not panic; errors are acceptable.
		_, _ = DecodeCell(r)
	})
}

// FuzzDecodeRelayCell exercises the relay cell decoder with arbitrary payloads.
func FuzzDecodeRelayCell(f *testing.F) {
	// Seed corpus: valid relay cell payload (header + data).
	validPayload := make([]byte, PayloadLen)
	validPayload[0] = RelayData          // command
	validPayload[1] = 0x00               // recognized hi
	validPayload[2] = 0x00               // recognized lo
	validPayload[3] = 0x00               // stream ID hi
	validPayload[4] = 0x01               // stream ID lo
	// digest: bytes 5–8 (zero)
	validPayload[9] = 0x00  // length hi
	validPayload[10] = 0x04 // length lo = 4 bytes of data
	copy(validPayload[11:], []byte("test"))
	f.Add(validPayload)

	// Seed corpus: too-short payload (should return error).
	f.Add(make([]byte, RelayCellHeaderLen-1))

	// Seed corpus: empty payload.
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, payload []byte) {
		// Must not panic; errors are acceptable.
		_, _ = DecodeRelayCell(payload)
	})
}
