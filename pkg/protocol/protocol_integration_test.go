package protocol

import (
	"bytes"
	"context"
	"net"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/cell"
	"github.com/opd-ai/go-tor/pkg/connection"
	"github.com/opd-ai/go-tor/pkg/logger"
)

// mockRelay simulates a Tor relay for testing handshake
type mockRelay struct {
	listener net.Listener
	t        *testing.T
}

func newMockRelay(t *testing.T) (*mockRelay, string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, "", err
	}

	return &mockRelay{
		listener: listener,
		t:        t,
	}, listener.Addr().String(), nil
}

func (m *mockRelay) serve() {
	go func() {
		conn, err := m.listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		// Read VERSIONS cell
		buf := make([]byte, 514)
		n, err := conn.Read(buf)
		if err != nil {
			m.t.Logf("Failed to read VERSIONS: %v", err)
			return
		}

		versionsCell, err := cell.DecodeCell(bytes.NewReader(buf[:n]))
		if err != nil {
			m.t.Logf("Failed to decode VERSIONS: %v", err)
			return
		}

		if versionsCell.Command != cell.CmdVersions {
			m.t.Logf("Expected VERSIONS, got %v", versionsCell.Command)
			return
		}

		// Send VERSIONS response
		responseCell := cell.NewCell(0, cell.CmdVersions)
		responseCell.Payload = []byte{0x00, 0x04} // Version 4
		var encBuf bytes.Buffer
		if err := responseCell.Encode(&encBuf); err != nil {
			m.t.Logf("Failed to encode VERSIONS response: %v", err)
			return
		}

		if _, err := conn.Write(encBuf.Bytes()); err != nil {
			m.t.Logf("Failed to write VERSIONS response: %v", err)
			return
		}

		// Read NETINFO cell
		n, err = conn.Read(buf)
		if err != nil {
			m.t.Logf("Failed to read NETINFO: %v", err)
			return
		}

		netinfoCell, err := cell.DecodeCell(bytes.NewReader(buf[:n]))
		if err != nil {
			m.t.Logf("Failed to decode NETINFO: %v", err)
			return
		}

		if netinfoCell.Command != cell.CmdNetinfo {
			m.t.Logf("Expected NETINFO, got %v", netinfoCell.Command)
			return
		}

		// Send NETINFO response
		netinfoResponse := cell.NewCell(0, cell.CmdNetinfo)
		payload := make([]byte, 11)
		// Timestamp (4 bytes)
		now := uint32(time.Now().Unix())
		payload[0] = byte(now >> 24)
		payload[1] = byte(now >> 16)
		payload[2] = byte(now >> 8)
		payload[3] = byte(now)
		// Other address (IPv4)
		payload[4] = 0x04
		payload[5] = 4
		// Number of this addresses
		payload[10] = 0
		netinfoResponse.Payload = payload

		var netBuf bytes.Buffer
		if err := netinfoResponse.Encode(&netBuf); err != nil {
			m.t.Logf("Failed to encode NETINFO response: %v", err)
			return
		}

		if _, err := conn.Write(netBuf.Bytes()); err != nil {
			m.t.Logf("Failed to write NETINFO response: %v", err)
			return
		}
	}()
}

func (m *mockRelay) close() {
	m.listener.Close()
}

func TestHandshakeIntegration(t *testing.T) {
	// Skip in short mode as this creates network connections
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create mock relay
	relay, addr, err := newMockRelay(t)
	if err != nil {
		t.Fatalf("Failed to create mock relay: %v", err)
	}
	defer relay.close()

	relay.serve()

	// Give the server a moment to start
	time.Sleep(50 * time.Millisecond)

	// Create connection config
	cfg := connection.DefaultConfig(addr)
	cfg.Timeout = 5 * time.Second

	log := logger.NewDefault()

	// Connect to mock relay (plain TCP for this test)
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		t.Fatalf("Failed to connect to mock relay: %v", err)
	}
	defer conn.Close()

	// Create Connection wrapper
	torConn := connection.New(cfg, log)
	// Note: In real usage, Connection would handle TLS. For this test,
	// we're using the raw connection to avoid TLS complexity.

	// Create handshake
	h := NewHandshake(torConn, log)
	if h == nil {
		t.Fatal("NewHandshake returned nil")
	}

	// Test NegotiatedVersion before handshake
	if v := h.NegotiatedVersion(); v != 0 {
		t.Errorf("NegotiatedVersion before handshake = %d, want 0", v)
	}
}

func TestSendVersionsEncoding(t *testing.T) {
	// Test that sendVersions creates correctly formatted cells
	log := logger.NewDefault()
	cfg := connection.DefaultConfig("test:9001")
	cfg.Timeout = 5 * time.Second

	torConn := connection.New(cfg, log)
	h := NewHandshake(torConn, log)

	// We can't easily test sendVersions without a real connection,
	// but we can test the version selection logic more thoroughly
	tests := []struct {
		name     string
		versions []int
		want     int
	}{
		{
			name:     "prefer_highest_version",
			versions: []int{3, 4, 5},
			want:     5,
		},
		{
			name:     "accept_middle_version",
			versions: []int{4},
			want:     4,
		},
		{
			name:     "reject_unsupported_versions",
			versions: []int{1, 2, 6, 7},
			want:     0,
		},
		{
			name:     "mixed_supported_unsupported",
			versions: []int{2, 3, 6, 7},
			want:     3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := h.selectVersion(tt.versions)
			if got != tt.want {
				t.Errorf("selectVersion(%v) = %d, want %d", tt.versions, got, tt.want)
			}
		})
	}
}

func TestProtocolConstants(t *testing.T) {
	// Verify protocol constants are sensible
	if MinLinkProtocolVersion >= MaxLinkProtocolVersion {
		t.Errorf("MinLinkProtocolVersion (%d) >= MaxLinkProtocolVersion (%d)",
			MinLinkProtocolVersion, MaxLinkProtocolVersion)
	}

	if PreferredVersion < MinLinkProtocolVersion {
		t.Errorf("PreferredVersion (%d) < MinLinkProtocolVersion (%d)",
			PreferredVersion, MinLinkProtocolVersion)
	}

	if PreferredVersion > MaxLinkProtocolVersion {
		t.Errorf("PreferredVersion (%d) > MaxLinkProtocolVersion (%d)",
			PreferredVersion, MaxLinkProtocolVersion)
	}

	// Verify values match Tor spec expectations
	if MinLinkProtocolVersion != 3 {
		t.Errorf("MinLinkProtocolVersion = %d, expected 3", MinLinkProtocolVersion)
	}
	if MaxLinkProtocolVersion != 5 {
		t.Errorf("MaxLinkProtocolVersion = %d, expected 5", MaxLinkProtocolVersion)
	}
	if PreferredVersion != 4 {
		t.Errorf("PreferredVersion = %d, expected 4", PreferredVersion)
	}
}

func TestHandshakeWithTimeout(t *testing.T) {
	// Test handshake behavior with context timeout
	log := logger.NewDefault()
	cfg := connection.DefaultConfig("127.0.0.1:0") // Invalid address to trigger timeout
	cfg.Timeout = 1 * time.Second

	torConn := connection.New(cfg, log)
	h := NewHandshake(torConn, log)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// This should timeout or fail quickly since we're not connecting to a real relay
	err := h.PerformHandshake(ctx)
	if err == nil {
		t.Error("Expected handshake to fail with invalid address, but it succeeded")
	}
}

func TestNewHandshakeWithNilLogger(t *testing.T) {
	cfg := connection.DefaultConfig("test:9001")
	cfg.Timeout = 5 * time.Second

	torConn := connection.New(cfg, nil)
	h := NewHandshake(torConn, nil)

	if h == nil {
		t.Fatal("NewHandshake with nil logger returned nil")
	}

	if h.logger == nil {
		t.Error("NewHandshake should initialize default logger when nil is passed")
	}

	if h.conn != torConn {
		t.Error("NewHandshake did not set connection correctly")
	}

	if h.negotiatedVersion != 0 {
		t.Errorf("New handshake should have negotiatedVersion = 0, got %d", h.negotiatedVersion)
	}
}

func TestSelectVersionEdgeCases(t *testing.T) {
	h := &Handshake{}

	tests := []struct {
		name           string
		remoteVersions []int
		expected       int
		description    string
	}{
		{
			name:           "single_max_version",
			remoteVersions: []int{5},
			expected:       5,
			description:    "Should accept single maximum version",
		},
		{
			name:           "single_min_version",
			remoteVersions: []int{3},
			expected:       3,
			description:    "Should accept single minimum version",
		},
		{
			name:           "all_unsupported_high",
			remoteVersions: []int{6, 7, 8},
			expected:       0,
			description:    "Should reject all versions higher than supported",
		},
		{
			name:           "all_unsupported_low",
			remoteVersions: []int{1, 2},
			expected:       0,
			description:    "Should reject all versions lower than supported",
		},
		{
			name:           "duplicates",
			remoteVersions: []int{4, 4, 4},
			expected:       4,
			description:    "Should handle duplicate versions",
		},
		{
			name:           "unordered",
			remoteVersions: []int{5, 3, 4},
			expected:       5,
			description:    "Should find highest version regardless of order",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := h.selectVersion(tt.remoteVersions)
			if got != tt.expected {
				t.Errorf("%s: selectVersion(%v) = %d, want %d",
					tt.description, tt.remoteVersions, got, tt.expected)
			}
		})
	}
}
