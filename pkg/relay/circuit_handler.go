// Package relay - Server-side circuit handling
// This file implements CREATE2/CREATED2 handling for relay servers
// Following tor-spec.txt §4-5 (server-side)
package relay

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/opd-ai/go-tor/pkg/cell"
	"github.com/opd-ai/go-tor/pkg/crypto"
	"github.com/opd-ai/go-tor/pkg/logger"
)

// ServerCircuit represents a server-side circuit
type ServerCircuit struct {
	CircuitID      uint32
	Created        time.Time
	LastActivity   time.Time
	KeyMaterial    []byte // 72 bytes from ntor handshake
	ForwardDigest  []byte // Forward digest state
	BackwardDigest []byte // Backward digest state
	mu             sync.RWMutex
}

// CircuitHandler manages server-side circuits for a relay
type CircuitHandler struct {
	keys      *RelayKeys
	circuits  map[uint32]*ServerCircuit
	mu        sync.RWMutex
	logger    *logger.Logger
	ctx       context.Context
	forwarder *ForwardingHandler
}

// NewCircuitHandler creates a new circuit handler
func NewCircuitHandler(keys *RelayKeys, log *logger.Logger) *CircuitHandler {
	if log == nil {
		log = logger.NewDefault()
	}
	h := &CircuitHandler{
		keys:     keys,
		circuits: make(map[uint32]*ServerCircuit),
		logger:   log.Component("circuit-handler"),
		ctx:      context.Background(),
	}
	// Create forwarding handler
	h.forwarder = NewForwardingHandler(h, log)
	return h
}

// HandleCellFromConnection processes cells from a client connection
// This handles CREATE2 cells for circuit creation and RELAY cells for forwarding
func (h *CircuitHandler) HandleCellFromConnection(conn net.Conn, c *cell.Cell) error {
	switch c.Command {
	case cell.CmdCreate2:
		return h.handleCreate2(conn, c)
	case cell.CmdRelay, cell.CmdRelayEarly:
		return h.handleRelay(conn, c)
	case cell.CmdDestroy:
		return h.handleDestroy(c)
	default:
		h.logger.Debug("Ignoring cell command", "command", c.Command)
		return nil
	}
}

// handleCreate2 processes a CREATE2 cell and sends CREATED2 response
// Per tor-spec.txt §5.1:
//
//	CREATE2 cell contains:
//	  HTYPE (2 bytes) - handshake type (0x0002 for ntor)
//	  HLEN  (2 bytes) - handshake data length
//	  HDATA (HLEN bytes) - handshake data
//
//	CREATED2 response contains:
//	  HLEN (2 bytes) - handshake response length
//	  HDATA (HLEN bytes) - handshake response
func (h *CircuitHandler) handleCreate2(conn net.Conn, c *cell.Cell) error {
	h.logger.Info("Received CREATE2",
		"circuit_id", c.CircID,
		"data_len", len(c.Payload))

	// Check if circuit already exists
	h.mu.RLock()
	_, exists := h.circuits[c.CircID]
	h.mu.RUnlock()

	if exists {
		h.logger.Warn("CREATE2 for existing circuit", "circuit_id", c.CircID)
		return h.sendDestroyCell(conn, c.CircID, cell.DestroyReasonProtocol)
	}

	// Parse CREATE2 payload
	if len(c.Payload) < 4 {
		h.logger.Warn("CREATE2 payload too short", "len", len(c.Payload))
		return h.sendDestroyCell(conn, c.CircID, cell.DestroyReasonProtocol)
	}

	htype := uint16(c.Payload[0])<<8 | uint16(c.Payload[1])
	hlen := uint16(c.Payload[2])<<8 | uint16(c.Payload[3])

	h.logger.Debug("CREATE2 handshake", "type", htype, "len", hlen)

	// Only support ntor (type 0x0002)
	if htype != 0x0002 {
		h.logger.Warn("Unsupported handshake type", "type", htype)
		return h.sendDestroyCell(conn, c.CircID, cell.DestroyReasonProtocol)
	}

	if len(c.Payload) < 4+int(hlen) {
		h.logger.Warn("CREATE2 payload incomplete", "expected", 4+hlen, "got", len(c.Payload))
		return h.sendDestroyCell(conn, c.CircID, cell.DestroyReasonProtocol)
	}

	handshakeData := c.Payload[4 : 4+hlen]

	// Validate handshake data length for ntor (84 bytes)
	if len(handshakeData) != 84 {
		h.logger.Warn("Invalid ntor handshake length", "len", len(handshakeData))
		return h.sendDestroyCell(conn, c.CircID, cell.DestroyReasonProtocol)
	}

	// Perform server-side ntor handshake
	response, keyMaterial, err := crypto.NtorServerHandshake(
		handshakeData,
		h.keys.NtorOnionKey,
		h.keys.Identity.Public,
	)
	if err != nil {
		h.logger.Error("Ntor handshake failed", "error", err)
		return h.sendDestroyCell(conn, c.CircID, cell.DestroyReasonInternal)
	}

	// Create circuit state
	circuit := &ServerCircuit{
		CircuitID:      c.CircID,
		Created:        time.Now(),
		LastActivity:   time.Now(),
		KeyMaterial:    keyMaterial,
		ForwardDigest:  make([]byte, sha256.Size),
		BackwardDigest: make([]byte, sha256.Size),
	}

	// Store circuit
	h.mu.Lock()
	h.circuits[c.CircID] = circuit
	h.mu.Unlock()

	h.logger.Info("Circuit created",
		"circuit_id", c.CircID,
		"key_material_len", len(keyMaterial))

	// Send CREATED2 response
	return h.sendCreated2(conn, c.CircID, response)
}

// sendCreated2 sends a CREATED2 cell with handshake response
func (h *CircuitHandler) sendCreated2(conn net.Conn, circuitID uint32, response []byte) error {
	// Build CREATED2 payload: HLEN (2) || HDATA (response)
	payload := make([]byte, 2+len(response))
	payload[0] = byte(len(response) >> 8)
	payload[1] = byte(len(response) & 0xff)
	copy(payload[2:], response)

	// Create CREATED2 cell
	created2 := &cell.Cell{
		CircID:  circuitID,
		Command: cell.CmdCreated2,
		Payload: payload,
	}

	// Encode and send
	if err := created2.Encode(conn); err != nil {
		return fmt.Errorf("failed to encode CREATED2: %w", err)
	}

	h.logger.Debug("Sent CREATED2",
		"circuit_id", circuitID,
		"response_len", len(response))

	return nil
}

// handleRelay processes RELAY and RELAY_EARLY cells
// Per tor-spec.txt §5.5-5.6, relay cells are forwarded to the next hop
// or handled locally if this is the end of the circuit
func (h *CircuitHandler) handleRelay(conn net.Conn, c *cell.Cell) error {
	h.mu.RLock()
	circuit, exists := h.circuits[c.CircID]
	h.mu.RUnlock()

	if !exists {
		h.logger.Warn("RELAY cell for unknown circuit", "circuit_id", c.CircID)
		return h.sendDestroyCell(conn, c.CircID, cell.DestroyReasonProtocol)
	}

	// Update activity timestamp
	circuit.mu.Lock()
	circuit.LastActivity = time.Now()
	circuit.mu.Unlock()

	h.logger.Debug("Received RELAY cell", "circuit_id", c.CircID, "command", c.Command)

	// Forward relay cell using ForwardingHandler
	// fromClient=true indicates this cell is from the client
	if err := h.forwarder.ForwardRelayCell(h.ctx, true, c.CircID, c); err != nil {
		h.logger.Error("Failed to forward RELAY cell",
			"circuit_id", c.CircID,
			"error", err)
		return err
	}

	return nil
}

// handleDestroy processes DESTROY cells
func (h *CircuitHandler) handleDestroy(c *cell.Cell) error {
	h.logger.Info("Received DESTROY", "circuit_id", c.CircID)

	// Clean up forwarding state if this is an extended circuit
	if h.forwarder != nil {
		h.forwarder.HandleDestroy(c.CircID)
	}

	// Remove circuit state
	h.mu.Lock()
	delete(h.circuits, c.CircID)
	h.mu.Unlock()

	return nil
}

// sendDestroyCell sends a DESTROY cell with specified reason
func (h *CircuitHandler) sendDestroyCell(conn net.Conn, circuitID uint32, reason byte) error {
	destroy := &cell.Cell{
		CircID:  circuitID,
		Command: cell.CmdDestroy,
		Payload: []byte{reason},
	}

	if err := destroy.Encode(conn); err != nil {
		return fmt.Errorf("failed to encode DESTROY: %w", err)
	}

	h.logger.Debug("Sent DESTROY", "circuit_id", circuitID, "reason", reason)
	return nil
}

// GetCircuit retrieves a circuit by ID
func (h *CircuitHandler) GetCircuit(circuitID uint32) (*ServerCircuit, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	circuit, exists := h.circuits[circuitID]
	return circuit, exists
}

// GetCircuitCount returns the number of active circuits
func (h *CircuitHandler) GetCircuitCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.circuits)
}

// CloseCircuit destroys a circuit
func (h *CircuitHandler) CloseCircuit(circuitID uint32) {
	h.mu.Lock()
	delete(h.circuits, circuitID)
	h.mu.Unlock()

	h.logger.Info("Circuit closed", "circuit_id", circuitID)
}

// CloseAll destroys all circuits
func (h *CircuitHandler) CloseAll() {
	// Close forwarding handler first
	if h.forwarder != nil {
		h.forwarder.CloseAll()
	}

	h.mu.Lock()
	count := len(h.circuits)
	h.circuits = make(map[uint32]*ServerCircuit)
	h.mu.Unlock()

	h.logger.Info("All circuits closed", "count", count)
}

// GetForwardingHandler returns the forwarding handler for extension operations
func (h *CircuitHandler) GetForwardingHandler() *ForwardingHandler {
	return h.forwarder
}
