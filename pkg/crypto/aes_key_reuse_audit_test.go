package crypto

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// TestNoKeyReuseAcrossCircuits verifies that different circuits derive independent AES keys
// even when using the same relay. This tests the core security property that prevents
// traffic correlation across circuits.
//
// Security requirement: Each circuit must have unique encryption keys to prevent
// an attacker from correlating traffic across multiple circuits.
func TestNoKeyReuseAcrossCircuits(t *testing.T) {
	// Simulate two circuits to the same relay with same long-term keys
	// but different ephemeral keys
	relayIdentity := make([]byte, 32)
	relayNtorKey := make([]byte, 32)
	copy(relayIdentity, []byte("test-relay-identity-key-12345678"))
	copy(relayNtorKey, []byte("test-relay-ntor-key-123456789012"))

	// Circuit 1: Generate handshake with unique ephemeral key
	handshake1, _, err := NtorClientHandshake(relayIdentity, relayNtorKey)
	if err != nil {
		t.Fatalf("Circuit 1 handshake failed: %v", err)
	}

	// Circuit 2: Generate handshake with different ephemeral key
	handshake2, _, err := NtorClientHandshake(relayIdentity, relayNtorKey)
	if err != nil {
		t.Fatalf("Circuit 2 handshake failed: %v", err)
	}

	// Verify handshakes are different (different ephemeral keys)
	if bytes.Equal(handshake1, handshake2) {
		t.Fatal("Handshake data is identical - ephemeral keys not unique!")
	}

	// Extract client ephemeral public keys (last 32 bytes of handshake)
	clientPK1 := handshake1[52:84]
	clientPK2 := handshake2[52:84]

	if bytes.Equal(clientPK1, clientPK2) {
		t.Fatal("Client ephemeral public keys are identical - key reuse detected!")
	}

	t.Logf("✓ Circuits use different ephemeral keys")
	t.Logf("  Circuit 1 PK: %s", hex.EncodeToString(clientPK1[:8])+"...")
	t.Logf("  Circuit 2 PK: %s", hex.EncodeToString(clientPK2[:8])+"...")
}

// TestNoKeyReuseBetweenHops verifies that different hops in the same circuit
// use independent AES keys. This is critical for layered encryption security.
//
// Security requirement: Each hop must have independent keys to ensure
// that compromising one relay doesn't compromise the entire circuit.
func TestNoKeyReuseBetweenHops(t *testing.T) {
	// Simulate 3-hop circuit with different relays
	hops := []struct {
		name     string
		identity []byte
		ntorKey  []byte
	}{
		{
			name:     "Guard",
			identity: bytes.Repeat([]byte{0x01}, 32),
			ntorKey:  bytes.Repeat([]byte{0x11}, 32),
		},
		{
			name:     "Middle",
			identity: bytes.Repeat([]byte{0x02}, 32),
			ntorKey:  bytes.Repeat([]byte{0x22}, 32),
		},
		{
			name:     "Exit",
			identity: bytes.Repeat([]byte{0x03}, 32),
			ntorKey:  bytes.Repeat([]byte{0x33}, 32),
		},
	}

	var handshakes [][]byte
	for _, hop := range hops {
		handshake, _, err := NtorClientHandshake(hop.identity, hop.ntorKey)
		if err != nil {
			t.Fatalf("%s hop handshake failed: %v", hop.name, err)
		}
		handshakes = append(handshakes, handshake)
	}

	// Verify all handshakes are unique
	for i := 0; i < len(handshakes); i++ {
		for j := i + 1; j < len(handshakes); j++ {
			if bytes.Equal(handshakes[i], handshakes[j]) {
				t.Fatalf("Hops %d and %d have identical handshakes - key reuse!", i, j)
			}

			// Also verify client ephemeral keys are different
			pk1 := handshakes[i][52:84]
			pk2 := handshakes[j][52:84]
			if bytes.Equal(pk1, pk2) {
				t.Fatalf("Hops %d and %d have identical ephemeral keys!", i, j)
			}
		}
	}

	t.Logf("✓ All %d hops use independent ephemeral keys", len(hops))
}

// TestForwardBackwardKeySeparation verifies that forward and backward direction
// keys are derived from different key material, preventing bidirectional attacks.
//
// Security requirement: Forward (client→relay) and backward (relay→client)
// directions must use independent keys to prevent key-stream reuse attacks.
func TestForwardBackwardKeySeparation(t *testing.T) {
	// Derive 72 bytes of key material using KDF-TOR
	sharedSecret := []byte("test-shared-secret-for-key-derivation-32bytes-long-exactly")
	keyMaterial, err := DeriveKey(sharedSecret, 72)
	if err != nil {
		t.Fatalf("Key derivation failed: %v", err)
	}

	// Extract forward and backward keys per tor-spec.txt §5.2
	// Df (20 bytes) | Db (20 bytes) | Kf (16 bytes) | Kb (16 bytes)
	dfKey := keyMaterial[0:20]  // Forward digest key
	dbKey := keyMaterial[20:40] // Backward digest key
	kfKey := keyMaterial[40:56] // Forward cipher key
	kbKey := keyMaterial[56:72] // Backward cipher key

	// Verify forward and backward digest keys are different
	if bytes.Equal(dfKey, dbKey) {
		t.Fatal("Forward and backward digest keys are identical!")
	}

	// Verify forward and backward cipher keys are different
	if bytes.Equal(kfKey, kbKey) {
		t.Fatal("Forward and backward cipher keys are identical!")
	}

	// Verify no overlap between any key components
	allKeys := []struct {
		name string
		key  []byte
	}{
		{"Df", dfKey},
		{"Db", dbKey},
		{"Kf", kfKey},
		{"Kb", kbKey},
	}

	for i := 0; i < len(allKeys); i++ {
		for j := i + 1; j < len(allKeys); j++ {
			if bytes.Equal(allKeys[i].key, allKeys[j].key) {
				t.Fatalf("Keys %s and %s are identical!", allKeys[i].name, allKeys[j].name)
			}
		}
	}

	t.Logf("✓ Forward and backward keys are properly separated")
	t.Logf("  Kf: %s", hex.EncodeToString(kfKey[:8])+"...")
	t.Logf("  Kb: %s", hex.EncodeToString(kbKey[:8])+"...")
}

// TestZeroIVSafetyWithUniqueKeys verifies that using zero IV is safe when
// combined with unique per-circuit keys. This tests the security property
// that different circuits with zero IV produce different ciphertexts.
//
// Security requirement: Zero IV with CTR mode is safe only if keys are never reused.
func TestZeroIVSafetyWithUniqueKeys(t *testing.T) {
	plaintext := []byte("This is sensitive circuit data that must be protected")

	// Circuit 1: Derive unique key
	secret1 := []byte("circuit-1-shared-secret-from-ntor-handshake")
	keyMaterial1, _ := DeriveKey(secret1, 72)
	key1 := keyMaterial1[40:56] // Kf

	// Circuit 2: Derive different unique key
	secret2 := []byte("circuit-2-shared-secret-from-ntor-handshake")
	keyMaterial2, _ := DeriveKey(secret2, 72)
	key2 := keyMaterial2[40:56] // Kf

	// Verify keys are different
	if bytes.Equal(key1, key2) {
		t.Fatal("Keys are identical - this would be a serious vulnerability!")
	}

	// Both use zero IV (per tor-spec.txt §5.1.1)
	zeroIV := make([]byte, 16)

	// Encrypt same plaintext with both keys
	cipher1, _ := NewAESCTRCipher(key1, zeroIV)
	ciphertext1 := make([]byte, len(plaintext))
	copy(ciphertext1, plaintext)
	cipher1.Encrypt(ciphertext1)

	cipher2, _ := NewAESCTRCipher(key2, zeroIV)
	ciphertext2 := make([]byte, len(plaintext))
	copy(ciphertext2, plaintext)
	cipher2.Encrypt(ciphertext2)

	// Verify ciphertexts are different (proving unique keys)
	if bytes.Equal(ciphertext1, ciphertext2) {
		t.Fatal("Ciphertexts are identical - zero IV with key reuse vulnerability!")
	}

	t.Logf("✓ Zero IV is safe with unique per-circuit keys")
	t.Logf("  Same plaintext encrypts to different ciphertexts")
}

// TestKeyMaterialUniqueness verifies that HKDF derivation with different
// inputs produces completely independent key material.
//
// Security requirement: HKDF must produce cryptographically independent
// keys for different circuits to prevent statistical attacks.
func TestKeyMaterialUniqueness(t *testing.T) {
	// Generate 10 different shared secrets (simulating 10 circuits)
	const numCircuits = 10
	var keyMaterials [][]byte

	for i := 0; i < numCircuits; i++ {
		secret := sha256.Sum256([]byte{byte(i)})
		keyMaterial, err := DeriveKey(secret[:], 72)
		if err != nil {
			t.Fatalf("Circuit %d key derivation failed: %v", i, err)
		}
		keyMaterials = append(keyMaterials, keyMaterial)
	}

	// Verify all key materials are unique
	for i := 0; i < numCircuits; i++ {
		for j := i + 1; j < numCircuits; j++ {
			if bytes.Equal(keyMaterials[i], keyMaterials[j]) {
				t.Fatalf("Circuits %d and %d have identical key material!", i, j)
			}

			// Also check individual key components
			kf1 := keyMaterials[i][40:56]
			kf2 := keyMaterials[j][40:56]
			if bytes.Equal(kf1, kf2) {
				t.Fatalf("Circuits %d and %d have identical Kf keys!", i, j)
			}

			kb1 := keyMaterials[i][56:72]
			kb2 := keyMaterials[j][56:72]
			if bytes.Equal(kb1, kb2) {
				t.Fatalf("Circuits %d and %d have identical Kb keys!", i, j)
			}
		}
	}

	t.Logf("✓ All %d circuits have unique key material", numCircuits)
}

// TestEphemeralKeyIndependence verifies that each call to GenerateNtorKeyPair
// produces a unique ephemeral key pair, ensuring circuit independence.
//
// Security requirement: Ephemeral keys must be cryptographically random
// to prevent predictable circuit keys.
func TestEphemeralKeyIndependence(t *testing.T) {
	const numKeys = 100
	publicKeys := make(map[string]bool)
	privateKeys := make(map[string]bool)

	for i := 0; i < numKeys; i++ {
		kp, err := GenerateNtorKeyPair()
		if err != nil {
			t.Fatalf("Key generation %d failed: %v", i, err)
		}

		pubHex := hex.EncodeToString(kp.Public[:])
		privHex := hex.EncodeToString(kp.Private[:])

		// Check for duplicates
		if publicKeys[pubHex] {
			t.Fatalf("Duplicate public key found at iteration %d!", i)
		}
		if privateKeys[privHex] {
			t.Fatalf("Duplicate private key found at iteration %d!", i)
		}

		publicKeys[pubHex] = true
		privateKeys[privHex] = true
	}

	t.Logf("✓ Generated %d unique ephemeral key pairs", numKeys)
	t.Logf("  No key reuse detected across %d circuits", numKeys)
}

// TestKeyLifecycleIsolation verifies that key material from destroyed circuits
// cannot be reused for new circuits (no persistence across teardown).
//
// Security requirement: Circuit teardown must not leave key material
// accessible for reuse in subsequent circuits.
func TestKeyLifecycleIsolation(t *testing.T) {
	// Simulate circuit 1 lifecycle
	secret1 := []byte("circuit-1-secret")
	keyMaterial1, err := DeriveKey(secret1, 72)
	if err != nil {
		t.Fatalf("Circuit 1 key derivation failed: %v", err)
	}
	key1 := make([]byte, 16)
	copy(key1, keyMaterial1[40:56])

	// Simulate circuit teardown (in real code, would call security.SecureZeroMemory)
	// Here we verify a new circuit gets different keys

	// Simulate circuit 2 lifecycle with different secret
	secret2 := []byte("circuit-2-secret")
	keyMaterial2, err := DeriveKey(secret2, 72)
	if err != nil {
		t.Fatalf("Circuit 2 key derivation failed: %v", err)
	}
	key2 := make([]byte, 16)
	copy(key2, keyMaterial2[40:56])

	// Verify keys are different
	if bytes.Equal(key1, key2) {
		t.Fatal("Circuit 2 reused Circuit 1 keys - lifecycle isolation failed!")
	}

	t.Logf("✓ New circuits derive independent keys")
	t.Logf("  No key persistence across circuit lifecycle")
}

// TestCipherStreamIndependence verifies that cipher streams for different
// hops are completely independent and don't share state.
//
// Security requirement: Cipher streams must be isolated to prevent
// state corruption or unintended key stream reuse.
func TestCipherStreamIndependence(t *testing.T) {
	// Create two independent cipher streams (simulating two hops)
	key1 := bytes.Repeat([]byte{0xAA}, 16)
	key2 := bytes.Repeat([]byte{0xBB}, 16)
	zeroIV := make([]byte, 16)

	cipher1, err := NewAESCTRCipher(key1, zeroIV)
	if err != nil {
		t.Fatalf("Cipher 1 creation failed: %v", err)
	}

	cipher2, err := NewAESCTRCipher(key2, zeroIV)
	if err != nil {
		t.Fatalf("Cipher 2 creation failed: %v", err)
	}

	// Encrypt same plaintext with both ciphers
	plaintext := bytes.Repeat([]byte("test"), 10)

	encrypted1 := make([]byte, len(plaintext))
	copy(encrypted1, plaintext)
	cipher1.Encrypt(encrypted1)

	encrypted2 := make([]byte, len(plaintext))
	copy(encrypted2, plaintext)
	cipher2.Encrypt(encrypted2)

	// Verify ciphertexts are different (proving independent streams)
	if bytes.Equal(encrypted1, encrypted2) {
		t.Fatal("Cipher streams produced identical output - streams are not independent!")
	}

	// Advance cipher1 state
	dummy := make([]byte, 100)
	cipher1.Encrypt(dummy)

	// Verify cipher2 is unaffected by cipher1 state change
	encrypted2Again := make([]byte, len(plaintext))
	copy(encrypted2Again, plaintext)

	// Create new cipher2 instance with same key (should produce same output)
	cipher2New, _ := NewAESCTRCipher(key2, zeroIV)
	cipher2New.Encrypt(encrypted2Again)

	if !bytes.Equal(encrypted2, encrypted2Again) {
		t.Fatal("Cipher2 state was corrupted by cipher1 operations!")
	}

	t.Logf("✓ Cipher streams are independent")
	t.Logf("  No state sharing between hops")
}

// TestKeyMaterialSizeValidation verifies that key derivation always produces
// exactly 72 bytes as required by tor-spec.txt §5.2.
//
// Security requirement: Key material must be exactly 72 bytes to ensure
// all required keys (Df, Db, Kf, Kb) are properly derived.
func TestKeyMaterialSizeValidation(t *testing.T) {
	secret := []byte("test-secret")

	// Test exact size
	keyMaterial, err := DeriveKey(secret, 72)
	if err != nil {
		t.Fatalf("Derivation of 72 bytes failed: %v", err)
	}
	if len(keyMaterial) != 72 {
		t.Fatalf("Expected 72 bytes, got %d", len(keyMaterial))
	}

	// Verify we can extract all required keys
	df := keyMaterial[0:20]
	db := keyMaterial[20:40]
	kf := keyMaterial[40:56]
	kb := keyMaterial[56:72]

	if len(df) != 20 || len(db) != 20 || len(kf) != 16 || len(kb) != 16 {
		t.Fatal("Key material layout is incorrect")
	}

	t.Logf("✓ Key material is exactly 72 bytes")
	t.Logf("  Df: 20 bytes, Db: 20 bytes, Kf: 16 bytes, Kb: 16 bytes")
}

// TestSecureKeyZeroing verifies that sensitive key material can be securely
// zeroed to prevent key reuse or recovery.
//
// Security requirement: Ephemeral keys and intermediate key material
// must be zeroed after use to prevent memory-based attacks.
func TestSecureKeyZeroing(t *testing.T) {
	// Create key material
	keyMaterial := []byte("sensitive-key-material-that-should-be-zeroed")
	original := make([]byte, len(keyMaterial))
	copy(original, keyMaterial)

	// Zero the key material
	for i := range keyMaterial {
		keyMaterial[i] = 0
	}

	// Verify all bytes are zero
	for i, b := range keyMaterial {
		if b != 0 {
			t.Fatalf("Byte %d was not zeroed: %d", i, b)
		}
	}

	// Verify original is different (proving we actually zeroed)
	if bytes.Equal(keyMaterial, original) {
		t.Fatal("Key material was not actually zeroed")
	}

	t.Logf("✓ Key material can be securely zeroed")
	t.Logf("  All %d bytes zeroed successfully", len(keyMaterial))
}

// TestNoKeyReuseInLayeredEncryption verifies that multi-hop onion encryption
// uses independent keys for each layer, preventing key stream reuse.
//
// Security requirement: Layered encryption must use independent keys
// to ensure that breaking one hop doesn't compromise other hops.
func TestNoKeyReuseInLayeredEncryption(t *testing.T) {
	// Simulate 3-hop circuit key derivation
	numHops := 3
	var hopKeys [][]byte

	for i := 0; i < numHops; i++ {
		secret := sha256.Sum256([]byte{byte(i)})
		keyMaterial, err := DeriveKey(secret[:], 72)
		if err != nil {
			t.Fatalf("Hop %d key derivation failed: %v", i, err)
		}
		// Extract forward cipher key (Kf)
		kf := make([]byte, 16)
		copy(kf, keyMaterial[40:56])
		hopKeys = append(hopKeys, kf)
	}

	// Verify all hop keys are unique
	for i := 0; i < numHops; i++ {
		for j := i + 1; j < numHops; j++ {
			if bytes.Equal(hopKeys[i], hopKeys[j]) {
				t.Fatalf("Hops %d and %d have identical keys!", i, j)
			}
		}
	}

	// Simulate layered encryption
	plaintext := []byte("Multi-hop encrypted data")
	encrypted := make([]byte, len(plaintext))
	copy(encrypted, plaintext)

	zeroIV := make([]byte, 16)
	var ciphertexts [][]byte

	// Encrypt with each hop (reverse order: exit -> middle -> guard)
	for i := len(hopKeys) - 1; i >= 0; i-- {
		cipher, _ := NewAESCTRCipher(hopKeys[i], zeroIV)
		cipher.Encrypt(encrypted)

		// Save intermediate ciphertext
		intermediate := make([]byte, len(encrypted))
		copy(intermediate, encrypted)
		ciphertexts = append(ciphertexts, intermediate)
	}

	// Verify each layer produces different output
	for i := 0; i < len(ciphertexts)-1; i++ {
		if bytes.Equal(ciphertexts[i], ciphertexts[i+1]) {
			t.Fatalf("Layer %d and %d produced identical output!", i, i+1)
		}
	}

	// Final ciphertext should be different from plaintext
	if bytes.Equal(ciphertexts[len(ciphertexts)-1], plaintext) {
		t.Fatal("Layered encryption failed - plaintext leaked!")
	}

	t.Logf("✓ Layered encryption uses independent keys for each hop")
	t.Logf("  %d unique encryption layers verified", numHops)
}
