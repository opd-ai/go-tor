package security

import (
	"math"
	"testing"
	"time"
)

// TestIntegerOverflowTimeConversion tests for CVE-2025-XXXX
// Integer overflow vulnerabilities in time conversions
func TestIntegerOverflowTimeConversion(t *testing.T) {
	tests := []struct {
		name      string
		timestamp int64
		wantError bool
	}{
		{
			name:      "normal timestamp",
			timestamp: time.Now().Unix(),
			wantError: false,
		},
		{
			name:      "negative timestamp",
			timestamp: -1,
			wantError: true,
		},
		{
			name:      "max int64 timestamp",
			timestamp: math.MaxInt64,
			wantError: false, // int64 max fits in uint64
		},
		{
			name:      "zero timestamp",
			timestamp: 0,
			wantError: false,
		},
		{
			name:      "Y2038 timestamp",
			timestamp: 2147483648, // After 2038 overflow on 32-bit
			wantError: false,      // Should handle properly
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test uint64 conversion
			result, err := safeInt64ToUint64(tt.timestamp)
			if tt.wantError && err == nil {
				t.Errorf("expected error for timestamp %d, got none", tt.timestamp)
			}
			if !tt.wantError && err != nil {
				t.Errorf("unexpected error for timestamp %d: %v", tt.timestamp, err)
			}
			if !tt.wantError && result != uint64(tt.timestamp) {
				t.Errorf("conversion mismatch: got %d, want %d", result, tt.timestamp)
			}

			// Test uint32 conversion
			result32, err := safeInt64ToUint32(tt.timestamp)
			if tt.timestamp > math.MaxUint32 {
				if err == nil {
					t.Errorf("expected error for timestamp %d > MaxUint32, got none", tt.timestamp)
				}
			} else if !tt.wantError && err != nil {
				t.Errorf("unexpected error for timestamp %d: %v", tt.timestamp, err)
			} else if !tt.wantError && err == nil && result32 != uint32(tt.timestamp) {
				t.Errorf("conversion mismatch: got %d, want %d", result32, uint32(tt.timestamp))
			}
		})
	}
}

// TestLengthOverflow tests for SEC-005
// Integer overflow in length calculations
func TestLengthOverflow(t *testing.T) {
	tests := []struct {
		name      string
		length    int
		wantError bool
	}{
		{
			name:      "normal length",
			length:    1024,
			wantError: false,
		},
		{
			name:      "max uint16",
			length:    65535,
			wantError: false,
		},
		{
			name:      "overflow uint16",
			length:    65536,
			wantError: true,
		},
		{
			name:      "negative length",
			length:    -1,
			wantError: true,
		},
		{
			name:      "zero length",
			length:    0,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := safeIntToUint16(tt.length)
			if tt.wantError && err == nil {
				t.Errorf("expected error for length %d, got none", tt.length)
			}
			if !tt.wantError && err != nil {
				t.Errorf("unexpected error for length %d: %v", tt.length, err)
			}
			if !tt.wantError && int(result) != tt.length {
				t.Errorf("conversion mismatch: got %d, want %d", result, tt.length)
			}
		})
	}
}

// TestWeakTLSCipherSuites tests for CVE-2025-YYYY
// Verifies that weak TLS cipher suites are not used
func TestWeakTLSCipherSuites(t *testing.T) {
	// These cipher suites should NOT be used (CBC mode, weak)
	weakCiphers := []uint16{
		0x002f, // TLS_RSA_WITH_AES_128_CBC_SHA
		0x0035, // TLS_RSA_WITH_AES_256_CBC_SHA
		0xc013, // TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA
		0xc014, // TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA
		0xc009, // TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA
		0xc00a, // TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA
	}

	// These cipher suites SHOULD be used (AEAD with forward secrecy)
	secureCiphers := []uint16{
		TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
		TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
	}

	// Create a recommended TLS config
	config := getRecommendedTLSConfig()

	// Verify no weak ciphers are present
	for _, weakCipher := range weakCiphers {
		for _, configCipher := range config.CipherSuites {
			if configCipher == weakCipher {
				t.Errorf("Weak cipher suite found in config: 0x%04x", weakCipher)
			}
		}
	}

	// Verify secure ciphers are present
	foundSecure := make(map[uint16]bool)
	for _, configCipher := range config.CipherSuites {
		for _, secureCipher := range secureCiphers {
			if configCipher == secureCipher {
				foundSecure[secureCipher] = true
			}
		}
	}

	if len(foundSecure) == 0 {
		t.Error("No secure cipher suites found in config")
	}

	// Verify minimum TLS version
	if config.MinVersion < VersionTLS12 {
		t.Errorf("TLS version too low: got 0x%04x, want >= 0x%04x (TLS 1.2)", config.MinVersion, VersionTLS12)
	}
}

// TestMemoryZeroingForSensitiveData tests for SEC-006
// Verifies that sensitive data is properly zeroed
func TestMemoryZeroingForSensitiveData(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "circuit key",
			data: make([]byte, 32),
		},
		{
			name: "session key",
			data: make([]byte, 16),
		},
		{
			name: "auth cookie",
			data: make([]byte, 32),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Fill with non-zero data
			for i := range tt.data {
				tt.data[i] = byte(i)
			}

			// Zero the data
			zeroSensitiveData(tt.data)

			// Verify all bytes are zero
			for i, b := range tt.data {
				if b != 0 {
					t.Errorf("byte %d not zeroed: got %d", i, b)
				}
			}
		})
	}
}

// TestConstantTimeComparison tests for CVE-2025-ZZZZ
// Verifies use of constant-time operations for cryptographic comparisons
func TestConstantTimeComparison(t *testing.T) {
	tests := []struct {
		name   string
		a      []byte
		b      []byte
		wantEq bool
	}{
		{
			name:   "equal slices",
			a:      []byte{1, 2, 3, 4},
			b:      []byte{1, 2, 3, 4},
			wantEq: true,
		},
		{
			name:   "different slices",
			a:      []byte{1, 2, 3, 4},
			b:      []byte{1, 2, 3, 5},
			wantEq: false,
		},
		{
			name:   "different lengths",
			a:      []byte{1, 2, 3},
			b:      []byte{1, 2, 3, 4},
			wantEq: false,
		},
		{
			name:   "empty slices",
			a:      []byte{},
			b:      []byte{},
			wantEq: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := constantTimeCompare(tt.a, tt.b)
			if result != tt.wantEq {
				t.Errorf("comparison mismatch: got %v, want %v", result, tt.wantEq)
			}
		})
	}
}

// TestInputValidation tests for SEC-001
// Comprehensive input validation tests
func TestInputValidation(t *testing.T) {
	tests := []struct {
		name      string
		input     []byte
		validator func([]byte) error
		wantError bool
	}{
		{
			name:      "valid cell",
			input:     make([]byte, 514), // Valid fixed cell
			validator: validateCellInput,
			wantError: false,
		},
		{
			name:      "too short cell",
			input:     make([]byte, 4),
			validator: validateCellInput,
			wantError: true,
		},
		{
			name:      "too long cell",
			input:     make([]byte, 70000),
			validator: validateCellInput,
			wantError: true,
		},
		{
			name:      "nil input",
			input:     nil,
			validator: validateCellInput,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.validator(tt.input)
			if tt.wantError && err == nil {
				t.Error("expected error, got none")
			}
			if !tt.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestRateLimiting tests for SEC-003
// Verifies rate limiting is enforced
func TestRateLimiting(t *testing.T) {
	limiter := newRateLimiter(10, time.Second) // 10 per second

	// Should allow up to 10 operations
	for i := 0; i < 10; i++ {
		if !limiter.Allow() {
			t.Errorf("operation %d should be allowed", i)
		}
	}

	// 11th operation should be rate limited
	if limiter.Allow() {
		t.Error("operation should be rate limited")
	}

	// After waiting, should allow again
	time.Sleep(1100 * time.Millisecond)
	if !limiter.Allow() {
		t.Error("operation should be allowed after wait")
	}
}

// TestResourceLimits tests for MED-004
// Verifies resource limits are enforced
func TestResourceLimits(t *testing.T) {
	tests := []struct {
		name          string
		resourceType  string
		limit         int
		attemptCreate int
		wantError     bool
	}{
		{
			name:          "within circuit limit",
			resourceType:  "circuit",
			limit:         10,
			attemptCreate: 10,
			wantError:     false,
		},
		{
			name:          "exceed circuit limit",
			resourceType:  "circuit",
			limit:         10,
			attemptCreate: 11,
			wantError:     true,
		},
		{
			name:          "within stream limit",
			resourceType:  "stream",
			limit:         100,
			attemptCreate: 100,
			wantError:     false,
		},
		{
			name:          "exceed stream limit",
			resourceType:  "stream",
			limit:         100,
			attemptCreate: 101,
			wantError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := newResourceManager(tt.limit)
			var err error
			for i := 0; i < tt.attemptCreate; i++ {
				err = manager.Allocate(tt.resourceType)
				if err != nil {
					break
				}
			}
			if tt.wantError && err == nil {
				t.Error("expected resource limit error, got none")
			}
			if !tt.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// Benchmark tests for performance validation

func BenchmarkSafeTimeConversion(b *testing.B) {
	now := time.Now()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = safeInt64ToUint64(now.Unix())
	}
}

func BenchmarkConstantTimeCompare(b *testing.B) {
	a := make([]byte, 32)
	b2 := make([]byte, 32)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		constantTimeCompare(a, b2)
	}
}

func BenchmarkZeroSensitiveData(b *testing.B) {
	data := make([]byte, 32)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		zeroSensitiveData(data)
	}
}
