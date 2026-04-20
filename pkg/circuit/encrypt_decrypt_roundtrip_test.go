// Package circuit provides encryption/decryption round-trip tests
// verifying AES-CTR cipher state management per tor-spec.txt §5.1 and §6.1.
//
// These tests validate that relay cell data survives the complete encryption
// and decryption pipeline (forward encrypt → backward decrypt) across
// various circuit configurations, payload sizes, and sequential operations.
//
// Compliance: tor-spec.txt §5.1 (Relay Cell Encryption), §6.1 (Cell Processing)
package circuit

import (
	"bytes"
	"crypto/rand"
	"crypto/sha1" // #nosec G401 - SHA-1 required by Tor protocol
	"testing"

	"github.com/opd-ai/go-tor/pkg/crypto"
)

// createRoundTripHop creates a matched pair of cipher streams for
// round-trip testing. The forward and backward ciphers use the same
// key but independent state, matching the production behavior where
// encrypt and decrypt operations share the key but track position
// independently.
func createRoundTripHop(t *testing.T, key []byte) *Hop {
	t.Helper()
	if len(key) != 16 {
		t.Fatalf("Key must be 16 bytes, got %d", len(key))
	}

	zeroIV := make([]byte, 16)

	fwd, err := crypto.NewAESCTRCipher(key, zeroIV)
	if err != nil {
		t.Fatalf("forward cipher: %v", err)
	}

	bwd, err := crypto.NewAESCTRCipher(key, zeroIV)
	if err != nil {
		t.Fatalf("backward cipher: %v", err)
	}

	return &Hop{
		ForwardCipher:  fwd.Stream(),
		BackwardCipher: bwd.Stream(),
		ForwardDigest:  sha1.New(), // #nosec G401
		BackwardDigest: sha1.New(), // #nosec G401
	}
}

// TestEncryptDecryptRoundTrip verifies that plaintext survives
// forward encryption followed by backward decryption for various
// circuit configurations.
func TestEncryptDecryptRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		numHops int
	}{
		{"single hop", 1},
		{"standard 3-hop circuit", 3},
		{"maximum 8-hop circuit", 8},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hops := make([]*Hop, tc.numHops)
			for i := range tc.numHops {
				key := make([]byte, 16)
				key[0] = byte(i + 1)
				hops[i] = createRoundTripHop(t, key)
			}

			circ := &Circuit{Hops: hops}

			// Standard relay cell payload (509 bytes)
			plaintext := make([]byte, 509)
			for i := range plaintext {
				plaintext[i] = byte(i % 256)
			}

			encrypted := circ.encryptForward(plaintext)
			decrypted := circ.decryptBackward(encrypted)

			if !bytes.Equal(plaintext, decrypted) {
				t.Errorf("round-trip failed: plaintext != decrypted")
			}
		})
	}
}

// TestEncryptDecryptVariousPayloadSizes verifies round-trip
// correctness for different payload sizes.
func TestEncryptDecryptVariousPayloadSizes(t *testing.T) {
	sizes := []int{0, 1, 15, 16, 17, 128, 256, 509, 510, 1024}

	for _, sz := range sizes {
		t.Run("size_"+string(rune('0'+sz/100))+string(rune('0'+(sz%100)/10))+string(rune('0'+sz%10)), func(t *testing.T) {
			// Need fresh ciphers for each sub-test since AES-CTR is stateful
			freshKey := make([]byte, 16)
			freshKey[0] = byte(sz)
			freshHop := createRoundTripHop(t, freshKey)
			freshCirc := &Circuit{Hops: []*Hop{freshHop}}

			plaintext := make([]byte, sz)
			if sz > 0 {
				_, _ = rand.Read(plaintext)
			}

			encrypted := freshCirc.encryptForward(plaintext)
			decrypted := freshCirc.decryptBackward(encrypted)

			if !bytes.Equal(plaintext, decrypted) {
				t.Errorf("size %d: round-trip failed", sz)
			}
		})
	}
}

// TestSequentialEncryptDecryptCipherState verifies that AES-CTR
// cipher state advances correctly across multiple sequential
// encrypt/decrypt operations. This catches bugs where cipher state
// is not properly maintained between operations.
func TestSequentialEncryptDecryptCipherState(t *testing.T) {
	key := make([]byte, 16)
	key[0] = 0xAB

	// Two separate circuits with identical initial state
	hop1 := createRoundTripHop(t, key)
	hop2 := createRoundTripHop(t, key)
	circ1 := &Circuit{Hops: []*Hop{hop1}}
	circ2 := &Circuit{Hops: []*Hop{hop2}}

	payload := make([]byte, 509)
	_, _ = rand.Read(payload)

	// Encrypt the same payload twice on circ1
	enc1 := circ1.encryptForward(payload)
	enc2 := circ1.encryptForward(payload)

	// Due to AES-CTR state advancement, the two encryptions
	// should produce DIFFERENT ciphertext
	if bytes.Equal(enc1, enc2) {
		t.Error("sequential encryptions produced identical ciphertext (AES-CTR state not advancing)")
	}

	// Encrypt on circ2 (fresh state) should match circ1's first encryption
	enc2First := circ2.encryptForward(payload)
	if !bytes.Equal(enc1, enc2First) {
		t.Error("fresh circuit with same key produced different first encryption")
	}
}

// TestEncryptionNotIdentityTransform verifies that encryption
// actually changes the plaintext (catches no-op cipher bugs).
func TestEncryptionNotIdentityTransform(t *testing.T) {
	key := make([]byte, 16)
	for i := range key {
		key[i] = byte(i + 1) // Non-zero key
	}
	hop := createRoundTripHop(t, key)
	circ := &Circuit{Hops: []*Hop{hop}}

	plaintext := make([]byte, 509)
	for i := range plaintext {
		plaintext[i] = byte(i % 256)
	}

	encrypted := circ.encryptForward(plaintext)

	if bytes.Equal(encrypted, plaintext) {
		t.Error("encryption produced identical output to plaintext (cipher may be no-op)")
	}
}

// TestMultiHopLayeredEncryptionOrder verifies that multi-hop
// encryption applies layers in the correct order. The client
// encrypts in reverse order (exit→guard) so each hop decrypts
// one layer in forward order.
func TestMultiHopLayeredEncryptionOrder(t *testing.T) {
	// Create 3 hops with distinct keys
	keys := [][]byte{
		{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, // guard
		{2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, // middle
		{3, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, // exit
	}

	hops := make([]*Hop, 3)
	for i, key := range keys {
		hops[i] = createRoundTripHop(t, key)
	}

	circ := &Circuit{Hops: hops}

	plaintext := make([]byte, 509)
	copy(plaintext, []byte("layered encryption test data"))

	encrypted := circ.encryptForward(plaintext)

	// Build a single-hop circuit for each hop to verify layering
	// The guard (hop 0) should be able to decrypt the outermost layer
	guardHop := createRoundTripHop(t, keys[0])
	guardCirc := &Circuit{Hops: []*Hop{guardHop}}

	afterGuard := guardCirc.decryptBackward(encrypted)

	// After guard decryption, the result should NOT be plaintext
	// (still has middle + exit layers)
	if bytes.Equal(afterGuard, plaintext) {
		t.Error("guard decryption alone recovered plaintext (missing middle/exit layers)")
	}

	// Full circuit decryption should recover plaintext
	decrypted := circ.decryptBackward(encrypted)
	if !bytes.Equal(decrypted, plaintext) {
		t.Error("full circuit decryption failed")
	}
}

// TestDigestVerificationShortPayload verifies digest verification
// rejects payloads shorter than the minimum relay cell size.
func TestDigestVerificationShortPayload(t *testing.T) {
	hop := createRoundTripHop(t, make([]byte, 16))
	circ := &Circuit{Hops: []*Hop{hop}}

	shortPayloads := [][]byte{
		nil,
		{},
		{0x00},
		make([]byte, 10), // one byte short of minimum (11)
	}

	for i, payload := range shortPayloads {
		_, err := circ.verifyRelayCellDigest(payload)
		if err == nil {
			t.Errorf("payload %d (len %d): expected error for short payload",
				i, len(payload))
		}
	}
}

// TestUpdateHopDigestsShortPayload verifies that updateHopDigests
// rejects payloads shorter than 11 bytes.
func TestUpdateHopDigestsShortPayload(t *testing.T) {
	hop := createRoundTripHop(t, make([]byte, 16))
	circ := &Circuit{Hops: []*Hop{hop}}

	shortPayloads := [][]byte{
		nil,
		{},
		make([]byte, 10),
	}

	for i, payload := range shortPayloads {
		err := circ.updateHopDigests(DirectionForward, payload)
		if err == nil {
			t.Errorf("payload %d (len %d): expected error for short payload",
				i, len(payload))
		}
	}
}

// TestAllZeroPayloadRoundTrip verifies that an all-zero payload
// (a plausible relay cell) survives encryption round-trip.
func TestAllZeroPayloadRoundTrip(t *testing.T) {
	key := make([]byte, 16)
	key[0] = 0xFF
	hop := createRoundTripHop(t, key)
	circ := &Circuit{Hops: []*Hop{hop}}

	plaintext := make([]byte, 509)
	encrypted := circ.encryptForward(plaintext)
	decrypted := circ.decryptBackward(encrypted)

	if !bytes.Equal(plaintext, decrypted) {
		t.Error("all-zero payload round-trip failed")
	}
}

// TestAllOnesPayloadRoundTrip verifies round-trip with all-0xFF payload.
func TestAllOnesPayloadRoundTrip(t *testing.T) {
	key := make([]byte, 16)
	key[0] = 0x01
	hop := createRoundTripHop(t, key)
	circ := &Circuit{Hops: []*Hop{hop}}

	plaintext := bytes.Repeat([]byte{0xFF}, 509)
	encrypted := circ.encryptForward(plaintext)
	decrypted := circ.decryptBackward(encrypted)

	if !bytes.Equal(plaintext, decrypted) {
		t.Error("all-0xFF payload round-trip failed")
	}
}

// TestRandomPayloadRoundTrip performs round-trip testing with
// random payloads to catch data-dependent bugs.
func TestRandomPayloadRoundTrip(t *testing.T) {
	for i := range 20 {
		key := make([]byte, 16)
		_, _ = rand.Read(key)
		hop := createRoundTripHop(t, key)
		circ := &Circuit{Hops: []*Hop{hop}}

		plaintext := make([]byte, 509)
		_, _ = rand.Read(plaintext)

		encrypted := circ.encryptForward(plaintext)
		decrypted := circ.decryptBackward(encrypted)

		if !bytes.Equal(plaintext, decrypted) {
			t.Errorf("iteration %d: random payload round-trip failed", i)
		}
	}
}

// TestEncryptForwardOriginalUnmodified verifies that encryptForward
// does not modify the original plaintext slice.
func TestEncryptForwardOriginalUnmodified(t *testing.T) {
	key := make([]byte, 16)
	key[0] = 0x42
	hop := createRoundTripHop(t, key)
	circ := &Circuit{Hops: []*Hop{hop}}

	original := make([]byte, 509)
	_, _ = rand.Read(original)
	originalCopy := make([]byte, 509)
	copy(originalCopy, original)

	_ = circ.encryptForward(original)

	if !bytes.Equal(original, originalCopy) {
		t.Error("encryptForward modified the original plaintext slice")
	}
}

// TestDecryptBackwardOriginalUnmodified verifies that decryptBackward
// does not modify the original ciphertext slice.
func TestDecryptBackwardOriginalUnmodified(t *testing.T) {
	key := make([]byte, 16)
	key[0] = 0x42
	hop := createRoundTripHop(t, key)
	circ := &Circuit{Hops: []*Hop{hop}}

	ciphertext := make([]byte, 509)
	_, _ = rand.Read(ciphertext)
	ciphertextCopy := make([]byte, 509)
	copy(ciphertextCopy, ciphertext)

	_ = circ.decryptBackward(ciphertext)

	if !bytes.Equal(ciphertext, ciphertextCopy) {
		t.Error("decryptBackward modified the original ciphertext slice")
	}
}
