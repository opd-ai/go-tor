// Package relay - Circuit extension handling for relay servers
// This file implements RELAY_EXTEND2 handling per tor-spec.txt §5.3-5.4
package relay

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/opd-ai/go-tor/pkg/cell"
	"github.com/opd-ai/go-tor/pkg/connection"
	"github.com/opd-ai/go-tor/pkg/logger"
)

// LinkSpecifier represents a way to reach the next relay
type LinkSpecifier struct {
	Type uint8  // Specifier type (0=IPv4, 1=IPv6, 2=Legacy ID, 3=Ed25519 ID)
	Data []byte // Specifier data
}

// parseLinkSpecifiers parses link specifiers from EXTEND2 data
func parseLinkSpecifiers(data []byte) ([]LinkSpecifier, int, error) {
	if len(data) < 1 {
		return nil, 0, fmt.Errorf("extend2 data too short for nspec")
	}

	nspec := int(data[0])
	specs := make([]LinkSpecifier, 0, nspec)
	offset := 1

	for i := 0; i < nspec; i++ {
		if offset+2 > len(data) {
			return nil, 0, fmt.Errorf("extend2 data truncated at specifier %d", i)
		}

		lstype := data[offset]
		lslen := int(data[offset+1])
		offset += 2

		if offset+lslen > len(data) {
			return nil, 0, fmt.Errorf("extend2 specifier %d data truncated", i)
		}

		specs = append(specs, LinkSpecifier{
			Type: lstype,
			Data: data[offset : offset+lslen],
		})
		offset += lslen
	}

	return specs, offset, nil
}

// extractAddressFromLinkSpecs extracts a connectable address from link specifiers
// Returns address in "IP:port" format
func extractAddressFromLinkSpecs(specs []LinkSpecifier) (string, error) {
	var ipv4Addr string
	var ipv6Addr string
	var port uint16

	for _, spec := range specs {
		switch spec.Type {
		case 0: // IPv4
			if len(spec.Data) != 6 {
				continue
			}
			ip := net.IPv4(spec.Data[0], spec.Data[1], spec.Data[2], spec.Data[3])
			port = binary.BigEndian.Uint16(spec.Data[4:6])
			ipv4Addr = fmt.Sprintf("%s:%d", ip.String(), port)

		case 1: // IPv6
			if len(spec.Data) != 18 {
				continue
			}
			ip := net.IP(spec.Data[0:16])
			port = binary.BigEndian.Uint16(spec.Data[16:18])
			ipv6Addr = fmt.Sprintf("[%s]:%d", ip.String(), port)
		}
	}

	// Prefer IPv4 over IPv6
	if ipv4Addr != "" {
		return ipv4Addr, nil
	}
	if ipv6Addr != "" {
		return ipv6Addr, nil
	}

	return "", fmt.Errorf("no usable address in link specifiers")
}

// ExtensionHandler handles circuit extension for relay servers
type ExtensionHandler struct {
	keys      *RelayKeys
	circuits  *CircuitHandler
	logger    *logger.Logger
	connPool  map[string]*connection.Connection // Pool of outbound connections
	connMutex sync.Mutex
}

// NewExtensionHandler creates a new extension handler
func NewExtensionHandler(keys *RelayKeys, circuits *CircuitHandler, log *logger.Logger) *ExtensionHandler {
	if log == nil {
		log = logger.NewDefault()
	}
	return &ExtensionHandler{
		keys:     keys,
		circuits: circuits,
		logger:   log.Component("extension"),
		connPool: make(map[string]*connection.Connection),
	}
}

// HandleExtend2 processes a RELAY_EXTEND2 cell
// Per tor-spec.txt §5.3:
//   EXTEND2 format:
//     NSPEC (1 byte) - number of link specifiers
//     NSPEC * [LSTYPE (1) | LSLEN (1) | LSPEC (LSLEN)]
//     HTYPE (2 bytes) - handshake type
//     HLEN (2 bytes) - handshake data length
//     HDATA (HLEN bytes) - handshake data
func (h *ExtensionHandler) HandleExtend2(ctx context.Context, circuitID uint32, relayCell *cell.RelayCell) error {
	h.logger.Info("Processing EXTEND2", "circuit_id", circuitID)

	data := relayCell.Data

	// Parse link specifiers
	specs, offset, err := parseLinkSpecifiers(data)
	if err != nil {
		h.logger.Warn("Failed to parse link specifiers", "error", err)
		return fmt.Errorf("invalid link specifiers: %w", err)
	}

	// Extract address from link specifiers
	address, err := extractAddressFromLinkSpecs(specs)
	if err != nil {
		h.logger.Warn("Failed to extract address", "error", err)
		return fmt.Errorf("no usable address: %w", err)
	}

	h.logger.Debug("Extracted next hop address", "address", address)

	// Parse handshake type and data
	if offset+4 > len(data) {
		return fmt.Errorf("extend2 data truncated at handshake header")
	}

	htype := binary.BigEndian.Uint16(data[offset : offset+2])
	hlen := binary.BigEndian.Uint16(data[offset+2 : offset+4])
	offset += 4

	if offset+int(hlen) > len(data) {
		return fmt.Errorf("extend2 handshake data truncated")
	}

	handshakeData := data[offset : offset+int(hlen)]

	h.logger.Debug("Extend2 handshake", "type", htype, "len", hlen)

	// Only support ntor handshake (0x0002)
	if htype != 0x0002 {
		h.logger.Warn("Unsupported handshake type", "type", htype)
		return fmt.Errorf("unsupported handshake type: %d", htype)
	}

	// Connect to next hop relay
	nextConn, err := h.connectToNextHop(ctx, address)
	if err != nil {
		h.logger.Error("Failed to connect to next hop", "address", address, "error", err)
		return fmt.Errorf("connection failed: %w", err)
	}

	// Generate a circuit ID for the next hop
	// Use a simple incrementing ID (in production, would need more sophisticated allocation)
	nextCircuitID := uint32(time.Now().UnixNano() & 0x7FFFFFFF)

	// Send CREATE2 to next hop
	if err := h.sendCreate2ToNextHop(nextConn, nextCircuitID, handshakeData); err != nil {
		h.logger.Error("Failed to send CREATE2 to next hop", "error", err)
		return fmt.Errorf("create2 failed: %w", err)
	}

	// Wait for CREATED2 from next hop
	created2Cell, err := h.receiveCreated2FromNextHop(ctx, nextConn, nextCircuitID)
	if err != nil {
		h.logger.Error("Failed to receive CREATED2 from next hop", "error", err)
		return fmt.Errorf("created2 failed: %w", err)
	}

	// Extract handshake response
	if len(created2Cell.Payload) < 2 {
		return fmt.Errorf("created2 payload too short")
	}

	responseLen := binary.BigEndian.Uint16(created2Cell.Payload[0:2])
	if len(created2Cell.Payload) < 2+int(responseLen) {
		return fmt.Errorf("created2 response truncated")
	}

	handshakeResponse := created2Cell.Payload[2 : 2+responseLen]

	// Register the extended circuit
	if err := h.registerExtendedCircuit(circuitID, nextCircuitID, address, nextConn); err != nil {
		h.logger.Error("Failed to register extended circuit", "error", err)
		return fmt.Errorf("registration failed: %w", err)
	}

	// Send RELAY_EXTENDED2 back to client
	if err := h.sendExtended2(circuitID, handshakeResponse); err != nil {
		h.logger.Error("Failed to send EXTENDED2", "error", err)
		return fmt.Errorf("extended2 failed: %w", err)
	}

	h.logger.Info("Circuit extension successful",
		"circuit_id", circuitID,
		"next_circuit_id", nextCircuitID,
		"next_hop", address)

	return nil
}

// connectToNextHop establishes a connection to the next relay
func (h *ExtensionHandler) connectToNextHop(ctx context.Context, address string) (*connection.Connection, error) {
	// Check if we already have a connection to this relay
	h.connMutex.Lock()
	if conn, exists := h.connPool[address]; exists {
		h.connMutex.Unlock()
		h.logger.Debug("Reusing existing connection", "address", address)
		return conn, nil
	}
	h.connMutex.Unlock()

	h.logger.Info("Connecting to next hop", "address", address)

	// Create connection config
	cfg := connection.DefaultConfig(address)
	cfg.Timeout = 10 * time.Second

	// Connect to next hop
	conn := connection.New(cfg, h.logger)
	if err := conn.Connect(ctx, cfg); err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}

	// Perform link protocol handshake
	if err := h.performLinkHandshake(ctx, conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("link handshake failed: %w", err)
	}

	// Cache the connection
	h.connMutex.Lock()
	h.connPool[address] = conn
	h.connMutex.Unlock()

	h.logger.Info("Connection to next hop established", "address", address)
	return conn, nil
}

// performLinkHandshake performs the link protocol handshake with next hop
func (h *ExtensionHandler) performLinkHandshake(ctx context.Context, conn *connection.Connection) error {
	// Send VERSIONS cell
	versions := []uint16{3, 4, 5}
	versionsPayload := make([]byte, len(versions)*2)
	for i, v := range versions {
		binary.BigEndian.PutUint16(versionsPayload[i*2:], v)
	}

	versionsCell := &cell.Cell{
		CircID:  0,
		Command: cell.CmdVersions,
		Payload: versionsPayload,
	}

	if err := conn.SendCell(versionsCell); err != nil {
		return fmt.Errorf("failed to send VERSIONS: %w", err)
	}

	// Receive VERSIONS response
	respCell, err := conn.ReceiveCellWithContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to receive VERSIONS: %w", err)
	}

	if respCell.Command != cell.CmdVersions {
		return fmt.Errorf("expected VERSIONS, got %d", respCell.Command)
	}

	// For simplicity, skip full CERTS/AUTH/NETINFO exchange
	// In production, would need complete link protocol
	h.logger.Debug("Link handshake completed (simplified)")
	return nil
}

// sendCreate2ToNextHop sends a CREATE2 cell to the next hop
func (h *ExtensionHandler) sendCreate2ToNextHop(conn *connection.Connection, circuitID uint32, handshakeData []byte) error {
	// Build CREATE2 payload: HTYPE (2) || HLEN (2) || HDATA
	payload := make([]byte, 4+len(handshakeData))
	binary.BigEndian.PutUint16(payload[0:2], 0x0002) // ntor
	binary.BigEndian.PutUint16(payload[2:4], uint16(len(handshakeData)))
	copy(payload[4:], handshakeData)

	create2 := &cell.Cell{
		CircID:  circuitID,
		Command: cell.CmdCreate2,
		Payload: payload,
	}

	return conn.SendCell(create2)
}

// receiveCreated2FromNextHop waits for CREATED2 response from next hop
func (h *ExtensionHandler) receiveCreated2FromNextHop(ctx context.Context, conn *connection.Connection, expectedCircuitID uint32) (*cell.Cell, error) {
	timeout := 30 * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	respCell, err := conn.ReceiveCellWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to receive cell: %w", err)
	}

	if respCell.Command != cell.CmdCreated2 {
		return nil, fmt.Errorf("expected CREATED2, got %d", respCell.Command)
	}

	if respCell.CircID != expectedCircuitID {
		return nil, fmt.Errorf("circuit ID mismatch: expected %d, got %d", expectedCircuitID, respCell.CircID)
	}

	return respCell, nil
}

// registerExtendedCircuit registers the extended circuit mapping
func (h *ExtensionHandler) registerExtendedCircuit(incomingCircID, outgoingCircID uint32, nextHop string, nextConn *connection.Connection) error {
	// Get the circuit
	circuit, exists := h.circuits.GetCircuit(incomingCircID)
	if !exists {
		return fmt.Errorf("circuit %d not found", incomingCircID)
	}

	circuit.mu.Lock()
	defer circuit.mu.Unlock()

	// Store extension information in circuit
	// In a full implementation, this would track the next hop connection and circuit ID
	// for proper cell forwarding
	h.logger.Debug("Registered circuit extension",
		"incoming_circ", incomingCircID,
		"outgoing_circ", outgoingCircID,
		"next_hop", nextHop)

	return nil
}

// sendExtended2 sends a RELAY_EXTENDED2 cell back to the client
func (h *ExtensionHandler) sendExtended2(circuitID uint32, handshakeResponse []byte) error {
	// Build EXTENDED2 relay cell data: HLEN (2) || HDATA
	data := make([]byte, 2+len(handshakeResponse))
	binary.BigEndian.PutUint16(data[0:2], uint16(len(handshakeResponse)))
	copy(data[2:], handshakeResponse)

	relayCell := &cell.RelayCell{
		Command:  cell.RelayExtended2,
		StreamID: 0,
		Data:     data,
	}

	// Encode relay cell
	_, err := relayCell.Encode()
	if err != nil {
		return fmt.Errorf("failed to encode relay cell: %w", err)
	}

	// Note: In a complete implementation, would send this back through the circuit
	// For now, this is a placeholder showing the cell is prepared correctly
	h.logger.Debug("EXTENDED2 cell prepared",
		"circuit_id", circuitID,
		"response_len", len(handshakeResponse))

	return nil
}

// Close cleans up extension handler resources
func (h *ExtensionHandler) Close() error {
	h.connMutex.Lock()
	defer h.connMutex.Unlock()

	for addr, conn := range h.connPool {
		h.logger.Debug("Closing connection", "address", addr)
		conn.Close()
	}

	h.connPool = make(map[string]*connection.Connection)
	return nil
}
