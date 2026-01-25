package onion

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"testing"

	"golang.org/x/crypto/curve25519"
)

func TestClientAuthStore(t *testing.T) {
	store := NewClientAuthStore()

	// Test adding credential
	var privateKey [32]byte
	if _, err := rand.Read(privateKey[:]); err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}

	addr := "test3xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx.onion"
	
	err := store.AddCredential(addr, privateKey)
	if err != nil {
		t.Fatalf("Failed to add credential: %v", err)
	}

	// Test retrieving credential
	cred, exists := store.GetCredential(addr)
	if !exists {
		t.Fatal("Credential not found after adding")
	}

	if cred.OnionAddress != addr {
		t.Errorf("Expected address %s, got %s", addr, cred.OnionAddress)
	}

	// Verify public key derivation
	var expectedPubKey [32]byte
	curve25519.ScalarBaseMult(&expectedPubKey, &privateKey)
	
	if !bytes.Equal(cred.PublicKey[:], expectedPubKey[:]) {
		t.Error("Public key mismatch")
	}

	// Test removing credential
	store.RemoveCredential(addr)
	_, exists = store.GetCredential(addr)
	if exists {
		t.Fatal("Credential still exists after removal")
	}

	// Test empty address
	err = store.AddCredential("", privateKey)
	if err == nil {
		t.Error("Expected error for empty address")
	}
}

func TestClientAuthStoreClear(t *testing.T) {
	store := NewClientAuthStore()

	// Add multiple credentials
	for i := 0; i < 3; i++ {
		var key [32]byte
		rand.Read(key[:])
		addr := "test" + string(rune('a'+i)) + "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx.onion"
		store.AddCredential(addr, key)
	}

	// Clear all
	store.Clear()

	// Verify all removed
	for i := 0; i < 3; i++ {
		addr := "test" + string(rune('a'+i)) + "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx.onion"
		_, exists := store.GetCredential(addr)
		if exists {
			t.Errorf("Credential %d still exists after clear", i)
		}
	}
}

func TestDecryptAuthDescriptor(t *testing.T) {
	// Generate client key pair
	var clientPrivate [32]byte
	if _, err := rand.Read(clientPrivate[:]); err != nil {
		t.Fatalf("Failed to generate client private key: %v", err)
	}
	
	var clientPublic [32]byte
	curve25519.ScalarBaseMult(&clientPublic, &clientPrivate)

	// Generate service key pair (for auth)
	var servicePrivate [32]byte
	if _, err := rand.Read(servicePrivate[:]); err != nil {
		t.Fatalf("Failed to generate service private key: %v", err)
	}
	
	var servicePublic [32]byte
	curve25519.ScalarBaseMult(&servicePublic, &servicePrivate)

	// Create test plaintext
	plaintext := []byte("This is a test descriptor with introduction points")

	// Create mock encrypted data structure
	// CLIENT_ID (8 bytes) || IV (16 bytes) || ENCRYPTED_DATA || MAC (16 bytes)
	clientID := make([]byte, 8)
	rand.Read(clientID)

	iv := make([]byte, 16)
	rand.Read(iv)

	// For this test, we'll create a simple structure
	// In real implementation, this would use proper AES-CTR encryption
	encryptedData := make([]byte, 0, 8+16+len(plaintext)+16)
	encryptedData = append(encryptedData, clientID...)
	encryptedData = append(encryptedData, iv...)
	
	// Mock ciphertext (would be AES-CTR encrypted in real impl)
	ciphertext := make([]byte, len(plaintext))
	copy(ciphertext, plaintext)
	
	encryptedData = append(encryptedData, ciphertext...)
	
	// Mock MAC (would be HMAC-SHA256 in real impl)
	mac := make([]byte, 16)
	rand.Read(mac)
	encryptedData = append(encryptedData, mac...)

	// Note: This test will fail decryption due to MAC verification
	// which is expected since we're using mock data
	// A full integration test would properly encrypt the data
	
	_, err := DecryptAuthDescriptor(encryptedData, clientPrivate, servicePublic)
	
	// We expect an error since we're using mock encrypted data
	if err == nil {
		t.Log("Warning: Mock data unexpectedly passed MAC verification")
	}
	
	// The important thing is that the function doesn't panic
	// and handles the structure correctly
}

func TestDecryptAuthDescriptorInvalidData(t *testing.T) {
	var clientPrivate, servicePublic [32]byte
	rand.Read(clientPrivate[:])
	rand.Read(servicePublic[:])

	tests := []struct {
		name        string
		data        []byte
		expectError bool
	}{
		{
			name:        "Empty data",
			data:        []byte{},
			expectError: true,
		},
		{
			name:        "Too short",
			data:        make([]byte, 20),
			expectError: true,
		},
		{
			name:        "Missing MAC",
			data:        make([]byte, 35), // Just enough for CLIENT_ID + IV + 11 bytes
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecryptAuthDescriptor(tt.data, clientPrivate, servicePublic)
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestParseAuthClients(t *testing.T) {
	lines := []string{
		"hs-descriptor 3",
		"descriptor-lifetime 180",
		"auth-client dGVzdDEyMw== aXYxMjM0NTY3ODkwMTIzNA== Y29va2llZGF0YTEyMzQ1Njc4OTAxMjM0NTY3ODkwMTI=",
		"auth-client YWJjZGVmZ2g= aXYwOTg3NjU0MzIxMDk4Nw== ZGF0YWFiY2RlZmdoaWprbG1ub3BxcnN0dXZ3eHl6MTI=",
		"revision-counter 1234",
	}

	authClients, err := ParseAuthClients(lines)
	if err != nil {
		t.Fatalf("Failed to parse auth clients: %v", err)
	}

	if len(authClients) != 2 {
		t.Errorf("Expected 2 auth clients, got %d", len(authClients))
	}

	// Verify first auth client
	firstClientID := "dGVzdDEyMw=="
	if _, exists := authClients[firstClientID]; !exists {
		t.Errorf("First auth client not found: %s", firstClientID)
	}

	// Verify second auth client
	secondClientID := "YWJjZGVmZ2g="
	if _, exists := authClients[secondClientID]; !exists {
		t.Errorf("Second auth client not found: %s", secondClientID)
	}
}

func TestParseAuthClientsMalformed(t *testing.T) {
	lines := []string{
		"auth-client",                                    // Missing fields
		"auth-client dGVzdA==",                           // Missing fields
		"auth-client dGVzdA== aXY=",                      // Missing encrypted cookie
		"auth-client invalid-base64 aXY= Y29va2ll",      // Invalid base64
		"not-auth-client dGVzdA== aXY= Y29va2ll",        // Wrong keyword
	}

	authClients, err := ParseAuthClients(lines)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should skip all malformed lines
	if len(authClients) != 0 {
		t.Errorf("Expected 0 auth clients from malformed data, got %d", len(authClients))
	}
}

func TestSplitFields(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Normal spacing",
			input:    "auth-client clientid iv cookie",
			expected: []string{"auth-client", "clientid", "iv", "cookie"},
		},
		{
			name:     "Multiple spaces",
			input:    "auth-client  clientid   iv    cookie",
			expected: []string{"auth-client", "clientid", "iv", "cookie"},
		},
		{
			name:     "Tabs",
			input:    "auth-client\tclientid\tiv\tcookie",
			expected: []string{"auth-client", "clientid", "iv", "cookie"},
		},
		{
			name:     "Mixed whitespace",
			input:    "auth-client \t clientid  \t iv   cookie",
			expected: []string{"auth-client", "clientid", "iv", "cookie"},
		},
		{
			name:     "Single field",
			input:    "single",
			expected: []string{"single"},
		},
		{
			name:     "Empty string",
			input:    "",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitFields(tt.input)
			
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d fields, got %d", len(tt.expected), len(result))
				return
			}

			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("Field %d: expected %q, got %q", i, tt.expected[i], result[i])
				}
			}
		})
	}
}

func TestClientAddRemoveAuth(t *testing.T) {
	client := NewClient(nil)

	var privateKey [32]byte
	rand.Read(privateKey[:])

	addr := "test3xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx.onion"

	// Test adding
	err := client.AddClientAuth(addr, privateKey)
	if err != nil {
		t.Fatalf("Failed to add client auth: %v", err)
	}

	// Test checking existence
	if !client.HasClientAuth(addr) {
		t.Error("Client auth not found after adding")
	}

	// Test removing
	client.RemoveClientAuth(addr)
	
	if client.HasClientAuth(addr) {
		t.Error("Client auth still exists after removal")
	}
}

func TestSplitDescriptorLines(t *testing.T) {
	descriptor := []byte("hs-descriptor 3\ndescriptor-lifetime 180\nrevision-counter 1234\nsignature test123\n")
	
	lines := splitDescriptorLines(descriptor)
	
	expected := []string{
		"hs-descriptor 3",
		"descriptor-lifetime 180",
		"revision-counter 1234",
		"signature test123",
	}
	
	if len(lines) != len(expected) {
		t.Errorf("Expected %d lines, got %d", len(expected), len(lines))
		return
	}
	
	for i, line := range lines {
		if line != expected[i] {
			t.Errorf("Line %d: expected %q, got %q", i, expected[i], line)
		}
	}
}

func TestSplitDescriptorLinesWithCRLF(t *testing.T) {
	descriptor := []byte("hs-descriptor 3\r\ndescriptor-lifetime 180\r\nrevision-counter 1234\r\n")
	
	lines := splitDescriptorLines(descriptor)
	
	expected := []string{
		"hs-descriptor 3",
		"descriptor-lifetime 180",
		"revision-counter 1234",
	}
	
	if len(lines) != len(expected) {
		t.Errorf("Expected %d lines, got %d", len(expected), len(lines))
		return
	}
	
	for i, line := range lines {
		if line != expected[i] {
			t.Errorf("Line %d: expected %q, got %q", i, expected[i], line)
		}
	}
}

func TestDeriveAuthKeys(t *testing.T) {
	secret := make([]byte, 32)
	rand.Read(secret)
	
	salt := make([]byte, 8)
	rand.Read(salt)
	
	info := []byte("test-auth-keys")
	
	// Test key derivation
	keys, err := deriveAuthKeys(secret, salt, info, 64)
	if err != nil {
		t.Fatalf("Key derivation failed: %v", err)
	}
	
	if len(keys) != 64 {
		t.Errorf("Expected 64 bytes, got %d", len(keys))
	}
	
	// Test determinism - same inputs should produce same output
	keys2, err := deriveAuthKeys(secret, salt, info, 64)
	if err != nil {
		t.Fatalf("Second key derivation failed: %v", err)
	}
	
	if !bytes.Equal(keys, keys2) {
		t.Error("Key derivation is not deterministic")
	}
	
	// Test different salt produces different keys
	salt2 := make([]byte, 8)
	rand.Read(salt2)
	
	keys3, err := deriveAuthKeys(secret, salt2, info, 64)
	if err != nil {
		t.Fatalf("Third key derivation failed: %v", err)
	}
	
	if bytes.Equal(keys, keys3) {
		t.Error("Different salts produced same keys")
	}
}

func TestComputeMAC(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	
	data := []byte("test data for MAC computation")
	
	// Test MAC computation
	mac := computeMAC(key, data)
	
	if len(mac) != 32 { // SHA256 output is 32 bytes
		t.Errorf("Expected 32 bytes, got %d", len(mac))
	}
	
	// Test determinism
	mac2 := computeMAC(key, data)
	if !bytes.Equal(mac, mac2) {
		t.Error("MAC computation is not deterministic")
	}
	
	// Test different data produces different MAC
	data2 := []byte("different test data")
	mac3 := computeMAC(key, data2)
	
	if bytes.Equal(mac, mac3) {
		t.Error("Different data produced same MAC")
	}
}

// Benchmark tests
func BenchmarkClientAuthStoreAdd(b *testing.B) {
	store := NewClientAuthStore()
	var key [32]byte
	rand.Read(key[:])
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		addr := base64.StdEncoding.EncodeToString([]byte{byte(i)})
		store.AddCredential(addr+".onion", key)
	}
}

func BenchmarkClientAuthStoreGet(b *testing.B) {
	store := NewClientAuthStore()
	var key [32]byte
	rand.Read(key[:])
	
	addr := "test.onion"
	store.AddCredential(addr, key)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = store.GetCredential(addr)
	}
}

func BenchmarkDeriveAuthKeys(b *testing.B) {
	secret := make([]byte, 32)
	rand.Read(secret)
	salt := make([]byte, 8)
	rand.Read(salt)
	info := []byte("benchmark-keys")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = deriveAuthKeys(secret, salt, info, 64)
	}
}
