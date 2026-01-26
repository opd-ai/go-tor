// Package circuit implements Tor circuit management and operations.
// This file contains comprehensive audit tests for layered encryption per tor-spec.txt §5.1
package circuit

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/opd-ai/go-tor/pkg/crypto"
)

// TestLayeredEncryptionAudit provides comprehensive verification of onion encryption
// per tor-spec.txt §5.1 "Relay Cell Encryption"
func TestLayeredEncryptionAudit(t *testing.T) {
	t.Run("EmptyCircuitEncryption", func(t *testing.T) {
		// Edge case: Circuit with no hops
		circuit := &Circuit{Hops: []*Hop{}}

		plaintext := []byte("test payload")
		encrypted := circuit.encryptForward(plaintext)

		// Should return copy without modification
		if !bytes.Equal(encrypted, plaintext) {
			t.Error("Empty circuit should not modify payload")
		}
	})

	t.Run("SingleHopEncryption", func(t *testing.T) {
		// Single hop circuit (unusual but valid)
		hop := createTestHop(t, "single-hop")
		circuit := &Circuit{Hops: []*Hop{hop}}

		plaintext := make([]byte, 509)
		copy(plaintext, []byte("Single hop test"))

		encrypted := circuit.encryptForward(plaintext)
		decrypted := circuit.decryptBackward(encrypted)

		if !bytes.Equal(plaintext, decrypted) {
			t.Error("Single hop round-trip failed")
		}
	})

	t.Run("MaximumHopsEncryption", func(t *testing.T) {
		// Test with maximum typical hop count (8 hops, though 3 is standard)
		// Tor allows up to 8 hops per path-spec.txt
		hops := make([]*Hop, 8)
		for i := 0; i < 8; i++ {
			hops[i] = createTestHop(t, string(rune('A'+i)))
		}

		circuit := &Circuit{Hops: hops}

		plaintext := make([]byte, 509)
		copy(plaintext, []byte("Maximum hops test"))

		encrypted := circuit.encryptForward(plaintext)
		decrypted := circuit.decryptBackward(encrypted)

		if !bytes.Equal(plaintext, decrypted) {
			t.Error("Maximum hops round-trip failed")
		}
	})

	t.Run("NilCipherHandling", func(t *testing.T) {
		// Edge case: Hop with nil cipher (shouldn't happen but must handle gracefully)
		hop := &Hop{
			ForwardCipher:  nil,
			BackwardCipher: nil,
		}
		circuit := &Circuit{Hops: []*Hop{hop}}

		plaintext := []byte("test with nil cipher")
		encrypted := circuit.encryptForward(plaintext)

		// Should not panic, should return copy
		if encrypted == nil {
			t.Error("encryptForward returned nil with nil cipher")
		}
	})

	t.Run("PayloadSizePreservation", func(t *testing.T) {
		// Verify that encryption preserves exact payload size
		// Critical for Tor protocol: all RELAY cells are exactly 509 bytes
		hops := []*Hop{
			createTestHop(t, "hop1"),
			createTestHop(t, "hop2"),
			createTestHop(t, "hop3"),
		}
		circuit := &Circuit{Hops: hops}

		sizes := []int{0, 1, 100, 509}
		for _, size := range sizes {
			plaintext := make([]byte, size)
			encrypted := circuit.encryptForward(plaintext)

			if len(encrypted) != size {
				t.Errorf("Size changed: input=%d, output=%d", size, len(encrypted))
			}

			decrypted := circuit.decryptBackward(encrypted)
			if len(decrypted) != size {
				t.Errorf("Size changed after round-trip: input=%d, output=%d", size, len(decrypted))
			}
		}
	})

	t.Run("EncryptionDeterminism", func(t *testing.T) {
		// Per tor-spec.txt §5.1: AES-CTR is deterministic with same state
		// Same plaintext with fresh ciphers should produce same ciphertext
		key := make([]byte, 16)
		iv := make([]byte, 16)

		plaintext := []byte("determinism test")

		// First encryption
		cipher1, _ := crypto.NewAESCTRCipher(key, iv)
		encrypted1 := make([]byte, len(plaintext))
		copy(encrypted1, plaintext)
		cipher1.Stream().XORKeyStream(encrypted1, encrypted1)

		// Second encryption with same key/IV
		cipher2, _ := crypto.NewAESCTRCipher(key, iv)
		encrypted2 := make([]byte, len(plaintext))
		copy(encrypted2, plaintext)
		cipher2.Stream().XORKeyStream(encrypted2, encrypted2)

		if !bytes.Equal(encrypted1, encrypted2) {
			t.Error("AES-CTR encryption is not deterministic with same key/IV")
		}
	})

	t.Run("EncryptionNonMutation", func(t *testing.T) {
		// Verify that encryptForward/decryptBackward don't mutate input
		hops := []*Hop{createTestHop(t, "test")}
		circuit := &Circuit{Hops: hops}

		original := []byte("immutability test")
		input := make([]byte, len(original))
		copy(input, original)

		_ = circuit.encryptForward(input)

		// Input should be unchanged
		if !bytes.Equal(input, original) {
			t.Error("encryptForward mutated input buffer")
		}

		encrypted := circuit.encryptForward(input)
		_ = circuit.decryptBackward(encrypted)

		// Encrypted buffer should be unchanged
		encryptedCopy := make([]byte, len(encrypted))
		copy(encryptedCopy, encrypted)
		_ = circuit.decryptBackward(encrypted)

		if !bytes.Equal(encrypted, encryptedCopy) {
			t.Error("decryptBackward mutated input buffer")
		}
	})
}

// TestRelayCellDigestVerification provides comprehensive digest verification tests
func TestRelayCellDigestVerification(t *testing.T) {
	t.Run("VerifyRelayCellDigestRecognition", func(t *testing.T) {
		// Test that verifyRelayCellDigest correctly identifies which hop sent the cell
		hops := []*Hop{
			createTestHopWithDigest(t),
			createTestHopWithDigest(t),
			createTestHopWithDigest(t),
		}
		circuit := &Circuit{Hops: hops}

		// Create a relay cell payload
		payload := make([]byte, 509)
		payload[0] = 3 // RelayCommand: RELAY_DATA
		// bytes 1-2: Recognized = 0 (must be 0 for recognized cell)
		binary.BigEndian.PutUint16(payload[1:3], 0)
		// bytes 3-4: StreamID
		binary.BigEndian.PutUint16(payload[3:5], 1)
		// bytes 5-8: Digest (will be computed)
		// bytes 9-10: Length
		binary.BigEndian.PutUint16(payload[9:11], 10)
		// Data
		copy(payload[11:], []byte("test data"))

		// Compute digest for hop 1 (middle hop)
		targetHop := hops[1]
		cellCopy := make([]byte, len(payload))
		copy(cellCopy, payload)
		cellCopy[5] = 0
		cellCopy[6] = 0
		cellCopy[7] = 0
		cellCopy[8] = 0

		// Update hop's backward digest
		targetHop.BackwardDigest.Write(cellCopy)
		digestSum := targetHop.BackwardDigest.Sum(nil)

		// Set digest in payload
		payload[5] = digestSum[0]
		payload[6] = digestSum[1]
		payload[7] = digestSum[2]
		payload[8] = digestSum[3]

		// Verify cell is recognized by hop 1
		hopIdx, err := circuit.verifyRelayCellDigest(payload)
		if err != nil {
			t.Fatalf("verifyRelayCellDigest failed: %v", err)
		}
		if hopIdx != 1 {
			t.Errorf("Wrong hop recognized cell: got %d, want 1", hopIdx)
		}
	})

	t.Run("UnrecognizedCellHandling", func(t *testing.T) {
		// Test that cells with invalid digest are not recognized
		hops := []*Hop{createTestHopWithDigest(t)}
		circuit := &Circuit{Hops: hops}

		payload := make([]byte, 509)
		payload[0] = 3                              // RELAY_DATA
		binary.BigEndian.PutUint16(payload[1:3], 0) // Recognized = 0
		// Set invalid digest
		payload[5] = 0xFF
		payload[6] = 0xFF
		payload[7] = 0xFF
		payload[8] = 0xFF

		hopIdx, err := circuit.verifyRelayCellDigest(payload)
		if err != nil {
			t.Fatalf("verifyRelayCellDigest failed: %v", err)
		}
		if hopIdx != -1 {
			t.Errorf("Invalid digest was recognized: got hop %d, want -1", hopIdx)
		}
	})

	t.Run("RecognizedFieldNonZero", func(t *testing.T) {
		// Per tor-spec.txt §6.1: Cell is only "recognized" if Recognized field is 0
		hop := createTestHopWithDigest(t)
		circuit := &Circuit{Hops: []*Hop{hop}}

		payload := make([]byte, 509)
		payload[0] = 3 // RELAY_DATA
		// Set Recognized field to non-zero (cell should not be recognized even with valid digest)
		binary.BigEndian.PutUint16(payload[1:3], 1)

		// Compute valid digest
		cellCopy := make([]byte, len(payload))
		copy(cellCopy, payload)
		cellCopy[5] = 0
		cellCopy[6] = 0
		cellCopy[7] = 0
		cellCopy[8] = 0

		hop.BackwardDigest.Write(cellCopy)
		digestSum := hop.BackwardDigest.Sum(nil)
		payload[5] = digestSum[0]
		payload[6] = digestSum[1]
		payload[7] = digestSum[2]
		payload[8] = digestSum[3]

		hopIdx, err := circuit.verifyRelayCellDigest(payload)
		if err != nil {
			t.Fatalf("verifyRelayCellDigest failed: %v", err)
		}
		if hopIdx != -1 {
			t.Errorf("Cell with Recognized!=0 was recognized: got hop %d, want -1", hopIdx)
		}
	})

	t.Run("ShortPayloadHandling", func(t *testing.T) {
		// Edge case: Payload too short (< 11 bytes minimum relay cell header)
		hop := createTestHopWithDigest(t)
		circuit := &Circuit{Hops: []*Hop{hop}}

		shortPayload := make([]byte, 5) // Too short

		hopIdx, err := circuit.verifyRelayCellDigest(shortPayload)
		if err == nil {
			t.Error("Expected error for short payload, got nil")
		}
		if hopIdx != -1 {
			t.Errorf("Short payload returned hop index %d, want -1", hopIdx)
		}
	})

	t.Run("DigestUpdateAfterRecognition", func(t *testing.T) {
		// Verify that digest is updated after successful recognition
		hop := createTestHopWithDigest(t)
		circuit := &Circuit{Hops: []*Hop{hop}}

		// Get initial digest state
		initialDigest := hop.BackwardDigest.Sum(nil)

		payload := make([]byte, 509)
		payload[0] = 3                              // RELAY_DATA
		binary.BigEndian.PutUint16(payload[1:3], 0) // Recognized = 0

		// Compute digest
		cellCopy := make([]byte, len(payload))
		copy(cellCopy, payload)
		cellCopy[5] = 0
		cellCopy[6] = 0
		cellCopy[7] = 0
		cellCopy[8] = 0

		// Get digest without updating state
		digestSum := hop.BackwardDigest.Sum(nil)
		payload[5] = digestSum[0]
		payload[6] = digestSum[1]
		payload[7] = digestSum[2]
		payload[8] = digestSum[3]

		// Recognize cell (this should update digest)
		hopIdx, err := circuit.verifyRelayCellDigest(payload)
		if err != nil {
			t.Fatalf("verifyRelayCellDigest failed: %v", err)
		}
		if hopIdx != 0 {
			t.Errorf("Cell not recognized: got hop %d, want 0", hopIdx)
		}

		// Get updated digest state
		updatedDigest := hop.BackwardDigest.Sum(nil)

		// Digest should have changed
		if bytes.Equal(initialDigest, updatedDigest) {
			t.Error("Digest was not updated after cell recognition")
		}
	})
}

// TestHopDigestUpdates verifies per-hop digest updates per tor-spec.txt §5.1
func TestHopDigestUpdates(t *testing.T) {
	t.Run("ForwardDigestUpdate", func(t *testing.T) {
		hops := []*Hop{
			createTestHopWithDigest(t),
			createTestHopWithDigest(t),
		}
		circuit := &Circuit{Hops: hops}

		payload := make([]byte, 509)
		payload[0] = 2 // RELAY_BEGIN

		// Get initial digests
		initialDigest1 := hops[0].ForwardDigest.Sum(nil)
		initialDigest2 := hops[1].ForwardDigest.Sum(nil)

		// Update forward digests
		err := circuit.updateHopDigests(DirectionForward, payload)
		if err != nil {
			t.Fatalf("updateHopDigests failed: %v", err)
		}

		// Verify both hops' digests were updated
		updatedDigest1 := hops[0].ForwardDigest.Sum(nil)
		updatedDigest2 := hops[1].ForwardDigest.Sum(nil)

		if bytes.Equal(initialDigest1, updatedDigest1) {
			t.Error("Hop 0 forward digest was not updated")
		}
		if bytes.Equal(initialDigest2, updatedDigest2) {
			t.Error("Hop 1 forward digest was not updated")
		}
	})

	t.Run("BackwardDigestUpdate", func(t *testing.T) {
		hops := []*Hop{
			createTestHopWithDigest(t),
			createTestHopWithDigest(t),
		}
		circuit := &Circuit{Hops: hops}

		payload := make([]byte, 509)
		payload[0] = 3 // RELAY_DATA

		initialDigest1 := hops[0].BackwardDigest.Sum(nil)
		initialDigest2 := hops[1].BackwardDigest.Sum(nil)

		err := circuit.updateHopDigests(DirectionBackward, payload)
		if err != nil {
			t.Fatalf("updateHopDigests failed: %v", err)
		}

		updatedDigest1 := hops[0].BackwardDigest.Sum(nil)
		updatedDigest2 := hops[1].BackwardDigest.Sum(nil)

		if bytes.Equal(initialDigest1, updatedDigest1) {
			t.Error("Hop 0 backward digest was not updated")
		}
		if bytes.Equal(initialDigest2, updatedDigest2) {
			t.Error("Hop 1 backward digest was not updated")
		}
	})

	t.Run("NilDigestHandling", func(t *testing.T) {
		// Edge case: Hop with nil digest
		hop := &Hop{
			ForwardDigest:  nil,
			BackwardDigest: nil,
		}
		circuit := &Circuit{Hops: []*Hop{hop}}

		payload := make([]byte, 509)

		// Should not panic with nil digests
		err := circuit.updateHopDigests(DirectionForward, payload)
		if err != nil {
			t.Errorf("updateHopDigests failed with nil digest: %v", err)
		}
	})

	t.Run("ShortPayloadDigestUpdate", func(t *testing.T) {
		hop := createTestHopWithDigest(t)
		circuit := &Circuit{Hops: []*Hop{hop}}

		shortPayload := make([]byte, 5) // Too short

		err := circuit.updateHopDigests(DirectionForward, shortPayload)
		if err == nil {
			t.Error("Expected error for short payload, got nil")
		}
	})
}

// TestEncryptionSecurityProperties verifies security-critical properties
func TestEncryptionSecurityProperties(t *testing.T) {
	t.Run("EncryptionChangesAllBits", func(t *testing.T) {
		// Verify that encryption changes the payload (not identity function)
		hop := createTestHop(t, "test")
		circuit := &Circuit{Hops: []*Hop{hop}}

		plaintext := make([]byte, 509)
		for i := range plaintext {
			plaintext[i] = byte(i % 256)
		}

		encrypted := circuit.encryptForward(plaintext)

		// Count different bytes
		differences := 0
		for i := range plaintext {
			if plaintext[i] != encrypted[i] {
				differences++
			}
		}

		// Should change most bytes (allow small margin for very short payloads)
		if differences < len(plaintext)/2 {
			t.Errorf("Encryption changed too few bytes: %d/%d", differences, len(plaintext))
		}
	})

	t.Run("DifferentHopsProduceDifferentCiphertext", func(t *testing.T) {
		// Same plaintext with different hop keys should produce different ciphertext
		key1 := make([]byte, 16)
		key2 := make([]byte, 16)
		for i := range key2 {
			key2[i] = byte(i)
		}

		hop1 := createTestHopWithKey(t, key1)
		hop2 := createTestHopWithKey(t, key2)

		circuit1 := &Circuit{Hops: []*Hop{hop1}}
		circuit2 := &Circuit{Hops: []*Hop{hop2}}

		plaintext := []byte("Same plaintext, different keys")
		payload := make([]byte, 509)
		copy(payload, plaintext)

		encrypted1 := circuit1.encryptForward(payload)
		encrypted2 := circuit2.encryptForward(payload)

		if bytes.Equal(encrypted1, encrypted2) {
			t.Error("Different hop keys produced identical ciphertext")
		}
	})

	t.Run("CiphertextIndistinguishability", func(t *testing.T) {
		// Different plaintexts should produce statistically different ciphertexts
		hop := createTestHop(t, "test")
		circuit := &Circuit{Hops: []*Hop{hop}}

		plaintext1 := make([]byte, 509)
		plaintext2 := make([]byte, 509)
		for i := range plaintext2 {
			plaintext2[i] = 0xFF
		}

		encrypted1 := circuit.encryptForward(plaintext1)
		encrypted2 := circuit.encryptForward(plaintext2)

		if bytes.Equal(encrypted1, encrypted2) {
			t.Error("Different plaintexts produced identical ciphertext")
		}
	})
}
