package circuit

import (
	"context"
	"encoding/binary"
	"net"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/cell"
)

// TestParseResolvedCell tests parsing of RELAY_RESOLVED cells
func TestParseResolvedCell(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		wantType  byte
		wantAddrs []string
		wantHost  string
		wantTTL   uint32
		wantError byte
		expectErr bool
	}{
		{
			name: "IPv4 response",
			data: func() []byte {
				// TYPE (0x04) | LENGTH (4) | IPv4 (192.0.2.1) | TTL (3600)
				data := make([]byte, 10)
				data[0] = DNSTypeIPv4
				data[1] = 4
				copy(data[2:6], []byte{192, 0, 2, 1})
				binary.BigEndian.PutUint32(data[6:10], 3600)
				return data
			}(),
			wantType:  DNSTypeIPv4,
			wantAddrs: []string{"192.0.2.1"},
			wantTTL:   3600,
			expectErr: false,
		},
		{
			name: "IPv6 response",
			data: func() []byte {
				// TYPE (0x06) | LENGTH (16) | IPv6 (2001:db8::1) | TTL (7200)
				data := make([]byte, 22)
				data[0] = DNSTypeIPv6
				data[1] = 16
				ip := net.ParseIP("2001:db8::1").To16()
				copy(data[2:18], ip)
				binary.BigEndian.PutUint32(data[18:22], 7200)
				return data
			}(),
			wantType:  DNSTypeIPv6,
			wantAddrs: []string{"2001:db8::1"},
			wantTTL:   7200,
			expectErr: false,
		},
		{
			name: "Hostname response (PTR)",
			data: func() []byte {
				// TYPE (0x00) | LENGTH (15) | "www.example.com\x00" | TTL (1800)
				hostname := "www.example.com\x00"
				data := make([]byte, 2+len(hostname)+4)
				data[0] = DNSTypeHostname
				data[1] = byte(len(hostname))
				copy(data[2:2+len(hostname)], []byte(hostname))
				binary.BigEndian.PutUint32(data[2+len(hostname):], 1800)
				return data
			}(),
			wantType:  DNSTypeHostname,
			wantHost:  "www.example.com",
			wantTTL:   1800,
			expectErr: false,
		},
		{
			name: "Error response - NXDOMAIN",
			data: func() []byte {
				// TYPE (0xF0) | LENGTH (1) | ERROR (0x03 = NXDOMAIN) | TTL (0)
				data := make([]byte, 7)
				data[0] = DNSTypeError
				data[1] = 1
				data[2] = DNSErrorNotExist
				binary.BigEndian.PutUint32(data[3:7], 0)
				return data
			}(),
			wantType:  DNSTypeError,
			wantError: DNSErrorNotExist,
			wantTTL:   0,
			expectErr: false,
		},
		{
			name: "Error response - Server failure",
			data: func() []byte {
				// TYPE (0xF0) | LENGTH (1) | ERROR (0x02 = SERVFAIL) | TTL (0)
				data := make([]byte, 7)
				data[0] = DNSTypeError
				data[1] = 1
				data[2] = DNSErrorServerFailure
				binary.BigEndian.PutUint32(data[3:7], 0)
				return data
			}(),
			wantType:  DNSTypeError,
			wantError: DNSErrorServerFailure,
			wantTTL:   0,
			expectErr: false,
		},
		{
			name:      "Empty data",
			data:      []byte{},
			expectErr: true,
		},
		{
			name: "Invalid IPv4 length",
			data: func() []byte {
				// TYPE (0x04) | LENGTH (3) - wrong! | garbage | TTL
				data := make([]byte, 9)
				data[0] = DNSTypeIPv4
				data[1] = 3 // Should be 4
				return data
			}(),
			expectErr: true,
		},
		{
			name: "Invalid IPv6 length",
			data: func() []byte {
				// TYPE (0x06) | LENGTH (8) - wrong! | garbage | TTL
				data := make([]byte, 14)
				data[0] = DNSTypeIPv6
				data[1] = 8 // Should be 16
				return data
			}(),
			expectErr: true,
		},
		{
			name:      "Truncated data",
			data:      []byte{0x04, 0x04, 0x01}, // Too short
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseResolvedCell(tt.data)

			if tt.expectErr {
				if err == nil {
					t.Errorf("parseResolvedCell() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("parseResolvedCell() unexpected error: %v", err)
				return
			}

			if result.Type != tt.wantType {
				t.Errorf("parseResolvedCell() Type = %d, want %d", result.Type, tt.wantType)
			}

			if result.TTL != tt.wantTTL {
				t.Errorf("parseResolvedCell() TTL = %d, want %d", result.TTL, tt.wantTTL)
			}

			if tt.wantType == DNSTypeError || tt.wantType == DNSTypeErrorTTL {
				if result.Error != tt.wantError {
					t.Errorf("parseResolvedCell() Error = %d, want %d", result.Error, tt.wantError)
				}
			}

			if tt.wantHost != "" {
				if result.Hostname != tt.wantHost {
					t.Errorf("parseResolvedCell() Hostname = %q, want %q", result.Hostname, tt.wantHost)
				}
			}

			if len(tt.wantAddrs) > 0 {
				if len(result.Addresses) != len(tt.wantAddrs) {
					t.Errorf("parseResolvedCell() got %d addresses, want %d", len(result.Addresses), len(tt.wantAddrs))
				} else {
					for i, wantAddr := range tt.wantAddrs {
						if result.Addresses[i].String() != wantAddr {
							t.Errorf("parseResolvedCell() Addresses[%d] = %v, want %v", i, result.Addresses[i], wantAddr)
						}
					}
				}
			}
		})
	}
}

// TestDNSConstants verifies DNS constant values match the spec
func TestDNSConstants(t *testing.T) {
	// Verify DNS types
	if DNSTypeHostname != 0x00 {
		t.Errorf("DNSTypeHostname = 0x%02X, want 0x00", DNSTypeHostname)
	}
	if DNSTypeIPv4 != 0x04 {
		t.Errorf("DNSTypeIPv4 = 0x%02X, want 0x04", DNSTypeIPv4)
	}
	if DNSTypeIPv6 != 0x06 {
		t.Errorf("DNSTypeIPv6 = 0x%02X, want 0x06", DNSTypeIPv6)
	}
	if DNSTypeError != 0xF0 {
		t.Errorf("DNSTypeError = 0x%02X, want 0xF0", DNSTypeError)
	}

	// Verify error codes
	if DNSErrorNotExist != 0x03 {
		t.Errorf("DNSErrorNotExist = 0x%02X, want 0x03", DNSErrorNotExist)
	}
	if DNSErrorServerFailure != 0x02 {
		t.Errorf("DNSErrorServerFailure = 0x%02X, want 0x02", DNSErrorServerFailure)
	}
}

// TestResolveHostnamePayload tests the RELAY_RESOLVE payload format for hostname queries
func TestResolveHostnamePayload(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		wantLen  int
	}{
		{
			name:     "Simple hostname",
			hostname: "example.com",
			wantLen:  12, // "example.com\x00" = 12 bytes
		},
		{
			name:     "Subdomain",
			hostname: "www.example.com",
			wantLen:  16, // "www.example.com\x00" = 16 bytes
		},
		{
			name:     "Long hostname",
			hostname: "very.long.subdomain.example.com",
			wantLen:  32, // "very.long.subdomain.example.com\x00" = 32 bytes
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create payload as would be done in ResolveHostname
			payload := append([]byte(tt.hostname), 0x00)

			if len(payload) != tt.wantLen {
				t.Errorf("Payload length = %d, want %d", len(payload), tt.wantLen)
			}

			// Verify null termination
			if payload[len(payload)-1] != 0x00 {
				t.Errorf("Payload not null-terminated")
			}

			// Verify content
			if string(payload[:len(payload)-1]) != tt.hostname {
				t.Errorf("Payload content = %q, want %q", string(payload[:len(payload)-1]), tt.hostname)
			}
		})
	}
}

// TestResolveIPPayload tests the RELAY_RESOLVE payload format for PTR queries
func TestResolveIPPayload(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		wantType byte
		wantLen  int
	}{
		{
			name:     "IPv4 address",
			ip:       "192.0.2.1",
			wantType: DNSTypeIPv4,
			wantLen:  6, // TYPE(1) + LENGTH(1) + IPv4(4)
		},
		{
			name:     "IPv6 address",
			ip:       "2001:db8::1",
			wantType: DNSTypeIPv6,
			wantLen:  18, // TYPE(1) + LENGTH(1) + IPv6(16)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ipAddr := net.ParseIP(tt.ip)
			if ipAddr == nil {
				t.Fatalf("Failed to parse IP: %s", tt.ip)
			}

			// Create payload as would be done in ResolveIP
			var payload []byte
			if ipv4 := ipAddr.To4(); ipv4 != nil {
				payload = make([]byte, 6)
				payload[0] = DNSTypeIPv4
				payload[1] = 4
				copy(payload[2:], ipv4)
			} else if ipv6 := ipAddr.To16(); ipv6 != nil {
				payload = make([]byte, 18)
				payload[0] = DNSTypeIPv6
				payload[1] = 16
				copy(payload[2:], ipv6)
			}

			if len(payload) != tt.wantLen {
				t.Errorf("Payload length = %d, want %d", len(payload), tt.wantLen)
			}

			if payload[0] != tt.wantType {
				t.Errorf("Payload type = 0x%02X, want 0x%02X", payload[0], tt.wantType)
			}
		})
	}
}

// TestDNSResultValidation tests DNSResult structure validation
func TestDNSResultValidation(t *testing.T) {
	tests := []struct {
		name   string
		result *DNSResult
		valid  bool
	}{
		{
			name: "Valid IPv4 result",
			result: &DNSResult{
				Type:      DNSTypeIPv4,
				TTL:       3600,
				Addresses: []net.IP{net.ParseIP("192.0.2.1")},
			},
			valid: true,
		},
		{
			name: "Valid IPv6 result",
			result: &DNSResult{
				Type:      DNSTypeIPv6,
				TTL:       7200,
				Addresses: []net.IP{net.ParseIP("2001:db8::1")},
			},
			valid: true,
		},
		{
			name: "Valid hostname result",
			result: &DNSResult{
				Type:     DNSTypeHostname,
				TTL:      1800,
				Hostname: "example.com",
			},
			valid: true,
		},
		{
			name: "Valid error result",
			result: &DNSResult{
				Type:  DNSTypeError,
				TTL:   0,
				Error: DNSErrorNotExist,
			},
			valid: true,
		},
		{
			name: "Multiple IPv4 addresses",
			result: &DNSResult{
				Type: DNSTypeIPv4,
				TTL:  3600,
				Addresses: []net.IP{
					net.ParseIP("192.0.2.1"),
					net.ParseIP("192.0.2.2"),
				},
			},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Basic validation
			if tt.result.Type == DNSTypeIPv4 || tt.result.Type == DNSTypeIPv6 {
				if len(tt.result.Addresses) == 0 {
					t.Errorf("IP result should have addresses")
				}
			}

			if tt.result.Type == DNSTypeHostname {
				if tt.result.Hostname == "" {
					t.Errorf("Hostname result should have hostname")
				}
			}

			if tt.result.Type == DNSTypeError {
				// Error results should have error code
				_ = tt.result.Error
			}
		})
	}
}

// mockConnection is a minimal mock connection for DNS testing that implements
// the cellSender interface required by Circuit.SendRelayCell. It accepts cells
// but doesn't actually send them, allowing tests to verify DNS resolution logic
// without requiring a real network connection.
type mockConnection struct{}

// SendCell implements the cellSender interface for mockConnection.
// It accepts cells but doesn't send them, returning nil to indicate success.
func (m *mockConnection) SendCell(c *cell.Cell) error {
	// Mock connection that accepts cells but doesn't do anything
	return nil
}

// MockCircuitForDNS creates a mock circuit for DNS testing
// Note: This is a simplified mock that doesn't fully simulate the circuit behavior
// For integration tests, use a real circuit with mock network layer
func MockCircuitForDNS(t *testing.T, responseData []byte) *Circuit {
	c := &Circuit{
		ID:               1,
		State:            StateOpen,
		relayReceiveChan: make(chan *cell.RelayCell, 1),
		conn:             &mockConnection{},
	}

	// Simulate the response by directly injecting into receive channel
	go func() {
		// Small delay to allow the call to be made
		time.Sleep(10 * time.Millisecond)

		// Send back RELAY_RESOLVED response
		resolvedCell := cell.NewRelayCell(0, cell.RelayResolved, responseData)
		c.relayReceiveChan <- resolvedCell
	}()

	return c
}

// TestResolveHostnameIntegration tests the full ResolveHostname flow
func TestResolveHostnameIntegration(t *testing.T) {
	// Create response data for "example.com" -> "192.0.2.1"
	responseData := make([]byte, 10)
	responseData[0] = DNSTypeIPv4
	responseData[1] = 4
	copy(responseData[2:6], []byte{192, 0, 2, 1})
	binary.BigEndian.PutUint32(responseData[6:10], 3600)

	c := MockCircuitForDNS(t, responseData)

	ctx := context.Background()
	result, err := c.ResolveHostname(ctx, "example.com")
	if err != nil {
		t.Fatalf("ResolveHostname() error = %v", err)
	}

	if result.Type != DNSTypeIPv4 {
		t.Errorf("Result type = %d, want %d", result.Type, DNSTypeIPv4)
	}

	if len(result.Addresses) != 1 {
		t.Errorf("Result has %d addresses, want 1", len(result.Addresses))
	}

	if result.Addresses[0].String() != "192.0.2.1" {
		t.Errorf("Result address = %v, want 192.0.2.1", result.Addresses[0])
	}

	if result.TTL != 3600 {
		t.Errorf("Result TTL = %d, want 3600", result.TTL)
	}
}

// TestResolveIPIntegration tests the full ResolveIP flow
func TestResolveIPIntegration(t *testing.T) {
	// Create response data for PTR query: "192.0.2.1" -> "example.com"
	hostname := "example.com\x00"
	responseData := make([]byte, 2+len(hostname)+4)
	responseData[0] = DNSTypeHostname
	responseData[1] = byte(len(hostname))
	copy(responseData[2:2+len(hostname)], []byte(hostname))
	binary.BigEndian.PutUint32(responseData[2+len(hostname):], 1800)

	c := MockCircuitForDNS(t, responseData)

	ctx := context.Background()
	result, err := c.ResolveIP(ctx, net.ParseIP("192.0.2.1"))
	if err != nil {
		t.Fatalf("ResolveIP() error = %v", err)
	}

	if result.Type != DNSTypeHostname {
		t.Errorf("Result type = %d, want %d", result.Type, DNSTypeHostname)
	}

	if result.Hostname != "example.com" {
		t.Errorf("Result hostname = %q, want %q", result.Hostname, "example.com")
	}

	if result.TTL != 1800 {
		t.Errorf("Result TTL = %d, want 1800", result.TTL)
	}
}

// TestResolveHostnameErrors tests error handling in ResolveHostname
func TestResolveHostnameErrors(t *testing.T) {
	tests := []struct {
		name        string
		hostname    string
		expectError string
	}{
		{
			name:        "Empty hostname",
			hostname:    "",
			expectError: "hostname cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Circuit{
				ID:               1,
				State:            StateOpen,
				relayReceiveChan: make(chan *cell.RelayCell, 1),
			}

			ctx := context.Background()
			_, err := c.ResolveHostname(ctx, tt.hostname)
			if err == nil {
				t.Errorf("ResolveHostname() expected error but got none")
			} else if err.Error() != tt.expectError {
				t.Errorf("ResolveHostname() error = %q, want %q", err.Error(), tt.expectError)
			}
		})
	}
}

// TestResolveIPErrors tests error handling in ResolveIP
func TestResolveIPErrors(t *testing.T) {
	tests := []struct {
		name        string
		ip          net.IP
		expectError string
	}{
		{
			name:        "Nil IP address",
			ip:          nil,
			expectError: "IP address cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Circuit{
				ID:               1,
				State:            StateOpen,
				relayReceiveChan: make(chan *cell.RelayCell, 1),
			}

			ctx := context.Background()
			_, err := c.ResolveIP(ctx, tt.ip)
			if err == nil {
				t.Errorf("ResolveIP() expected error but got none")
			} else if err.Error() != tt.expectError {
				t.Errorf("ResolveIP() error = %q, want %q", err.Error(), tt.expectError)
			}
		})
	}
}
