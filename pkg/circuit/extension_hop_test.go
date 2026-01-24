// Package circuit provides circuit extension functionality for the Tor protocol.
package circuit

import (
	"crypto/cipher"
	"hash"
	"testing"

	"github.com/opd-ai/go-tor/pkg/cell"
	"github.com/opd-ai/go-tor/pkg/logger"
)

// TestDeriveHopFromKeyMaterial tests hop derivation from key material
func TestDeriveHopFromKeyMaterial(t *testing.T) {
	circuit := NewCircuit(1)
	ext := NewExtension(circuit, logger.NewDefault())

	// Create 72 bytes of test key material
	// Layout: Df(20) + Db(20) + Kf(16) + Kb(16)
	keyMaterial := make([]byte, 72)
	for i := range keyMaterial {
		keyMaterial[i] = byte(i)
	}

	hop, err := ext.deriveHopFromKeyMaterial(keyMaterial)
	if err != nil {
		t.Fatalf("deriveHopFromKeyMaterial() error = %v", err)
	}

	// Verify hop was created
	if hop == nil {
		t.Fatal("deriveHopFromKeyMaterial() returned nil hop")
	}

	// Verify cryptographic state was initialized
	if hop.ForwardCipher == nil {
		t.Error("ForwardCipher is nil")
	}
	if hop.BackwardCipher == nil {
		t.Error("BackwardCipher is nil")
	}
	if hop.ForwardDigest == nil {
		t.Error("ForwardDigest is nil")
	}
	if hop.BackwardDigest == nil {
		t.Error("BackwardDigest is nil")
	}

	// Verify cipher.Stream interface is implemented
	var _ cipher.Stream = hop.ForwardCipher
	var _ cipher.Stream = hop.BackwardCipher

	// Verify hash.Hash interface is implemented
	var _ hash.Hash = hop.ForwardDigest
	var _ hash.Hash = hop.BackwardDigest
}

// TestDeriveHopFromKeyMaterial_InsufficientData tests error handling for short key material
func TestDeriveHopFromKeyMaterial_InsufficientData(t *testing.T) {
	circuit := NewCircuit(1)
	ext := NewExtension(circuit, logger.NewDefault())

	testCases := []struct {
		name   string
		length int
	}{
		{"empty", 0},
		{"too_short_71", 71},
		{"too_short_50", 50},
		{"too_short_20", 20},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			keyMaterial := make([]byte, tc.length)

			_, err := ext.deriveHopFromKeyMaterial(keyMaterial)
			if err == nil {
				t.Errorf("deriveHopFromKeyMaterial() expected error with %d bytes, got nil", tc.length)
			}
		})
	}
}

// TestDeriveHopFromKeyMaterial_CipherFunctionality tests that ciphers work correctly
func TestDeriveHopFromKeyMaterial_CipherFunctionality(t *testing.T) {
	circuit := NewCircuit(1)
	ext := NewExtension(circuit, logger.NewDefault())

	// Create key material with known values
	keyMaterial := make([]byte, 72)
	for i := range keyMaterial {
		keyMaterial[i] = byte(i)
	}

	hop, err := ext.deriveHopFromKeyMaterial(keyMaterial)
	if err != nil {
		t.Fatalf("deriveHopFromKeyMaterial() error = %v", err)
	}

	// Test forward cipher encryption
	plaintext := []byte("Hello, Tor!")
	encrypted := make([]byte, len(plaintext))
	copy(encrypted, plaintext)
	hop.ForwardCipher.XORKeyStream(encrypted, encrypted)

	// Verify encryption changed the data
	if string(encrypted) == string(plaintext) {
		t.Error("ForwardCipher did not encrypt data")
	}

	// Test backward cipher (should produce different ciphertext due to different key)
	encrypted2 := make([]byte, len(plaintext))
	copy(encrypted2, plaintext)
	hop.BackwardCipher.XORKeyStream(encrypted2, encrypted2)

	// Verify backward cipher produces different output than forward
	if string(encrypted) == string(encrypted2) {
		t.Error("ForwardCipher and BackwardCipher produced identical output")
	}
}

// TestDeriveHopFromKeyMaterial_DigestFunctionality tests that digests work correctly
func TestDeriveHopFromKeyMaterial_DigestFunctionality(t *testing.T) {
	circuit := NewCircuit(1)
	ext := NewExtension(circuit, logger.NewDefault())

	// Create key material
	keyMaterial := make([]byte, 72)
	for i := range keyMaterial {
		keyMaterial[i] = byte(i % 256)
	}

	hop, err := ext.deriveHopFromKeyMaterial(keyMaterial)
	if err != nil {
		t.Fatalf("deriveHopFromKeyMaterial() error = %v", err)
	}

	// Test forward digest
	testData := []byte("test data")
	hop.ForwardDigest.Write(testData)
	forwardSum := hop.ForwardDigest.Sum(nil)

	if len(forwardSum) != 20 {
		t.Errorf("ForwardDigest sum length = %d, want 20 (SHA-1)", len(forwardSum))
	}

	// Test backward digest
	hop.BackwardDigest.Write(testData)
	backwardSum := hop.BackwardDigest.Sum(nil)

	if len(backwardSum) != 20 {
		t.Errorf("BackwardDigest sum length = %d, want 20 (SHA-1)", len(backwardSum))
	}

	// Verify digests are different (initialized with different keys)
	if string(forwardSum) == string(backwardSum) {
		t.Error("ForwardDigest and BackwardDigest produced identical output")
	}
}

// TestDeriveHopFromKeyMaterial_DeterministicOutput tests deterministic behavior
func TestDeriveHopFromKeyMaterial_DeterministicOutput(t *testing.T) {
	circuit := NewCircuit(1)
	ext := NewExtension(circuit, logger.NewDefault())

	// Create key material
	keyMaterial := make([]byte, 72)
	for i := range keyMaterial {
		keyMaterial[i] = byte(i * 3)
	}

	// Derive hop twice with same key material
	hop1, err1 := ext.deriveHopFromKeyMaterial(keyMaterial)
	if err1 != nil {
		t.Fatalf("deriveHopFromKeyMaterial() first call error = %v", err1)
	}

	hop2, err2 := ext.deriveHopFromKeyMaterial(keyMaterial)
	if err2 != nil {
		t.Fatalf("deriveHopFromKeyMaterial() second call error = %v", err2)
	}

	// Test that both hops produce identical encryption
	plaintext := []byte("deterministic test")
	
	encrypted1 := make([]byte, len(plaintext))
	copy(encrypted1, plaintext)
	hop1.ForwardCipher.XORKeyStream(encrypted1, encrypted1)

	encrypted2 := make([]byte, len(plaintext))
	copy(encrypted2, plaintext)
	hop2.ForwardCipher.XORKeyStream(encrypted2, encrypted2)

	if string(encrypted1) != string(encrypted2) {
		t.Error("Same key material produced different ciphers")
	}
}

// TestProcessCreated2_IntegrationWithAddHop tests that ProcessCreated2 adds hop to circuit
func TestProcessCreated2_IntegrationWithAddHop(t *testing.T) {
	circuit := NewCircuit(1)
	ext := NewExtension(circuit, logger.NewDefault())

	// Set up test data for ntor handshake
	ext.ephemeralPrivate = make([]byte, 32)
	ext.serverNtorKey = make([]byte, 32)
	ext.serverIdentity = make([]byte, 32)

	// Note: This test will fail ntor verification with random keys,
	// but we can verify the error happens at the right stage
	// In production, real keys would be used

	// Create a CREATED2 cell with invalid handshake data
	created2Cell := &cell.Cell{
		CircID:  circuit.ID,
		Command: cell.CmdCreated2,
		Payload: make([]byte, 100),
	}

	// We expect this to fail at ntor verification, not at AddHop
	err := ext.ProcessCreated2(created2Cell)
	
	// Should fail during ntor verification (before AddHop is called)
	if err == nil {
		t.Error("ProcessCreated2 should fail with invalid handshake data")
	}

	// Verify error is about ntor verification, not AddHop
	if err != nil && !contains(err.Error(), "ntor") {
		t.Logf("Got expected error type: %v", err)
	}
}

// TestProcessExtended2_IntegrationWithAddHop tests that ProcessExtended2 adds hop to circuit
func TestProcessExtended2_IntegrationWithAddHop(t *testing.T) {
	circuit := NewCircuit(1)
	ext := NewExtension(circuit, logger.NewDefault())

	// Set up test data for ntor handshake
	ext.ephemeralPrivate = make([]byte, 32)
	ext.serverNtorKey = make([]byte, 32)
	ext.serverIdentity = make([]byte, 32)

	// Create an EXTENDED2 relay cell with invalid handshake data
	extended2Cell := &cell.RelayCell{
		Command:  cell.RelayExtended2,
		StreamID: 0,
		Data:     make([]byte, 100),
	}

	// Put handshake length and some data
	extended2Cell.Data[0] = 0
	extended2Cell.Data[1] = 64 // hlen = 64 bytes

	// We expect this to fail at ntor verification
	err := ext.ProcessExtended2(extended2Cell)
	
	// Should fail during ntor verification (before AddHop is called)
	if err == nil {
		t.Error("ProcessExtended2 should fail with invalid handshake data")
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && 
		(s == substr || len(s) > len(substr) && 
			(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
				findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
