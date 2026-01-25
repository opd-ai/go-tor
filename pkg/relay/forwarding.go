// Package relay - Cell forwarding for relay servers
// This file implements relay cell forwarding per tor-spec.txt §5.5-5.6
package relay

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/opd-ai/go-tor/pkg/cell"
	"github.com/opd-ai/go-tor/pkg/logger"
)

// ExtendedCircuit tracks a circuit that has been extended to next hop
type ExtendedCircuit struct {
	ClientCircuitID  uint32 // Circuit ID on client side
	NextHopCircuitID uint32 // Circuit ID on next hop side
	NextHopAddress   string // Address of next hop
	NextHopConn      net.Conn
	RelayEarlyCount  int // Count of RELAY_EARLY cells forwarded
	mu               sync.Mutex
}

// ForwardingHandler manages cell forwarding between circuits
type ForwardingHandler struct {
	circuits   *CircuitHandler
	extended   map[uint32]*ExtendedCircuit // Map from client circuit ID to extended circuit
	extendedMu sync.RWMutex
	logger     *logger.Logger
}

// NewForwardingHandler creates a new forwarding handler
func NewForwardingHandler(circuits *CircuitHandler, log *logger.Logger) *ForwardingHandler {
	if log == nil {
		log = logger.NewDefault()
	}
	return &ForwardingHandler{
		circuits: circuits,
		extended: make(map[uint32]*ExtendedCircuit),
		logger:   log.Component("forwarding"),
	}
}

// RegisterExtendedCircuit registers an extended circuit for forwarding
func (h *ForwardingHandler) RegisterExtendedCircuit(clientCircID, nextHopCircID uint32, nextHopAddr string, nextHopConn net.Conn) error {
	h.extendedMu.Lock()
	defer h.extendedMu.Unlock()

	if _, exists := h.extended[clientCircID]; exists {
		return fmt.Errorf("circuit %d already extended", clientCircID)
	}

	h.extended[clientCircID] = &ExtendedCircuit{
		ClientCircuitID:  clientCircID,
		NextHopCircuitID: nextHopCircID,
		NextHopAddress:   nextHopAddr,
		NextHopConn:      nextHopConn,
		RelayEarlyCount:  0,
	}

	h.logger.Info("Registered extended circuit",
		"client_circuit_id", clientCircID,
		"next_hop_circuit_id", nextHopCircID,
		"next_hop_address", nextHopAddr)

	return nil
}

// ForwardRelayCell forwards a relay cell from client to next hop
// Per tor-spec.txt §5.5:
// - RELAY_EARLY cells are limited to 8 per circuit direction
// - After 8 RELAY_EARLY cells, convert to RELAY cells
// - Track counts to prevent circuit extension attacks
func (h *ForwardingHandler) ForwardRelayCell(ctx context.Context, fromClient bool, circuitID uint32, c *cell.Cell) error {
	// Check if this is an extended circuit
	h.extendedMu.RLock()
	ext, isExtended := h.extended[circuitID]
	h.extendedMu.RUnlock()

	if !isExtended {
		// Circuit not extended, this is the end of the circuit
		// Handle locally (stream operations, etc.)
		return h.handleLocalRelayCell(ctx, circuitID, c)
	}

	// Forward to next hop
	if fromClient {
		return h.forwardToNextHop(ext, c)
	}
	return h.forwardToClient(ext, c)
}

// forwardToNextHop forwards a cell from client to next hop
func (h *ForwardingHandler) forwardToNextHop(ext *ExtendedCircuit, c *cell.Cell) error {
	ext.mu.Lock()
	defer ext.mu.Unlock()

	// Handle RELAY_EARLY cell counting (tor-spec.txt §5.5)
	if c.Command == cell.CmdRelayEarly {
		if ext.RelayEarlyCount >= 8 {
			// Convert to RELAY cell after 8 RELAY_EARLY cells
			h.logger.Debug("Converting RELAY_EARLY to RELAY",
				"circuit_id", ext.ClientCircuitID,
				"count", ext.RelayEarlyCount)
			c.Command = cell.CmdRelay
		} else {
			ext.RelayEarlyCount++
			h.logger.Debug("Forwarding RELAY_EARLY",
				"circuit_id", ext.ClientCircuitID,
				"count", ext.RelayEarlyCount)
		}
	}

	// Create forwarded cell with next hop circuit ID
	forwardedCell := &cell.Cell{
		CircID:  ext.NextHopCircuitID,
		Command: c.Command,
		Payload: c.Payload,
	}

	// Send to next hop
	if err := forwardedCell.Encode(ext.NextHopConn); err != nil {
		h.logger.Error("Failed to forward cell to next hop",
			"circuit_id", ext.ClientCircuitID,
			"error", err)
		return fmt.Errorf("forward to next hop failed: %w", err)
	}

	h.logger.Debug("Forwarded cell to next hop",
		"client_circuit_id", ext.ClientCircuitID,
		"next_hop_circuit_id", ext.NextHopCircuitID,
		"command", c.Command)

	return nil
}

// forwardToClient forwards a cell from next hop to client
func (h *ForwardingHandler) forwardToClient(ext *ExtendedCircuit, c *cell.Cell) error {
	// This would be called when receiving cells from next hop
	// For now, this is a placeholder as we need connection tracking
	h.logger.Debug("Forwarding cell to client",
		"circuit_id", ext.ClientCircuitID,
		"command", c.Command)
	return nil
}

// handleLocalRelayCell handles relay cells for circuits that end at this relay
func (h *ForwardingHandler) handleLocalRelayCell(ctx context.Context, circuitID uint32, c *cell.Cell) error {
	// Decode relay cell
	relayCell, err := cell.DecodeRelayCell(c.Payload)
	if err != nil {
		h.logger.Warn("Failed to decode relay cell",
			"circuit_id", circuitID,
			"error", err)
		return fmt.Errorf("invalid relay cell: %w", err)
	}

	h.logger.Debug("Handling local relay cell",
		"circuit_id", circuitID,
		"command", cell.RelayCmdString(relayCell.Command),
		"stream_id", relayCell.StreamID)

	// Check for exit attempts and enforce exit policy
	switch relayCell.Command {
	case cell.RelayBegin, cell.RelayBeginDir:
		// Exit policy: reject all exit traffic
		return h.rejectExitAttempt(circuitID, relayCell.StreamID)

	case cell.RelayExtend2:
		// This should be handled by extension handler
		h.logger.Debug("RELAY_EXTEND2 on local circuit - should be handled separately")
		return nil

	case cell.RelayData, cell.RelayEnd, cell.RelaySendme:
		// Stream operations not supported in non-exit relay
		h.logger.Debug("Stream operation on non-exit relay - ignoring",
			"command", cell.RelayCmdString(relayCell.Command))
		return nil

	case cell.RelayTruncate:
		// Handle circuit truncation
		return h.handleTruncate(circuitID)

	default:
		h.logger.Debug("Unhandled relay command",
			"circuit_id", circuitID,
			"command", cell.RelayCmdString(relayCell.Command))
		return nil
	}
}

// rejectExitAttempt sends RELAY_END with EXITPOLICY reason
func (h *ForwardingHandler) rejectExitAttempt(circuitID uint32, streamID uint16) error {
	h.logger.Info("Rejecting exit attempt (exit policy)",
		"circuit_id", circuitID,
		"stream_id", streamID)

	// Create RELAY_END cell with EXITPOLICY reason
	endData := []byte{cell.EndReasonExitPolicy}
	relayCell, err := cell.NewRelayCell(streamID, cell.RelayEnd, endData)
	if err != nil {
		return fmt.Errorf("failed to create RELAY_END: %w", err)
	}

	payload, err := relayCell.Encode()
	if err != nil {
		return fmt.Errorf("failed to encode RELAY_END: %w", err)
	}

	// This would need to be sent back on the circuit
	// For now, log the action
	h.logger.Debug("Created RELAY_END cell",
		"circuit_id", circuitID,
		"stream_id", streamID,
		"reason", "EXITPOLICY",
		"payload_len", len(payload))

	return nil
}

// handleTruncate handles RELAY_TRUNCATE cells per tor-spec.txt §5.5
// Returns true if the circuit had an extension that was torn down
func (h *ForwardingHandler) handleTruncate(circuitID uint32) error {
	h.logger.Info("Received RELAY_TRUNCATE", "circuit_id", circuitID)

	// Remove extended circuit if it exists
	h.extendedMu.Lock()
	defer h.extendedMu.Unlock()
	
	if ext, exists := h.extended[circuitID]; exists {
		// Close connection to next hop
		if ext.NextHopConn != nil {
			ext.NextHopConn.Close()
		}
		delete(h.extended, circuitID)
		h.logger.Info("Truncated extended circuit",
			"circuit_id", circuitID,
			"next_hop_circuit_id", ext.NextHopCircuitID)
		
		// Note: RELAY_TRUNCATED response should be sent by the OR handler
		// that has access to the client connection. The truncation itself
		// is complete - we've torn down the extension to the next hop.
	}

	return nil
}

// HandleDestroy handles DESTROY cells and cleans up extended circuits
func (h *ForwardingHandler) HandleDestroy(circuitID uint32) error {
	h.logger.Info("Handling DESTROY", "circuit_id", circuitID)

	// Clean up extended circuit
	h.extendedMu.Lock()
	if ext, exists := h.extended[circuitID]; exists {
		// Send DESTROY to next hop
		if ext.NextHopConn != nil {
			destroyCell := &cell.Cell{
				CircID:  ext.NextHopCircuitID,
				Command: cell.CmdDestroy,
				Payload: []byte{cell.DestroyReasonDestroyed},
			}
			destroyCell.Encode(ext.NextHopConn)
			ext.NextHopConn.Close()
		}
		delete(h.extended, circuitID)
		h.logger.Info("Destroyed extended circuit",
			"circuit_id", circuitID,
			"next_hop_circuit_id", ext.NextHopCircuitID)
	}
	h.extendedMu.Unlock()

	return nil
}

// GetExtendedCircuitCount returns the number of extended circuits
func (h *ForwardingHandler) GetExtendedCircuitCount() int {
	h.extendedMu.RLock()
	defer h.extendedMu.RUnlock()
	return len(h.extended)
}

// CloseAll closes all extended circuits
func (h *ForwardingHandler) CloseAll() {
	h.extendedMu.Lock()
	defer h.extendedMu.Unlock()

	for circID, ext := range h.extended {
		if ext.NextHopConn != nil {
			ext.NextHopConn.Close()
		}
		h.logger.Debug("Closed extended circuit", "circuit_id", circID)
	}
	h.extended = make(map[uint32]*ExtendedCircuit)
	h.logger.Info("Closed all extended circuits")
}
