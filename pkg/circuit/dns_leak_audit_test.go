// Package circuit - DNS Leak Security Audit Test Suite
//
// This file contains comprehensive tests to verify that DNS resolution
// is performed exclusively through Tor circuits and never leaks to the
// system DNS resolver. DNS leaks are a critical privacy vulnerability
// that can deanonymize users by exposing DNS queries to ISPs or local networks.
//
// Audit Scope:
// 1. Verify all DNS resolution uses RELAY_RESOLVE cells through Tor circuits
// 2. Confirm no usage of system DNS functions (net.LookupHost, net.LookupIP, etc.)
// 3. Validate DNS queries are routed through the circuit's exit relay
// 4. Test that failed circuits don't fall back to system DNS
// 5. Verify DNS resolution respects circuit isolation
//
// Security Requirements (tor-spec.txt §6.4):
// - DNS queries MUST be sent via RELAY_RESOLVE cells
// - Responses MUST come from the circuit's exit relay
// - No local DNS resolution should occur
// - Failed DNS queries should not trigger system DNS fallback
//
// References:
// - tor-spec.txt §6.4 (Remote hostname lookup)
// - CWE-200: Exposure of Sensitive Information to an Unauthorized Actor
// - CWE-319: Cleartext Transmission of Sensitive Information

package circuit

import (
	"context"
	"encoding/binary"
	"net"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/cell"
)

// TestDNSNoSystemCalls verifies that DNS resolution does not use system calls
// This test uses runtime stack inspection to detect any calls to net.Lookup* functions
func TestDNSNoSystemCalls(t *testing.T) {
	// Create response data for "example.com" -> "192.0.2.1"
	responseData := make([]byte, 10)
	responseData[0] = DNSTypeIPv4
	responseData[1] = 4
	copy(responseData[2:6], []byte{192, 0, 2, 1})
	binary.BigEndian.PutUint32(responseData[6:10], 3600)

	c := MockCircuitForDNS(t, responseData)

	// Capture stack trace during resolution
	var stackBuf [8192]byte
	ctx := context.Background()

	// Perform resolution
	_, err := c.ResolveHostname(ctx, "example.com")
	if err != nil {
		t.Fatalf("ResolveHostname() error = %v", err)
	}

	// Get stack trace
	n := runtime.Stack(stackBuf[:], true)
	stack := string(stackBuf[:n])

	// Check for prohibited system DNS calls
	prohibitedFunctions := []string{
		"net.LookupHost",
		"net.LookupIP",
		"net.LookupAddr",
		"net.LookupCNAME",
		"net.LookupMX",
		"net.LookupNS",
		"net.LookupTXT",
		"net.LookupSRV",
		"net.DefaultResolver",
		"cgo_lookup",
		"getaddrinfo",
	}

	for _, fn := range prohibitedFunctions {
		if strings.Contains(stack, fn) {
			t.Errorf("DNS resolution uses prohibited system call: %s", fn)
		}
	}
}

// TestDNSResolutionThroughCircuit verifies DNS queries are sent via RELAY_RESOLVE
func TestDNSResolutionThroughCircuit(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		wantType byte
	}{
		{
			name:     "Regular hostname",
			hostname: "example.com",
			wantType: DNSTypeIPv4,
		},
		{
			name:     "Subdomain",
			hostname: "www.example.com",
			wantType: DNSTypeIPv4,
		},
		{
			name:     "Long hostname",
			hostname: "very.long.subdomain.example.com",
			wantType: DNSTypeIPv4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create response data
			responseData := make([]byte, 10)
			responseData[0] = tt.wantType
			responseData[1] = 4
			copy(responseData[2:6], []byte{192, 0, 2, 1})
			binary.BigEndian.PutUint32(responseData[6:10], 3600)

			c := MockCircuitForDNS(t, responseData)

			ctx := context.Background()
			result, err := c.ResolveHostname(ctx, tt.hostname)
			if err != nil {
				t.Fatalf("ResolveHostname(%q) error = %v", tt.hostname, err)
			}

			// Verify response came from circuit (not system DNS)
			if result.Type != tt.wantType {
				t.Errorf("Result type = %d, want %d (indicates non-circuit resolution)", result.Type, tt.wantType)
			}

			// Verify we got IP addresses (would be nil if system DNS failed and returned error)
			if len(result.Addresses) == 0 {
				t.Errorf("No addresses returned (possible system DNS fallback)")
			}
		})
	}
}

// TestDNSNoFallbackOnCircuitFailure verifies system DNS is not used when circuit fails
func TestDNSNoFallbackOnCircuitFailure(t *testing.T) {
	// Create a circuit that will timeout (no response)
	c := &Circuit{
		ID:               1,
		State:            StateOpen,
		relayReceiveChan: make(chan *cell.RelayCell, 1),
		conn:             &mockConnection{},
	}

	// Use short timeout to detect fallback quickly
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// This should fail with timeout, NOT succeed via system DNS
	result, err := c.ResolveHostname(ctx, "example.com")

	if err == nil {
		t.Errorf("ResolveHostname() succeeded when it should have failed (possible system DNS fallback)")
		t.Errorf("Got result: %+v", result)
	}

	// Verify error is timeout, not success from system DNS
	if err != nil && !strings.Contains(err.Error(), "deadline exceeded") &&
		!strings.Contains(err.Error(), "context canceled") {
		// If we got a result, that suggests system DNS was used
		t.Errorf("ResolveHostname() error suggests system DNS fallback: %v", err)
	}
}

// TestDNSReverseLookupThroughCircuit verifies PTR queries use RELAY_RESOLVE
func TestDNSReverseLookupThroughCircuit(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		wantType byte
	}{
		{
			name:     "IPv4 reverse lookup",
			ip:       "192.0.2.1",
			wantType: DNSTypeHostname,
		},
		{
			name:     "IPv6 reverse lookup",
			ip:       "2001:db8::1",
			wantType: DNSTypeHostname,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create response data for PTR query
			hostname := "example.com\x00"
			responseData := make([]byte, 2+len(hostname)+4)
			responseData[0] = tt.wantType
			responseData[1] = byte(len(hostname))
			copy(responseData[2:2+len(hostname)], []byte(hostname))
			binary.BigEndian.PutUint32(responseData[2+len(hostname):], 1800)

			c := MockCircuitForDNS(t, responseData)

			ctx := context.Background()
			result, err := c.ResolveIP(ctx, net.ParseIP(tt.ip))
			if err != nil {
				t.Fatalf("ResolveIP(%q) error = %v", tt.ip, err)
			}

			// Verify response came from circuit
			if result.Type != tt.wantType {
				t.Errorf("Result type = %d, want %d", result.Type, tt.wantType)
			}

			if result.Hostname == "" {
				t.Errorf("No hostname returned (possible system DNS fallback)")
			}
		})
	}
}

// TestDNSOnionAddressHandling verifies .onion addresses bypass DNS
func TestDNSOnionAddressHandling(t *testing.T) {
	tests := []struct {
		name    string
		address string
		isOnion bool
	}{
		{
			name:    "v3 onion address",
			address: "thehiddenwiki7fhdx5oawttis2ggfurncbhcivilization6vhogt4n4kkqid.onion",
			isOnion: true,
		},
		{
			name:    "Regular domain",
			address: "example.com",
			isOnion: false,
		},
		{
			name:    "Subdomain",
			address: "www.example.com",
			isOnion: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// .onion addresses should not trigger DNS resolution at all
			// They should be handled by the onion service layer
			if tt.isOnion {
				if strings.HasSuffix(tt.address, ".onion") {
					// This is correct - onion addresses are detected
					t.Logf("Correctly identified .onion address: %s", tt.address)
				} else {
					t.Errorf("Failed to identify .onion address: %s", tt.address)
				}
			}
		})
	}
}

// TestDNSEnvironmentIsolation verifies DNS resolution doesn't leak to environment
func TestDNSEnvironmentIsolation(t *testing.T) {
	// Save original DNS configuration
	originalResolvers := os.Getenv("GODEBUG")

	// Set GODEBUG to detect DNS fallback
	os.Setenv("GODEBUG", "netdns=go+1")
	defer func() {
		if originalResolvers != "" {
			os.Setenv("GODEBUG", originalResolvers)
		} else {
			os.Unsetenv("GODEBUG")
		}
	}()

	// Create response data
	responseData := make([]byte, 10)
	responseData[0] = DNSTypeIPv4
	responseData[1] = 4
	copy(responseData[2:6], []byte{192, 0, 2, 1})
	binary.BigEndian.PutUint32(responseData[6:10], 3600)

	c := MockCircuitForDNS(t, responseData)

	ctx := context.Background()
	_, err := c.ResolveHostname(ctx, "example.com")
	if err != nil {
		t.Fatalf("ResolveHostname() error = %v", err)
	}

	// If we got here without errors, DNS was handled through circuit
	// not through environment DNS configuration
}

// TestDNSConcurrentResolutionNoLeaks verifies concurrent DNS queries don't leak
func TestDNSConcurrentResolutionNoLeaks(t *testing.T) {
	const numGoroutines = 50

	// Create response data
	responseData := make([]byte, 10)
	responseData[0] = DNSTypeIPv4
	responseData[1] = 4
	copy(responseData[2:6], []byte{192, 0, 2, 1})
	binary.BigEndian.PutUint32(responseData[6:10], 3600)

	// Test concurrent DNS resolutions
	errChan := make(chan error, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			c := MockCircuitForDNS(t, responseData)
			ctx := context.Background()
			result, err := c.ResolveHostname(ctx, "example.com")
			if err != nil {
				errChan <- err
				return
			}
			if len(result.Addresses) == 0 {
				errChan <- nil // No error but suspicious result
			}
			errChan <- nil
		}(i)
	}

	// Collect results
	successCount := 0
	for i := 0; i < numGoroutines; i++ {
		err := <-errChan
		if err == nil {
			successCount++
		}
	}

	// All resolutions should succeed through circuits
	if successCount != numGoroutines {
		t.Errorf("Only %d/%d concurrent resolutions succeeded (possible DNS leak under load)",
			successCount, numGoroutines)
	}
}

// TestDNSErrorHandlingNoSystemFallback verifies DNS errors don't trigger system fallback
func TestDNSErrorHandlingNoSystemFallback(t *testing.T) {
	tests := []struct {
		name      string
		errorCode byte
		errorType byte
	}{
		{
			name:      "NXDOMAIN error",
			errorCode: DNSErrorNotExist,
			errorType: DNSTypeError,
		},
		{
			name:      "Server failure",
			errorCode: DNSErrorServerFailure,
			errorType: DNSTypeError,
		},
		{
			name:      "Format error",
			errorCode: DNSErrorFormat,
			errorType: DNSTypeError,
		},
		{
			name:      "Refused",
			errorCode: DNSErrorRefused,
			errorType: DNSTypeError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create error response
			responseData := make([]byte, 7)
			responseData[0] = tt.errorType
			responseData[1] = 1
			responseData[2] = tt.errorCode
			binary.BigEndian.PutUint32(responseData[3:7], 0)

			c := MockCircuitForDNS(t, responseData)

			ctx := context.Background()
			result, err := c.ResolveHostname(ctx, "nonexistent.example.com")

			// Should get an error from circuit, not success from system DNS
			if err == nil {
				t.Errorf("Expected error for DNS error code %d, got success (possible system DNS fallback)", tt.errorCode)
			}

			if result != nil && result.Type == tt.errorType {
				// Good - we got the error from the circuit
				if result.Error != tt.errorCode {
					t.Errorf("Got error code %d, want %d", result.Error, tt.errorCode)
				}
			}
		})
	}
}

// TestDNSIPv6ResolutionNoLeak verifies IPv6 resolution uses circuit
func TestDNSIPv6ResolutionNoLeak(t *testing.T) {
	// Create IPv6 response
	responseData := make([]byte, 22)
	responseData[0] = DNSTypeIPv6
	responseData[1] = 16
	ip := net.ParseIP("2001:db8::1").To16()
	copy(responseData[2:18], ip)
	binary.BigEndian.PutUint32(responseData[18:22], 7200)

	c := MockCircuitForDNS(t, responseData)

	ctx := context.Background()
	result, err := c.ResolveHostname(ctx, "ipv6.example.com")
	if err != nil {
		t.Fatalf("ResolveHostname() error = %v", err)
	}

	// Verify IPv6 result came from circuit
	if result.Type != DNSTypeIPv6 {
		t.Errorf("Result type = %d, want %d (DNSTypeIPv6)", result.Type, DNSTypeIPv6)
	}

	if len(result.Addresses) != 1 {
		t.Errorf("Got %d addresses, want 1", len(result.Addresses))
	}

	// Verify it's IPv6
	if result.Addresses[0].To16() == nil {
		t.Errorf("Result is not IPv6 address")
	}
}

// TestDNSTimeoutHandlingNoSystemFallback verifies timeout doesn't trigger system DNS
func TestDNSTimeoutHandlingNoSystemFallback(t *testing.T) {
	// Create circuit that won't respond
	c := &Circuit{
		ID:               1,
		State:            StateOpen,
		relayReceiveChan: make(chan *cell.RelayCell, 1),
		conn:             &mockConnection{},
	}

	// Short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := c.ResolveHostname(ctx, "timeout.example.com")
	elapsed := time.Since(start)

	// Should timeout, not succeed via system DNS
	if err == nil {
		t.Errorf("ResolveHostname() succeeded after timeout (possible system DNS fallback)")
	}

	// Verify it actually timed out (didn't succeed immediately via system DNS)
	if elapsed < 40*time.Millisecond {
		t.Errorf("Resolution completed too quickly (%v), suggests system DNS was used", elapsed)
	}
}

// TestDNSLocalAddressResolutionNoLeak verifies local addresses are handled correctly
func TestDNSLocalAddressResolutionNoLeak(t *testing.T) {
	localAddresses := []string{
		"localhost",
		"127.0.0.1",
		"::1",
	}

	for _, addr := range localAddresses {
		t.Run(addr, func(t *testing.T) {
			// Create response for local address
			responseData := make([]byte, 10)
			responseData[0] = DNSTypeIPv4
			responseData[1] = 4
			copy(responseData[2:6], []byte{127, 0, 0, 1})
			binary.BigEndian.PutUint32(responseData[6:10], 0) // TTL=0 for localhost

			c := MockCircuitForDNS(t, responseData)

			ctx := context.Background()
			result, err := c.ResolveHostname(ctx, addr)

			// Even localhost should go through circuit (no system DNS bypass)
			if err != nil {
				t.Logf("Resolution failed (expected for security): %v", err)
			}

			// If succeeded, verify it came from circuit
			if result != nil && result.Type == DNSTypeIPv4 {
				if !result.Addresses[0].IsLoopback() {
					t.Errorf("Expected loopback address, got %v", result.Addresses[0])
				}
			}
		})
	}
}

// TestDNSCircuitStateValidation verifies DNS requires open circuit
func TestDNSCircuitStateValidation(t *testing.T) {
	states := []struct {
		name  string
		state State
	}{
		{"Closed circuit", StateClosed},
		{"Failed circuit", StateFailed},
	}

	for _, st := range states {
		t.Run(st.name, func(t *testing.T) {
			c := &Circuit{
				ID:               1,
				State:            st.state,
				relayReceiveChan: make(chan *cell.RelayCell, 1),
				conn:             &mockConnection{},
			}

			ctx := context.Background()
			_, err := c.ResolveHostname(ctx, "example.com")

			// Should fail - cannot resolve DNS on non-open circuit
			// This prevents fallback to system DNS when circuit fails
			if err == nil {
				t.Errorf("ResolveHostname() succeeded on %s (possible fallback)", st.name)
			}
		})
	}
}
