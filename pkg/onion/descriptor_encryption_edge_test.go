// Package onion provides edge-case tests for descriptor encryption
// and decryption per rend-spec-v3.txt §2.4-2.5.
//
// These tests verify that descriptor parsing and decryption correctly
// handle malformed inputs, boundary conditions, and adversarial data.
// Onion service descriptors are fetched from the Tor network and must
// be parsed defensively.
//
// Compliance: rend-spec-v3.txt §2.4 (Descriptor Format), §2.5 (Encryption)
package onion

import (
	"crypto/rand"
	"strings"
	"testing"
)

// TestDecryptDescriptorEdgeCases verifies DecryptDescriptor handles
// various malformed inputs without panics.
func TestDecryptDescriptorEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		descriptor  *Descriptor
		address     *Address
		timePeriod  uint64
		wantErr     bool
		errContains string
	}{
		{
			name:        "nil descriptor",
			descriptor:  nil,
			address:     &Address{Pubkey: make([]byte, 32)},
			timePeriod:  1,
			wantErr:     true,
			errContains: "descriptor is nil",
		},
		{
			name:        "nil address",
			descriptor:  &Descriptor{RawDescriptor: []byte("test")},
			address:     nil,
			timePeriod:  1,
			wantErr:     true,
			errContains: "address is nil",
		},
		{
			name:       "empty pubkey in address",
			descriptor: &Descriptor{RawDescriptor: []byte("test")},
			address:    &Address{Pubkey: []byte{}},
			timePeriod: 1,
			wantErr:    true,
			errContains: "invalid public key length",
		},
		{
			name:       "short pubkey (31 bytes)",
			descriptor: &Descriptor{RawDescriptor: []byte("test")},
			address:    &Address{Pubkey: make([]byte, 31)},
			timePeriod: 1,
			wantErr:    true,
			errContains: "invalid public key length",
		},
		{
			name:       "long pubkey (33 bytes)",
			descriptor: &Descriptor{RawDescriptor: []byte("test")},
			address:    &Address{Pubkey: make([]byte, 33)},
			timePeriod: 1,
			wantErr:    true,
			errContains: "invalid public key length",
		},
		{
			name:       "no superencrypted section",
			descriptor: &Descriptor{RawDescriptor: []byte("hs-descriptor 3\n")},
			address:    &Address{Pubkey: make([]byte, 32)},
			timePeriod: 1,
			wantErr:    false, // Returns descriptor as-is
		},
		{
			name:       "superencrypted but no BEGIN marker",
			descriptor: &Descriptor{RawDescriptor: []byte("superencrypted\nsome data\n")},
			address:    &Address{Pubkey: make([]byte, 32)},
			timePeriod: 1,
			wantErr:    true,
			errContains: "missing BEGIN MESSAGE marker",
		},
		{
			name: "superencrypted with BEGIN but no END",
			descriptor: &Descriptor{
				RawDescriptor: []byte("superencrypted\n-----BEGIN MESSAGE-----\ndata\n"),
			},
			address:    &Address{Pubkey: make([]byte, 32)},
			timePeriod: 1,
			wantErr:    true,
			errContains: "missing END MESSAGE marker",
		},
		{
			name: "superencrypted with invalid base64",
			descriptor: &Descriptor{
				RawDescriptor: []byte("superencrypted\n-----BEGIN MESSAGE-----\n!!!invalid!!!\n-----END MESSAGE-----\n"),
			},
			address:    &Address{Pubkey: make([]byte, 32)},
			timePeriod: 1,
			wantErr:    true,
			errContains: "failed to decode encrypted data",
		},
		{
			name: "superencrypted with valid base64 but too short",
			descriptor: &Descriptor{
				RawDescriptor: []byte("superencrypted\n-----BEGIN MESSAGE-----\ndGVzdA==\n-----END MESSAGE-----\n"),
			},
			address:    &Address{Pubkey: make([]byte, 32)},
			timePeriod: 1,
			wantErr:    true,
			errContains: "encrypted data too short",
		},
		{
			name:       "time period zero",
			descriptor: &Descriptor{RawDescriptor: []byte("hs-descriptor 3\n")},
			address:    &Address{Pubkey: make([]byte, 32)},
			timePeriod: 0,
			wantErr:    false,
		},
		{
			name:       "time period max uint64",
			descriptor: &Descriptor{RawDescriptor: []byte("hs-descriptor 3\n")},
			address:    &Address{Pubkey: make([]byte, 32)},
			timePeriod: ^uint64(0),
			wantErr:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := DecryptDescriptor(tc.descriptor, tc.address, tc.timePeriod)

			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tc.errContains != "" && !strings.Contains(err.Error(), tc.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.errContains)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if result == nil {
					t.Error("result is nil")
				}
			}
		})
	}
}

// TestParseDescriptorEdgeCases verifies ParseDescriptor handles
// malformed descriptor data defensively.
func TestParseDescriptorEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		raw         []byte
		wantErr     bool
		errContains string
	}{
		{
			name:        "nil input",
			raw:         nil,
			wantErr:     true,
			errContains: "empty descriptor",
		},
		{
			name:        "empty input",
			raw:         []byte{},
			wantErr:     true,
			errContains: "empty descriptor",
		},
		{
			name:    "single newline",
			raw:     []byte("\n"),
			wantErr: false, // Parsed as empty descriptor
		},
		{
			name:    "hs-descriptor version 3",
			raw:     []byte("hs-descriptor 3\n"),
			wantErr: false,
		},
		{
			name:        "hs-descriptor wrong version",
			raw:         []byte("hs-descriptor 2\n"),
			wantErr:     true,
			errContains: "unsupported descriptor version",
		},
		{
			name:    "descriptor-lifetime valid",
			raw:     []byte("hs-descriptor 3\ndescriptor-lifetime 180\n"),
			wantErr: false,
		},
		{
			name:        "descriptor-lifetime non-numeric",
			raw:         []byte("hs-descriptor 3\ndescriptor-lifetime abc\n"),
			wantErr:     true,
			errContains: "invalid descriptor-lifetime",
		},
		{
			name:    "unknown keywords ignored",
			raw:     []byte("hs-descriptor 3\nunknown-keyword value\nanother-unknown\n"),
			wantErr: false,
		},
		{
			name:    "very long lines",
			raw:     []byte("hs-descriptor 3\n" + strings.Repeat("x", 10000) + "\n"),
			wantErr: false,
		},
		{
			name:    "many empty lines",
			raw:     []byte("hs-descriptor 3\n" + strings.Repeat("\n", 1000)),
			wantErr: false,
		},
		{
			name:    "binary data in descriptor",
			raw:     append([]byte("hs-descriptor 3\n"), make([]byte, 256)...),
			wantErr: false, // Parser should handle gracefully
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ParseDescriptor(tc.raw)

			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tc.errContains != "" && !strings.Contains(err.Error(), tc.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.errContains)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if result == nil {
					t.Error("result is nil")
				}
			}
		})
	}
}

// TestDecryptAuthDescriptorEdgeCases verifies DecryptAuthDescriptor
// handles malformed encrypted data defensively.
func TestDecryptAuthDescriptorEdgeCases(t *testing.T) {
	var clientPrivate, servicePub [32]byte
	_, _ = rand.Read(clientPrivate[:])
	_, _ = rand.Read(servicePub[:])

	tests := []struct {
		name        string
		data        []byte
		wantErr     bool
		errContains string
	}{
		{
			name:        "nil data",
			data:        nil,
			wantErr:     true,
			errContains: "too short",
		},
		{
			name:        "empty data",
			data:        []byte{},
			wantErr:     true,
			errContains: "too short",
		},
		{
			name:        "data exactly 39 bytes (below minimum)",
			data:        make([]byte, 39),
			wantErr:     true,
			errContains: "too short",
		},
		{
			name:        "data exactly 40 bytes (minimum, MAC verification fails)",
			data:        make([]byte, 40),
			wantErr:     true,
			errContains: "MAC verification failed",
		},
		{
			name:    "data 41 bytes (minimum with 1 byte ciphertext)",
			data:    make([]byte, 41),
			wantErr: true, // Will fail MAC verification
			errContains: "MAC verification failed",
		},
		{
			name:    "random data sufficient length",
			data:    randomBytesOnion(t, 100),
			wantErr: true, // Will fail MAC verification
			errContains: "MAC verification failed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DecryptAuthDescriptor(tc.data, clientPrivate, servicePub)

			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tc.errContains != "" && !strings.Contains(err.Error(), tc.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.errContains)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

// TestDeriveDescriptorKeysEdgeCases verifies deriveDescriptorKeys
// handles edge cases in key derivation.
func TestDeriveDescriptorKeysEdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		secret []byte
		salt   []byte
		info   string
	}{
		{"empty secret", []byte{}, []byte("salt"), "info"},
		{"empty salt", []byte("secret"), []byte{}, "info"},
		{"empty info", []byte("secret"), []byte("salt"), ""},
		{"all empty", []byte{}, []byte{}, ""},
		{"long secret", make([]byte, 1000), []byte("salt"), "info"},
		{"long salt", []byte("secret"), make([]byte, 1000), "info"},
		{"long info", []byte("secret"), []byte("salt"), strings.Repeat("i", 1000)},
		{"zero bytes secret", make([]byte, 32), make([]byte, 16), "test"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			key, err := deriveDescriptorKeys(tc.secret, tc.salt, tc.info)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(key) != 32 {
				t.Errorf("key length = %d, want 32", len(key))
			}
		})
	}
}

// TestDeriveDescriptorKeysDeterministic verifies that the same
// inputs always produce the same output.
func TestDeriveDescriptorKeysDeterministic(t *testing.T) {
	secret := []byte("test-secret")
	salt := []byte("test-salt")
	info := "test-info"

	key1, err := deriveDescriptorKeys(secret, salt, info)
	if err != nil {
		t.Fatalf("first derivation: %v", err)
	}

	key2, err := deriveDescriptorKeys(secret, salt, info)
	if err != nil {
		t.Fatalf("second derivation: %v", err)
	}

	if len(key1) != len(key2) {
		t.Fatalf("key lengths differ: %d vs %d", len(key1), len(key2))
	}

	for i := range key1 {
		if key1[i] != key2[i] {
			t.Errorf("byte %d differs: %02x vs %02x", i, key1[i], key2[i])
			break
		}
	}
}

// TestDeriveDescriptorKeysDifferentInputs verifies that different
// inputs produce different outputs.
func TestDeriveDescriptorKeysDifferentInputs(t *testing.T) {
	key1, _ := deriveDescriptorKeys([]byte("secret1"), []byte("salt"), "info")
	key2, _ := deriveDescriptorKeys([]byte("secret2"), []byte("salt"), "info")

	match := true
	for i := range key1 {
		if key1[i] != key2[i] {
			match = false
			break
		}
	}
	if match {
		t.Error("different secrets produced identical keys")
	}
}

// TestParseDecryptedLayerEdgeCases verifies parseDecryptedLayer
// handles malformed decrypted data.
func TestParseDecryptedLayerEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{"nil data", nil, false},
		{"empty data", []byte{}, false},
		{"single newline", []byte("\n"), false},
		{"unknown keywords", []byte("unknown-keyword value\n"), false},
		{"introduction-point line only", []byte("introduction-point AAAA\n"), false},
		{"multiple intro points", []byte("introduction-point A\nintroduction-point B\n"), false},
		{"very long line", []byte(strings.Repeat("x", 100000) + "\n"), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("parseDecryptedLayer panicked: %v", r)
				}
			}()

			result, err := parseDecryptedLayer(tc.data)
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if result == nil {
					t.Error("result is nil")
				}
			}
		})
	}
}

// TestDeriveAuthKeysEdgeCases verifies deriveAuthKeys handles
// edge cases in key derivation for client authorization.
func TestDeriveAuthKeysEdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		secret []byte
		salt   []byte
		info   []byte
		length int
	}{
		{"standard 64 bytes", []byte("secret"), []byte("salt"), []byte("info"), 64},
		{"zero length", []byte("secret"), []byte("salt"), []byte("info"), 0},
		{"single byte", []byte("secret"), []byte("salt"), []byte("info"), 1},
		{"256 bytes", []byte("secret"), []byte("salt"), []byte("info"), 256},
		{"empty secret", []byte{}, []byte("salt"), []byte("info"), 32},
		{"empty salt", []byte("secret"), []byte{}, []byte("info"), 32},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			keys, err := deriveAuthKeys(tc.secret, tc.salt, tc.info, tc.length)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(keys) != tc.length {
				t.Errorf("key length = %d, want %d", len(keys), tc.length)
			}
		})
	}
}

// randomBytesOnion generates random bytes for testing.
func randomBytesOnion(t *testing.T, n int) []byte {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("failed to generate random bytes: %v", err)
	}
	return b
}
