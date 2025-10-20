package protocol

import (
	"testing"
	"time"

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
		name   string
		check  func() bool
		errMsg string
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
			name: "preferred_in_range",
			check: func() bool {
				return PreferredVersion >= MinLinkProtocolVersion && PreferredVersion <= MaxLinkProtocolVersion
			},
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

// SEC-L002: Additional tests to improve protocol package coverage to 70%+

func TestSetTimeout(t *testing.T) {
	h := NewHandshake(nil, nil)

	// Test default timeout
	if h.timeout != DefaultHandshakeTimeout {
		t.Errorf("Expected default timeout %v, got %v", DefaultHandshakeTimeout, h.timeout)
	}

	// Set custom timeout
	customTimeout := 5 * time.Second
	h.SetTimeout(customTimeout)

	if h.timeout != customTimeout {
		t.Errorf("Expected timeout %v after SetTimeout, got %v", customTimeout, h.timeout)
	}
}

func TestHandshakeTimeout(t *testing.T) {
	// Verify the default timeout constant
	if DefaultHandshakeTimeout != 10*time.Second {
		t.Errorf("Expected DefaultHandshakeTimeout of 10s, got %v", DefaultHandshakeTimeout)
	}
}

func TestSelectVersionExtraCases(t *testing.T) {
	h := &Handshake{}

	tests := []struct {
		name           string
		remoteVersions []int
		expected       int
		description    string
	}{
		{
			name:           "nil_versions",
			remoteVersions: nil,
			expected:       0,
			description:    "Should handle nil version list",
		},
		{
			name:           "very_large_versions",
			remoteVersions: []int{100, 200, 300},
			expected:       0,
			description:    "Should reject versions far above supported range",
		},
		{
			name:           "negative_versions",
			remoteVersions: []int{-1, -2},
			expected:       0,
			description:    "Should reject negative versions",
		},
		{
			name:           "zero_version",
			remoteVersions: []int{0},
			expected:       0,
			description:    "Should reject version 0",
		},
		{
			name:           "exact_range_match",
			remoteVersions: []int{MinLinkProtocolVersion, MaxLinkProtocolVersion},
			expected:       MaxLinkProtocolVersion,
			description:    "Should prefer max version when both endpoints supported",
		},
		{
			name:           "gap_in_versions",
			remoteVersions: []int{3, 5}, // Missing 4
			expected:       5,
			description:    "Should select highest available even with gaps",
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

func TestNetinfoTimestampEncoding(t *testing.T) {
	// Test timestamp encoding logic (similar to sendNetinfo)
	testTime := time.Unix(1234567890, 0) // Known timestamp
	timestamp := uint32(testTime.Unix())

	// Encode timestamp (big-endian)
	payload := make([]byte, 4)
	payload[0] = byte(timestamp >> 24)
	payload[1] = byte(timestamp >> 16)
	payload[2] = byte(timestamp >> 8)
	payload[3] = byte(timestamp)

	// Decode timestamp
	decoded := uint32(payload[0])<<24 | uint32(payload[1])<<16 | uint32(payload[2])<<8 | uint32(payload[3])

	if decoded != timestamp {
		t.Errorf("Timestamp encoding/decoding mismatch: encoded %d, decoded %d", timestamp, decoded)
	}
}

func TestVersionNegotiationLogic(t *testing.T) {
	h := &Handshake{}

	// Test that we prefer the highest mutual version
	scenarios := []struct {
		remote   []int
		expected int
	}{
		{[]int{3, 4, 5, 6}, MaxLinkProtocolVersion}, // Select our max
		{[]int{1, 2, 3}, MinLinkProtocolVersion},    // Select our min
		{[]int{4}, PreferredVersion},                // Select preferred if available
		{[]int{3, 5}, MaxLinkProtocolVersion},       // Skip 4 if not in remote
		{[]int{1, 2, 6, 7}, 0},                      // No mutual version
	}

	for i, sc := range scenarios {
		got := h.selectVersion(sc.remote)
		if got != sc.expected {
			t.Errorf("Scenario %d: selectVersion(%v) = %d, want %d", i, sc.remote, got, sc.expected)
		}
	}
}

func TestHandshakeInitialization(t *testing.T) {
	log := logger.NewDefault()
	h := NewHandshake(nil, log)

	// Verify all fields are properly initialized
	if h == nil {
		t.Fatal("NewHandshake returned nil")
	}

	if h.logger != log {
		t.Error("Logger not set correctly")
	}

	if h.timeout != DefaultHandshakeTimeout {
		t.Errorf("Expected default timeout %v, got %v", DefaultHandshakeTimeout, h.timeout)
	}

	if h.negotiatedVersion != 0 {
		t.Errorf("Expected initial negotiatedVersion 0, got %d", h.negotiatedVersion)
	}

	if h.conn != nil {
		t.Error("Connection should be nil when passed nil")
	}
}

func TestVersionRangeValidation(t *testing.T) {
	// Ensure our version constants make sense
	if MinLinkProtocolVersion != 3 {
		t.Errorf("MinLinkProtocolVersion should be 3 (per Tor spec), got %d", MinLinkProtocolVersion)
	}

	if MaxLinkProtocolVersion != 5 {
		t.Errorf("MaxLinkProtocolVersion should be 5 (current implementation), got %d", MaxLinkProtocolVersion)
	}

	if PreferredVersion != 4 {
		t.Errorf("PreferredVersion should be 4 (4-byte CircID), got %d", PreferredVersion)
	}

	// Verify logical consistency
	if MinLinkProtocolVersion > MaxLinkProtocolVersion {
		t.Error("MinLinkProtocolVersion > MaxLinkProtocolVersion")
	}

	if PreferredVersion < MinLinkProtocolVersion || PreferredVersion > MaxLinkProtocolVersion {
		t.Error("PreferredVersion not in valid range")
	}
}

func TestPayloadLengthValidation(t *testing.T) {
	// Test odd-length payload (invalid for VERSIONS cell)
	oddPayload := []byte{0x00, 0x03, 0x00} // 3 bytes - invalid

	versions := []int{}
	for i := 0; i < len(oddPayload)/2; i++ {
		version := int(oddPayload[i*2])<<8 | int(oddPayload[i*2+1])
		versions = append(versions, version)
	}

	// Should only decode complete version pairs
	expectedCount := len(oddPayload) / 2
	if len(versions) != expectedCount {
		t.Errorf("Expected %d versions from %d-byte payload, got %d", expectedCount, len(oddPayload), len(versions))
	}
}

func TestMultipleTimeoutSettings(t *testing.T) {
	h := NewHandshake(nil, nil)

	timeouts := []time.Duration{
		1 * time.Second,
		5 * time.Second,
		10 * time.Second,
		30 * time.Second,
		1 * time.Minute,
	}

	for _, timeout := range timeouts {
		h.SetTimeout(timeout)
		if h.timeout != timeout {
			t.Errorf("SetTimeout(%v) failed, got %v", timeout, h.timeout)
		}
	}
}

func TestZeroTimeout(t *testing.T) {
	h := NewHandshake(nil, nil)

	// Setting zero timeout should be allowed (caller's responsibility to use valid values)
	h.SetTimeout(0)
	if h.timeout != 0 {
		t.Errorf("Expected timeout 0, got %v", h.timeout)
	}
}

func TestNegativeVersionSelection(t *testing.T) {
	h := &Handshake{}

	// Negative versions should never be selected
	result := h.selectVersion([]int{-1, -5, -10})
	if result != 0 {
		t.Errorf("Expected 0 for negative versions, got %d", result)
	}
}
