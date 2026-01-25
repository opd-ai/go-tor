package onion

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha3"
	"encoding/base32"
	"strings"
	"testing"
)

// TestV3AddressSpecCompliance_Format verifies v3 onion address format per rend-spec-v3.txt
// Section 2: "A v3 onion address is 56 characters long, consisting of the
// public key (32 bytes), a checksum (2 bytes), and a version byte (1 byte),
// all base32-encoded without padding."
func TestV3AddressSpecCompliance_Format(t *testing.T) {
	tests := []struct {
		name          string
		setup         func() string
		expectSuccess bool
		checkLength   bool
	}{
		{
			name: "valid v3 address - 56 characters",
			setup: func() string {
				// Generate a valid ed25519 key pair
				pub, _, err := ed25519.GenerateKey(rand.Reader)
				if err != nil {
					t.Fatalf("Failed to generate key: %v", err)
				}

				// Compute checksum per spec: SHA3-256(".onion checksum" || pubkey || version)[:2]
				checksum := computeV3Checksum(pub, V3Version)

				// Construct: pubkey (32) || checksum (2) || version (1) = 35 bytes
				data := make([]byte, 0, 35)
				data = append(data, pub...)
				data = append(data, checksum...)
				data = append(data, V3Version)

				// Encode to base32 (no padding)
				encoder := base32.StdEncoding.WithPadding(base32.NoPadding)
				encoded := strings.ToLower(encoder.EncodeToString(data))

				return encoded + ".onion"
			},
			expectSuccess: true,
			checkLength:   true,
		},
		{
			name: "address without .onion suffix",
			setup: func() string {
				pub, _, _ := ed25519.GenerateKey(rand.Reader)
				checksum := computeV3Checksum(pub, V3Version)
				data := append(pub, checksum...)
				data = append(data, V3Version)
				encoder := base32.StdEncoding.WithPadding(base32.NoPadding)
				return strings.ToLower(encoder.EncodeToString(data))
			},
			expectSuccess: true,
			checkLength:   true,
		},
		{
			name: "invalid - too short (55 characters)",
			setup: func() string {
				return "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxy.onion"
			},
			expectSuccess: false,
			checkLength:   false,
		},
		{
			name: "invalid - too long (57 characters)",
			setup: func() string {
				return "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyza.onion"
			},
			expectSuccess: false,
			checkLength:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr := tt.setup()
			addrWithoutSuffix := strings.TrimSuffix(addr, ".onion")

			if tt.checkLength {
				// Verify 56 character length requirement per spec
				if len(addrWithoutSuffix) != V3AddressLength {
					t.Errorf("Address length = %d, want %d", len(addrWithoutSuffix), V3AddressLength)
				}
			}

			parsed, err := ParseAddress(addr)
			if tt.expectSuccess {
				if err != nil {
					t.Errorf("ParseAddress() unexpected error = %v", err)
				}
				if parsed == nil {
					t.Errorf("ParseAddress() returned nil")
				}
			} else {
				if err == nil {
					t.Errorf("ParseAddress() expected error, got nil")
				}
			}
		})
	}
}

// TestV3AddressSpecCompliance_Encoding verifies base32 encoding per rend-spec-v3.txt
// "The onion address is base32-encoded using RFC 4648 encoding without padding"
func TestV3AddressSpecCompliance_Encoding(t *testing.T) {
	tests := []struct {
		name        string
		setup       func() (string, []byte)
		expectError bool
	}{
		{
			name: "lowercase base32 (canonical form)",
			setup: func() (string, []byte) {
				pub, _, _ := ed25519.GenerateKey(rand.Reader)
				checksum := computeV3Checksum(pub, V3Version)
				data := append(pub, checksum...)
				data = append(data, V3Version)
				encoder := base32.StdEncoding.WithPadding(base32.NoPadding)
				encoded := strings.ToLower(encoder.EncodeToString(data))
				return encoded, pub
			},
			expectError: false,
		},
		{
			name: "uppercase base32 (should be accepted)",
			setup: func() (string, []byte) {
				pub, _, _ := ed25519.GenerateKey(rand.Reader)
				checksum := computeV3Checksum(pub, V3Version)
				data := append(pub, checksum...)
				data = append(data, V3Version)
				encoder := base32.StdEncoding.WithPadding(base32.NoPadding)
				encoded := strings.ToUpper(encoder.EncodeToString(data))
				return encoded, pub
			},
			expectError: false,
		},
		{
			name: "mixed case base32 (should be accepted)",
			setup: func() (string, []byte) {
				pub, _, _ := ed25519.GenerateKey(rand.Reader)
				checksum := computeV3Checksum(pub, V3Version)
				data := append(pub, checksum...)
				data = append(data, V3Version)
				encoder := base32.StdEncoding.WithPadding(base32.NoPadding)
				encoded := encoder.EncodeToString(data)
				// Mix case
				runes := []rune(encoded)
				for i := range runes {
					if i%2 == 0 {
						runes[i] = []rune(strings.ToLower(string(runes[i])))[0]
					}
				}
				return string(runes), pub
			},
			expectError: false,
		},
		{
			name: "invalid characters (non-base32)",
			setup: func() (string, []byte) {
				// Include invalid characters like 1, 8, 9 (not in base32 alphabet)
				return "1111111111111111111111111111111111111111111111111111111", nil
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, expectedPubkey := tt.setup()

			parsed, err := ParseAddress(encoded)
			if tt.expectError {
				if err == nil {
					t.Errorf("ParseAddress() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ParseAddress() unexpected error = %v", err)
				return
			}

			// Verify public key was extracted correctly
			if expectedPubkey != nil && !bytes.Equal(parsed.Pubkey, expectedPubkey) {
				t.Errorf("Extracted public key doesn't match expected")
			}
		})
	}
}

// TestV3AddressSpecCompliance_Checksum verifies checksum computation per rend-spec-v3.txt
// "checksum = H('.onion checksum' || pubkey || version)[:2]"
// where H is SHA3-256
func TestV3AddressSpecCompliance_Checksum(t *testing.T) {
	tests := []struct {
		name          string
		setup         func() (string, bool)
		expectSuccess bool
	}{
		{
			name: "valid checksum",
			setup: func() (string, bool) {
				pub, _, _ := ed25519.GenerateKey(rand.Reader)
				checksum := computeV3Checksum(pub, V3Version)
				data := append(pub, checksum...)
				data = append(data, V3Version)
				encoder := base32.StdEncoding.WithPadding(base32.NoPadding)
				return strings.ToLower(encoder.EncodeToString(data)), true
			},
			expectSuccess: true,
		},
		{
			name: "invalid checksum (first byte flipped)",
			setup: func() (string, bool) {
				pub, _, _ := ed25519.GenerateKey(rand.Reader)
				checksum := computeV3Checksum(pub, V3Version)
				// Corrupt checksum
				checksum[0] ^= 0xFF
				data := append(pub, checksum...)
				data = append(data, V3Version)
				encoder := base32.StdEncoding.WithPadding(base32.NoPadding)
				return strings.ToLower(encoder.EncodeToString(data)), false
			},
			expectSuccess: false,
		},
		{
			name: "invalid checksum (second byte flipped)",
			setup: func() (string, bool) {
				pub, _, _ := ed25519.GenerateKey(rand.Reader)
				checksum := computeV3Checksum(pub, V3Version)
				// Corrupt checksum
				checksum[1] ^= 0xFF
				data := append(pub, checksum...)
				data = append(data, V3Version)
				encoder := base32.StdEncoding.WithPadding(base32.NoPadding)
				return strings.ToLower(encoder.EncodeToString(data)), false
			},
			expectSuccess: false,
		},
		{
			name: "invalid checksum (both bytes flipped)",
			setup: func() (string, bool) {
				pub, _, _ := ed25519.GenerateKey(rand.Reader)
				checksum := computeV3Checksum(pub, V3Version)
				// Corrupt checksum
				checksum[0] ^= 0xFF
				checksum[1] ^= 0xFF
				data := append(pub, checksum...)
				data = append(data, V3Version)
				encoder := base32.StdEncoding.WithPadding(base32.NoPadding)
				return strings.ToLower(encoder.EncodeToString(data)), false
			},
			expectSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr, shouldSucceed := tt.setup()

			parsed, err := ParseAddress(addr)
			if shouldSucceed {
				if err != nil {
					t.Errorf("ParseAddress() unexpected error = %v", err)
				}
				if parsed == nil {
					t.Errorf("ParseAddress() returned nil")
				}
			} else {
				if err == nil {
					t.Errorf("ParseAddress() expected error, got nil")
				}
				if !strings.Contains(err.Error(), "invalid checksum") {
					t.Errorf("ParseAddress() error = %v, want 'invalid checksum'", err)
				}
			}
		})
	}
}

// TestV3AddressSpecCompliance_ChecksumAlgorithm verifies the checksum algorithm
// Per rend-spec-v3.txt: checksum = SHA3-256(".onion checksum" || pubkey || version)[:2]
func TestV3AddressSpecCompliance_ChecksumAlgorithm(t *testing.T) {
	tests := []struct {
		name     string
		pubkey   []byte
		version  byte
		expected []byte
	}{
		{
			name:    "known test vector 1",
			pubkey:  make([]byte, 32), // All zeros
			version: V3Version,
			expected: func() []byte {
				h := sha3.New256()
				h.Write([]byte(".onion checksum"))
				h.Write(make([]byte, 32))
				h.Write([]byte{V3Version})
				return h.Sum(nil)[:2]
			}(),
		},
		{
			name:    "known test vector 2",
			pubkey:  bytes32(0xFF), // All 0xFF
			version: V3Version,
			expected: func() []byte {
				h := sha3.New256()
				h.Write([]byte(".onion checksum"))
				h.Write(bytes32(0xFF))
				h.Write([]byte{V3Version})
				return h.Sum(nil)[:2]
			}(),
		},
		{
			name:    "known test vector 3",
			pubkey:  sequentialBytes(32), // 0x00, 0x01, 0x02, ...
			version: V3Version,
			expected: func() []byte {
				h := sha3.New256()
				h.Write([]byte(".onion checksum"))
				h.Write(sequentialBytes(32))
				h.Write([]byte{V3Version})
				return h.Sum(nil)[:2]
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			computed := computeV3Checksum(tt.pubkey, tt.version)

			if !bytes.Equal(computed, tt.expected) {
				t.Errorf("computeV3Checksum() = %x, want %x", computed, tt.expected)
			}

			// Verify the checksum is exactly 2 bytes
			if len(computed) != V3ChecksumLen {
				t.Errorf("Checksum length = %d, want %d", len(computed), V3ChecksumLen)
			}
		})
	}
}

// TestV3AddressSpecCompliance_VersionByte verifies version byte handling
// Per rend-spec-v3.txt: version byte must be 0x03 for v3 onion services
func TestV3AddressSpecCompliance_VersionByte(t *testing.T) {
	tests := []struct {
		name          string
		version       byte
		expectSuccess bool
	}{
		{
			name:          "valid version 0x03",
			version:       V3Version,
			expectSuccess: true,
		},
		{
			name:          "invalid version 0x00",
			version:       0x00,
			expectSuccess: false,
		},
		{
			name:          "invalid version 0x01",
			version:       0x01,
			expectSuccess: false,
		},
		{
			name:          "invalid version 0x02",
			version:       0x02,
			expectSuccess: false,
		},
		{
			name:          "invalid version 0x04",
			version:       0x04,
			expectSuccess: false,
		},
		{
			name:          "invalid version 0xFF",
			version:       0xFF,
			expectSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pub, _, _ := ed25519.GenerateKey(rand.Reader)
			checksum := computeV3Checksum(pub, tt.version)
			data := append(pub, checksum...)
			data = append(data, tt.version)
			encoder := base32.StdEncoding.WithPadding(base32.NoPadding)
			addr := strings.ToLower(encoder.EncodeToString(data))

			parsed, err := ParseAddress(addr)
			if tt.expectSuccess {
				if err != nil {
					t.Errorf("ParseAddress() unexpected error = %v", err)
				}
				if parsed == nil {
					t.Errorf("ParseAddress() returned nil")
				} else if parsed.Version != V3 {
					t.Errorf("Parsed version = %v, want V3", parsed.Version)
				}
			} else {
				if err == nil {
					t.Errorf("ParseAddress() expected error, got nil")
				}
				if !strings.Contains(err.Error(), "invalid version") {
					t.Errorf("ParseAddress() error = %v, want 'invalid version'", err)
				}
			}
		})
	}
}

// TestV3AddressSpecCompliance_PublicKeyExtraction verifies public key extraction
// Per rend-spec-v3.txt: The first 32 bytes contain the ed25519 public key
func TestV3AddressSpecCompliance_PublicKeyExtraction(t *testing.T) {
	tests := []struct {
		name       string
		genPubkey  func() []byte
		checkMatch bool
	}{
		{
			name: "random ed25519 public key",
			genPubkey: func() []byte {
				pub, _, _ := ed25519.GenerateKey(rand.Reader)
				return pub
			},
			checkMatch: true,
		},
		{
			name: "all zeros public key",
			genPubkey: func() []byte {
				return make([]byte, 32)
			},
			checkMatch: true,
		},
		{
			name: "all ones public key",
			genPubkey: func() []byte {
				return bytes32(0xFF)
			},
			checkMatch: true,
		},
		{
			name: "sequential bytes public key",
			genPubkey: func() []byte {
				return sequentialBytes(32)
			},
			checkMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectedPubkey := tt.genPubkey()
			checksum := computeV3Checksum(expectedPubkey, V3Version)
			data := append(expectedPubkey, checksum...)
			data = append(data, V3Version)
			encoder := base32.StdEncoding.WithPadding(base32.NoPadding)
			addr := strings.ToLower(encoder.EncodeToString(data))

			parsed, err := ParseAddress(addr)
			if err != nil {
				t.Errorf("ParseAddress() unexpected error = %v", err)
				return
			}

			// Verify public key length
			if len(parsed.Pubkey) != V3PubkeyLen {
				t.Errorf("Public key length = %d, want %d", len(parsed.Pubkey), V3PubkeyLen)
			}

			// Verify public key matches
			if tt.checkMatch && !bytes.Equal(parsed.Pubkey, expectedPubkey) {
				t.Errorf("Extracted public key doesn't match expected")
			}
		})
	}
}

// TestV3AddressSpecCompliance_RoundTrip verifies encoding and parsing are inverses
func TestV3AddressSpecCompliance_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		runs int
	}{
		{
			name: "single round trip",
			runs: 1,
		},
		{
			name: "multiple round trips",
			runs: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i := 0; i < tt.runs; i++ {
				// Generate a random ed25519 key pair
				pub, _, err := ed25519.GenerateKey(rand.Reader)
				if err != nil {
					t.Fatalf("Failed to generate key: %v", err)
				}

				// Create address from public key
				addr := &Address{
					Version: V3,
					Pubkey:  pub,
				}

				// Encode to string
				encoded := addr.Encode()

				// Parse back
				parsed, err := ParseAddress(encoded)
				if err != nil {
					t.Errorf("Round trip %d: ParseAddress() error = %v", i, err)
					continue
				}

				// Verify version matches
				if parsed.Version != addr.Version {
					t.Errorf("Round trip %d: Version = %v, want %v", i, parsed.Version, addr.Version)
				}

				// Verify public key matches
				if !bytes.Equal(parsed.Pubkey, addr.Pubkey) {
					t.Errorf("Round trip %d: Public key mismatch", i)
				}

				// Verify re-encoding produces same address
				reencoded := parsed.Encode()
				if reencoded != encoded {
					t.Errorf("Round trip %d: Re-encoded address doesn't match original", i)
				}
			}
		})
	}
}

// TestV3AddressSpecCompliance_EdgeCases tests edge cases in address parsing
func TestV3AddressSpecCompliance_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		setup       func() string
		expectError bool
		errorMsg    string
	}{
		{
			name: "address with trailing whitespace",
			setup: func() string {
				pub, _, _ := ed25519.GenerateKey(rand.Reader)
				checksum := computeV3Checksum(pub, V3Version)
				data := append(pub, checksum...)
				data = append(data, V3Version)
				encoder := base32.StdEncoding.WithPadding(base32.NoPadding)
				return strings.ToLower(encoder.EncodeToString(data)) + ".onion   "
			},
			expectError: true, // Implementation doesn't trim whitespace from full address
			errorMsg:    "unsupported onion address format",
		},
		{
			name: "address with leading whitespace",
			setup: func() string {
				pub, _, _ := ed25519.GenerateKey(rand.Reader)
				checksum := computeV3Checksum(pub, V3Version)
				data := append(pub, checksum...)
				data = append(data, V3Version)
				encoder := base32.StdEncoding.WithPadding(base32.NoPadding)
				return "   " + strings.ToLower(encoder.EncodeToString(data)) + ".onion"
			},
			expectError: true, // Leading whitespace causes length mismatch
			errorMsg:    "unsupported onion address format",
		},
		{
			name: "empty string",
			setup: func() string {
				return ""
			},
			expectError: true,
			errorMsg:    "unsupported onion address format",
		},
		{
			name: "just .onion suffix",
			setup: func() string {
				return ".onion"
			},
			expectError: true,
			errorMsg:    "unsupported onion address format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr := tt.setup()

			parsed, err := ParseAddress(addr)
			if tt.expectError {
				if err == nil {
					t.Errorf("ParseAddress() expected error, got nil")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("ParseAddress() error = %v, want substring %q", err, tt.errorMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("ParseAddress() unexpected error = %v", err)
			}
			if parsed == nil {
				t.Errorf("ParseAddress() returned nil")
			}
		})
	}
}

// Helper functions

// bytes32 creates a 32-byte slice filled with the given value
func bytes32(val byte) []byte {
	b := make([]byte, 32)
	for i := range b {
		b[i] = val
	}
	return b
}

// sequentialBytes creates a byte slice with sequential values 0x00, 0x01, 0x02, ...
func sequentialBytes(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i)
	}
	return b
}
