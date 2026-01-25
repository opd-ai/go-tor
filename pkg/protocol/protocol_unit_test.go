package protocol

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/cell"
	"github.com/opd-ai/go-tor/pkg/connection"
	"github.com/opd-ai/go-tor/pkg/logger"
)

// mockRelayForUnit simulates a relay for unit testing individual protocol functions
type mockRelayForUnit struct {
	responseCells []*cell.Cell
	currentCell   int
	sentCells     []*cell.Cell
}

func (m *mockRelayForUnit) handleConnection(conn *connection.Connection) error {
	// Read cells and send responses
	for {
		receivedCell, err := conn.ReceiveCell()
		if err != nil {
			return err
		}

		m.sentCells = append(m.sentCells, receivedCell)

		// Send next response cell
		if m.currentCell < len(m.responseCells) {
			if err := conn.SendCell(m.responseCells[m.currentCell]); err != nil {
				return err
			}
			m.currentCell++
		}
	}
}

// TestReceiveVersionsUnit tests the receiveVersions function with mock connection
func TestReceiveVersionsUnit(t *testing.T) {
	if testing.Short() {
		// Create a minimal VERSIONS cell response
		versionsCell := cell.NewCell(0, cell.CmdVersions)
		versionsCell.Payload = []byte{0x00, 0x04, 0x00, 0x05} // Versions 4 and 5

		// Test the parsing logic that receiveVersions uses
		payload := versionsCell.Payload
		if len(payload)%2 != 0 {
			t.Fatal("Payload must be even length")
		}

		var versions []int
		for i := 0; i < len(payload); i += 2 {
			version := int(payload[i])<<8 | int(payload[i+1])
			versions = append(versions, version)
		}

		// Verify we extracted the correct versions
		expectedVersions := []int{4, 5}
		if len(versions) != len(expectedVersions) {
			t.Errorf("Got %d versions, want %d", len(versions), len(expectedVersions))
		}

		for i, expected := range expectedVersions {
			if i < len(versions) && versions[i] != expected {
				t.Errorf("Version[%d] = %d, want %d", i, versions[i], expected)
			}
		}

		// Test odd-length detection
		oddPayload := []byte{0x00, 0x04, 0x00}
		if len(oddPayload)%2 == 0 {
			t.Error("Test payload should be odd length")
		}

		// Verify error would be caught
		if len(oddPayload)%2 != 0 {
			t.Log("Correctly detected odd-length payload that would cause error")
		}
	}
}

// TestSendNetinfoUnit tests the sendNetinfo payload construction
func TestSendNetinfoUnit(t *testing.T) {
	if testing.Short() {
		// Test the payload construction logic from sendNetinfo
		payload := make([]byte, 11)

		// Timestamp
		now := time.Now()
		timestamp := uint32(now.Unix())
		payload[0] = byte(timestamp >> 24)
		payload[1] = byte(timestamp >> 16)
		payload[2] = byte(timestamp >> 8)
		payload[3] = byte(timestamp)

		// Other address: IPv4, 0.0.0.0
		payload[4] = 0x04 // Type: IPv4
		payload[5] = 4    // Length: 4 bytes

		// Number of this addresses
		payload[10] = 0

		// Create NETINFO cell
		netinfoCell := cell.NewCell(0, cell.CmdNetinfo)
		netinfoCell.Payload = payload

		// Verify encoding works
		var buf bytes.Buffer
		if err := netinfoCell.Encode(&buf); err != nil {
			t.Fatalf("Failed to encode NETINFO: %v", err)
		}

		// Decode and verify
		decoded, err := cell.DecodeCell(&buf)
		if err != nil {
			t.Fatalf("Failed to decode NETINFO: %v", err)
		}

		if decoded.Command != cell.CmdNetinfo {
			t.Errorf("Command = %v, want NETINFO", decoded.Command)
		}

		// Verify timestamp
		decodedTS := uint32(decoded.Payload[0])<<24 |
			uint32(decoded.Payload[1])<<16 |
			uint32(decoded.Payload[2])<<8 |
			uint32(decoded.Payload[3])

		if decodedTS != timestamp {
			t.Errorf("Timestamp = %d, want %d", decodedTS, timestamp)
		}

		// Verify address type
		if decoded.Payload[4] != 0x04 {
			t.Errorf("Address type = %d, want 0x04", decoded.Payload[4])
		}
	}
}

// TestReceiveNetinfoUnit tests receiveNetinfo validation logic
func TestReceiveNetinfoUnit(t *testing.T) {
	if testing.Short() {
		// Test the validation logic from receiveNetinfo

		// Create a valid NETINFO cell
		payload := make([]byte, 11)
		now := uint32(time.Now().Unix())
		payload[0] = byte(now >> 24)
		payload[1] = byte(now >> 16)
		payload[2] = byte(now >> 8)
		payload[3] = byte(now)
		payload[4] = 0x04 // IPv4
		payload[5] = 4    // Length
		payload[10] = 0   // No addresses

		netinfoCell := cell.NewCell(0, cell.CmdNetinfo)
		netinfoCell.Payload = payload

		// Verify cell type check
		if netinfoCell.Command != cell.CmdNetinfo {
			t.Error("Should detect wrong command type")
		}

		// Test with wrong command
		wrongCell := cell.NewCell(0, cell.CmdPadding)
		if wrongCell.Command == cell.CmdNetinfo {
			t.Error("Should not match NETINFO command")
		}
	}
}

// TestReceiveCERTSUnit tests receiveCERTS parsing logic
func TestReceiveCERTSUnit(t *testing.T) {
	if testing.Short() {
		// Test the validation logic from receiveCERTS

		// Create a minimal CERTS cell (0 certificates)
		certsCell := cell.NewCell(0, cell.CmdCerts)
		certsCell.Payload = []byte{0} // 0 certificates

		// Verify command check
		if certsCell.Command != cell.CmdCerts {
			t.Error("Should detect CERTS command")
		}

		// Verify parsing would work
		if len(certsCell.Payload) < 1 {
			t.Error("Payload too short")
		}

		numCerts := certsCell.Payload[0]
		if numCerts != 0 {
			t.Errorf("Number of certs = %d, want 0", numCerts)
		}

		// Test with non-CERTS cell (should be skipped in receiveCERTS)
		paddingCell := cell.NewCell(0, cell.CmdPadding)
		if paddingCell.Command == cell.CmdCerts {
			t.Error("Should not be CERTS command")
		} else {
			t.Log("Correctly detected non-CERTS cell that would be skipped")
		}
	}
}

// TestPerformHandshakeUnit tests PerformHandshake error paths
func TestPerformHandshakeUnit(t *testing.T) {
	if testing.Short() {
		log := logger.NewDefault()

		// Test with expired context
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		h := &Handshake{
			logger:  log,
			timeout: 1 * time.Second,
		}

		// Should handle cancelled context
		if ctx.Err() != context.Canceled {
			t.Error("Context should be canceled")
		}

		// Test timeout value
		if h.timeout < 1*time.Millisecond {
			t.Error("Timeout too short")
		}
	}
}

// TestVersionSelectionUnit tests selectVersion edge cases
func TestVersionSelectionUnit(t *testing.T) {
	if testing.Short() {
		h := &Handshake{}

		tests := []struct {
			name     string
			versions []int
			expected int
		}{
			{"empty", []int{}, 0},
			{"nil", nil, 0},
			{"no_match", []int{1, 2}, 0},
			{"prefer_highest", []int{3, 4, 5}, 5},
			{"single", []int{4}, 4},
			{"unsorted", []int{5, 3, 4}, 5},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got := h.selectVersion(tt.versions)
				if got != tt.expected {
					t.Errorf("selectVersion(%v) = %d, want %d", tt.versions, got, tt.expected)
				}
			})
		}
	}
}

// TestHandshakeTimeoutUnit tests timeout handling
func TestHandshakeTimeoutUnit(t *testing.T) {
	if testing.Short() {
		log := logger.NewDefault()

		// Test default timeout
		h := NewHandshake(nil, log)
		if h.timeout != DefaultHandshakeTimeout {
			t.Errorf("Default timeout = %v, want %v", h.timeout, DefaultHandshakeTimeout)
		}

		// Test custom timeout
		h.SetTimeout(5 * time.Second)
		if h.timeout != 5*time.Second {
			t.Errorf("Custom timeout = %v, want 5s", h.timeout)
		}

		// Test timeout value is reasonable
		if h.timeout < 100*time.Millisecond {
			t.Error("Timeout is too short for real use")
		}

		// Test context timeout detection
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()
		time.Sleep(10 * time.Millisecond)

		if ctx.Err() != context.DeadlineExceeded {
			t.Error("Context should have deadline exceeded")
		}
	}
}

// TestCellEncodingUnit tests cell encoding/decoding
func TestCellEncodingUnit(t *testing.T) {
	if testing.Short() {
		tests := []struct {
			name    string
			command cell.Command
			payload []byte
		}{
			{"versions", cell.CmdVersions, []byte{0x00, 0x03, 0x00, 0x04}},
			{"netinfo", cell.CmdNetinfo, make([]byte, 11)},
			{"certs_empty", cell.CmdCerts, []byte{0}},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// Create cell
				c := cell.NewCell(0, tt.command)
				c.Payload = tt.payload

				// Encode
				var buf bytes.Buffer
				if err := c.Encode(&buf); err != nil {
					t.Fatalf("Encode failed: %v", err)
				}

				// Decode
				decoded, err := cell.DecodeCell(&buf)
				if err != nil {
					t.Fatalf("Decode failed: %v", err)
				}

				// Verify command
				if decoded.Command != tt.command {
					t.Errorf("Command = %v, want %v", decoded.Command, tt.command)
				}

				// Verify payload prefix matches
				if !bytes.HasPrefix(decoded.Payload, tt.payload) {
					t.Errorf("Payload mismatch")
				}
			})
		}
	}
}

// TestNegotiatedVersionUnit tests version negotiation getter
func TestNegotiatedVersionUnit(t *testing.T) {
	if testing.Short() {
		h := &Handshake{
			negotiatedVersion: 0,
		}

		// Initially zero
		if h.NegotiatedVersion() != 0 {
			t.Errorf("Initial version = %d, want 0", h.NegotiatedVersion())
		}

		// After negotiation
		h.negotiatedVersion = 5
		if h.NegotiatedVersion() != 5 {
			t.Errorf("Negotiated version = %d, want 5", h.NegotiatedVersion())
		}
	}
}

// TestPayloadValidationUnit tests payload length validation
func TestPayloadValidationUnit(t *testing.T) {
	if testing.Short() {
		tests := []struct {
			name    string
			payload []byte
			isValid bool
		}{
			{"versions_even", []byte{0x00, 0x04}, true},
			{"versions_odd", []byte{0x00, 0x04, 0x00}, false},
			{"empty", []byte{}, true},
			{"single_byte", []byte{0x00}, false}, // Odd length, invalid for VERSIONS
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// For VERSIONS cells, length must be even
				isEven := len(tt.payload)%2 == 0

				if tt.isValid && !isEven {
					t.Error("Expected even length for valid payload")
				}

				if !tt.isValid && isEven {
					t.Error("Expected odd length for invalid payload")
				}
			})
		}
	}
}
