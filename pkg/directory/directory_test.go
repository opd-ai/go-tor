package directory

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/logger"
)

func TestNewClient(t *testing.T) {
	client := NewClient(nil)

	if client == nil {
		t.Fatal("NewClient() returned nil")
	}
	if client.logger == nil {
		t.Error("logger should be initialized")
	}
	if client.httpClient == nil {
		t.Error("httpClient should be initialized")
	}
	if len(client.authorities) == 0 {
		t.Error("authorities should be initialized")
	}
}

func TestNewClientWithLogger(t *testing.T) {
	log := logger.NewDefault()
	client := NewClient(log)

	if client == nil {
		t.Fatal("NewClient() returned nil")
	}
	if client.logger == nil {
		t.Error("logger should be initialized")
	}
}

func TestParseConsensus(t *testing.T) {
	// Sample consensus document fragment (matching actual format)
	consensusData := `network-status-version 3
vote-status consensus
r Test1 AAAAAAAAAAAAAAAAAAAAAA BBBBBBBBBBBBB 2024-01-01 00:00:00 192.168.1.1 9001 0
s Fast Guard Running Stable Valid
r Test2 CCCCCCCCCCCCCCCCCCCCCC DDDDDDDDDDDDD 2024-01-01 00:00:00 192.168.1.2 9002 9030
s Exit Fast Running Stable Valid
r Test3 EEEEEEEEEEEEEEEEEEEEEE FFFFFFFFFFFFF 2024-01-01 00:00:00 192.168.1.3 9003 0
s Running Valid
`

	client := NewClient(nil)
	reader := strings.NewReader(consensusData)

	relays, err := client.parseConsensus(reader)
	if err != nil {
		t.Fatalf("parseConsensus() error = %v", err)
	}

	if len(relays) != 3 {
		t.Errorf("parseConsensus() returned %d relays, want 3", len(relays))
		return
	}

	// Check first relay
	if relays[0].Nickname != "Test1" {
		t.Errorf("relay[0].Nickname = %s, want Test1", relays[0].Nickname)
	}
	if relays[0].Address != "192.168.1.1" {
		t.Errorf("relay[0].Address = %s, want 192.168.1.1", relays[0].Address)
	}
	if relays[0].ORPort != 9001 {
		t.Errorf("relay[0].ORPort = %d, want 9001", relays[0].ORPort)
	}
	if !relays[0].HasFlag("Guard") {
		t.Error("relay[0] should have Guard flag")
	}

	// Check second relay
	if relays[1].Nickname != "Test2" {
		t.Errorf("relay[1].Nickname = %s, want Test2", relays[1].Nickname)
	}
	if relays[1].DirPort != 9030 {
		t.Errorf("relay[1].DirPort = %d, want 9030", relays[1].DirPort)
	}
	if !relays[1].HasFlag("Exit") {
		t.Error("relay[1] should have Exit flag")
	}
}

func TestParseConsensusEmpty(t *testing.T) {
	client := NewClient(nil)
	reader := strings.NewReader("")

	relays, err := client.parseConsensus(reader)
	if err != nil {
		t.Fatalf("parseConsensus() error = %v", err)
	}

	if len(relays) != 0 {
		t.Errorf("parseConsensus() returned %d relays, want 0", len(relays))
	}
}

func TestRelayHasFlag(t *testing.T) {
	relay := &Relay{
		Nickname: "Test",
		Flags:    []string{"Fast", "Guard", "Running", "Stable", "Valid"},
	}

	tests := []struct {
		flag     string
		expected bool
	}{
		{"Fast", true},
		{"Guard", true},
		{"Running", true},
		{"Exit", false},
		{"NotAFlag", false},
	}

	for _, tt := range tests {
		t.Run(tt.flag, func(t *testing.T) {
			got := relay.HasFlag(tt.flag)
			if got != tt.expected {
				t.Errorf("HasFlag(%s) = %v, want %v", tt.flag, got, tt.expected)
			}
		})
	}
}

func TestRelayIsGuard(t *testing.T) {
	tests := []struct {
		name     string
		flags    []string
		expected bool
	}{
		{"with_guard_flag", []string{"Fast", "Guard", "Running"}, true},
		{"without_guard_flag", []string{"Fast", "Running"}, false},
		{"empty_flags", []string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			relay := &Relay{Flags: tt.flags}
			got := relay.IsGuard()
			if got != tt.expected {
				t.Errorf("IsGuard() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestRelayIsExit(t *testing.T) {
	tests := []struct {
		name     string
		flags    []string
		expected bool
	}{
		{"with_exit_flag", []string{"Exit", "Fast", "Running"}, true},
		{"without_exit_flag", []string{"Fast", "Running"}, false},
		{"empty_flags", []string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			relay := &Relay{Flags: tt.flags}
			got := relay.IsExit()
			if got != tt.expected {
				t.Errorf("IsExit() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestRelayIsStable(t *testing.T) {
	relay := &Relay{Flags: []string{"Fast", "Stable", "Running"}}
	if !relay.IsStable() {
		t.Error("IsStable() = false, want true")
	}

	relay2 := &Relay{Flags: []string{"Fast", "Running"}}
	if relay2.IsStable() {
		t.Error("IsStable() = true, want false")
	}
}

func TestRelayIsRunning(t *testing.T) {
	relay := &Relay{Flags: []string{"Running"}}
	if !relay.IsRunning() {
		t.Error("IsRunning() = false, want true")
	}

	relay2 := &Relay{Flags: []string{"Fast"}}
	if relay2.IsRunning() {
		t.Error("IsRunning() = true, want false")
	}
}

func TestRelayIsValid(t *testing.T) {
	relay := &Relay{Flags: []string{"Valid", "Running"}}
	if !relay.IsValid() {
		t.Error("IsValid() = false, want true")
	}

	relay2 := &Relay{Flags: []string{"Running"}}
	if relay2.IsValid() {
		t.Error("IsValid() = true, want false")
	}
}

func TestRelayString(t *testing.T) {
	relay := &Relay{
		Nickname: "TestRelay",
		Address:  "192.168.1.1",
		ORPort:   9001,
	}

	expected := "TestRelay (192.168.1.1:9001)"
	got := relay.String()

	if got != expected {
		t.Errorf("String() = %s, want %s", got, expected)
	}
}

func TestFetchConsensusTimeout(t *testing.T) {
	client := NewClient(nil)
	// Use invalid authorities to test timeout
	client.authorities = []string{"http://192.0.2.1:9999/consensus"}
	client.httpClient.Timeout = 100 * time.Millisecond

	ctx := context.Background()
	_, err := client.FetchConsensus(ctx)

	if err == nil {
		t.Error("FetchConsensus() should fail with invalid authority")
	}
}

func TestFetchConsensusContextCanceled(t *testing.T) {
	client := NewClient(nil)
	client.authorities = []string{"http://192.0.2.1:9999/consensus"}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := client.FetchConsensus(ctx)

	if err == nil {
		t.Error("FetchConsensus() should fail with canceled context")
	}
}

func TestDefaultAuthorities(t *testing.T) {
	if len(DefaultAuthorities) == 0 {
		t.Error("DefaultAuthorities should not be empty")
	}

	for i, auth := range DefaultAuthorities {
		if !strings.HasPrefix(auth, "https://") && !strings.HasPrefix(auth, "http://") {
			t.Errorf("DefaultAuthorities[%d] = %s, should start with http:// or https://", i, auth)
		}
	}
}

// SPEC-003: Tests for enhanced consensus validation

func TestValidateConsensusMetadata(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		meta    *ConsensusMetadata
		wantErr bool
	}{
		{
			name: "valid_consensus",
			meta: &ConsensusMetadata{
				ValidAfter:     now.Add(-1 * time.Hour),
				FreshUntil:     now.Add(1 * time.Hour),
				ValidUntil:     now.Add(3 * time.Hour),
				SignatureCount: 5,
				AuthorityCount: 9,
				Signatures: []*ConsensusSignature{
					{Algorithm: "sha256", Identity: "A", SigningKeyDigest: "B", Signature: "s1"},
					{Algorithm: "sha256", Identity: "C", SigningKeyDigest: "D", Signature: "s2"},
					{Algorithm: "sha256", Identity: "E", SigningKeyDigest: "F", Signature: "s3"},
					{Algorithm: "sha256", Identity: "G", SigningKeyDigest: "H", Signature: "s4"},
					{Algorithm: "sha256", Identity: "I", SigningKeyDigest: "J", Signature: "s5"},
				},
			},
			wantErr: false,
		},
		{
			name: "insufficient_signatures",
			meta: &ConsensusMetadata{
				ValidAfter:     now.Add(-1 * time.Hour),
				FreshUntil:     now.Add(1 * time.Hour),
				ValidUntil:     now.Add(3 * time.Hour),
				SignatureCount: 1,
				AuthorityCount: 9,
				Signatures: []*ConsensusSignature{
					{Algorithm: "sha256", Identity: "A", SigningKeyDigest: "B", Signature: "s1"},
				},
			},
			wantErr: true,
		},
		{
			name: "insufficient_authorities",
			meta: &ConsensusMetadata{
				ValidAfter:     now.Add(-1 * time.Hour),
				FreshUntil:     now.Add(1 * time.Hour),
				ValidUntil:     now.Add(3 * time.Hour),
				SignatureCount: 5,
				AuthorityCount: 2,
				Signatures: []*ConsensusSignature{
					{Algorithm: "sha256", Identity: "A", SigningKeyDigest: "B", Signature: "s1"},
					{Algorithm: "sha256", Identity: "C", SigningKeyDigest: "D", Signature: "s2"},
					{Algorithm: "sha256", Identity: "E", SigningKeyDigest: "F", Signature: "s3"},
					{Algorithm: "sha256", Identity: "G", SigningKeyDigest: "H", Signature: "s4"},
					{Algorithm: "sha256", Identity: "I", SigningKeyDigest: "J", Signature: "s5"},
				},
			},
			wantErr: true,
		},
		{
			name: "expired_consensus",
			meta: &ConsensusMetadata{
				ValidAfter:     now.Add(-5 * time.Hour),
				FreshUntil:     now.Add(-3 * time.Hour),
				ValidUntil:     now.Add(-1 * time.Hour),
				SignatureCount: 5,
				AuthorityCount: 9,
				Signatures: []*ConsensusSignature{
					{Algorithm: "sha256", Identity: "A", SigningKeyDigest: "B", Signature: "s1"},
					{Algorithm: "sha256", Identity: "C", SigningKeyDigest: "D", Signature: "s2"},
					{Algorithm: "sha256", Identity: "E", SigningKeyDigest: "F", Signature: "s3"},
					{Algorithm: "sha256", Identity: "G", SigningKeyDigest: "H", Signature: "s4"},
					{Algorithm: "sha256", Identity: "I", SigningKeyDigest: "J", Signature: "s5"},
				},
			},
			wantErr: true,
		},
		{
			name: "future_consensus",
			meta: &ConsensusMetadata{
				ValidAfter:     now.Add(2 * time.Hour),
				FreshUntil:     now.Add(3 * time.Hour),
				ValidUntil:     now.Add(5 * time.Hour),
				SignatureCount: 5,
				AuthorityCount: 9,
				Signatures: []*ConsensusSignature{
					{Algorithm: "sha256", Identity: "A", SigningKeyDigest: "B", Signature: "s1"},
					{Algorithm: "sha256", Identity: "C", SigningKeyDigest: "D", Signature: "s2"},
					{Algorithm: "sha256", Identity: "E", SigningKeyDigest: "F", Signature: "s3"},
					{Algorithm: "sha256", Identity: "G", SigningKeyDigest: "H", Signature: "s4"},
					{Algorithm: "sha256", Identity: "I", SigningKeyDigest: "J", Signature: "s5"},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConsensusMetadata(tt.meta)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConsensusMetadata() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConsensusMetadataStructure(t *testing.T) {
	// Test that ConsensusMetadata can be created and used
	meta := &ConsensusMetadata{
		ValidAfter:     time.Now(),
		FreshUntil:     time.Now().Add(1 * time.Hour),
		ValidUntil:     time.Now().Add(3 * time.Hour),
		SignatureCount: 5,
		AuthorityCount: 9,
		Signatures: []*ConsensusSignature{
			{Algorithm: "sha256", Identity: "A", SigningKeyDigest: "B", Signature: "s1"},
			{Algorithm: "sha256", Identity: "C", SigningKeyDigest: "D", Signature: "s2"},
			{Algorithm: "sha256", Identity: "E", SigningKeyDigest: "F", Signature: "s3"},
			{Algorithm: "sha256", Identity: "G", SigningKeyDigest: "H", Signature: "s4"},
			{Algorithm: "sha256", Identity: "I", SigningKeyDigest: "J", Signature: "s5"},
		},
	}

	if meta.SignatureCount != 5 {
		t.Errorf("Expected 5 signatures, got %d", meta.SignatureCount)
	}
	if meta.AuthorityCount != 9 {
		t.Errorf("Expected 9 authorities, got %d", meta.AuthorityCount)
	}
	if len(meta.Signatures) != 5 {
		t.Errorf("Expected 5 signature structures, got %d", len(meta.Signatures))
	}
}

// SPEC-001: Tests for microdescriptor parsing and key extraction

func TestParseMicrodescriptorDigest(t *testing.T) {
	consensusData := `network-status-version 3
vote-status consensus
r TestRelay AAAAAAAAAAAAAAAAAAAAAA BBBBBBBBBBBBB 2024-01-01 00:00:00 192.168.1.1 9001 0
a sha256=dGVzdGRpZ2VzdA==
s Fast Guard Running Stable Valid
`

	client := NewClient(nil)
	reader := strings.NewReader(consensusData)

	relays, err := client.parseConsensus(reader)
	if err != nil {
		t.Fatalf("parseConsensus() error = %v", err)
	}

	if len(relays) != 1 {
		t.Fatalf("Expected 1 relay, got %d", len(relays))
	}

	if relays[0].MicrodescDigest != "dGVzdGRpZ2VzdA==" {
		t.Errorf("Expected microdesc digest 'dGVzdGRpZ2VzdA==', got '%s'", relays[0].MicrodescDigest)
	}
}

func TestParseMicrodescriptors(t *testing.T) {
	// Sample microdescriptor format per dir-spec.txt
	mdData := `onion-key
-----BEGIN RSA PUBLIC KEY-----
MIGJAoGBAKrJn...
-----END RSA PUBLIC KEY-----
ntor-onion-key hSDwCYkwp1R0i33ctD0CAwEAAaOCAZIwggGOMB0GA1UdDgQWBBSoSmpjBH3duubRObem
id ed25519
dGVzdGlkZW50aXR5a2V5MTIzNDU2Nzg5MDEy

onion-key
-----BEGIN RSA PUBLIC KEY-----
MIGJAoGBAKrJn...
-----END RSA PUBLIC KEY-----
ntor-onion-key AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=
id ed25519
BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB
`

	client := NewClient(nil)
	digestMap := make(map[string][]*Relay)

	// Create test relay
	relay := &Relay{
		Nickname: "TestRelay",
	}

	// Calculate expected digest
	testDigest := client.calculateMicrodescriptorDigest([]string{
		"onion-key",
		"-----BEGIN RSA PUBLIC KEY-----",
		"MIGJAoGBAKrJn...",
		"-----END RSA PUBLIC KEY-----",
		"ntor-onion-key hSDwCYkwp1R0i33ctD0CAwEAAaOCAZIwggGOMB0GA1UdDgQWBBSoSmpjBH3duubRObem",
		"id ed25519",
		"dGVzdGlkZW50aXR5a2V5MTIzNDU2Nzg5MDEy",
	})
	digestMap[testDigest] = []*Relay{relay}

	err := client.parseMicrodescriptors([]byte(mdData), digestMap)
	if err != nil {
		t.Fatalf("parseMicrodescriptors() error = %v", err)
	}

	if relay.NtorOnionKey != nil {
		t.Logf("Ntor key populated: %d bytes", len(relay.NtorOnionKey))
	}
	if relay.IdentityKey != nil {
		t.Logf("Identity key populated: %d bytes", len(relay.IdentityKey))
	}
}

func TestRelayHasValidKeys(t *testing.T) {
	tests := []struct {
		name     string
		relay    *Relay
		expected bool
	}{
		{
			name: "both_keys_valid",
			relay: &Relay{
				IdentityKey:  make([]byte, 32),
				NtorOnionKey: make([]byte, 32),
			},
			expected: true,
		},
		{
			name: "missing_identity_key",
			relay: &Relay{
				NtorOnionKey: make([]byte, 32),
			},
			expected: false,
		},
		{
			name: "missing_ntor_key",
			relay: &Relay{
				IdentityKey: make([]byte, 32),
			},
			expected: false,
		},
		{
			name: "wrong_identity_key_length",
			relay: &Relay{
				IdentityKey:  make([]byte, 16),
				NtorOnionKey: make([]byte, 32),
			},
			expected: false,
		},
		{
			name: "wrong_ntor_key_length",
			relay: &Relay{
				IdentityKey:  make([]byte, 32),
				NtorOnionKey: make([]byte, 16),
			},
			expected: false,
		},
		{
			name:     "both_keys_missing",
			relay:    &Relay{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.relay.HasValidKeys()
			if got != tt.expected {
				t.Errorf("HasValidKeys() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGetIdentityKey(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	relay := &Relay{
		IdentityKey: key,
	}

	got := relay.GetIdentityKey()
	if len(got) != 32 {
		t.Errorf("GetIdentityKey() returned %d bytes, want 32", len(got))
	}
	if !bytesEqual(got, key) {
		t.Error("GetIdentityKey() returned different key")
	}
}

func TestGetNtorOnionKey(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 100)
	}

	relay := &Relay{
		NtorOnionKey: key,
	}

	got := relay.GetNtorOnionKey()
	if len(got) != 32 {
		t.Errorf("GetNtorOnionKey() returned %d bytes, want 32", len(got))
	}
	if !bytesEqual(got, key) {
		t.Error("GetNtorOnionKey() returned different key")
	}
}

func bytesEqual(a, b []byte) bool {
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

// SPEC-003: Tests for consensus signature parsing

func TestParseConsensusWithSignatures(t *testing.T) {
	// Mock consensus with directory signatures
	consensusData := `network-status-version 3
valid-after 2026-01-24 12:00:00
fresh-until 2026-01-24 13:00:00
valid-until 2026-01-24 15:00:00
r TestRelay1 AAAAAAAAAAAAAAAAAAAAAA BBBBBBBBBBBBB 2026-01-24 12:00:00 192.0.2.1 9001 9030
s Fast Guard Running Stable Valid
directory-signature sha256 AABBCCDD EEFF0011
-----BEGIN SIGNATURE-----
dGVzdHNpZ25hdHVyZTEyMzQ1Ng==
-----END SIGNATURE-----
directory-signature sha256 11223344 55667788
-----BEGIN SIGNATURE-----
YW5vdGhlcnNpZ25hdHVyZTc4OTA=
-----END SIGNATURE-----
directory-signature sha256 99AABBCC DDEEFF00
-----BEGIN SIGNATURE-----
dGhpcmRzaWduYXR1cmU5OTg4Nzc=
-----END SIGNATURE-----
`

	client := NewClient(nil)
	relays, metadata, err := client.parseConsensusWithMetadata(strings.NewReader(consensusData))
	if err != nil {
		t.Fatalf("parseConsensusWithMetadata() error = %v", err)
	}

	// Validate relay parsing still works
	if len(relays) != 1 {
		t.Errorf("Expected 1 relay, got %d", len(relays))
	}

	// Validate metadata was parsed
	if metadata == nil {
		t.Fatal("Expected metadata, got nil")
	}

	// Check signature count
	if metadata.SignatureCount != 3 {
		t.Errorf("SignatureCount = %d, want 3", metadata.SignatureCount)
	}

	// Check signatures were parsed
	if len(metadata.Signatures) != 3 {
		t.Errorf("len(Signatures) = %d, want 3", len(metadata.Signatures))
	}

	// Validate first signature
	if len(metadata.Signatures) > 0 {
		sig := metadata.Signatures[0]
		if sig.Algorithm != "sha256" {
			t.Errorf("Signature[0].Algorithm = %s, want sha256", sig.Algorithm)
		}
		if sig.Identity != "AABBCCDD" {
			t.Errorf("Signature[0].Identity = %s, want AABBCCDD", sig.Identity)
		}
		if sig.SigningKeyDigest != "EEFF0011" {
			t.Errorf("Signature[0].SigningKeyDigest = %s, want EEFF0011", sig.SigningKeyDigest)
		}
		if !strings.Contains(sig.Signature, "BEGIN SIGNATURE") {
			t.Errorf("Signature[0].Signature missing BEGIN marker")
		}
	}

	// Check timestamps
	expectedValidAfter, _ := time.Parse("2006-01-02 15:04:05", "2026-01-24 12:00:00")
	if !metadata.ValidAfter.Equal(expectedValidAfter) {
		t.Errorf("ValidAfter = %v, want %v", metadata.ValidAfter, expectedValidAfter)
	}

	// Validate metadata passes validation
	if err := ValidateConsensusMetadata(metadata); err != nil {
		t.Errorf("ValidateConsensusMetadata() error = %v", err)
	}
}

func TestParseConsensusWithoutSignatures(t *testing.T) {
	consensusData := `network-status-version 3
valid-after 2026-01-24 12:00:00
fresh-until 2026-01-24 13:00:00
valid-until 2026-01-24 15:00:00
r TestRelay1 AAAAAAAAAAAAAAAAAAAAAA BBBBBBBBBBBBB 2026-01-24 12:00:00 192.0.2.1 9001 9030
s Fast Guard Running Valid
`

	client := NewClient(nil)
	_, metadata, err := client.parseConsensusWithMetadata(strings.NewReader(consensusData))
	if err != nil {
		t.Fatalf("parseConsensusWithMetadata() error = %v", err)
	}

	// Should fail validation due to insufficient signatures
	if err := ValidateConsensusMetadata(metadata); err == nil {
		t.Error("Expected validation error for consensus without signatures")
	}
}

func TestValidateConsensusMetadataEnhanced(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		meta    *ConsensusMetadata
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid_with_signatures",
			meta: &ConsensusMetadata{
				ValidAfter:     now.Add(-1 * time.Hour),
				ValidUntil:     now.Add(3 * time.Hour),
				SignatureCount: 3,
				AuthorityCount: 6,
				Signatures: []*ConsensusSignature{
					{Algorithm: "sha256", Identity: "AAA", SigningKeyDigest: "BBB", Signature: "sig1"},
					{Algorithm: "sha256", Identity: "CCC", SigningKeyDigest: "DDD", Signature: "sig2"},
					{Algorithm: "sha256", Identity: "EEE", SigningKeyDigest: "FFF", Signature: "sig3"},
				},
			},
			wantErr: false,
		},
		{
			name: "signature_count_mismatch",
			meta: &ConsensusMetadata{
				ValidAfter:     now.Add(-1 * time.Hour),
				ValidUntil:     now.Add(3 * time.Hour),
				SignatureCount: 3,
				AuthorityCount: 6,
				Signatures: []*ConsensusSignature{
					{Algorithm: "sha256", Identity: "AAA", SigningKeyDigest: "BBB", Signature: "sig1"},
				},
			},
			wantErr: true,
			errMsg:  "signature count mismatch",
		},
		{
			name: "missing_signature_fields",
			meta: &ConsensusMetadata{
				ValidAfter:     now.Add(-1 * time.Hour),
				ValidUntil:     now.Add(3 * time.Hour),
				SignatureCount: 2,
				AuthorityCount: 6,
				Signatures: []*ConsensusSignature{
					{Algorithm: "sha256", Identity: "AAA", SigningKeyDigest: "", Signature: "sig1"},
					{Algorithm: "", Identity: "CCC", SigningKeyDigest: "DDD", Signature: "sig2"},
				},
			},
			wantErr: true,
			errMsg:  "missing required fields",
		},
		{
			name: "missing_timestamps",
			meta: &ConsensusMetadata{
				SignatureCount: 3,
				AuthorityCount: 6,
				Signatures: []*ConsensusSignature{
					{Algorithm: "sha256", Identity: "AAA", SigningKeyDigest: "BBB", Signature: "sig1"},
					{Algorithm: "sha256", Identity: "CCC", SigningKeyDigest: "DDD", Signature: "sig2"},
					{Algorithm: "sha256", Identity: "EEE", SigningKeyDigest: "FFF", Signature: "sig3"},
				},
			},
			wantErr: true,
			errMsg:  "missing required timestamp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConsensusMetadata(tt.meta)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConsensusMetadata() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("Error message = %v, want to contain %v", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestConsensusSignatureStructure(t *testing.T) {
	sig := &ConsensusSignature{
		Algorithm:        "sha256",
		Identity:         "1234567890ABCDEF",
		SigningKeyDigest: "FEDCBA0987654321",
		Signature:        "-----BEGIN SIGNATURE-----\nbase64data\n-----END SIGNATURE-----",
	}

	if sig.Algorithm != "sha256" {
		t.Errorf("Algorithm = %s, want sha256", sig.Algorithm)
	}
	if sig.Identity == "" {
		t.Error("Identity should not be empty")
	}
	if sig.SigningKeyDigest == "" {
		t.Error("SigningKeyDigest should not be empty")
	}
	if !strings.Contains(sig.Signature, "BEGIN SIGNATURE") {
		t.Error("Signature should contain PEM markers")
	}
}

func TestVerifyConsensusSignatures(t *testing.T) {
	// Create valid-length signature (128 bytes for RSA-1024)
	validSig128 := base64.StdEncoding.EncodeToString(make([]byte, 128))
	validSig256 := base64.StdEncoding.EncodeToString(make([]byte, 256))
	shortSig := base64.StdEncoding.EncodeToString(make([]byte, 64))

	tests := []struct {
		name          string
		consensusBody []byte
		meta          *ConsensusMetadata
		wantErr       bool
		errMsg        string
	}{
		{
			name:          "Empty consensus body",
			consensusBody: []byte{},
			meta: &ConsensusMetadata{
				Signatures: []*ConsensusSignature{
					{Algorithm: "sha256", Identity: "ABC", SigningKeyDigest: "DEF", Signature: validSig128},
				},
			},
			wantErr: true,
			errMsg:  "empty consensus body",
		},
		{
			name:          "No signatures",
			consensusBody: []byte("test body"),
			meta:          &ConsensusMetadata{Signatures: []*ConsensusSignature{}},
			wantErr:       true,
			errMsg:        "no signatures",
		},
		{
			name:          "Sufficient known authority signatures",
			consensusBody: []byte("network-status-version 3\ntest consensus body"),
			meta: &ConsensusMetadata{
				SignatureCount: 3,
				Signatures: []*ConsensusSignature{
					{Algorithm: "sha256", Identity: "ED03BB616EB2F60BEC80151114BB25CEF515B226", SigningKeyDigest: "KEY1", Signature: validSig128}, // gabelmoo
					{Algorithm: "sha256", Identity: "F533C81CEF0BC0267857C99B2F471ADF249FA232", SigningKeyDigest: "KEY2", Signature: validSig256}, // moria1
					{Algorithm: "sha256", Identity: "23D15D965BC35114467363C165C4F724B64B4F66", SigningKeyDigest: "KEY3", Signature: validSig128}, // longclaw
				},
			},
			wantErr: false,
		},
		{
			name:          "Unknown authority signatures",
			consensusBody: []byte("test body"),
			meta: &ConsensusMetadata{
				SignatureCount: 3,
				Signatures: []*ConsensusSignature{
					{Algorithm: "sha256", Identity: "UNKNOWN1111111111111111111111111111111111", SigningKeyDigest: "KEY1", Signature: validSig128},
					{Algorithm: "sha256", Identity: "UNKNOWN2222222222222222222222222222222222", SigningKeyDigest: "KEY2", Signature: validSig128},
					{Algorithm: "sha256", Identity: "UNKNOWN3333333333333333333333333333333333", SigningKeyDigest: "KEY3", Signature: validSig128},
				},
			},
			wantErr: true,
			errMsg:  "insufficient known authorities",
		},
		{
			name:          "Mixed known and unknown authorities",
			consensusBody: []byte("test body"),
			meta: &ConsensusMetadata{
				SignatureCount: 5,
				Signatures: []*ConsensusSignature{
					{Algorithm: "sha256", Identity: "ED03BB616EB2F60BEC80151114BB25CEF515B226", SigningKeyDigest: "KEY1", Signature: validSig128}, // gabelmoo (known)
					{Algorithm: "sha256", Identity: "UNKNOWN1111111111111111111111111111111111", SigningKeyDigest: "KEY2", Signature: validSig128}, // unknown
					{Algorithm: "sha256", Identity: "F533C81CEF0BC0267857C99B2F471ADF249FA232", SigningKeyDigest: "KEY3", Signature: validSig128}, // moria1 (known)
					{Algorithm: "sha256", Identity: "23D15D965BC35114467363C165C4F724B64B4F66", SigningKeyDigest: "KEY4", Signature: validSig128}, // longclaw (known)
					{Algorithm: "sha256", Identity: "UNKNOWN2222222222222222222222222222222222", SigningKeyDigest: "KEY5", Signature: validSig128}, // unknown
				},
			},
			wantErr: false, // Should pass with 3 known authorities
		},
		{
			name:          "Insufficient valid signatures",
			consensusBody: []byte("test body"),
			meta: &ConsensusMetadata{
				SignatureCount: 1,
				Signatures: []*ConsensusSignature{
					{Algorithm: "sha256", Identity: "ED03BB616EB2F60BEC80151114BB25CEF515B226", SigningKeyDigest: "KEY1", Signature: validSig128},
				},
			},
			wantErr: true,
			errMsg:  "insufficient known authorities",
		},
		{
			name:          "Invalid base64 signature",
			consensusBody: []byte("test body"),
			meta: &ConsensusMetadata{
				SignatureCount: 3,
				Signatures: []*ConsensusSignature{
					{Algorithm: "sha256", Identity: "ED03BB616EB2F60BEC80151114BB25CEF515B226", SigningKeyDigest: "KEY1", Signature: "!!!invalid-base64!!!"},
					{Algorithm: "sha256", Identity: "F533C81CEF0BC0267857C99B2F471ADF249FA232", SigningKeyDigest: "KEY2", Signature: validSig128},
					{Algorithm: "sha256", Identity: "23D15D965BC35114467363C165C4F724B64B4F66", SigningKeyDigest: "KEY3", Signature: validSig128},
				},
			},
			wantErr: true, // Only 2 valid signatures, need 3 authorities
		},
		{
			name:          "Signature too short",
			consensusBody: []byte("test body"),
			meta: &ConsensusMetadata{
				SignatureCount: 4,
				Signatures: []*ConsensusSignature{
					{Algorithm: "sha256", Identity: "ED03BB616EB2F60BEC80151114BB25CEF515B226", SigningKeyDigest: "KEY1", Signature: shortSig},      // too short
					{Algorithm: "sha256", Identity: "F533C81CEF0BC0267857C99B2F471ADF249FA232", SigningKeyDigest: "KEY2", Signature: validSig128},   // valid
					{Algorithm: "sha256", Identity: "23D15D965BC35114467363C165C4F724B64B4F66", SigningKeyDigest: "KEY3", Signature: validSig128},   // valid
					{Algorithm: "sha256", Identity: "0232AF901C31A04EE9848595AF9BB7620D4C5B2E", SigningKeyDigest: "KEY4", Signature: validSig128},   // valid (dannenberg)
				},
			},
			wantErr: false, // Should pass with 3 valid signatures
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := VerifyConsensusSignatures(tt.consensusBody, tt.meta)
			if (err != nil) != tt.wantErr {
				t.Errorf("VerifyConsensusSignatures() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errMsg != "" && (err == nil || !strings.Contains(err.Error(), tt.errMsg)) {
				t.Errorf("Error message = %v, want to contain %v", err, tt.errMsg)
			}
		})
	}
}

func TestIsKnownAuthority(t *testing.T) {
	tests := []struct {
		name    string
		v3ident string
		want    bool
	}{
		{"gabelmoo", "ED03BB616EB2F60BEC80151114BB25CEF515B226", true},
		{"moria1", "F533C81CEF0BC0267857C99B2F471ADF249FA232", true},
		{"longclaw", "23D15D965BC35114467363C165C4F724B64B4F66", true},
		{"lowercase gabelmoo", "ed03bb616eb2f60bec80151114bb25cef515b226", true},
		{"unknown", "1111111111111111111111111111111111111111", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isKnownAuthority(tt.v3ident); got != tt.want {
				t.Errorf("isKnownAuthority(%v) = %v, want %v", tt.v3ident, got, tt.want)
			}
		})
	}
}

func TestGetAuthorityName(t *testing.T) {
	tests := []struct {
		name    string
		v3ident string
		want    string
	}{
		{"gabelmoo", "ED03BB616EB2F60BEC80151114BB25CEF515B226", "gabelmoo"},
		{"moria1", "F533C81CEF0BC0267857C99B2F471ADF249FA232", "moria1"},
		{"longclaw", "23D15D965BC35114467363C165C4F724B64B4F66", "longclaw"},
		{"lowercase", "ed03bb616eb2f60bec80151114bb25cef515b226", "gabelmoo"},
		{"unknown", "1111111111111111111111111111111111111111", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getAuthorityName(tt.v3ident); got != tt.want {
				t.Errorf("getAuthorityName(%v) = %v, want %v", tt.v3ident, got, tt.want)
			}
		})
	}
}

func TestKnownAuthoritiesDatabase(t *testing.T) {
	// Verify all known authorities are properly configured
	if len(KnownAuthorities) < 9 {
		t.Errorf("Expected at least 9 known authorities, got %d", len(KnownAuthorities))
	}

	// Verify each authority has required fields
	for _, auth := range KnownAuthorities {
		if auth.Nickname == "" {
			t.Errorf("Authority missing nickname: %+v", auth)
		}
		if len(auth.V3Ident) != 40 {
			t.Errorf("Authority %s has invalid v3ident length: %d (expected 40)", auth.Nickname, len(auth.V3Ident))
		}
		if auth.Address == "" {
			t.Errorf("Authority %s missing address", auth.Nickname)
		}
	}

	// Verify no duplicate v3idents
	seen := make(map[string]string)
	for _, auth := range KnownAuthorities {
		if existing, found := seen[auth.V3Ident]; found {
			t.Errorf("Duplicate v3ident %s found in authorities %s and %s", auth.V3Ident, existing, auth.Nickname)
		}
		seen[auth.V3Ident] = auth.Nickname
	}
}
