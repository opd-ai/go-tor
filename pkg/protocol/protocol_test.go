package protocol

import (
	"testing"

	"github.com/opd-ai/go-tor/pkg/connection"
	"github.com/opd-ai/go-tor/pkg/logger"
)

func TestVersionConstants(t *testing.T) {
	if MinLinkProtocolVersion < 1 {
		t.Errorf("MinLinkProtocolVersion = %d, should be >= 1", MinLinkProtocolVersion)
	}
	if MaxLinkProtocolVersion < MinLinkProtocolVersion {
		t.Errorf("MaxLinkProtocolVersion (%d) < MinLinkProtocolVersion (%d)",
			MaxLinkProtocolVersion, MinLinkProtocolVersion)
	}
	if PreferredVersion < MinLinkProtocolVersion || PreferredVersion > MaxLinkProtocolVersion {
		t.Errorf("PreferredVersion (%d) not in range [%d, %d]",
			PreferredVersion, MinLinkProtocolVersion, MaxLinkProtocolVersion)
	}
}

func TestSelectVersion(t *testing.T) {
	h := &Handshake{}

	tests := []struct {
		name           string
		remoteVersions []int
		expected       int
	}{
		{
			name:           "exact_match_preferred",
			remoteVersions: []int{3, 4, 5},
			expected:       5, // Should select highest
		},
		{
			name:           "only_min_version",
			remoteVersions: []int{3},
			expected:       3,
		},
		{
			name:           "no_compatible_version",
			remoteVersions: []int{1, 2},
			expected:       0,
		},
		{
			name:           "mixed_versions",
			remoteVersions: []int{2, 4, 6},
			expected:       4,
		},
		{
			name:           "empty_versions",
			remoteVersions: []int{},
			expected:       0,
		},
		{
			name:           "highest_version_first",
			remoteVersions: []int{5, 4, 3},
			expected:       5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := h.selectVersion(tt.remoteVersions)
			if got != tt.expected {
				t.Errorf("selectVersion(%v) = %d, want %d", tt.remoteVersions, got, tt.expected)
			}
		})
	}
}

func TestNewHandshake(t *testing.T) {
	// Test with nil logger
	h := NewHandshake(nil, nil)
	if h == nil {
		t.Fatal("NewHandshake() returned nil")
	}
	if h.logger == nil {
		t.Error("logger should be initialized with default")
	}
}

func TestNegotiatedVersion(t *testing.T) {
	h := &Handshake{
		negotiatedVersion: 4,
	}

	if got := h.NegotiatedVersion(); got != 4 {
		t.Errorf("NegotiatedVersion() = %d, want 4", got)
	}
}

func TestSelectVersionAdditionalCases(t *testing.T) {
	h := &Handshake{}

	tests := []struct {
		name           string
		remoteVersions []int
		expected       int
		description    string
	}{
		{
			name:           "versions_above_max",
			remoteVersions: []int{6, 7, 8},
			expected:       0,
			description:    "No compatible version when remote only supports versions above our max",
		},
		{
			name:           "versions_below_min",
			remoteVersions: []int{1, 2},
			expected:       0,
			description:    "No compatible version when remote only supports versions below our min",
		},
		{
			name:           "single_compatible",
			remoteVersions: []int{1, 2, 3, 6, 7},
			expected:       3,
			description:    "Should select version 3 when it's the only compatible version",
		},
		{
			name:           "prefer_highest",
			remoteVersions: []int{3, 4, 5, 6},
			expected:       5,
			description:    "Should prefer highest compatible version",
		},
		{
			name:           "unordered_versions",
			remoteVersions: []int{5, 3, 4},
			expected:       5,
			description:    "Should handle unordered version lists correctly",
		},
		{
			name:           "duplicate_versions",
			remoteVersions: []int{4, 4, 4, 3, 3},
			expected:       4,
			description:    "Should handle duplicate versions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := h.selectVersion(tt.remoteVersions)
			if got != tt.expected {
				t.Errorf("selectVersion(%v) = %d, want %d: %s",
					tt.remoteVersions, got, tt.expected, tt.description)
			}
		})
	}
}

func TestProtocolConstantsValidation(t *testing.T) {
	tests := []struct {
		name     string
		check    func() bool
		errMsg   string
	}{
		{
			name:   "min_version_positive",
			check:  func() bool { return MinLinkProtocolVersion > 0 },
			errMsg: "MinLinkProtocolVersion must be positive",
		},
		{
			name:   "max_ge_min",
			check:  func() bool { return MaxLinkProtocolVersion >= MinLinkProtocolVersion },
			errMsg: "MaxLinkProtocolVersion must be >= MinLinkProtocolVersion",
		},
		{
			name:   "preferred_in_range",
			check:  func() bool { return PreferredVersion >= MinLinkProtocolVersion && PreferredVersion <= MaxLinkProtocolVersion },
			errMsg: "PreferredVersion must be in supported range",
		},
		{
			name:   "min_is_3",
			check:  func() bool { return MinLinkProtocolVersion == 3 },
			errMsg: "MinLinkProtocolVersion should be 3 per Tor spec",
		},
		{
			name:   "max_is_5",
			check:  func() bool { return MaxLinkProtocolVersion == 5 },
			errMsg: "MaxLinkProtocolVersion should be 5 per current implementation",
		},
		{
			name:   "preferred_is_4",
			check:  func() bool { return PreferredVersion == 4 },
			errMsg: "PreferredVersion should be 4 (uses 4-byte circuit IDs)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.check() {
				t.Error(tt.errMsg)
			}
		})
	}
}

func TestNewHandshakeWithConnection(t *testing.T) {
	// Create a mock connection (nil is OK for this test)
	var conn *connection.Connection = nil
	log := logger.NewDefault()

	h := NewHandshake(conn, log)
	
	if h == nil {
		t.Fatal("NewHandshake returned nil")
	}
	
	if h.conn != conn {
		t.Error("Connection not set correctly")
	}
	
	if h.logger == nil {
		t.Error("Logger not initialized")
	}
	
	if h.negotiatedVersion != 0 {
		t.Errorf("Expected initial negotiatedVersion to be 0, got %d", h.negotiatedVersion)
	}
}

func TestVersionPayloadEncoding(t *testing.T) {
	// Test that version encoding/decoding is consistent
	testVersions := []uint16{3, 4, 5}
	
	// Encode versions (simulating what sendVersions does)
	payload := make([]byte, len(testVersions)*2)
	for i, v := range testVersions {
		payload[i*2] = byte(v >> 8)
		payload[i*2+1] = byte(v)
	}
	
	// Decode versions (simulating what receiveVersions does)
	var decoded []int
	for i := 0; i < len(payload); i += 2 {
		version := int(payload[i])<<8 | int(payload[i+1])
		decoded = append(decoded, version)
	}
	
	// Verify round-trip
	if len(decoded) != len(testVersions) {
		t.Fatalf("Length mismatch: encoded %d versions, decoded %d", len(testVersions), len(decoded))
	}
	
	for i, v := range testVersions {
		if decoded[i] != int(v) {
			t.Errorf("Version mismatch at index %d: encoded %d, decoded %d", i, v, decoded[i])
		}
	}
}
