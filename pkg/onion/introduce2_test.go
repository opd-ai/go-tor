// Package onion - INTRODUCE2 Cell Parsing Tests
package onion

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"io"
	"testing"

	"golang.org/x/crypto/hkdf"

	"github.com/opd-ai/go-tor/pkg/crypto"
)

// TestParseIntroduce2ValidCell tests parsing a valid INTRODUCE2 cell
func TestParseIntroduce2ValidCell(t *testing.T) {
	// Generate test keys
	introAuthKey := make([]byte, 32)
	introEncKey := make([]byte, 32)
	if _, err := rand.Read(introAuthKey); err != nil {
		t.Fatalf("Failed to generate auth key: %v", err)
	}
	if _, err := rand.Read(introEncKey); err != nil {
		t.Fatalf("Failed to generate enc key: %v", err)
	}

	// Create rendezvous cookie
	rendezvousCookie := make([]byte, 20)
	if _, err := rand.Read(rendezvousCookie); err != nil {
		t.Fatalf("Failed to generate cookie: %v", err)
	}

	// Create client onion key
	clientOnionKey := make([]byte, 32)
	if _, err := rand.Read(clientOnionKey); err != nil {
		t.Fatalf("Failed to generate onion key: %v", err)
	}

	// Create link specifiers (IPv4 example)
	linkSpecData := []byte{
		0x00,         // Type: TLS-over-TCP-IPv4
		0x06,         // Length: 6 bytes
		192, 0, 2, 1, // IP: 192.0.2.1
		0x1F, 0x90, // Port: 8080
	}

	// Build inner plaintext
	innerPlaintext := make([]byte, 0)
	innerPlaintext = append(innerPlaintext, rendezvousCookie...) // 20 bytes
	innerPlaintext = append(innerPlaintext, 0x01)                // NSPEC: 1 link specifier
	innerPlaintext = append(innerPlaintext, linkSpecData...)     // Link specifier
	innerPlaintext = append(innerPlaintext, 0x00)                // Onion key type: ntor
	innerPlaintext = append(innerPlaintext, 0x00, 0x20)          // Onion key len: 32
	innerPlaintext = append(innerPlaintext, clientOnionKey...)   // Client onion key
	innerPlaintext = append(innerPlaintext, 0x00)                // No extensions

	// Encrypt inner plaintext
	kdfInfo := []byte("tor-hs-ntor-curve25519-sha3-256-1:hs_key_extract")
	kdf := hkdf.New(sha256.New, introEncKey, nil, kdfInfo)
	keys := make([]byte, 64)
	if _, err := io.ReadFull(kdf, keys); err != nil {
		t.Fatalf("Key derivation failed: %v", err)
	}

	encKey := keys[0:32]
	macKey := keys[32:64]

	ciphertext, err := crypto.EncryptAES256CTR(innerPlaintext, encKey, make([]byte, 16))
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	// Compute MAC
	mac := hmac.New(sha256.New, macKey)
	mac.Write(ciphertext)
	macValue := mac.Sum(nil)

	// Build encrypted data
	encryptedData := append(ciphertext, macValue...)

	// Build outer layer
	clientAuthKey := make([]byte, 32)
	if _, err := rand.Read(clientAuthKey); err != nil {
		t.Fatalf("Failed to generate client auth key: %v", err)
	}

	outerCell := make([]byte, 0)
	outerCell = append(outerCell, 0x02)       // Auth key type: ED25519-SHA3-256
	outerCell = append(outerCell, 0x00, 0x20) // Auth key len: 32
	outerCell = append(outerCell, clientAuthKey...)
	outerCell = append(outerCell, 0x00) // No outer extensions
	outerCell = append(outerCell, encryptedData...)

	// Parse the cell
	request, err := ParseIntroduce2(outerCell, introAuthKey, introEncKey)
	if err != nil {
		t.Fatalf("ParseIntroduce2 failed: %v", err)
	}

	// Verify parsed data
	if !bytes.Equal(request.RendezvousCookie, rendezvousCookie) {
		t.Errorf("Rendezvous cookie mismatch")
	}

	if !bytes.Equal(request.ClientOnionKey, clientOnionKey) {
		t.Errorf("Client onion key mismatch")
	}

	if !bytes.Equal(request.ClientAuthKey, clientAuthKey) {
		t.Errorf("Client auth key mismatch")
	}

	if len(request.LinkSpecifiers) != 1 {
		t.Fatalf("Expected 1 link specifier, got %d", len(request.LinkSpecifiers))
	}

	if request.LinkSpecifiers[0].Type != 0x00 {
		t.Errorf("Expected link specifier type 0x00, got 0x%02x", request.LinkSpecifiers[0].Type)
	}
}

// TestParseIntroduce2TooShort tests handling of truncated cells
func TestParseIntroduce2TooShort(t *testing.T) {
	introAuthKey := make([]byte, 32)
	introEncKey := make([]byte, 32)

	shortCell := make([]byte, 50)

	_, err := ParseIntroduce2(shortCell, introAuthKey, introEncKey)
	if err == nil {
		t.Error("Expected error for short cell")
	}
}

// TestParseIntroduce2InvalidAuthKeyType tests unsupported auth key types
func TestParseIntroduce2InvalidAuthKeyType(t *testing.T) {
	introAuthKey := make([]byte, 32)
	introEncKey := make([]byte, 32)

	cell := make([]byte, 200)
	cell[0] = 0xFF // Invalid auth key type
	binary.BigEndian.PutUint16(cell[1:3], 32)

	_, err := ParseIntroduce2(cell, introAuthKey, introEncKey)
	if err == nil {
		t.Error("Expected error for invalid auth key type")
	}
}

// TestParseIntroduce2InvalidMAC tests MAC verification failure
func TestParseIntroduce2InvalidMAC(t *testing.T) {
	introAuthKey := make([]byte, 32)
	introEncKey := make([]byte, 32)
	if _, err := rand.Read(introAuthKey); err != nil {
		t.Fatalf("Failed to generate auth key: %v", err)
	}
	if _, err := rand.Read(introEncKey); err != nil {
		t.Fatalf("Failed to generate enc key: %v", err)
	}

	// Build minimal valid structure
	clientAuthKey := make([]byte, 32)
	if _, err := rand.Read(clientAuthKey); err != nil {
		t.Fatalf("Failed to generate client auth key: %v", err)
	}

	// Fake encrypted data with bad MAC
	fakeData := make([]byte, 100)
	if _, err := rand.Read(fakeData); err != nil {
		t.Fatalf("Failed to generate fake data: %v", err)
	}

	outerCell := make([]byte, 0)
	outerCell = append(outerCell, 0x02)       // Auth key type
	outerCell = append(outerCell, 0x00, 0x20) // Auth key len
	outerCell = append(outerCell, clientAuthKey...)
	outerCell = append(outerCell, 0x00)        // No extensions
	outerCell = append(outerCell, fakeData...) // Fake encrypted data

	_, err := ParseIntroduce2(outerCell, introAuthKey, introEncKey)
	if err == nil {
		t.Error("Expected error for invalid MAC")
	}
	if err != nil && err.Error() != "MAC verification failed" {
		t.Logf("Got error: %v (expected MAC verification failure)", err)
	}
}

// TestLinkSpecifierToAddress tests converting link specifiers to addresses
func TestLinkSpecifierToAddress(t *testing.T) {
	tests := []struct {
		name     string
		specs    []LinkSpecifier
		wantAddr string
		wantErr  bool
	}{
		{
			name: "IPv4",
			specs: []LinkSpecifier{
				{
					Type: 0x00,
					Data: []byte{192, 0, 2, 1, 0x1F, 0x90}, // 192.0.2.1:8080
				},
			},
			wantAddr: "192.0.2.1:8080",
			wantErr:  false,
		},
		{
			name: "IPv6",
			specs: []LinkSpecifier{
				{
					Type: 0x01,
					Data: []byte{
						0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
						0x1F, 0x90, // Port 8080
					},
				},
			},
			wantAddr: "[2001:0db8:0000:0000:0000:0000:0000:0001]:8080",
			wantErr:  false,
		},
		{
			name:     "No supported specifier",
			specs:    []LinkSpecifier{{Type: 0xFF, Data: []byte{1, 2, 3}}},
			wantAddr: "",
			wantErr:  true,
		},
		{
			name:     "Empty",
			specs:    []LinkSpecifier{},
			wantAddr: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr, err := LinkSpecifierToAddress(tt.specs)
			if (err != nil) != tt.wantErr {
				t.Errorf("LinkSpecifierToAddress() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if addr != tt.wantAddr {
				t.Errorf("LinkSpecifierToAddress() = %v, want %v", addr, tt.wantAddr)
			}
		})
	}
}

// TestParseIntroduce2WithExtensions tests handling of extension fields
func TestParseIntroduce2WithExtensions(t *testing.T) {
	// This test verifies that extensions are properly skipped/parsed
	// In the current implementation, we skip outer extensions
	// This is a placeholder for when extension support is needed

	t.Skip("Extension support not yet required - skipping")
}

// BenchmarkParseIntroduce2 benchmarks INTRODUCE2 parsing
func BenchmarkParseIntroduce2(b *testing.B) {
	// Generate test keys
	introAuthKey := make([]byte, 32)
	introEncKey := make([]byte, 32)
	rand.Read(introAuthKey)
	rand.Read(introEncKey)

	// Create a minimal valid cell (simplified for benchmarking)
	clientAuthKey := make([]byte, 32)
	rand.Read(clientAuthKey)

	// Build minimal inner plaintext
	innerPlaintext := make([]byte, 20+1+8+1+3+32+1) // cookie+nspec+linkspec+keytype+keylen+key+ext
	rand.Read(innerPlaintext[:20])                  // Cookie
	innerPlaintext[20] = 0x01                       // 1 link spec
	innerPlaintext[21] = 0x00                       // Type IPv4
	innerPlaintext[22] = 0x06                       // Len 6
	// Skip link spec data
	innerPlaintext[29] = 0x00 // Onion key type
	binary.BigEndian.PutUint16(innerPlaintext[30:32], 32)
	// Onion key at 32:64
	innerPlaintext[64] = 0x00 // No extensions

	// Encrypt
	kdfInfo := []byte("tor-hs-ntor-curve25519-sha3-256-1:hs_key_extract")
	kdf := hkdf.New(sha256.New, introEncKey, nil, kdfInfo)
	keys := make([]byte, 64)
	io.ReadFull(kdf, keys)

	encKey := keys[0:32]
	macKey := keys[32:64]

	ciphertext, _ := crypto.EncryptAES256CTR(innerPlaintext, encKey, make([]byte, 16))
	mac := hmac.New(sha256.New, macKey)
	mac.Write(ciphertext)
	macValue := mac.Sum(nil)

	encryptedData := append(ciphertext, macValue...)

	outerCell := make([]byte, 0)
	outerCell = append(outerCell, 0x02, 0x00, 0x20)
	outerCell = append(outerCell, clientAuthKey...)
	outerCell = append(outerCell, 0x00)
	outerCell = append(outerCell, encryptedData...)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ParseIntroduce2(outerCell, introAuthKey, introEncKey)
		if err != nil {
			b.Fatalf("Unexpected error: %v", err)
		}
	}
}
