package circuit

import (
	"testing"
)

func TestIsolationLevel_String(t *testing.T) {
	tests := []struct {
		level    IsolationLevel
		expected string
	}{
		{IsolationNone, "none"},
		{IsolationDestination, "destination"},
		{IsolationCredential, "credential"},
		{IsolationPort, "port"},
		{IsolationSession, "session"},
		{IsolationLevel(99), "unknown(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.level.String(); got != tt.expected {
				t.Errorf("IsolationLevel.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestParseIsolationLevel(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  IsolationLevel
		expectErr bool
	}{
		{"none", "none", IsolationNone, false},
		{"destination", "destination", IsolationDestination, false},
		{"credential", "credential", IsolationCredential, false},
		{"credentials", "credentials", IsolationCredential, false},
		{"port", "port", IsolationPort, false},
		{"session", "session", IsolationSession, false},
		{"case insensitive", "DESTINATION", IsolationDestination, false},
		{"invalid", "invalid", IsolationNone, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseIsolationLevel(tt.input)
			if tt.expectErr {
				if err == nil {
					t.Errorf("ParseIsolationLevel() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("ParseIsolationLevel() unexpected error: %v", err)
				return
			}
			if got != tt.expected {
				t.Errorf("ParseIsolationLevel() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNewIsolationKey(t *testing.T) {
	key := NewIsolationKey(IsolationDestination)
	if key == nil {
		t.Fatal("NewIsolationKey() returned nil")
	}
	if key.Level != IsolationDestination {
		t.Errorf("NewIsolationKey().Level = %v, want %v", key.Level, IsolationDestination)
	}
}

func TestIsolationKey_WithDestination(t *testing.T) {
	key := NewIsolationKey(IsolationDestination).
		WithDestination("example.com:80")

	if key.Destination != "example.com:80" {
		t.Errorf("WithDestination() set Destination = %v, want %v", key.Destination, "example.com:80")
	}
}

func TestIsolationKey_WithCredentials(t *testing.T) {
	key := NewIsolationKey(IsolationCredential).
		WithCredentials("user123")

	if key.Credentials == "" {
		t.Error("WithCredentials() did not set Credentials")
	}
	if key.Credentials == "user123" {
		t.Error("WithCredentials() did not hash the username")
	}
	// Verify it's a valid hex string (SHA256 produces 64 hex chars)
	if len(key.Credentials) != 64 {
		t.Errorf("WithCredentials() hash length = %d, want 64", len(key.Credentials))
	}
}

func TestIsolationKey_WithSourcePort(t *testing.T) {
	key := NewIsolationKey(IsolationPort).
		WithSourcePort(12345)

	if key.SourcePort != 12345 {
		t.Errorf("WithSourcePort() set SourcePort = %v, want %v", key.SourcePort, 12345)
	}
}

func TestIsolationKey_WithSessionToken(t *testing.T) {
	key := NewIsolationKey(IsolationSession).
		WithSessionToken("session-abc-123")

	if key.SessionToken == "" {
		t.Error("WithSessionToken() did not set SessionToken")
	}
	if key.SessionToken == "session-abc-123" {
		t.Error("WithSessionToken() did not hash the token")
	}
	// Verify it's a valid hex string (SHA256 produces 64 hex chars)
	if len(key.SessionToken) != 64 {
		t.Errorf("WithSessionToken() hash length = %d, want 64", len(key.SessionToken))
	}
}

func TestIsolationKey_String(t *testing.T) {
	tests := []struct {
		name string
		key  *IsolationKey
		want string
	}{
		{
			name: "nil key",
			key:  nil,
			want: "none",
		},
		{
			name: "none level",
			key:  NewIsolationKey(IsolationNone),
			want: "none",
		},
		{
			name: "destination",
			key:  NewIsolationKey(IsolationDestination).WithDestination("example.com:80"),
			want: "level=destination,dest=example.com:80",
		},
		{
			name: "credential",
			key:  NewIsolationKey(IsolationCredential).WithCredentials("user"),
			want: "level=credential,creds=", // Will have hash prefix
		},
		{
			name: "port",
			key:  NewIsolationKey(IsolationPort).WithSourcePort(12345),
			want: "level=port,port=12345",
		},
		{
			name: "session",
			key:  NewIsolationKey(IsolationSession).WithSessionToken("token"),
			want: "level=session,session=", // Will have hash prefix
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.key.String()
			// For credential and session, just check prefix
			if tt.key != nil && (tt.key.Level == IsolationCredential || tt.key.Level == IsolationSession) {
				if len(got) < len(tt.want) {
					t.Errorf("IsolationKey.String() = %v, want prefix %v", got, tt.want)
				}
				if got[:len(tt.want)] != tt.want {
					t.Errorf("IsolationKey.String() = %v, want prefix %v", got, tt.want)
				}
			} else if got != tt.want {
				t.Errorf("IsolationKey.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsolationKey_Key(t *testing.T) {
	tests := []struct {
		name string
		key  *IsolationKey
		want string
	}{
		{
			name: "nil key",
			key:  nil,
			want: "",
		},
		{
			name: "none level",
			key:  NewIsolationKey(IsolationNone),
			want: "",
		},
		{
			name: "destination",
			key:  NewIsolationKey(IsolationDestination).WithDestination("example.com:80"),
			want: "dest:example.com:80",
		},
		{
			name: "port",
			key:  NewIsolationKey(IsolationPort).WithSourcePort(12345),
			want: "port:12345",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.key.Key()
			if got != tt.want {
				t.Errorf("IsolationKey.Key() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsolationKey_Equals(t *testing.T) {
	tests := []struct {
		name     string
		key1     *IsolationKey
		key2     *IsolationKey
		expected bool
	}{
		{
			name:     "both nil",
			key1:     nil,
			key2:     nil,
			expected: true,
		},
		{
			name:     "one nil",
			key1:     NewIsolationKey(IsolationNone),
			key2:     nil,
			expected: false,
		},
		{
			name:     "both none",
			key1:     NewIsolationKey(IsolationNone),
			key2:     NewIsolationKey(IsolationNone),
			expected: true,
		},
		{
			name:     "different levels",
			key1:     NewIsolationKey(IsolationDestination).WithDestination("example.com:80"),
			key2:     NewIsolationKey(IsolationPort).WithSourcePort(12345),
			expected: false,
		},
		{
			name:     "same destination",
			key1:     NewIsolationKey(IsolationDestination).WithDestination("example.com:80"),
			key2:     NewIsolationKey(IsolationDestination).WithDestination("example.com:80"),
			expected: true,
		},
		{
			name:     "different destination",
			key1:     NewIsolationKey(IsolationDestination).WithDestination("example.com:80"),
			key2:     NewIsolationKey(IsolationDestination).WithDestination("example.com:443"),
			expected: false,
		},
		{
			name:     "same credentials",
			key1:     NewIsolationKey(IsolationCredential).WithCredentials("user"),
			key2:     NewIsolationKey(IsolationCredential).WithCredentials("user"),
			expected: true,
		},
		{
			name:     "different credentials",
			key1:     NewIsolationKey(IsolationCredential).WithCredentials("user1"),
			key2:     NewIsolationKey(IsolationCredential).WithCredentials("user2"),
			expected: false,
		},
		{
			name:     "same port",
			key1:     NewIsolationKey(IsolationPort).WithSourcePort(12345),
			key2:     NewIsolationKey(IsolationPort).WithSourcePort(12345),
			expected: true,
		},
		{
			name:     "different port",
			key1:     NewIsolationKey(IsolationPort).WithSourcePort(12345),
			key2:     NewIsolationKey(IsolationPort).WithSourcePort(54321),
			expected: false,
		},
		{
			name:     "same session",
			key1:     NewIsolationKey(IsolationSession).WithSessionToken("token"),
			key2:     NewIsolationKey(IsolationSession).WithSessionToken("token"),
			expected: true,
		},
		{
			name:     "different session",
			key1:     NewIsolationKey(IsolationSession).WithSessionToken("token1"),
			key2:     NewIsolationKey(IsolationSession).WithSessionToken("token2"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.key1.Equals(tt.key2); got != tt.expected {
				t.Errorf("IsolationKey.Equals() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestIsolationKey_Validate(t *testing.T) {
	tests := []struct {
		name      string
		key       *IsolationKey
		expectErr bool
	}{
		{
			name:      "nil key",
			key:       nil,
			expectErr: true,
		},
		{
			name:      "none level",
			key:       NewIsolationKey(IsolationNone),
			expectErr: false,
		},
		{
			name:      "valid destination",
			key:       NewIsolationKey(IsolationDestination).WithDestination("example.com:80"),
			expectErr: false,
		},
		{
			name:      "invalid destination - no port",
			key:       NewIsolationKey(IsolationDestination).WithDestination("example.com"),
			expectErr: true,
		},
		{
			name:      "destination missing",
			key:       NewIsolationKey(IsolationDestination),
			expectErr: true,
		},
		{
			name:      "valid credential",
			key:       NewIsolationKey(IsolationCredential).WithCredentials("user"),
			expectErr: false,
		},
		{
			name:      "credential missing",
			key:       NewIsolationKey(IsolationCredential),
			expectErr: true,
		},
		{
			name:      "valid port",
			key:       NewIsolationKey(IsolationPort).WithSourcePort(12345),
			expectErr: false,
		},
		{
			name:      "port zero",
			key:       NewIsolationKey(IsolationPort),
			expectErr: true,
		},
		{
			name:      "valid session",
			key:       NewIsolationKey(IsolationSession).WithSessionToken("token"),
			expectErr: false,
		},
		{
			name:      "session missing",
			key:       NewIsolationKey(IsolationSession),
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.key.Validate()
			if tt.expectErr && err == nil {
				t.Error("Validate() expected error, got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Validate() unexpected error: %v", err)
			}
		})
	}
}

func TestIsolationKey_Clone(t *testing.T) {
	original := NewIsolationKey(IsolationDestination).
		WithDestination("example.com:80")

	clone := original.Clone()

	if clone == nil {
		t.Fatal("Clone() returned nil")
	}

	if clone == original {
		t.Error("Clone() returned same pointer")
	}

	if !clone.Equals(original) {
		t.Error("Clone() not equal to original")
	}

	// Modify clone and verify original unchanged
	clone.Destination = "other.com:443"
	if original.Destination != "example.com:80" {
		t.Error("Modifying clone affected original")
	}
}

func TestIsolationKey_Clone_Nil(t *testing.T) {
	var key *IsolationKey
	clone := key.Clone()
	if clone != nil {
		t.Errorf("Clone() of nil = %v, want nil", clone)
	}
}

func TestIsolationKey_CredentialHashing(t *testing.T) {
	// Verify that same credentials produce same hash
	key1 := NewIsolationKey(IsolationCredential).WithCredentials("user123")
	key2 := NewIsolationKey(IsolationCredential).WithCredentials("user123")

	if key1.Credentials != key2.Credentials {
		t.Error("Same credentials produced different hashes")
	}

	// Verify different credentials produce different hashes
	key3 := NewIsolationKey(IsolationCredential).WithCredentials("user456")
	if key1.Credentials == key3.Credentials {
		t.Error("Different credentials produced same hash")
	}
}

func TestIsolationKey_SessionTokenHashing(t *testing.T) {
	// Verify that same tokens produce same hash
	key1 := NewIsolationKey(IsolationSession).WithSessionToken("token123")
	key2 := NewIsolationKey(IsolationSession).WithSessionToken("token123")

	if key1.SessionToken != key2.SessionToken {
		t.Error("Same tokens produced different hashes")
	}

	// Verify different tokens produce different hashes
	key3 := NewIsolationKey(IsolationSession).WithSessionToken("token456")
	if key1.SessionToken == key3.SessionToken {
		t.Error("Different tokens produced same hash")
	}
}
