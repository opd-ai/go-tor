package onion

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"testing"
	"time"

	"golang.org/x/crypto/chacha20poly1305"
)

// TestDecryptDescriptor tests the descriptor decryption functionality
func TestDecryptDescriptor(t *testing.T) {
	// Generate a test onion address
	pubkey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	addr := &Address{
		Version: V3,
		Pubkey:  pubkey,
	}

	timePeriod := GetTimePeriod(time.Now())

	tests := []struct {
		name        string
		setupDesc   func() *Descriptor
		wantErr     bool
		errContains string
	}{
		{
			name: "descriptor without superencrypted section",
			setupDesc: func() *Descriptor {
				return &Descriptor{
					Version:       3,
					RawDescriptor: []byte("hs-descriptor 3\nrevision-counter 1\n"),
					IntroPoints:   make([]IntroductionPoint, 0),
				}
			},
			wantErr: false,
		},
		{
			name: "descriptor with valid encrypted section",
			setupDesc: func() *Descriptor {
				// Create a simple encrypted descriptor
				plaintext := []byte("introduction-point\nauth-key\nABCDEFGH==\n")

				// Derive encryption keys
				blindedPubkey := ComputeBlindedPubkey(pubkey, timePeriod)
				salt := make([]byte, 16)
				rand.Read(salt)

				keys, _ := deriveDescriptorKeys(blindedPubkey, salt, "hsdir-superencrypted-data")
				nonce, _ := deriveDescriptorKeys(blindedPubkey, salt, "hsdir-superencrypted-nonce")

				// Encrypt with XChaCha20-Poly1305
				aead, _ := chacha20poly1305.NewX(keys[:32])
				ciphertext := aead.Seal(nil, nonce[:chacha20poly1305.NonceSizeX], plaintext, nil)

				// Build encrypted data: SALT || CIPHERTEXT
				encryptedData := append(salt, ciphertext...)
				encryptedB64 := base64.StdEncoding.EncodeToString(encryptedData)

				raw := "hs-descriptor 3\nrevision-counter 1\nsuperencrypted\n-----BEGIN MESSAGE-----\n" +
					encryptedB64 + "\n-----END MESSAGE-----\n"

				return &Descriptor{
					Version:       3,
					RawDescriptor: []byte(raw),
					IntroPoints:   make([]IntroductionPoint, 0),
				}
			},
			wantErr: false,
		},
		{
			name: "nil descriptor",
			setupDesc: func() *Descriptor {
				return nil
			},
			wantErr:     true,
			errContains: "descriptor is nil",
		},
		{
			name: "descriptor with malformed encrypted section",
			setupDesc: func() *Descriptor {
				raw := "hs-descriptor 3\nsuperencrypted\n-----BEGIN MESSAGE-----\ninvalid-base64!!!\n-----END MESSAGE-----\n"
				return &Descriptor{
					Version:       3,
					RawDescriptor: []byte(raw),
				}
			},
			wantErr:     true,
			errContains: "failed to decode encrypted data",
		},
		{
			name: "descriptor with too short encrypted data",
			setupDesc: func() *Descriptor {
				shortData := make([]byte, 10)
				encryptedB64 := base64.StdEncoding.EncodeToString(shortData)
				raw := "hs-descriptor 3\nsuperencrypted\n-----BEGIN MESSAGE-----\n" +
					encryptedB64 + "\n-----END MESSAGE-----\n"
				return &Descriptor{
					Version:       3,
					RawDescriptor: []byte(raw),
				}
			},
			wantErr:     true,
			errContains: "encrypted data too short",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			desc := tt.setupDesc()
			result, err := DecryptDescriptor(desc, addr, timePeriod)

			if tt.wantErr {
				if err == nil {
					t.Errorf("DecryptDescriptor() expected error, got nil")
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("DecryptDescriptor() error = %v, want error containing %q", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("DecryptDescriptor() unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Error("DecryptDescriptor() returned nil result without error")
			}
		})
	}
}

// TestDecryptDescriptor_NilAddress tests decryption with nil address
func TestDecryptDescriptor_NilAddress(t *testing.T) {
	desc := &Descriptor{
		Version:       3,
		RawDescriptor: []byte("hs-descriptor 3\n"),
	}

	_, err := DecryptDescriptor(desc, nil, 0)
	if err == nil {
		t.Error("DecryptDescriptor() with nil address should return error")
	}
	if !contains(err.Error(), "address is nil") {
		t.Errorf("DecryptDescriptor() error = %v, want error containing 'address is nil'", err)
	}
}

// TestDecryptDescriptor_InvalidPublicKey tests decryption with invalid public key length
func TestDecryptDescriptor_InvalidPublicKey(t *testing.T) {
	desc := &Descriptor{
		Version:       3,
		RawDescriptor: []byte("hs-descriptor 3\n"),
	}
	addr := &Address{
		Version: V3,
		Pubkey:  []byte("short"), // Invalid length
	}

	_, err := DecryptDescriptor(desc, addr, 0)
	if err == nil {
		t.Error("DecryptDescriptor() with invalid pubkey should return error")
	}
	if !contains(err.Error(), "invalid public key length") {
		t.Errorf("DecryptDescriptor() error = %v, want error containing 'invalid public key length'", err)
	}
}

// TestDeriveDescriptorKeys tests the key derivation function
func TestDeriveDescriptorKeys(t *testing.T) {
	secret := make([]byte, 32)
	salt := make([]byte, 16)
	rand.Read(secret)
	rand.Read(salt)

	key1, err := deriveDescriptorKeys(secret, salt, "test-info-1")
	if err != nil {
		t.Fatalf("deriveDescriptorKeys() error = %v", err)
	}
	if len(key1) != 32 {
		t.Errorf("deriveDescriptorKeys() returned %d bytes, want 32", len(key1))
	}

	// Same inputs should produce same key
	key2, err := deriveDescriptorKeys(secret, salt, "test-info-1")
	if err != nil {
		t.Fatalf("deriveDescriptorKeys() error = %v", err)
	}
	if string(key1) != string(key2) {
		t.Error("deriveDescriptorKeys() produced different keys for same inputs")
	}

	// Different info should produce different key
	key3, err := deriveDescriptorKeys(secret, salt, "test-info-2")
	if err != nil {
		t.Fatalf("deriveDescriptorKeys() error = %v", err)
	}
	if string(key1) == string(key3) {
		t.Error("deriveDescriptorKeys() produced same key for different info strings")
	}
}

// TestParseDecryptedLayer tests parsing of decrypted descriptor layer
func TestParseDecryptedLayer(t *testing.T) {
	tests := []struct {
		name            string
		data            string
		wantIntroPoints int
	}{
		{
			name:            "empty data",
			data:            "",
			wantIntroPoints: 0,
		},
		{
			name:            "single introduction point",
			data:            "introduction-point\nauth-key\n" + base64.StdEncoding.EncodeToString(make([]byte, 32)) + "\n",
			wantIntroPoints: 1,
		},
		{
			name: "multiple introduction points",
			data: "introduction-point\nauth-key\n" + base64.StdEncoding.EncodeToString(make([]byte, 32)) + "\n" +
				"introduction-point\nauth-key\n" + base64.StdEncoding.EncodeToString(make([]byte, 32)) + "\n",
			wantIntroPoints: 2,
		},
		{
			name:            "introduction point with onion key",
			data:            "introduction-point\nonion-key\nntor " + base64.StdEncoding.EncodeToString(make([]byte, 32)) + "\n",
			wantIntroPoints: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseDecryptedLayer([]byte(tt.data))
			if err != nil {
				t.Errorf("parseDecryptedLayer() error = %v", err)
				return
			}

			if len(result.IntroPoints) != tt.wantIntroPoints {
				t.Errorf("parseDecryptedLayer() got %d intro points, want %d",
					len(result.IntroPoints), tt.wantIntroPoints)
			}
		})
	}
}

// TestParseLinkSpecifiers tests link specifier parsing
func TestParseLinkSpecifiers(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		wantCount int
	}{
		{
			name:      "empty data",
			data:      []byte{0}, // NSPEC = 0
			wantCount: 0,
		},
		{
			name: "single link specifier",
			data: []byte{
				1,              // NSPEC = 1
				0x00,           // LSTYPE = 0 (IPv4)
				6,              // LSLEN = 6
				192, 168, 1, 1, // IP address
				0x1F, 0x90, // Port 8080
			},
			wantCount: 1,
		},
		{
			name: "multiple link specifiers",
			data: []byte{
				2,    // NSPEC = 2
				0x00, // LSTYPE = 0
				6,
				192, 168, 1, 1,
				0x1F, 0x90,
				0x02, // LSTYPE = 2 (legacy ID)
				20,   // LSLEN = 20
				1, 2, 3, 4, 5, 6, 7, 8, 9, 10,
				11, 12, 13, 14, 15, 16, 17, 18, 19, 20,
			},
			wantCount: 2,
		},
		{
			name:      "truncated data",
			data:      []byte{1, 0x00}, // Missing LSLEN and LSPEC
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			intro := &IntroductionPoint{
				LinkSpecifiers: make([]LinkSpecifier, 0),
			}

			parseLinkSpecifiers(tt.data, intro)

			if len(intro.LinkSpecifiers) != tt.wantCount {
				t.Errorf("parseLinkSpecifiers() got %d link specifiers, want %d",
					len(intro.LinkSpecifiers), tt.wantCount)
			}
		})
	}
}

// TestDecryptDescriptor_Integration tests end-to-end encryption and decryption
func TestDecryptDescriptor_Integration(t *testing.T) {
	// Generate test key pair
	pubkey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	addr := &Address{
		Version: V3,
		Pubkey:  pubkey,
	}

	timePeriod := GetTimePeriod(time.Now())

	// Create plaintext with introduction point
	plaintext := `introduction-point
auth-key
` + base64.StdEncoding.EncodeToString(make([]byte, 32)) + `
onion-key
ntor ` + base64.StdEncoding.EncodeToString(make([]byte, 32)) + `
enc-key ntor ` + base64.StdEncoding.EncodeToString(make([]byte, 32)) + `
`

	// Encrypt the plaintext
	blindedPubkey := ComputeBlindedPubkey(pubkey, timePeriod)
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		t.Fatalf("Failed to generate salt: %v", err)
	}

	keys, err := deriveDescriptorKeys(blindedPubkey, salt, "hsdir-superencrypted-data")
	if err != nil {
		t.Fatalf("Failed to derive keys: %v", err)
	}

	nonce, err := deriveDescriptorKeys(blindedPubkey, salt, "hsdir-superencrypted-nonce")
	if err != nil {
		t.Fatalf("Failed to derive nonce: %v", err)
	}

	aead, err := chacha20poly1305.NewX(keys[:32])
	if err != nil {
		t.Fatalf("Failed to create cipher: %v", err)
	}

	ciphertext := aead.Seal(nil, nonce[:chacha20poly1305.NonceSizeX], []byte(plaintext), nil)

	// Build encrypted descriptor
	encryptedData := append(salt, ciphertext...)
	encryptedB64 := base64.StdEncoding.EncodeToString(encryptedData)

	raw := `hs-descriptor 3
revision-counter 123
superencrypted
-----BEGIN MESSAGE-----
` + encryptedB64 + `
-----END MESSAGE-----
`

	desc := &Descriptor{
		Version:       3,
		RawDescriptor: []byte(raw),
		IntroPoints:   make([]IntroductionPoint, 0),
	}

	// Decrypt the descriptor
	decrypted, err := DecryptDescriptor(desc, addr, timePeriod)
	if err != nil {
		t.Fatalf("DecryptDescriptor() error = %v", err)
	}

	// Verify introduction points were parsed
	if len(decrypted.IntroPoints) != 1 {
		t.Errorf("DecryptDescriptor() got %d intro points, want 1", len(decrypted.IntroPoints))
	}

	if len(decrypted.IntroPoints) > 0 {
		intro := decrypted.IntroPoints[0]
		if len(intro.AuthKey) != 32 {
			t.Errorf("Introduction point auth key length = %d, want 32", len(intro.AuthKey))
		}
		if len(intro.OnionKey) != 32 {
			t.Errorf("Introduction point onion key length = %d, want 32", len(intro.OnionKey))
		}
		if len(intro.EncKey) != 32 {
			t.Errorf("Introduction point enc key length = %d, want 32", len(intro.EncKey))
		}
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && (s[:len(substr)] == substr || contains(s[1:], substr))))
}
