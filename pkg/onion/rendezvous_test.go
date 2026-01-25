// Package onion - Rendezvous Protocol Tests
package onion

import (
	"context"
	"encoding/binary"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/circuit"
	"github.com/opd-ai/go-tor/pkg/directory"
	"github.com/opd-ai/go-tor/pkg/logger"
	"github.com/opd-ai/go-tor/pkg/path"
)

// mockCircuitBuilder implements a test circuit builder
type mockCircuitBuilder struct {
	buildFunc func(ctx context.Context, p *path.Path, timeout time.Duration) (*circuit.Circuit, error)
}

func (m *mockCircuitBuilder) BuildCircuit(ctx context.Context, p *path.Path, timeout time.Duration) (*circuit.Circuit, error) {
	if m.buildFunc != nil {
		return m.buildFunc(ctx, p, timeout)
	}
	// Return a mock circuit
	return &circuit.Circuit{ID: 12345}, nil
}

func (m *mockCircuitBuilder) SetRateLimiter(limiter interface{})      {}
func (m *mockCircuitBuilder) SetMetricsRecorder(recorder interface{}) {}

// mockPathSelector implements a test path selector
type mockPathSelector struct {
	relays []*directory.Relay
}

func (m *mockPathSelector) GetRelays() []*directory.Relay {
	return m.relays
}

func (m *mockPathSelector) SelectPath(exitPort int) (*path.Path, error) {
	if len(m.relays) < 3 {
		return nil, nil
	}
	return &path.Path{
		Guard:  m.relays[0],
		Middle: m.relays[1],
		Exit:   m.relays[2],
	}, nil
}

func (m *mockPathSelector) UpdateConsensus(ctx context.Context) error {
	return nil
}

func (m *mockPathSelector) ConfirmGuard(fingerprint string) {}

// createTestRelays creates a set of test relays for testing
func createTestRelays() []*directory.Relay {
	return []*directory.Relay{
		{
			Nickname:    "TestGuard",
			Fingerprint: "AAAAAAAAAAAAAAAAAAAAAA",
			Address:     "1.2.3.4", // Different /16 subnet
			ORPort:      9001,
			Bandwidth:   1000000,
			IdentityKey: make([]byte, 32), // Guard has Ed25519 identity
			Flags:       []string{"Guard", "Fast", "Stable", "Running", "Valid"},
		},
		{
			Nickname:    "TestMiddle",
			Fingerprint: "BBBBBBBBBBBBBBBBBBBBBB",
			Address:     "5.6.7.8", // Different /16 subnet
			ORPort:      9001,
			Bandwidth:   800000,
			IdentityKey: make([]byte, 32),
			Flags:       []string{"Fast", "Stable", "Running", "Valid"},
		},
		{
			Nickname:    "TestRendezvous",
			Fingerprint: "CCCCCCCCCCCCCCCCCCCCCC",
			Address:     "10.0.0.1", // Different /16 subnet
			ORPort:      443,
			Bandwidth:   500000,
			IdentityKey: make([]byte, 32),
			Flags:       []string{"Fast", "Running", "Valid"},
		},
	}
}

// createIPv4LinkSpecifiers creates test link specifiers for IPv4
func createIPv4LinkSpecifiers(ip string, port uint16) []LinkSpecifier {
	// Parse IP string to bytes
	ipData := make([]byte, 6)
	// Simple IP parsing for test data
	if ip == "10.0.0.1" {
		ipData[0] = 10
		ipData[1] = 0
		ipData[2] = 0
		ipData[3] = 1
	} else if ip == "192.168.1.1" {
		ipData[0] = 192
		ipData[1] = 168
		ipData[2] = 1
		ipData[3] = 1
	}
	binary.BigEndian.PutUint16(ipData[4:6], port)

	return []LinkSpecifier{
		{
			Type: 0x00, // TLS-over-TCP-IPv4
			Data: ipData,
		},
	}
}

// createIPv6LinkSpecifiers creates test link specifiers for IPv6
func createIPv6LinkSpecifiers() []LinkSpecifier {
	ipData := make([]byte, 18)
	// 2001:db8::1 port 443
	ipData[0] = 0x20
	ipData[1] = 0x01
	ipData[2] = 0x0d
	ipData[3] = 0xb8
	// Rest are zeros except last byte
	ipData[15] = 0x01
	binary.BigEndian.PutUint16(ipData[16:18], 443)

	return []LinkSpecifier{
		{
			Type: 0x01, // TLS-over-TCP-IPv6
			Data: ipData,
		},
	}
}

// createLinkSpecifiersWithFingerprint creates link specs with Ed25519 fingerprint
func createLinkSpecifiersWithFingerprint(ip string, port uint16, fingerprint []byte) []LinkSpecifier {
	specs := createIPv4LinkSpecifiers(ip, port)

	// Add Ed25519 fingerprint
	specs = append(specs, LinkSpecifier{
		Type: 0x03, // Ed25519 identity
		Data: fingerprint,
	})

	return specs
}

func TestNewRendezvousCircuitBuilder(t *testing.T) {
	builder := &mockCircuitBuilder{}
	selector := &mockPathSelector{}
	log := logger.NewDefault()

	rcb := NewRendezvousCircuitBuilder(&circuit.Builder{}, &path.Selector{}, log)

	if rcb == nil {
		t.Fatal("NewRendezvousCircuitBuilder returned nil")
	}
	if rcb.logger == nil {
		t.Error("logger is nil")
	}

	// Test with nil logger
	rcb2 := NewRendezvousCircuitBuilder(builder, selector, nil)
	if rcb2 == nil {
		t.Fatal("NewRendezvousCircuitBuilder with nil logger returned nil")
	}
	if rcb2.logger == nil {
		t.Error("logger should be created when nil is passed")
	}
}

func TestExtractRelayInfo_IPv4(t *testing.T) {
	rcb := NewRendezvousCircuitBuilder(nil, nil, nil)

	linkSpecs := createIPv4LinkSpecifiers("10.0.0.1", 443)

	info, err := rcb.extractRelayInfo(linkSpecs)
	if err != nil {
		t.Fatalf("extractRelayInfo failed: %v", err)
	}

	if info.Address != "10.0.0.1:443" {
		t.Errorf("expected address 10.0.0.1:443, got %s", info.Address)
	}
	if info.IPv4 != "10.0.0.1" {
		t.Errorf("expected IPv4 10.0.0.1, got %s", info.IPv4)
	}
}

func TestExtractRelayInfo_IPv6(t *testing.T) {
	rcb := NewRendezvousCircuitBuilder(nil, nil, nil)

	linkSpecs := createIPv6LinkSpecifiers()

	info, err := rcb.extractRelayInfo(linkSpecs)
	if err != nil {
		t.Fatalf("extractRelayInfo failed: %v", err)
	}

	// IPv6 address should be formatted
	if info.Address == "" {
		t.Error("expected non-empty address for IPv6")
	}
	if info.IPv6 == "" {
		t.Error("expected non-empty IPv6 field")
	}
}

func TestExtractRelayInfo_WithFingerprint(t *testing.T) {
	rcb := NewRendezvousCircuitBuilder(nil, nil, nil)

	fingerprint := make([]byte, 32)
	for i := range fingerprint {
		fingerprint[i] = byte(i)
	}

	linkSpecs := createLinkSpecifiersWithFingerprint("10.0.0.1", 443, fingerprint)

	info, err := rcb.extractRelayInfo(linkSpecs)
	if err != nil {
		t.Fatalf("extractRelayInfo failed: %v", err)
	}

	if info.Address != "10.0.0.1:443" {
		t.Errorf("expected address 10.0.0.1:443, got %s", info.Address)
	}
	if len(info.Fingerprint) != 32 {
		t.Errorf("expected fingerprint length 32, got %d", len(info.Fingerprint))
	}
	if !bytesEqual(info.Fingerprint, fingerprint) {
		t.Error("fingerprint does not match")
	}
}

func TestExtractRelayInfo_NoAddress(t *testing.T) {
	rcb := NewRendezvousCircuitBuilder(nil, nil, nil)

	// Link spec with only fingerprint, no address
	linkSpecs := []LinkSpecifier{
		{
			Type: 0x03, // Ed25519 identity only
			Data: make([]byte, 32),
		},
	}

	_, err := rcb.extractRelayInfo(linkSpecs)
	if err == nil {
		t.Error("expected error for link specs with no address")
	}
}

func TestFindRelayInConsensus_ByEd25519(t *testing.T) {
	relays := createTestRelays()

	// Set specific Ed25519 identity on test relay
	testFingerprint := make([]byte, 32)
	for i := range testFingerprint {
		testFingerprint[i] = byte(0xAB)
	}
	relays[2].IdentityKey = testFingerprint

	selector := &mockPathSelector{relays: relays}
	rcb := NewRendezvousCircuitBuilder(nil, selector, nil)

	info := &RelayInfo{
		Address:     "10.0.0.1:443",
		Fingerprint: testFingerprint,
	}

	relay, err := rcb.findRelayInConsensus(info)
	if err != nil {
		t.Fatalf("findRelayInConsensus failed: %v", err)
	}

	if relay.Nickname != "TestRendezvous" {
		t.Errorf("expected TestRendezvous, got %s", relay.Nickname)
	}
}

func TestFindRelayInConsensus_ByIPv4Fallback(t *testing.T) {
	relays := createTestRelays()
	selector := &mockPathSelector{relays: relays}
	rcb := NewRendezvousCircuitBuilder(nil, selector, nil)

	info := &RelayInfo{
		Address:     "10.0.0.1:443",
		IPv4:        "10.0.0.1",
		Fingerprint: nil, // No fingerprint - will fall back to IPv4 match
	}

	relay, err := rcb.findRelayInConsensus(info)
	if err != nil {
		t.Fatalf("findRelayInConsensus failed: %v", err)
	}

	if relay.Nickname != "TestRendezvous" {
		t.Errorf("expected nickname TestRendezvous, got %s", relay.Nickname)
	}
}

func TestFindRelayInConsensus_NotFound(t *testing.T) {
	relays := createTestRelays()
	selector := &mockPathSelector{relays: relays}
	rcb := NewRendezvousCircuitBuilder(nil, selector, nil)

	// Create a non-matching fingerprint (all 0xFF)
	nonMatchingFingerprint := make([]byte, 32)
	for i := range nonMatchingFingerprint {
		nonMatchingFingerprint[i] = 0xFF
	}

	info := &RelayInfo{
		Address:     "192.0.2.1:443",
		IPv4:        "192.0.2.1",
		Fingerprint: nonMatchingFingerprint,
	}

	_, err := rcb.findRelayInConsensus(info)
	if err == nil {
		t.Error("expected error for relay not found")
	}
}

func TestFindRelayInConsensus_NoRelays(t *testing.T) {
	selector := &mockPathSelector{relays: []*directory.Relay{}}
	rcb := NewRendezvousCircuitBuilder(nil, selector, nil)

	info := &RelayInfo{
		Address: "10.0.0.1:443",
		IPv4:    "10.0.0.1",
	}

	_, err := rcb.findRelayInConsensus(info)
	if err == nil {
		t.Error("expected error for no relays in consensus")
	}
}

func TestSelectPathToRelay(t *testing.T) {
	relays := createTestRelays()
	selector := &mockPathSelector{relays: relays}
	rcb := NewRendezvousCircuitBuilder(nil, selector, nil)

	exitRelay := relays[2] // TestRendezvous

	p, err := rcb.selectPathToRelay(exitRelay)
	if err != nil {
		t.Fatalf("selectPathToRelay failed: %v", err)
	}

	if p.Exit.Nickname != "TestRendezvous" {
		t.Errorf("expected exit TestRendezvous, got %s", p.Exit.Nickname)
	}
	if p.Guard.Fingerprint == p.Exit.Fingerprint {
		t.Error("guard should not be the same as exit")
	}
	if p.Middle.Fingerprint == p.Exit.Fingerprint {
		t.Error("middle should not be the same as exit")
	}
	if p.Guard.Fingerprint == p.Middle.Fingerprint {
		t.Error("guard should not be the same as middle")
	}
}

func TestSelectPathToRelay_InsufficientRelays(t *testing.T) {
	// Only 2 relays available
	relays := createTestRelays()[:2]
	selector := &mockPathSelector{relays: relays}
	rcb := NewRendezvousCircuitBuilder(nil, selector, nil)

	exitRelay := relays[1]

	_, err := rcb.selectPathToRelay(exitRelay)
	if err == nil {
		t.Error("expected error for insufficient relays")
	}
}

func TestBuildRendezvousCircuit_Success(t *testing.T) {
	relays := createTestRelays()

	// Set Ed25519 identity on rendezvous relay
	testFingerprint := make([]byte, 32)
	for i := range testFingerprint {
		testFingerprint[i] = byte(0xCC)
	}
	relays[2].IdentityKey = testFingerprint

	selector := &mockPathSelector{relays: relays}

	builtCircuit := &circuit.Circuit{ID: 99999}
	builder := &mockCircuitBuilder{
		buildFunc: func(ctx context.Context, p *path.Path, timeout time.Duration) (*circuit.Circuit, error) {
			// Verify path is correct
			if p.Exit.Nickname != "TestRendezvous" {
				t.Errorf("expected exit TestRendezvous, got %s", p.Exit.Nickname)
			}
			return builtCircuit, nil
		},
	}

	rcb := NewRendezvousCircuitBuilder(builder, selector, nil)

	linkSpecs := createLinkSpecifiersWithFingerprint("10.0.0.1", 443, testFingerprint)

	ctx := context.Background()
	circ, err := rcb.BuildRendezvousCircuit(ctx, linkSpecs, 10*time.Second)
	if err != nil {
		t.Fatalf("BuildRendezvousCircuit failed: %v", err)
	}

	if circ.ID != 99999 {
		t.Errorf("expected circuit ID 99999, got %d", circ.ID)
	}
}

func TestBuildRendezvousCircuit_NoBuilder(t *testing.T) {
	selector := &mockPathSelector{relays: createTestRelays()}
	rcb := NewRendezvousCircuitBuilder(nil, selector, nil)

	linkSpecs := createIPv4LinkSpecifiers("10.0.0.1", 443)

	ctx := context.Background()
	_, err := rcb.BuildRendezvousCircuit(ctx, linkSpecs, 10*time.Second)
	if err == nil {
		t.Error("expected error for nil circuit builder")
	}
}

func TestBuildRendezvousCircuit_NoSelector(t *testing.T) {
	builder := &mockCircuitBuilder{}
	rcb := NewRendezvousCircuitBuilder(builder, nil, nil)

	linkSpecs := createIPv4LinkSpecifiers("10.0.0.1", 443)

	ctx := context.Background()
	_, err := rcb.BuildRendezvousCircuit(ctx, linkSpecs, 10*time.Second)
	if err == nil {
		t.Error("expected error for nil path selector")
	}
}

func TestBuildRendezvousCircuit_InvalidLinkSpecs(t *testing.T) {
	relays := createTestRelays()
	selector := &mockPathSelector{relays: relays}
	builder := &mockCircuitBuilder{}
	rcb := NewRendezvousCircuitBuilder(builder, selector, nil)

	// Link specs with no address
	linkSpecs := []LinkSpecifier{
		{
			Type: 0x03, // Ed25519 only, no address
			Data: make([]byte, 32),
		},
	}

	ctx := context.Background()
	_, err := rcb.BuildRendezvousCircuit(ctx, linkSpecs, 10*time.Second)
	if err == nil {
		t.Error("expected error for invalid link specs")
	}
}

func TestBytesEqual(t *testing.T) {
	a := []byte{1, 2, 3, 4}
	b := []byte{1, 2, 3, 4}
	c := []byte{1, 2, 3, 5}
	d := []byte{1, 2, 3}

	if !bytesEqual(a, b) {
		t.Error("equal slices should return true")
	}
	if bytesEqual(a, c) {
		t.Error("different slices should return false")
	}
	if bytesEqual(a, d) {
		t.Error("different length slices should return false")
	}
}

func TestIsInList(t *testing.T) {
	relays := createTestRelays()
	rcb := NewRendezvousCircuitBuilder(nil, nil, nil)

	list := []*directory.Relay{relays[0], relays[1]}

	if !rcb.isInList(relays[0], list) {
		t.Error("relay should be in list")
	}
	if rcb.isInList(relays[2], list) {
		t.Error("relay should not be in list")
	}
}

func TestSelectWeighted(t *testing.T) {
	relays := createTestRelays()
	rcb := NewRendezvousCircuitBuilder(nil, nil, nil)

	// Test with relays
	selected := rcb.selectWeighted(relays)
	if selected == nil {
		t.Error("selectWeighted should return a relay")
	}

	// Test with empty list
	selected = rcb.selectWeighted([]*directory.Relay{})
	if selected != nil {
		t.Error("selectWeighted should return nil for empty list")
	}

	// Test with single relay
	selected = rcb.selectWeighted(relays[:1])
	if selected != relays[0] {
		t.Error("selectWeighted should return the only relay")
	}

	// Test with zero bandwidth relays
	zeroBWRelays := []*directory.Relay{
		{Nickname: "Zero1", Bandwidth: 0},
		{Nickname: "Zero2", Bandwidth: 0},
	}
	selected = rcb.selectWeighted(zeroBWRelays)
	if selected == nil {
		t.Error("selectWeighted should handle zero bandwidth relays")
	}
}
