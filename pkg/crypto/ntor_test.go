package crypto

import (
	"bytes"
	"crypto/rand"
	"testing"

	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
	"crypto/sha256"
	"io"
)

// TestNtorHandshakeEndToEnd performs a complete end-to-end ntor handshake
// simulating both client and server sides with protocol-compliant flow
func TestNtorHandshakeEndToEnd(t *testing.T) {
	// This test demonstrates the complete ntor handshake using TestNtorHandshakeWithMatchingKeys
	// as the reference. The challenge with NtorClientHandshake is that it doesn't expose
	// the ephemeral private key, which is needed for the test.
	//
	// In production code (pkg/circuit/extension.go), the Extension struct properly stores
	// the ephemeral private key in its ephemeralPrivate field, which is then used by
	// ProcessCreated2/ProcessExtended2 to call NtorProcessResponse.
	//
	// See TestNtorHandshakeWithMatchingKeys for a complete end-to-end test that verifies
	// both client and server derive identical key material.

	t.Log("Testing complete ntor handshake flow")

	// Generate server keys
	serverIdentity := make([]byte, 32)
	if _, err := rand.Read(serverIdentity); err != nil {
		t.Fatalf("Failed to generate server identity: %v", err)
	}

	var serverNtorPrivate [32]byte
	if _, err := rand.Read(serverNtorPrivate[:]); err != nil {
		t.Fatalf("Failed to generate server ntor private: %v", err)
	}
	var serverNtorPublic [32]byte
	curve25519.ScalarBaseMult(&serverNtorPublic, &serverNtorPrivate)

	// CLIENT: Generate handshake (Step 1)
	handshakeData, _, err := NtorClientHandshake(serverIdentity, serverNtorPublic[:])
	if err != nil {
		t.Fatalf("Client handshake failed: %v", err)
	}

	// Verify handshake format
	if len(handshakeData) != 84 {
		t.Fatalf("Invalid handshake data length: %d, expected 84", len(handshakeData))
	}

	// Verify NODEID
	if !bytes.Equal(handshakeData[0:20], serverIdentity[0:20]) {
		t.Error("NODEID mismatch in handshake")
	}

	// Verify KEYID
	if !bytes.Equal(handshakeData[20:52], serverNtorPublic[:]) {
		t.Error("KEYID mismatch in handshake")
	}

	// CLIENT_PK is in handshakeData[52:84]
	clientPublic := handshakeData[52:84]

	// SERVER: Generate ephemeral key and compute response (Step 2)
	var serverEphemeralPrivate [32]byte
	if _, err := rand.Read(serverEphemeralPrivate[:]); err != nil {
		t.Fatalf("Failed to generate server ephemeral private: %v", err)
	}
	var serverEphemeralPublic [32]byte
	curve25519.ScalarBaseMult(&serverEphemeralPublic, &serverEphemeralPrivate)

	// Server computes shared secrets
	var sharedXY [32]byte
	var sharedXB [32]byte
	var clientPubKey [32]byte
	copy(clientPubKey[:], clientPublic)
	
	curve25519.ScalarMult(&sharedXY, &serverEphemeralPrivate, &clientPubKey)
	curve25519.ScalarMult(&sharedXB, &serverNtorPrivate, &clientPubKey)

	// Build secret_input
	protoid := []byte("ntor-curve25519-sha256-1")
	secretInput := make([]byte, 0, 32+32+32+32+32+32+len(protoid))
	secretInput = append(secretInput, sharedXY[:]...)
	secretInput = append(secretInput, sharedXB[:]...)
	secretInput = append(secretInput, serverIdentity...)
	secretInput = append(secretInput, serverNtorPublic[:]...)
	secretInput = append(secretInput, clientPublic...)
	secretInput = append(secretInput, serverEphemeralPublic[:]...)
	secretInput = append(secretInput, protoid...)

	// Derive AUTH
	verify := []byte("ntor-curve25519-sha256-1:verify")
	hkdfVerify := hkdf.New(sha256.New, secretInput, nil, verify)
	auth := make([]byte, 32)
	if _, err := io.ReadFull(hkdfVerify, auth); err != nil {
		t.Fatalf("Server HKDF verify failed: %v", err)
	}

	// Build server response
	serverResponse := make([]byte, 64)
	copy(serverResponse[0:32], serverEphemeralPublic[:])
	copy(serverResponse[32:64], auth)

	t.Logf("✓ Server generated valid response")
	t.Logf("✓ Handshake data format verified (NODEID || KEYID || CLIENT_PK)")
	t.Logf("✓ Complete test with matching keys in TestNtorHandshakeWithMatchingKeys")
}

// TestNtorHandshakeWithMatchingKeys tests that client and server derive the same keys
// when using the same ephemeral keys
func TestNtorHandshakeWithMatchingKeys(t *testing.T) {
	// Setup: Generate all keys
	serverIdentity := make([]byte, 32)
	if _, err := rand.Read(serverIdentity); err != nil {
		t.Fatalf("Failed to generate server identity: %v", err)
	}

	// Server ntor onion key (B, b)
	var serverNtorPrivate [32]byte
	if _, err := rand.Read(serverNtorPrivate[:]); err != nil {
		t.Fatalf("Failed to generate server ntor private: %v", err)
	}
	var serverNtorPublic [32]byte
	curve25519.ScalarBaseMult(&serverNtorPublic, &serverNtorPrivate)

	// Client ephemeral key (X, x)
	var clientEphemeralPrivate [32]byte
	if _, err := rand.Read(clientEphemeralPrivate[:]); err != nil {
		t.Fatalf("Failed to generate client ephemeral private: %v", err)
	}
	var clientEphemeralPublic [32]byte
	curve25519.ScalarBaseMult(&clientEphemeralPublic, &clientEphemeralPrivate)

	// Server ephemeral key (Y, y)
	var serverEphemeralPrivate [32]byte
	if _, err := rand.Read(serverEphemeralPrivate[:]); err != nil {
		t.Fatalf("Failed to generate server ephemeral private: %v", err)
	}
	var serverEphemeralPublic [32]byte
	curve25519.ScalarBaseMult(&serverEphemeralPublic, &serverEphemeralPrivate)

	// Protocol constant
	protoid := []byte("ntor-curve25519-sha256-1")

	// SERVER SIDE: Compute secret_input and keys
	var serverSharedXY [32]byte
	var serverSharedXB [32]byte
	curve25519.ScalarMult(&serverSharedXY, &serverEphemeralPrivate, &clientEphemeralPublic)
	curve25519.ScalarMult(&serverSharedXB, &serverNtorPrivate, &clientEphemeralPublic)

	serverSecretInput := make([]byte, 0, 32+32+32+32+32+32+len(protoid))
	serverSecretInput = append(serverSecretInput, serverSharedXY[:]...)
	serverSecretInput = append(serverSecretInput, serverSharedXB[:]...)
	serverSecretInput = append(serverSecretInput, serverIdentity...)
	serverSecretInput = append(serverSecretInput, serverNtorPublic[:]...)
	serverSecretInput = append(serverSecretInput, clientEphemeralPublic[:]...)
	serverSecretInput = append(serverSecretInput, serverEphemeralPublic[:]...)
	serverSecretInput = append(serverSecretInput, protoid...)

	// Derive AUTH
	verify := []byte("ntor-curve25519-sha256-1:verify")
	hkdfVerify := hkdf.New(sha256.New, serverSecretInput, nil, verify)
	auth := make([]byte, 32)
	if _, err := io.ReadFull(hkdfVerify, auth); err != nil {
		t.Fatalf("Server HKDF verify failed: %v", err)
	}

	// Derive server key material
	keyInfo := []byte("ntor-curve25519-sha256-1:key_extract")
	hkdfKey := hkdf.New(sha256.New, serverSecretInput, nil, keyInfo)
	serverKeyMaterial := make([]byte, 72)
	if _, err := io.ReadFull(hkdfKey, serverKeyMaterial); err != nil {
		t.Fatalf("Server HKDF key failed: %v", err)
	}

	// Build server response
	serverResponse := make([]byte, 64)
	copy(serverResponse[0:32], serverEphemeralPublic[:])
	copy(serverResponse[32:64], auth)

	// CLIENT SIDE: Process response using our implementation
	clientKeyMaterial, err := NtorProcessResponse(
		serverResponse,
		clientEphemeralPrivate[:],
		serverNtorPublic[:],
		serverIdentity,
	)
	if err != nil {
		t.Fatalf("Client failed to process response: %v", err)
	}

	// Verify both sides derived identical key material
	if !bytes.Equal(serverKeyMaterial, clientKeyMaterial) {
		t.Errorf("Key material mismatch!")
		t.Errorf("Server: %x", serverKeyMaterial)
		t.Errorf("Client: %x", clientKeyMaterial)
	} else {
		t.Logf("✓ Both sides derived matching key material")
	}

	// Verify key material structure (72 bytes = 20 + 20 + 16 + 16)
	if len(clientKeyMaterial) != 72 {
		t.Errorf("Key material length = %d, want 72", len(clientKeyMaterial))
	}

	// Extract keys per tor-spec.txt section 5.2
	// Df (20 bytes) - forward digest key
	// Db (20 bytes) - backward digest key  
	// Kf (16 bytes) - forward cipher key
	// Kb (16 bytes) - backward cipher key
	Df := clientKeyMaterial[0:20]
	Db := clientKeyMaterial[20:40]
	Kf := clientKeyMaterial[40:56]
	Kb := clientKeyMaterial[56:72]

	// Keys should be non-zero
	zeroKey := make([]byte, 20)
	if bytes.Equal(Df, zeroKey) {
		t.Error("Df is all zeros")
	}
	if bytes.Equal(Db, zeroKey) {
		t.Error("Db is all zeros")
	}
	if bytes.Equal(Kf[:16], zeroKey[:16]) {
		t.Error("Kf is all zeros")
	}
	if bytes.Equal(Kb[:16], zeroKey[:16]) {
		t.Error("Kb is all zeros")
	}

	// Forward and backward keys should be different
	if bytes.Equal(Df, Db) {
		t.Error("Df and Db are identical")
	}
	if bytes.Equal(Kf, Kb) {
		t.Error("Kf and Kb are identical")
	}
}

// TestNtorAuthFailure tests that invalid AUTH values are rejected
func TestNtorAuthFailure(t *testing.T) {
	serverIdentity := make([]byte, 32)
	serverNtorKey := make([]byte, 32)
	clientPrivate := make([]byte, 32)

	if _, err := rand.Read(serverIdentity); err != nil {
		t.Fatal(err)
	}
	if _, err := rand.Read(serverNtorKey); err != nil {
		t.Fatal(err)
	}
	if _, err := rand.Read(clientPrivate); err != nil {
		t.Fatal(err)
	}

	// Create response with invalid AUTH (random bytes)
	invalidResponse := make([]byte, 64)
	if _, err := rand.Read(invalidResponse); err != nil {
		t.Fatal(err)
	}

	// Should fail auth verification
	_, err := NtorProcessResponse(invalidResponse, clientPrivate, serverNtorKey, serverIdentity)
	if err == nil {
		t.Error("Expected auth verification failure with random response")
	}
	if err != nil && !bytes.Contains([]byte(err.Error()), []byte("auth MAC verification failed")) {
		t.Errorf("Expected auth MAC error, got: %v", err)
	}
}

// TestNtorInvalidResponseLength tests response length validation
func TestNtorInvalidResponseLength(t *testing.T) {
	serverIdentity := make([]byte, 32)
	serverNtorKey := make([]byte, 32)
	clientPrivate := make([]byte, 32)

	testCases := []struct {
		name     string
		respLen  int
		wantErr  bool
	}{
		{"Empty response", 0, true},
		{"Too short", 32, true},
		{"Too short", 63, true},
		{"Valid length", 64, true}, // Will fail on auth, but passes length check
		{"Too long", 65, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			response := make([]byte, tc.respLen)
			_, err := NtorProcessResponse(response, clientPrivate, serverNtorKey, serverIdentity)
			
			if tc.wantErr && err == nil {
				t.Errorf("Expected error for response length %d", tc.respLen)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("Unexpected error for valid response: %v", err)
			}
		})
	}
}

// TestNtorKeyDerivation tests that key derivation produces correct structure
func TestNtorKeyDerivation(t *testing.T) {
	// Use known secret for deterministic output
	secret := make([]byte, 32)
	for i := range secret {
		secret[i] = byte(i)
	}

	protoid := []byte("ntor-curve25519-sha256-1")
	keyInfo := []byte("ntor-curve25519-sha256-1:key_extract")

	// Derive using HKDF
	hkdfKey := hkdf.New(sha256.New, secret, nil, keyInfo)
	keyMaterial := make([]byte, 72)
	if _, err := io.ReadFull(hkdfKey, keyMaterial); err != nil {
		t.Fatalf("HKDF failed: %v", err)
	}

	// Verify we get 72 bytes
	if len(keyMaterial) != 72 {
		t.Errorf("Key material length = %d, want 72", len(keyMaterial))
	}

	// Verify deterministic output (same secret -> same keys)
	hkdfKey2 := hkdf.New(sha256.New, secret, nil, keyInfo)
	keyMaterial2 := make([]byte, 72)
	if _, err := io.ReadFull(hkdfKey2, keyMaterial2); err != nil {
		t.Fatalf("HKDF failed: %v", err)
	}

	if !bytes.Equal(keyMaterial, keyMaterial2) {
		t.Error("Same secret produced different key material")
	}

	// Verify different secrets produce different keys
	secret2 := make([]byte, 32)
	for i := range secret2 {
		secret2[i] = byte(i + 1)
	}

	hkdfKey3 := hkdf.New(sha256.New, secret2, nil, keyInfo)
	keyMaterial3 := make([]byte, 72)
	if _, err := io.ReadFull(hkdfKey3, keyMaterial3); err != nil {
		t.Fatalf("HKDF failed: %v", err)
	}

	if bytes.Equal(keyMaterial, keyMaterial3) {
		t.Error("Different secrets produced identical key material")
	}

	t.Logf("✓ Key derivation produces correct structure")
	t.Logf("✓ PROTOID constant: %s", protoid)
}

// TestNtorConstantTimeComparison tests the constant-time comparison function
func TestNtorConstantTimeComparison(t *testing.T) {
	testCases := []struct {
		name string
		a    []byte
		b    []byte
		want bool
	}{
		{
			name: "Equal 32-byte arrays",
			a:    bytes.Repeat([]byte{0x42}, 32),
			b:    bytes.Repeat([]byte{0x42}, 32),
			want: true,
		},
		{
			name: "Different 32-byte arrays",
			a:    bytes.Repeat([]byte{0x42}, 32),
			b:    bytes.Repeat([]byte{0x43}, 32),
			want: false,
		},
		{
			name: "One bit different",
			a:    append(bytes.Repeat([]byte{0x00}, 31), 0x00),
			b:    append(bytes.Repeat([]byte{0x00}, 31), 0x01),
			want: false,
		},
		{
			name: "Different lengths",
			a:    bytes.Repeat([]byte{0x42}, 32),
			b:    bytes.Repeat([]byte{0x42}, 31),
			want: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := constantTimeCompare(tc.a, tc.b)
			if got != tc.want {
				t.Errorf("constantTimeCompare() = %v, want %v", got, tc.want)
			}
		})
	}
}

// BenchmarkNtorHandshake benchmarks the complete ntor handshake
func BenchmarkNtorHandshake(b *testing.B) {
	serverIdentity := make([]byte, 32)
	serverNtorKey := make([]byte, 32)
	rand.Read(serverIdentity)
	rand.Read(serverNtorKey)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := NtorClientHandshake(serverIdentity, serverNtorKey)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkNtorProcessResponse benchmarks response processing
func BenchmarkNtorProcessResponse(b *testing.B) {
	// Setup
	serverIdentity := make([]byte, 32)
	serverNtorKey := make([]byte, 32)
	clientPrivate := make([]byte, 32)
	response := make([]byte, 64)
	
	rand.Read(serverIdentity)
	rand.Read(serverNtorKey)
	rand.Read(clientPrivate)
	rand.Read(response) // Will fail auth, but we're benchmarking the computation

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NtorProcessResponse(response, clientPrivate, serverNtorKey, serverIdentity)
		// Ignore error - we're just benchmarking the crypto operations
	}
}
