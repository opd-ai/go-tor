// Package cell provides relay cell functionality for Tor protocol
package cell

import (
	"encoding/binary"
	"fmt"

	"github.com/opd-ai/go-tor/pkg/security"
)

// Relay commands from tor-spec.txt section 6.1
const (
	RelayBegin        byte = 1
	RelayData         byte = 2
	RelayEnd          byte = 3
	RelayConnected    byte = 4
	RelayResolve      byte = 11
	RelayResolved     byte = 12
	RelayBeginDir     byte = 13
	RelayExtend2      byte = 14
	RelayExtended2    byte = 15
	RelayIntroduce1   byte = 32 // INTRODUCE1 cell for onion services
	RelayIntroduce2   byte = 33 // INTRODUCE2 cell for onion services
	RelayRendezvous1  byte = 34 // RENDEZVOUS1 cell for onion services
	RelayRendezvous2  byte = 35 // RENDEZVOUS2 cell for onion services
	RelayIntroEstab   byte = 38 // ESTABLISH_INTRO cell for onion services
	RelayIntroEstdAck byte = 39 // INTRO_ESTABLISHED cell for onion services
)

// RelayCell represents the payload of a RELAY or RELAY_EARLY cell
type RelayCell struct {
	Command    byte    // Relay command
	Recognized uint16  // Must be zero
	StreamID   uint16  // Stream ID
	Digest     [4]byte // Running digest
	Length     uint16  // Length of data
	Data       []byte  // Relay data
}

// RelayCell header size: Command(1) + Recognized(2) + StreamID(2) + Digest(4) + Length(2) = 11 bytes
const RelayCellHeaderLen = 11

// NewRelayCell creates a new relay cell
func NewRelayCell(streamID uint16, cmd byte, data []byte) *RelayCell {
	// Safely convert data length to uint16
	length, err := security.SafeLenToUint16(data)
	if err != nil {
		// Data is too large, truncate to max uint16
		length = 65535
	}

	return &RelayCell{
		Command:    cmd,
		Recognized: 0,
		StreamID:   streamID,
		Digest:     [4]byte{0, 0, 0, 0},
		Length:     length,
		Data:       data,
	}
}

// Encode encodes the relay cell into a byte slice
func (rc *RelayCell) Encode() ([]byte, error) {
	// Maximum relay cell data size
	maxDataLen := PayloadLen - RelayCellHeaderLen
	if len(rc.Data) > maxDataLen {
		return nil, fmt.Errorf("relay cell data too large: %d > %d", len(rc.Data), maxDataLen)
	}

	// Create payload buffer
	payload := make([]byte, PayloadLen)

	// Write header
	payload[0] = rc.Command
	binary.BigEndian.PutUint16(payload[1:3], rc.Recognized)
	binary.BigEndian.PutUint16(payload[3:5], rc.StreamID)
	copy(payload[5:9], rc.Digest[:])
	binary.BigEndian.PutUint16(payload[9:11], rc.Length)

	// Write data
	copy(payload[11:], rc.Data)

	// Rest is zero padding (already initialized to zero)

	return payload, nil
}

// DecodeRelayCell decodes a relay cell from a payload
func DecodeRelayCell(payload []byte) (*RelayCell, error) {
	if len(payload) < RelayCellHeaderLen {
		return nil, fmt.Errorf("payload too short for relay cell: %d < %d", len(payload), RelayCellHeaderLen)
	}

	rc := &RelayCell{
		Command:    payload[0],
		Recognized: binary.BigEndian.Uint16(payload[1:3]),
		StreamID:   binary.BigEndian.Uint16(payload[3:5]),
		Length:     binary.BigEndian.Uint16(payload[9:11]),
	}
	copy(rc.Digest[:], payload[5:9])

	// Validate length - defense in depth (AUDIT-015)
	maxDataLen := uint16(PayloadLen - RelayCellHeaderLen)
	if rc.Length > maxDataLen {
		return nil, fmt.Errorf("relay cell length exceeds maximum: %d > %d", rc.Length, maxDataLen)
	}
	if int(rc.Length) > len(payload)-RelayCellHeaderLen {
		return nil, fmt.Errorf("relay cell data length exceeds payload: %d > %d", rc.Length, len(payload)-RelayCellHeaderLen)
	}

	// Extract data
	if rc.Length > 0 {
		rc.Data = make([]byte, rc.Length)
		copy(rc.Data, payload[11:11+rc.Length])
	}

	return rc, nil
}

// RelayCmdString returns a human-readable string for a relay command
func RelayCmdString(cmd byte) string {
	switch cmd {
	case RelayBegin:
		return "RELAY_BEGIN"
	case RelayData:
		return "RELAY_DATA"
	case RelayEnd:
		return "RELAY_END"
	case RelayConnected:
		return "RELAY_CONNECTED"
	case RelayResolve:
		return "RELAY_RESOLVE"
	case RelayResolved:
		return "RELAY_RESOLVED"
	case RelayBeginDir:
		return "RELAY_BEGIN_DIR"
	case RelayExtend2:
		return "RELAY_EXTEND2"
	case RelayExtended2:
		return "RELAY_EXTENDED2"
	default:
		return fmt.Sprintf("RELAY_UNKNOWN(%d)", cmd)
	}
}
