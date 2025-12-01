// Package circuit provides circuit management for the Tor protocol.
// Circuits are paths through the Tor network used to route traffic.
package circuit

import (
	"context"
	"crypto/cipher"
	"crypto/sha1" // #nosec G505 - SHA-1 required by Tor protocol (tor-spec.txt §6.1)
	"crypto/subtle"
	"encoding/binary"
	"fmt"
	"hash"
	"io"
	"sync"
	"time"

	"github.com/opd-ai/go-tor/pkg/cell"
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
	ID               uint32
	State            State
	CreatedAt        time.Time
	Hops             []*Hop
	IsolationKey     *IsolationKey // Isolation key for circuit isolation
	conn             interface{}   // Connection to the entry guard (interface{} to avoid circular import)
	mu               sync.RWMutex
	paddingEnabled   bool          // SPEC-002: Enable/disable circuit padding
	paddingInterval  time.Duration // SPEC-002: Interval for padding cells
	lastPaddingTime  time.Time     // SPEC-002: Last time a padding cell was sent
	lastActivityTime time.Time     // SPEC-002: Last time any cell was sent/received
	// CRYPTO-001: Running digests for relay cell verification per tor-spec.txt §6.1
	forwardDigest  hash.Hash // Client → Exit direction
	backwardDigest hash.Hash // Exit → Client direction
	// Stream protocol support
	relayReceiveChan chan *cell.RelayCell // Channel for receiving relay cells
	streamManager    interface{}          // Stream manager (interface{} to avoid circular import)
	// Flow control per tor-spec.txt §7.4
	packageWindow  int // Circuit-level package window (cells we can send)
	deliverWindow  int // Circuit-level deliver window (cells we can receive)
	sendmeReceived int // Count of DATA cells received (for sending SENDME)
	sendmeSent     int // Count of SENDME cells sent
	// SECURITY-001: Replay protection per tor-spec.txt
	replayProtection *cell.ReplayProtection // Replay protection for cells
}

// Hop represents a single hop in a circuit (one relay)
type Hop struct {
	Fingerprint string // Router fingerprint
	Address     string // Router address (IP:port)
	IsGuard     bool   // Whether this is a guard node
	IsExit      bool   // Whether this is an exit node

	// Cryptographic state for this hop (per tor-spec.txt §5.2)
	// These are derived from the key material during circuit extension
	ForwardCipher  cipher.Stream // AES-CTR cipher for encrypting cells (client→relay)
	BackwardCipher cipher.Stream // AES-CTR cipher for decrypting cells (relay→client)
	ForwardDigest  hash.Hash     // SHA-1 running digest for forward direction
	BackwardDigest hash.Hash     // SHA-1 running digest for backward direction
}

// NewHop creates a new hop with the given parameters
func NewHop(fingerprint, address string, isGuard, isExit bool) *Hop {
	return &Hop{
		Fingerprint: fingerprint,
		Address:     address,
		IsGuard:     isGuard,
		IsExit:      isExit,
	}
}

// SetCryptoState sets the cryptographic state for this hop
// This should be called after circuit extension when key material is derived
func (h *Hop) SetCryptoState(forwardCipher, backwardCipher cipher.Stream, forwardDigest, backwardDigest hash.Hash) {
	h.ForwardCipher = forwardCipher
	h.BackwardCipher = backwardCipher
	h.ForwardDigest = forwardDigest
	h.BackwardDigest = backwardDigest
}

// NewCircuit creates a new circuit with the given ID
func NewCircuit(id uint32) *Circuit {
	now := time.Now()
	return &Circuit{
		ID:               id,
		State:            StateBuilding,
		CreatedAt:        now,
		Hops:             make([]*Hop, 0, 3),             // Typical circuit has 3 hops
		IsolationKey:     nil,                            // No isolation by default (backward compatible)
		conn:             nil,                            // Connection set later
		paddingEnabled:   true,                           // SPEC-002: Enable padding by default
		paddingInterval:  5 * time.Second,                // SPEC-002: Default 5-second padding interval
		lastPaddingTime:  now,                            // SPEC-002: Initialize padding timer
		lastActivityTime: now,                            // SPEC-002: Initialize activity timer
		forwardDigest:    sha1.New(),                     // CRYPTO-001: Initialize forward digest
		backwardDigest:   sha1.New(),                     // CRYPTO-001: Initialize backward digest
		relayReceiveChan: make(chan *cell.RelayCell, 32), // Buffer for incoming relay cells
		streamManager:    nil,                            // Stream manager set later
		packageWindow:    1000,                           // tor-spec.txt §7.4: Initial circuit window is 1000
		deliverWindow:    1000,                           // tor-spec.txt §7.4: Initial circuit window is 1000
		sendmeReceived:   0,                              // No DATA cells received yet
		sendmeSent:       0,                              // No SENDME cells sent yet
		replayProtection: cell.NewReplayProtection(),     // SECURITY-001: Initialize replay protection
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

// SetConnection sets the underlying connection for this circuit
// conn should be a *connection.Connection, but we use interface{} to avoid circular imports
func (c *Circuit) SetConnection(conn interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.conn = conn
}

// SetStreamManager sets the stream manager for this circuit
// mgr should be a *stream.Manager, but we use interface{} to avoid circular imports
func (c *Circuit) SetStreamManager(mgr interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.streamManager = mgr
}

// encryptForward encrypts a relay cell payload with each hop's forward cipher
// This implements the onion encryption per tor-spec.txt §6.1
// The payload is encrypted in ORDER (guard -> middle -> exit) so the exit node decrypts last
func (c *Circuit) encryptForward(payload []byte) []byte {
	c.mu.RLock()
	hops := c.Hops
	c.mu.RUnlock()

	// Make a copy to avoid modifying the original
	encrypted := make([]byte, len(payload))
	copy(encrypted, payload)

	// Encrypt with each hop's cipher in forward order (guard -> middle -> exit)
	// Each hop will decrypt one layer, like peeling an onion
	for i := len(hops) - 1; i >= 0; i-- {
		hop := hops[i]
		if hop.ForwardCipher != nil {
			// XOR with the cipher stream (AES-CTR encryption)
			hop.ForwardCipher.XORKeyStream(encrypted, encrypted)
		}
	}

	return encrypted
}

// decryptBackward decrypts a relay cell payload from the circuit
// This implements the onion decryption per tor-spec.txt §6.1
// The payload is decrypted in REVERSE order (exit -> middle -> guard)
func (c *Circuit) decryptBackward(payload []byte) []byte {
	c.mu.RLock()
	hops := c.Hops
	c.mu.RUnlock()

	// Make a copy to avoid modifying the original
	decrypted := make([]byte, len(payload))
	copy(decrypted, payload)

	// Decrypt with each hop's cipher in reverse order (exit -> middle -> guard)
	// We receive the cell from the guard, which is the last to encrypt (first to decrypt)
	for _, hop := range hops {
		if hop.BackwardCipher != nil {
			// XOR with the cipher stream (AES-CTR decryption)
			hop.BackwardCipher.XORKeyStream(decrypted, decrypted)
		}
	}

	return decrypted
}

// updateHopDigests updates the per-hop running digests for a relay cell
// This is called after encryption/decryption to update each hop's digest state
func (c *Circuit) updateHopDigests(direction Direction, payload []byte) error {
	c.mu.RLock()
	hops := c.Hops
	c.mu.RUnlock()

	if len(payload) < 11 {
		return fmt.Errorf("relay cell data too short: %d < 11", len(payload))
	}

	// Create a copy with digest field zeroed (bytes 5-8)
	cellCopy := make([]byte, len(payload))
	copy(cellCopy, payload)
	cellCopy[5] = 0
	cellCopy[6] = 0
	cellCopy[7] = 0
	cellCopy[8] = 0

	// Update the appropriate digest for each hop
	if direction == DirectionForward {
		// Forward: update each hop's forward digest
		for _, hop := range hops {
			if hop.ForwardDigest != nil {
				if _, err := hop.ForwardDigest.Write(cellCopy); err != nil {
					return fmt.Errorf("failed to update forward digest for hop: %w", err)
				}
			}
		}
	} else {
		// Backward: update each hop's backward digest
		for _, hop := range hops {
			if hop.BackwardDigest != nil {
				if _, err := hop.BackwardDigest.Write(cellCopy); err != nil {
					return fmt.Errorf("failed to update backward digest for hop: %w", err)
				}
			}
		}
	}

	return nil
}

// verifyRelayCellDigest verifies the digest of an incoming relay cell
// Returns the hop index that recognized the cell, or -1 if unrecognized
func (c *Circuit) verifyRelayCellDigest(payload []byte) (int, error) {
	c.mu.RLock()
	hops := c.Hops
	c.mu.RUnlock()

	if len(payload) < 11 {
		return -1, fmt.Errorf("relay cell payload too short: %d < 11", len(payload))
	}

	// Extract the digest from the cell (bytes 5-8)
	var cellDigest [4]byte
	copy(cellDigest[:], payload[5:9])

	// Check if this cell is recognized by any hop
	// A cell is "recognized" if:
	// 1. The digest matches the hop's running backward digest
	// 2. The "recognized" field is zero (bytes 1-2)

	recognized := binary.BigEndian.Uint16(payload[1:3])

	// Try each hop to see which one recognizes this cell
	for hopIdx, hop := range hops {
		if hop.BackwardDigest == nil {
			continue
		}

		// Compute expected digest for this hop
		// Create a copy with digest zeroed
		cellCopy := make([]byte, len(payload))
		copy(cellCopy, payload)
		cellCopy[5] = 0
		cellCopy[6] = 0
		cellCopy[7] = 0
		cellCopy[8] = 0

		// Get the current digest state (without modifying it)
		expectedSum := hop.BackwardDigest.Sum(nil)
		expected := [4]byte{expectedSum[0], expectedSum[1], expectedSum[2], expectedSum[3]}

		// Check if digest matches AND recognized field is zero
		if subtle.ConstantTimeCompare(expected[:], cellDigest[:]) == 1 && recognized == 0 {
			// This hop recognizes the cell
			// Now update the digest with this cell
			if _, err := hop.BackwardDigest.Write(cellCopy); err != nil {
				return -1, fmt.Errorf("failed to update backward digest: %w", err)
			}
			return hopIdx, nil
		}
	}

	// No hop recognized this cell - might be for a stream we don't have
	// or an error condition
	return -1, nil
}

// decrementPackageWindow decrements the circuit-level package window
// Returns an error if the window is exhausted
func (c *Circuit) decrementPackageWindow() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.packageWindow <= 0 {
		return fmt.Errorf("package window exhausted: cannot send more cells until SENDME received")
	}

	c.packageWindow--
	return nil
}

// incrementPackageWindow increments the circuit-level package window
// This is called when we receive a SENDME cell
func (c *Circuit) incrementPackageWindow() {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Per tor-spec.txt §7.4, each SENDME increments the window by 100
	c.packageWindow += 100
}

// decrementDeliverWindow decrements the circuit-level deliver window
// Returns an error if the window is exhausted
func (c *Circuit) decrementDeliverWindow() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.deliverWindow <= 0 {
		return fmt.Errorf("deliver window exhausted: cannot receive more cells until SENDME sent")
	}

	c.deliverWindow--
	c.sendmeReceived++

	return nil
}

// shouldSendCircuitSendme checks if we should send a circuit-level SENDME
// Per tor-spec.txt §7.4, send SENDME every 100 cells received
func (c *Circuit) shouldSendCircuitSendme() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.sendmeReceived >= 100
}

// sendCircuitSendme sends a circuit-level SENDME cell
func (c *Circuit) sendCircuitSendme() error {
	c.mu.Lock()
	c.sendmeReceived = 0
	c.sendmeSent++
	c.deliverWindow += 100 // Increment our deliver window
	c.mu.Unlock()

	// Send SENDME cell (stream ID 0 indicates circuit-level)
	sendmeCell := cell.NewRelayCell(0, cell.RelaySendme, []byte{})
	return c.SendRelayCell(sendmeCell)
}

// SendRelayCell sends a relay cell through the circuit
// This encrypts the relay cell with per-hop cryptography and sends it through the connection
func (c *Circuit) SendRelayCell(relayCell *cell.RelayCell) error {
	// Check flow control for DATA cells
	// Per tor-spec.txt §7.4, only DATA cells count against the package window
	if relayCell.Command == cell.RelayData {
		if err := c.decrementPackageWindow(); err != nil {
			return fmt.Errorf("flow control: %w", err)
		}
	}

	c.mu.Lock()
	conn := c.conn
	state := c.State
	hops := c.Hops
	c.mu.Unlock()

	if state != StateOpen {
		return fmt.Errorf("circuit not open: state=%s", state)
	}

	if conn == nil {
		return fmt.Errorf("circuit has no connection")
	}

	// Encode the relay cell (digest field will be zeroed initially)
	payload, err := relayCell.Encode()
	if err != nil {
		return fmt.Errorf("failed to encode relay cell: %w", err)
	}

	// Compute the digest for the exit hop (last hop in the circuit)
	// Per tor-spec.txt §6.1, each hop maintains its own running digest
	if len(hops) > 0 {
		exitHop := hops[len(hops)-1]
		if exitHop.ForwardDigest != nil {
			// Create a copy with digest zeroed for digest computation
			cellCopy := make([]byte, len(payload))
			copy(cellCopy, payload)
			cellCopy[5] = 0
			cellCopy[6] = 0
			cellCopy[7] = 0
			cellCopy[8] = 0

			// Update the exit hop's forward digest
			if _, err := exitHop.ForwardDigest.Write(cellCopy); err != nil {
				return fmt.Errorf("failed to update forward digest: %w", err)
			}

			// Get the digest and set it in the payload
			digestSum := exitHop.ForwardDigest.Sum(nil)
			payload[5] = digestSum[0]
			payload[6] = digestSum[1]
			payload[7] = digestSum[2]
			payload[8] = digestSum[3]
		}
	}

	// Encrypt the payload with per-hop cryptography (onion encryption)
	// Each hop will decrypt one layer
	encryptedPayload := c.encryptForward(payload)

	// Create a RELAY cell with the encrypted payload
	cellToSend := &cell.Cell{
		CircID:  c.ID,
		Command: cell.CmdRelay,
		Payload: encryptedPayload,
	}

	// Send through connection (type assert to interface with SendCell method)
	type cellSender interface {
		SendCell(*cell.Cell) error
	}
	sender, ok := conn.(cellSender)
	if !ok {
		return fmt.Errorf("connection does not support SendCell")
	}

	if err := sender.SendCell(cellToSend); err != nil {
		return fmt.Errorf("failed to send cell: %w", err)
	}

	// Record activity
	c.RecordActivity()

	return nil
}

// ReceiveRelayCell receives a relay cell from the circuit
// This blocks until a relay cell is received or the context is cancelled
func (c *Circuit) ReceiveRelayCell(ctx context.Context) (*cell.RelayCell, error) {
	select {
	case relayCell := <-c.relayReceiveChan:
		return relayCell, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// ReceiveRelayCellTimeout receives a relay cell with a timeout
func (c *Circuit) ReceiveRelayCellTimeout(timeout time.Duration) (*cell.RelayCell, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return c.ReceiveRelayCell(ctx)
}

// DeliverRelayCell delivers a relay cell to this circuit (called by connection layer)
// This decrypts the cell, verifies the digest, handles flow control, and pushes it to the receive channel
func (c *Circuit) DeliverRelayCell(cellData *cell.Cell) error {
	if cellData.CircID != c.ID {
		return fmt.Errorf("circuit ID mismatch: expected %d, got %d", c.ID, cellData.CircID)
	}

	// Decrypt the relay cell with per-hop cryptography (onion decryption)
	// Each hop decrypts one layer
	decryptedPayload := c.decryptBackward(cellData.Payload)

	// SECURITY-001: Validate against replay attacks before processing
	// We check the decrypted payload to ensure the same cell content isn't replayed
	if c.replayProtection != nil {
		// Get next sequence for backward direction
		c.mu.Lock()
		seqNum := c.replayProtection.GetNextSequence(cell.ReplayBackward)
		err := c.replayProtection.ValidateAndTrack(cell.ReplayBackward, seqNum, decryptedPayload)
		c.mu.Unlock()
		if err != nil {
			return fmt.Errorf("replay protection: %w", err)
		}
	}

	// Verify which hop recognizes this cell
	hopIdx, err := c.verifyRelayCellDigest(decryptedPayload)
	if err != nil {
		return fmt.Errorf("failed to verify relay cell digest: %w", err)
	}

	if hopIdx < 0 {
		// Cell not recognized by any hop
		// This might be a cell for a different stream or an error
		// Per tor-spec.txt §6.1, unrecognized cells should be dropped
		// Silently drop unrecognized cells
		return nil
	}

	// Decode the relay cell
	relayCell, err := cell.DecodeRelayCell(decryptedPayload)
	if err != nil {
		return fmt.Errorf("failed to decode relay cell: %w", err)
	}

	// Handle flow control per tor-spec.txt §7.4
	switch relayCell.Command {
	case cell.RelayData:
		// DATA cells count against our deliver window
		if err := c.decrementDeliverWindow(); err != nil {
			return fmt.Errorf("flow control: %w", err)
		}

		// Check if we should send a SENDME
		if c.shouldSendCircuitSendme() {
			// Send SENDME in background to avoid blocking
			go func() {
				if err := c.sendCircuitSendme(); err != nil {
					// Log error but don't fail the delivery
					// (in production, should have proper logging)
				}
			}()
		}

	case cell.RelaySendme:
		// SENDME cell increments our package window
		if relayCell.StreamID == 0 {
			// Circuit-level SENDME
			c.incrementPackageWindow()
			// Don't deliver SENDME cells to the application layer
			return nil
		}
		// Stream-level SENDME - deliver to stream manager
		// (handled below)
	}

	// Record activity
	c.RecordActivity()

	// Deliver to receive channel (non-blocking with timeout)
	select {
	case c.relayReceiveChan <- relayCell:
		return nil
	case <-time.After(100 * time.Millisecond):
		return fmt.Errorf("relay receive channel full or blocked")
	}
}

// OpenStream opens a new stream on this circuit
// This is a convenience method that integrates with the stream manager
func (c *Circuit) OpenStream(streamID uint16, target string, port uint16) error {
	// Send RELAY_BEGIN cell
	beginPayload := []byte(fmt.Sprintf("%s:%d\x00", target, port))
	beginCell := cell.NewRelayCell(streamID, cell.RelayBegin, beginPayload)

	if err := c.SendRelayCell(beginCell); err != nil {
		return fmt.Errorf("failed to send RELAY_BEGIN: %w", err)
	}

	// Wait for RELAY_CONNECTED response
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	connectedCell, err := c.ReceiveRelayCell(ctx)
	if err != nil {
		return fmt.Errorf("failed to receive RELAY_CONNECTED: %w", err)
	}

	if connectedCell.StreamID != streamID {
		// Not for this stream, put it back?
		// For now, error out
		return fmt.Errorf("received cell for wrong stream: expected %d, got %d", streamID, connectedCell.StreamID)
	}

	if connectedCell.Command == cell.RelayEnd {
		// Stream was rejected
		reason := "unknown"
		if len(connectedCell.Data) > 0 {
			reason = fmt.Sprintf("reason=%d", connectedCell.Data[0])
		}
		return fmt.Errorf("stream rejected by exit: %s", reason)
	}

	if connectedCell.Command != cell.RelayConnected {
		return fmt.Errorf("expected RELAY_CONNECTED, got %s", cell.RelayCmdString(connectedCell.Command))
	}

	return nil
}

// ReadFromStream reads data from a specific stream
// This is used by the SOCKS proxy to receive data from the exit node
func (c *Circuit) ReadFromStream(ctx context.Context, streamID uint16) ([]byte, error) {
	for {
		relayCell, err := c.ReceiveRelayCell(ctx)
		if err != nil {
			if err == io.EOF {
				return nil, io.EOF
			}
			return nil, fmt.Errorf("failed to receive relay cell: %w", err)
		}

		// Filter for our stream
		if relayCell.StreamID != streamID {
			// Cell for different stream, skip
			// TODO: Deliver to correct stream via stream manager
			continue
		}

		switch relayCell.Command {
		case cell.RelayData:
			return relayCell.Data, nil
		case cell.RelayEnd:
			return nil, io.EOF
		default:
			// Unexpected command for this stream
			continue
		}
	}
}

// WriteToStream writes data to a specific stream
// This is used by the SOCKS proxy to send data to the exit node
func (c *Circuit) WriteToStream(streamID uint16, data []byte) error {
	dataCell := cell.NewRelayCell(streamID, cell.RelayData, data)
	return c.SendRelayCell(dataCell)
}

// EndStream sends a RELAY_END cell for a stream
func (c *Circuit) EndStream(streamID uint16, reason byte) error {
	endCell := cell.NewRelayCell(streamID, cell.RelayEnd, []byte{reason})
	return c.SendRelayCell(endCell)
}

// SECURITY-001: Replay protection methods

// GetReplayStats returns replay protection statistics for this circuit.
// This is useful for monitoring and debugging replay detection.
func (c *Circuit) GetReplayStats() cell.Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.replayProtection == nil {
		return cell.Stats{}
	}
	return c.replayProtection.Stats()
}

// GetReplayAttempts returns the total number of detected replay attempts.
func (c *Circuit) GetReplayAttempts() uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.replayProtection == nil {
		return 0
	}
	return c.replayProtection.TotalReplayAttempts()
}

// ValidateCellForReplay validates a cell against replay attacks.
// This is called during cell processing to detect replayed cells.
// direction: cell.ReplayForward for outgoing, cell.ReplayBackward for incoming
func (c *Circuit) ValidateCellForReplay(direction cell.ReplayDirection, cellData []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.replayProtection == nil {
		return nil // Replay protection not initialized (shouldn't happen)
	}

	// Get the next sequence number for this direction
	seqNum := c.replayProtection.GetNextSequence(direction)

	// Validate and track the cell
	return c.replayProtection.ValidateAndTrack(direction, seqNum, cellData)
}

// ResetReplayProtection resets the replay protection state.
// This should be called when the circuit is torn down or when
// a new circuit is established on the same Circuit object.
func (c *Circuit) ResetReplayProtection() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.replayProtection != nil {
		c.replayProtection.Reset()
	}
}
