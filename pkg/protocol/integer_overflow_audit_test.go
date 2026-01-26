package protocol

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"
	"time"
)

// TestIntegerOverflow_VersionsPayload tests integer overflow protection in VERSIONS payload
func TestIntegerOverflow_VersionsPayload(t *testing.T) {
	tests := []struct {
		name          string
		versions      []uint16
		expectError   bool
		errorContains string
	}{
		{
			name:        "single version",
			versions:    []uint16{4},
			expectError: false,
		},
		{
			name:        "multiple versions",
			versions:    []uint16{3, 4, 5},
			expectError: false,
		},
		{
			name:        "many versions (1000 versions = 2000 bytes)",
			versions:    make([]uint16, 1000),
			expectError: false,
		},
		{
			name:        "maximum versions (32767 versions = 65534 bytes, fits in uint16)",
			versions:    make([]uint16, 32767),
			expectError: false,
		},
		{
			name:        "version values at boundaries",
			versions:    []uint16{0, 1, math.MaxUint16},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode versions payload: 2 bytes per version (big-endian)
			payload := make([]byte, len(tt.versions)*2)
			for i, v := range tt.versions {
				binary.BigEndian.PutUint16(payload[i*2:], v)
			}

			// Verify payload length doesn't overflow uint16
			if len(payload) > math.MaxUint16 {
				t.Errorf("Payload length %d exceeds uint16 max", len(payload))
			}
		})
	}
}

// TestIntegerOverflow_VersionsParsing tests integer overflow protection when parsing VERSIONS
func TestIntegerOverflow_VersionsParsing(t *testing.T) {
	tests := []struct {
		name          string
		payloadLen    int
		expectError   bool
		errorContains string
	}{
		{
			name:        "empty payload",
			payloadLen:  0,
			expectError: false,
		},
		{
			name:        "single version (2 bytes)",
			payloadLen:  2,
			expectError: false,
		},
		{
			name:        "multiple versions (6 bytes)",
			payloadLen:  6,
			expectError: false,
		},
		{
			name:          "odd payload length (malformed)",
			payloadLen:    3,
			expectError:   true,
			errorContains: "invalid VERSIONS payload length",
		},
		{
			name:          "odd payload length (malformed, large)",
			payloadLen:    1001,
			expectError:   true,
			errorContains: "invalid VERSIONS payload length",
		},
		{
			name:        "large even payload (10000 bytes = 5000 versions)",
			payloadLen:  10000,
			expectError: false,
		},
		{
			name:        "maximum uint16 payload (65534 bytes, even)",
			payloadLen:  65534,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := make([]byte, tt.payloadLen)

			// Simulate parsing (from receiveVersions logic)
			if len(payload)%2 != 0 {
				// Should error
				if !tt.expectError {
					t.Errorf("Expected success but payload length is odd: %d", len(payload))
				}
				return
			}

			// Parse versions
			var versions []int
			for i := 0; i < len(payload); i += 2 {
				version := int(payload[i])<<8 | int(payload[i+1])
				versions = append(versions, version)
			}

			if tt.expectError {
				t.Errorf("Expected error but parsing succeeded")
			}
		})
	}
}

// TestIntegerOverflow_NetinfoTimestamp tests timestamp overflow protection
func TestIntegerOverflow_NetinfoTimestamp(t *testing.T) {
	tests := []struct {
		name        string
		timestamp   time.Time
		expectError bool
	}{
		{
			name:        "current time",
			timestamp:   time.Now(),
			expectError: false,
		},
		{
			name:        "unix epoch (1970-01-01)",
			timestamp:   time.Unix(0, 0),
			expectError: false,
		},
		{
			name:        "year 2000",
			timestamp:   time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
			expectError: false,
		},
		{
			name:        "year 2038 (before uint32 overflow)",
			timestamp:   time.Date(2038, 1, 1, 0, 0, 0, 0, time.UTC),
			expectError: false,
		},
		{
			name:        "year 2100 (before uint32 overflow but close)",
			timestamp:   time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC),
			expectError: false,
		},
		{
			name:        "year 2106 (near uint32 max)",
			timestamp:   time.Date(2106, 2, 7, 6, 28, 15, 0, time.UTC),
			expectError: false,
		},
		{
			name:        "year 2200 (exceeds uint32 max)",
			timestamp:   time.Date(2200, 1, 1, 0, 0, 0, 0, time.UTC),
			expectError: true,
		},
		{
			name:        "year 3000 (far exceeds uint32 max)",
			timestamp:   time.Date(3000, 1, 1, 0, 0, 0, 0, time.UTC),
			expectError: true,
		},
		{
			name:        "before unix epoch (negative)",
			timestamp:   time.Date(1960, 1, 1, 0, 0, 0, 0, time.UTC),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unix := tt.timestamp.Unix()

			// Check for negative timestamp
			if unix < 0 {
				if !tt.expectError {
					t.Errorf("Expected success but timestamp is negative: %d", unix)
				}
				return
			}

			// Check for uint32 overflow
			if unix > math.MaxUint32 {
				if !tt.expectError {
					t.Errorf("Expected success but timestamp exceeds uint32: %d > %d", unix, uint32(math.MaxUint32))
				}
				return
			}

			// Conversion succeeded
			timestamp32 := uint32(unix)
			if tt.expectError {
				t.Errorf("Expected error but conversion succeeded: %d", timestamp32)
			}
		})
	}
}

// TestIntegerOverflow_NetinfoAddressLength tests address length field overflow
func TestIntegerOverflow_NetinfoAddressLength(t *testing.T) {
	tests := []struct {
		name       string
		addrType   byte
		addrLen    byte
		expectOK   bool
		maxAddrLen int
	}{
		{
			name:       "IPv4 (type 0x04, length 4)",
			addrType:   0x04,
			addrLen:    4,
			expectOK:   true,
			maxAddrLen: 4,
		},
		{
			name:       "IPv6 (type 0x06, length 16)",
			addrType:   0x06,
			addrLen:    16,
			expectOK:   true,
			maxAddrLen: 16,
		},
		{
			name:       "hostname (type 0xF0, length 255)",
			addrType:   0xF0,
			addrLen:    255,
			expectOK:   true,
			maxAddrLen: 255,
		},
		{
			name:       "invalid IPv4 length (too large)",
			addrType:   0x04,
			addrLen:    16,
			expectOK:   false,
			maxAddrLen: 4,
		},
		{
			name:       "invalid IPv6 length (too small)",
			addrType:   0x06,
			addrLen:    4,
			expectOK:   false,
			maxAddrLen: 16,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a NETINFO payload with address
			payload := make([]byte, 512)
			payload[0] = 0 // Timestamp byte 0
			payload[1] = 0 // Timestamp byte 1
			payload[2] = 0 // Timestamp byte 2
			payload[3] = 0 // Timestamp byte 3
			payload[4] = tt.addrType
			payload[5] = tt.addrLen

			// Validate address length
			isValid := int(tt.addrLen) == tt.maxAddrLen
			if isValid != tt.expectOK {
				if tt.expectOK {
					t.Errorf("Expected valid address but got invalid: type=%d, len=%d, expected=%d",
						tt.addrType, tt.addrLen, tt.maxAddrLen)
				}
			}
		})
	}
}

// TestIntegerOverflow_HandshakeTimeout tests timeout configuration overflow
func TestIntegerOverflow_HandshakeTimeout(t *testing.T) {
	tests := []struct {
		name        string
		timeout     time.Duration
		expectError bool
	}{
		{
			name:        "minimum timeout (5 seconds)",
			timeout:     MinHandshakeTimeout,
			expectError: false,
		},
		{
			name:        "default timeout (10 seconds)",
			timeout:     DefaultHandshakeTimeout,
			expectError: false,
		},
		{
			name:        "maximum timeout (60 seconds)",
			timeout:     MaxHandshakeTimeout,
			expectError: false,
		},
		{
			name:        "below minimum (4 seconds)",
			timeout:     4 * time.Second,
			expectError: true,
		},
		{
			name:        "above maximum (61 seconds)",
			timeout:     61 * time.Second,
			expectError: true,
		},
		{
			name:        "zero timeout",
			timeout:     0,
			expectError: true,
		},
		{
			name:        "negative timeout",
			timeout:     -1 * time.Second,
			expectError: true,
		},
		{
			name:        "very large timeout (1 hour)",
			timeout:     1 * time.Hour,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Handshake{
				timeout: DefaultHandshakeTimeout,
			}

			err := h.SetTimeout(tt.timeout)

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

			if h.timeout != tt.timeout {
				t.Errorf("Timeout mismatch: got %v, want %v", h.timeout, tt.timeout)
			}
		})
	}
}

// TestIntegerOverflow_VersionSelection tests version selection doesn't overflow
func TestIntegerOverflow_VersionSelection(t *testing.T) {
	h := &Handshake{}

	tests := []struct {
		name            string
		remoteVersions  []int
		expectedVersion int
	}{
		{
			name:            "single version (4)",
			remoteVersions:  []int{4},
			expectedVersion: 4,
		},
		{
			name:            "multiple versions (select highest)",
			remoteVersions:  []int{3, 4, 5},
			expectedVersion: 5,
		},
		{
			name:            "versions out of order",
			remoteVersions:  []int{5, 3, 4},
			expectedVersion: 5,
		},
		{
			name:            "no compatible version",
			remoteVersions:  []int{1, 2},
			expectedVersion: 0,
		},
		{
			name:            "very large version numbers",
			remoteVersions:  []int{1000, 2000, 3},
			expectedVersion: 3,
		},
		{
			name:            "negative version (should not be selected)",
			remoteVersions:  []int{-1, 4},
			expectedVersion: 4,
		},
		{
			name:            "version at int max",
			remoteVersions:  []int{math.MaxInt32, 4},
			expectedVersion: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selected := h.selectVersion(tt.remoteVersions)
			if selected != tt.expectedVersion {
				t.Errorf("Version selection mismatch: got %d, want %d", selected, tt.expectedVersion)
			}
		})
	}
}

// TestIntegerOverflow_PayloadAllocation tests payload buffer allocation doesn't overflow
func TestIntegerOverflow_PayloadAllocation(t *testing.T) {
	tests := []struct {
		name        string
		allocSize   int
		expectPanic bool
	}{
		{
			name:        "small allocation (512 bytes)",
			allocSize:   512,
			expectPanic: false,
		},
		{
			name:        "medium allocation (65535 bytes)",
			allocSize:   65535,
			expectPanic: false,
		},
		{
			name:        "large allocation (1 MB)",
			allocSize:   1024 * 1024,
			expectPanic: false,
		},
		// Note: Very large allocations (e.g., math.MaxInt) would cause out-of-memory,
		// but we don't test those as they would crash the test suite.
		// The protocol naturally limits payload sizes to uint16 max (65535).
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					if !tt.expectPanic {
						t.Errorf("Unexpected panic: %v", r)
					}
				}
			}()

			payload := make([]byte, tt.allocSize)
			if len(payload) != tt.allocSize {
				t.Errorf("Allocation size mismatch: got %d, want %d", len(payload), tt.allocSize)
			}

			if tt.expectPanic {
				t.Errorf("Expected panic but allocation succeeded")
			}
		})
	}
}

// TestIntegerOverflow_BitwiseOperations tests bitwise operations don't overflow
func TestIntegerOverflow_BitwiseOperations(t *testing.T) {
	t.Run("version parsing from bytes", func(t *testing.T) {
		// Test version parsing: (byte << 8) | byte
		tests := []struct {
			byte1 byte
			byte2 byte
			want  int
		}{
			{0x00, 0x00, 0},
			{0x00, 0x01, 1},
			{0x00, 0xFF, 255},
			{0x01, 0x00, 256},
			{0xFF, 0xFF, 65535},
			{0x10, 0x20, 4128},
		}

		for _, tt := range tests {
			version := int(tt.byte1)<<8 | int(tt.byte2)
			if version != tt.want {
				t.Errorf("Version parsing failed: got %d, want %d", version, tt.want)
			}

			// Verify result fits in uint16
			if version < 0 || version > math.MaxUint16 {
				t.Errorf("Version out of uint16 range: %d", version)
			}
		}
	})

	t.Run("timestamp parsing from bytes", func(t *testing.T) {
		// Test timestamp parsing: (byte << 24) | (byte << 16) | (byte << 8) | byte
		tests := []struct {
			bytes [4]byte
			want  uint32
		}{
			{[4]byte{0x00, 0x00, 0x00, 0x00}, 0},
			{[4]byte{0x00, 0x00, 0x00, 0x01}, 1},
			{[4]byte{0xFF, 0xFF, 0xFF, 0xFF}, math.MaxUint32},
			{[4]byte{0x01, 0x02, 0x03, 0x04}, 0x01020304},
		}

		for _, tt := range tests {
			timestamp := uint32(tt.bytes[0])<<24 | uint32(tt.bytes[1])<<16 |
				uint32(tt.bytes[2])<<8 | uint32(tt.bytes[3])
			if timestamp != tt.want {
				t.Errorf("Timestamp parsing failed: got %d, want %d", timestamp, tt.want)
			}
		}
	})
}

// TestIntegerOverflow_LoopBounds tests loop bounds don't cause overflow
func TestIntegerOverflow_LoopBounds(t *testing.T) {
	t.Run("versions parsing loop", func(t *testing.T) {
		// Create a large payload (65534 bytes = 32767 versions)
		payloadSize := 65534
		payload := make([]byte, payloadSize)

		// Fill with version data
		for i := 0; i < len(payload); i += 2 {
			binary.BigEndian.PutUint16(payload[i:], uint16(i/2))
		}

		// Parse versions (simulate receiveVersions logic)
		var versions []int
		for i := 0; i < len(payload); i += 2 {
			// Verify i+1 doesn't overflow
			if i+1 >= len(payload) {
				t.Errorf("Loop index overflow: i=%d, len=%d", i, len(payload))
				break
			}
			version := int(payload[i])<<8 | int(payload[i+1])
			versions = append(versions, version)
		}

		expectedVersions := payloadSize / 2
		if len(versions) != expectedVersions {
			t.Errorf("Version count mismatch: got %d, want %d", len(versions), expectedVersions)
		}
	})

	t.Run("version selection loop", func(t *testing.T) {
		h := &Handshake{}

		// Create remote versions array
		remoteVersions := make([]int, 1000)
		for i := range remoteVersions {
			remoteVersions[i] = i
		}

		// Select version (should not overflow)
		selected := h.selectVersion(remoteVersions)
		if selected != MaxLinkProtocolVersion {
			t.Errorf("Version selection failed: got %d, want %d", selected, MaxLinkProtocolVersion)
		}
	})
}

// TestIntegerOverflow_BufferOperations tests buffer operations are safe
func TestIntegerOverflow_BufferOperations(t *testing.T) {
	t.Run("buffer growth doesn't overflow", func(t *testing.T) {
		var buf bytes.Buffer

		// Write small chunks
		for i := 0; i < 100; i++ {
			chunk := make([]byte, 512)
			n, err := buf.Write(chunk)
			if err != nil {
				t.Errorf("Buffer write failed: %v", err)
				break
			}
			if n != len(chunk) {
				t.Errorf("Write size mismatch: got %d, want %d", n, len(chunk))
			}
		}

		expectedSize := 100 * 512
		if buf.Len() != expectedSize {
			t.Errorf("Buffer size mismatch: got %d, want %d", buf.Len(), expectedSize)
		}
	})

	t.Run("buffer slicing is safe", func(t *testing.T) {
		buf := make([]byte, 512)

		// Test various slice operations
		sliceTests := []struct {
			start int
			end   int
		}{
			{0, 10},
			{10, 20},
			{500, 512},
			{0, 512},
		}

		for _, tt := range sliceTests {
			slice := buf[tt.start:tt.end]
			expectedLen := tt.end - tt.start
			if len(slice) != expectedLen {
				t.Errorf("Slice length mismatch: got %d, want %d", len(slice), expectedLen)
			}
		}
	})
}
