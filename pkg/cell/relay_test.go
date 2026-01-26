package cell

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewRelayCell(t *testing.T) {
	streamID := uint16(42)
	cmd := RelayBegin
	data := []byte("test data")

	rc, err := NewRelayCell(streamID, cmd, data)
	if err != nil {
		t.Fatalf("NewRelayCell() error = %v", err)
	}

	if rc.StreamID != streamID {
		t.Errorf("StreamID = %v, want %v", rc.StreamID, streamID)
	}
	if rc.Command != cmd {
		t.Errorf("Command = %v, want %v", rc.Command, cmd)
	}
	if rc.Length != uint16(len(data)) {
		t.Errorf("Length = %v, want %v", rc.Length, len(data))
	}
	if !bytes.Equal(rc.Data, data) {
		t.Errorf("Data = %v, want %v", rc.Data, data)
	}
}

func TestRelayCellEncodeDecode(t *testing.T) {
	tests := []struct {
		name     string
		streamID uint16
		cmd      byte
		data     []byte
	}{
		{"empty data", 1, RelayBegin, []byte{}},
		{"small data", 2, RelayData, []byte("hello")},
		{"larger data", 3, RelayEnd, bytes.Repeat([]byte("x"), 100)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original, err := NewRelayCell(tt.streamID, tt.cmd, tt.data)
			if err != nil {
				t.Fatalf("NewRelayCell() error = %v", err)
			}

			encoded, err := original.Encode()
			if err != nil {
				t.Fatalf("Encode() error = %v", err)
			}

			if len(encoded) != PayloadLen {
				t.Errorf("Encoded length = %v, want %v", len(encoded), PayloadLen)
			}

			decoded, err := DecodeRelayCell(encoded)
			if err != nil {
				t.Fatalf("DecodeRelayCell() error = %v", err)
			}

			if decoded.Command != original.Command {
				t.Errorf("Command = %v, want %v", decoded.Command, original.Command)
			}
			if decoded.StreamID != original.StreamID {
				t.Errorf("StreamID = %v, want %v", decoded.StreamID, original.StreamID)
			}
			if decoded.Length != original.Length {
				t.Errorf("Length = %v, want %v", decoded.Length, original.Length)
			}
			if !bytes.Equal(decoded.Data, original.Data) {
				t.Errorf("Data = %v, want %v", decoded.Data, original.Data)
			}
		})
	}
}

func TestRelayCellEncodeTooLarge(t *testing.T) {
	maxDataLen := PayloadLen - RelayCellHeaderLen
	tooLargeData := make([]byte, maxDataLen+1)

	// NewRelayCell should now reject data that's too large
	_, err := NewRelayCell(1, RelayData, tooLargeData)
	if err == nil {
		t.Error("NewRelayCell() expected error for data too large, got nil")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("Expected 'too large' error, got: %v", err)
	}
}

func TestNewRelayCellDataTooLarge(t *testing.T) {
	// Test data larger than uint16 max (65535 bytes)
	tooLargeData := make([]byte, 65536)

	_, err := NewRelayCell(1, RelayData, tooLargeData)
	if err == nil {
		t.Error("NewRelayCell() expected error for data > 65535 bytes, got nil")
	}
}

func TestNewRelayCellMaxSize(t *testing.T) {
	// Test data at exactly uint16 max (65535 bytes)
	// NewRelayCell should reject this as it exceeds PayloadLen - RelayCellHeaderLen (498 bytes)
	maxData := make([]byte, 65535)

	_, err := NewRelayCell(1, RelayData, maxData)
	if err == nil {
		t.Error("NewRelayCell() expected error for 65535 bytes (exceeds max relay data), got nil")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("Expected 'too large' error, got: %v", err)
	}
}

func TestDecodeRelayCellTooShort(t *testing.T) {
	shortPayload := make([]byte, RelayCellHeaderLen-1)

	_, err := DecodeRelayCell(shortPayload)
	if err == nil {
		t.Error("DecodeRelayCell() expected error for short payload, got nil")
	}
}

func TestDecodeRelayCellInvalidLength(t *testing.T) {
	payload := make([]byte, PayloadLen)
	// Set length to exceed available space
	payload[9] = 0xFF  // Length high byte
	payload[10] = 0xFF // Length low byte

	_, err := DecodeRelayCell(payload)
	if err == nil {
		t.Error("DecodeRelayCell() expected error for invalid length, got nil")
	}
}

func TestRelayCmdString(t *testing.T) {
	tests := []struct {
		cmd      byte
		expected string
	}{
		{RelayBegin, "RELAY_BEGIN"},
		{RelayData, "RELAY_DATA"},
		{RelayEnd, "RELAY_END"},
		{RelayConnected, "RELAY_CONNECTED"},
		{RelayResolve, "RELAY_RESOLVE"},
		{RelayResolved, "RELAY_RESOLVED"},
		{RelayBeginDir, "RELAY_BEGIN_DIR"},
		{RelayExtend2, "RELAY_EXTEND2"},
		{RelayExtended2, "RELAY_EXTENDED2"},
		{255, "RELAY_UNKNOWN(255)"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := RelayCmdString(tt.cmd); got != tt.expected {
				t.Errorf("RelayCmdString() = %v, want %v", got, tt.expected)
			}
		})
	}
}
