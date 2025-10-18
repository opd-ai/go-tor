package onion

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base32"
	"strings"
	"testing"
)

// TestParseV3Address tests parsing of v3 onion addresses
func TestParseV3Address(t *testing.T) {
	tests := []struct {
		name      string
		address   string
		wantErr   bool
		errString string
	}{
		{
			name: "valid v3 address with .onion",
			// This is a properly formed v3 address (generated below)
			address: generateValidV3Address(t),
			wantErr: false,
		},
		{
			name: "valid v3 address without .onion",
			// Will be generated and stripped of .onion
			address: strings.TrimSuffix(generateValidV3Address(t), ".onion"),
			wantErr: false,
		},
		{
			name:      "invalid length - too short",
			address:   "thisiswaytooshort.onion",
			wantErr:   true,
			errString: "unsupported onion address format",
		},
		{
			name:      "invalid length - too long",
			address:   "thisistoolongforanyonionaddressformatthatweknowabout.onion",
			wantErr:   true,
			errString: "unsupported onion address format",
		},
		{
			name:      "invalid base32 encoding",
			address:   "!!!invalid!!!base32!!!encoding!!!0123456789abcdef.onion",
			wantErr:   true,
			errString: "unsupported onion address format",
		},
		{
			name:      "invalid checksum",
			address:   generateInvalidChecksumAddress(t),
			wantErr:   true,
			errString: "invalid checksum",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr, err := ParseAddress(tt.address)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseAddress() expected error, got nil")
				} else if tt.errString != "" && !strings.Contains(err.Error(), tt.errString) {
					t.Errorf("ParseAddress() error = %v, want substring %v", err, tt.errString)
				}
				return
			}
			if err != nil {
				t.Errorf("ParseAddress() unexpected error = %v", err)
				return
			}
			if addr == nil {
				t.Errorf("ParseAddress() returned nil address")
				return
			}
			if addr.Version != V3 {
				t.Errorf("ParseAddress() version = %v, want V3", addr.Version)
			}
			if len(addr.Pubkey) != V3PubkeyLen {
				t.Errorf("ParseAddress() pubkey length = %d, want %d", len(addr.Pubkey), V3PubkeyLen)
			}
		})
	}
}

// TestAddressEncode tests encoding addresses back to string format
func TestAddressEncode(t *testing.T) {
	// Generate a valid v3 address
	original := generateValidV3Address(t)

	// Parse it
	addr, err := ParseAddress(original)
	if err != nil {
		t.Fatalf("ParseAddress() failed: %v", err)
	}

	// Encode it back
	encoded := addr.Encode()

	// Should match original (case-insensitive)
	if !strings.EqualFold(encoded, original) {
		t.Errorf("Encode() = %v, want %v", encoded, original)
	}

	// Parse again to verify round-trip
	addr2, err := ParseAddress(encoded)
	if err != nil {
		t.Fatalf("ParseAddress() second time failed: %v", err)
	}

	// Pubkeys should match
	if len(addr.Pubkey) != len(addr2.Pubkey) {
		t.Errorf("Pubkey length mismatch: %d vs %d", len(addr.Pubkey), len(addr2.Pubkey))
	}
	for i := range addr.Pubkey {
		if addr.Pubkey[i] != addr2.Pubkey[i] {
			t.Errorf("Pubkey mismatch at byte %d: %02x vs %02x", i, addr.Pubkey[i], addr2.Pubkey[i])
		}
	}
}

// TestAddressString tests the String() method
func TestAddressString(t *testing.T) {
	original := generateValidV3Address(t)
	addr, err := ParseAddress(original)
	if err != nil {
		t.Fatalf("ParseAddress() failed: %v", err)
	}

	str := addr.String()
	if !strings.HasSuffix(str, ".onion") {
		t.Errorf("String() = %v, want suffix .onion", str)
	}
	if len(strings.TrimSuffix(str, ".onion")) != V3AddressLength {
		t.Errorf("String() address part length = %d, want %d",
			len(strings.TrimSuffix(str, ".onion")), V3AddressLength)
	}
}

// TestIsOnionAddress tests the IsOnionAddress helper
func TestIsOnionAddress(t *testing.T) {
	tests := []struct {
		name string
		addr string
		want bool
	}{
		{"valid .onion", "test.onion", true},
		{"valid long .onion", generateValidV3Address(t), true},
		{"no .onion suffix", "example.com", false},
		{"partial .onion", "test.onio", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsOnionAddress(tt.addr); got != tt.want {
				t.Errorf("IsOnionAddress() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestV3ChecksumComputation tests the checksum computation
func TestV3ChecksumComputation(t *testing.T) {
	// Use a known pubkey
	pubkey := make([]byte, V3PubkeyLen)
	for i := range pubkey {
		pubkey[i] = byte(i)
	}

	// Compute checksum twice - should be deterministic
	checksum1 := computeV3Checksum(pubkey, V3Version)
	checksum2 := computeV3Checksum(pubkey, V3Version)

	if len(checksum1) != V3ChecksumLen {
		t.Errorf("computeV3Checksum() returned %d bytes, want %d", len(checksum1), V3ChecksumLen)
	}

	if checksum1[0] != checksum2[0] || checksum1[1] != checksum2[1] {
		t.Errorf("computeV3Checksum() not deterministic: %v vs %v", checksum1, checksum2)
	}

	// Different pubkey should give different checksum
	pubkey[0] = 0xFF
	checksum3 := computeV3Checksum(pubkey, V3Version)
	if checksum1[0] == checksum3[0] && checksum1[1] == checksum3[1] {
		t.Errorf("computeV3Checksum() same for different pubkeys")
	}
}

// TestRealWorldV3Address tests with a real-world v3 address format
func TestRealWorldV3Address(t *testing.T) {
	// DuckDuckGo's onion address (example of a real v3 address format)
	// Note: Using a similar format, not the actual address for testing
	realAddr := generateValidV3Address(t)

	addr, err := ParseAddress(realAddr)
	if err != nil {
		t.Fatalf("ParseAddress() failed for real-world format: %v", err)
	}

	if addr.Version != V3 {
		t.Errorf("ParseAddress() version = %v, want V3", addr.Version)
	}

	// Verify it can be encoded back
	encoded := addr.Encode()
	if !strings.EqualFold(encoded, realAddr) {
		t.Errorf("Round-trip failed: encoded = %v, original = %v", encoded, realAddr)
	}
}

// Helper function to generate a valid v3 address
func generateValidV3Address(t *testing.T) string {
	// Generate a random ed25519 public key
	pubkey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate ed25519 key: %v", err)
	}

	// Construct the address: pubkey || checksum || version
	checksum := computeV3Checksum(pubkey, V3Version)
	data := make([]byte, 0, V3PubkeyLen+V3ChecksumLen+1)
	data = append(data, pubkey...)
	data = append(data, checksum...)
	data = append(data, V3Version)

	// Encode to base32
	encoder := base32.StdEncoding.WithPadding(base32.NoPadding)
	encoded := strings.ToLower(encoder.EncodeToString(data))

	return encoded + ".onion"
}

// Helper function to generate an address with invalid checksum
func generateInvalidChecksumAddress(t *testing.T) string {
	// Generate a random ed25519 public key
	pubkey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate ed25519 key: %v", err)
	}

	// Construct with WRONG checksum
	wrongChecksum := []byte{0xFF, 0xFF}
	data := make([]byte, 0, V3PubkeyLen+V3ChecksumLen+1)
	data = append(data, pubkey...)
	data = append(data, wrongChecksum...)
	data = append(data, V3Version)

	// Encode to base32
	encoder := base32.StdEncoding.WithPadding(base32.NoPadding)
	encoded := strings.ToLower(encoder.EncodeToString(data))

	return encoded + ".onion"
}

// TestNewClient tests creating a new onion service client
func TestNewClient(t *testing.T) {
	client := NewClient()
	if client == nil {
		t.Errorf("NewClient() returned nil")
	}
}

// Benchmark tests
func BenchmarkParseV3Address(b *testing.B) {
	addr := generateValidV3AddressBench(b)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = ParseAddress(addr)
	}
}

func BenchmarkEncodeV3Address(b *testing.B) {
	addr := generateValidV3AddressBench(b)
	parsed, err := ParseAddress(addr)
	if err != nil {
		b.Fatalf("Failed to parse address: %v", err)
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = parsed.Encode()
	}
}

func generateValidV3AddressBench(b *testing.B) string {
	pubkey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		b.Fatalf("Failed to generate ed25519 key: %v", err)
	}

	checksum := computeV3Checksum(pubkey, V3Version)
	data := make([]byte, 0, V3PubkeyLen+V3ChecksumLen+1)
	data = append(data, pubkey...)
	data = append(data, checksum...)
	data = append(data, V3Version)

	encoder := base32.StdEncoding.WithPadding(base32.NoPadding)
	encoded := strings.ToLower(encoder.EncodeToString(data))

	return encoded + ".onion"
}
