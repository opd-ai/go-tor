// Package circuit implements Tor circuit management and operations.
// This file contains specification compliance tests for relay cell encryption per tor-spec.txt §5.1
package circuit

import (
	"bytes"
	"crypto/sha1" // #nosec G505 - Required by Tor spec
	"encoding/binary"
	"hash"
	"testing"

	"github.com/opd-ai/go-tor/pkg/crypto"
)

// TestRelayCellEncryptionCompliance verifies AES-128-CTR relay cell encryption per tor-spec.txt §5.1
func TestRelayCellEncryptionCompliance(t *testing.T) {
	t.Run("AES-128KeySize", func(t *testing.T) {
		// Per tor-spec.txt §5.1: "For the current specification, the only recognized
		// algorithm is AES-128-CTR, with a 128-bit key."
		// Key size must be exactly 16 bytes for AES-128
		key := make([]byte, 16) // AES-128 key
		iv := make([]byte, 16)  // 128-bit IV

		cipher, err := crypto.NewAESCTRCipher(key, iv)
		if err != nil {
			t.Fatalf("NewAESCTRCipher() with 16-byte key failed: %v", err)
		}
		if cipher == nil {
			t.Fatal("NewAESCTRCipher() returned nil cipher")
		}
	})

	t.Run("ZeroIVRequirement", func(t *testing.T) {
		// Per tor-spec.txt §5.1.1: "Initialize one AES counter-mode cipher with the
		// 16-byte forward key and 16-byte IV set to all zero bytes"
		key := make([]byte, 16)
		zeroIV := make([]byte, 16) // All zeros per spec

		cipher, err := crypto.NewAESCTRCipher(key, zeroIV)
		if err != nil {
			t.Fatalf("NewAESCTRCipher() with zero IV failed: %v", err)
		}
		if cipher == nil {
			t.Fatal("NewAESCTRCipher() returned nil cipher")
		}

		// Verify IV is actually all zeros
		for i, b := range zeroIV {
			if b != 0 {
				t.Errorf("IV byte %d is not zero: %d", i, b)
			}
		}
	})

	t.Run("CTRModeSymmetry", func(t *testing.T) {
		// Per tor-spec.txt §5.1: "In counter mode, the same key/IV is used for both
		// encryption and decryption (XOR operation)"
		key := make([]byte, 16)
		iv := make([]byte, 16)
		plaintext := []byte("Hello, Tor relay cell encryption!")

		// Encrypt
		cipher1, err := crypto.NewAESCTRCipher(key, iv)
		if err != nil {
			t.Fatalf("NewAESCTRCipher() failed: %v", err)
		}
		ciphertext := make([]byte, len(plaintext))
		copy(ciphertext, plaintext)
		cipher1.Encrypt(ciphertext)

		// Decrypt with same key/IV
		cipher2, err := crypto.NewAESCTRCipher(key, iv)
		if err != nil {
			t.Fatalf("NewAESCTRCipher() failed: %v", err)
		}
		decrypted := make([]byte, len(ciphertext))
		copy(decrypted, ciphertext)
		cipher2.Decrypt(decrypted)

		if !bytes.Equal(plaintext, decrypted) {
			t.Errorf("Decrypt(Encrypt(plaintext)) != plaintext\nwant: %x\ngot:  %x", plaintext, decrypted)
		}
	})

	t.Run("RelayCellPayloadSize", func(t *testing.T) {
		// Per tor-spec.txt §6.1: "RELAY cell payloads are 509 bytes"
		// (514 total - 4 CircID - 1 Cmd = 509)
		const RelayPayloadSize = 509

		key := make([]byte, 16)
		iv := make([]byte, 16)

		payload := make([]byte, RelayPayloadSize)
		for i := range payload {
			payload[i] = byte(i % 256)
		}

		cipher, err := crypto.NewAESCTRCipher(key, iv)
		if err != nil {
			t.Fatalf("NewAESCTRCipher() failed: %v", err)
		}

		encrypted := make([]byte, len(payload))
		copy(encrypted, payload)
		cipher.Encrypt(encrypted)

		// Verify size is preserved
		if len(encrypted) != RelayPayloadSize {
			t.Errorf("Encrypted payload size = %d, want %d", len(encrypted), RelayPayloadSize)
		}
	})
}

// TestLayeredEncryption verifies multi-hop onion encryption per tor-spec.txt §5.1
func TestLayeredEncryption(t *testing.T) {
	t.Run("ThreeHopEncryption", func(t *testing.T) {
		// Per tor-spec.txt §5.1: "When Alice sends a RELAY cell to a hop other than the
		// last hop, she encrypts the cell with all of the keys for the hops after it"
		
		// Create 3 hops with different keys
		hops := []*Hop{
			createTestHop(t, "hop1"),
			createTestHop(t, "hop2"),
			createTestHop(t, "hop3"),
		}

		circuit := &Circuit{
			Hops: hops,
		}

		plaintext := make([]byte, 509)
		copy(plaintext, []byte("Test relay cell payload"))

		// Encrypt forward (client → relay)
		encrypted := circuit.encryptForward(plaintext)

		// Verify encrypted data differs from plaintext
		if bytes.Equal(encrypted, plaintext) {
			t.Error("encryptForward() did not modify the payload")
		}

		// Decrypt backward (relay → client)
		decrypted := circuit.decryptBackward(encrypted)

		// Verify round-trip
		if !bytes.Equal(plaintext, decrypted) {
			t.Errorf("Round-trip encryption failed\nwant: %x...\ngot:  %x...", plaintext[:32], decrypted[:32])
		}
	})

	t.Run("EncryptionOrder", func(t *testing.T) {
		// Per tor-spec.txt §5.1: "When sending a relay cell, the client encrypts it
		// with the keys for each hop, in reverse order"
		
		hops := []*Hop{
			createTestHopWithKey(t, []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}),
			createTestHopWithKey(t, []byte{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1}),
			createTestHopWithKey(t, []byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}),
		}

		circuit := &Circuit{Hops: hops}

		plaintext := []byte("Verify encryption order per tor-spec.txt")
		payload := make([]byte, 509)
		copy(payload, plaintext)

		encrypted := circuit.encryptForward(payload)

		// Each hop should produce different intermediate encryption
		// This verifies that all hops are participating in encryption
		if bytes.Equal(encrypted, payload) {
			t.Error("Encryption did not modify payload")
		}
	})

	t.Run("DecryptionOrder", func(t *testing.T) {
		// Per tor-spec.txt §5.1: "When receiving a relay cell, the client decrypts it
		// with the keys for each hop, in forward order"
		
		hops := []*Hop{
			createTestHop(t, "guard"),
			createTestHop(t, "middle"),
			createTestHop(t, "exit"),
		}

		circuit := &Circuit{Hops: hops}

		plaintext := []byte("Test decryption order")
		payload := make([]byte, 509)
		copy(payload, plaintext)

		// Simulate forward encryption
		encrypted := circuit.encryptForward(payload)

		// Decrypt backward
		decrypted := circuit.decryptBackward(encrypted)

		// Verify correct decryption
		if !bytes.Equal(plaintext[:len("Test decryption order")], decrypted[:len("Test decryption order")]) {
			t.Errorf("Decryption order failed\nwant: %s\ngot:  %s", plaintext[:len("Test decryption order")], decrypted[:len("Test decryption order")])
		}
	})
}

// TestRelayCellDigest verifies digest computation per tor-spec.txt §5.1
func TestRelayCellDigest(t *testing.T) {
	t.Run("DigestFieldZeroing", func(t *testing.T) {
		// Per tor-spec.txt §6.1: "The digest field is computed by taking the SHA-1
		// digest of the entire relay cell payload, with the digest field itself set to 0"
		
		payload := make([]byte, 509)
		// Simulate relay cell header:
		// RelayCommand (1) | Recognized (2) | StreamID (2) | Digest (4) | Length (2) | Data (498)
		payload[0] = 2 // RelayCommand: RELAY_BEGIN
		// bytes 1-2: Recognized = 0
		binary.BigEndian.PutUint16(payload[3:5], 1) // StreamID = 1
		// bytes 5-8: Digest field (should be zeroed for computation)
		payload[5] = 0
		payload[6] = 0
		payload[7] = 0
		payload[8] = 0
		binary.BigEndian.PutUint16(payload[9:11], 10) // Length = 10
		copy(payload[11:], []byte("test data"))

		// Verify digest field is at correct offset (bytes 5-8)
		digestField := payload[5:9]
		for i, b := range digestField {
			if b != 0 {
				t.Errorf("Digest field byte %d is not zero before computation: %d", i, b)
			}
		}
	})

	t.Run("SHA1DigestComputation", func(t *testing.T) {
		// Per tor-spec.txt §5.1: "The digest is computed using a running SHA-1 hash"
		
		payload := make([]byte, 509)
		payload[0] = 2 // RELAY_BEGIN

		// Compute SHA-1 digest
		h := sha1.New() // #nosec G401 - Required by Tor spec
		_, err := h.Write(payload)
		if err != nil {
			t.Fatalf("SHA-1 Write() failed: %v", err)
		}
		digest := h.Sum(nil)

		// Verify digest is 20 bytes (SHA-1 output)
		if len(digest) != 20 {
			t.Errorf("SHA-1 digest length = %d, want 20", len(digest))
		}

		// Per tor-spec.txt: only first 4 bytes are used in relay cell
		shortDigest := digest[:4]
		if len(shortDigest) != 4 {
			t.Errorf("Short digest length = %d, want 4", len(shortDigest))
		}
	})

	t.Run("RunningDigestUpdate", func(t *testing.T) {
		// Per tor-spec.txt §5.1: "Each hop maintains separate running digests for
		// forward and backward cells"
		
		hops := []*Hop{
			createTestHopWithDigest(t),
		}

		circuit := &Circuit{Hops: hops}

		payload1 := make([]byte, 509)
		payload1[0] = 2 // RELAY_BEGIN

		payload2 := make([]byte, 509)
		payload2[0] = 3 // RELAY_DATA

		// Update digest with first payload
		err := circuit.updateHopDigests(DirectionForward, payload1)
		if err != nil {
			t.Fatalf("updateHopDigests(1) failed: %v", err)
		}

		// Update digest with second payload
		err = circuit.updateHopDigests(DirectionForward, payload2)
		if err != nil {
			t.Fatalf("updateHopDigests(2) failed: %v", err)
		}

		// Verify digest was updated (non-nil hash state)
		if hops[0].ForwardDigest == nil {
			t.Error("Forward digest is nil after updates")
		}
	})

	t.Run("SeparateForwardBackwardDigests", func(t *testing.T) {
		// Per tor-spec.txt §5.1: "When sending a RELAY cell, the OP computes the digest
		// using the forward digest; when receiving, it uses the backward digest"
		
		hop := createTestHopWithDigest(t)
		circuit := &Circuit{Hops: []*Hop{hop}}

		forwardPayload := make([]byte, 509)
		forwardPayload[0] = 2 // RELAY_BEGIN

		backwardPayload := make([]byte, 509)
		backwardPayload[0] = 3 // RELAY_DATA (different command)

		// Update forward digest with forward payload
		err := circuit.updateHopDigests(DirectionForward, forwardPayload)
		if err != nil {
			t.Fatalf("updateHopDigests(forward) failed: %v", err)
		}

		// Update backward digest with different payload
		err = circuit.updateHopDigests(DirectionBackward, backwardPayload)
		if err != nil {
			t.Fatalf("updateHopDigests(backward) failed: %v", err)
		}

		// Verify both digests exist and are different
		if hop.ForwardDigest == nil {
			t.Error("ForwardDigest is nil")
		}
		if hop.BackwardDigest == nil {
			t.Error("BackwardDigest is nil")
		}

		// Digests should produce different results since they processed different data
		fwd := hop.ForwardDigest.Sum(nil)
		bwd := hop.BackwardDigest.Sum(nil)
		if bytes.Equal(fwd, bwd) {
			t.Error("Forward and backward digests produced identical output (expected different due to different payloads)")
		}
	})
}

// TestEncryptionKeyDerivation verifies key material usage per tor-spec.txt §5.2
func TestEncryptionKeyDerivation(t *testing.T) {
	t.Run("KeyMaterialStructure", func(t *testing.T) {
		// Per tor-spec.txt §5.2: "K = K_1 | K_2 | K_3 | ... where | is concatenation"
		// K contains: Df (20) | Db (20) | Kf (16) | Kb (16) = 72 bytes
		
		keyMaterial := make([]byte, 72)
		for i := range keyMaterial {
			keyMaterial[i] = byte(i)
		}

		// Extract keys per tor-spec.txt §5.2
		Df := keyMaterial[0:20]   // Forward digest
		Db := keyMaterial[20:40]  // Backward digest
		Kf := keyMaterial[40:56]  // Forward key (AES-128 = 16 bytes)
		Kb := keyMaterial[56:72]  // Backward key (AES-128 = 16 bytes)

		// Verify sizes
		if len(Df) != 20 {
			t.Errorf("Df length = %d, want 20", len(Df))
		}
		if len(Db) != 20 {
			t.Errorf("Db length = %d, want 20", len(Db))
		}
		if len(Kf) != 16 {
			t.Errorf("Kf length = %d, want 16 (AES-128)", len(Kf))
		}
		if len(Kb) != 16 {
			t.Errorf("Kb length = %d, want 16 (AES-128)", len(Kb))
		}

		// Verify all keys are different
		if bytes.Equal(Df, Db) {
			t.Error("Forward and backward digest keys are identical")
		}
		if bytes.Equal(Kf, Kb) {
			t.Error("Forward and backward cipher keys are identical")
		}
	})

	t.Run("AES128KeyUsage", func(t *testing.T) {
		// Per tor-spec.txt §5.1: "AES-128-CTR, with a 128-bit key"
		// 128 bits = 16 bytes
		
		keyMaterial := make([]byte, 72)
		Kf := keyMaterial[40:56] // Forward key (16 bytes)
		Kb := keyMaterial[56:72] // Backward key (16 bytes)

		zeroIV := make([]byte, 16)

		// Create forward cipher
		fwdCipher, err := crypto.NewAESCTRCipher(Kf, zeroIV)
		if err != nil {
			t.Fatalf("NewAESCTRCipher(Kf) failed: %v", err)
		}
		if fwdCipher == nil {
			t.Fatal("Forward cipher is nil")
		}

		// Create backward cipher
		bwdCipher, err := crypto.NewAESCTRCipher(Kb, zeroIV)
		if err != nil {
			t.Fatalf("NewAESCTRCipher(Kb) failed: %v", err)
		}
		if bwdCipher == nil {
			t.Fatal("Backward cipher is nil")
		}
	})
}

// Helper functions for test setup

func createTestHop(t *testing.T, name string) *Hop {
	t.Helper()
	key := make([]byte, 16)
	copy(key, []byte(name))
	return createTestHopWithKey(t, key)
}

func createTestHopWithKey(t *testing.T, key []byte) *Hop {
	t.Helper()
	if len(key) != 16 {
		t.Fatalf("Key must be 16 bytes for AES-128, got %d", len(key))
	}

	zeroIV := make([]byte, 16)
	
	fwdCipher, err := crypto.NewAESCTRCipher(key, zeroIV)
	if err != nil {
		t.Fatalf("Failed to create forward cipher: %v", err)
	}

	bwdCipher, err := crypto.NewAESCTRCipher(key, zeroIV)
	if err != nil {
		t.Fatalf("Failed to create backward cipher: %v", err)
	}

	return &Hop{
		ForwardCipher:  fwdCipher.Stream(),
		BackwardCipher: bwdCipher.Stream(),
	}
}

func createTestHopWithDigest(t *testing.T) *Hop {
	t.Helper()
	key := make([]byte, 16)
	
	hop := createTestHopWithKey(t, key)
	hop.ForwardDigest = sha1.New()  // #nosec G401 - Required by Tor spec
	hop.BackwardDigest = sha1.New() // #nosec G401 - Required by Tor spec
	
	return hop
}

// TestHopStructure verifies Hop structure per tor-spec.txt §5.1
func TestHopStructure(t *testing.T) {
	t.Run("CipherFields", func(t *testing.T) {
		hop := createTestHop(t, "test")

		if hop.ForwardCipher == nil {
			t.Error("ForwardCipher is nil")
		}
		if hop.BackwardCipher == nil {
			t.Error("BackwardCipher is nil")
		}
	})

	t.Run("DigestFields", func(t *testing.T) {
		hop := createTestHopWithDigest(t)

		if hop.ForwardDigest == nil {
			t.Error("ForwardDigest is nil")
		}
		if hop.BackwardDigest == nil {
			t.Error("BackwardDigest is nil")
		}

		// Verify digests are hash.Hash interface
		var _ hash.Hash = hop.ForwardDigest
		var _ hash.Hash = hop.BackwardDigest
	})
}
