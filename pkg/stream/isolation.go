// Package stream provides Tor stream management for multiplexing connections over circuits.
// This file implements stream isolation enforcement to prevent applications from different
// sources sharing circuits, addressing AUDIT.md MED-006 and ROADMAP.md Phase 2.2.
package stream

import (
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/opd-ai/go-tor/pkg/circuit"
	"github.com/opd-ai/go-tor/pkg/logger"
)

// IsolationMode defines how stream isolation is enforced.
type IsolationMode int

const (
	// IsolationModeOff disables isolation enforcement (legacy mode).
	IsolationModeOff IsolationMode = iota
	// IsolationModeWarn logs violations but allows them.
	IsolationModeWarn
	// IsolationModeStrict rejects requests that violate isolation boundaries.
	IsolationModeStrict
)

// String returns a string representation of the isolation mode.
func (m IsolationMode) String() string {
	switch m {
	case IsolationModeOff:
		return "off"
	case IsolationModeWarn:
		return "warn"
	case IsolationModeStrict:
		return "strict"
	default:
		return fmt.Sprintf("unknown(%d)", m)
	}
}

// ParseIsolationMode parses a string into an IsolationMode.
func ParseIsolationMode(s string) (IsolationMode, error) {
	switch strings.ToLower(s) {
	case "off", "":
		return IsolationModeOff, nil
	case "warn":
		return IsolationModeWarn, nil
	case "strict":
		return IsolationModeStrict, nil
	default:
		return IsolationModeOff, fmt.Errorf("invalid isolation mode: %s", s)
	}
}

// IsolationPolicy defines the rules for stream isolation.
type IsolationPolicy struct {
	// Mode determines how violations are handled.
	Mode IsolationMode

	// IsolateBySOCKSAuth isolates streams by SOCKS5 username.
	IsolateBySOCKSAuth bool

	// IsolateByDestination isolates streams by target destination.
	IsolateByDestination bool

	// IsolateBySourcePort isolates streams by client source port.
	IsolateBySourcePort bool

	// IsolateBySession isolates streams by session token.
	IsolateBySession bool

	// EnforceOnExistingCircuits checks isolation on circuit reuse.
	EnforceOnExistingCircuits bool
}

// DefaultIsolationPolicy returns a sensible default policy.
func DefaultIsolationPolicy() *IsolationPolicy {
	return &IsolationPolicy{
		Mode:                      IsolationModeOff,
		IsolateBySOCKSAuth:        false,
		IsolateByDestination:      false,
		IsolateBySourcePort:       false,
		IsolateBySession:          false,
		EnforceOnExistingCircuits: true,
	}
}

// StrictIsolationPolicy returns a policy with strict enforcement.
func StrictIsolationPolicy() *IsolationPolicy {
	return &IsolationPolicy{
		Mode:                      IsolationModeStrict,
		IsolateBySOCKSAuth:        true,
		IsolateByDestination:      true,
		IsolateBySourcePort:       false,
		IsolateBySession:          true,
		EnforceOnExistingCircuits: true,
	}
}

// IsolationEnforcer validates and enforces stream isolation policies.
// It ensures that streams with different isolation requirements do not share circuits.
type IsolationEnforcer struct {
	policy *IsolationPolicy
	logger *logger.Logger

	// Track circuits and their isolation keys
	mu             sync.RWMutex
	circuitKeys    map[uint32]*circuit.IsolationKey // circuitID -> isolation key
	circuitStreams map[uint32][]uint16              // circuitID -> stream IDs
}

// NewIsolationEnforcer creates a new isolation enforcer with the given policy.
func NewIsolationEnforcer(policy *IsolationPolicy, log *logger.Logger) *IsolationEnforcer {
	if policy == nil {
		policy = DefaultIsolationPolicy()
	}
	if log == nil {
		log = logger.NewDefault()
	}

	return &IsolationEnforcer{
		policy:         policy,
		logger:         log.Component("isolation-enforcer"),
		circuitKeys:    make(map[uint32]*circuit.IsolationKey),
		circuitStreams: make(map[uint32][]uint16),
	}
}

// StreamRequest represents a request to create a stream with isolation context.
type StreamRequest struct {
	// Target is the destination address (host:port).
	Target string

	// SourceAddr is the client's source address.
	SourceAddr net.Addr

	// SOCKSUsername is the SOCKS5 authentication username (if any).
	SOCKSUsername string

	// SessionToken is an explicit session identifier (if any).
	SessionToken string
}

// IsolationResult contains the result of isolation validation.
type IsolationResult struct {
	// Allowed indicates if the request is allowed.
	Allowed bool

	// Key is the isolation key for this request.
	Key *circuit.IsolationKey

	// Reason describes why the request was denied (if not allowed).
	Reason string

	// Warnings contains any non-fatal isolation issues.
	Warnings []string
}

// ValidateStreamRequest checks if a stream request satisfies isolation requirements.
// It returns an IsolationKey if isolation is needed, or nil if no isolation is required.
func (e *IsolationEnforcer) ValidateStreamRequest(req *StreamRequest) *IsolationResult {
	if e.policy.Mode == IsolationModeOff {
		return &IsolationResult{Allowed: true, Key: nil}
	}

	result := &IsolationResult{Allowed: true, Warnings: make([]string, 0)}

	// Determine isolation level based on policy
	level := e.determineIsolationLevel(req)
	if level == circuit.IsolationNone {
		return result
	}

	// Build isolation key based on the determined level
	key := circuit.NewIsolationKey(level)
	result.Key = key

	switch level {
	case circuit.IsolationDestination:
		if req.Target == "" {
			result.Warnings = append(result.Warnings, "destination isolation requested but no target specified")
		}
		key.WithDestination(req.Target)

	case circuit.IsolationCredential:
		if req.SOCKSUsername == "" {
			if e.policy.Mode == IsolationModeStrict {
				result.Allowed = false
				result.Reason = "credential isolation requires SOCKS5 authentication"
				return result
			}
			result.Warnings = append(result.Warnings, "credential isolation requested but no username provided")
		} else {
			key.WithCredentials(req.SOCKSUsername)
		}

	case circuit.IsolationPort:
		port := e.extractSourcePort(req.SourceAddr)
		if port == 0 {
			result.Warnings = append(result.Warnings, "port isolation requested but source port not available")
		}
		key.WithSourcePort(port)

	case circuit.IsolationSession:
		if req.SessionToken == "" {
			if e.policy.Mode == IsolationModeStrict {
				result.Allowed = false
				result.Reason = "session isolation requires session token"
				return result
			}
			result.Warnings = append(result.Warnings, "session isolation requested but no token provided")
		} else {
			key.WithSessionToken(req.SessionToken)
		}
	}

	// Validate the key
	if err := key.Validate(); err != nil {
		if e.policy.Mode == IsolationModeStrict {
			result.Allowed = false
			result.Reason = fmt.Sprintf("invalid isolation key: %v", err)
			return result
		}
		result.Warnings = append(result.Warnings, fmt.Sprintf("isolation key validation failed: %v", err))
	}

	return result
}

// determineIsolationLevel determines the appropriate isolation level based on policy.
func (e *IsolationEnforcer) determineIsolationLevel(req *StreamRequest) circuit.IsolationLevel {
	// Priority order: session > credential > destination > port
	if e.policy.IsolateBySession && req.SessionToken != "" {
		return circuit.IsolationSession
	}
	if e.policy.IsolateBySOCKSAuth && req.SOCKSUsername != "" {
		return circuit.IsolationCredential
	}
	if e.policy.IsolateByDestination && req.Target != "" {
		return circuit.IsolationDestination
	}
	if e.policy.IsolateBySourcePort {
		return circuit.IsolationPort
	}
	return circuit.IsolationNone
}

// extractSourcePort extracts the source port from a network address.
func (e *IsolationEnforcer) extractSourcePort(addr net.Addr) uint16 {
	if addr == nil {
		return 0
	}
	switch a := addr.(type) {
	case *net.TCPAddr:
		return uint16(a.Port)
	case *net.UDPAddr:
		return uint16(a.Port)
	default:
		// Try parsing as string "host:port"
		_, portStr, err := net.SplitHostPort(addr.String())
		if err != nil {
			return 0
		}
		var port uint16
		if _, err := fmt.Sscanf(portStr, "%d", &port); err == nil {
			return port
		}
		return 0
	}
}

// CheckCircuitCompatibility checks if a stream can use an existing circuit.
// Returns true if the circuit's isolation key is compatible with the stream's requirements.
func (e *IsolationEnforcer) CheckCircuitCompatibility(
	circuitID uint32,
	streamKey *circuit.IsolationKey,
) (bool, string) {
	if !e.policy.EnforceOnExistingCircuits {
		return true, ""
	}

	e.mu.RLock()
	circuitKey, exists := e.circuitKeys[circuitID]
	e.mu.RUnlock()

	if !exists {
		// No isolation set on circuit, allow if stream has no isolation
		if streamKey == nil || streamKey.Level == circuit.IsolationNone {
			return true, ""
		}
		// Circuit has no isolation, but stream requires it
		if e.policy.Mode == IsolationModeStrict {
			return false, "stream requires isolation but circuit has none"
		}
		e.logger.Warn("Stream isolation mismatch",
			"circuit_id", circuitID,
			"stream_key", streamKey.String(),
			"circuit_key", "none")
		return true, ""
	}

	// Both have isolation - check compatibility
	if streamKey == nil || streamKey.Level == circuit.IsolationNone {
		// Stream has no isolation but circuit does - allowed
		return true, ""
	}

	// Check if keys match
	if !streamKey.Equals(circuitKey) {
		msg := fmt.Sprintf("isolation key mismatch: stream=%s, circuit=%s",
			streamKey.String(), circuitKey.String())

		if e.policy.Mode == IsolationModeStrict {
			return false, msg
		}

		e.logger.Warn("Stream isolation mismatch (allowed)", "details", msg)
	}

	return true, ""
}

// RegisterCircuit registers a circuit with its isolation key.
// If the circuit is already registered, this is a no-op to preserve existing stream tracking.
// The original isolation key is preserved when re-registering to maintain consistency.
func (e *IsolationEnforcer) RegisterCircuit(circuitID uint32, key *circuit.IsolationKey) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Check if circuit already exists to preserve stream tracking
	if existingKey, exists := e.circuitKeys[circuitID]; exists {
		e.logger.Debug("Circuit already registered, preserving existing key and streams",
			"circuit_id", circuitID,
			"existing_key", existingKey,
			"attempted_key", key)
		return
	}

	e.circuitKeys[circuitID] = key
	e.circuitStreams[circuitID] = make([]uint16, 0)

	e.logger.Debug("Circuit registered",
		"circuit_id", circuitID,
		"isolation_key", key)
}

// RegisterStream registers a stream on a circuit.
func (e *IsolationEnforcer) RegisterStream(circuitID uint32, streamID uint16) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.circuitStreams[circuitID] = append(e.circuitStreams[circuitID], streamID)

	e.logger.Debug("Stream registered",
		"circuit_id", circuitID,
		"stream_id", streamID)
}

// UnregisterCircuit removes a circuit and all its streams from tracking.
func (e *IsolationEnforcer) UnregisterCircuit(circuitID uint32) {
	e.mu.Lock()
	defer e.mu.Unlock()

	delete(e.circuitKeys, circuitID)
	delete(e.circuitStreams, circuitID)

	e.logger.Debug("Circuit unregistered", "circuit_id", circuitID)
}

// UnregisterStream removes a stream from tracking.
func (e *IsolationEnforcer) UnregisterStream(circuitID uint32, streamID uint16) {
	e.mu.Lock()
	defer e.mu.Unlock()

	streams := e.circuitStreams[circuitID]
	for i, id := range streams {
		if id == streamID {
			e.circuitStreams[circuitID] = append(streams[:i], streams[i+1:]...)
			break
		}
	}

	e.logger.Debug("Stream unregistered",
		"circuit_id", circuitID,
		"stream_id", streamID)
}

// GetCircuitIsolationKey returns the isolation key for a circuit.
func (e *IsolationEnforcer) GetCircuitIsolationKey(circuitID uint32) *circuit.IsolationKey {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.circuitKeys[circuitID]
}

// IsolationStats contains statistics about tracked circuits and streams.
type IsolationStats struct {
	TrackedCircuits  int
	TrackedStreams   int
	IsolatedCircuits int // Number of circuits that have isolation enabled
}

// Stats returns current isolation tracking statistics.
func (e *IsolationEnforcer) Stats() IsolationStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	totalStreams := 0
	isolatedCircuits := 0
	for circuitID, streams := range e.circuitStreams {
		totalStreams += len(streams)
		if key, exists := e.circuitKeys[circuitID]; exists && key != nil && key.Level != circuit.IsolationNone {
			isolatedCircuits++
		}
	}

	return IsolationStats{
		TrackedCircuits:  len(e.circuitKeys),
		TrackedStreams:   totalStreams,
		IsolatedCircuits: isolatedCircuits,
	}
}

// Policy returns the current isolation policy.
func (e *IsolationEnforcer) Policy() *IsolationPolicy {
	return e.policy
}

// SetPolicy updates the isolation policy.
func (e *IsolationEnforcer) SetPolicy(policy *IsolationPolicy) {
	if policy == nil {
		policy = DefaultIsolationPolicy()
	}
	e.policy = policy
	e.logger.Info("Isolation policy updated", "mode", policy.Mode)
}
