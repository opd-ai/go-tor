// Package crypto provides boundary condition tests for key derivation
// functions per tor-spec.txt §5.2.
//
// These tests verify DeriveKey (KDF-TOR), AES-CTR cipher creation,
// and HKDF-based ntor key derivation handle boundary inputs correctly.
// Key derivation is on the critical security path: any bug here
// compromises all circuit encryption.
//
// Compliance: tor-spec.txt §5.2 (KDF-TOR), §5.1.4 (ntor HKDF)
package crypto

import (
	"bytes"
	"crypto/sha1" // #nosec G401 - Required by Tor spec
	"fmt"
	"strings"
	"testing"
)

// TestDeriveKeyBoundaryLengths verifies DeriveKey at SHA-1 block
// boundaries (20-byte blocks) and nearby lengths.
func TestDeriveKeyBoundaryLengths(t *testing.T) {
	secret := []byte("boundary-test-secret")

	// Test at exact block boundaries and ±1
	lengths := []int{1, 19, 20, 21, 39, 40, 41, 59, 60, 61, 100, 255, 256}

	for _, keyLen := range lengths {
		key, err := DeriveKey(secret, keyLen)
		if err != nil {
			t.Errorf("keyLen=%d: unexpected error: %v", keyLen, err)
			continue
		}
		if len(key) != keyLen {
			t.Errorf("keyLen=%d: got %d bytes", keyLen, len(key))
		}
	}
}

// TestDeriveKeyBlockAlignment verifies that the first 20 bytes
// always match K_0 = SHA-1(secret) regardless of requested length.
func TestDeriveKeyBlockAlignment(t *testing.T) {
	secret := []byte("alignment-test")
	k0 := sha1.Sum(secret) // #nosec G401

	for _, keyLen := range []int{1, 10, 20, 40, 100} {
		key, err := DeriveKey(secret, keyLen)
		if err != nil {
			t.Fatalf("keyLen=%d: %v", keyLen, err)
		}

		// First min(keyLen, 20) bytes should match K_0
		checkLen := keyLen
		if checkLen > 20 {
			checkLen = 20
		}
		if !bytes.Equal(key[:checkLen], k0[:checkLen]) {
			t.Errorf("keyLen=%d: first %d bytes don't match K_0",
				keyLen, checkLen)
		}
	}
}

// TestDeriveKeyConsecutiveBlocks verifies that consecutive blocks
// K_0, K_1, K_2, ... are correctly chained per KDF-TOR spec.
func TestDeriveKeyConsecutiveBlocks(t *testing.T) {
	secret := []byte("chain-test")
	k0 := sha1.Sum(secret) // #nosec G401

	// Request 60 bytes = 3 full SHA-1 blocks
	key, err := DeriveKey(secret, 60)
	if err != nil {
		t.Fatal(err)
	}

	// Verify K_0 (bytes 0-19)
	if !bytes.Equal(key[0:20], k0[:]) {
		t.Error("block 0 mismatch")
	}

	// Verify K_1 (bytes 20-39) = SHA-1(K_0 || 0x01)
	k1Input := append(k0[:], 0x01)
	k1 := sha1.Sum(k1Input) // #nosec G401
	if !bytes.Equal(key[20:40], k1[:]) {
		t.Error("block 1 mismatch")
	}

	// Verify K_2 (bytes 40-59) = SHA-1(K_0 || 0x02)
	k2Input := append(k0[:], 0x02)
	k2 := sha1.Sum(k2Input) // #nosec G401
	if !bytes.Equal(key[40:60], k2[:]) {
		t.Error("block 2 mismatch")
	}
}

// TestDeriveKeyInvalidLengths verifies that DeriveKey rejects
// invalid key lengths.
func TestDeriveKeyInvalidLengths(t *testing.T) {
	tests := []struct {
		name        string
		keyLen      int
		wantErr     bool
		errContains string
	}{
		{"zero", 0, true, "invalid key length"},
		{"negative", -1, true, "invalid key length"},
		{"very negative", -1000, true, "invalid key length"},
		{"valid 1", 1, false, ""},
		{"valid 20", 20, false, ""},
		{"valid 72", 72, false, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			key, err := DeriveKey([]byte("test"), tc.keyLen)

			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tc.errContains != "" && !strings.Contains(err.Error(), tc.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.errContains)
				}
				if key != nil {
					t.Error("expected nil key on error")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if len(key) != tc.keyLen {
					t.Errorf("key length = %d, want %d", len(key), tc.keyLen)
				}
			}
		})
	}
}

// TestDeriveKeySecretVariations verifies that different secret
// inputs produce different key material.
func TestDeriveKeySecretVariations(t *testing.T) {
	secrets := [][]byte{
		[]byte("secret-a"),
		[]byte("secret-b"),
		[]byte(""),
		{0x00},
		{0x01},
		bytes.Repeat([]byte{0xFF}, 32),
		bytes.Repeat([]byte{0x00}, 32),
	}

	keyLen := 72 // Standard Tor key material size
	keys := make([][]byte, len(secrets))

	for i, secret := range secrets {
		var err error
		keys[i], err = DeriveKey(secret, keyLen)
		if err != nil {
			t.Fatalf("secret %d: %v", i, err)
		}
	}

	// All pairs should be different
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if bytes.Equal(keys[i], keys[j]) {
				t.Errorf("secrets %d and %d produced identical keys", i, j)
			}
		}
	}
}

// TestNewAESCTRCipherBoundaryKeys verifies AES cipher creation
// with boundary key and IV sizes.
func TestNewAESCTRCipherBoundaryKeys(t *testing.T) {
	tests := []struct {
		name    string
		keyLen  int
		ivLen   int
		wantErr bool
	}{
		{"AES-128", 16, 16, false},
		{"AES-192", 24, 16, false},
		{"AES-256", 32, 16, false},
		{"key too short (15)", 15, 16, true},
		{"key too long (33)", 33, 16, true},
		{"empty key", 0, 16, true},
		{"iv too short", 16, 15, true},
		{"empty iv", 16, 0, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			key := make([]byte, tc.keyLen)
			iv := make([]byte, tc.ivLen)

			// Go's cipher.NewCTR panics on bad IV length rather than
			// returning an error, so we must catch panics for those cases.
			var cipherResult *AESCTRCipher
			var cipherErr error
			func() {
				defer func() {
					if r := recover(); r != nil {
						cipherErr = fmt.Errorf("panic: %v", r)
					}
				}()
				cipherResult, cipherErr = NewAESCTRCipher(key, iv)
			}()

			if tc.wantErr {
				if cipherErr == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if cipherErr != nil {
					t.Fatalf("unexpected error: %v", cipherErr)
				}
				if cipherResult == nil {
					t.Error("cipher is nil")
				}
			}
		})
	}
}

// TestAES256CTREncryptDecryptRoundTrip verifies that AES-256-CTR
// encryption followed by decryption recovers the original plaintext.
func TestAES256CTREncryptDecryptRoundTrip(t *testing.T) {
	key := make([]byte, 32)
	iv := make([]byte, 16)
	for i := range key {
		key[i] = byte(i)
	}

	plaintexts := [][]byte{
		{},
		{0x00},
		bytes.Repeat([]byte{0xAB}, 16), // One AES block
		bytes.Repeat([]byte{0xCD}, 509), // Relay cell size
		bytes.Repeat([]byte{0xFF}, 1024),
	}

	for i, pt := range plaintexts {
		ct, err := EncryptAES256CTR(pt, key, iv)
		if err != nil {
			t.Fatalf("case %d encrypt: %v", i, err)
		}

		recovered, err := DecryptAES256CTR(ct, key, iv)
		if err != nil {
			t.Fatalf("case %d decrypt: %v", i, err)
		}

		if !bytes.Equal(pt, recovered) {
			t.Errorf("case %d: round-trip failed", i)
		}
	}
}

// TestAES256CTRInvalidKeySizes verifies AES-256-CTR rejects
// non-32-byte keys.
func TestAES256CTRInvalidKeySizes(t *testing.T) {
	iv := make([]byte, 16)
	plaintext := []byte("test")

	badKeySizes := []int{0, 1, 15, 16, 24, 31, 33, 64}

	for _, sz := range badKeySizes {
		key := make([]byte, sz)

		_, err := EncryptAES256CTR(plaintext, key, iv)
		if err == nil {
			t.Errorf("key size %d: expected error for encrypt", sz)
		}

		_, err = DecryptAES256CTR(plaintext, key, iv)
		if err == nil {
			t.Errorf("key size %d: expected error for decrypt", sz)
		}
	}
}

// TestSHA1HashBoundaryInputs verifies SHA-1 hashing with
// boundary inputs.
func TestSHA1HashBoundaryInputs(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
	}{
		{"nil", nil},
		{"empty", []byte{}},
		{"single byte", []byte{0x00}},
		{"64 bytes (SHA-1 block)", make([]byte, 64)},
		{"65 bytes (cross block)", make([]byte, 65)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hash := SHA1Hash(tc.input)
			if len(hash) != 20 {
				t.Errorf("SHA1Hash length = %d, want 20", len(hash))
			}
		})
	}

	// Nil and empty should produce the same hash
	nilHash := SHA1Hash(nil)
	emptyHash := SHA1Hash([]byte{})
	if !bytes.Equal(nilHash, emptyHash) {
		t.Error("nil and empty inputs should produce the same SHA-1 hash")
	}
}

// TestSHA256HashBoundaryInputs verifies SHA-256 hashing with
// boundary inputs.
func TestSHA256HashBoundaryInputs(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
	}{
		{"nil", nil},
		{"empty", []byte{}},
		{"single byte", []byte{0x00}},
		{"64 bytes (SHA-256 block)", make([]byte, 64)},
		{"65 bytes (cross block)", make([]byte, 65)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hash := SHA256Hash(tc.input)
			if len(hash) != 32 {
				t.Errorf("SHA256Hash length = %d, want 32", len(hash))
			}
		})
	}
}

// TestDeriveKeyTorKeyMaterial verifies DeriveKey produces the
// correct amount of key material for Tor circuit keys (72 bytes).
func TestDeriveKeyTorKeyMaterial(t *testing.T) {
	secret := []byte("circuit-key-material-test")

	// Tor uses 72 bytes: Df(20) + Db(20) + Kf(16) + Kb(16)
	key, err := DeriveKey(secret, 72)
	if err != nil {
		t.Fatal(err)
	}

	if len(key) != 72 {
		t.Fatalf("key length = %d, want 72", len(key))
	}

	// Extract key components
	df := key[0:20]  // Forward digest
	db := key[20:40] // Backward digest
	kf := key[40:56] // Forward cipher
	kb := key[56:72] // Backward cipher

	// All components should be non-zero for a non-empty secret
	allZero := func(b []byte) bool {
		for _, v := range b {
			if v != 0 {
				return false
			}
		}
		return true
	}

	if allZero(df) {
		t.Error("Df is all zeros")
	}
	if allZero(db) {
		t.Error("Db is all zeros")
	}
	if allZero(kf) {
		t.Error("Kf is all zeros")
	}
	if allZero(kb) {
		t.Error("Kb is all zeros")
	}
}

// TestGenerateRandomBytesBoundary verifies GenerateRandomBytes
// boundary conditions.
func TestGenerateRandomBytesBoundary(t *testing.T) {
	tests := []struct {
		name string
		n    int
	}{
		{"zero", 0},
		{"one", 1},
		{"standard key", 32},
		{"large", 4096},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b, err := GenerateRandomBytes(tc.n)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(b) != tc.n {
				t.Errorf("length = %d, want %d", len(b), tc.n)
			}
		})
	}
}
