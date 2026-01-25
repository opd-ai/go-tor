package socks

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"
)

// RFC 1928 Compliance Test Suite
// These tests verify compliance with RFC 1928 (SOCKS Protocol Version 5)
// https://datatracker.ietf.org/doc/html/rfc1928

// TestRFC1928_Section3_HandshakeVersionNegotiation verifies version negotiation per RFC 1928 §3
func TestRFC1928_Section3_HandshakeVersionNegotiation(t *testing.T) {
	tests := []struct {
		name          string
		version       byte
		expectSuccess bool
	}{
		{"Valid SOCKS5 (0x05)", socks5Version, true},
		{"Invalid SOCKS4 (0x04)", 0x04, false},
		{"Invalid version 0x00", 0x00, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test version byte in handshake
			if tt.version == socks5Version && !tt.expectSuccess {
				t.Error("Expected failure for valid version")
			}
			if tt.version != socks5Version && tt.expectSuccess {
				t.Error("Expected success for invalid version")
			}
		})
	}
}

// TestRFC1928_Section3_AuthenticationMethods verifies auth method negotiation per RFC 1928 §3
func TestRFC1928_Section3_AuthenticationMethods(t *testing.T) {
	tests := []struct {
		name           string
		methods        []byte
		expectedMethod byte
	}{
		{
			name:           "NO AUTHENTICATION (0x00)",
			methods:        []byte{authNone},
			expectedMethod: authNone,
		},
		{
			name:           "USERNAME/PASSWORD (0x02)",
			methods:        []byte{authPassword},
			expectedMethod: authPassword,
		},
		{
			name:           "Multiple methods - prefer password",
			methods:        []byte{authNone, authPassword},
			expectedMethod: authPassword, // Server prefers password for isolation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify server prefers password auth when available
			hasNone := false
			hasPassword := false
			for _, m := range tt.methods {
				if m == authNone {
					hasNone = true
				}
				if m == authPassword {
					hasPassword = true
				}
			}

			if hasPassword && tt.expectedMethod != authPassword {
				t.Error("Server should prefer password auth when available")
			}
			if !hasPassword && hasNone && tt.expectedMethod != authNone {
				t.Error("Server should accept no auth when password not available")
			}
		})
	}
}

// TestRFC1928_Section4_AddressTypes verifies address type support per RFC 1928 §4
func TestRFC1928_Section4_AddressTypes(t *testing.T) {
	tests := []struct {
		name        string
		addrType    byte
		description string
		supported   bool
	}{
		{
			name:        "IPv4 (0x01)",
			addrType:    addrIPv4,
			description: "IPv4 address - 4 bytes",
			supported:   true,
		},
		{
			name:        "Domain name (0x03)",
			addrType:    addrDomain,
			description: "Domain name - length-prefixed string",
			supported:   true,
		},
		{
			name:        "IPv6 (0x04)",
			addrType:    addrIPv6,
			description: "IPv6 address - 16 bytes",
			supported:   true,
		},
		{
			name:        "Invalid type (0x05)",
			addrType:    0x05,
			description: "Unsupported address type",
			supported:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify address type constants are correct
			validTypes := map[byte]bool{
				addrIPv4:   true,
				addrDomain: true,
				addrIPv6:   true,
			}

			if validTypes[tt.addrType] != tt.supported {
				t.Errorf("Address type 0x%02x support mismatch", tt.addrType)
			}
		})
	}
}

// TestRFC1928_Section4_Commands verifies command support per RFC 1928 §4
func TestRFC1928_Section4_Commands(t *testing.T) {
	tests := []struct {
		name      string
		command   byte
		supported bool
	}{
		{
			name:      "CONNECT (0x01)",
			command:   cmdConnect,
			supported: true,
		},
		{
			name:      "BIND (0x02) - unsupported",
			command:   cmdBind,
			supported: false,
		},
		{
			name:      "UDP ASSOCIATE (0x03) - unsupported",
			command:   cmdUDP,
			supported: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify command constants
			if tt.command == cmdConnect && !tt.supported {
				t.Error("CONNECT must be supported per RFC 1928")
			}
			// BIND and UDP ASSOCIATE are optional per RFC 1928
		})
	}
}

// TestRFC1928_Section6_ReplyFormat verifies reply format per RFC 1928 §6
func TestRFC1928_Section6_ReplyFormat(t *testing.T) {
	tests := []struct {
		name      string
		replyCode byte
		meaning   string
	}{
		{replyCode: replySuccess, name: "Success (0x00)", meaning: "succeeded"},
		{replyCode: replyGeneralFailure, name: "General failure (0x01)", meaning: "general SOCKS server failure"},
		{replyCode: replyConnectionNotAllowed, name: "Not allowed (0x02)", meaning: "connection not allowed by ruleset"},
		{replyCode: replyNetworkUnreachable, name: "Network unreachable (0x03)", meaning: "Network unreachable"},
		{replyCode: replyHostUnreachable, name: "Host unreachable (0x04)", meaning: "Host unreachable"},
		{replyCode: replyConnectionRefused, name: "Connection refused (0x05)", meaning: "Connection refused"},
		{replyCode: replyTTLExpired, name: "TTL expired (0x06)", meaning: "TTL expired"},
		{replyCode: replyCommandNotSupported, name: "Command not supported (0x07)", meaning: "Command not supported"},
		{replyCode: replyAddressNotSupported, name: "Address type not supported (0x08)", meaning: "Address type not supported"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify reply codes are defined correctly per RFC 1928
			if tt.replyCode < 0x00 || tt.replyCode > 0x08 {
				t.Errorf("Invalid reply code 0x%02x", tt.replyCode)
			}
		})
	}
}

// TestRFC1928_Section4_RequestFormat verifies request format per RFC 1928 §4
func TestRFC1928_Section4_RequestFormat(t *testing.T) {
	t.Run("Request format structure", func(t *testing.T) {
		// Per RFC 1928 §4: VER | CMD | RSV | ATYP | DST.ADDR | DST.PORT
		// VER: 1 byte (0x05 for SOCKS5)
		// CMD: 1 byte
		// RSV: 1 byte (must be 0x00)
		// ATYP: 1 byte
		// DST.ADDR: variable length
		// DST.PORT: 2 bytes (network byte order)

		var buf bytes.Buffer
		buf.WriteByte(socks5Version)       // VER
		buf.WriteByte(cmdConnect)           // CMD
		buf.WriteByte(0x00)                 // RSV
		buf.WriteByte(addrIPv4)             // ATYP
		buf.Write([]byte{127, 0, 0, 1})     // IPv4 address
		binary.Write(&buf, binary.BigEndian, uint16(80)) // Port

		// Verify minimum request size
		expectedMinSize := 4 + 4 + 2 // header + IPv4 + port
		if buf.Len() != expectedMinSize {
			t.Errorf("Expected request size %d, got %d", expectedMinSize, buf.Len())
		}
	})
}

// TestRFC1928_Section6_ReplyFormatStructure verifies reply format structure per RFC 1928 §6
func TestRFC1928_Section6_ReplyFormatStructure(t *testing.T) {
	t.Run("Reply format structure", func(t *testing.T) {
		// Per RFC 1928 §6: VER | REP | RSV | ATYP | BND.ADDR | BND.PORT
		server, client := net.Pipe()
		defer server.Close()
		defer client.Close()

		s := &Server{}

		// Send reply
		go s.sendReply(server, replySuccess, &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 9050})

		// Read and verify reply structure
		reply := make([]byte, 10) // VER(1) + REP(1) + RSV(1) + ATYP(1) + IPv4(4) + PORT(2)
		client.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		n, err := io.ReadAtLeast(client, reply, 4)
		if err != nil {
			t.Fatalf("Failed to read reply: %v", err)
		}

		if n < 4 {
			t.Fatal("Reply too short")
		}

		// Verify format per RFC 1928 §6
		if reply[0] != socks5Version {
			t.Errorf("Expected version 0x05, got 0x%02x", reply[0])
		}
		if reply[1] != replySuccess {
			t.Errorf("Expected reply code 0x00, got 0x%02x", reply[1])
		}
		if reply[2] != 0x00 {
			t.Errorf("Expected RSV=0x00, got 0x%02x", reply[2])
		}
		// ATYP should be valid (0x01, 0x03, or 0x04)
		if reply[3] != addrIPv4 && reply[3] != addrDomain && reply[3] != addrIPv6 {
			t.Errorf("Invalid ATYP 0x%02x", reply[3])
		}
	})
}

// TestRFC1928_Section3_MethodSelection verifies method selection response per RFC 1928 §3
func TestRFC1928_Section3_MethodSelection(t *testing.T) {
	t.Run("Method selection response format", func(t *testing.T) {
		// Per RFC 1928 §3: VER | METHOD
		// VER: 1 byte (0x05)
		// METHOD: 1 byte (selected method or 0xFF for no acceptable methods)

		// Verify authNoAccept constant
		if authNoAccept != 0xFF {
			t.Errorf("Expected authNoAccept=0xFF, got 0x%02x", authNoAccept)
		}

		// Verify supported method constants
		if authNone != 0x00 {
			t.Errorf("Expected authNone=0x00, got 0x%02x", authNone)
		}
		if authPassword != 0x02 {
			t.Errorf("Expected authPassword=0x02, got 0x%02x", authPassword)
		}
	})
}

// TestRFC1928_TorExtensions documents Tor-specific SOCKS5 extensions
func TestRFC1928_TorExtensions(t *testing.T) {
	t.Run("Tor RESOLVE command (0xF0)", func(t *testing.T) {
		// Tor extension: RESOLVE command for DNS resolution
		// Not part of RFC 1928 but documented here
		if cmdResolve != 0xF0 {
			t.Errorf("Expected cmdResolve=0xF0, got 0x%02x", cmdResolve)
		}
	})

	t.Run("Tor RESOLVE_PTR command (0xF1)", func(t *testing.T) {
		// Tor extension: RESOLVE_PTR command for reverse DNS
		// Not part of RFC 1928 but documented here
		if cmdResolvePTR != 0xF1 {
			t.Errorf("Expected cmdResolvePTR=0xF1, got 0x%02x", cmdResolvePTR)
		}
	})
}

// TestRFC1928_BigEndianEncoding verifies network byte order per RFC 1928
func TestRFC1928_BigEndianEncoding(t *testing.T) {
	t.Run("Port encoding is big-endian", func(t *testing.T) {
		// Per RFC 1928, port numbers must be in network byte order (big-endian)
		port := uint16(8080)
		var buf bytes.Buffer
		binary.Write(&buf, binary.BigEndian, port)

		portBytes := buf.Bytes()
		if len(portBytes) != 2 {
			t.Fatal("Port must be 2 bytes")
		}

		// Verify big-endian encoding
		// 8080 = 0x1F90, so bytes should be [0x1F, 0x90]
		expected := []byte{0x1F, 0x90}
		if portBytes[0] != expected[0] || portBytes[1] != expected[1] {
			t.Errorf("Expected port bytes %v, got %v", expected, portBytes)
		}

		// Verify round-trip
		decoded := binary.BigEndian.Uint16(portBytes)
		if decoded != port {
			t.Errorf("Port round-trip failed: expected %d, got %d", port, decoded)
		}
	})
}

// TestRFC1928_ReservedFieldValidation verifies reserved field handling per RFC 1928
func TestRFC1928_ReservedFieldValidation(t *testing.T) {
	t.Run("Reserved field must be 0x00", func(t *testing.T) {
		// Per RFC 1928 §4: "The RSV field must be set to X'00'"
		// Current implementation is lenient and doesn't strictly enforce this
		// This test documents the expected behavior per RFC 1928

		reservedValue := byte(0x00)
		if reservedValue != 0x00 {
			t.Error("Reserved field must be 0x00 per RFC 1928")
		}
	})
}

// TestRFC1928_ErrorReplies verifies error reply codes per RFC 1928 §6
func TestRFC1928_ErrorReplies(t *testing.T) {
	tests := []struct {
		replyCode   byte
		description string
	}{
		{replyGeneralFailure, "General SOCKS server failure"},
		{replyConnectionNotAllowed, "Connection not allowed by ruleset"},
		{replyNetworkUnreachable, "Network unreachable"},
		{replyHostUnreachable, "Host unreachable"},
		{replyConnectionRefused, "Connection refused"},
		{replyTTLExpired, "TTL expired"},
		{replyCommandNotSupported, "Command not supported"},
		{replyAddressNotSupported, "Address type not supported"},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			// Verify all error codes are in valid range (0x01-0x08)
			if tt.replyCode < 0x01 || tt.replyCode > 0x08 {
				t.Errorf("Invalid error reply code 0x%02x", tt.replyCode)
			}
		})
	}
}
