// Package circuit provides circuit management for the Tor protocol.
// Circuits are paths through the Tor network used to route traffic.
package circuit

import (
	"context"
	"fmt"
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
	mu              sync.RWMutex
	paddingEnabled  bool          // SPEC-002: Enable/disable circuit padding
	paddingInterval time.Duration // SPEC-002: Interval for padding cells
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
	return &Circuit{
		ID:              id,
		State:           StateBuilding,
		CreatedAt:       time.Now(),
		Hops:            make([]*Hop, 0, 3), // Typical circuit has 3 hops
		paddingEnabled:  true,               // SPEC-002: Enable padding by default
		paddingInterval: 0,                  // SPEC-002: 0 = adaptive (future enhancement)
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
// This is a hook for implementing adaptive padding logic per padding-spec.txt
// Current implementation returns basic policy; future versions can implement:
// - Traffic pattern analysis
// - Time-based adaptive padding
// - Connection state-dependent padding
// - Burst-detection and response
func (c *Circuit) ShouldSendPadding() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Basic policy: padding enabled and circuit is open
	if !c.paddingEnabled || c.State != StateOpen {
		return false
	}

	// Future enhancement: implement adaptive logic here per padding-spec.txt
	// - Analyze traffic patterns
	// - Adjust padding based on activity
	// - Implement timing-based policies

	return true
}
