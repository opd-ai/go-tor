// Package crypto - Server-side ntor handshake implementation
// This file implements the server side of the ntor handshake for onion services
// Following tor-spec.txt §5.1.4
package crypto

import (
	"crypto/sha256"
	"fmt"
	"io"

	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
)

// NtorServerHandshake performs the server side of the ntor handshake
// This is used by onion services to respond to client connections
//
// Parameters:
//   - clientHandshake: The client's handshake data from CREATE2/EXTEND2 (84 bytes)
//   - serverNtorKey: The server's long-term ntor onion key (private, 32 bytes)
//   - serverIdentity: The server's identity key (public, 32 bytes)
//
// Returns:
//   - response: The handshake response to send (64 bytes: Y || AUTH)
//   - keyMaterial: The derived circuit keys (72 bytes)
//   - err: Error if handshake fails
//
// Implements tor-spec.txt section 5.1.4 (server side)
func NtorServerHandshake(clientHandshake, serverNtorKey, serverIdentity []byte) (response, keyMaterial []byte, err error) {
	// Validate input lengths
	if len(clientHandshake) != 84 {
		return nil, nil, fmt.Errorf("invalid client handshake length: %d, expected 84", len(clientHandshake))
	}
	if len(serverNtorKey) != 32 {
		return nil, nil, fmt.Errorf("invalid server ntor key length: %d", len(serverNtorKey))
	}
	if len(serverIdentity) != 32 {
		return nil, nil, fmt.Errorf("invalid server identity length: %d", len(serverIdentity))
	}

	// Parse client handshake: NODEID (20) || KEYID (32) || CLIENT_PK (32)
	// We already know our identity, so we just need the client's public key X
	var clientPK [32]byte
	copy(clientPK[:], clientHandshake[52:84])

	// Generate ephemeral server keypair (y, Y)
	serverEphemeral, err := GenerateNtorKeyPair()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate server ephemeral key: %w", err)
	}

	// Convert server long-term private key to array
	var serverB [32]byte
	copy(serverB[:], serverNtorKey)

	// Compute shared secrets:
	// EXP(X,y) - Diffie-Hellman with client's ephemeral key
	var sharedXY [32]byte
	curve25519.ScalarMult(&sharedXY, &serverEphemeral.Private, &clientPK)

	// EXP(X,b) - Diffie-Hellman with server's long-term key
	var sharedXB [32]byte
	curve25519.ScalarMult(&sharedXB, &serverB, &clientPK)

	// Build secret_input per tor-spec.txt 5.1.4:
	// secret_input = EXP(X,y) || EXP(X,b) || ID || B || X || Y || PROTOID
	protoid := []byte("ntor-curve25519-sha256-1")

	// Compute server's public key B from private key
	var serverPublic [32]byte
	curve25519.ScalarBaseMult(&serverPublic, &serverB)

	secretInput := make([]byte, 0, 32+32+32+32+32+32+len(protoid))
	secretInput = append(secretInput, sharedXY[:]...)               // EXP(X,y)
	secretInput = append(secretInput, sharedXB[:]...)               // EXP(X,b)
	secretInput = append(secretInput, serverIdentity...)            // ID
	secretInput = append(secretInput, serverPublic[:]...)           // B
	secretInput = append(secretInput, clientPK[:]...)               // X
	secretInput = append(secretInput, serverEphemeral.Public[:]...) // Y
	secretInput = append(secretInput, protoid...)                   // PROTOID

	// Derive verification key for AUTH computation
	verify := []byte("ntor-curve25519-sha256-1:verify")
	hkdfVerify := hkdf.New(sha256.New, secretInput, nil, verify)
	auth := make([]byte, 32)
	if _, err := io.ReadFull(hkdfVerify, auth); err != nil {
		return nil, nil, fmt.Errorf("HKDF verify derivation failed: %w", err)
	}

	// Derive key material for circuit use
	keyInfo := []byte("ntor-curve25519-sha256-1:key_extract")
	hkdfKey := hkdf.New(sha256.New, secretInput, nil, keyInfo)
	keyMaterial = make([]byte, 72) // Tor uses 72 bytes of key material
	if _, err := io.ReadFull(hkdfKey, keyMaterial); err != nil {
		return nil, nil, fmt.Errorf("HKDF key derivation failed: %w", err)
	}

	// Build response: Y || AUTH
	response = make([]byte, 64)
	copy(response[0:32], serverEphemeral.Public[:])
	copy(response[32:64], auth)

	return response, keyMaterial, nil
}
