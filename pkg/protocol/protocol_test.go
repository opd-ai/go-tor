package protocol

import (
	"testing"
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
