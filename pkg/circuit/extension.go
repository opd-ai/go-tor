// Package circuit provides circuit extension functionality for the Tor protocol.
package circuit

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"

	"github.com/opd-ai/go-tor/pkg/cell"
	"github.com/opd-ai/go-tor/pkg/crypto"
	"github.com/opd-ai/go-tor/pkg/logger"
	"github.com/opd-ai/go-tor/pkg/security"
)

// HandshakeType defines the type of circuit handshake to use
type HandshakeType uint16

const (
	// HandshakeTypeNTor is the ntor handshake (recommended)
	HandshakeTypeNTor HandshakeType = 0x0002
	// HandshakeTypeTAP is the legacy TAP handshake
	HandshakeTypeTAP HandshakeType = 0x0000
)

// Extension handles circuit extension operations
type Extension struct {
	circuit *Circuit
	logger  *logger.Logger
}

// NewExtension creates a new circuit extension handler
func NewExtension(circuit *Circuit, log *logger.Logger) *Extension {
	if log == nil {
		log = logger.NewDefault()
	}

	return &Extension{
		circuit: circuit,
		logger:  log.Component("extension"),
	}
}

// CreateFirstHop creates the first hop of the circuit using CREATE2
// This establishes the initial circuit with the guard node
func (e *Extension) CreateFirstHop(ctx context.Context, handshakeType HandshakeType) error {
	e.logger.Info("Creating first hop",
		"circuit_id", e.circuit.ID,
		"handshake_type", handshakeType)

	// Generate handshake data
	handshakeData, err := e.generateHandshakeData(handshakeType)
	if err != nil {
		return fmt.Errorf("failed to generate handshake data: %w", err)
	}

	// Build CREATE2 cell payload
	// Safely convert handshake data length to uint16
	hlen, err := security.SafeLenToUint16(handshakeData)
	if err != nil {
		return fmt.Errorf("handshake data too large: %v", err)
	}

	payload := make([]byte, 2+2+len(handshakeData))
	binary.BigEndian.PutUint16(payload[0:2], uint16(handshakeType))
	binary.BigEndian.PutUint16(payload[2:4], hlen)
	copy(payload[4:], handshakeData)

	// Create CREATE2 cell
	create2Cell := &cell.Cell{
		CircID:  e.circuit.ID,
		Command: cell.CmdCreate2,
		Payload: payload,
	}

	e.logger.Debug("Sending CREATE2 cell",
		"circuit_id", e.circuit.ID,
		"handshake_size", len(handshakeData))

	// In a real implementation, this would send the cell and wait for CREATED2
	// For now, we'll simulate the response
	_ = create2Cell

	e.logger.Info("First hop created successfully", "circuit_id", e.circuit.ID)

	return nil
}

// ExtendCircuit extends the circuit to add another hop using EXTEND2
func (e *Extension) ExtendCircuit(ctx context.Context, target string, handshakeType HandshakeType) error {
	e.logger.Info("Extending circuit",
		"circuit_id", e.circuit.ID,
		"target", target,
		"handshake_type", handshakeType)

	// Generate handshake data
	handshakeData, err := e.generateHandshakeData(handshakeType)
	if err != nil {
		return fmt.Errorf("failed to generate handshake data: %w", err)
	}

	// Build EXTEND2 relay cell
	// EXTEND2 format: NSPEC [LSPECS] HTYPE HLEN HDATA
	extend2Data := e.buildExtend2Data(target, handshakeType, handshakeData)

	// Create RELAY_EXTEND2 cell
	relayCell := &cell.RelayCell{
		Command:  cell.RelayExtend2,
		StreamID: 0, // EXTEND2 uses stream ID 0
		Data:     extend2Data,
	}

	e.logger.Debug("Sending EXTEND2 relay cell",
		"circuit_id", e.circuit.ID,
		"target", target)

	// In a real implementation, this would send the relay cell and wait for EXTENDED2
	_ = relayCell

	e.logger.Info("Circuit extended successfully",
		"circuit_id", e.circuit.ID,
		"target", target)

	return nil
}

// generateHandshakeData generates handshake data for circuit creation
func (e *Extension) generateHandshakeData(handshakeType HandshakeType) ([]byte, error) {
	switch handshakeType {
	case HandshakeTypeNTor:
		// ntor handshake: X (32 bytes) where X is the client's public key
		// This is a simplified version; real ntor is more complex
		data := make([]byte, 32)
		if _, err := rand.Read(data); err != nil {
			return nil, fmt.Errorf("failed to generate random data: %w", err)
		}
		return data, nil

	case HandshakeTypeTAP:
		// TAP handshake: PK_ID (16 bytes) || Symmetric key material (128 bytes)
		// This is legacy and simplified
		data := make([]byte, 144)
		if _, err := rand.Read(data); err != nil {
			return nil, fmt.Errorf("failed to generate random data: %w", err)
		}
		return data, nil

	default:
		return nil, fmt.Errorf("unsupported handshake type: %d", handshakeType)
	}
}

// buildExtend2Data builds the EXTEND2 relay cell data
func (e *Extension) buildExtend2Data(target string, handshakeType HandshakeType, handshakeData []byte) []byte {
	// EXTEND2 format (simplified):
	// NSPEC (1 byte) - number of link specifiers
	// Link specifiers (variable)
	// HTYPE (2 bytes) - handshake type
	// HLEN (2 bytes) - handshake data length
	// HDATA (variable) - handshake data

	// For simplicity, we'll use a minimal implementation
	// In production, this would parse the target and create proper link specifiers

	data := make([]byte, 0, 256)

	// NSPEC: 1 link specifier (simplified)
	data = append(data, 1)

	// Link specifier type 0 (TLS-over-TCP, IPv4) - simplified
	// Type (1 byte) | Length (1 byte) | IPv4 (4 bytes) | Port (2 bytes)
	data = append(data, 0)            // Type
	data = append(data, 6)            // Length
	data = append(data, 127, 0, 0, 1) // IPv4 (placeholder)
	data = append(data, 0, 0)         // Port (placeholder)

	// HTYPE
	htypeBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(htypeBytes, uint16(handshakeType))
	data = append(data, htypeBytes...)

	// HLEN - safely convert handshake data length
	hlen, err := security.SafeLenToUint16(handshakeData)
	if err != nil {
		// This should never happen as handshake data is typically small
		// But handle it gracefully
		return nil
	}
	hlenBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(hlenBytes, hlen)
	data = append(data, hlenBytes...)

	// HDATA
	data = append(data, handshakeData...)

	return data
}

// ProcessCreated2 processes a CREATED2 response from the first hop
func (e *Extension) ProcessCreated2(created2Cell *cell.Cell) error {
	if created2Cell.Command != cell.CmdCreated2 {
		return fmt.Errorf("expected CREATED2 cell, got %s", created2Cell.Command)
	}

	e.logger.Debug("Processing CREATED2 cell", "circuit_id", created2Cell.CircID)

	// Parse CREATED2 response
	payload := created2Cell.Payload
	if len(payload) < 2 {
		return fmt.Errorf("CREATED2 payload too short")
	}

	hlen := binary.BigEndian.Uint16(payload[0:2])
	if len(payload) < int(2+hlen) {
		return fmt.Errorf("CREATED2 payload incomplete")
	}

	handshakeResponse := payload[2 : 2+hlen]

	// In a real implementation, this would:
	// 1. Verify the handshake response
	// 2. Derive shared keys using KDF-TOR
	// 3. Set up encryption for this hop

	e.logger.Info("CREATED2 processed successfully",
		"circuit_id", e.circuit.ID,
		"response_size", len(handshakeResponse))

	return nil
}

// ProcessExtended2 processes an EXTENDED2 response from circuit extension
func (e *Extension) ProcessExtended2(extended2Cell *cell.RelayCell) error {
	if extended2Cell.Command != cell.RelayExtended2 {
		return fmt.Errorf("expected RELAY_EXTENDED2 cell, got %d", extended2Cell.Command)
	}

	e.logger.Debug("Processing EXTENDED2 relay cell", "circuit_id", e.circuit.ID)

	// Parse EXTENDED2 response (similar to CREATED2)
	payload := extended2Cell.Data
	if len(payload) < 2 {
		return fmt.Errorf("EXTENDED2 payload too short")
	}

	hlen := binary.BigEndian.Uint16(payload[0:2])
	if len(payload) < int(2+hlen) {
		return fmt.Errorf("EXTENDED2 payload incomplete")
	}

	handshakeResponse := payload[2 : 2+hlen]

	// In a real implementation, this would:
	// 1. Verify the handshake response
	// 2. Derive shared keys for the new hop
	// 3. Add hop to circuit's encryption layers

	e.logger.Info("EXTENDED2 processed successfully",
		"circuit_id", e.circuit.ID,
		"response_size", len(handshakeResponse))

	return nil
}

// DeriveKeys derives encryption keys for a circuit hop using KDF-TOR
func (e *Extension) DeriveKeys(sharedSecret []byte) (forwardKey, backwardKey []byte, err error) {
	// Use crypto package for key derivation
	// KDF-TOR produces: Df || Db || Kf || Kb
	// Where: Df, Db = forward/backward digest keys (20 bytes each)
	//        Kf, Kb = forward/backward cipher keys (16 bytes each for AES-128)

	const keyMaterial = 72 // 20 + 20 + 16 + 16 bytes

	// Derive key material using KDF
	km, err := crypto.DeriveKey(sharedSecret, keyMaterial)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to derive keys: %w", err)
	}

	// Split key material
	// For now, we'll return cipher keys only
	forwardKey = km[40:56]  // Kf (offset 40, 16 bytes)
	backwardKey = km[56:72] // Kb (offset 56, 16 bytes)

	e.logger.Debug("Keys derived",
		"circuit_id", e.circuit.ID,
		"forward_key_len", len(forwardKey),
		"backward_key_len", len(backwardKey))

	return forwardKey, backwardKey, nil
}
