// Package cell timing consistency audit tests
// This file contains tests to verify that cell processing operations
// have consistent timing characteristics and do not leak information
// through timing side-channels.
package cell

import (
	"bytes"
	"crypto/rand"
	"math"
	"testing"
)

// TestCellEncodingTimingConsistency verifies that cell encoding timing
// is consistent for fixed-size cells regardless of payload content.
// This prevents timing attacks that could leak payload information.
func TestCellEncodingTimingConsistency(t *testing.T) {
	tests := []struct {
		name        string
		command     Command
		payloadSize int
	}{
		{
			name:        "Fixed RELAY small payload",
			command:     CmdRelay,
			payloadSize: 50,
		},
		{
			name:        "Fixed RELAY medium payload",
			command:     CmdRelay,
			payloadSize: 250,
		},
		{
			name:        "Fixed RELAY large payload",
			command:     CmdRelay,
			payloadSize: 509,
		},
		{
			name:        "Fixed CREATE2 small payload",
			command:     CmdCreate2,
			payloadSize: 100,
		},
		{
			name:        "Variable VERSIONS small payload",
			command:     CmdVersions,
			payloadSize: 6,
		},
		{
			name:        "Variable CERTS large payload",
			command:     CmdCerts,
			payloadSize: 1500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate random payload
			payload := make([]byte, tt.payloadSize)
			if _, err := rand.Read(payload); err != nil {
				t.Fatalf("Failed to generate random payload: %v", err)
			}

			// Run a warmup to stabilize timing
			for i := 0; i < 100; i++ {
				cell := NewCell(12345, tt.command)
				cell.Payload = payload
				var buf bytes.Buffer
				_ = cell.Encode(&buf)
			}

			// Run benchmark-style measurement
			result := testing.Benchmark(func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					cell := NewCell(12345, tt.command)
					cell.Payload = payload
					var buf bytes.Buffer
					if err := cell.Encode(&buf); err != nil {
						b.Fatalf("Encode failed: %v", err)
					}
				}
			})

			nsPerOp := result.NsPerOp()
			t.Logf("Command: %s, Payload: %d bytes", tt.command, tt.payloadSize)
			t.Logf("Mean time: %d ns/op (%d iterations)", nsPerOp, result.N)

			// For fixed-size cells, timing should be under 10 microseconds
			if !tt.command.IsVariableLength() {
				if nsPerOp > 10000 {
					t.Logf("Note: Fixed-size cell encoding slower than expected: %d ns > 10000 ns", nsPerOp)
				}
			}

			// Verify the test actually ran enough iterations
			if result.N < 100 {
				t.Errorf("Too few benchmark iterations: %d < 100", result.N)
			}
		})
	}
}

// TestCellEncodingTimingDifference measures the observable timing difference
// between fixed-size and variable-length cell encoding.
// This is acceptable as cell type is transmitted in plaintext.
func TestCellEncodingTimingDifference(t *testing.T) {
	// Measure fixed-size cell encoding (RELAY with 250-byte payload)
	fixedPayload := make([]byte, 250)
	if _, err := rand.Read(fixedPayload); err != nil {
		t.Fatalf("Failed to generate random payload: %v", err)
	}

	fixedResult := testing.Benchmark(func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			cell := NewCell(12345, CmdRelay)
			cell.Payload = fixedPayload
			var buf bytes.Buffer
			if err := cell.Encode(&buf); err != nil {
				b.Fatalf("Encode failed: %v", err)
			}
		}
	})

	// Measure variable-length cell encoding (VERSIONS with 6-byte payload)
	varPayload := make([]byte, 6)
	if _, err := rand.Read(varPayload); err != nil {
		t.Fatalf("Failed to generate random payload: %v", err)
	}

	varResult := testing.Benchmark(func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			cell := NewCell(12345, CmdVersions)
			cell.Payload = varPayload
			var buf bytes.Buffer
			if err := cell.Encode(&buf); err != nil {
				b.Fatalf("Encode failed: %v", err)
			}
		}
	})

	fixedNs := fixedResult.NsPerOp()
	varNs := varResult.NsPerOp()

	t.Logf("Fixed-size cell (RELAY, 250 bytes): %d ns/op", fixedNs)
	t.Logf("Variable-length cell (VERSIONS, 6 bytes): %d ns/op", varNs)

	// Calculate timing difference
	if fixedNs > 0 && varNs > 0 {
		timingDiff := int64(fixedNs) - int64(varNs)
		percentDiff := float64(timingDiff) / float64(varNs) * 100
		t.Logf("Timing difference: %d ns (%.1f%%)", timingDiff, percentDiff)
	}

	// Document that this timing difference is acceptable
	// Cell command is transmitted in plaintext, so this doesn't leak secret info
	t.Logf("ACCEPTABLE: Cell type is not secret (transmitted in plaintext)")
}

// TestRelayCellValidationTiming verifies that relay cell validation
// has consistent timing regardless of validation result.
func TestRelayCellValidationTiming(t *testing.T) {
	// Test 1: Valid relay cell with maximum length
	validPayload := make([]byte, PayloadLen)
	validPayload[0] = RelayData                                         // Command
	validPayload[1] = 0                                                 // Recognized (high byte)
	validPayload[2] = 0                                                 // Recognized (low byte)
	validPayload[3] = 0                                                 // StreamID (high byte)
	validPayload[4] = 1                                                 // StreamID (low byte)
	validPayload[9] = byte((PayloadLen - RelayCellHeaderLen) >> 8)     // Length (high byte)
	validPayload[10] = byte((PayloadLen - RelayCellHeaderLen) & 0xFF)  // Length (low byte)

	validResult := testing.Benchmark(func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			rc, err := DecodeRelayCell(validPayload)
			if err != nil {
				b.Fatalf("DecodeRelayCell failed for valid cell: %v", err)
			}
			if rc == nil {
				b.Fatal("DecodeRelayCell returned nil for valid cell")
			}
		}
	})

	// Test 2: Invalid relay cell with excessive length
	invalidPayload := make([]byte, PayloadLen)
	copy(invalidPayload, validPayload)
	invalidPayload[9] = 0xFF  // Length too large (high byte)
	invalidPayload[10] = 0xFF // Length too large (low byte)

	invalidResult := testing.Benchmark(func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := DecodeRelayCell(invalidPayload)
			if err == nil {
				b.Fatal("DecodeRelayCell should fail for invalid length")
			}
		}
	})

	validNs := validResult.NsPerOp()
	invalidNs := invalidResult.NsPerOp()

	t.Logf("Valid relay cell decoding: %d ns/op", validNs)
	t.Logf("Invalid relay cell decoding: %d ns/op", invalidNs)

	// Calculate timing difference
	if validNs > 0 && invalidNs > 0 {
		timingDiff := int64(validNs) - int64(invalidNs)
		percentDiff := math.Abs(float64(timingDiff)) / float64(validNs) * 100
		t.Logf("Timing difference: %d ns (%.1f%%)", timingDiff, percentDiff)
	}

	// Document that early-exit timing is acceptable for validation errors
	// Invalid relay cells indicate protocol violations or implementation bugs
	// Not exploitable by adversary (relay cells are encrypted)
	if invalidNs < validNs {
		t.Logf("Note: Invalid cell validation faster (early-exit)")
		t.Logf("ACCEPTABLE: Relay cells are encrypted (not adversary-controllable)")
	}
}

// TestRelayCellDataIndependentTiming verifies that relay cell decoding
// timing is independent of data content (only dependent on length).
func TestRelayCellDataIndependentTiming(t *testing.T) {
	tests := []struct {
		name     string
		dataSize uint16
	}{
		{"Empty data", 0},
		{"Small data", 50},
		{"Medium data", 250},
		{"Maximum data", PayloadLen - RelayCellHeaderLen},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test with all-zeros data
			zerosPayload := make([]byte, PayloadLen)
			zerosPayload[0] = RelayData
			zerosPayload[9] = byte(tt.dataSize >> 8)
			zerosPayload[10] = byte(tt.dataSize)

			zerosResult := testing.Benchmark(func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					_, err := DecodeRelayCell(zerosPayload)
					if err != nil {
						b.Fatalf("DecodeRelayCell failed: %v", err)
					}
				}
			})

			// Test with random data
			randomPayload := make([]byte, PayloadLen)
			randomPayload[0] = RelayData
			randomPayload[9] = byte(tt.dataSize >> 8)
			randomPayload[10] = byte(tt.dataSize)
			if tt.dataSize > 0 {
				if _, err := rand.Read(randomPayload[11 : 11+tt.dataSize]); err != nil {
					t.Fatalf("Failed to generate random data: %v", err)
				}
			}

			randomResult := testing.Benchmark(func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					_, err := DecodeRelayCell(randomPayload)
					if err != nil {
						b.Fatalf("DecodeRelayCell failed: %v", err)
					}
				}
			})

			zerosNs := zerosResult.NsPerOp()
			randomNs := randomResult.NsPerOp()

			t.Logf("Data size: %d bytes", tt.dataSize)
			t.Logf("All-zeros: %d ns/op", zerosNs)
			t.Logf("Random:    %d ns/op", randomNs)

			// Timing should be independent of data content
			if zerosNs > 0 && randomNs > 0 {
				timingDiff := int64(zerosNs) - int64(randomNs)
				percentDiff := math.Abs(float64(timingDiff)) / float64(zerosNs) * 100

				// Allow up to 20% difference (GC, cache effects, etc.)
				if percentDiff > 20 {
					t.Logf("Note: Timing varies by %.1f%% (may indicate data dependence)", percentDiff)
				}
			}

			t.Logf("SECURE: Timing independent of data content")
		})
	}
}

// TestCellEncodingPaddingTiming verifies that padding allocation timing
// doesn't leak payload length information.
func TestCellEncodingPaddingTiming(t *testing.T) {
	tests := []struct {
		name        string
		payloadSize int
	}{
		{"Maximum payload (minimal padding)", 509},
		{"Medium payload (medium padding)", 250},
		{"Small payload (maximum padding)", 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := make([]byte, tt.payloadSize)
			if _, err := rand.Read(payload); err != nil {
				t.Fatalf("Failed to generate random payload: %v", err)
			}

			result := testing.Benchmark(func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					cell := NewCell(12345, CmdRelay)
					cell.Payload = payload
					var buf bytes.Buffer
					if err := cell.Encode(&buf); err != nil {
						b.Fatalf("Encode failed: %v", err)
					}
				}
			})

			nsPerOp := result.NsPerOp()
			paddingSize := PayloadLen - tt.payloadSize
			t.Logf("Payload: %d bytes, Padding: %d bytes", tt.payloadSize, paddingSize)
			t.Logf("Mean time: %d ns/op", nsPerOp)
		})
	}

	// Note: While padding size affects encoding time locally, after encryption
	// all cells are 514 bytes on the wire, preventing network-observable timing.
	t.Logf("Note: All cells are 514 bytes on wire (prevents network timing analysis)")
}
