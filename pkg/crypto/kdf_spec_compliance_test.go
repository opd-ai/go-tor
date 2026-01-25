// KDF-TOR Specification Compliance Tests
//
// This file contains comprehensive tests verifying the KDF-TOR key derivation
// function implementation against tor-spec.txt §5.2.
//
// Specification References:
// - tor-spec.txt §5.2: KDF-TOR key derivation function
// - Legacy handshakes (CREATE/CREATE_FAST) use KDF-TOR
// - Modern handshakes (CREATE2/ntor) use HKDF-SHA256 instead

package crypto

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"testing"
)

// TestKDFTORSpecCompliance_Algorithm verifies the KDF-TOR algorithm per tor-spec.txt §5.2
//
// From tor-spec.txt §5.2:
//
//	K = K_0 | K_1 | K_2 | ...
//	Where:
//	- K_0 = H(g^xy)  [in our case: H(secret)]
//	- K_i = H(K_0 | [i]) for i >= 1
//	- H is SHA-1
//	- | is concatenation
//	- [i] is the byte value i
func TestKDFTORSpecCompliance_Algorithm(t *testing.T) {
	t.Run("K_0 is SHA-1 of secret", func(t *testing.T) {
		secret := []byte("test secret")
		expectedK0 := sha1.Sum(secret)

		// Request 20 bytes (one SHA-1 block = K_0)
		key, err := DeriveKey(secret, 20)
		if err != nil {
			t.Fatalf("DeriveKey() error = %v", err)
		}

		if !bytes.Equal(key, expectedK0[:]) {
			t.Errorf("K_0 = H(secret) not satisfied\ngot:  %x\nwant: %x", key, expectedK0)
		}
	})

	t.Run("K_1 is SHA-1 of K_0 || [1]", func(t *testing.T) {
		secret := []byte("test secret")
		k0 := sha1.Sum(secret)

		// Manually compute K_1 per spec: K_1 = H(K_0 | [1])
		data := append(k0[:], byte(1))
		expectedK1 := sha1.Sum(data)

		// Request 40 bytes (K_0 + K_1)
		key, err := DeriveKey(secret, 40)
		if err != nil {
			t.Fatalf("DeriveKey() error = %v", err)
		}

		// Verify K_0 portion
		if !bytes.Equal(key[:20], k0[:]) {
			t.Errorf("K_0 portion incorrect\ngot:  %x\nwant: %x", key[:20], k0)
		}

		// Verify K_1 portion
		if !bytes.Equal(key[20:40], expectedK1[:]) {
			t.Errorf("K_1 = H(K_0 | [1]) not satisfied\ngot:  %x\nwant: %x", key[20:40], expectedK1)
		}
	})

	t.Run("K_i follows iterative formula", func(t *testing.T) {
		secret := []byte("test secret")
		k0 := sha1.Sum(secret)

		// Request 100 bytes (K_0 + K_1 + K_2 + K_3 + K_4)
		key, err := DeriveKey(secret, 100)
		if err != nil {
			t.Fatalf("DeriveKey() error = %v", err)
		}

		// Verify each block follows K_i = H(K_0 | [i])
		expectedBlocks := [][]byte{k0[:]} // K_0

		for i := byte(1); i <= 4; i++ {
			data := append(k0[:], i)
			ki := sha1.Sum(data)
			expectedBlocks = append(expectedBlocks, ki[:])
		}

		// Concatenate expected blocks
		expected := bytes.Join(expectedBlocks, nil)

		if !bytes.Equal(key, expected[:100]) {
			t.Errorf("KDF-TOR iterative formula not satisfied")
			for i, block := range expectedBlocks {
				start := i * 20
				end := start + 20
				if end > len(key) {
					end = len(key)
				}
				if !bytes.Equal(key[start:end], block[:end-start]) {
					t.Errorf("K_%d mismatch\ngot:  %x\nwant: %x", i, key[start:end], block[:end-start])
				}
			}
		}
	})
}

// TestKDFTORSpecCompliance_HashFunction verifies SHA-1 usage per tor-spec.txt §5.2
func TestKDFTORSpecCompliance_HashFunction(t *testing.T) {
	t.Run("Uses SHA-1 hash function", func(t *testing.T) {
		secret := []byte("test secret")
		key, err := DeriveKey(secret, 20)
		if err != nil {
			t.Fatalf("DeriveKey() error = %v", err)
		}

		// Verify output is a SHA-1 hash (20 bytes)
		if len(key) != 20 {
			t.Errorf("Expected 20 bytes (SHA-1 output), got %d", len(key))
		}

		// Verify it matches SHA-1(secret)
		expectedHash := sha1.Sum(secret)
		if !bytes.Equal(key, expectedHash[:]) {
			t.Errorf("Output does not match SHA-1(secret)")
		}
	})

	t.Run("Each block is 20 bytes (SHA-1 output size)", func(t *testing.T) {
		secret := []byte("test")
		// Request 60 bytes (3 SHA-1 blocks)
		key, err := DeriveKey(secret, 60)
		if err != nil {
			t.Fatalf("DeriveKey() error = %v", err)
		}

		// Each block should be exactly 20 bytes
		for i := 0; i < 3; i++ {
			start := i * 20
			end := start + 20
			block := key[start:end]
			if len(block) != 20 {
				t.Errorf("Block %d has length %d, expected 20", i, len(block))
			}
		}
	})
}

// TestKDFTORSpecCompliance_KeyLength verifies key length handling per tor-spec.txt §5.2
func TestKDFTORSpecCompliance_KeyLength(t *testing.T) {
	secret := []byte("test secret")

	tests := []struct {
		name         string
		keyLen       int
		expectBlocks int // Number of SHA-1 blocks needed
		description  string
	}{
		{
			name:         "20 bytes (1 SHA-1 block)",
			keyLen:       20,
			expectBlocks: 1,
			description:  "Exactly K_0",
		},
		{
			name:         "40 bytes (2 SHA-1 blocks)",
			keyLen:       40,
			expectBlocks: 2,
			description:  "K_0 | K_1",
		},
		{
			name:         "72 bytes (standard Tor key material)",
			keyLen:       72,
			expectBlocks: 4,
			description:  "Df (20) + Db (20) + Kf (16) + Kb (16)",
		},
		{
			name:         "100 bytes (5 SHA-1 blocks)",
			keyLen:       100,
			expectBlocks: 5,
			description:  "K_0 | K_1 | K_2 | K_3 | K_4",
		},
		{
			name:         "19 bytes (partial block)",
			keyLen:       19,
			expectBlocks: 1,
			description:  "Truncated K_0",
		},
		{
			name:         "21 bytes (partial block)",
			keyLen:       21,
			expectBlocks: 2,
			description:  "K_0 + 1 byte of K_1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := DeriveKey(secret, tt.keyLen)
			if err != nil {
				t.Fatalf("DeriveKey() error = %v", err)
			}

			if len(key) != tt.keyLen {
				t.Errorf("Expected %d bytes, got %d", tt.keyLen, len(key))
			}

			// Verify it's a truncated version of the full derivation
			fullLen := tt.expectBlocks * 20
			fullKey, err := DeriveKey(secret, fullLen)
			if err != nil {
				t.Fatalf("DeriveKey() error = %v", err)
			}

			if !bytes.Equal(key, fullKey[:tt.keyLen]) {
				t.Errorf("Key is not a truncation of full derivation")
			}
		})
	}
}

// TestKDFTORSpecCompliance_StandardKeyMaterial verifies standard 72-byte key material
// per tor-spec.txt §5.2
//
// Standard key material structure:
//   - Df (20 bytes): Forward digest key (SHA-1)
//   - Db (20 bytes): Backward digest key (SHA-1)
//   - Kf (16 bytes): Forward encryption key (AES-128)
//   - Kb (16 bytes): Backward encryption key (AES-128)
//     Total: 72 bytes
func TestKDFTORSpecCompliance_StandardKeyMaterial(t *testing.T) {
	t.Run("72 bytes produces standard key material", func(t *testing.T) {
		secret := []byte("test secret")
		key, err := DeriveKey(secret, 72)
		if err != nil {
			t.Fatalf("DeriveKey() error = %v", err)
		}

		if len(key) != 72 {
			t.Fatalf("Expected 72 bytes, got %d", len(key))
		}

		// Extract key components
		df := key[0:20]  // Forward digest
		db := key[20:40] // Backward digest
		kf := key[40:56] // Forward encryption key
		kb := key[56:72] // Backward encryption key

		// Verify all components are non-zero
		if isAllZeros(df) {
			t.Error("Forward digest (Df) is all zeros")
		}
		if isAllZeros(db) {
			t.Error("Backward digest (Db) is all zeros")
		}
		if isAllZeros(kf) {
			t.Error("Forward encryption key (Kf) is all zeros")
		}
		if isAllZeros(kb) {
			t.Error("Backward encryption key (Kb) is all zeros")
		}

		// Verify forward and backward digests are different
		if bytes.Equal(df, db) {
			t.Error("Forward and backward digests should be different")
		}

		// Verify forward and backward encryption keys are different
		if bytes.Equal(kf, kb) {
			t.Error("Forward and backward encryption keys should be different")
		}
	})

	t.Run("Key components are derived from iterative formula", func(t *testing.T) {
		secret := []byte("test secret")
		key, err := DeriveKey(secret, 72)
		if err != nil {
			t.Fatalf("DeriveKey() error = %v", err)
		}

		// Manually compute the first 4 SHA-1 blocks
		k0 := sha1.Sum(secret)
		k1 := sha1.Sum(append(k0[:], byte(1)))
		k2 := sha1.Sum(append(k0[:], byte(2)))
		k3 := sha1.Sum(append(k0[:], byte(3)))

		// Concatenate: K_0 | K_1 | K_2 | K_3 = 80 bytes, then truncate to 72
		expected := append(k0[:], k1[:]...)
		expected = append(expected, k2[:]...)
		expected = append(expected, k3[:]...)
		expected = expected[:72]

		if !bytes.Equal(key, expected) {
			t.Errorf("72-byte key material does not match KDF-TOR formula")
			t.Logf("Got:      %x", key)
			t.Logf("Expected: %x", expected)
		}
	})
}

// TestKDFTORSpecCompliance_Determinism verifies deterministic output per tor-spec.txt §5.2
func TestKDFTORSpecCompliance_Determinism(t *testing.T) {
	t.Run("Same secret produces same key", func(t *testing.T) {
		secret := []byte("test secret")
		keyLen := 72

		key1, err := DeriveKey(secret, keyLen)
		if err != nil {
			t.Fatalf("DeriveKey() error = %v", err)
		}

		key2, err := DeriveKey(secret, keyLen)
		if err != nil {
			t.Fatalf("DeriveKey() error = %v", err)
		}

		if !bytes.Equal(key1, key2) {
			t.Error("KDF-TOR is not deterministic")
		}
	})

	t.Run("Different secrets produce different keys", func(t *testing.T) {
		secret1 := []byte("secret1")
		secret2 := []byte("secret2")
		keyLen := 72

		key1, err := DeriveKey(secret1, keyLen)
		if err != nil {
			t.Fatalf("DeriveKey() error = %v", err)
		}

		key2, err := DeriveKey(secret2, keyLen)
		if err != nil {
			t.Fatalf("DeriveKey() error = %v", err)
		}

		if bytes.Equal(key1, key2) {
			t.Error("Different secrets should produce different keys")
		}
	})
}

// TestKDFTORSpecCompliance_TestVectors verifies against known test vectors
func TestKDFTORSpecCompliance_TestVectors(t *testing.T) {
	// Test vector: simple case with known secret
	secret := []byte("hello")
	k0 := sha1.Sum(secret)

	tests := []struct {
		name     string
		keyLen   int
		expected string // hex-encoded expected output
	}{
		{
			name:     "20 bytes (K_0 only)",
			keyLen:   20,
			expected: hex.EncodeToString(k0[:]),
		},
		{
			name:   "40 bytes (K_0 | K_1)",
			keyLen: 40,
			expected: func() string {
				k1 := sha1.Sum(append(k0[:], byte(1)))
				return hex.EncodeToString(append(k0[:], k1[:]...))
			}(),
		},
		{
			name:   "72 bytes (standard key material)",
			keyLen: 72,
			expected: func() string {
				k1 := sha1.Sum(append(k0[:], byte(1)))
				k2 := sha1.Sum(append(k0[:], byte(2)))
				k3 := sha1.Sum(append(k0[:], byte(3)))
				result := append(k0[:], k1[:]...)
				result = append(result, k2[:]...)
				result = append(result, k3[:]...)
				return hex.EncodeToString(result[:72])
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := DeriveKey(secret, tt.keyLen)
			if err != nil {
				t.Fatalf("DeriveKey() error = %v", err)
			}

			got := hex.EncodeToString(key)
			if got != tt.expected {
				t.Errorf("Test vector mismatch\ngot:  %s\nwant: %s", got, tt.expected)
			}
		})
	}
}

// TestKDFTORSpecCompliance_EdgeCases verifies edge case handling
func TestKDFTORSpecCompliance_EdgeCases(t *testing.T) {
	t.Run("Empty secret is valid", func(t *testing.T) {
		secret := []byte{}
		key, err := DeriveKey(secret, 20)
		if err != nil {
			t.Fatalf("DeriveKey() with empty secret error = %v", err)
		}

		// Should produce SHA-1(empty)
		expectedK0 := sha1.Sum(secret)
		if !bytes.Equal(key, expectedK0[:]) {
			t.Error("Empty secret should produce SHA-1(empty)")
		}
	})

	t.Run("Single byte key length", func(t *testing.T) {
		secret := []byte("test")
		key, err := DeriveKey(secret, 1)
		if err != nil {
			t.Fatalf("DeriveKey() error = %v", err)
		}

		if len(key) != 1 {
			t.Errorf("Expected 1 byte, got %d", len(key))
		}

		// Should be first byte of K_0
		k0 := sha1.Sum(secret)
		if key[0] != k0[0] {
			t.Error("Single byte should be first byte of K_0")
		}
	})

	t.Run("Large key length (1000 bytes)", func(t *testing.T) {
		secret := []byte("test")
		keyLen := 1000
		key, err := DeriveKey(secret, keyLen)
		if err != nil {
			t.Fatalf("DeriveKey() error = %v", err)
		}

		if len(key) != keyLen {
			t.Errorf("Expected %d bytes, got %d", keyLen, len(key))
		}

		// Should require ceil(1000/20) = 50 blocks (K_0 through K_49)
		// Verify first 20 bytes are K_0
		k0 := sha1.Sum(secret)
		if !bytes.Equal(key[:20], k0[:]) {
			t.Error("First 20 bytes should be K_0")
		}
	})

	t.Run("Invalid key length (zero)", func(t *testing.T) {
		secret := []byte("test")
		_, err := DeriveKey(secret, 0)
		if err == nil {
			t.Error("Expected error for zero key length")
		}
	})

	t.Run("Invalid key length (negative)", func(t *testing.T) {
		secret := []byte("test")
		_, err := DeriveKey(secret, -1)
		if err == nil {
			t.Error("Expected error for negative key length")
		}
	})
}

// TestKDFTORSpecCompliance_Concatenation verifies concatenation operation
func TestKDFTORSpecCompliance_Concatenation(t *testing.T) {
	t.Run("Multiple blocks are concatenated correctly", func(t *testing.T) {
		secret := []byte("test")
		keyLen := 60 // 3 blocks

		key, err := DeriveKey(secret, keyLen)
		if err != nil {
			t.Fatalf("DeriveKey() error = %v", err)
		}

		// Manually compute and concatenate 3 blocks
		k0 := sha1.Sum(secret)
		k1 := sha1.Sum(append(k0[:], byte(1)))
		k2 := sha1.Sum(append(k0[:], byte(2)))

		expected := append(k0[:], k1[:]...)
		expected = append(expected, k2[:]...)

		if !bytes.Equal(key, expected) {
			t.Error("Blocks are not concatenated correctly")
		}
	})

	t.Run("Partial blocks are truncated correctly", func(t *testing.T) {
		secret := []byte("test")
		keyLen := 25 // K_0 (20 bytes) + 5 bytes of K_1

		key, err := DeriveKey(secret, keyLen)
		if err != nil {
			t.Fatalf("DeriveKey() error = %v", err)
		}

		// Verify it's exactly K_0 | first 5 bytes of K_1
		k0 := sha1.Sum(secret)
		k1 := sha1.Sum(append(k0[:], byte(1)))

		expected := append(k0[:], k1[:5]...)

		if !bytes.Equal(key, expected) {
			t.Error("Partial block truncation incorrect")
			t.Logf("Got:      %x", key)
			t.Logf("Expected: %x", expected)
		}
	})
}

// isAllZeros checks if a byte slice is all zeros
func isAllZeros(b []byte) bool {
	for _, v := range b {
		if v != 0 {
			return false
		}
	}
	return true
}
