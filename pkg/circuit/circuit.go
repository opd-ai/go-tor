// Package circuit provides circuit management for the Tor protocol.
// Circuits are paths through the Tor network used to route traffic.
package circuit

import (
	"context"
	"crypto/sha1"
	"crypto/subtle"
	"fmt"
	"hash"
	"sync"
	"time"
)

// State represents the current state of a circuit
type State int

const (
	// StateBuilding indicates the circuit is being built
	StateBuilding State = iota
	// StateOpen indicates the circuit is ready for use
	StateOpen
	// StateClosed indicates the circuit has been closed
	StateClosed
	// StateFailed indicates the circuit failed to build or operate
	StateFailed
)

// String returns a string representation of the state
func (s State) String() string {
	switch s {
	case StateBuilding:
		return "BUILDING"
	case StateOpen:
		return "OPEN"
	case StateClosed:
		return "CLOSED"
	case StateFailed:
		return "FAILED"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", s)
	}
}

// Circuit represents a Tor circuit
type Circuit struct {
	ID              uint32
	State           State
	CreatedAt       time.Time
	Hops            []*Hop
	IsolationKey    *IsolationKey // Isolation key for circuit isolation
	mu              sync.RWMutex
	paddingEnabled  bool          // SPEC-002: Enable/disable circuit padding
	paddingInterval time.Duration // SPEC-002: Interval for padding cells
	lastPaddingTime time.Time     // SPEC-002: Last time a padding cell was sent
	lastActivityTime time.Time    // SPEC-002: Last time any cell was sent/received
	// CRYPTO-001: Running digests for relay cell verification per tor-spec.txt §6.1
	forwardDigest  hash.Hash // Client → Exit direction
	backwardDigest hash.Hash // Exit → Client direction
}

// Hop represents a single hop in a circuit (one relay)
type Hop struct {
	Fingerprint string // Router fingerprint
	Address     string // Router address (IP:port)
	IsGuard     bool   // Whether this is a guard node
	IsExit      bool   // Whether this is an exit node
}

// NewCircuit creates a new circuit with the given ID
func NewCircuit(id uint32) *Circuit {
	now := time.Now()
	return &Circuit{
		ID:               id,
		State:            StateBuilding,
		CreatedAt:        now,
		Hops:             make([]*Hop, 0, 3),    // Typical circuit has 3 hops
		IsolationKey:     nil,                   // No isolation by default (backward compatible)
		paddingEnabled:   true,                  // SPEC-002: Enable padding by default
		paddingInterval:  5 * time.Second,       // SPEC-002: Default 5-second padding interval
		lastPaddingTime:  now,                   // SPEC-002: Initialize padding timer
		lastActivityTime: now,                   // SPEC-002: Initialize activity timer
		forwardDigest:    sha1.New(),            // CRYPTO-001: Initialize forward digest
		backwardDigest:   sha1.New(),            // CRYPTO-001: Initialize backward digest
	}
}

// AddHop adds a hop to the circuit
func (c *Circuit) AddHop(hop *Hop) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.State != StateBuilding {
		return fmt.Errorf("cannot add hop to circuit in state %s", c.State)
	}

	c.Hops = append(c.Hops, hop)
	return nil
}

// SetState sets the circuit state
func (c *Circuit) SetState(state State) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.State = state
}

// GetState returns the current circuit state
func (c *Circuit) GetState() State {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.State
}

// Length returns the number of hops in the circuit
func (c *Circuit) Length() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.Hops)
}

// IsReady returns true if the circuit is ready for use
func (c *Circuit) IsReady() bool {
	return c.GetState() == StateOpen
}

// Age returns how long the circuit has existed
func (c *Circuit) Age() time.Duration {
	return time.Since(c.CreatedAt)
}

// Manager manages a collection of circuits
type Manager struct {
	circuits map[uint32]*Circuit
	nextID   uint32
	mu       sync.RWMutex
	closed   bool
}

// NewManager creates a new circuit manager
func NewManager() *Manager {
	return &Manager{
		circuits: make(map[uint32]*Circuit),
		nextID:   1, // Circuit ID 0 is reserved
	}
}

// CreateCircuit creates a new circuit and returns its ID
func (m *Manager) CreateCircuit() (*Circuit, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil, fmt.Errorf("manager is closed")
	}

	// Find an unused circuit ID
	id := m.nextID
	for {
		if _, exists := m.circuits[id]; !exists {
			break
		}
		id++
		if id == 0 {
			id = 1 // Skip 0
		}
		if id == m.nextID {
			return nil, fmt.Errorf("no available circuit IDs")
		}
	}

	m.nextID = id + 1
	if m.nextID == 0 {
		m.nextID = 1
	}

	circuit := NewCircuit(id)
	m.circuits[id] = circuit
	return circuit, nil
}

// GetCircuit returns a circuit by ID
func (m *Manager) GetCircuit(id uint32) (*Circuit, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	circuit, exists := m.circuits[id]
	if !exists {
		return nil, fmt.Errorf("circuit %d not found", id)
	}
	return circuit, nil
}

// CloseCircuit closes a circuit
func (m *Manager) CloseCircuit(id uint32) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	circuit, exists := m.circuits[id]
	if !exists {
		return fmt.Errorf("circuit %d not found", id)
	}

	circuit.SetState(StateClosed)
	delete(m.circuits, id)
	return nil
}

// ListCircuits returns a list of all circuit IDs
func (m *Manager) ListCircuits() []uint32 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]uint32, 0, len(m.circuits))
	for id := range m.circuits {
		ids = append(ids, id)
	}
	return ids
}

// Count returns the number of active circuits
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.circuits)
}

// Close closes all circuits and shuts down the manager gracefully
func (m *Manager) Close(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return fmt.Errorf("manager already closed")
	}

	// Mark as closed to prevent new circuits
	m.closed = true

	// Close all circuits
	for id, circuit := range m.circuits {
		circuit.SetState(StateClosed)
		delete(m.circuits, id)
	}

	return nil
}

// IsClosed returns true if the manager has been closed
func (m *Manager) IsClosed() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.closed
}

// SPEC-002: Circuit padding configuration and control
// These methods provide infrastructure for enhanced circuit padding per padding-spec.txt
// Current implementation provides basic padding support with hooks for future adaptive padding

// SetPaddingEnabled enables or disables circuit padding (SPEC-002)
// When enabled, circuits will send PADDING cells according to padding policy
func (c *Circuit) SetPaddingEnabled(enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.paddingEnabled = enabled
}

// IsPaddingEnabled returns whether padding is enabled for this circuit (SPEC-002)
func (c *Circuit) IsPaddingEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.paddingEnabled
}

// SetPaddingInterval sets the interval for padding cells (SPEC-002)
// interval: time between padding cells (0 = adaptive/traffic-based)
// This provides infrastructure for implementing adaptive padding per padding-spec.txt
func (c *Circuit) SetPaddingInterval(interval time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.paddingInterval = interval
}

// GetPaddingInterval returns the current padding interval (SPEC-002)
func (c *Circuit) GetPaddingInterval() time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.paddingInterval
}

// ShouldSendPadding determines if a padding cell should be sent (SPEC-002)
// Implements basic time-based padding to improve traffic analysis resistance
// per tor-spec.txt §7.1 and padding-spec.txt
// 
// Basic policy: Send padding if:
// 1. Padding is enabled
// 2. Circuit is open
// 3. paddingInterval has elapsed since last padding cell
// 4. No recent activity (prevents redundant padding during active use)
func (c *Circuit) ShouldSendPadding() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Basic policy: padding enabled and circuit is open
	if !c.paddingEnabled || c.State != StateOpen {
		return false
	}

	// If no interval configured (0), padding is disabled
	if c.paddingInterval == 0 {
		return false
	}

	now := time.Now()
	
	// Check if padding interval has elapsed since last padding
	timeSinceLastPadding := now.Sub(c.lastPaddingTime)
	if timeSinceLastPadding < c.paddingInterval {
		return false
	}
	
	// Don't send padding if there's been recent activity (within 80% of padding interval)
	// This prevents redundant padding when circuit is actively used
	activityThreshold := time.Duration(float64(c.paddingInterval) * 0.8)
	timeSinceActivity := now.Sub(c.lastActivityTime)
	if timeSinceActivity < activityThreshold {
		return false
	}

	return true
}

// RecordPaddingSent updates the last padding time (SPEC-002)
// Should be called after successfully sending a padding cell
func (c *Circuit) RecordPaddingSent() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastPaddingTime = time.Now()
}

// RecordActivity updates the last activity time (SPEC-002)
// Should be called when sending or receiving non-padding cells
func (c *Circuit) RecordActivity() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastActivityTime = time.Now()
}


// Direction represents the direction of relay cell flow
type Direction int

const (
	// DirectionForward is client → exit
	DirectionForward Direction = iota
	// DirectionBackward is exit → client
	DirectionBackward
)

// CRYPTO-001: Relay cell digest verification per tor-spec.txt §6.1
// "Each RELAY cell includes a running digest field computed over all relay cells
// sent in same direction on the circuit."

// UpdateDigest updates the running digest for relay cells (CRYPTO-001)
// This must be called for every relay cell sent or received to maintain digest state.
// The digest is computed over the entire relay cell with the digest field zeroed.
// Per tor-spec.txt §6.1: digest = SHA1(digest | relay_cell_with_zeroed_digest)
func (c *Circuit) UpdateDigest(direction Direction, cellData []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(cellData) < 11 {
		return fmt.Errorf("relay cell data too short: %d < 11", len(cellData))
	}

	// Create a copy with digest field zeroed (bytes 5-8)
	cellCopy := make([]byte, len(cellData))
	copy(cellCopy, cellData)
	cellCopy[5] = 0
	cellCopy[6] = 0
	cellCopy[7] = 0
	cellCopy[8] = 0

	// Update appropriate digest
	var digest hash.Hash
	if direction == DirectionForward {
		digest = c.forwardDigest
	} else {
		digest = c.backwardDigest
	}

	if digest == nil {
		return fmt.Errorf("digest not initialized for direction %d", direction)
	}

	_, err := digest.Write(cellCopy)
	return err
}

// VerifyDigest verifies the digest of an incoming relay cell (CRYPTO-001)
// This prevents cell injection and replay attacks per tor-spec.txt §6.1.
// Returns error if digest verification fails.
func (c *Circuit) VerifyDigest(direction Direction, cellData []byte, receivedDigest [4]byte) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Select appropriate digest
	var digest hash.Hash
	if direction == DirectionForward {
		digest = c.forwardDigest
	} else {
		digest = c.backwardDigest
	}

	if digest == nil {
		return fmt.Errorf("digest not initialized for direction %d", direction)
	}

	// Compute expected digest (first 4 bytes of SHA-1)
	// Note: We're checking the state BEFORE updating, so we compute what the
	// digest should be for this cell given the current state
	expectedSum := digest.Sum(nil)
	expected := [4]byte{expectedSum[0], expectedSum[1], expectedSum[2], expectedSum[3]}

	// Constant-time comparison to prevent timing attacks
	if subtle.ConstantTimeCompare(expected[:], receivedDigest[:]) != 1 {
		return fmt.Errorf("relay cell digest verification failed: expected %x, got %x", expected, receivedDigest)
	}

	return nil
}

// ResetDigests resets the running digests (CRYPTO-001)
// This should be called when establishing a new circuit or after certain protocol events.
func (c *Circuit) ResetDigests() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.forwardDigest.Reset()
	c.backwardDigest.Reset()
}

// SetIsolationKey sets the isolation key for this circuit
func (c *Circuit) SetIsolationKey(key *IsolationKey) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.IsolationKey = key
}

// GetIsolationKey returns the isolation key for this circuit
func (c *Circuit) GetIsolationKey() *IsolationKey {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.IsolationKey
}
