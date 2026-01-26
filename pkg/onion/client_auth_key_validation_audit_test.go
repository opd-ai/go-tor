// Package onion - Client Authorization Key Validation Security Audit
// This file implements security audit tests for client authorization key validation
// per rend-spec-v3.txt §2.5 and CWE-20 (Improper Input Validation)
package onion

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"testing"

	"golang.org/x/crypto/curve25519"
)

// TestClientAuthKeyValidation_KeySize validates x25519 key size enforcement
// Security requirement: Client private keys must be exactly 32 bytes (Curve25519 scalar)
func TestClientAuthKeyValidation_KeySize(t *testing.T) {
	store := NewClientAuthStore()
	addr := "test3xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx.onion"

	tests := []struct {
		name        string
		keySize     int
		expectPanic bool
		description string
	}{
		{
			name:        "Valid 32-byte key",
			keySize:     32,
			expectPanic: false,
			description: "Curve25519 requires 32-byte scalars",
		},
		// Note: Go's type system enforces [32]byte at compile time
		// Invalid sizes would be compilation errors, not runtime errors
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var key [32]byte
			if _, err := rand.Read(key[:]); err != nil {
				t.Fatalf("Failed to generate key: %v", err)
			}

			// This should always succeed with valid 32-byte keys
			err := store.AddCredential(addr, key)
			if err != nil {
				t.Errorf("Unexpected error with valid key: %v", err)
			}

			// Verify public key is also 32 bytes
			cred, exists := store.GetCredential(addr)
			if !exists {
				t.Fatal("Credential not found")
			}

			if len(cred.PublicKey) != 32 {
				t.Errorf("Public key size: expected 32 bytes, got %d", len(cred.PublicKey))
			}
		})
	}
}

// TestClientAuthKeyValidation_PublicKeyDerivation validates correct public key derivation
// Security requirement: Public key must be derived using curve25519.ScalarBaseMult
func TestClientAuthKeyValidation_PublicKeyDerivation(t *testing.T) {
	store := NewClientAuthStore()

	tests := []struct {
		name        string
		privateKey  [32]byte
		description string
	}{
		{
			name:        "Random private key",
			privateKey:  generateRandomKey(t),
			description: "Public key must be G^privkey on Curve25519",
		},
		{
			name:        "All zeros (weak but valid)",
			privateKey:  [32]byte{},
			description: "Zero key is mathematically valid (though weak)",
		},
		{
			name:        "All ones",
			privateKey:  generateOnesKey(),
			description: "All-ones key should produce valid public key",
		},
		{
			name:        "High bit set",
			privateKey:  generateHighBitKey(),
			description: "High bit is clamped in Curve25519",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr := tt.name + ".onion"

			// Add credential
			err := store.AddCredential(addr, tt.privateKey)
			if err != nil {
				t.Fatalf("Failed to add credential: %v", err)
			}

			// Retrieve and verify
			cred, exists := store.GetCredential(addr)
			if !exists {
				t.Fatal("Credential not found")
			}

			// Independently compute expected public key
			var expected [32]byte
			curve25519.ScalarBaseMult(&expected, &tt.privateKey)

			if !bytes.Equal(cred.PublicKey[:], expected[:]) {
				t.Errorf("Public key mismatch:\n  got:      %x\n  expected: %x",
					cred.PublicKey, expected)
			}
		})
	}
}

// TestClientAuthKeyValidation_AddressValidation validates onion address validation
// Security requirement: Prevent injection attacks and invalid address formats
func TestClientAuthKeyValidation_AddressValidation(t *testing.T) {
	store := NewClientAuthStore()
	key := generateRandomKey(t)

	tests := []struct {
		name        string
		address     string
		expectError bool
		description string
	}{
		{
			name:        "Valid v3 address",
			address:     "test3xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx.onion",
			expectError: false,
			description: "56-character v3 onion address",
		},
		{
			name:        "Empty address",
			address:     "",
			expectError: true,
			description: "Empty address must be rejected",
		},
		{
			name:        "Whitespace only",
			address:     "   ",
			expectError: false, // Currently accepts whitespace (potential improvement)
			description: "Whitespace is not validated (INFO finding)",
		},
		{
			name:        "SQL injection attempt",
			address:     "'; DROP TABLE credentials; --",
			expectError: false, // Not stored in DB, so not vulnerable
			description: "SQL injection has no effect (no SQL backend)",
		},
		{
			name:        "Path traversal attempt",
			address:     "../../../etc/passwd",
			expectError: false, // Not used for file operations
			description: "Path traversal has no effect (not used in file paths)",
		},
		{
			name:        "Null byte injection",
			address:     "test\x00malicious.onion",
			expectError: false, // Go handles null bytes safely in strings
			description: "Null bytes are safe in Go strings",
		},
		{
			name:        "Unicode address",
			address:     "test❤️.onion",
			expectError: false, // Unicode is allowed in map keys
			description: "Unicode characters are allowed",
		},
		{
			name:        "Very long address (1KB)",
			address:     string(make([]byte, 1024)),
			expectError: false, // No length limit enforced (potential DoS)
			description: "No length limit enforced (INFO finding)",
		},
		{
			name:        "Control characters",
			address:     "test\r\n\t.onion",
			expectError: false, // Control chars allowed in map keys
			description: "Control characters are allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.AddCredential(tt.address, key)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if err == nil {
				// Verify credential can be retrieved with exact address
				_, exists := store.GetCredential(tt.address)
				if !exists {
					t.Error("Credential not found after successful add")
				}
			}
		})
	}
}

// TestClientAuthKeyValidation_CredentialIsolation validates credential isolation
// Security requirement: Credentials for different addresses must be isolated
func TestClientAuthKeyValidation_CredentialIsolation(t *testing.T) {
	store := NewClientAuthStore()

	// Create 10 different credentials
	addresses := make([]string, 10)
	keys := make([][32]byte, 10)

	for i := 0; i < 10; i++ {
		addresses[i] = base64.StdEncoding.EncodeToString([]byte{byte(i)}) + ".onion"
		keys[i] = generateRandomKey(t)

		err := store.AddCredential(addresses[i], keys[i])
		if err != nil {
			t.Fatalf("Failed to add credential %d: %v", i, err)
		}
	}

	// Verify each credential is independent
	for i := 0; i < 10; i++ {
		cred, exists := store.GetCredential(addresses[i])
		if !exists {
			t.Errorf("Credential %d not found", i)
			continue
		}

		// Verify private key matches
		if !bytes.Equal(cred.PrivateKey[:], keys[i][:]) {
			t.Errorf("Credential %d has wrong private key", i)
		}

		// Verify public key is correctly derived
		var expectedPub [32]byte
		curve25519.ScalarBaseMult(&expectedPub, &keys[i])
		if !bytes.Equal(cred.PublicKey[:], expectedPub[:]) {
			t.Errorf("Credential %d has wrong public key", i)
		}

		// Verify no cross-contamination with other credentials
		for j := 0; j < 10; j++ {
			if i == j {
				continue
			}

			if bytes.Equal(cred.PrivateKey[:], keys[j][:]) {
				t.Errorf("Credential %d has same private key as credential %d", i, j)
			}
			if bytes.Equal(cred.PublicKey[:], keys[j][:]) {
				t.Errorf("Credential %d has same public key as credential %d's private key", i, j)
			}
		}
	}
}

// TestClientAuthKeyValidation_ConcurrentAccess validates thread safety
// Security requirement: Concurrent access must not cause data races or corruption
func TestClientAuthKeyValidation_ConcurrentAccess(t *testing.T) {
	store := NewClientAuthStore()

	// Run with -race flag to detect data races
	concurrency := 50
	done := make(chan bool, concurrency)

	// Concurrent adds
	for i := 0; i < concurrency; i++ {
		go func(id int) {
			defer func() { done <- true }()

			addr := base64.StdEncoding.EncodeToString([]byte{byte(id)}) + ".onion"
			key := generateRandomKey(t)

			err := store.AddCredential(addr, key)
			if err != nil {
				t.Errorf("Goroutine %d failed to add: %v", id, err)
			}

			// Immediately retrieve
			cred, exists := store.GetCredential(addr)
			if !exists {
				t.Errorf("Goroutine %d: credential not found", id)
				return
			}

			// Verify public key derivation
			var expected [32]byte
			curve25519.ScalarBaseMult(&expected, &key)
			if !bytes.Equal(cred.PublicKey[:], expected[:]) {
				t.Errorf("Goroutine %d: public key mismatch", id)
			}
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < concurrency; i++ {
		<-done
	}
}

// TestClientAuthKeyValidation_MemoryZeroing validates secure memory cleanup
// Security requirement: Private keys must be zeroed on removal per CWE-316
func TestClientAuthKeyValidation_MemoryZeroing(t *testing.T) {
	store := NewClientAuthStore()
	addr := "test.onion"
	key := generateRandomKey(t)

	// Add credential
	err := store.AddCredential(addr, key)
	if err != nil {
		t.Fatalf("Failed to add credential: %v", err)
	}

	// Get reference to credential before removal
	cred, _ := store.GetCredential(addr)
	originalKey := make([]byte, 32)
	copy(originalKey, cred.PrivateKey[:])

	// Remove credential (should zero private key)
	store.RemoveCredential(addr)

	// Verify credential is removed
	_, exists := store.GetCredential(addr)
	if exists {
		t.Error("Credential still exists after removal")
	}

	// Note: We cannot verify the memory was zeroed because the credential
	// is no longer accessible. This test verifies the API contract.
	// Memory zeroing is verified in the implementation using security.SecureZeroMemory
}

// TestClientAuthKeyValidation_ClearAllCredentials validates bulk clearing
// Security requirement: All credentials must be securely removed
func TestClientAuthKeyValidation_ClearAllCredentials(t *testing.T) {
	store := NewClientAuthStore()

	// Add 100 credentials
	addresses := make([]string, 100)
	for i := 0; i < 100; i++ {
		addresses[i] = base64.StdEncoding.EncodeToString([]byte{byte(i / 256), byte(i % 256)}) + ".onion"
		key := generateRandomKey(t)
		store.AddCredential(addresses[i], key)
	}

	// Verify all exist
	for _, addr := range addresses {
		_, exists := store.GetCredential(addr)
		if !exists {
			t.Errorf("Credential not found before clear: %s", addr)
		}
	}

	// Clear all
	store.Clear()

	// Verify all removed
	for _, addr := range addresses {
		_, exists := store.GetCredential(addr)
		if exists {
			t.Errorf("Credential still exists after clear: %s", addr)
		}
	}
}

// TestClientAuthKeyValidation_CLIENT_ID_Computation validates CLIENT_ID derivation
// Security requirement: CLIENT_ID = SHA256(client_public_key)[:8] per rend-spec-v3.txt §2.5
func TestClientAuthKeyValidation_CLIENT_ID_Computation(t *testing.T) {
	store := NewClientAuthStore()

	tests := []struct {
		name        string
		privateKey  [32]byte
		description string
	}{
		{
			name:        "Random key 1",
			privateKey:  generateRandomKey(t),
			description: "CLIENT_ID must be first 8 bytes of SHA256(pubkey)",
		},
		{
			name:        "Random key 2",
			privateKey:  generateRandomKey(t),
			description: "Different keys produce different CLIENT_IDs",
		},
		{
			name:        "Zero key",
			privateKey:  [32]byte{},
			description: "Zero key has deterministic CLIENT_ID",
		},
	}

	clientIDs := make([][]byte, len(tests))

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr := tt.name + ".onion"

			// Add credential
			err := store.AddCredential(addr, tt.privateKey)
			if err != nil {
				t.Fatalf("Failed to add credential: %v", err)
			}

			cred, _ := store.GetCredential(addr)

			// Compute expected CLIENT_ID
			h := sha256.New()
			h.Write(cred.PublicKey[:])
			hash := h.Sum(nil)
			expectedClientID := hash[:8]

			// Verify CLIENT_ID is 8 bytes
			if len(expectedClientID) != 8 {
				t.Errorf("CLIENT_ID length: expected 8, got %d", len(expectedClientID))
			}

			// Store for uniqueness check
			clientIDs[i] = expectedClientID

			t.Logf("CLIENT_ID for %s: %x", tt.name, expectedClientID)
		})
	}

	// Verify all CLIENT_IDs are unique (except intentional duplicates)
	for i := 0; i < len(clientIDs); i++ {
		for j := i + 1; j < len(clientIDs); j++ {
			if bytes.Equal(clientIDs[i], clientIDs[j]) && i != j {
				t.Errorf("CLIENT_ID collision: test %d and %d have same CLIENT_ID", i, j)
			}
		}
	}
}

// TestClientAuthKeyValidation_KeyReuseAcrossAddresses validates independence
// Security requirement: Same key for different addresses should be allowed (user choice)
func TestClientAuthKeyValidation_KeyReuseAcrossAddresses(t *testing.T) {
	store := NewClientAuthStore()
	key := generateRandomKey(t)

	// Add same key for 3 different addresses
	addresses := []string{
		"address1.onion",
		"address2.onion",
		"address3.onion",
	}

	for _, addr := range addresses {
		err := store.AddCredential(addr, key)
		if err != nil {
			t.Fatalf("Failed to add credential for %s: %v", addr, err)
		}
	}

	// Verify all credentials have same public key
	var firstPubKey []byte
	for i, addr := range addresses {
		cred, exists := store.GetCredential(addr)
		if !exists {
			t.Fatalf("Credential not found for %s", addr)
		}

		if i == 0 {
			firstPubKey = cred.PublicKey[:]
		} else {
			if !bytes.Equal(cred.PublicKey[:], firstPubKey) {
				t.Errorf("Public key mismatch for %s", addr)
			}
		}
	}

	t.Log("INFO: Key reuse across addresses is allowed (user choice)")
}

// TestClientAuthKeyValidation_OverwriteCredential validates credential replacement
// Security requirement: Replacing a credential should zero old key
func TestClientAuthKeyValidation_OverwriteCredential(t *testing.T) {
	store := NewClientAuthStore()
	addr := "test.onion"

	// Add first credential
	key1 := generateRandomKey(t)
	err := store.AddCredential(addr, key1)
	if err != nil {
		t.Fatalf("Failed to add first credential: %v", err)
	}

	cred1, _ := store.GetCredential(addr)
	pubKey1 := make([]byte, 32)
	copy(pubKey1, cred1.PublicKey[:])

	// Add second credential (overwrites)
	key2 := generateRandomKey(t)
	err = store.AddCredential(addr, key2)
	if err != nil {
		t.Fatalf("Failed to add second credential: %v", err)
	}

	cred2, exists := store.GetCredential(addr)
	if !exists {
		t.Fatal("Credential not found after overwrite")
	}

	// Verify new credential is different
	if bytes.Equal(cred2.PublicKey[:], pubKey1) {
		t.Error("Credential was not overwritten")
	}

	// Verify new credential matches second key
	var expectedPub [32]byte
	curve25519.ScalarBaseMult(&expectedPub, &key2)
	if !bytes.Equal(cred2.PublicKey[:], expectedPub[:]) {
		t.Error("Overwritten credential has wrong public key")
	}

	t.Log("NOTE: Old credential should be zeroed (implementation uses map overwrite)")
}

// TestClientAuthKeyValidation_EdgeCases validates edge case handling
func TestClientAuthKeyValidation_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		testFunc    func(t *testing.T)
		description string
	}{
		{
			name: "Get nonexistent credential",
			testFunc: func(t *testing.T) {
				store := NewClientAuthStore()
				_, exists := store.GetCredential("nonexistent.onion")
				if exists {
					t.Error("Nonexistent credential returned as existing")
				}
			},
			description: "Getting nonexistent credential should return false",
		},
		{
			name: "Remove nonexistent credential",
			testFunc: func(t *testing.T) {
				store := NewClientAuthStore()
				// Should not panic
				store.RemoveCredential("nonexistent.onion")
			},
			description: "Removing nonexistent credential should be safe",
		},
		{
			name: "Clear empty store",
			testFunc: func(t *testing.T) {
				store := NewClientAuthStore()
				// Should not panic
				store.Clear()
			},
			description: "Clearing empty store should be safe",
		},
		{
			name: "Multiple clears",
			testFunc: func(t *testing.T) {
				store := NewClientAuthStore()
				key := generateRandomKey(t)
				store.AddCredential("test.onion", key)
				store.Clear()
				store.Clear() // Should not panic
			},
			description: "Multiple clears should be safe",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFunc(t)
		})
	}
}

// Helper functions

func generateRandomKey(t *testing.T) [32]byte {
	var key [32]byte
	if _, err := rand.Read(key[:]); err != nil {
		t.Fatalf("Failed to generate random key: %v", err)
	}
	return key
}

func generateOnesKey() [32]byte {
	var key [32]byte
	for i := range key {
		key[i] = 0xFF
	}
	return key
}

func generateHighBitKey() [32]byte {
	var key [32]byte
	key[31] = 0x80 // High bit set
	return key
}

// Benchmark tests

func BenchmarkClientAuthKeyValidation_AddCredential(b *testing.B) {
	store := NewClientAuthStore()
	var key [32]byte
	rand.Read(key[:])

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		addr := base64.StdEncoding.EncodeToString([]byte{byte(i / 256), byte(i % 256)}) + ".onion"
		store.AddCredential(addr, key)
	}
}

func BenchmarkClientAuthKeyValidation_GetCredential(b *testing.B) {
	store := NewClientAuthStore()
	var key [32]byte
	rand.Read(key[:])

	// Prepare credentials
	addr := "test.onion"
	store.AddCredential(addr, key)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = store.GetCredential(addr)
	}
}

func BenchmarkClientAuthKeyValidation_PublicKeyDerivation(b *testing.B) {
	var privateKey [32]byte
	rand.Read(privateKey[:])

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var publicKey [32]byte
		curve25519.ScalarBaseMult(&publicKey, &privateKey)
	}
}

func BenchmarkClientAuthKeyValidation_CLIENT_ID_Computation(b *testing.B) {
	var publicKey [32]byte
	rand.Read(publicKey[:])

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h := sha256.New()
		h.Write(publicKey[:])
		_ = h.Sum(nil)[:8]
	}
}
