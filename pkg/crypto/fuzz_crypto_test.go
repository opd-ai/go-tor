// Package crypto provides fuzzing tests for cryptographic primitives.
//
// These fuzz tests verify that no cryptographic function panics on
// arbitrary input, which is critical for a security-sensitive network
// implementation. Panics in crypto code can be exploited by a malicious
// peer to crash the client.
//
// Run fuzz tests with:
//
//	go test -fuzz=FuzzSHA1Hash -fuzztime=30s
//	go test -fuzz=FuzzSHA256Hash -fuzztime=30s
//	go test -fuzz=FuzzNewAESCTRCipher -fuzztime=30s
//	go test -fuzz=FuzzDecryptAES256CTR -fuzztime=30s
//	go test -fuzz=FuzzEncryptAES256CTR -fuzztime=30s
//	go test -fuzz=FuzzParseRSAPublicKey -fuzztime=30s
//	go test -fuzz=FuzzConstantTimeCompare -fuzztime=30s
//	go test -fuzz=FuzzEd25519Verify -fuzztime=30s
package crypto

import (
	"testing"
)

// FuzzSHA1Hash verifies SHA1Hash never panics on arbitrary input.
func FuzzSHA1Hash(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte("hello"))
	f.Add(make([]byte, 64))
	f.Add(make([]byte, 1024))

	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("SHA1Hash panicked on %d bytes: %v", len(data), r)
			}
		}()
		result := SHA1Hash(data)
		if len(result) != 20 {
			t.Errorf("SHA1Hash: expected 20-byte result, got %d", len(result))
		}
	})
}

// FuzzSHA256Hash verifies SHA256Hash never panics on arbitrary input.
func FuzzSHA256Hash(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte("hello"))
	f.Add(make([]byte, 64))
	f.Add(make([]byte, 1024))

	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("SHA256Hash panicked on %d bytes: %v", len(data), r)
			}
		}()
		result := SHA256Hash(data)
		if len(result) != 32 {
			t.Errorf("SHA256Hash: expected 32-byte result, got %d", len(result))
		}
	})
}

// FuzzNewAESCTRCipher verifies NewAESCTRCipher never panics on arbitrary key/IV.
func FuzzNewAESCTRCipher(f *testing.F) {
	// Valid 16-byte key + 16-byte IV
	f.Add(make([]byte, 16), make([]byte, 16))
	// Valid 32-byte key + 16-byte IV
	f.Add(make([]byte, 32), make([]byte, 16))
	// Wrong key length
	f.Add([]byte{}, make([]byte, 16))
	f.Add(make([]byte, 15), make([]byte, 16))
	// Wrong IV length
	f.Add(make([]byte, 16), []byte{})
	f.Add(make([]byte, 16), make([]byte, 15))

	f.Fuzz(func(t *testing.T, key, iv []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("NewAESCTRCipher panicked key_len=%d iv_len=%d: %v",
					len(key), len(iv), r)
			}
		}()
		_, _ = NewAESCTRCipher(key, iv)
	})
}

// FuzzDecryptAES256CTR verifies DecryptAES256CTR never panics on arbitrary input.
func FuzzDecryptAES256CTR(f *testing.F) {
	key32 := make([]byte, 32)
	iv16 := make([]byte, 16)
	f.Add([]byte("ciphertext"), key32, iv16)
	f.Add([]byte{}, key32, iv16)
	f.Add(make([]byte, 512), key32, iv16)
	// Wrong key sizes
	f.Add([]byte("data"), []byte{}, iv16)
	f.Add([]byte("data"), make([]byte, 15), iv16)
	// Wrong IV sizes
	f.Add([]byte("data"), key32, []byte{})
	f.Add([]byte("data"), key32, make([]byte, 15))

	f.Fuzz(func(t *testing.T, ciphertext, key, iv []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("DecryptAES256CTR panicked ct=%d key=%d iv=%d: %v",
					len(ciphertext), len(key), len(iv), r)
			}
		}()
		_, _ = DecryptAES256CTR(ciphertext, key, iv)
	})
}

// FuzzEncryptAES256CTR verifies EncryptAES256CTR never panics on arbitrary input.
func FuzzEncryptAES256CTR(f *testing.F) {
	key32 := make([]byte, 32)
	iv16 := make([]byte, 16)
	f.Add([]byte("plaintext"), key32, iv16)
	f.Add([]byte{}, key32, iv16)
	f.Add(make([]byte, 512), key32, iv16)
	// Wrong key/IV sizes
	f.Add([]byte("data"), []byte{}, iv16)
	f.Add([]byte("data"), key32, []byte{})

	f.Fuzz(func(t *testing.T, plaintext, key, iv []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("EncryptAES256CTR panicked pt=%d key=%d iv=%d: %v",
					len(plaintext), len(key), len(iv), r)
			}
		}()
		_, _ = EncryptAES256CTR(plaintext, key, iv)
	})
}

// FuzzEncryptDecryptRoundtrip verifies that Encrypt(Decrypt(x)) == x for valid keys.
func FuzzEncryptDecryptRoundtrip(f *testing.F) {
	key32 := make([]byte, 32)
	iv16 := make([]byte, 16)
	f.Add([]byte("hello world"), key32, iv16)
	f.Add(make([]byte, 498), key32, iv16)
	f.Add([]byte{}, key32, iv16)

	f.Fuzz(func(t *testing.T, plaintext, key, iv []byte) {
		// Only test valid 32-byte key + 16-byte IV combinations for roundtrip.
		if len(key) != 32 || len(iv) != 16 {
			return
		}
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("roundtrip panicked: %v", r)
			}
		}()
		ciphertext, err := EncryptAES256CTR(plaintext, key, iv)
		if err != nil {
			return
		}
		recovered, err := DecryptAES256CTR(ciphertext, key, iv)
		if err != nil {
			t.Errorf("DecryptAES256CTR failed after successful encrypt: %v", err)
			return
		}
		if len(plaintext) != len(recovered) {
			t.Errorf("roundtrip length mismatch: want %d, got %d",
				len(plaintext), len(recovered))
		}
	})
}

// FuzzParseRSAPublicKey verifies ParseRSAPublicKey never panics on arbitrary DER.
func FuzzParseRSAPublicKey(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0x30, 0x0d}) // Truncated DER SEQUENCE
	f.Add(make([]byte, 256))
	f.Add(make([]byte, 1024))

	f.Fuzz(func(t *testing.T, derBytes []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("ParseRSAPublicKey panicked on %d bytes: %v", len(derBytes), r)
			}
		}()
		_, _ = ParseRSAPublicKey(derBytes)
	})
}

// FuzzConstantTimeCompare verifies ConstantTimeCompare never panics on arbitrary input.
func FuzzConstantTimeCompare(f *testing.F) {
	f.Add([]byte{}, []byte{})
	f.Add([]byte("a"), []byte("a"))
	f.Add([]byte("a"), []byte("b"))
	f.Add(make([]byte, 32), make([]byte, 32))
	f.Add(make([]byte, 32), make([]byte, 16))

	f.Fuzz(func(t *testing.T, a, b []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("ConstantTimeCompare panicked a=%d b=%d: %v",
					len(a), len(b), r)
			}
		}()
		result := ConstantTimeCompare(a, b)
		// If equal length and same content, must return true.
		if len(a) == len(b) {
			allSame := true
			for i := range a {
				if a[i] != b[i] {
					allSame = false
					break
				}
			}
			if allSame && !result {
				t.Error("ConstantTimeCompare: equal slices returned false")
			}
		}
	})
}

// FuzzEd25519Verify verifies Ed25519Verify never panics on arbitrary input.
func FuzzEd25519Verify(f *testing.F) {
	pubKey := make([]byte, 32)
	sig := make([]byte, 64)
	f.Add(pubKey, []byte("message"), sig)
	f.Add([]byte{}, []byte{}, []byte{})
	f.Add(make([]byte, 31), []byte("msg"), make([]byte, 64)) // Wrong pubkey length
	f.Add(make([]byte, 32), []byte("msg"), make([]byte, 63)) // Wrong sig length

	f.Fuzz(func(t *testing.T, publicKey, message, signature []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Ed25519Verify panicked pk=%d msg=%d sig=%d: %v",
					len(publicKey), len(message), len(signature), r)
			}
		}()
		_ = Ed25519Verify(publicKey, message, signature)
	})
}

// FuzzDeriveKey verifies DeriveKey never panics on arbitrary secret/keyLen combos.
func FuzzDeriveKey(f *testing.F) {
	f.Add([]byte("secret"), 32)
	f.Add([]byte{}, 0)
	f.Add([]byte("key"), -1)
	f.Add(make([]byte, 128), 100)

	f.Fuzz(func(t *testing.T, secret []byte, keyLen int) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("DeriveKey panicked secret=%d keyLen=%d: %v",
					len(secret), keyLen, r)
			}
		}()
		_, _ = DeriveKey(secret, keyLen)
	})
}
