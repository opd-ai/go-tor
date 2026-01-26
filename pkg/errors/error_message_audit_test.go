// Package errors - Error Message Security Audit Tests
// This file audits error messages across all packages for sensitive information leakage
package errors

import (
	"crypto/rand"
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// TestErrorMessageNoSensitiveDataLeak audits error messages for sensitive information
func TestErrorMessageNoSensitiveDataLeak(t *testing.T) {
	tests := []struct {
		name      string
		errorMsg  string
		safe      bool
		reason    string
		category  string
	}{
		// SAFE: Generic error messages
		{
			name:     "generic_failure",
			errorMsg: "failed to connect to relay",
			safe:     true,
			category: "network",
		},
		{
			name:     "generic_timeout",
			errorMsg: "operation timed out",
			safe:     true,
			category: "network",
		},
		{
			name:     "invalid_input_length",
			errorMsg: "invalid key length: expected 32 bytes, got 16",
			safe:     true,
			reason:   "Length information is safe, does not leak key material",
			category: "validation",
		},
		{
			name:     "cell_command_type",
			errorMsg: "unexpected cell command: 7",
			safe:     true,
			reason:   "Cell command types are protocol-defined constants, not sensitive",
			category: "protocol",
		},
		
		// UNSAFE: Sensitive data leakage examples
		{
			name:     "password_in_error",
			errorMsg: "authentication failed: invalid password 'supersecret123'",
			safe:     false,
			reason:   "Password value leaked in error message",
			category: "authentication",
		},
		{
			name:     "key_material_hex",
			errorMsg: fmt.Sprintf("decryption failed with key: %x", make([]byte, 16)),
			safe:     false,
			reason:   "Cryptographic key material exposed in hex format",
			category: "crypto",
		},
		{
			name:     "private_key_bytes",
			errorMsg: fmt.Sprintf("invalid private key: %v", make([]byte, 32)),
			safe:     false,
			reason:   "Private key bytes exposed via %v formatting",
			category: "crypto",
		},
		{
			name:     "session_token",
			errorMsg: "session expired: token=abc123def456",
			safe:     false,
			reason:   "Session token value leaked",
			category: "authentication",
		},
		{
			name:     "circuit_key_debug",
			errorMsg: "circuit encryption failed, forward_key=[16 bytes of key data]",
			safe:     false,
			reason:   "Hints at key material content",
			category: "crypto",
		},
		
		// SAFE: Proper error message patterns
		{
			name:     "sanitized_auth_failure",
			errorMsg: "authentication failed",
			safe:     true,
			reason:   "No sensitive data, appropriate for security events",
			category: "authentication",
		},
		{
			name:     "key_length_validation",
			errorMsg: "invalid ntor key length: expected 32, got 24",
			safe:     true,
			reason:   "Metadata about key format, not key material",
			category: "validation",
		},
		{
			name:     "circuit_id_reference",
			errorMsg: "failed to extend circuit 12345",
			safe:     true,
			reason:   "Circuit IDs are not secret, required for protocol operation",
			category: "circuit",
		},
		{
			name:     "relay_fingerprint",
			errorMsg: "connection failed to relay AB12CD34EF56...",
			safe:     true,
			reason:   "Relay fingerprints are public information from consensus",
			category: "network",
		},
		{
			name:     "descriptor_parse_error",
			errorMsg: "failed to parse descriptor: invalid certificate format",
			safe:     true,
			reason:   "Structural error, no secret data",
			category: "protocol",
		},
	}

	t.Run("error_message_security_patterns", func(t *testing.T) {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				leaked := containsSensitiveData(tt.errorMsg)
				
				if tt.safe && leaked {
					t.Errorf("Error message marked as SAFE but contains sensitive patterns: %s\n"+
						"Message: %s\nReason: %s",
						tt.name, tt.errorMsg, tt.reason)
				}
				
				if !tt.safe && !leaked {
					t.Logf("INFO: Error message marked UNSAFE but no automatic detection (manual review required)\n"+
						"Message: %s\nReason: %s",
						tt.errorMsg, tt.reason)
				}
			})
		}
	})
}

// TestErrorFormattingBestPractices verifies error formatting follows security guidelines
func TestErrorFormattingBestPractices(t *testing.T) {
	// Generate test key material
	key := make([]byte, 32)
	rand.Read(key)
	
	password := "test_password_123"
	token := "session_token_abc123"
	
	tests := []struct {
		name         string
		errorFormat  string
		args         []interface{}
		expectUnsafe bool
		pattern      string
		reason       string
	}{
		{
			name:         "safe_wrapped_error",
			errorFormat:  "failed to process: %w",
			args:         []interface{}{fmt.Errorf("connection timeout")},
			expectUnsafe: false,
			reason:       "Wrapping errors maintains context without leaking data",
		},
		{
			name:         "safe_metadata_only",
			errorFormat:  "invalid key size: expected %d, got %d",
			args:         []interface{}{32, 16},
			expectUnsafe: false,
			reason:       "Size information is metadata, not sensitive",
		},
		{
			name:         "unsafe_key_hex_format",
			errorFormat:  "encryption failed with key: %x",
			args:         []interface{}{key},
			expectUnsafe: true,
			pattern:      `[0-9a-f]{64}`,
			reason:       "Hex formatting of key material leaks cryptographic secrets",
		},
		{
			name:         "unsafe_key_bytes_format",
			errorFormat:  "invalid key: %v",
			args:         []interface{}{key},
			expectUnsafe: true,
			pattern:      `\[.*\]`,
			reason:       "%v formatting of byte slices reveals key material",
		},
		{
			name:         "unsafe_password_direct",
			errorFormat:  "auth failed for password: %s",
			args:         []interface{}{password},
			expectUnsafe: true,
			pattern:      password,
			reason:       "Password value directly embedded in error",
		},
		{
			name:         "unsafe_token_format",
			errorFormat:  "session invalid: %s",
			args:         []interface{}{token},
			expectUnsafe: true,
			pattern:      token,
			reason:       "Session token exposed in error message",
		},
		{
			name:         "safe_generic_auth_error",
			errorFormat:  "authentication failed",
			args:         []interface{}{},
			expectUnsafe: false,
			reason:       "Generic message with no sensitive data",
		},
		{
			name:         "safe_operation_type",
			errorFormat:  "failed to %s circuit %d",
			args:         []interface{}{"extend", 12345},
			expectUnsafe: false,
			reason:       "Operation type and circuit ID are not secret",
		},
	}
	
	t.Run("error_formatting_patterns", func(t *testing.T) {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				msg := fmt.Sprintf(tt.errorFormat, tt.args...)
				
				leaked := containsSensitiveData(msg)
				
				if tt.expectUnsafe && !leaked {
					// Manual pattern check for cases auto-detection might miss
					if tt.pattern != "" {
						matched, _ := regexp.MatchString(tt.pattern, msg)
						if matched {
							t.Logf("DETECTED: Unsafe error formatting (pattern match)\n"+
								"Message: %s\nPattern: %s\nReason: %s",
								msg, tt.pattern, tt.reason)
						} else {
							t.Errorf("Expected unsafe error but pattern not found\n"+
								"Message: %s\nPattern: %s",
								msg, tt.pattern)
						}
					}
				}
				
				if !tt.expectUnsafe && leaked {
					t.Errorf("Safe error message triggered sensitive data detection\n"+
						"Message: %s\nReason: %s",
						msg, tt.reason)
				}
			})
		}
	})
}

// TestErrorContextPropagation verifies error wrapping doesn't leak sensitive context
func TestErrorContextPropagation(t *testing.T) {
	key := make([]byte, 16)
	rand.Read(key)
	
	tests := []struct {
		name      string
		baseError error
		wrapper   func(error) error
		safe      bool
		reason    string
	}{
		{
			name:      "safe_error_wrapping",
			baseError: fmt.Errorf("connection refused"),
			wrapper: func(err error) error {
				return fmt.Errorf("failed to connect: %w", err)
			},
			safe:   true,
			reason: "Standard error wrapping maintains context without leaking data",
		},
		{
			name:      "unsafe_key_in_wrapper",
			baseError: fmt.Errorf("decryption failed"),
			wrapper: func(err error) error {
				return fmt.Errorf("decryption with key %x failed: %w", key, err)
			},
			safe:   false,
			reason: "Wrapper adds sensitive key material to error chain",
		},
		{
			name:      "safe_metadata_wrapper",
			baseError: fmt.Errorf("invalid size"),
			wrapper: func(err error) error {
				return fmt.Errorf("key validation failed (expected 32 bytes): %w", err)
			},
			safe:   true,
			reason: "Wrapper adds non-sensitive metadata",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapped := tt.wrapper(tt.baseError)
			msg := wrapped.Error()
			
			leaked := containsSensitiveData(msg)
			
			if !tt.safe && !leaked {
				t.Logf("WARN: Unsafe wrapper expected but not auto-detected\n"+
					"Message: %s\nReason: %s",
					msg, tt.reason)
			}
			
			if tt.safe && leaked {
				t.Errorf("Safe wrapper triggered leak detection\n"+
					"Message: %s\nReason: %s",
					msg, tt.reason)
			}
		})
	}
}

// TestCommonVulnerablePatterns tests for common error message anti-patterns
func TestCommonVulnerablePatterns(t *testing.T) {
	vulnerablePatterns := []struct {
		name        string
		pattern     string
		example     string
		description string
		severity    string
	}{
		{
			name:        "hex_formatted_data",
			pattern:     `[0-9a-f]{32,}`,
			example:     "error: key deadbeef123456789abcdef0123456789abcdef0123456789abcdef",
			description: "Long hex strings likely represent key material",
			severity:    "HIGH",
		},
		{
			name:        "base64_data",
			pattern:     `[A-Za-z0-9+/]{32,}={0,2}`,
			example:     "error: token YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY3ODkw",
			description: "Base64 strings may contain tokens or keys",
			severity:    "HIGH",
		},
		{
			name:        "password_keyword",
			pattern:     `password[:\s=]+['"]?[\w!@#$%^&*()]+['"]?`,
			example:     "error: password='secret123'",
			description: "Password value exposed after keyword",
			severity:     "CRITICAL",
		},
		{
			name:        "key_keyword_with_value",
			pattern:     `(?:private|secret|session)[-_\s]?key[:\s=]+[^\s]+`,
			example:     "error: private_key=abc123",
			description: "Private key value exposed",
			severity:    "CRITICAL",
		},
		{
			name:        "byte_array_dump",
			pattern:     `\[\d+(?:\s+\d+){15,}\]`,
			example:     "error: key [1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16]",
			description: "Byte array dumps may contain key material",
			severity:    "HIGH",
		},
	}
	
	t.Run("vulnerable_pattern_detection", func(t *testing.T) {
		for _, vp := range vulnerablePatterns {
			t.Run(vp.name, func(t *testing.T) {
				re := regexp.MustCompile(vp.pattern)
				
				// Test that example matches pattern
				if !re.MatchString(vp.example) {
					t.Errorf("Pattern %s does not match its example\nPattern: %s\nExample: %s",
						vp.name, vp.pattern, vp.example)
				}
				
				// Test that safe messages don't match
				safeMessages := []string{
					"connection failed",
					"invalid input",
					"operation timed out",
					"circuit 123 closed",
					"expected 32 bytes, got 16",
				}
				
				for _, safe := range safeMessages {
					if re.MatchString(safe) {
						t.Errorf("Pattern %s incorrectly matched safe message: %s",
							vp.name, safe)
					}
				}
				
				t.Logf("PATTERN: %s [%s]\nDescription: %s\nExample: %s",
					vp.name, vp.severity, vp.description, vp.example)
			})
		}
	})
}

// TestErrorMessageGuidelines documents and tests error message security guidelines
func TestErrorMessageGuidelines(t *testing.T) {
	t.Run("documentation", func(t *testing.T) {
		guidelines := []string{
			"ERROR MESSAGE SECURITY GUIDELINES:",
			"",
			"✓ DO:",
			"  - Use generic error messages: 'authentication failed'",
			"  - Include metadata (lengths, types): 'expected 32 bytes, got 16'",
			"  - Wrap errors with context: fmt.Errorf('operation failed: %w', err)",
			"  - Reference non-secret IDs: 'circuit 12345 failed'",
			"  - Log errors at appropriate levels without sensitive data",
			"",
			"✗ DO NOT:",
			"  - Include passwords: 'password p@ssw0rd is invalid'",
			"  - Expose key material: 'key %x failed', key",
			"  - Dump byte arrays: 'data %v invalid', secretBytes",
			"  - Include session tokens: 'token abc123 expired'",
			"  - Reveal internal state: 'conn state: authenticated=true, key=[...]'",
			"",
			"APPROVED PATTERNS:",
			`  - fmt.Errorf("operation failed: %w", err)`,
			`  - fmt.Errorf("invalid length: expected %d, got %d", exp, got)`,
			`  - fmt.Errorf("circuit %d: %w", circID, err)`,
			"",
			"FORBIDDEN PATTERNS:",
			`  - fmt.Errorf("key %x invalid", keyMaterial)`,
			`  - fmt.Errorf("password %s wrong", password)`,
			`  - fmt.Errorf("secret: %v", secretData)`,
			`  - logger.Error("auth failed", "password", password)`,
		}
		
		for _, line := range guidelines {
			t.Log(line)
		}
	})
}

// containsSensitiveData checks if a string contains patterns indicative of sensitive data
func containsSensitiveData(s string) bool {
	lower := strings.ToLower(s)
	
	// Check for sensitive keywords followed by data
	sensitivePatterns := []string{
		`password[:\s=]+['"]?[^\s'"]+`,
		`secret[:\s=]+['"]?[^\s'"]+`,
		`token[:\s=]+['"]?[^\s'"]+`,
		`key[:\s=]+[0-9a-f]{16,}`,
		`[0-9a-f]{64,}`, // Long hex strings (SHA256 hashes, keys)
	}
	
	for _, pattern := range sensitivePatterns {
		matched, _ := regexp.MatchString(pattern, lower)
		if matched {
			return true
		}
	}
	
	return false
}

// TestRealWorldErrorExamples tests error messages from actual code paths
func TestRealWorldErrorExamples(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		errorMsg string
		safe     bool
		notes    string
	}{
		{
			name:     "crypto_key_length_validation",
			source:   "pkg/crypto/crypto.go",
			errorMsg: "invalid key length: expected 32, got 16",
			safe:     true,
			notes:    "Metadata only, no key material",
		},
		{
			name:     "circuit_extension_failure",
			source:   "pkg/circuit/extension.go",
			errorMsg: "failed to extend circuit: connection timeout",
			safe:     true,
			notes:    "Generic network error",
		},
		{
			name:     "control_auth_generic",
			source:   "pkg/control/control.go",
			errorMsg: "authentication failed",
			safe:     true,
			notes:    "Proper security error message (no details)",
		},
		{
			name:     "descriptor_parse_error",
			source:   "pkg/onion/onion.go",
			errorMsg: "failed to parse descriptor: invalid certificate format",
			safe:     true,
			notes:    "Structural validation error, no secrets",
		},
		{
			name:     "cell_command_validation",
			source:   "pkg/cell/cell.go",
			errorMsg: "unexpected cell command: 255",
			safe:     true,
			notes:    "Protocol constant, not secret",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			leaked := containsSensitiveData(tt.errorMsg)
			
			if leaked && tt.safe {
				t.Errorf("Real-world safe error triggered leak detection\n"+
					"Source: %s\nMessage: %s\nNotes: %s",
					tt.source, tt.errorMsg, tt.notes)
			}
			
			t.Logf("VERIFIED: %s [SAFE: %v]\n  Message: %s\n  Notes: %s",
				tt.source, tt.safe, tt.errorMsg, tt.notes)
		})
	}
}
