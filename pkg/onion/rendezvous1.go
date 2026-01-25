// Package onion - RENDEZVOUS1 Cell Construction
// This file implements RENDEZVOUS1 cell construction for onion service hosting
// Following rend-spec-v3.txt §3.3
package onion

import (
	"context"
	"fmt"

	"github.com/opd-ai/go-tor/pkg/cell"
	"github.com/opd-ai/go-tor/pkg/crypto"
)

// Rendezvous1Cell represents a RENDEZVOUS1 cell to send to a rendezvous point
type Rendezvous1Cell struct {
	RendezvousCookie []byte // 20-byte cookie from INTRODUCE2
	HandshakeData    []byte // ntor handshake response (64 bytes: Y || AUTH)
}

// BuildRendezvous1Cell constructs a RENDEZVOUS1 cell for an onion service
//
// This completes the server-side of the ntor handshake to establish
// an end-to-end encrypted connection between the onion service and client.
//
// Parameters:
//   - rendezvousCookie: The 20-byte cookie from INTRODUCE2 cell
//   - clientHandshake: The client's ntor handshake data from INTRODUCE2
//   - serverNtorKey: The onion service's ntor key (private, 32 bytes)
//   - serverIdentity: The onion service's identity key (public, 32 bytes)
//
// Returns:
//   - cell: The constructed RENDEZVOUS1 relay cell
//   - keyMaterial: The derived circuit keys (72 bytes) for stream encryption
//   - err: Error if handshake or cell construction fails
//
// Following rend-spec-v3.txt §3.3
func BuildRendezvous1Cell(rendezvousCookie, clientHandshake, serverNtorKey, serverIdentity []byte, circuitID uint32, streamID uint16) (*cell.RelayCell, []byte, error) {
	// Validate inputs
	if len(rendezvousCookie) != 20 {
		return nil, nil, fmt.Errorf("invalid rendezvous cookie length: %d, expected 20", len(rendezvousCookie))
	}

	if len(clientHandshake) != 84 {
		return nil, nil, fmt.Errorf("invalid client handshake length: %d, expected 84", len(clientHandshake))
	}

	if len(serverNtorKey) != 32 {
		return nil, nil, fmt.Errorf("invalid server ntor key length: %d", len(serverNtorKey))
	}

	if len(serverIdentity) != 32 {
		return nil, nil, fmt.Errorf("invalid server identity length: %d", len(serverIdentity))
	}

	// Perform server-side ntor handshake
	handshakeResponse, keyMaterial, err := crypto.NtorServerHandshake(clientHandshake, serverNtorKey, serverIdentity)
	if err != nil {
		return nil, nil, fmt.Errorf("ntor server handshake failed: %w", err)
	}

	// Build RENDEZVOUS1 cell payload:
	// RENDEZVOUS_COOKIE (20 bytes) || HANDSHAKE_DATA (64 bytes)
	payload := make([]byte, 20+64)
	copy(payload[0:20], rendezvousCookie)
	copy(payload[20:84], handshakeResponse)

	// Create RENDEZVOUS1 relay cell
	rendezvous1 := &cell.RelayCell{
		Command:  cell.RelayRendezvous1,
		StreamID: streamID,
		Data:     payload,
	}

	return rendezvous1, keyMaterial, nil
}

// SendRendezvous1 sends a RENDEZVOUS1 cell on a rendezvous circuit
//
// This is a convenience function that builds and sends the RENDEZVOUS1 cell.
//
// Parameters:
//   - circuit: The circuit to the rendezvous point
//   - circuitID: The circuit ID for the RENDEZVOUS1 cell
//   - rendezvousCookie: The 20-byte cookie from INTRODUCE2
//   - clientHandshake: The client's ntor handshake (84 bytes from INTRODUCE2)
//   - serverNtorKey: The service's ntor private key (32 bytes)
//   - serverIdentity: The service's identity public key (32 bytes)
//
// Returns:
//   - keyMaterial: The derived encryption keys for the stream (72 bytes)
//   - err: Error if sending fails
func SendRendezvous1(circuit CircuitInterface, circuitID uint32, rendezvousCookie, clientHandshake, serverNtorKey, serverIdentity []byte) ([]byte, error) {
	if circuit == nil {
		return nil, fmt.Errorf("circuit is nil")
	}

	// Build RENDEZVOUS1 cell
	// Use stream ID 0 (rendezvous cells don't use stream IDs)
	rendezvous1Cell, keyMaterial, err := BuildRendezvous1Cell(
		rendezvousCookie,
		clientHandshake,
		serverNtorKey,
		serverIdentity,
		circuitID,
		0, // Stream ID 0 for RENDEZVOUS1
	)
	if err != nil {
		return nil, fmt.Errorf("failed to build RENDEZVOUS1 cell: %w", err)
	}

	// Send the cell on the circuit
	err = circuit.SendRelayCell(rendezvous1Cell)
	if err != nil {
		return nil, fmt.Errorf("failed to send RENDEZVOUS1 cell: %w", err)
	}

	return keyMaterial, nil
}

// CircuitInterface defines the minimal interface needed for sending relay cells
type CircuitInterface interface {
	SendRelayCell(cell *cell.RelayCell) error
	ReceiveRelayCell(ctx context.Context) (*cell.RelayCell, error)
	GetID() uint32
}
