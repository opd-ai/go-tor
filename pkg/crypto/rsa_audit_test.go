package crypto

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1" // #nosec G505
	"testing"
)

// TestRSAOAEPPadding verifies RSA-OAEP padding implementation per tor-spec.txt §0.3
// Tor protocol mandates RSA-1024-OAEP-SHA1 for hybrid encryption
func TestRSAOAEPPadding(t *testing.T) {
	tests := []struct {
		name     string
		bits     int
		dataSize int
	}{
		{
			name:     "1024-bit key with small data",
			bits:     1024,
			dataSize: 32,
		},
		{
			name:     "1024-bit key with max data",
			bits:     1024,
			dataSize: 86, // Max for RSA-1024-OAEP-SHA1: (1024/8) - 2*20 - 2 = 86
		},
		{
			name:     "2048-bit key",
			bits:     2048,
			dataSize: 64,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			privateKey, err := GenerateRSAKey(tt.bits)
			if err != nil {
				t.Fatalf("GenerateRSAKey(%d) error = %v", tt.bits, err)
			}

			publicKey := privateKey.PublicKey()
			plaintext := make([]byte, tt.dataSize)
			_, err = rand.Read(plaintext)
			if err != nil {
				t.Fatalf("rand.Read() error = %v", err)
			}

			// Encrypt using OAEP
			ciphertext, err := publicKey.Encrypt(plaintext)
			if err != nil {
				t.Fatalf("Encrypt() error = %v", err)
			}

			// Verify ciphertext length equals modulus size
			expectedLen := tt.bits / 8
			if len(ciphertext) != expectedLen {
				t.Errorf("ciphertext length = %d, want %d", len(ciphertext), expectedLen)
			}

			// Decrypt using OAEP
			decrypted, err := privateKey.Decrypt(ciphertext)
			if err != nil {
				t.Fatalf("Decrypt() error = %v", err)
			}

			// Verify plaintext matches
			if !bytes.Equal(decrypted, plaintext) {
				t.Errorf("decrypted plaintext doesn't match original")
			}
		})
	}
}

// TestRSAOAEPPaddingUniqueness verifies OAEP produces different ciphertexts for same plaintext
// This tests the randomized padding property of OAEP
func TestRSAOAEPPaddingUniqueness(t *testing.T) {
	privateKey, err := GenerateRSAKey(1024)
	if err != nil {
		t.Fatalf("GenerateRSAKey() error = %v", err)
	}

	publicKey := privateKey.PublicKey()
	plaintext := []byte("The same message encrypted twice")

	ciphertext1, err := publicKey.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("First Encrypt() error = %v", err)
	}

	ciphertext2, err := publicKey.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Second Encrypt() error = %v", err)
	}

	// OAEP is randomized, so same plaintext should produce different ciphertexts
	if bytes.Equal(ciphertext1, ciphertext2) {
		t.Error("OAEP produced identical ciphertexts for same plaintext (should be randomized)")
	}

	// Both should decrypt to same plaintext
	decrypted1, err := privateKey.Decrypt(ciphertext1)
	if err != nil {
		t.Fatalf("Decrypt ciphertext1 error = %v", err)
	}

	decrypted2, err := privateKey.Decrypt(ciphertext2)
	if err != nil {
		t.Fatalf("Decrypt ciphertext2 error = %v", err)
	}

	if !bytes.Equal(decrypted1, plaintext) {
		t.Error("First decryption doesn't match plaintext")
	}

	if !bytes.Equal(decrypted2, plaintext) {
		t.Error("Second decryption doesn't match plaintext")
	}
}

// TestRSAOAEPMaxMessageSize verifies OAEP respects maximum message size
func TestRSAOAEPMaxMessageSize(t *testing.T) {
	privateKey, err := GenerateRSAKey(1024)
	if err != nil {
		t.Fatalf("GenerateRSAKey() error = %v", err)
	}

	publicKey := privateKey.PublicKey()

	// Max size for RSA-1024-OAEP-SHA1: k - 2*hLen - 2 = 128 - 40 - 2 = 86 bytes
	maxSize := 86

	// Test at max size (should succeed)
	maxPlaintext := make([]byte, maxSize)
	_, err = rand.Read(maxPlaintext)
	if err != nil {
		t.Fatalf("rand.Read() error = %v", err)
	}

	_, err = publicKey.Encrypt(maxPlaintext)
	if err != nil {
		t.Errorf("Encrypt at max size (%d bytes) failed: %v", maxSize, err)
	}

	// Test over max size (should fail)
	oversizePlaintext := make([]byte, maxSize+1)
	_, err = rand.Read(oversizePlaintext)
	if err != nil {
		t.Fatalf("rand.Read() error = %v", err)
	}

	_, err = publicKey.Encrypt(oversizePlaintext)
	if err == nil {
		t.Errorf("Encrypt with oversize message (%d bytes) should fail", len(oversizePlaintext))
	}
}

// TestRSAOAEPSHA1Usage verifies OAEP uses SHA-1 hash as mandated by Tor spec
func TestRSAOAEPSHA1Usage(t *testing.T) {
	privateKey, err := GenerateRSAKey(1024)
	if err != nil {
		t.Fatalf("GenerateRSAKey() error = %v", err)
	}

	publicKey := privateKey.PublicKey()
	plaintext := []byte("Test SHA-1 usage in OAEP")

	// Encrypt using our wrapper (should use SHA-1)
	ciphertext, err := publicKey.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	// Decrypt using standard library with SHA-1 (should work if we're using SHA-1)
	decrypted, err := rsa.DecryptOAEP(sha1.New(), rand.Reader, privateKey.key, ciphertext, nil) // #nosec G401
	if err != nil {
		t.Fatalf("DecryptOAEP with SHA-1 error = %v (implementation may not be using SHA-1)", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Error("Decryption with SHA-1 failed - implementation may not be using correct hash")
	}
}

// TestRSAKeySize validates RSA key size per Tor specification
// Tor requires minimum 1024-bit RSA keys (tor-spec.txt §0.3)
func TestRSAKeySize(t *testing.T) {
	tests := []struct {
		name      string
		bits      int
		wantError bool
	}{
		{
			name:      "512-bit (rejected by Go)",
			bits:      512,
			wantError: true, // Go crypto rejects keys < 1024 bits as insecure
		},
		{
			name:      "1024-bit (Tor minimum)",
			bits:      1024,
			wantError: false,
		},
		{
			name:      "2048-bit (recommended)",
			bits:      2048,
			wantError: false,
		},
		{
			name:      "4096-bit (strong)",
			bits:      4096,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := GenerateRSAKey(tt.bits)
			if (err != nil) != tt.wantError {
				t.Errorf("GenerateRSAKey(%d) error = %v, wantError %v", tt.bits, err, tt.wantError)
				return
			}

			if err == nil {
				// Verify key size matches request
				if key.key.N.BitLen() != tt.bits {
					t.Errorf("Generated key has %d bits, want %d", key.key.N.BitLen(), tt.bits)
				}
			}
		})
	}
}

// TestRSAKeySizeValidation verifies minimum key size enforcement for Tor
func TestRSAKeySizeValidation(t *testing.T) {
	// Generate 1024-bit key (Tor minimum per tor-spec.txt §0.3)
	key1024, err := GenerateRSAKey(1024)
	if err != nil {
		t.Fatalf("GenerateRSAKey(1024) error = %v", err)
	}

	if key1024.key.N.BitLen() < 1024 {
		t.Errorf("1024-bit key has %d bits, want at least 1024", key1024.key.N.BitLen())
	}

	// Generate 2048-bit key (recommended)
	key2048, err := GenerateRSAKey(2048)
	if err != nil {
		t.Fatalf("GenerateRSAKey(2048) error = %v", err)
	}

	if key2048.key.N.BitLen() < 2048 {
		t.Errorf("2048-bit key has %d bits, want at least 2048", key2048.key.N.BitLen())
	}

	// Verify keys are different
	if key1024.key.N.Cmp(key2048.key.N) == 0 {
		t.Error("Generated identical moduli for different key sizes")
	}
}

// TestRSAKeyGeneration verifies RSA key generation produces valid key pairs
func TestRSAKeyGeneration(t *testing.T) {
	privateKey, err := GenerateRSAKey(1024)
	if err != nil {
		t.Fatalf("GenerateRSAKey() error = %v", err)
	}

	// Verify private key is not nil
	if privateKey == nil || privateKey.key == nil {
		t.Fatal("Generated private key is nil")
	}

	// Verify public key can be extracted
	publicKey := privateKey.PublicKey()
	if publicKey == nil || publicKey.key == nil {
		t.Fatal("Public key is nil")
	}

	// Verify modulus matches
	if privateKey.key.N.Cmp(publicKey.key.N) != 0 {
		t.Error("Public and private key moduli don't match")
	}

	// Verify exponent is standard (65537)
	if publicKey.key.E != 65537 {
		t.Logf("Warning: public exponent is %d, commonly 65537", publicKey.key.E)
	}
}

// TestHybridEncryption verifies hybrid encryption combining RSA and AES
// This tests the pattern used in Tor's TAP and CREATE_FAST protocols
func TestHybridEncryption(t *testing.T) {
	// Generate RSA key pair (1024-bit per Tor spec)
	rsaPrivateKey, err := GenerateRSAKey(1024)
	if err != nil {
		t.Fatalf("GenerateRSAKey() error = %v", err)
	}
	rsaPublicKey := rsaPrivateKey.PublicKey()

	// Simulate hybrid encryption:
	// 1. Generate random AES key (32 bytes for AES-256)
	aesKey := make([]byte, AES256KeySize)
	_, err = rand.Read(aesKey)
	if err != nil {
		t.Fatalf("rand.Read() error = %v", err)
	}

	// 2. Encrypt AES key with RSA-OAEP
	encryptedAESKey, err := rsaPublicKey.Encrypt(aesKey)
	if err != nil {
		t.Fatalf("RSA Encrypt() error = %v", err)
	}

	// 3. Use AES to encrypt larger data
	largeData := make([]byte, 1024)
	_, err = rand.Read(largeData)
	if err != nil {
		t.Fatalf("rand.Read() error = %v", err)
	}

	iv := make([]byte, 16)
	_, err = rand.Read(iv)
	if err != nil {
		t.Fatalf("rand.Read() error = %v", err)
	}

	encryptedData, err := EncryptAES256CTR(largeData, aesKey, iv)
	if err != nil {
		t.Fatalf("EncryptAES256CTR() error = %v", err)
	}

	// Decryption process:
	// 1. Decrypt AES key with RSA-OAEP
	decryptedAESKey, err := rsaPrivateKey.Decrypt(encryptedAESKey)
	if err != nil {
		t.Fatalf("RSA Decrypt() error = %v", err)
	}

	if !bytes.Equal(decryptedAESKey, aesKey) {
		t.Fatal("Decrypted AES key doesn't match original")
	}

	// 2. Decrypt data with AES
	decryptedData, err := DecryptAES256CTR(encryptedData, decryptedAESKey, iv)
	if err != nil {
		t.Fatalf("DecryptAES256CTR() error = %v", err)
	}

	if !bytes.Equal(decryptedData, largeData) {
		t.Error("Decrypted data doesn't match original")
	}
}

// TestHybridEncryptionKeyTransport verifies key transport pattern
func TestHybridEncryptionKeyTransport(t *testing.T) {
	// Generate 1024-bit RSA key (Tor minimum)
	serverPrivate, err := GenerateRSAKey(1024)
	if err != nil {
		t.Fatalf("GenerateRSAKey() error = %v", err)
	}
	serverPublic := serverPrivate.PublicKey()

	// Client generates session key
	sessionKey := make([]byte, AES128KeySize)
	_, err = rand.Read(sessionKey)
	if err != nil {
		t.Fatalf("rand.Read() error = %v", err)
	}

	// Client encrypts session key with server's public key
	encryptedSession, err := serverPublic.Encrypt(sessionKey)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	// Server decrypts session key
	decryptedSession, err := serverPrivate.Decrypt(encryptedSession)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}

	// Verify session key matches
	if !bytes.Equal(decryptedSession, sessionKey) {
		t.Error("Decrypted session key doesn't match original")
	}

	// Both parties now have the same session key
	// Test symmetric encryption with session key
	message := []byte("Encrypted message using shared session key")
	iv := make([]byte, 16)
	_, err = rand.Read(iv)
	if err != nil {
		t.Fatalf("rand.Read() error = %v", err)
	}

	// Client encrypts with session key
	cipher, err := NewAESCTRCipher(sessionKey, iv)
	if err != nil {
		t.Fatalf("NewAESCTRCipher() error = %v", err)
	}

	encrypted := make([]byte, len(message))
	copy(encrypted, message)
	cipher.Encrypt(encrypted)

	// Server decrypts with session key
	decipher, err := NewAESCTRCipher(decryptedSession, iv)
	if err != nil {
		t.Fatalf("NewAESCTRCipher() error = %v", err)
	}

	decrypted := make([]byte, len(encrypted))
	copy(decrypted, encrypted)
	decipher.Decrypt(decrypted)

	if !bytes.Equal(decrypted, message) {
		t.Error("Hybrid encryption communication failed")
	}
}

// TestHybridEncryptionWithMultipleKeys verifies multiple session keys can be transported
func TestHybridEncryptionWithMultipleKeys(t *testing.T) {
	privateKey, err := GenerateRSAKey(1024)
	if err != nil {
		t.Fatalf("GenerateRSAKey() error = %v", err)
	}
	publicKey := privateKey.PublicKey()

	// Generate and transport multiple keys (simulating multi-hop key exchange)
	numKeys := 3
	originalKeys := make([][]byte, numKeys)
	encryptedKeys := make([][]byte, numKeys)

	for i := 0; i < numKeys; i++ {
		key := make([]byte, AES128KeySize)
		_, err = rand.Read(key)
		if err != nil {
			t.Fatalf("rand.Read() error = %v", err)
		}
		originalKeys[i] = key

		encrypted, err := publicKey.Encrypt(key)
		if err != nil {
			t.Fatalf("Encrypt key %d error = %v", i, err)
		}
		encryptedKeys[i] = encrypted
	}

	// Decrypt and verify each key
	for i := 0; i < numKeys; i++ {
		decrypted, err := privateKey.Decrypt(encryptedKeys[i])
		if err != nil {
			t.Fatalf("Decrypt key %d error = %v", i, err)
		}

		if !bytes.Equal(decrypted, originalKeys[i]) {
			t.Errorf("Key %d: decrypted doesn't match original", i)
		}
	}
}

// TestRSAOAEPEmptyMessage verifies handling of empty message
func TestRSAOAEPEmptyMessage(t *testing.T) {
	privateKey, err := GenerateRSAKey(1024)
	if err != nil {
		t.Fatalf("GenerateRSAKey() error = %v", err)
	}

	publicKey := privateKey.PublicKey()
	plaintext := []byte{}

	// Encrypt empty message
	ciphertext, err := publicKey.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt empty message error = %v", err)
	}

	// Decrypt
	decrypted, err := privateKey.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt error = %v", err)
	}

	// Verify empty plaintext
	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("Decrypted = %v, want empty", decrypted)
	}
}

// TestRSAOAEPCorruptedCiphertext verifies error handling for corrupted data
func TestRSAOAEPCorruptedCiphertext(t *testing.T) {
	privateKey, err := GenerateRSAKey(1024)
	if err != nil {
		t.Fatalf("GenerateRSAKey() error = %v", err)
	}

	publicKey := privateKey.PublicKey()
	plaintext := []byte("Valid message")

	ciphertext, err := publicKey.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	// Corrupt the ciphertext
	corruptedCiphertext := make([]byte, len(ciphertext))
	copy(corruptedCiphertext, ciphertext)
	corruptedCiphertext[0] ^= 0xFF
	corruptedCiphertext[len(corruptedCiphertext)-1] ^= 0xFF

	// Decryption should fail
	_, err = privateKey.Decrypt(corruptedCiphertext)
	if err == nil {
		t.Error("Decrypt should fail with corrupted ciphertext")
	}
}
