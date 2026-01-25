package connection

import (
	"testing"

	"github.com/opd-ai/go-tor/pkg/logger"
)

// TestConnectionExpectedIdentityGetters tests the new getter methods for expected identity
func TestConnectionExpectedIdentityGetters(t *testing.T) {
	testIdentity := []byte("01234567890123456789012345678901") // 32 bytes
	testFingerprint := "0011223344556677889900AABBCCDDEEFF00112233"

	tests := []struct {
		name            string
		setupConfig     func() *Config
		wantIdentity    []byte
		wantFingerprint string
	}{
		{
			name: "both values set",
			setupConfig: func() *Config {
				cfg := DefaultConfig("127.0.0.1:9001")
				cfg.ExpectedIdentity = testIdentity
				cfg.ExpectedFingerprint = testFingerprint
				return cfg
			},
			wantIdentity:    testIdentity,
			wantFingerprint: testFingerprint,
		},
		{
			name: "only identity set",
			setupConfig: func() *Config {
				cfg := DefaultConfig("127.0.0.1:9001")
				cfg.ExpectedIdentity = testIdentity
				return cfg
			},
			wantIdentity:    testIdentity,
			wantFingerprint: "",
		},
		{
			name: "only fingerprint set",
			setupConfig: func() *Config {
				cfg := DefaultConfig("127.0.0.1:9001")
				cfg.ExpectedFingerprint = testFingerprint
				return cfg
			},
			wantIdentity:    nil,
			wantFingerprint: testFingerprint,
		},
		{
			name: "neither value set (default)",
			setupConfig: func() *Config {
				return DefaultConfig("127.0.0.1:9001")
			},
			wantIdentity:    nil,
			wantFingerprint: "",
		},
		{
			name: "empty identity and fingerprint",
			setupConfig: func() *Config {
				cfg := DefaultConfig("127.0.0.1:9001")
				cfg.ExpectedIdentity = []byte{}
				cfg.ExpectedFingerprint = ""
				return cfg
			},
			wantIdentity:    []byte{},
			wantFingerprint: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.setupConfig()
			conn := New(cfg, logger.NewDefault())

			gotIdentity := conn.ExpectedIdentity()
			gotFingerprint := conn.ExpectedFingerprint()

			// Compare identity bytes
			if !bytesEqual(gotIdentity, tt.wantIdentity) {
				t.Errorf("ExpectedIdentity() = %v, want %v", gotIdentity, tt.wantIdentity)
			}

			// Compare fingerprint strings
			if gotFingerprint != tt.wantFingerprint {
				t.Errorf("ExpectedFingerprint() = %v, want %v", gotFingerprint, tt.wantFingerprint)
			}
		})
	}
}

// TestConnectionStoresExpectedValues tests that connection stores the expected values from config
func TestConnectionStoresExpectedValues(t *testing.T) {
	testIdentity := []byte("test-identity-32-bytes-long!!!!!")
	testFingerprint := "ABCDEF1234567890"

	cfg := DefaultConfig("192.168.1.1:443")
	cfg.ExpectedIdentity = testIdentity
	cfg.ExpectedFingerprint = testFingerprint

	conn := New(cfg, nil) // nil logger to test default logger path

	// Verify values are stored
	if !bytesEqual(conn.ExpectedIdentity(), testIdentity) {
		t.Errorf("Connection did not store ExpectedIdentity correctly")
	}

	if conn.ExpectedFingerprint() != testFingerprint {
		t.Errorf("Connection did not store ExpectedFingerprint correctly")
	}
}

// bytesEqual compares two byte slices for equality, handling nil cases
func bytesEqual(a, b []byte) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
