// Package cell provides types and functions for encoding and decoding Tor protocol cells.
// Tor uses fixed-size (512 bytes) and variable-size cells for communication.
package cell

import (
	"encoding/binary"
	"fmt"
	"io"
)

// Cell size constants from tor-spec.txt
const (
	// CircIDLen is the length of circuit IDs in bytes (4 bytes for link protocol version >= 4)
	CircIDLen = 4
	// CmdLen is the length of the command field
	CmdLen = 1
	// PayloadLen is the length of the payload in fixed-size cells
	PayloadLen = 509
	// CellLen is the total length of a fixed-size cell
	CellLen = CircIDLen + CmdLen + PayloadLen // 514 bytes
)

// Command represents a cell command type
type Command byte

// Cell commands from tor-spec.txt section 3
const (
	// Fixed-size commands
	CmdPadding     Command = 0
	CmdCreate      Command = 1
	CmdCreated     Command = 2
	CmdRelay       Command = 3
	CmdDestroy     Command = 4
	CmdCreateFast  Command = 5
	CmdCreatedFast Command = 6
	CmdVersions    Command = 7
	CmdNetinfo     Command = 8
	CmdRelayEarly  Command = 9
	CmdCreate2     Command = 10
	CmdCreated2    Command = 11

	// Variable-length commands
	CmdVPadding      Command = 128
	CmdCerts         Command = 129
	CmdAuthChallenge Command = 130
	CmdAuthenticate  Command = 131
	CmdAuthorize     Command = 132
)

// Cell represents a Tor protocol cell
type Cell struct {
	CircID  uint32  // Circuit ID
	Command Command // Cell command
	Payload []byte  // Cell payload
}

// IsVariableLength returns true if the command indicates a variable-length cell
func (c Command) IsVariableLength() bool {
	return c >= 128
}

// String returns a human-readable representation of the command
func (c Command) String() string {
	switch c {
	case CmdPadding:
		return "PADDING"
	case CmdCreate:
		return "CREATE"
	case CmdCreated:
		return "CREATED"
	case CmdRelay:
		return "RELAY"
	case CmdDestroy:
		return "DESTROY"
	case CmdCreateFast:
		return "CREATE_FAST"
	case CmdCreatedFast:
		return "CREATED_FAST"
	case CmdVersions:
		return "VERSIONS"
	case CmdNetinfo:
		return "NETINFO"
	case CmdRelayEarly:
		return "RELAY_EARLY"
	case CmdCreate2:
		return "CREATE2"
	case CmdCreated2:
		return "CREATED2"
	case CmdVPadding:
		return "VPADDING"
	case CmdCerts:
		return "CERTS"
	case CmdAuthChallenge:
		return "AUTH_CHALLENGE"
	case CmdAuthenticate:
		return "AUTHENTICATE"
	case CmdAuthorize:
		return "AUTHORIZE"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", c)
	}
}

// NewCell creates a new cell with the given circuit ID and command
func NewCell(circID uint32, cmd Command) *Cell {
	return &Cell{
		CircID:  circID,
		Command: cmd,
		Payload: make([]byte, 0),
	}
}

// Encode writes the cell to the provided writer
func (c *Cell) Encode(w io.Writer) error {
	// Write circuit ID (4 bytes, big-endian)
	if err := binary.Write(w, binary.BigEndian, c.CircID); err != nil {
		return fmt.Errorf("failed to write circuit ID: %w", err)
	}

	// Write command (1 byte)
	if err := binary.Write(w, binary.BigEndian, c.Command); err != nil {
		return fmt.Errorf("failed to write command: %w", err)
	}

	// Handle variable-length cells
	if c.Command.IsVariableLength() {
		// Write payload length (2 bytes, big-endian)
		payloadLen := uint16(len(c.Payload))
		if err := binary.Write(w, binary.BigEndian, payloadLen); err != nil {
			return fmt.Errorf("failed to write payload length: %w", err)
		}
	}

	// Write payload
	if _, err := w.Write(c.Payload); err != nil {
		return fmt.Errorf("failed to write payload: %w", err)
	}

	// Pad fixed-size cells
	if !c.Command.IsVariableLength() {
		padding := PayloadLen - len(c.Payload)
		if padding > 0 {
			paddingBytes := make([]byte, padding)
			if _, err := w.Write(paddingBytes); err != nil {
				return fmt.Errorf("failed to write padding: %w", err)
			}
		}
	}

	return nil
}

// DecodeCell reads a cell from the provided reader
func DecodeCell(r io.Reader) (*Cell, error) {
	cell := &Cell{}

	// Read circuit ID (4 bytes)
	if err := binary.Read(r, binary.BigEndian, &cell.CircID); err != nil {
		return nil, fmt.Errorf("failed to read circuit ID: %w", err)
	}

	// Read command (1 byte)
	if err := binary.Read(r, binary.BigEndian, &cell.Command); err != nil {
		return nil, fmt.Errorf("failed to read command: %w", err)
	}

	// Handle variable-length cells
	if cell.Command.IsVariableLength() {
		// Read payload length (2 bytes)
		var payloadLen uint16
		if err := binary.Read(r, binary.BigEndian, &payloadLen); err != nil {
			return nil, fmt.Errorf("failed to read payload length: %w", err)
		}

		// Read payload
		cell.Payload = make([]byte, payloadLen)
		if _, err := io.ReadFull(r, cell.Payload); err != nil {
			return nil, fmt.Errorf("failed to read variable-length payload: %w", err)
		}
	} else {
		// Fixed-size cell: read entire payload (509 bytes)
		cell.Payload = make([]byte, PayloadLen)
		if _, err := io.ReadFull(r, cell.Payload); err != nil {
			return nil, fmt.Errorf("failed to read fixed-length payload: %w", err)
		}
	}

	return cell, nil
}
