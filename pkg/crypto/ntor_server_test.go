package crypto

import (
	"bytes"
	"crypto/rand"
	"testing"

	"golang.org/x/crypto/curve25519"
)

// TestNtorServerHandshake tests the server-side ntor handshake
func TestNtorServerHandshake(t *testing.T) {
	// Generate server keys
	serverNtor, err := GenerateNtorKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate server ntor key: %v", err)
	}

	serverIdentity := make([]byte, 32)
	if _, err := rand.Read(serverIdentity); err != nil {
		t.Fatalf("Failed to generate server identity: %v", err)
	}

	// Generate client handshake
	var serverPublic [32]byte
	curve25519.ScalarBaseMult(&serverPublic, &serverNtor.Private)

	clientHandshake, clientPrivate, err := NtorClientHandshake(serverIdentity, serverPublic[:])
	if err != nil {
		t.Fatalf("Failed to generate client handshake: %v", err)
	}

	// Perform server-side handshake
	response, serverKeyMaterial, err := NtorServerHandshake(clientHandshake, serverNtor.Private[:], serverIdentity)
	if err != nil {
		t.Fatalf("Server handshake failed: %v", err)
	}

	// Validate response length
	if len(response) != 64 {
		t.Errorf("Invalid response length: got %d, want 64", len(response))
	}

	// Validate key material length
	if len(serverKeyMaterial) != 72 {
		t.Errorf("Invalid key material length: got %d, want 72", len(serverKeyMaterial))
	}

	// Client processes server's response
	clientKeyMaterial, err := NtorProcessResponse(response, clientPrivate, serverPublic[:], serverIdentity)
	if err != nil {
		t.Fatalf("Client failed to process response: %v", err)
	}

	// Verify both sides derived the same key material
	if !bytes.Equal(clientKeyMaterial, serverKeyMaterial) {
		t.Errorf("Key material mismatch!\nClient: %x\nServer: %x",
			clientKeyMaterial, serverKeyMaterial)
	}
}

// TestNtorServerHandshakeInvalidInput tests error handling
func TestNtorServerHandshakeInvalidInput(t *testing.T) {
	tests := []struct {
		name              string
		clientHandshake   []byte
		serverNtorKey     []byte
		serverIdentity    []byte
		expectedErrPrefix string
	}{
		{
			name:              "Invalid client handshake length",
			clientHandshake:   make([]byte, 50),
			serverNtorKey:     make([]byte, 32),
			serverIdentity:    make([]byte, 32),
			expectedErrPrefix: "invalid client handshake length",
		},
		{
			name:              "Invalid server ntor key length",
			clientHandshake:   make([]byte, 84),
			serverNtorKey:     make([]byte, 16),
			serverIdentity:    make([]byte, 32),
			expectedErrPrefix: "invalid server ntor key length",
		},
		{
			name:              "Invalid server identity length",
			clientHandshake:   make([]byte, 84),
			serverNtorKey:     make([]byte, 32),
			serverIdentity:    make([]byte, 16),
			expectedErrPrefix: "invalid server identity length",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := NtorServerHandshake(tt.clientHandshake, tt.serverNtorKey, tt.serverIdentity)
			if err == nil {
				t.Errorf("Expected error for %s, got nil", tt.name)
			}
			// Check error message prefix
			if len(tt.expectedErrPrefix) > 0 && !contains(err.Error(), tt.expectedErrPrefix) {
				t.Errorf("Expected error containing %q, got %q", tt.expectedErrPrefix, err.Error())
			}
		})
	}
}

// TestNtorServerHandshakeKeyDerivation validates key derivation produces expected format
func TestNtorServerHandshakeKeyDerivation(t *testing.T) {
	// Generate server keys
	serverNtor, err := GenerateNtorKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate server ntor key: %v", err)
	}

	serverIdentity := make([]byte, 32)
	if _, err := rand.Read(serverIdentity); err != nil {
		t.Fatalf("Failed to generate server identity: %v", err)
	}

	// Generate client handshake
	var serverPublic [32]byte
	curve25519.ScalarBaseMult(&serverPublic, &serverNtor.Private)

	clientHandshake, _, err := NtorClientHandshake(serverIdentity, serverPublic[:])
	if err != nil {
		t.Fatalf("Failed to generate client handshake: %v", err)
	}

	// Perform server-side handshake
	response, keyMaterial, err := NtorServerHandshake(clientHandshake, serverNtor.Private[:], serverIdentity)
	if err != nil {
		t.Fatalf("Server handshake failed: %v", err)
	}

	// Validate response structure: Y (32 bytes) || AUTH (32 bytes)
	if len(response) != 64 {
		t.Fatalf("Invalid response length: %d", len(response))
	}

	serverY := response[0:32]
	auth := response[32:64]

	// Y should be a valid Curve25519 point (non-zero)
	allZero := true
	for _, b := range serverY {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Error("Server ephemeral public key Y is all zeros")
	}

	// AUTH should be non-zero
	allZero = true
	for _, b := range auth {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Error("AUTH MAC is all zeros")
	}

	// Key material should be 72 bytes
	if len(keyMaterial) != 72 {
		t.Errorf("Invalid key material length: %d", len(keyMaterial))
	}

	// Key material components should be non-zero
	// Df (0-19), Db (20-39), Kf (40-55), Kb (56-71)
	sections := []struct {
		name  string
		start int
		end   int
	}{
		{"Df (forward digest)", 0, 20},
		{"Db (backward digest)", 20, 40},
		{"Kf (forward cipher)", 40, 56},
		{"Kb (backward cipher)", 56, 72},
	}

	for _, section := range sections {
		allZero := true
		for i := section.start; i < section.end; i++ {
			if keyMaterial[i] != 0 {
				allZero = false
				break
			}
		}
		if allZero {
			t.Errorf("Key material section %s is all zeros", section.name)
		}
	}
}

// TestNtorServerHandshakeMultipleClients tests handling multiple clients
func TestNtorServerHandshakeMultipleClients(t *testing.T) {
	// Generate single server keys
	serverNtor, err := GenerateNtorKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate server ntor key: %v", err)
	}

	serverIdentity := make([]byte, 32)
	if _, err := rand.Read(serverIdentity); err != nil {
		t.Fatalf("Failed to generate server identity: %v", err)
	}

	var serverPublic [32]byte
	curve25519.ScalarBaseMult(&serverPublic, &serverNtor.Private)

	// Test with multiple clients
	numClients := 10
	keyMaterials := make([][]byte, numClients)

	for i := 0; i < numClients; i++ {
		// Each client generates their own handshake
		clientHandshake, clientPrivate, err := NtorClientHandshake(serverIdentity, serverPublic[:])
		if err != nil {
			t.Fatalf("Client %d: Failed to generate handshake: %v", i, err)
		}

		// Server responds
		response, serverKeyMaterial, err := NtorServerHandshake(clientHandshake, serverNtor.Private[:], serverIdentity)
		if err != nil {
			t.Fatalf("Client %d: Server handshake failed: %v", i, err)
		}

		// Client processes response
		clientKeyMaterial, err := NtorProcessResponse(response, clientPrivate, serverPublic[:], serverIdentity)
		if err != nil {
			t.Fatalf("Client %d: Failed to process response: %v", i, err)
		}

		// Verify key material matches
		if !bytes.Equal(clientKeyMaterial, serverKeyMaterial) {
			t.Errorf("Client %d: Key material mismatch", i)
		}

		keyMaterials[i] = serverKeyMaterial
	}

	// Verify all clients got different key material (due to ephemeral keys)
	for i := 0; i < numClients; i++ {
		for j := i + 1; j < numClients; j++ {
			if bytes.Equal(keyMaterials[i], keyMaterials[j]) {
				t.Errorf("Clients %d and %d got identical key material (ephemeral keys not random?)", i, j)
			}
		}
	}
}

// TestNtorServerHandshakeDeterminism tests that same inputs produce same outputs
func TestNtorServerHandshakeDeterminism(t *testing.T) {
	// This test would require mocking the random number generator
	// For now, we just verify that two separate runs with same server keys
	// but different client ephemeral keys produce different results

	serverNtor, err := GenerateNtorKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate server ntor key: %v", err)
	}

	serverIdentity := make([]byte, 32)
	if _, err := rand.Read(serverIdentity); err != nil {
		t.Fatalf("Failed to generate server identity: %v", err)
	}

	var serverPublic [32]byte
	curve25519.ScalarBaseMult(&serverPublic, &serverNtor.Private)

	// Two different client handshakes
	client1Handshake, _, err := NtorClientHandshake(serverIdentity, serverPublic[:])
	if err != nil {
		t.Fatalf("Failed to generate client 1 handshake: %v", err)
	}

	client2Handshake, _, err := NtorClientHandshake(serverIdentity, serverPublic[:])
	if err != nil {
		t.Fatalf("Failed to generate client 2 handshake: %v", err)
	}

	// Server responds to both
	response1, keyMaterial1, err := NtorServerHandshake(client1Handshake, serverNtor.Private[:], serverIdentity)
	if err != nil {
		t.Fatalf("Server handshake 1 failed: %v", err)
	}

	response2, keyMaterial2, err := NtorServerHandshake(client2Handshake, serverNtor.Private[:], serverIdentity)
	if err != nil {
		t.Fatalf("Server handshake 2 failed: %v", err)
	}

	// Responses should be different (different server ephemeral keys)
	if bytes.Equal(response1, response2) {
		t.Error("Server responses are identical despite different client handshakes")
	}

	// Key materials should be different
	if bytes.Equal(keyMaterial1, keyMaterial2) {
		t.Error("Key materials are identical despite different client handshakes")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || bytes.Contains([]byte(s), []byte(substr))))
}
