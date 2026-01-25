package directory

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/logger"
)

// TestConsensusParsingSpecCompliance_NetworkStatusVersion verifies network-status-version parsing
// per dir-spec.txt §3.4: "network-status-version" SP version NL
// Current version is 3 for consensus documents
func TestConsensusParsingSpecCompliance_NetworkStatusVersion(t *testing.T) {
	tests := []struct {
		name            string
		consensusData   string
		expectedVersion int
	}{
		{
			name:            "valid version 3",
			consensusData:   "network-status-version 3\n",
			expectedVersion: 3,
		},
		{
			name:            "missing version",
			consensusData:   "r nickname AAAA 2024-01-01 12:00:00 192.0.2.1 443 80\n",
			expectedVersion: 0, // Not parsed, should be 0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(logger.NewDefault())
			_, metadata, err := client.parseConsensusWithMetadata(strings.NewReader(tt.consensusData))
			if err != nil {
				t.Fatalf("parseConsensusWithMetadata failed: %v", err)
			}

			if metadata.NetworkStatusVersion != tt.expectedVersion {
				t.Errorf("NetworkStatusVersion = %d, want %d", metadata.NetworkStatusVersion, tt.expectedVersion)
			}
		})
	}
}

// TestConsensusParsingSpecCompliance_TimestampFormat verifies timestamp parsing
// per dir-spec.txt §3.4: Timestamps use format "YYYY-MM-DD HH:MM:SS"
// Three required timestamps: valid-after, fresh-until, valid-until
func TestConsensusParsingSpecCompliance_TimestampFormat(t *testing.T) {
	tests := []struct {
		name          string
		consensusData string
		checkField    func(*ConsensusMetadata) time.Time
		expectedTime  time.Time
	}{
		{
			name:          "valid-after timestamp",
			consensusData: "valid-after 2024-01-15 12:00:00\n",
			checkField:    func(m *ConsensusMetadata) time.Time { return m.ValidAfter },
			expectedTime:  time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
		},
		{
			name:          "fresh-until timestamp",
			consensusData: "fresh-until 2024-01-15 13:00:00\n",
			checkField:    func(m *ConsensusMetadata) time.Time { return m.FreshUntil },
			expectedTime:  time.Date(2024, 1, 15, 13, 0, 0, 0, time.UTC),
		},
		{
			name:          "valid-until timestamp",
			consensusData: "valid-until 2024-01-15 15:00:00\n",
			checkField:    func(m *ConsensusMetadata) time.Time { return m.ValidUntil },
			expectedTime:  time.Date(2024, 1, 15, 15, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(logger.NewDefault())
			_, metadata, err := client.parseConsensusWithMetadata(strings.NewReader(tt.consensusData))
			if err != nil {
				t.Fatalf("parseConsensusWithMetadata failed: %v", err)
			}

			actualTime := tt.checkField(metadata)
			if !actualTime.Equal(tt.expectedTime) {
				t.Errorf("Timestamp = %v, want %v", actualTime, tt.expectedTime)
			}
		})
	}
}

// TestConsensusParsingSpecCompliance_RelayEntryFormat verifies "r" line parsing
// per dir-spec.txt §3.4.1: Two formats supported:
// 1. Regular consensus (9 fields): r nickname identity digest published IP ORPort DirPort
// 2. Microdescriptor consensus (8 fields): r nickname identity published IP ORPort DirPort
func TestConsensusParsingSpecCompliance_RelayEntryFormat(t *testing.T) {
	tests := []struct {
		name             string
		rLine            string
		expectedNickname string
		expectedAddress  string
		expectedORPort   int
		expectedDirPort  int
	}{
		{
			name:             "regular consensus format (9 fields)",
			rLine:            "r TestRelay AAAA BBBB 2024-01-15 12:00:00 192.0.2.1 443 80",
			expectedNickname: "TestRelay",
			expectedAddress:  "192.0.2.1",
			expectedORPort:   443,
			expectedDirPort:  80,
		},
		{
			name:             "microdescriptor consensus format (8 fields)",
			rLine:            "r TestRelay AAAA 2024-01-15 12:00:00 192.0.2.2 9001 9030",
			expectedNickname: "TestRelay",
			expectedAddress:  "192.0.2.2",
			expectedORPort:   9001,
			expectedDirPort:  9030,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(logger.NewDefault())
			relays, _, err := client.parseConsensusWithMetadata(strings.NewReader(tt.rLine + "\n"))
			if err != nil {
				t.Fatalf("parseConsensusWithMetadata failed: %v", err)
			}

			if len(relays) != 1 {
				t.Fatalf("Expected 1 relay, got %d", len(relays))
			}

			relay := relays[0]
			if relay.Nickname != tt.expectedNickname {
				t.Errorf("Nickname = %s, want %s", relay.Nickname, tt.expectedNickname)
			}
			if relay.Address != tt.expectedAddress {
				t.Errorf("Address = %s, want %s", relay.Address, tt.expectedAddress)
			}
			if relay.ORPort != tt.expectedORPort {
				t.Errorf("ORPort = %d, want %d", relay.ORPort, tt.expectedORPort)
			}
			if relay.DirPort != tt.expectedDirPort {
				t.Errorf("DirPort = %d, want %d", relay.DirPort, tt.expectedDirPort)
			}
		})
	}
}

// TestConsensusParsingSpecCompliance_FlagsLine verifies "s" line parsing
// per dir-spec.txt §3.4.1: "s" SP Flags NL
// Flags are space-separated: Authority, BadExit, Exit, Fast, Guard, HSDir, Running, Stable, V2Dir, Valid
func TestConsensusParsingSpecCompliance_FlagsLine(t *testing.T) {
	tests := []struct {
		name          string
		consensusData string
		expectedFlags []string
	}{
		{
			name: "guard relay flags",
			consensusData: `r TestGuard AAAA 2024-01-15 12:00:00 192.0.2.1 443 80
s Fast Guard Running Stable Valid
`,
			expectedFlags: []string{"Fast", "Guard", "Running", "Stable", "Valid"},
		},
		{
			name: "exit relay flags",
			consensusData: `r TestExit AAAA 2024-01-15 12:00:00 192.0.2.2 443 80
s Exit Fast Running Stable Valid
`,
			expectedFlags: []string{"Exit", "Fast", "Running", "Stable", "Valid"},
		},
		{
			name: "no flags",
			consensusData: `r TestRelay AAAA 2024-01-15 12:00:00 192.0.2.3 443 80
s
`,
			expectedFlags: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(logger.NewDefault())
			relays, _, err := client.parseConsensusWithMetadata(strings.NewReader(tt.consensusData))
			if err != nil {
				t.Fatalf("parseConsensusWithMetadata failed: %v", err)
			}

			if len(relays) != 1 {
				t.Fatalf("Expected 1 relay, got %d", len(relays))
			}

			relay := relays[0]
			if len(relay.Flags) != len(tt.expectedFlags) {
				t.Errorf("Flags count = %d, want %d (got %v)", len(relay.Flags), len(tt.expectedFlags), relay.Flags)
			}

			for i, flag := range tt.expectedFlags {
				if i >= len(relay.Flags) || relay.Flags[i] != flag {
					t.Errorf("Flag[%d] = %s, want %s", i, relay.Flags[i], flag)
				}
			}
		})
	}
}

// TestConsensusParsingSpecCompliance_BandwidthWeights verifies "w" line parsing
// per dir-spec.txt §3.4.1 and path-spec.txt §2.2:
// "w" SP "Bandwidth=" INT [SP "Measured=" INT] [SP "Unmeasured=1"] NL
func TestConsensusParsingSpecCompliance_BandwidthWeights(t *testing.T) {
	tests := []struct {
		name              string
		consensusData     string
		expectedBandwidth uint64
	}{
		{
			name: "bandwidth weight only",
			consensusData: `r TestRelay AAAA 2024-01-15 12:00:00 192.0.2.1 443 80
w Bandwidth=12345
`,
			expectedBandwidth: 12345,
		},
		{
			name: "bandwidth with measured",
			consensusData: `r TestRelay AAAA 2024-01-15 12:00:00 192.0.2.1 443 80
w Bandwidth=67890 Measured=65000
`,
			expectedBandwidth: 67890,
		},
		{
			name: "high bandwidth relay",
			consensusData: `r TestRelay AAAA 2024-01-15 12:00:00 192.0.2.1 443 80
w Bandwidth=1000000000
`,
			expectedBandwidth: 1000000000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(logger.NewDefault())
			relays, _, err := client.parseConsensusWithMetadata(strings.NewReader(tt.consensusData))
			if err != nil {
				t.Fatalf("parseConsensusWithMetadata failed: %v", err)
			}

			if len(relays) != 1 {
				t.Fatalf("Expected 1 relay, got %d", len(relays))
			}

			relay := relays[0]
			if relay.Bandwidth != tt.expectedBandwidth {
				t.Errorf("Bandwidth = %d, want %d", relay.Bandwidth, tt.expectedBandwidth)
			}
		})
	}
}

// TestConsensusParsingSpecCompliance_MicrodescriptorDigests verifies "m" and "a" line parsing
// per dir-spec.txt §3.4.1:
// Legacy: "a" SP algname "=" digest NL (e.g., "a sha256=base64digest")
// Modern: "m" SP 32*Base64Character NL (consensus-method 33+)
func TestConsensusParsingSpecCompliance_MicrodescriptorDigests(t *testing.T) {
	tests := []struct {
		name           string
		consensusData  string
		expectedDigest string
	}{
		{
			name: "modern m line format (consensus-method 33)",
			consensusData: `r TestRelay AAAA 2024-01-15 12:00:00 192.0.2.1 443 80
m YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY=
`,
			expectedDigest: "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY=",
		},
		{
			name: "legacy a line format (sha256)",
			consensusData: `r TestRelay AAAA 2024-01-15 12:00:00 192.0.2.1 443 80
a sha256=bGVnYWN5Zm9ybWF0dGVzdGRpZ2VzdDEyMzQ1Njc4OTA=
`,
			expectedDigest: "bGVnYWN5Zm9ybWF0dGVzdGRpZ2VzdDEyMzQ1Njc4OTA=",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(logger.NewDefault())
			relays, _, err := client.parseConsensusWithMetadata(strings.NewReader(tt.consensusData))
			if err != nil {
				t.Fatalf("parseConsensusWithMetadata failed: %v", err)
			}

			if len(relays) != 1 {
				t.Fatalf("Expected 1 relay, got %d", len(relays))
			}

			relay := relays[0]
			if relay.MicrodescDigest != tt.expectedDigest {
				t.Errorf("MicrodescDigest = %s, want %s", relay.MicrodescDigest, tt.expectedDigest)
			}
		})
	}
}

// TestConsensusParsingSpecCompliance_DirectorySignatures verifies signature block parsing
// per dir-spec.txt §3.4: "directory-signature" [algorithm] identity-key-digest signing-key-digest NL
// Followed by base64-encoded signature block
func TestConsensusParsingSpecCompliance_DirectorySignatures(t *testing.T) {
	tests := []struct {
		name               string
		consensusData      string
		expectedSigCount   int
		expectedAlgorithm  string
		expectedIdentity   string
		expectedSigningKey string
	}{
		{
			name: "sha1 signature (2-arg format)",
			consensusData: `directory-signature IDENTITY1234567890ABCDEF123456789 SIGNING1234567890ABCDEF123456789
-----BEGIN SIGNATURE-----
dGVzdHNpZ25hdHVyZWRhdGExMjM0NTY3ODkw
-----END SIGNATURE-----
`,
			expectedSigCount:   1,
			expectedAlgorithm:  "sha1",
			expectedIdentity:   "IDENTITY1234567890ABCDEF123456789",
			expectedSigningKey: "SIGNING1234567890ABCDEF123456789",
		},
		{
			name: "sha256 signature (3-arg format)",
			consensusData: `directory-signature sha256 IDENTITY1234567890ABCDEF123456789 SIGNING1234567890ABCDEF123456789
-----BEGIN SIGNATURE-----
dGVzdHNpZ25hdHVyZWRhdGEyMjIyMjIyMjIyMjI=
-----END SIGNATURE-----
`,
			expectedSigCount:   1,
			expectedAlgorithm:  "sha256",
			expectedIdentity:   "IDENTITY1234567890ABCDEF123456789",
			expectedSigningKey: "SIGNING1234567890ABCDEF123456789",
		},
		{
			name: "multiple signatures",
			consensusData: `directory-signature IDENTITY1 SIGNING1
-----BEGIN SIGNATURE-----
c2lnMQ==
-----END SIGNATURE-----
directory-signature sha256 IDENTITY2 SIGNING2
-----BEGIN SIGNATURE-----
c2lnMg==
-----END SIGNATURE-----
`,
			expectedSigCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(logger.NewDefault())
			_, metadata, err := client.parseConsensusWithMetadata(strings.NewReader(tt.consensusData))
			if err != nil {
				t.Fatalf("parseConsensusWithMetadata failed: %v", err)
			}

			if metadata.SignatureCount != tt.expectedSigCount {
				t.Errorf("SignatureCount = %d, want %d", metadata.SignatureCount, tt.expectedSigCount)
			}

			if len(metadata.Signatures) != tt.expectedSigCount {
				t.Errorf("len(Signatures) = %d, want %d", len(metadata.Signatures), tt.expectedSigCount)
			}

			if tt.expectedSigCount > 0 && tt.expectedAlgorithm != "" {
				sig := metadata.Signatures[0]
				if sig.Algorithm != tt.expectedAlgorithm {
					t.Errorf("Algorithm = %s, want %s", sig.Algorithm, tt.expectedAlgorithm)
				}
				if sig.Identity != tt.expectedIdentity {
					t.Errorf("Identity = %s, want %s", sig.Identity, tt.expectedIdentity)
				}
				if sig.SigningKeyDigest != tt.expectedSigningKey {
					t.Errorf("SigningKeyDigest = %s, want %s", sig.SigningKeyDigest, tt.expectedSigningKey)
				}
			}
		})
	}
}

// TestConsensusParsingSpecCompliance_ConsensusParams verifies "params" line parsing
// per dir-spec.txt §3.4.1: "params" SP key=value SP key=value ... NL
// Network-wide configuration parameters
func TestConsensusParsingSpecCompliance_ConsensusParams(t *testing.T) {
	tests := []struct {
		name           string
		consensusData  string
		expectedParams map[string]int
	}{
		{
			name: "single parameter",
			consensusData: `params circwindow=1000
`,
			expectedParams: map[string]int{"circwindow": 1000},
		},
		{
			name: "multiple parameters",
			consensusData: `params circwindow=1000 min_paths_for_circs_pct=60 UseOptimisticData=1
`,
			expectedParams: map[string]int{
				"circwindow":               1000,
				"min_paths_for_circs_pct":  60,
				"UseOptimisticData":        1,
			},
		},
		{
			name: "padding parameters",
			consensusData: `params nf_ito_low=1500 nf_ito_high=9500 circpad_padding_disabled=0
`,
			expectedParams: map[string]int{
				"nf_ito_low":                1500,
				"nf_ito_high":               9500,
				"circpad_padding_disabled":  0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(logger.NewDefault())
			_, metadata, err := client.parseConsensusWithMetadata(strings.NewReader(tt.consensusData))
			if err != nil {
				t.Fatalf("parseConsensusWithMetadata failed: %v", err)
			}

			if len(metadata.Params) != len(tt.expectedParams) {
				t.Errorf("Params count = %d, want %d", len(metadata.Params), len(tt.expectedParams))
			}

			for key, expectedValue := range tt.expectedParams {
				if actualValue, ok := metadata.Params[key]; !ok {
					t.Errorf("Missing parameter %s", key)
				} else if actualValue != expectedValue {
					t.Errorf("Params[%s] = %d, want %d", key, actualValue, expectedValue)
				}
			}
		})
	}
}

// TestConsensusParsingSpecCompliance_MalformedEntryRejection verifies rejection threshold
// per SEC-004: Reject consensus if >10% of entries are malformed
func TestConsensusParsingSpecCompliance_MalformedEntryRejection(t *testing.T) {
	// Create consensus with 100 entries, 15 malformed (>10% threshold)
	var consensusData strings.Builder
	for i := 0; i < 85; i++ {
		consensusData.WriteString("r TestRelay AAAA 2024-01-15 12:00:00 192.0.2.1 443 80\n")
	}
	for i := 0; i < 15; i++ {
		consensusData.WriteString("r InvalidEntry\n") // Malformed (too few fields)
	}

	client := NewClient(logger.NewDefault())
	_, _, err := client.parseConsensusWithMetadata(strings.NewReader(consensusData.String()))

	if err == nil {
		t.Error("Expected error for excessive malformed entries, got nil")
	}

	if !strings.Contains(err.Error(), "excessive malformed entries") {
		t.Errorf("Expected 'excessive malformed entries' error, got: %v", err)
	}
}

// TestConsensusMetadataValidation_RequiredFields verifies metadata validation
// per dir-spec.txt §3.4: All consensus documents must have timestamps and signatures
func TestConsensusMetadataValidation_RequiredFields(t *testing.T) {
	tests := []struct {
		name        string
		metadata    *ConsensusMetadata
		expectError bool
		errorSubstr string
	}{
		{
			name: "valid metadata",
			metadata: &ConsensusMetadata{
				ValidAfter:     time.Now().Add(-1 * time.Hour),
				ValidUntil:     time.Now().Add(2 * time.Hour),
				SignatureCount: 6,
				AuthorityCount: 9,
				Signatures: []*ConsensusSignature{
					{Algorithm: "sha256", Identity: "ID1", Signature: "sig1"},
					{Algorithm: "sha256", Identity: "ID2", Signature: "sig2"},
					{Algorithm: "sha256", Identity: "ID3", Signature: "sig3"},
					{Algorithm: "sha256", Identity: "ID4", Signature: "sig4"},
					{Algorithm: "sha256", Identity: "ID5", Signature: "sig5"},
					{Algorithm: "sha256", Identity: "ID6", Signature: "sig6"},
				},
			},
			expectError: false,
		},
		{
			name: "missing timestamps",
			metadata: &ConsensusMetadata{
				SignatureCount: 5,
				AuthorityCount: 9,
			},
			expectError: true,
			errorSubstr: "missing required timestamp",
		},
		{
			name: "insufficient signatures",
			metadata: &ConsensusMetadata{
				ValidAfter:     time.Now().Add(-1 * time.Hour),
				ValidUntil:     time.Now().Add(2 * time.Hour),
				SignatureCount: 3,
				AuthorityCount: 9,
			},
			expectError: true,
			errorSubstr: "insufficient signatures",
		},
		{
			name: "insufficient authorities",
			metadata: &ConsensusMetadata{
				ValidAfter:     time.Now().Add(-1 * time.Hour),
				ValidUntil:     time.Now().Add(2 * time.Hour),
				SignatureCount: 5,
				AuthorityCount: 3,
			},
			expectError: true,
			errorSubstr: "insufficient authorities",
		},
		{
			name: "expired consensus",
			metadata: &ConsensusMetadata{
				ValidAfter:     time.Now().Add(-5 * time.Hour),
				ValidUntil:     time.Now().Add(-1 * time.Hour),
				SignatureCount: 5,
				AuthorityCount: 9,
			},
			expectError: true,
			errorSubstr: "expired",
		},
		{
			name: "future consensus (clock skew)",
			metadata: &ConsensusMetadata{
				ValidAfter:     time.Now().Add(1 * time.Hour),
				ValidUntil:     time.Now().Add(5 * time.Hour),
				SignatureCount: 5,
				AuthorityCount: 9,
			},
			expectError: true,
			errorSubstr: "too far in the future",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConsensusMetadata(tt.metadata)

			if tt.expectError && err == nil {
				t.Error("Expected error, got nil")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if tt.expectError && tt.errorSubstr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errorSubstr) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errorSubstr, err)
				}
			}
		})
	}
}

// TestConsensusParsingSpecCompliance_PaddingParams verifies padding parameter extraction
// per padding-spec.txt: Padding parameters are encoded in consensus params
func TestConsensusParsingSpecCompliance_PaddingParams(t *testing.T) {
	tests := []struct {
		name          string
		params        map[string]int
		expectedField func(*PaddingParams) int
		expectedValue int
	}{
		{
			name:          "APE gap minimum",
			params:        map[string]int{"nf_ito_low": 2000},
			expectedField: func(p *PaddingParams) int { return p.APEGapMinMS },
			expectedValue: 2000,
		},
		{
			name:          "APE gap maximum",
			params:        map[string]int{"nf_ito_high": 10000},
			expectedField: func(p *PaddingParams) int { return p.APEGapMaxMS },
			expectedValue: 10000,
		},
		{
			name:          "padding disabled",
			params:        map[string]int{"circpad_padding_disabled": 1},
			expectedField: func(p *PaddingParams) int { if p.PaddingDisabled { return 1 }; return 0 },
			expectedValue: 1,
		},
		{
			name:          "default values (no params)",
			params:        map[string]int{},
			expectedField: func(p *PaddingParams) int { return p.APEGapMinMS },
			expectedValue: 1500, // Default from spec
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata := &ConsensusMetadata{
				Params: tt.params,
			}

			paddingParams := GetPaddingParams(metadata)
			actualValue := tt.expectedField(paddingParams)

			if actualValue != tt.expectedValue {
				t.Errorf("PaddingParam = %d, want %d", actualValue, tt.expectedValue)
			}
		})
	}
}

// TestConsensusParsingSpecCompliance_CompleteConsensus verifies parsing of a minimal complete consensus
func TestConsensusParsingSpecCompliance_CompleteConsensus(t *testing.T) {
	consensusData := `network-status-version 3
valid-after 2024-01-15 12:00:00
fresh-until 2024-01-15 13:00:00
valid-until 2024-01-15 15:00:00
params circwindow=1000 min_paths_for_circs_pct=60
r TestGuard1 AAAA 2024-01-15 12:00:00 192.0.2.1 443 80
s Fast Guard Running Stable Valid
w Bandwidth=50000
m YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY=
r TestExit1 BBBB 2024-01-15 12:00:00 192.0.2.2 443 80
s Exit Fast Running Stable Valid
w Bandwidth=75000
m ZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY3ODkw
directory-signature sha256 TESTIDENTITY1234567890ABCDEF12345 TESTSIGNING1234567890ABCDEF12345
-----BEGIN SIGNATURE-----
dGVzdHNpZ25hdHVyZWRhdGExMjM0NTY3ODkw
-----END SIGNATURE-----
`

	client := NewClient(logger.NewDefault())
	relays, metadata, err := client.parseConsensusWithMetadata(strings.NewReader(consensusData))
	if err != nil {
		t.Fatalf("parseConsensusWithMetadata failed: %v", err)
	}

	// Verify relay count
	if len(relays) != 2 {
		t.Errorf("len(relays) = %d, want 2", len(relays))
	}

	// Verify metadata
	if metadata.NetworkStatusVersion != 3 {
		t.Errorf("NetworkStatusVersion = %d, want 3", metadata.NetworkStatusVersion)
	}

	if metadata.SignatureCount != 1 {
		t.Errorf("SignatureCount = %d, want 1", metadata.SignatureCount)
	}

	if len(metadata.Params) != 2 {
		t.Errorf("len(Params) = %d, want 2", len(metadata.Params))
	}

	// Verify relay details
	guard := relays[0]
	if guard.Nickname != "TestGuard1" {
		t.Errorf("guard.Nickname = %s, want TestGuard1", guard.Nickname)
	}
	if !guard.HasFlag("Guard") {
		t.Error("guard should have Guard flag")
	}
	if guard.Bandwidth != 50000 {
		t.Errorf("guard.Bandwidth = %d, want 50000", guard.Bandwidth)
	}

	exit := relays[1]
	if exit.Nickname != "TestExit1" {
		t.Errorf("exit.Nickname = %s, want TestExit1", exit.Nickname)
	}
	if !exit.HasFlag("Exit") {
		t.Error("exit should have Exit flag")
	}
	if exit.Bandwidth != 75000 {
		t.Errorf("exit.Bandwidth = %d, want 75000", exit.Bandwidth)
	}
}

// Note: Tests for isKnownAuthority, getAuthorityName, and extractSignatureData
// are already present in directory_test.go and are not duplicated here

// TestVerifyConsensusSignatures_Integration verifies signature verification workflow
// Note: This is a unit test for the error paths; integration tests with real signatures
// would require fetching actual consensus documents from directory authorities
func TestVerifyConsensusSignatures_ErrorCases(t *testing.T) {
	ctx := context.Background()
	client := NewClient(logger.NewDefault())

	tests := []struct {
		name          string
		consensusBody []byte
		metadata      *ConsensusMetadata
		expectError   bool
		errorSubstr   string
	}{
		{
			name:          "empty consensus body",
			consensusBody: []byte{},
			metadata:      &ConsensusMetadata{},
			expectError:   true,
			errorSubstr:   "empty consensus body",
		},
		{
			name:          "no signatures",
			consensusBody: []byte("test consensus body"),
			metadata: &ConsensusMetadata{
				Signatures: []*ConsensusSignature{},
			},
			expectError: true,
			errorSubstr: "no signatures",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.VerifyConsensusSignatures(ctx, tt.consensusBody, tt.metadata)

			if tt.expectError && err == nil {
				t.Error("Expected error, got nil")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if tt.expectError && tt.errorSubstr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errorSubstr) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errorSubstr, err)
				}
			}
		})
	}
}
