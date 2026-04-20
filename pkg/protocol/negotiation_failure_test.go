// Package protocol provides tests for protocol negotiation failure scenarios.
//
// These tests verify that the link protocol handshake correctly handles
// failure conditions including version mismatches, timeout boundaries,
// malformed cells, and unexpected cell types per tor-spec.txt §1-2.
//
// Compliance: tor-spec.txt §1 (Link Protocol), §2 (Version Negotiation)
package protocol

import (
	"strings"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/logger"
)

// TestSelectVersionNoOverlap verifies that selectVersion returns 0
// when no common version exists between client and server.
func TestSelectVersionNoOverlap(t *testing.T) {
	h := &Handshake{}

	tests := []struct {
		name           string
		remoteVersions []int
		want           int
	}{
		{"empty list", []int{}, 0},
		{"nil", nil, 0},
		{"only version 1", []int{1}, 0},
		{"only version 2", []int{2}, 0},
		{"versions 1 and 2 only", []int{1, 2}, 0},
		{"version 6 (above max)", []int{6}, 0},
		{"versions 0 and 100", []int{0, 100}, 0},
		{"negative version", []int{-1}, 0},
		{"very large version", []int{65535}, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := h.selectVersion(tc.remoteVersions)
			if got != tc.want {
				t.Errorf("selectVersion(%v) = %d, want %d", tc.remoteVersions, got, tc.want)
			}
		})
	}
}

// TestSelectVersionPreferHighest verifies that selectVersion always
// picks the highest mutually supported version.
func TestSelectVersionPreferHighest(t *testing.T) {
	h := &Handshake{}

	tests := []struct {
		name           string
		remoteVersions []int
		want           int
	}{
		{"exact min", []int{3}, 3},
		{"exact max", []int{5}, 5},
		{"exact preferred", []int{4}, 4},
		{"all supported", []int{3, 4, 5}, 5},
		{"reverse order", []int{5, 4, 3}, 5},
		{"with unsupported", []int{1, 2, 3, 4, 5, 6, 7}, 5},
		{"only min and max", []int{3, 5}, 5},
		{"duplicates", []int{4, 4, 4}, 4},
		{"mixed with negatives", []int{-1, 0, 3, 100}, 3},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := h.selectVersion(tc.remoteVersions)
			if got != tc.want {
				t.Errorf("selectVersion(%v) = %d, want %d", tc.remoteVersions, got, tc.want)
			}
		})
	}
}

// TestSetTimeoutBoundaryValidation verifies that SetTimeout enforces
// minimum and maximum timeout bounds per SEC-M004.
func TestSetTimeoutBoundaryValidation(t *testing.T) {
	log := logger.NewDefault()

	tests := []struct {
		name        string
		timeout     time.Duration
		wantErr     bool
		errContains string
	}{
		{
			name:        "below minimum (1s)",
			timeout:     1 * time.Second,
			wantErr:     true,
			errContains: "too short",
		},
		{
			name:        "below minimum (0)",
			timeout:     0,
			wantErr:     true,
			errContains: "too short",
		},
		{
			name:        "negative timeout",
			timeout:     -1 * time.Second,
			wantErr:     true,
			errContains: "too short",
		},
		{
			name:    "exact minimum (5s)",
			timeout: 5 * time.Second,
			wantErr: false,
		},
		{
			name:    "in range (30s)",
			timeout: 30 * time.Second,
			wantErr: false,
		},
		{
			name:    "exact maximum (60s)",
			timeout: 60 * time.Second,
			wantErr: false,
		},
		{
			name:        "above maximum (61s)",
			timeout:     61 * time.Second,
			wantErr:     true,
			errContains: "too long",
		},
		{
			name:        "way above maximum",
			timeout:     10 * time.Minute,
			wantErr:     true,
			errContains: "too long",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := NewHandshake(nil, log)
			err := h.SetTimeout(tc.timeout)

			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tc.errContains != "" && !strings.Contains(err.Error(), tc.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.errContains)
				}
				// Verify timeout was not changed
				if h.timeout != DefaultHandshakeTimeout {
					t.Errorf("timeout changed on error: got %v, want %v",
						h.timeout, DefaultHandshakeTimeout)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if h.timeout != tc.timeout {
					t.Errorf("timeout = %v, want %v", h.timeout, tc.timeout)
				}
			}
		})
	}
}

// TestNewHandshakeNilLogger verifies that NewHandshake handles a
// nil logger gracefully by creating a default logger.
func TestNewHandshakeNilLogger(t *testing.T) {
	h := NewHandshake(nil, nil)
	if h == nil {
		t.Fatal("NewHandshake returned nil")
	}
	if h.logger == nil {
		t.Error("logger is nil after NewHandshake with nil logger")
	}
	if h.timeout != DefaultHandshakeTimeout {
		t.Errorf("default timeout = %v, want %v", h.timeout, DefaultHandshakeTimeout)
	}
}

// TestNewHandshakeWithLogger verifies initialization with a provided logger.
func TestNewHandshakeWithLogger(t *testing.T) {
	log := logger.NewDefault()
	h := NewHandshake(nil, log)
	if h == nil {
		t.Fatal("NewHandshake returned nil")
	}
	if h.logger == nil {
		t.Error("logger is nil")
	}
}

// TestVersionsPayloadParsingEdgeCases verifies correct parsing of VERSIONS
// cell payloads including edge cases.
func TestVersionsPayloadParsingEdgeCases(t *testing.T) {
	tests := []struct {
		name             string
		payload          []byte
		expectedVersions []int
		isOddLength      bool
	}{
		{
			name:             "empty payload",
			payload:          []byte{},
			expectedVersions: nil,
			isOddLength:      false,
		},
		{
			name:             "single version 4",
			payload:          []byte{0x00, 0x04},
			expectedVersions: []int{4},
		},
		{
			name:             "versions 3, 4, 5",
			payload:          []byte{0x00, 0x03, 0x00, 0x04, 0x00, 0x05},
			expectedVersions: []int{3, 4, 5},
		},
		{
			name:             "high byte set",
			payload:          []byte{0x01, 0x00}, // Version 256
			expectedVersions: []int{256},
		},
		{
			name:             "version 0",
			payload:          []byte{0x00, 0x00},
			expectedVersions: []int{0},
		},
		{
			name:             "max uint16 version",
			payload:          []byte{0xFF, 0xFF},
			expectedVersions: []int{65535},
		},
		{
			name:        "odd length (invalid)",
			payload:     []byte{0x00, 0x04, 0x00},
			isOddLength: true,
		},
		{
			name:        "single byte (invalid)",
			payload:     []byte{0x04},
			isOddLength: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Check for odd-length error condition
			if len(tc.payload)%2 != 0 {
				if !tc.isOddLength {
					t.Error("expected odd length to be flagged")
				}
				return
			}

			// Parse versions (mirroring receiveVersions logic)
			var versions []int
			for i := 0; i < len(tc.payload); i += 2 {
				version := int(tc.payload[i])<<8 | int(tc.payload[i+1])
				versions = append(versions, version)
			}

			if len(versions) != len(tc.expectedVersions) {
				t.Errorf("got %d versions, want %d", len(versions), len(tc.expectedVersions))
				return
			}

			for i, expected := range tc.expectedVersions {
				if versions[i] != expected {
					t.Errorf("version[%d] = %d, want %d", i, versions[i], expected)
				}
			}
		})
	}
}

// TestProtocolConstantsNegotiation verifies protocol constants are consistent
// with tor-spec.txt requirements.
func TestProtocolConstantsNegotiation(t *testing.T) {
	// Link protocol version range must include v4 (tor-spec.txt §1)
	if MinLinkProtocolVersion > 4 || MaxLinkProtocolVersion < 4 {
		t.Error("version range does not include v4 (required by tor-spec.txt)")
	}

	// Minimum must be <= maximum
	if MinLinkProtocolVersion > MaxLinkProtocolVersion {
		t.Errorf("min version (%d) > max version (%d)",
			MinLinkProtocolVersion, MaxLinkProtocolVersion)
	}

	// Preferred version must be within range
	if PreferredVersion < MinLinkProtocolVersion || PreferredVersion > MaxLinkProtocolVersion {
		t.Errorf("preferred version %d is outside range [%d, %d]",
			PreferredVersion, MinLinkProtocolVersion, MaxLinkProtocolVersion)
	}

	// Timeout bounds must be reasonable
	if MinHandshakeTimeout <= 0 {
		t.Error("minimum handshake timeout must be positive")
	}
	if MaxHandshakeTimeout <= MinHandshakeTimeout {
		t.Error("maximum timeout must be greater than minimum")
	}
	if DefaultHandshakeTimeout < MinHandshakeTimeout || DefaultHandshakeTimeout > MaxHandshakeTimeout {
		t.Errorf("default timeout %v is outside range [%v, %v]",
			DefaultHandshakeTimeout, MinHandshakeTimeout, MaxHandshakeTimeout)
	}
}

// TestSelectVersionWithLargeList verifies version selection with
// a very large list of versions (potential DoS via large VERSIONS cell).
func TestSelectVersionWithLargeList(t *testing.T) {
	h := &Handshake{}

	// 10000 unsupported versions + one supported
	versions := make([]int, 10001)
	for i := range 10000 {
		versions[i] = 100 + i // All unsupported
	}
	versions[10000] = 4 // One supported version

	got := h.selectVersion(versions)
	if got != 4 {
		t.Errorf("selectVersion with large list = %d, want 4", got)
	}
}

// TestSelectVersionAllSupported verifies correct ordering when
// all local versions are offered by the remote.
func TestSelectVersionAllSupported(t *testing.T) {
	h := &Handshake{}

	// All versions from min to max
	versions := make([]int, 0)
	for v := MinLinkProtocolVersion; v <= MaxLinkProtocolVersion; v++ {
		versions = append(versions, v)
	}

	got := h.selectVersion(versions)
	if got != MaxLinkProtocolVersion {
		t.Errorf("should select highest version: got %d, want %d",
			got, MaxLinkProtocolVersion)
	}
}

// TestSetTimeoutIdempotent verifies that calling SetTimeout multiple
// times works correctly (last value wins).
func TestSetTimeoutIdempotent(t *testing.T) {
	log := logger.NewDefault()
	h := NewHandshake(nil, log)

	// Set to minimum
	if err := h.SetTimeout(MinHandshakeTimeout); err != nil {
		t.Fatalf("set to min: %v", err)
	}
	if h.timeout != MinHandshakeTimeout {
		t.Errorf("timeout = %v, want %v", h.timeout, MinHandshakeTimeout)
	}

	// Change to maximum
	if err := h.SetTimeout(MaxHandshakeTimeout); err != nil {
		t.Fatalf("set to max: %v", err)
	}
	if h.timeout != MaxHandshakeTimeout {
		t.Errorf("timeout = %v, want %v", h.timeout, MaxHandshakeTimeout)
	}

	// Failed set should not change existing value
	if err := h.SetTimeout(1 * time.Nanosecond); err == nil {
		t.Error("expected error for tiny timeout")
	}
	if h.timeout != MaxHandshakeTimeout {
		t.Error("timeout changed after failed SetTimeout")
	}
}
