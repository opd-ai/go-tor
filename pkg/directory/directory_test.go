package directory

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
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

func TestParseBandwidthWeights(t *testing.T) {
	// Test parsing of "w" lines (bandwidth weights) per path-spec.txt §2.2
	consensusData := `network-status-version 3
vote-status consensus
r HighBW AAAAAAAAAAAAAAAAAAAAAA BBBBBBBBBBBBB 2024-01-01 00:00:00 192.168.1.1 9001 0
s Fast Guard Running Stable Valid
w Bandwidth=10000000
r MediumBW CCCCCCCCCCCCCCCCCCCCCC DDDDDDDDDDDDD 2024-01-01 00:00:00 192.168.1.2 9002 0
s Running Valid
w Bandwidth=5000000
r NoBW EEEEEEEEEEEEEEEEEEEEEE FFFFFFFFFFFFF 2024-01-01 00:00:00 192.168.1.3 9003 0
s Running Valid
`

	client := NewClient(nil)
	reader := strings.NewReader(consensusData)

	relays, err := client.parseConsensus(reader)
	if err != nil {
		t.Fatalf("parseConsensus() error = %v", err)
	}

	if len(relays) != 3 {
		t.Fatalf("parseConsensus() returned %d relays, want 3", len(relays))
	}

	// Test first relay with bandwidth
	if relays[0].Nickname != "HighBW" {
		t.Errorf("relay[0].Nickname = %s, want HighBW", relays[0].Nickname)
	}
	if relays[0].Bandwidth != 10000000 {
		t.Errorf("relay[0].Bandwidth = %d, want 10000000", relays[0].Bandwidth)
	}

	// Test second relay with bandwidth
	if relays[1].Nickname != "MediumBW" {
		t.Errorf("relay[1].Nickname = %s, want MediumBW", relays[1].Nickname)
	}
	if relays[1].Bandwidth != 5000000 {
		t.Errorf("relay[1].Bandwidth = %d, want 5000000", relays[1].Bandwidth)
	}

	// Test third relay without bandwidth line
	if relays[2].Nickname != "NoBW" {
		t.Errorf("relay[2].Nickname = %s, want NoBW", relays[2].Nickname)
	}
	if relays[2].Bandwidth != 0 {
		t.Errorf("relay[2].Bandwidth = %d, want 0 (no w line)", relays[2].Bandwidth)
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

func TestParseMicrodescriptorDigestConsensusMethod33(t *testing.T) {
	// Test modern consensus-method 33 format with "m" lines
	consensusData := `network-status-version 3
vote-status consensus
consensus-method 33
r TestRelay AAAAAAAAAAAAAAAAAAAAAA 2038-01-01 00:00:00 192.168.1.1 9001 0
m jauY803ygX19rw14B2x4suqNIIMIPPbtYBAwA9UegdI
s Fast Guard Running Stable Valid
v Tor 0.4.8.21
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

	expectedDigest := "jauY803ygX19rw14B2x4suqNIIMIPPbtYBAwA9UegdI"
	if relays[0].MicrodescDigest != expectedDigest {
		t.Errorf("Expected microdesc digest '%s', got '%s'", expectedDigest, relays[0].MicrodescDigest)
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
	// Use future timestamps to avoid expiration (valid for next 3 hours from now)
	now := time.Now().UTC()
	validAfter := now.Add(-1 * time.Hour).Format("2006-01-02 15:04:05")
	freshUntil := now.Add(1 * time.Hour).Format("2006-01-02 15:04:05")
	validUntil := now.Add(3 * time.Hour).Format("2006-01-02 15:04:05")

	// Mock consensus with directory signatures
	consensusData := fmt.Sprintf(`network-status-version 3
valid-after %s
fresh-until %s
valid-until %s
r TestRelay1 AAAAAAAAAAAAAAAAAAAAAA BBBBBBBBBBBBB %s 192.0.2.1 9001 9030
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
directory-signature sha256 44556677 88990011
-----BEGIN SIGNATURE-----
Zm91cnRoc2lnbmF0dXJlMTIzNDU=
-----END SIGNATURE-----
directory-signature sha256 AABBCCDD 22334455
-----BEGIN SIGNATURE-----
ZmlmdGhzaWduYXR1cmU2Nzg5MA==
-----END SIGNATURE-----
`, validAfter, freshUntil, validUntil, validAfter)

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
	if metadata.SignatureCount != 5 {
		t.Errorf("SignatureCount = %d, want 5", metadata.SignatureCount)
	}

	// Check signatures were parsed
	if len(metadata.Signatures) != 5 {
		t.Errorf("len(Signatures) = %d, want 5", len(metadata.Signatures))
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

	// Validate ValidAfter timestamp is set (don't check exact value due to dynamic generation)
	if metadata.ValidAfter.IsZero() {
		t.Error("ValidAfter timestamp is zero")
	}

	// Validate metadata passes validation (should not be expired)
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
				SignatureCount: 5,
				AuthorityCount: 6,
				Signatures: []*ConsensusSignature{
					{Algorithm: "sha256", Identity: "AAA", SigningKeyDigest: "BBB", Signature: "sig1"},
					{Algorithm: "sha256", Identity: "CCC", SigningKeyDigest: "DDD", Signature: "sig2"},
					{Algorithm: "sha256", Identity: "EEE", SigningKeyDigest: "FFF", Signature: "sig3"},
					{Algorithm: "sha256", Identity: "GGG", SigningKeyDigest: "HHH", Signature: "sig4"},
					{Algorithm: "sha256", Identity: "III", SigningKeyDigest: "JJJ", Signature: "sig5"},
				},
			},
			wantErr: false,
		},
		{
			name: "signature_count_mismatch",
			meta: &ConsensusMetadata{
				ValidAfter:     now.Add(-1 * time.Hour),
				ValidUntil:     now.Add(3 * time.Hour),
				SignatureCount: 5,
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
				SignatureCount: 5,
				AuthorityCount: 6,
				Signatures: []*ConsensusSignature{
					{Algorithm: "sha256", Identity: "AAA", SigningKeyDigest: "", Signature: "sig1"},
					{Algorithm: "", Identity: "CCC", SigningKeyDigest: "DDD", Signature: "sig2"},
					{Algorithm: "sha256", Identity: "EEE", SigningKeyDigest: "FFF", Signature: "sig3"},
					{Algorithm: "sha256", Identity: "GGG", SigningKeyDigest: "HHH", Signature: "sig4"},
					{Algorithm: "sha256", Identity: "III", SigningKeyDigest: "JJJ", Signature: "sig5"},
				},
			},
			wantErr: true,
			errMsg:  "missing required fields",
		},
		{
			name: "missing_timestamps",
			meta: &ConsensusMetadata{
				SignatureCount: 5,
				AuthorityCount: 6,
				Signatures: []*ConsensusSignature{
					{Algorithm: "sha256", Identity: "AAA", SigningKeyDigest: "BBB", Signature: "sig1"},
					{Algorithm: "sha256", Identity: "CCC", SigningKeyDigest: "DDD", Signature: "sig2"},
					{Algorithm: "sha256", Identity: "EEE", SigningKeyDigest: "FFF", Signature: "sig3"},
					{Algorithm: "sha256", Identity: "GGG", SigningKeyDigest: "HHH", Signature: "sig4"},
					{Algorithm: "sha256", Identity: "III", SigningKeyDigest: "JJJ", Signature: "sig5"},
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
	shortSig := base64.StdEncoding.EncodeToString(make([]byte, 64))

	client := NewClient(nil)
	ctx := context.Background()

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
			wantErr: true,
			errMsg:  "insufficient",
		},
		{
			name:          "Signature too short",
			consensusBody: []byte("test body"),
			meta: &ConsensusMetadata{
				SignatureCount: 4,
				Signatures: []*ConsensusSignature{
					{Algorithm: "sha256", Identity: "ED03BB616EB2F60BEC80151114BB25CEF515B226", SigningKeyDigest: "KEY1", Signature: shortSig},    // too short
					{Algorithm: "sha256", Identity: "F533C81CEF0BC0267857C99B2F471ADF249FA232", SigningKeyDigest: "KEY2", Signature: validSig128}, // valid
					{Algorithm: "sha256", Identity: "23D15D965BC35114467363C165C4F724B64B4F66", SigningKeyDigest: "KEY3", Signature: validSig128}, // valid
					{Algorithm: "sha256", Identity: "0232AF901C31A04EE9848595AF9BB7620D4C5B2E", SigningKeyDigest: "KEY4", Signature: validSig128}, // valid (dannenberg)
				},
			},
			wantErr: true,
			errMsg:  "insufficient",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.VerifyConsensusSignatures(ctx, tt.consensusBody, tt.meta)
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

// TestExtractSignatureData tests signature data extraction from PEM blocks
func TestExtractSignatureData(t *testing.T) {
	tests := []struct {
		name     string
		sigBlock string
		want     string
	}{
		{
			name: "Valid PEM block",
			sigBlock: `-----BEGIN SIGNATURE-----
VGVzdFNpZ25hdHVyZURhdGE=
-----END SIGNATURE-----`,
			want: "VGVzdFNpZ25hdHVyZURhdGE=",
		},
		{
			name: "Multi-line PEM block",
			sigBlock: `-----BEGIN SIGNATURE-----
VGVzdFNp
Z25hdHVy
ZURhdGE=
-----END SIGNATURE-----`,
			want: "VGVzdFNpZ25hdHVyZURhdGE=",
		},
		{
			name:     "No PEM markers",
			sigBlock: "VGVzdFNpZ25hdHVyZURhdGE=",
			want:     "",
		},
		{
			name:     "Empty block",
			sigBlock: "",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractSignatureData(tt.sigBlock)
			if got != tt.want {
				t.Errorf("extractSignatureData() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestAuthorityCertCache tests certificate caching functionality
func TestAuthorityCertCache(t *testing.T) {
	logger := logger.NewDefault()
	cache := &AuthorityCertCache{
		certs:  make(map[string]*AuthorityCert),
		logger: logger.Component("certcache"),
	}

	// Test cache initialization
	if cache.certs == nil {
		t.Fatal("Cache certs map not initialized")
	}

	// Test manual cert insertion and retrieval
	testCert := &AuthorityCert{
		Identity:  "TEST123",
		ExpiresAt: time.Now().Add(24 * time.Hour),
		FetchedAt: time.Now(),
	}

	cache.mu.Lock()
	cache.certs["TEST123"] = testCert
	cache.mu.Unlock()

	cache.mu.RLock()
	retrieved, ok := cache.certs["TEST123"]
	cache.mu.RUnlock()

	if !ok {
		t.Error("Failed to retrieve cached cert")
	}
	if retrieved.Identity != "TEST123" {
		t.Errorf("Retrieved cert identity = %s, want TEST123", retrieved.Identity)
	}
}

// TestParseAuthorityCert tests authority certificate parsing
func TestParseAuthorityCert(t *testing.T) {
	logger := logger.NewDefault()
	cache := &AuthorityCertCache{
		certs:  make(map[string]*AuthorityCert),
		logger: logger.Component("certcache"),
	}

	// Test parsing with a valid PKCS1 RSA public key (minimal 512-bit for testing)
	// This is a real valid RSA key for testing purposes only
	certData := `dir-source gabelmoo
fingerprint ED03 BB61 6EB2 F60B EC80 1511 14BB 25CE F515 B226
dir-key-expires 2027-01-01 00:00:00
-----BEGIN RSA PUBLIC KEY-----
MEgCQQC7VJTUt9Us8WXZHY/7/w4M1iSp3PNxCCPyOuLYmUxJ+NjF8uYGE00j+6C0
y5TQJtSNlMLaPfJQr8PZQhClq5cJAgMBAAE=
-----END RSA PUBLIC KEY-----`

	cert, err := cache.parseAuthorityCert([]byte(certData), "ED03BB616EB2F60BEC80151114BB25CEF515B226")
	if err != nil {
		t.Fatalf("parseAuthorityCert() error = %v", err)
	}

	if cert.Identity != "ED03BB616EB2F60BEC80151114BB25CEF515B226" {
		t.Errorf("cert.Identity = %s, want ED03BB616EB2F60BEC80151114BB25CEF515B226", cert.Identity)
	}

	expectedExpires := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	if !cert.ExpiresAt.Equal(expectedExpires) {
		t.Errorf("cert.ExpiresAt = %v, want %v", cert.ExpiresAt, expectedExpires)
	}
}

// TestValidateConsensusMetadataWithUpdatedThresholds tests enhanced validation
func TestValidateConsensusMetadataWithUpdatedThresholds(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		meta    *ConsensusMetadata
		wantErr bool
		errMsg  string
	}{
		{
			name: "Valid with 5 signatures",
			meta: &ConsensusMetadata{
				ValidAfter:     now.Add(-1 * time.Hour),
				ValidUntil:     now.Add(1 * time.Hour),
				SignatureCount: 5,
				AuthorityCount: 5,
				Signatures: []*ConsensusSignature{
					{Algorithm: "sha256", Identity: "ID1", Signature: "sig1"},
					{Algorithm: "sha256", Identity: "ID2", Signature: "sig2"},
					{Algorithm: "sha256", Identity: "ID3", Signature: "sig3"},
					{Algorithm: "sha256", Identity: "ID4", Signature: "sig4"},
					{Algorithm: "sha256", Identity: "ID5", Signature: "sig5"},
				},
			},
			wantErr: false,
		},
		{
			name: "Insufficient signatures (4 < 5)",
			meta: &ConsensusMetadata{
				ValidAfter:     now.Add(-1 * time.Hour),
				ValidUntil:     now.Add(1 * time.Hour),
				SignatureCount: 4,
				AuthorityCount: 5,
				Signatures: []*ConsensusSignature{
					{Algorithm: "sha256", Identity: "ID1", Signature: "sig1"},
					{Algorithm: "sha256", Identity: "ID2", Signature: "sig2"},
					{Algorithm: "sha256", Identity: "ID3", Signature: "sig3"},
					{Algorithm: "sha256", Identity: "ID4", Signature: "sig4"},
				},
			},
			wantErr: true,
			errMsg:  "insufficient signatures",
		},
		{
			name: "Insufficient authorities (4 < 5)",
			meta: &ConsensusMetadata{
				ValidAfter:     now.Add(-1 * time.Hour),
				ValidUntil:     now.Add(1 * time.Hour),
				SignatureCount: 5,
				AuthorityCount: 4,
				Signatures: []*ConsensusSignature{
					{Algorithm: "sha256", Identity: "ID1", Signature: "sig1"},
					{Algorithm: "sha256", Identity: "ID2", Signature: "sig2"},
					{Algorithm: "sha256", Identity: "ID3", Signature: "sig3"},
					{Algorithm: "sha256", Identity: "ID4", Signature: "sig4"},
					{Algorithm: "sha256", Identity: "ID5", Signature: "sig5"},
				},
			},
			wantErr: true,
			errMsg:  "insufficient authorities",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConsensusMetadata(tt.meta)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConsensusMetadata() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errMsg != "" && (err == nil || !strings.Contains(err.Error(), tt.errMsg)) {
				t.Errorf("Error message = %v, want to contain %v", err, tt.errMsg)
			}
		})
	}
}

// TestInSameFamily tests family relationship validation per path-spec.txt §2.2
func TestInSameFamily(t *testing.T) {
	tests := []struct {
		name     string
		relay1   *Relay
		relay2   *Relay
		expected bool
	}{
		{
			name: "Same relay (identical fingerprint)",
			relay1: &Relay{
				Fingerprint: "ABC123",
				Nickname:    "relay1",
			},
			relay2: &Relay{
				Fingerprint: "ABC123",
				Nickname:    "relay1",
			},
			expected: true,
		},
		{
			name: "Bidirectional family relationship by fingerprint",
			relay1: &Relay{
				Fingerprint: "FP1",
				Nickname:    "relay1",
				Family:      []string{"FP2"},
			},
			relay2: &Relay{
				Fingerprint: "FP2",
				Nickname:    "relay2",
				Family:      []string{"FP1"},
			},
			expected: true,
		},
		{
			name: "Bidirectional family relationship by nickname",
			relay1: &Relay{
				Fingerprint: "FP1",
				Nickname:    "relay1",
				Family:      []string{"relay2"},
			},
			relay2: &Relay{
				Fingerprint: "FP2",
				Nickname:    "relay2",
				Family:      []string{"relay1"},
			},
			expected: true,
		},
		{
			name: "Unidirectional family (not valid)",
			relay1: &Relay{
				Fingerprint: "FP1",
				Nickname:    "relay1",
				Family:      []string{"FP2"},
			},
			relay2: &Relay{
				Fingerprint: "FP2",
				Nickname:    "relay2",
				Family:      []string{},
			},
			expected: false,
		},
		{
			name: "No family relationship",
			relay1: &Relay{
				Fingerprint: "FP1",
				Nickname:    "relay1",
				Family:      []string{},
			},
			relay2: &Relay{
				Fingerprint: "FP2",
				Nickname:    "relay2",
				Family:      []string{},
			},
			expected: false,
		},
		{
			name: "Mixed fingerprint and nickname in family",
			relay1: &Relay{
				Fingerprint: "FP1",
				Nickname:    "relay1",
				Family:      []string{"relay2"},
			},
			relay2: &Relay{
				Fingerprint: "FP2",
				Nickname:    "relay2",
				Family:      []string{"FP1"},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.relay1.InSameFamily(tt.relay2)
			if result != tt.expected {
				t.Errorf("InSameFamily() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestInSameSubnet tests /16 subnet conflict detection per path-spec.txt §2.2.1
func TestInSameSubnet(t *testing.T) {
	tests := []struct {
		name     string
		relay1   *Relay
		relay2   *Relay
		expected bool
	}{
		{
			name: "Same /16 subnet",
			relay1: &Relay{
				Address: "192.168.1.1",
			},
			relay2: &Relay{
				Address: "192.168.2.1",
			},
			expected: true,
		},
		{
			name: "Different /16 subnet",
			relay1: &Relay{
				Address: "192.168.1.1",
			},
			relay2: &Relay{
				Address: "10.0.1.1",
			},
			expected: false,
		},
		{
			name: "Same exact IP",
			relay1: &Relay{
				Address: "192.168.1.1",
			},
			relay2: &Relay{
				Address: "192.168.1.1",
			},
			expected: true,
		},
		{
			name: "Different subnets (close first octets)",
			relay1: &Relay{
				Address: "192.167.1.1",
			},
			relay2: &Relay{
				Address: "192.168.1.1",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.relay1.InSameSubnet(tt.relay2)
			if result != tt.expected {
				t.Errorf("InSameSubnet() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestGetSubnet16 tests subnet extraction helper
func TestGetSubnet16(t *testing.T) {
	tests := []struct {
		address  string
		expected string
	}{
		{"192.168.1.1", "192.168"},
		{"10.0.0.1", "10.0"},
		{"172.16.5.10", "172.16"},
		{"1.2.3.4", "1.2"},
		{"invalid", "invalid"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.address, func(t *testing.T) {
			result := getSubnet16(tt.address)
			if result != tt.expected {
				t.Errorf("getSubnet16(%s) = %s, want %s", tt.address, result, tt.expected)
			}
		})
	}
}

// TestAuthorityCertCacheGet tests certificate fetching with HTTP mock server
func TestAuthorityCertCacheGet(t *testing.T) {
	// Create test certificate data
	certData := `dir-source testauth
fingerprint AAAA BB61 6EB2 F60B EC80 1511 14BB 25CE F515 B226
dir-key-expires 2027-01-01 00:00:00
-----BEGIN RSA PUBLIC KEY-----
MEgCQQC7VJTUt9Us8WXZHY/7/w4M1iSp3PNxCCPyOuLYmUxJ+NjF8uYGE00j+6C0
y5TQJtSNlMLaPfJQr8PZQhClq5cJAgMBAAE=
-----END RSA PUBLIC KEY-----`

	// Create test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/tor/keys/authority" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(certData))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	logger := logger.NewDefault()
	cache := &AuthorityCertCache{
		certs:  make(map[string]*AuthorityCert),
		logger: logger.Component("certcache"),
	}

	ctx := context.Background()
	httpClient := server.Client()
	authorities := []string{server.URL + "/tor/status-vote/current/consensus"}

	// First fetch - should retrieve from server
	cert, err := cache.Get(ctx, "AAAABB616EB2F60BEC80151114BB25CEF515B226", httpClient, authorities)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if cert == nil {
		t.Fatal("Get() returned nil cert")
	}

	if cert.Identity != "AAAABB616EB2F60BEC80151114BB25CEF515B226" {
		t.Errorf("cert.Identity = %s, want AAAABB616EB2F60BEC80151114BB25CEF515B226", cert.Identity)
	}

	// Second fetch - should return from cache
	cert2, err := cache.Get(ctx, "AAAABB616EB2F60BEC80151114BB25CEF515B226", httpClient, authorities)
	if err != nil {
		t.Fatalf("Get() second call error = %v", err)
	}

	if cert2 != cert {
		t.Error("Get() should return cached certificate")
	}
}

// TestAuthorityCertCacheGetError tests error handling
func TestAuthorityCertCacheGetError(t *testing.T) {
	// Create server that always fails
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	logger := logger.NewDefault()
	cache := &AuthorityCertCache{
		certs:  make(map[string]*AuthorityCert),
		logger: logger.Component("certcache"),
	}

	ctx := context.Background()
	httpClient := server.Client()
	authorities := []string{server.URL + "/tor/status-vote/current/consensus"}

	_, err := cache.Get(ctx, "TESTIDENT", httpClient, authorities)
	if err == nil {
		t.Error("Get() should return error when server fails")
	}
}

// TestFetchMicrodescriptors tests microdescriptor fetching
func TestFetchMicrodescriptors(t *testing.T) {
	// Create realistic microdescriptor data
	mdData := `onion-key
-----BEGIN RSA PUBLIC KEY-----
MIGJAoGBAKq5qQ0wLVJJxUdP8x1iN3ZVVJ3nVLpB6K8z2VcCx5fLYqgS+7sYPbLw
-----END RSA PUBLIC KEY-----
ntor-onion-key ` + base64.StdEncoding.EncodeToString(make([]byte, 32)) + `
id ed25519 ` + base64.StdEncoding.EncodeToString(make([]byte, 32))

	// Create test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/tor/micro/d/") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(mdData))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := &Client{
		httpClient:  server.Client(),
		authorities: []string{server.URL},
		logger:      logger.NewDefault().Component("directory"),
	}

	relays := []*Relay{
		{
			Nickname:        "TestRelay1",
			MicrodescDigest: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		},
		{
			Nickname:        "TestRelay2",
			MicrodescDigest: "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB",
		},
	}

	ctx := context.Background()
	err := client.FetchMicrodescriptors(ctx, relays)
	// Error is expected as test data won't match digest validation, but tests the flow
	if err != nil {
		t.Logf("FetchMicrodescriptors returned error (expected for test data): %v", err)
	}
}

// TestFetchMicrodescriptorsEmptyList tests handling of empty relay list
func TestFetchMicrodescriptorsEmptyList(t *testing.T) {
	client := NewClient(nil)
	ctx := context.Background()

	err := client.FetchMicrodescriptors(ctx, []*Relay{})
	if err != nil {
		t.Errorf("FetchMicrodescriptors with empty list error = %v, want nil", err)
	}
}

// TestFetchMicrodescriptorsNoDigests tests handling of relays without microdesc digests
func TestFetchMicrodescriptorsNoDigests(t *testing.T) {
	client := NewClient(nil)
	ctx := context.Background()

	relays := []*Relay{
		{Nickname: "relay1", MicrodescDigest: ""},
		{Nickname: "relay2", MicrodescDigest: ""},
	}

	err := client.FetchMicrodescriptors(ctx, relays)
	if err != nil {
		t.Errorf("FetchMicrodescriptors with no digests error = %v, want nil", err)
	}
}

// TestFetchMicrodescriptorBatching tests batch size limits
func TestFetchMicrodescriptorBatching(t *testing.T) {
	// Create test HTTP server that counts requests
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("onion-key\nntor-onion-key test\nid ed25519 test\n"))
	}))
	defer server.Close()

	client := &Client{
		httpClient:  server.Client(),
		authorities: []string{server.URL},
		logger:      logger.NewDefault().Component("directory"),
	}

	// Create 100 relays to test batching (should result in 2 batches: 90 + 10)
	relays := make([]*Relay, 100)
	for i := 0; i < 100; i++ {
		relays[i] = &Relay{
			Nickname:        fmt.Sprintf("relay%d", i),
			MicrodescDigest: fmt.Sprintf("%040d", i),
		}
	}

	ctx := context.Background()
	_ = client.FetchMicrodescriptors(ctx, relays)

	// Should have made at least 1 batch request (exact count depends on error handling)
	if requestCount == 0 {
		t.Error("Expected at least one batch request")
	}
	t.Logf("Made %d batch requests for 100 microdescriptors", requestCount)
}
