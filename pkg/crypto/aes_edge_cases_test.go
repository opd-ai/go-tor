package crypto

import (
	"bytes"
	"crypto/aes"
	"testing"
)

// TestNewAESCTRCipher_InvalidIVLength tests that NewAESCTRCipher returns an error with invalid IV length.
// Previously cipher.NewCTR would panic; now NewAESCTRCipher validates the IV and returns an error.
func TestNewAESCTRCipher_InvalidIVLength(t *testing.T) {
	key := make([]byte, 16)

	tests := []struct {
		name      string
		ivLen     int
		wantError bool
	}{
		{"valid IV (16 bytes)", 16, false},
		{"short IV (8 bytes)", 8, true},
		{"long IV (32 bytes)", 32, true},
		{"zero IV (0 bytes)", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			iv := make([]byte, tt.ivLen)

			c, err := NewAESCTRCipher(key, iv)
			if tt.wantError {
				if err == nil {
					t.Error("Expected error with invalid IV length, got none")
				}
			} else {
				if err != nil {
					t.Fatalf("NewAESCTRCipher failed with valid IV: %v", err)
				}
				if c == nil {
					t.Error("NewAESCTRCipher returned nil cipher")
				}
			}
		})
	}
}

// TestNewAESCTRCipher_InvalidKeyLength tests various key lengths
func TestNewAESCTRCipher_InvalidKeyLength(t *testing.T) {
	iv := make([]byte, aes.BlockSize)

	tests := []struct {
		name    string
		keyLen  int
		wantErr bool
	}{
		{"AES-128 (16 bytes)", 16, false},
		{"AES-192 (24 bytes)", 24, false},
		{"AES-256 (32 bytes)", 32, false},
		{"invalid (8 bytes)", 8, true},
		{"invalid (12 bytes)", 12, true},
		{"invalid (20 bytes)", 20, true},
		{"invalid (0 bytes)", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := make([]byte, tt.keyLen)

			cipher, err := NewAESCTRCipher(key, iv)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error with invalid key length, got nil")
				}
				if cipher != nil {
					t.Error("Expected nil cipher with invalid key, got non-nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error with valid key length: %v", err)
				}
				if cipher == nil {
					t.Error("Expected non-nil cipher with valid key, got nil")
				}
			}
		})
	}
}

// TestAESCTRCipher_128BitKey specifically tests AES-128 as required by tor-spec.txt §5.1
func TestAESCTRCipher_128BitKey(t *testing.T) {
	// tor-spec.txt §5.1: "The encryption algorithm is AES-128-CTR, with a 128-bit key."
	key := make([]byte, AES128KeySize) // 16 bytes = 128 bits
	iv := make([]byte, aes.BlockSize)

	// Fill with test pattern
	for i := range key {
		key[i] = byte(i)
	}

	plaintext := []byte("Tor relay cell payload encryption test")

	// Encrypt
	cipher1, err := NewAESCTRCipher(key, iv)
	if err != nil {
		t.Fatalf("NewAESCTRCipher with 128-bit key failed: %v", err)
	}

	ciphertext := make([]byte, len(plaintext))
	copy(ciphertext, plaintext)
	cipher1.Encrypt(ciphertext)

	// Verify ciphertext differs from plaintext
	if bytes.Equal(ciphertext, plaintext) {
		t.Error("Ciphertext should differ from plaintext")
	}

	// Decrypt
	cipher2, err := NewAESCTRCipher(key, iv)
	if err != nil {
		t.Fatalf("NewAESCTRCipher with 128-bit key failed: %v", err)
	}

	decrypted := make([]byte, len(ciphertext))
	copy(decrypted, ciphertext)
	cipher2.Decrypt(decrypted)

	// Verify round trip
	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("Decrypted text doesn't match original\nGot: %x\nWant: %x", decrypted, plaintext)
	}
}

// TestAESCTRCipher_ZeroIV tests encryption with zero IV as required by tor-spec.txt §5.1.1
func TestAESCTRCipher_ZeroIV(t *testing.T) {
	// tor-spec.txt §5.1.1: Circuit-level encryption uses zero IV
	key := make([]byte, 16)
	zeroIV := make([]byte, 16) // Zero-initialized

	plaintext := []byte("Test message with zero IV")

	cipher, err := NewAESCTRCipher(key, zeroIV)
	if err != nil {
		t.Fatalf("NewAESCTRCipher with zero IV failed: %v", err)
	}

	ciphertext := make([]byte, len(plaintext))
	copy(ciphertext, plaintext)
	cipher.Encrypt(ciphertext)

	// Verify encryption occurred
	if bytes.Equal(ciphertext, plaintext) {
		t.Error("Encryption with zero IV produced no change")
	}

	// Verify deterministic encryption (same key + IV produces same output)
	cipher2, err := NewAESCTRCipher(key, zeroIV)
	if err != nil {
		t.Fatalf("Second NewAESCTRCipher failed: %v", err)
	}

	ciphertext2 := make([]byte, len(plaintext))
	copy(ciphertext2, plaintext)
	cipher2.Encrypt(ciphertext2)

	if !bytes.Equal(ciphertext, ciphertext2) {
		t.Error("Same key and IV should produce same ciphertext")
	}
}

// TestAESCTRCipher_InPlaceVsCopy verifies both in-place and copy-based encryption work identically
func TestAESCTRCipher_InPlaceVsCopy(t *testing.T) {
	key := make([]byte, 16)
	iv := make([]byte, 16)
	plaintext := []byte("Test message for in-place vs copy comparison")

	// In-place encryption (modifies buffer)
	cipher1, err := NewAESCTRCipher(key, iv)
	if err != nil {
		t.Fatalf("NewAESCTRCipher failed: %v", err)
	}

	inPlace := make([]byte, len(plaintext))
	copy(inPlace, plaintext)
	cipher1.Encrypt(inPlace) // Modifies inPlace

	// Copy-based encryption (separate source and destination)
	cipher2, err := NewAESCTRCipher(key, iv)
	if err != nil {
		t.Fatalf("Second NewAESCTRCipher failed: %v", err)
	}

	source := make([]byte, len(plaintext))
	copy(source, plaintext)
	dest := make([]byte, len(plaintext))
	cipher2.stream.XORKeyStream(dest, source)

	// Both methods should produce identical ciphertext
	if !bytes.Equal(inPlace, dest) {
		t.Errorf("In-place and copy-based encryption differ\nIn-place: %x\nCopy: %x", inPlace, dest)
	}
}

// TestAESCTRCipher_StreamInterface verifies the Stream() method returns usable cipher.Stream
func TestAESCTRCipher_StreamInterface(t *testing.T) {
	key := make([]byte, 16)
	iv := make([]byte, 16)

	cipher, err := NewAESCTRCipher(key, iv)
	if err != nil {
		t.Fatalf("NewAESCTRCipher failed: %v", err)
	}

	// Get the stream interface
	stream := cipher.Stream()
	if stream == nil {
		t.Fatal("Stream() returned nil")
	}

	// Use the stream interface directly
	plaintext := []byte("Test stream interface")
	ciphertext := make([]byte, len(plaintext))
	stream.XORKeyStream(ciphertext, plaintext)

	// Verify encryption occurred
	if bytes.Equal(ciphertext, plaintext) {
		t.Error("Stream interface encryption produced no change")
	}

	// Decrypt using another cipher instance
	cipher2, err := NewAESCTRCipher(key, iv)
	if err != nil {
		t.Fatalf("Second NewAESCTRCipher failed: %v", err)
	}

	decrypted := make([]byte, len(ciphertext))
	cipher2.Stream().XORKeyStream(decrypted, ciphertext)

	if !bytes.Equal(decrypted, plaintext) {
		t.Error("Stream interface round trip failed")
	}
}

// TestAESCTRCipher_VariousLengths tests encryption of various payload sizes
func TestAESCTRCipher_VariousLengths(t *testing.T) {
	key := make([]byte, 16)
	iv := make([]byte, 16)

	// Test various lengths including edge cases
	lengths := []int{
		0,    // Empty (edge case)
		1,    // Single byte
		15,   // One byte less than block size
		16,   // Exactly one block
		17,   // One byte more than block size
		498,  // Tor relay cell payload size
		512,  // Standard buffer size
		1024, // Large payload
	}

	for _, length := range lengths {
		t.Run(string(rune(length)), func(t *testing.T) {
			plaintext := make([]byte, length)
			for i := range plaintext {
				plaintext[i] = byte(i % 256)
			}

			// Encrypt
			cipher1, err := NewAESCTRCipher(key, iv)
			if err != nil {
				t.Fatalf("NewAESCTRCipher failed: %v", err)
			}

			ciphertext := make([]byte, length)
			copy(ciphertext, plaintext)
			cipher1.Encrypt(ciphertext)

			// For non-empty plaintexts, verify encryption changed data
			if length > 0 && bytes.Equal(ciphertext, plaintext) {
				t.Error("Encryption produced no change for non-empty plaintext")
			}

			// Decrypt
			cipher2, err := NewAESCTRCipher(key, iv)
			if err != nil {
				t.Fatalf("Second NewAESCTRCipher failed: %v", err)
			}

			decrypted := make([]byte, length)
			copy(decrypted, ciphertext)
			cipher2.Decrypt(decrypted)

			// Verify round trip
			if !bytes.Equal(decrypted, plaintext) {
				t.Errorf("Round trip failed for length %d", length)
			}
		})
	}
}

// TestAESCTRCipher_MultipleOperations tests sequential encryption operations
func TestAESCTRCipher_MultipleOperations(t *testing.T) {
	key := make([]byte, 16)
	iv := make([]byte, 16)

	cipher, err := NewAESCTRCipher(key, iv)
	if err != nil {
		t.Fatalf("NewAESCTRCipher failed: %v", err)
	}

	// Encrypt multiple blocks sequentially
	block1 := []byte("First block of 16b!") // 19 bytes
	block2 := []byte("Second block!!!!!!!") // 19 bytes

	cipher.Encrypt(block1)
	cipher.Encrypt(block2)

	// Decrypt with fresh cipher (same key/IV)
	cipher2, err := NewAESCTRCipher(key, iv)
	if err != nil {
		t.Fatalf("Second NewAESCTRCipher failed: %v", err)
	}

	// Decrypt as single stream
	combined := append([]byte{}, block1...)
	combined = append(combined, block2...)

	expected := []byte("First block of 16b!Second block!!!!!!!")
	decrypted := make([]byte, len(combined))
	copy(decrypted, combined)
	cipher2.Decrypt(decrypted)

	if !bytes.Equal(decrypted, expected) {
		t.Errorf("Multiple operations decrypt failed\nGot: %s\nWant: %s", decrypted, expected)
	}
}
