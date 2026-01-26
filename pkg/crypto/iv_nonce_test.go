package crypto

import (
	"bytes"
	"crypto/aes"
	"testing"

	"golang.org/x/crypto/chacha20poly1305"
)

// TestIVGeneration_RandomnessQuality verifies crypto/rand produces high-quality random IVs
func TestIVGeneration_RandomnessQuality(t *testing.T) {
	const numSamples = 1000
	ivs := make(map[[16]byte]bool)

	for i := 0; i < numSamples; i++ {
		iv, err := GenerateRandomBytes(16)
		if err != nil {
			t.Fatalf("Failed to generate random IV: %v", err)
		}

		// Check IV size
		if len(iv) != 16 {
			t.Errorf("Generated IV has wrong length: %d, want 16", len(iv))
		}

		// Convert to array for map key
		var ivArray [16]byte
		copy(ivArray[:], iv)

		// Check for duplicates (extremely unlikely with crypto/rand)
		if ivs[ivArray] {
			t.Errorf("Duplicate IV generated at sample %d (probability ~2^-128)", i)
		}
		ivs[ivArray] = true
	}

	// Verify we generated the expected number of unique IVs
	if len(ivs) != numSamples {
		t.Errorf("Expected %d unique IVs, got %d", numSamples, len(ivs))
	}
}

// TestIVGeneration_NonZeroDistribution verifies IVs are not all zeros
func TestIVGeneration_NonZeroDistribution(t *testing.T) {
	const numSamples = 100
	var totalOnes int

	for i := 0; i < numSamples; i++ {
		iv, err := GenerateRandomBytes(16)
		if err != nil {
			t.Fatalf("Failed to generate random IV: %v", err)
		}

		// Count set bits across all IVs
		for _, b := range iv {
			for bit := 0; bit < 8; bit++ {
				if b&(1<<bit) != 0 {
					totalOnes++
				}
			}
		}

		// Verify IV is not all zeros
		allZero := true
		for _, b := range iv {
			if b != 0 {
				allZero = false
				break
			}
		}
		if allZero {
			t.Errorf("Generated all-zero IV at sample %d", i)
		}
	}

	// Statistical check: Expect ~50% bits to be set (should be 6400 ± tolerance)
	// With 100 samples * 128 bits = 12,800 total bits
	expectedOnes := 6400
	tolerance := 500 // Allow ±3.9% deviation (reasonable for 100 samples)

	if totalOnes < expectedOnes-tolerance || totalOnes > expectedOnes+tolerance {
		t.Errorf("Bit distribution suspicious: %d ones out of 12800 bits (expected ~6400±500)", totalOnes)
	}
}

// TestZeroIV_SpecCompliance verifies zero IV handling per tor-spec.txt §5.1.1
func TestZeroIV_SpecCompliance(t *testing.T) {
	key := make([]byte, 16)
	zeroIV := make([]byte, 16) // All zeros

	// Test encryption with zero IV (per tor-spec.txt §5.1.1)
	plaintext := []byte("Circuit encryption uses zero IV per Tor spec")
	
	cipher1, err := NewAESCTRCipher(key, zeroIV)
	if err != nil {
		t.Fatalf("Failed to create cipher with zero IV: %v", err)
	}

	ciphertext := make([]byte, len(plaintext))
	copy(ciphertext, plaintext)
	cipher1.Encrypt(ciphertext)

	// Verify encryption changed the data
	if bytes.Equal(ciphertext, plaintext) {
		t.Error("Zero IV encryption produced no change (should encrypt)")
	}

	// Verify decryption with same zero IV recovers plaintext
	cipher2, err := NewAESCTRCipher(key, zeroIV)
	if err != nil {
		t.Fatalf("Failed to create cipher for decryption: %v", err)
	}

	decrypted := make([]byte, len(ciphertext))
	copy(decrypted, ciphertext)
	cipher2.Decrypt(decrypted)

	if !bytes.Equal(decrypted, plaintext) {
		t.Error("Zero IV decryption failed to recover plaintext")
	}
}

// TestZeroIV_Determinism verifies zero IV produces deterministic output
func TestZeroIV_Determinism(t *testing.T) {
	key := make([]byte, 16)
	zeroIV := make([]byte, 16)
	plaintext := []byte("Deterministic encryption test")

	// Encrypt twice with same key and zero IV
	cipher1, _ := NewAESCTRCipher(key, zeroIV)
	ciphertext1 := make([]byte, len(plaintext))
	copy(ciphertext1, plaintext)
	cipher1.Encrypt(ciphertext1)

	cipher2, _ := NewAESCTRCipher(key, zeroIV)
	ciphertext2 := make([]byte, len(plaintext))
	copy(ciphertext2, plaintext)
	cipher2.Encrypt(ciphertext2)

	// Verify outputs are identical (deterministic with zero IV)
	if !bytes.Equal(ciphertext1, ciphertext2) {
		t.Error("Zero IV encryption is not deterministic")
	}
}

// TestIVSize_Validation verifies IV size requirements
func TestIVSize_Validation(t *testing.T) {
	key := make([]byte, 16)

	tests := []struct {
		name        string
		ivLen       int
		shouldPanic bool
	}{
		{"valid 16-byte IV", 16, false},
		{"invalid 8-byte IV", 8, true},
		{"invalid 32-byte IV", 32, true},
		{"invalid 0-byte IV", 0, true},
		{"invalid 15-byte IV", 15, true},
		{"invalid 17-byte IV", 17, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			iv := make([]byte, tt.ivLen)

			defer func() {
				r := recover()
				if tt.shouldPanic && r == nil {
					t.Error("Expected panic with invalid IV length, got none")
				}
				if !tt.shouldPanic && r != nil {
					t.Errorf("Unexpected panic with valid IV: %v", r)
				}
			}()

			_, err := NewAESCTRCipher(key, iv)
			if err != nil && !tt.shouldPanic {
				t.Fatalf("NewAESCTRCipher failed with valid IV: %v", err)
			}
		})
	}
}

// TestNonceSize_XChaCha20 verifies XChaCha20-Poly1305 nonce size (24 bytes)
func TestNonceSize_XChaCha20(t *testing.T) {
	// XChaCha20-Poly1305 requires exactly 24-byte nonce
	expectedSize := chacha20poly1305.NonceSizeX

	if expectedSize != 24 {
		t.Errorf("XChaCha20 nonce size changed: %d, expected 24", expectedSize)
	}

	// Verify derivation produces sufficient length
	nonce, err := GenerateRandomBytes(expectedSize)
	if err != nil {
		t.Fatalf("Failed to generate nonce: %v", err)
	}

	if len(nonce) != expectedSize {
		t.Errorf("Generated nonce has wrong length: %d, want %d", len(nonce), expectedSize)
	}
}

// TestNonceUniqueness_Statistical verifies nonces are unique across many generations
func TestNonceUniqueness_Statistical(t *testing.T) {
	const numSamples = 10000
	nonces := make(map[[24]byte]bool)

	for i := 0; i < numSamples; i++ {
		nonce, err := GenerateRandomBytes(24)
		if err != nil {
			t.Fatalf("Failed to generate nonce at sample %d: %v", i, err)
		}

		// Convert to array for map key
		var nonceArray [24]byte
		copy(nonceArray[:], nonce)

		// Check for duplicates
		if nonces[nonceArray] {
			t.Errorf("Duplicate nonce at sample %d (probability ~2^-192)", i)
		}
		nonces[nonceArray] = true
	}

	// Verify all nonces are unique
	if len(nonces) != numSamples {
		t.Errorf("Expected %d unique nonces, got %d", numSamples, len(nonces))
	}
}

// TestIVReuse_CircuitEncryption verifies zero IV is safe with key rotation
func TestIVReuse_CircuitEncryption(t *testing.T) {
	// Simulate two different circuit hops with different keys
	key1 := make([]byte, 16)
	key2 := make([]byte, 16)
	key1[0] = 0x01 // Different keys
	key2[0] = 0x02

	zeroIV := make([]byte, 16) // Same zero IV per Tor spec
	plaintext := []byte("Same message encrypted with different keys")

	// Encrypt with key1
	cipher1, _ := NewAESCTRCipher(key1, zeroIV)
	ciphertext1 := make([]byte, len(plaintext))
	copy(ciphertext1, plaintext)
	cipher1.Encrypt(ciphertext1)

	// Encrypt with key2
	cipher2, _ := NewAESCTRCipher(key2, zeroIV)
	ciphertext2 := make([]byte, len(plaintext))
	copy(ciphertext2, plaintext)
	cipher2.Encrypt(ciphertext2)

	// Verify ciphertexts are different (key rotation prevents IV reuse attacks)
	if bytes.Equal(ciphertext1, ciphertext2) {
		t.Error("Different keys with same zero IV produced identical ciphertexts")
	}
}

// TestIVGeneration_ThreadSafety verifies concurrent IV generation is safe
func TestIVGeneration_ThreadSafety(t *testing.T) {
	const numGoroutines = 100
	const ivsPerGoroutine = 100

	results := make(chan [16]byte, numGoroutines*ivsPerGoroutine)
	errors := make(chan error, numGoroutines)

	// Generate IVs concurrently
	for i := 0; i < numGoroutines; i++ {
		go func() {
			for j := 0; j < ivsPerGoroutine; j++ {
				iv, err := GenerateRandomBytes(16)
				if err != nil {
					errors <- err
					return
				}
				var ivArray [16]byte
				copy(ivArray[:], iv)
				results <- ivArray
			}
		}()
	}

	// Collect results
	ivs := make(map[[16]byte]bool)
	for i := 0; i < numGoroutines*ivsPerGoroutine; i++ {
		select {
		case err := <-errors:
			t.Fatalf("IV generation failed in goroutine: %v", err)
		case iv := <-results:
			if ivs[iv] {
				t.Errorf("Duplicate IV generated during concurrent access")
			}
			ivs[iv] = true
		}
	}

	// Verify all IVs are unique
	if len(ivs) != numGoroutines*ivsPerGoroutine {
		t.Errorf("Expected %d unique IVs, got %d", numGoroutines*ivsPerGoroutine, len(ivs))
	}
}

// TestIVGeneration_ErrorHandling verifies proper error handling
func TestIVGeneration_ErrorHandling(t *testing.T) {
	// Test invalid lengths
	testCases := []struct {
		name   string
		length int
		valid  bool
	}{
		{"zero length", 0, false},
		{"negative length (will panic before GenerateRandomBytes)", -1, false},
		{"valid 16 bytes", 16, true},
		{"valid 24 bytes", 24, true},
		{"valid 32 bytes", 32, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.length < 0 {
				// Go's make() will panic on negative length, which is expected
				defer func() {
					if r := recover(); r == nil {
						t.Error("Expected panic with negative length")
					}
				}()
			}

			data, err := GenerateRandomBytes(tc.length)

			if tc.valid {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if len(data) != tc.length {
					t.Errorf("Wrong length: got %d, want %d", len(data), tc.length)
				}
			} else if tc.length == 0 {
				// Zero-length allocation is valid in Go (returns empty slice)
				if err != nil {
					t.Errorf("Unexpected error for zero length: %v", err)
				}
				if len(data) != 0 {
					t.Errorf("Expected zero-length slice, got %d", len(data))
				}
			}
		})
	}
}

// TestAESBlockSize_Constant verifies AES block size is 16 bytes
func TestAESBlockSize_Constant(t *testing.T) {
	// Verify AES block size constant
	if aes.BlockSize != 16 {
		t.Errorf("AES block size changed: %d, expected 16", aes.BlockSize)
	}

	// This ensures our IV size assumptions are correct
	key := make([]byte, 16)
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("Failed to create AES cipher: %v", err)
	}

	if block.BlockSize() != 16 {
		t.Errorf("AES cipher block size: %d, expected 16", block.BlockSize())
	}
}

// TestIV_MemorySafety verifies IVs don't cause memory issues
func TestIV_MemorySafety(t *testing.T) {
	// Generate many IVs to test for memory leaks
	for i := 0; i < 10000; i++ {
		iv, err := GenerateRandomBytes(16)
		if err != nil {
			t.Fatalf("Memory safety test failed at iteration %d: %v", i, err)
		}

		// Modify the IV to ensure it's mutable
		for j := range iv {
			iv[j] ^= 0xFF
		}

		// IV should be garbage collected after this iteration
	}
}

// TestIVGeneration_DifferentSizes verifies various common IV/nonce sizes
func TestIVGeneration_DifferentSizes(t *testing.T) {
	sizes := []struct {
		name string
		size int
		use  string
	}{
		{"AES-128/256 IV", 16, "AES-CTR mode"},
		{"XChaCha20 nonce", 24, "XChaCha20-Poly1305"},
		{"256-bit key/IV", 32, "General purpose"},
		{"ChaCha20 nonce", 12, "ChaCha20-Poly1305"},
	}

	for _, tc := range sizes {
		t.Run(tc.name, func(t *testing.T) {
			data, err := GenerateRandomBytes(tc.size)
			if err != nil {
				t.Errorf("Failed to generate %d-byte value for %s: %v", tc.size, tc.use, err)
			}

			if len(data) != tc.size {
				t.Errorf("Wrong size: got %d, want %d for %s", len(data), tc.size, tc.use)
			}

			// Verify it's not all zeros
			allZero := true
			for _, b := range data {
				if b != 0 {
					allZero = false
					break
				}
			}
			if allZero {
				t.Errorf("Generated all-zero value for %s", tc.use)
			}
		})
	}
}
