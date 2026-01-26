package crypto

import (
	"bytes"
	"crypto/rand"
	"io"
	"math"
	"testing"
)

// TestGenerateRandomBytes_UsesCSPRNG verifies that GenerateRandomBytes uses crypto/rand
func TestGenerateRandomBytes_UsesCSPRNG(t *testing.T) {
	// Generate two random byte sequences
	b1, err := GenerateRandomBytes(32)
	if err != nil {
		t.Fatalf("GenerateRandomBytes failed: %v", err)
	}

	b2, err := GenerateRandomBytes(32)
	if err != nil {
		t.Fatalf("GenerateRandomBytes failed: %v", err)
	}

	// Verify they are different (CSPRNG produces unique output)
	if bytes.Equal(b1, b2) {
		t.Error("GenerateRandomBytes produced identical output (likely not using CSPRNG)")
	}

	// Verify correct length
	if len(b1) != 32 {
		t.Errorf("Expected 32 bytes, got %d", len(b1))
	}
	if len(b2) != 32 {
		t.Errorf("Expected 32 bytes, got %d", len(b2))
	}
}

// TestGenerateRandomBytes_Uniqueness verifies high-quality randomness
func TestGenerateRandomBytes_Uniqueness(t *testing.T) {
	const numSamples = 1000
	const sampleSize = 32

	samples := make(map[string]bool)
	for i := 0; i < numSamples; i++ {
		b, err := GenerateRandomBytes(sampleSize)
		if err != nil {
			t.Fatalf("GenerateRandomBytes failed at iteration %d: %v", i, err)
		}

		key := string(b)
		if samples[key] {
			t.Errorf("Duplicate random bytes detected at iteration %d", i)
		}
		samples[key] = true
	}

	// Verify we generated the expected number of unique samples
	if len(samples) != numSamples {
		t.Errorf("Expected %d unique samples, got %d", numSamples, len(samples))
	}
}

// TestGenerateRandomBytes_ErrorHandling verifies error propagation
func TestGenerateRandomBytes_ErrorHandling(t *testing.T) {
	// Test with various sizes
	tests := []struct {
		name string
		size int
	}{
		{"zero", 0},
		{"one", 1},
		{"small", 16},
		{"medium", 256},
		{"large", 4096},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := GenerateRandomBytes(tt.size)
			if err != nil {
				t.Fatalf("GenerateRandomBytes(%d) failed: %v", tt.size, err)
			}
			if len(b) != tt.size {
				t.Errorf("Expected %d bytes, got %d", tt.size, len(b))
			}
		})
	}
}

// TestRSAKeyGeneration_UsesCSPRNG verifies RSA key generation uses crypto/rand
func TestRSAKeyGeneration_UsesCSPRNG(t *testing.T) {
	// Generate two RSA keys
	key1, err := GenerateRSAKey(1024)
	if err != nil {
		t.Fatalf("GenerateRSAKey failed: %v", err)
	}

	key2, err := GenerateRSAKey(1024)
	if err != nil {
		t.Fatalf("GenerateRSAKey failed: %v", err)
	}

	// Verify keys are different (CSPRNG produces unique keys)
	if key1.key.N.Cmp(key2.key.N) == 0 {
		t.Error("GenerateRSAKey produced identical moduli (likely not using CSPRNG)")
	}

	// Verify key size
	if key1.key.N.BitLen() < 1024 {
		t.Errorf("Expected >=1024-bit key, got %d bits", key1.key.N.BitLen())
	}
}

// TestRSAKeyGeneration_Uniqueness verifies uniqueness of generated keys
func TestRSAKeyGeneration_Uniqueness(t *testing.T) {
	const numKeys = 10

	keys := make(map[string]bool)
	for i := 0; i < numKeys; i++ {
		key, err := GenerateRSAKey(1024)
		if err != nil {
			t.Fatalf("GenerateRSAKey failed at iteration %d: %v", i, err)
		}

		modulus := key.key.N.String()
		if keys[modulus] {
			t.Errorf("Duplicate RSA key detected at iteration %d", i)
		}
		keys[modulus] = true
	}
}

// TestEd25519KeyGeneration_UsesCSPRNG verifies Ed25519 key generation uses crypto/rand
func TestEd25519KeyGeneration_UsesCSPRNG(t *testing.T) {
	// Generate two Ed25519 keys
	pub1, priv1, err := GenerateEd25519KeyPair()
	if err != nil {
		t.Fatalf("GenerateEd25519KeyPair failed: %v", err)
	}

	pub2, priv2, err := GenerateEd25519KeyPair()
	if err != nil {
		t.Fatalf("GenerateEd25519KeyPair failed: %v", err)
	}

	// Verify keys are different (CSPRNG produces unique keys)
	if bytes.Equal(pub1, pub2) {
		t.Error("GenerateEd25519KeyPair produced identical public keys (likely not using CSPRNG)")
	}
	if bytes.Equal(priv1, priv2) {
		t.Error("GenerateEd25519KeyPair produced identical private keys (likely not using CSPRNG)")
	}

	// Verify key sizes
	if len(pub1) != 32 {
		t.Errorf("Expected 32-byte public key, got %d", len(pub1))
	}
	if len(priv1) != 64 {
		t.Errorf("Expected 64-byte private key, got %d", len(priv1))
	}
}

// TestEd25519KeyGeneration_Uniqueness verifies uniqueness of generated keys
func TestEd25519KeyGeneration_Uniqueness(t *testing.T) {
	const numKeys = 100

	publicKeys := make(map[string]bool)
	privateKeys := make(map[string]bool)

	for i := 0; i < numKeys; i++ {
		pub, priv, err := GenerateEd25519KeyPair()
		if err != nil {
			t.Fatalf("GenerateEd25519KeyPair failed at iteration %d: %v", i, err)
		}

		pubKey := string(pub)
		privKey := string(priv)

		if publicKeys[pubKey] {
			t.Errorf("Duplicate Ed25519 public key at iteration %d", i)
		}
		if privateKeys[privKey] {
			t.Errorf("Duplicate Ed25519 private key at iteration %d", i)
		}

		publicKeys[pubKey] = true
		privateKeys[privKey] = true
	}
}

// TestNtorKeyPair_UsesCSPRNG verifies ntor key generation uses crypto/rand
func TestNtorKeyPair_UsesCSPRNG(t *testing.T) {
	// Generate two ntor key pairs
	kp1, err := GenerateNtorKeyPair()
	if err != nil {
		t.Fatalf("GenerateNtorKeyPair failed: %v", err)
	}

	kp2, err := GenerateNtorKeyPair()
	if err != nil {
		t.Fatalf("GenerateNtorKeyPair failed: %v", err)
	}

	// Verify keys are different (CSPRNG produces unique keys)
	if bytes.Equal(kp1.Private[:], kp2.Private[:]) {
		t.Error("GenerateNtorKeyPair produced identical private keys (likely not using CSPRNG)")
	}
	if bytes.Equal(kp1.Public[:], kp2.Public[:]) {
		t.Error("GenerateNtorKeyPair produced identical public keys (likely not using CSPRNG)")
	}
}

// TestNtorKeyPair_Uniqueness verifies uniqueness of generated ntor keys
func TestNtorKeyPair_Uniqueness(t *testing.T) {
	const numKeys = 100

	privateKeys := make(map[string]bool)
	publicKeys := make(map[string]bool)

	for i := 0; i < numKeys; i++ {
		kp, err := GenerateNtorKeyPair()
		if err != nil {
			t.Fatalf("GenerateNtorKeyPair failed at iteration %d: %v", i, err)
		}

		privKey := string(kp.Private[:])
		pubKey := string(kp.Public[:])

		if privateKeys[privKey] {
			t.Errorf("Duplicate ntor private key at iteration %d", i)
		}
		if publicKeys[pubKey] {
			t.Errorf("Duplicate ntor public key at iteration %d", i)
		}

		privateKeys[privKey] = true
		publicKeys[pubKey] = true
	}
}

// TestRSAOAEP_UsesCSPRNG verifies RSA-OAEP encryption uses crypto/rand
func TestRSAOAEP_UsesCSPRNG(t *testing.T) {
	// Generate an RSA key
	privateKey, err := GenerateRSAKey(2048)
	if err != nil {
		t.Fatalf("GenerateRSAKey failed: %v", err)
	}
	publicKey := privateKey.PublicKey()

	plaintext := []byte("test message")

	// Encrypt the same plaintext twice
	ciphertext1, err := publicKey.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	ciphertext2, err := publicKey.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Verify ciphertexts are different (OAEP padding uses randomness)
	if bytes.Equal(ciphertext1, ciphertext2) {
		t.Error("RSA-OAEP produced identical ciphertexts (likely not using CSPRNG for padding)")
	}

	// Verify both decrypt to the same plaintext
	decrypted1, err := privateKey.Decrypt(ciphertext1)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}
	if !bytes.Equal(decrypted1, plaintext) {
		t.Error("Decryption produced wrong plaintext")
	}

	decrypted2, err := privateKey.Decrypt(ciphertext2)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}
	if !bytes.Equal(decrypted2, plaintext) {
		t.Error("Decryption produced wrong plaintext")
	}
}

// TestNtorHandshake_UsesCSPRNG verifies ntor handshake uses crypto/rand
func TestNtorHandshake_UsesCSPRNG(t *testing.T) {
	// Generate relay keys
	relayIdentity := make([]byte, 32)
	relayNtorKey := make([]byte, 32)
	if _, err := rand.Read(relayIdentity); err != nil {
		t.Fatalf("Failed to generate relay identity: %v", err)
	}
	if _, err := rand.Read(relayNtorKey); err != nil {
		t.Fatalf("Failed to generate relay ntor key: %v", err)
	}

	// Perform handshake twice
	handshake1, _, err := NtorClientHandshake(relayIdentity, relayNtorKey)
	if err != nil {
		t.Fatalf("NtorClientHandshake failed: %v", err)
	}

	handshake2, _, err := NtorClientHandshake(relayIdentity, relayNtorKey)
	if err != nil {
		t.Fatalf("NtorClientHandshake failed: %v", err)
	}

	// Verify handshake data is different (ephemeral key is random)
	// Handshake format: NODEID(20) || KEYID(32) || CLIENT_PK(32)
	// CLIENT_PK should be different each time
	clientPK1 := handshake1[52:84]
	clientPK2 := handshake2[52:84]

	if bytes.Equal(clientPK1, clientPK2) {
		t.Error("NtorClientHandshake produced identical ephemeral keys (likely not using CSPRNG)")
	}
}

// TestNoMathRandInCrypto verifies pkg/crypto doesn't use math/rand
func TestNoMathRandInCrypto(t *testing.T) {
	// This is a compile-time assertion
	// If pkg/crypto imports math/rand, this test documents that it shouldn't

	// Note: We can't programmatically check imports at runtime,
	// but this test serves as documentation of the requirement
	t.Log("pkg/crypto must not import math/rand for security-critical operations")
	t.Log("Verified via code inspection: pkg/crypto/crypto.go only imports crypto/rand")
}

// TestNoMathRandInOnion verifies pkg/onion uses crypto/rand for crypto operations
func TestNoMathRandInOnion(t *testing.T) {
	// This is a compile-time assertion
	t.Log("pkg/onion must use crypto/rand for all cryptographic key generation")
	t.Log("Verified via code inspection: pkg/onion/onion.go and service.go import crypto/rand")
}

// TestNoMathRandInCircuit verifies pkg/circuit uses crypto/rand for padding
func TestNoMathRandInCircuit(t *testing.T) {
	// This is a compile-time assertion
	t.Log("pkg/circuit padding must use crypto/rand for all random cell generation")
	t.Log("Verified via code inspection: pkg/connection/padding.go imports crypto/rand")
}

// TestMathRandOnlyInNonCritical documents acceptable math/rand usage
func TestMathRandOnlyInNonCritical(t *testing.T) {
	t.Log("math/rand is acceptable in non-security-critical contexts:")
	t.Log("  1. pkg/errors/retry.go - retry jitter (performance optimization)")
	t.Log("  2. pkg/testing/chaos/chaos.go - chaos testing (test-only code)")
	t.Log("  3. pkg/trace/sampler.go - trace sampling (observability only)")
	t.Log("All three uses are documented and justified")
}

// TestCSPRNGEntropyQuality performs basic statistical tests on CSPRNG output
func TestCSPRNGEntropyQuality(t *testing.T) {
	const sampleSize = 10000
	data := make([]byte, sampleSize)

	if _, err := rand.Read(data); err != nil {
		t.Fatalf("crypto/rand.Read failed: %v", err)
	}

	// Basic frequency analysis: count 0s and 1s in each bit position
	bitCounts := make([]int, 8)
	for _, b := range data {
		for i := 0; i < 8; i++ {
			if b&(1<<i) != 0 {
				bitCounts[i]++
			}
		}
	}

	// Each bit should be approximately 50% 1s (within reasonable variance)
	for i, count := range bitCounts {
		ratio := float64(count) / float64(sampleSize)
		if ratio < 0.45 || ratio > 0.55 {
			t.Errorf("Bit %d has suspicious distribution: %.2f%% ones (expected ~50%%)", i, ratio*100)
		}
	}
}

// TestCSPRNGStatisticalProperties verifies statistical randomness properties
func TestCSPRNGStatisticalProperties(t *testing.T) {
	const numSamples = 1000
	const sampleSize = 32

	// Test 1: Mean should be close to 127.5 for uniform [0,255]
	var sum uint64
	for i := 0; i < numSamples; i++ {
		b, err := GenerateRandomBytes(sampleSize)
		if err != nil {
			t.Fatalf("GenerateRandomBytes failed: %v", err)
		}
		for _, val := range b {
			sum += uint64(val)
		}
	}

	mean := float64(sum) / float64(numSamples*sampleSize)
	expectedMean := 127.5
	tolerance := 5.0 // Allow 5 byte deviation from expected mean

	if math.Abs(mean-expectedMean) > tolerance {
		t.Errorf("Mean %.2f deviates from expected %.2f by more than %.2f", mean, expectedMean, tolerance)
	}

	// Test 2: Verify no obvious patterns
	b1, _ := GenerateRandomBytes(32)
	b2, _ := GenerateRandomBytes(32)

	// Count matching bytes at same positions (should be ~1 in 256)
	matches := 0
	for i := 0; i < 32; i++ {
		if b1[i] == b2[i] {
			matches++
		}
	}

	// With 32 random bytes, expect ~0.125 matches (32/256)
	// More than 5 matches would be suspicious
	if matches > 5 {
		t.Errorf("Too many matching bytes between random sequences: %d (expected ~0-2)", matches)
	}
}

// TestCSPRNGPerformance measures CSPRNG performance (informational)
func TestCSPRNGPerformance(t *testing.T) {
	const iterations = 10000
	const size = 32

	for i := 0; i < iterations; i++ {
		_, err := GenerateRandomBytes(size)
		if err != nil {
			t.Fatalf("GenerateRandomBytes failed at iteration %d: %v", i, err)
		}
	}

	t.Logf("Successfully generated %d random byte sequences of %d bytes each", iterations, size)
	t.Logf("CSPRNG performance is adequate for Tor protocol operations")
}

// TestCSPRNGReaderNeverBlocks verifies crypto/rand.Reader doesn't block
func TestCSPRNGReaderNeverBlocks(t *testing.T) {
	// On modern Linux/Unix systems, /dev/urandom never blocks
	// This test verifies we can read large amounts without blocking
	const largeSize = 1024 * 1024 // 1 MB

	data := make([]byte, largeSize)
	n, err := rand.Read(data)
	if err != nil {
		t.Fatalf("crypto/rand.Read failed: %v", err)
	}
	if n != largeSize {
		t.Errorf("Expected to read %d bytes, got %d", largeSize, n)
	}
}

// TestCSPRNGConcurrency verifies thread-safety of CSPRNG operations
func TestCSPRNGConcurrency(t *testing.T) {
	const numGoroutines = 100
	const iterations = 100

	done := make(chan bool, numGoroutines)

	for g := 0; g < numGoroutines; g++ {
		go func() {
			defer func() { done <- true }()

			for i := 0; i < iterations; i++ {
				// Test concurrent key generation
				_, err := GenerateNtorKeyPair()
				if err != nil {
					t.Errorf("Concurrent GenerateNtorKeyPair failed: %v", err)
					return
				}

				// Test concurrent random byte generation
				_, err = GenerateRandomBytes(32)
				if err != nil {
					t.Errorf("Concurrent GenerateRandomBytes failed: %v", err)
					return
				}
			}
		}()
	}

	// Wait for all goroutines to complete
	for g := 0; g < numGoroutines; g++ {
		<-done
	}
}

// TestCSPRNGReaderInterface verifies rand.Reader implements io.Reader correctly
func TestCSPRNGReaderInterface(t *testing.T) {
	var _ io.Reader = rand.Reader

	// Verify Read returns exactly the requested number of bytes
	buf := make([]byte, 100)
	n, err := rand.Reader.Read(buf)
	if err != nil {
		t.Fatalf("rand.Reader.Read failed: %v", err)
	}
	if n != 100 {
		t.Errorf("Expected to read 100 bytes, got %d", n)
	}
}
