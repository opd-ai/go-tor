package onion

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base32"
	"strings"
	"testing"
)

// TestOnionAddressParsingSecurityAudit performs comprehensive security validation
// of onion address parsing per rend-spec-v3.txt and input validation best practices.
//
// This audit covers:
// 1. Input sanitization and bounds checking
// 2. Malformed input handling
// 3. Injection attack prevention
// 4. Resource exhaustion protection
// 5. Character encoding validation
// 6. Checksum verification security
//
// Audit performed: January 26, 2026
// Specification: rend-spec-v3.txt Section 2 (v3 Onion Address Format)

// TestAddressParsingInputSanitization verifies input sanitization and bounds checking
func TestAddressParsingInputSanitization(t *testing.T) {
	tests := []struct {
		name      string
		address   string
		expectErr bool
		errSubstr string
	}{
		{
			name:      "empty string",
			address:   "",
			expectErr: true,
			errSubstr: "unsupported onion address format",
		},
		{
			name:      "whitespace only",
			address:   "   ",
			expectErr: true,
			errSubstr: "unsupported onion address format",
		},
		{
			name:      "null bytes in address",
			address:   "test\x00test" + strings.Repeat("x", 48) + ".onion",
			expectErr: true,
			errSubstr: "unsupported onion address format",
		},
		{
			name:      "control characters",
			address:   "test\r\ntest" + strings.Repeat("x", 48) + ".onion",
			expectErr: true,
			errSubstr: "unsupported onion address format",
		},
		{
			name:      "unicode characters",
			address:   "tëst" + strings.Repeat("x", 52) + ".onion",
			expectErr: true,
			errSubstr: "unsupported onion address format",
		},
		{
			name:      "mixed case (should work - normalized internally)",
			address:   generateValidAddressWithCase("MiXeD"),
			expectErr: false,
		},
		{
			name:      "all uppercase (should work)",
			address:   generateValidAddressWithCase("UPPER"),
			expectErr: false,
		},
		{
			name:      "leading whitespace",
			address:   "   " + generateValidAddress(t),
			expectErr: true,
			errSubstr: "unsupported onion address format",
		},
		{
			name:      "trailing whitespace",
			address:   generateValidAddress(t) + "   ",
			expectErr: true,
			errSubstr: "unsupported onion address format",
		},
		{
			name:      "embedded whitespace",
			address:   "test" + strings.Repeat("x", 26) + " " + strings.Repeat("x", 26) + ".onion",
			expectErr: true,
			errSubstr: "unsupported onion address format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := ParseAddress(tt.address)

			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected error, got nil")
				} else if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("Expected error containing %q, got: %v", tt.errSubstr, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if parsed == nil {
					t.Errorf("ParseAddress returned nil without error")
				}
			}
		})
	}
}

// TestAddressParsingMalformedInput tests handling of malformed input
func TestAddressParsingMalformedInput(t *testing.T) {
	tests := []struct {
		name      string
		address   string
		expectErr bool
		errSubstr string
	}{
		{
			name:      "invalid base32 alphabet (1,8,9,0)",
			address:   "18901890" + strings.Repeat("a", 48) + ".onion",
			expectErr: true,
			errSubstr: "invalid base32 encoding",
		},
		{
			name:      "special characters",
			address:   "test@#$%" + strings.Repeat("a", 48) + ".onion",
			expectErr: true,
			errSubstr: "invalid base32 encoding",
		},
		{
			name:      "padding characters (not allowed in v3)",
			address:   strings.Repeat("a", 56) + "=" + ".onion",
			expectErr: true,
			errSubstr: "unsupported onion address format",
		},
		{
			name:      "truncated address (incomplete decode)",
			address:   strings.Repeat("a", 50) + ".onion",
			expectErr: true,
			errSubstr: "unsupported onion address format",
		},
		{
			name:      "oversized address (57+ chars)",
			address:   strings.Repeat("a", 60) + ".onion",
			expectErr: true,
			errSubstr: "unsupported onion address format",
		},
		{
			name:      "just .onion suffix",
			address:   ".onion",
			expectErr: true,
			errSubstr: "unsupported onion address format",
		},
		{
			name:      "multiple .onion suffixes",
			address:   generateValidAddress(t) + ".onion",
			expectErr: true,
			errSubstr: "unsupported onion address format",
		},
		{
			name:      "invalid suffix (.onion2)",
			address:   strings.Repeat("a", 56) + ".onion2",
			expectErr: true,
			errSubstr: "unsupported onion address format",
		},
		{
			name:      "no suffix at all (should work if 56 chars)",
			address:   strings.TrimSuffix(generateValidAddress(t), ".onion"),
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := ParseAddress(tt.address)

			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected error, got nil")
				} else if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("Expected error containing %q, got: %v", tt.errSubstr, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if parsed == nil {
					t.Errorf("ParseAddress returned nil without error")
				}
			}
		})
	}
}

// TestAddressParsingInjectionAttackPrevention tests injection attack vectors
func TestAddressParsingInjectionAttackPrevention(t *testing.T) {
	tests := []struct {
		name      string
		address   string
		expectErr bool
		desc      string
	}{
		{
			name:      "SQL injection attempt",
			address:   "'; DROP TABLE addresses; --" + strings.Repeat("a", 28) + ".onion",
			expectErr: true,
			desc:      "SQL injection in address should fail",
		},
		{
			name:      "Shell command injection",
			address:   "$(whoami)" + strings.Repeat("a", 47) + ".onion",
			expectErr: true,
			desc:      "Shell command injection should fail",
		},
		{
			name:      "Path traversal attempt",
			address:   "../../../etc/passwd" + strings.Repeat("a", 37) + ".onion",
			expectErr: true,
			desc:      "Path traversal should fail",
		},
		{
			name:      "Format string injection",
			address:   "%s%s%s%s" + strings.Repeat("a", 48) + ".onion",
			expectErr: true,
			desc:      "Format string injection should fail",
		},
		{
			name:      "XML/HTML injection",
			address:   "<script>alert(1)</script>" + strings.Repeat("a", 31) + ".onion",
			expectErr: true,
			desc:      "XML/HTML injection should fail",
		},
		{
			name:      "LDAP injection",
			address:   "*(uid=*)" + strings.Repeat("a", 48) + ".onion",
			expectErr: true,
			desc:      "LDAP injection should fail",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := ParseAddress(tt.address)

			if tt.expectErr {
				if err == nil {
					t.Errorf("%s: expected error, got nil", tt.desc)
				}
				if parsed != nil {
					t.Errorf("%s: expected nil parsed address, got: %v", tt.desc, parsed)
				}
			} else {
				if err != nil {
					t.Errorf("%s: unexpected error: %v", tt.desc, err)
				}
				if parsed == nil {
					t.Errorf("%s: expected valid parsed address, got nil", tt.desc)
				}
			}
		})
	}
}

// TestAddressParsingResourceExhaustion tests DoS protection
func TestAddressParsingResourceExhaustion(t *testing.T) {
	tests := []struct {
		name      string
		setup     func() string
		expectErr bool
		desc      string
	}{
		{
			name: "extremely long address (10KB)",
			setup: func() string {
				return strings.Repeat("a", 10000) + ".onion"
			},
			expectErr: true,
			desc:      "Very long address should be rejected",
		},
		{
			name: "maximum allowed size (56 chars)",
			setup: func() string {
				return generateValidAddress(t)
			},
			expectErr: false,
			desc:      "Valid 56-char address should succeed",
		},
		{
			name: "repeated dots",
			setup: func() string {
				return strings.Repeat(".", 1000) + "onion"
			},
			expectErr: true,
			desc:      "Repeated dots should be rejected",
		},
		{
			name: "deeply nested structure attempt",
			setup: func() string {
				nested := strings.Repeat(".onion", 100)
				return strings.Repeat("a", 56) + nested
			},
			expectErr: true,
			desc:      "Deeply nested structure should be rejected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr := tt.setup()
			parsed, err := ParseAddress(addr)

			if tt.expectErr {
				if err == nil {
					t.Errorf("%s: expected error, got nil", tt.desc)
				}
			} else {
				if err != nil {
					t.Errorf("%s: unexpected error: %v", tt.desc, err)
				}
				if parsed == nil {
					t.Errorf("%s: expected valid address, got nil", tt.desc)
				}
			}
		})
	}
}

// TestAddressParsingChecksumValidation tests checksum security
func TestAddressParsingChecksumValidation(t *testing.T) {
	t.Run("corrupted checksum detected", func(t *testing.T) {
		// Generate valid address
		pub, _, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatalf("Failed to generate key: %v", err)
		}

		// Compute correct checksum
		checksum := computeV3Checksum(pub, V3Version)

		// Corrupt the checksum
		corruptedChecksum := make([]byte, 2)
		corruptedChecksum[0] = checksum[0] ^ 0xFF
		corruptedChecksum[1] = checksum[1] ^ 0xFF

		// Build address with corrupted checksum
		data := make([]byte, 0, 35)
		data = append(data, pub...)
		data = append(data, corruptedChecksum...)
		data = append(data, V3Version)

		encoder := base32.StdEncoding.WithPadding(base32.NoPadding)
		encoded := strings.ToLower(encoder.EncodeToString(data))
		addr := encoded + ".onion"

		// Should fail checksum validation
		_, err = ParseAddress(addr)
		if err == nil {
			t.Error("Expected checksum validation to fail, got nil error")
		}
		if !strings.Contains(err.Error(), "invalid checksum") {
			t.Errorf("Expected 'invalid checksum' error, got: %v", err)
		}
	})

	t.Run("single bit flip in checksum", func(t *testing.T) {
		pub, _, _ := ed25519.GenerateKey(rand.Reader)
		checksum := computeV3Checksum(pub, V3Version)

		// Flip single bit in first checksum byte
		corruptedChecksum := make([]byte, 2)
		copy(corruptedChecksum, checksum)
		corruptedChecksum[0] ^= 0x01

		data := make([]byte, 0, 35)
		data = append(data, pub...)
		data = append(data, corruptedChecksum...)
		data = append(data, V3Version)

		encoder := base32.StdEncoding.WithPadding(base32.NoPadding)
		encoded := strings.ToLower(encoder.EncodeToString(data))
		addr := encoded + ".onion"

		_, err := ParseAddress(addr)
		if err == nil {
			t.Error("Single bit flip should be detected")
		}
	})

	t.Run("checksum collision resistance", func(t *testing.T) {
		// Generate two different keys
		pub1, _, _ := ed25519.GenerateKey(rand.Reader)
		pub2, _, _ := ed25519.GenerateKey(rand.Reader)

		checksum1 := computeV3Checksum(pub1, V3Version)
		checksum2 := computeV3Checksum(pub2, V3Version)

		// Checksums should be different for different keys
		if bytes.Equal(checksum1, checksum2) {
			t.Error("Checksum collision detected (unlikely but should not happen)")
		}
	})
}

// TestAddressParsingVersionValidation tests version byte validation
func TestAddressParsingVersionValidation(t *testing.T) {
	tests := []struct {
		name      string
		version   byte
		expectErr bool
	}{
		{
			name:      "valid version 0x03",
			version:   0x03,
			expectErr: false,
		},
		{
			name:      "invalid version 0x00",
			version:   0x00,
			expectErr: true,
		},
		{
			name:      "invalid version 0x01",
			version:   0x01,
			expectErr: true,
		},
		{
			name:      "invalid version 0x02",
			version:   0x02,
			expectErr: true,
		},
		{
			name:      "invalid version 0x04",
			version:   0x04,
			expectErr: true,
		},
		{
			name:      "invalid version 0xFF",
			version:   0xFF,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pub, _, err := ed25519.GenerateKey(rand.Reader)
			if err != nil {
				t.Fatalf("Failed to generate key: %v", err)
			}

			checksum := computeV3Checksum(pub, tt.version)
			data := make([]byte, 0, 35)
			data = append(data, pub...)
			data = append(data, checksum...)
			data = append(data, tt.version)

			encoder := base32.StdEncoding.WithPadding(base32.NoPadding)
			encoded := strings.ToLower(encoder.EncodeToString(data))
			addr := encoded + ".onion"

			_, err = ParseAddress(addr)
			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected error for version 0x%02x, got nil", tt.version)
				}
				if !strings.Contains(err.Error(), "invalid version") {
					t.Errorf("Expected 'invalid version' error, got: %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for valid version: %v", err)
				}
			}
		})
	}
}

// TestAddressParsingLengthValidation tests length validation edge cases
func TestAddressParsingLengthValidation(t *testing.T) {
	tests := []struct {
		name      string
		length    int
		expectErr bool
	}{
		{
			name:      "exactly 56 characters (valid)",
			length:    56,
			expectErr: false,
		},
		{
			name:      "55 characters (too short)",
			length:    55,
			expectErr: true,
		},
		{
			name:      "57 characters (too long)",
			length:    57,
			expectErr: true,
		},
		{
			name:      "0 characters (empty)",
			length:    0,
			expectErr: true,
		},
		{
			name:      "1 character",
			length:    1,
			expectErr: true,
		},
		{
			name:      "100 characters",
			length:    100,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var addr string
			if tt.length == 56 {
				addr = generateValidAddress(t)
			} else {
				addr = strings.Repeat("a", tt.length) + ".onion"
			}

			_, err := ParseAddress(addr)
			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected error for length %d, got nil", tt.length)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for valid length: %v", err)
				}
			}
		})
	}
}

// TestAddressParsingConcurrentSafety tests thread safety
func TestAddressParsingConcurrentSafety(t *testing.T) {
	validAddr := generateValidAddress(t)
	invalidAddr := "invalid" + strings.Repeat("x", 50) + ".onion"

	const numGoroutines = 100
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Goroutine %d panicked: %v", id, r)
				}
				done <- true
			}()

			// Parse valid address
			_, err := ParseAddress(validAddr)
			if err != nil {
				t.Errorf("Goroutine %d: unexpected error on valid address: %v", id, err)
			}

			// Parse invalid address
			_, err = ParseAddress(invalidAddr)
			if err == nil {
				t.Errorf("Goroutine %d: expected error on invalid address, got nil", id)
			}
		}(i)
	}

	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}

// TestAddressParsingRoundTrip verifies parse-encode-parse consistency
func TestAddressParsingRoundTrip(t *testing.T) {
	for i := 0; i < 10; i++ {
		// Generate valid address
		original := generateValidAddress(t)

		// Parse it
		parsed1, err := ParseAddress(original)
		if err != nil {
			t.Fatalf("Round trip %d: first parse failed: %v", i, err)
		}

		// Encode it back
		encoded := parsed1.Encode()

		// Parse again
		parsed2, err := ParseAddress(encoded)
		if err != nil {
			t.Fatalf("Round trip %d: second parse failed: %v", i, err)
		}

		// Verify public keys match
		if !bytes.Equal(parsed1.Pubkey, parsed2.Pubkey) {
			t.Errorf("Round trip %d: public keys don't match", i)
		}

		// Verify versions match
		if parsed1.Version != parsed2.Version {
			t.Errorf("Round trip %d: versions don't match", i)
		}
	}
}

// Helper functions

func generateValidAddress(t *testing.T) string {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	checksum := computeV3Checksum(pub, V3Version)
	data := make([]byte, 0, 35)
	data = append(data, pub...)
	data = append(data, checksum...)
	data = append(data, V3Version)

	encoder := base32.StdEncoding.WithPadding(base32.NoPadding)
	encoded := strings.ToLower(encoder.EncodeToString(data))

	return encoded + ".onion"
}

func generateValidAddressWithCase(caseType string) string {
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	checksum := computeV3Checksum(pub, V3Version)
	data := make([]byte, 0, 35)
	data = append(data, pub...)
	data = append(data, checksum...)
	data = append(data, V3Version)

	encoder := base32.StdEncoding.WithPadding(base32.NoPadding)
	encoded := encoder.EncodeToString(data)

	switch caseType {
	case "UPPER":
		encoded = strings.ToUpper(encoded)
	case "MiXeD":
		// Mix case: alternate every 2 characters
		runes := []rune(encoded)
		for i := 0; i < len(runes); i++ {
			if i%4 < 2 {
				runes[i] = []rune(strings.ToUpper(string(runes[i])))[0]
			} else {
				runes[i] = []rune(strings.ToLower(string(runes[i])))[0]
			}
		}
		encoded = string(runes)
	default:
		encoded = strings.ToLower(encoded)
	}

	return encoded + ".onion"
}
