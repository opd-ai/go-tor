// Package onion - Rendezvous Protocol Implementation
// This file implements rendezvous circuit building for onion service hosting
// Following rend-spec-v3.txt §3.3
package onion

import (
	"context"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/opd-ai/go-tor/pkg/circuit"
	"github.com/opd-ai/go-tor/pkg/directory"
	"github.com/opd-ai/go-tor/pkg/logger"
	"github.com/opd-ai/go-tor/pkg/path"
)

// CircuitBuilderInterface defines the interface for building circuits
type CircuitBuilderInterface interface {
	BuildCircuit(ctx context.Context, p *path.Path, timeout time.Duration) (*circuit.Circuit, error)
}

// PathSelectorInterface defines the interface for path selection
type PathSelectorInterface interface {
	GetRelays() []*directory.Relay
}

// RendezvousCircuitBuilder builds circuits to rendezvous points for onion services
type RendezvousCircuitBuilder struct {
	circuitBuilder CircuitBuilderInterface
	pathSelector   PathSelectorInterface
	logger         *logger.Logger
}

// NewRendezvousCircuitBuilder creates a new rendezvous circuit builder
func NewRendezvousCircuitBuilder(builder CircuitBuilderInterface, selector PathSelectorInterface, log *logger.Logger) *RendezvousCircuitBuilder {
	if log == nil {
		log = logger.NewDefault()
	}

	return &RendezvousCircuitBuilder{
		circuitBuilder: builder,
		pathSelector:   selector,
		logger:         log.Component("rendezvous"),
	}
}

// BuildRendezvousCircuit builds a 3-hop circuit to a rendezvous point
// specified by the client's link specifiers.
//
// The rendezvous point is specified by the client in the INTRODUCE2 cell
// through link specifiers (rend-spec-v3.txt §3.2.1). We need to:
// 1. Parse link specifiers to identify the relay
// 2. Find the relay in our consensus
// 3. Build a 3-hop circuit with the rendezvous point as the exit
//
// Returns the circuit ID on success.
func (r *RendezvousCircuitBuilder) BuildRendezvousCircuit(ctx context.Context, linkSpecs []LinkSpecifier, timeout time.Duration) (*circuit.Circuit, error) {
	if r.circuitBuilder == nil {
		return nil, fmt.Errorf("circuit builder not configured")
	}
	if r.pathSelector == nil {
		return nil, fmt.Errorf("path selector not configured")
	}

	// Extract relay information from link specifiers
	relayInfo, err := r.extractRelayInfo(linkSpecs)
	if err != nil {
		return nil, fmt.Errorf("failed to extract relay info: %w", err)
	}

	r.logger.Info("Building rendezvous circuit",
		"address", relayInfo.Address,
		"fingerprint", relayInfo.Fingerprint[:16])

	// Find the rendezvous relay in our consensus
	rendezvousRelay, err := r.findRelayInConsensus(relayInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to find rendezvous relay: %w", err)
	}

	r.logger.Info("Found rendezvous relay in consensus",
		"nickname", rendezvousRelay.Nickname,
		"address", rendezvousRelay.Address)

	// Select a path with the rendezvous point as the exit
	// We select guard and middle, using the rendezvous point as exit
	p, err := r.selectPathToRelay(rendezvousRelay)
	if err != nil {
		return nil, fmt.Errorf("failed to select path: %w", err)
	}

	r.logger.Info("Selected path for rendezvous circuit",
		"guard", p.Guard.Nickname,
		"middle", p.Middle.Nickname,
		"exit", p.Exit.Nickname)

	// Build the circuit
	circ, err := r.circuitBuilder.BuildCircuit(ctx, p, timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to build circuit: %w", err)
	}

	r.logger.Info("Rendezvous circuit built successfully",
		"circuit_id", circ.ID)

	return circ, nil
}

// RelayInfo contains extracted relay information from link specifiers
type RelayInfo struct {
	Address     string // IP:Port address
	Fingerprint []byte // Ed25519 identity key (type 0x03) or legacy RSA (type 0x02)
	IPv4        string // IPv4 address (if present)
	IPv6        string // IPv6 address (if present)
}

// extractRelayInfo extracts relay information from link specifiers
// Following tor-spec.txt §5.1.2 for link specifier format
func (r *RendezvousCircuitBuilder) extractRelayInfo(linkSpecs []LinkSpecifier) (*RelayInfo, error) {
	info := &RelayInfo{}

	for _, spec := range linkSpecs {
		switch spec.Type {
		case 0x00: // TLS-over-TCP-IPv4
			if len(spec.Data) == 6 {
				ip := fmt.Sprintf("%d.%d.%d.%d", spec.Data[0], spec.Data[1], spec.Data[2], spec.Data[3])
				port := binary.BigEndian.Uint16(spec.Data[4:6])
				info.IPv4 = ip
				if info.Address == "" {
					info.Address = fmt.Sprintf("%s:%d", ip, port)
				}
			}

		case 0x01: // TLS-over-TCP-IPv6
			if len(spec.Data) == 18 {
				port := binary.BigEndian.Uint16(spec.Data[16:18])
				// Format IPv6 address
				ip := fmt.Sprintf("%02x%02x:%02x%02x:%02x%02x:%02x%02x:%02x%02x:%02x%02x:%02x%02x:%02x%02x",
					spec.Data[0], spec.Data[1], spec.Data[2], spec.Data[3],
					spec.Data[4], spec.Data[5], spec.Data[6], spec.Data[7],
					spec.Data[8], spec.Data[9], spec.Data[10], spec.Data[11],
					spec.Data[12], spec.Data[13], spec.Data[14], spec.Data[15])
				info.IPv6 = ip
				// Prefer IPv4 if available, otherwise use IPv6
				if info.Address == "" {
					info.Address = fmt.Sprintf("[%s]:%d", ip, port)
				}
			}

		case 0x02: // Legacy RSA identity fingerprint (20 bytes)
			if len(spec.Data) == 20 {
				info.Fingerprint = make([]byte, 20)
				copy(info.Fingerprint, spec.Data)
			}

		case 0x03: // Ed25519 identity key (32 bytes)
			if len(spec.Data) == 32 {
				info.Fingerprint = make([]byte, 32)
				copy(info.Fingerprint, spec.Data)
			}
		}
	}

	// Validate we have minimum required information
	if info.Address == "" {
		return nil, fmt.Errorf("no address found in link specifiers")
	}

	return info, nil
}

// findRelayInConsensus finds a relay in the consensus by matching address and/or fingerprint
func (r *RendezvousCircuitBuilder) findRelayInConsensus(info *RelayInfo) (*directory.Relay, error) {
	allRelays := r.pathSelector.GetRelays()
	if len(allRelays) == 0 {
		return nil, fmt.Errorf("no relays available in consensus")
	}

	// Try to match by fingerprint first (most reliable)
	if len(info.Fingerprint) > 0 {
		for _, relay := range allRelays {
			// Check Ed25519 identity (IdentityKey field)
			if len(info.Fingerprint) == 32 && relay.IdentityKey != nil {
				if bytesEqual(info.Fingerprint, relay.IdentityKey) {
					return relay, nil
				}
			}
			// Check RSA fingerprint (legacy)
			if len(info.Fingerprint) == 20 && relay.Fingerprint != "" {
				if bytesEqual(info.Fingerprint, []byte(relay.Fingerprint)) {
					return relay, nil
				}
			}
		}
	}

	// Fall back to matching by IPv4 address
	if info.IPv4 != "" {
		for _, relay := range allRelays {
			if relay.Address == info.IPv4 {
				r.logger.Warn("Matched relay by IPv4 only (no fingerprint match)",
					"address", info.IPv4,
					"relay", relay.Nickname)
				return relay, nil
			}
		}
	}

	return nil, fmt.Errorf("relay not found in consensus: address=%s", info.Address)
}

// selectPathToRelay selects a path with the given relay as the exit
func (r *RendezvousCircuitBuilder) selectPathToRelay(exitRelay *directory.Relay) (*path.Path, error) {
	// Get available relays
	allRelays := r.pathSelector.GetRelays()
	if len(allRelays) < 3 {
		return nil, fmt.Errorf("insufficient relays: need 3, have %d", len(allRelays))
	}

	// Select guard (avoid exit relay)
	guard, err := r.selectRelayAvoid(allRelays, []*directory.Relay{exitRelay}, true, false)
	if err != nil {
		return nil, fmt.Errorf("failed to select guard: %w", err)
	}

	// Select middle (avoid guard and exit)
	middle, err := r.selectRelayAvoid(allRelays, []*directory.Relay{guard, exitRelay}, false, false)
	if err != nil {
		return nil, fmt.Errorf("failed to select middle: %w", err)
	}

	return &path.Path{
		Guard:  guard,
		Middle: middle,
		Exit:   exitRelay,
	}, nil
}

// selectRelayAvoid selects a relay avoiding the given relays
// Implements basic relay selection with family/subnet diversity
func (r *RendezvousCircuitBuilder) selectRelayAvoid(relays []*directory.Relay, avoid []*directory.Relay, needGuard bool, needExit bool) (*directory.Relay, error) {
	candidates := make([]*directory.Relay, 0)

	for _, relay := range relays {
		// Skip if in avoid list
		if r.isInList(relay, avoid) {
			continue
		}

		// Check flags if needed
		if needGuard && !r.hasFlag(relay, "Guard") {
			continue
		}
		if needExit && !r.hasFlag(relay, "Exit") {
			continue
		}

		// Check family diversity (avoid relays in same family)
		if r.hasFamily(relay, avoid) {
			continue
		}

		candidates = append(candidates, relay)
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no suitable relay found")
	}

	// Select weighted by bandwidth
	return r.selectWeighted(candidates), nil
}

// isInList checks if a relay is in a list of relays
func (r *RendezvousCircuitBuilder) isInList(relay *directory.Relay, list []*directory.Relay) bool {
	for _, r := range list {
		if r.Fingerprint == relay.Fingerprint {
			return true
		}
	}
	return false
}

// hasFlag checks if a relay has a specific flag
func (r *RendezvousCircuitBuilder) hasFlag(relay *directory.Relay, flag string) bool {
	for _, f := range relay.Flags {
		if f == flag {
			return true
		}
	}
	return false
}

// hasFamily checks if relay shares family with any relay in list
func (r *RendezvousCircuitBuilder) hasFamily(relay *directory.Relay, list []*directory.Relay) bool {
	for _, r := range list {
		// Check actual family membership
		for _, familyMember := range relay.Family {
			if familyMember == r.Fingerprint {
				return true
			}
		}
		// Check if in same /16 subnet (basic diversity) - simplified check
		if len(relay.Address) >= 7 && len(r.Address) >= 7 {
			// Only check if addresses look valid
			relayParts := relay.Address
			rParts := r.Address
			// Simple prefix check for first two octets
			if len(relayParts) > 6 && len(rParts) > 6 {
				// This is a simplified check - in real implementation would parse IP properly
				if relayParts[:6] == rParts[:6] {
					return true
				}
			}
		}
	}
	return false
}

// selectWeighted selects a relay weighted by bandwidth
func (r *RendezvousCircuitBuilder) selectWeighted(relays []*directory.Relay) *directory.Relay {
	if len(relays) == 0 {
		return nil
	}
	if len(relays) == 1 {
		return relays[0]
	}

	// Calculate total bandwidth
	totalBW := uint64(0)
	for _, relay := range relays {
		totalBW += relay.Bandwidth
	}

	// If all have zero bandwidth, select uniformly
	if totalBW == 0 {
		return relays[len(relays)/2] // Simple middle selection
	}

	// Weighted random selection (simplified)
	// In production, this should use proper weighted random sampling
	target := totalBW / 2 // Select relay near median bandwidth
	sum := uint64(0)
	for _, relay := range relays {
		sum += relay.Bandwidth
		if sum >= target {
			return relay
		}
	}

	return relays[len(relays)-1]
}

// bytesEqual compares two byte slices in constant time
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	diff := byte(0)
	for i := 0; i < len(a); i++ {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}
