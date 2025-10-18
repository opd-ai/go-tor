package cell

import (
	"bytes"
	"testing"
)

func TestCommandIsVariableLength(t *testing.T) {
	tests := []struct {
		cmd      Command
		expected bool
	}{
		{CmdPadding, false},
		{CmdCreate, false},
		{CmdRelay, false},
		{CmdVPadding, true},
		{CmdCerts, true},
		{Command(200), true},
	}
	
	for _, tt := range tests {
		t.Run(tt.cmd.String(), func(t *testing.T) {
			if got := tt.cmd.IsVariableLength(); got != tt.expected {
				t.Errorf("IsVariableLength() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCommandString(t *testing.T) {
	tests := []struct {
		cmd      Command
		expected string
	}{
		{CmdPadding, "PADDING"},
		{CmdCreate, "CREATE"},
		{CmdCreated, "CREATED"},
		{CmdRelay, "RELAY"},
		{CmdDestroy, "DESTROY"},
		{Command(255), "UNKNOWN(255)"},
	}
	
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.cmd.String(); got != tt.expected {
				t.Errorf("String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNewCell(t *testing.T) {
	circID := uint32(12345)
	cmd := CmdCreate
	
	cell := NewCell(circID, cmd)
	
	if cell.CircID != circID {
		t.Errorf("CircID = %v, want %v", cell.CircID, circID)
	}
	if cell.Command != cmd {
		t.Errorf("Command = %v, want %v", cell.Command, cmd)
	}
	if cell.Payload == nil {
		t.Error("Payload is nil, want non-nil slice")
	}
}

func TestCellEncodeDecodeFixedSize(t *testing.T) {
	original := &Cell{
		CircID:  12345,
		Command: CmdCreate,
		Payload: []byte{1, 2, 3, 4, 5},
	}
	
	var buf bytes.Buffer
	if err := original.Encode(&buf); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	
	// Fixed-size cell should be exactly CellLen bytes
	if buf.Len() != CellLen {
		t.Errorf("Encoded cell length = %v, want %v", buf.Len(), CellLen)
	}
	
	decoded, err := DecodeCell(&buf)
	if err != nil {
		t.Fatalf("DecodeCell() error = %v", err)
	}
	
	if decoded.CircID != original.CircID {
		t.Errorf("CircID = %v, want %v", decoded.CircID, original.CircID)
	}
	if decoded.Command != original.Command {
		t.Errorf("Command = %v, want %v", decoded.Command, original.Command)
	}
	if len(decoded.Payload) != PayloadLen {
		t.Errorf("Payload length = %v, want %v", len(decoded.Payload), PayloadLen)
	}
	// Check that the actual data matches (first 5 bytes)
	for i := 0; i < 5; i++ {
		if decoded.Payload[i] != original.Payload[i] {
			t.Errorf("Payload[%d] = %v, want %v", i, decoded.Payload[i], original.Payload[i])
		}
	}
}

func TestCellEncodeDecodeVariableLength(t *testing.T) {
	original := &Cell{
		CircID:  67890,
		Command: CmdCerts,
		Payload: []byte{10, 20, 30, 40, 50, 60, 70, 80, 90, 100},
	}
	
	var buf bytes.Buffer
	if err := original.Encode(&buf); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	
	// Variable-length cell: CircID(4) + Cmd(1) + Len(2) + Payload(10) = 17 bytes
	expectedLen := CircIDLen + CmdLen + 2 + len(original.Payload)
	if buf.Len() != expectedLen {
		t.Errorf("Encoded cell length = %v, want %v", buf.Len(), expectedLen)
	}
	
	decoded, err := DecodeCell(&buf)
	if err != nil {
		t.Fatalf("DecodeCell() error = %v", err)
	}
	
	if decoded.CircID != original.CircID {
		t.Errorf("CircID = %v, want %v", decoded.CircID, original.CircID)
	}
	if decoded.Command != original.Command {
		t.Errorf("Command = %v, want %v", decoded.Command, original.Command)
	}
	if !bytes.Equal(decoded.Payload, original.Payload) {
		t.Errorf("Payload = %v, want %v", decoded.Payload, original.Payload)
	}
}

func TestCellEncodeDecodePadding(t *testing.T) {
	// Test that padding cell works correctly
	original := &Cell{
		CircID:  0,
		Command: CmdPadding,
		Payload: []byte{},
	}
	
	var buf bytes.Buffer
	if err := original.Encode(&buf); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	
	if buf.Len() != CellLen {
		t.Errorf("Encoded padding cell length = %v, want %v", buf.Len(), CellLen)
	}
	
	decoded, err := DecodeCell(&buf)
	if err != nil {
		t.Fatalf("DecodeCell() error = %v", err)
	}
	
	if decoded.CircID != 0 {
		t.Errorf("CircID = %v, want 0", decoded.CircID)
	}
	if decoded.Command != CmdPadding {
		t.Errorf("Command = %v, want %v", decoded.Command, CmdPadding)
	}
}
