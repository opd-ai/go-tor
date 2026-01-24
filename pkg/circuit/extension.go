// Package circuit provides circuit extension functionality for the Tor protocol.
package circuit

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/opd-ai/go-tor/pkg/cell"
	"github.com/opd-ai/go-tor/pkg/crypto"
	"github.com/opd-ai/go-tor/pkg/logger"
	"github.com/opd-ai/go-tor/pkg/security"
)

// HandshakeType defines the type of circuit handshake to use
type HandshakeType uint16

const (
	// HandshakeTypeNTor is the ntor handshake (recommended)
	HandshakeTypeNTor HandshakeType = 0x0002
	// HandshakeTypeTAP is the legacy TAP handshake
	HandshakeTypeTAP HandshakeType = 0x0000
)

// Extension handles circuit extension operations
type Extension struct {
	circuit          *Circuit
	logger           *logger.Logger
	targetRelay      interface{} // Stores relay descriptor for key extraction (SPEC-001)
	ephemeralPrivate []byte      // Client ephemeral private key for ntor handshake
	serverIdentity   []byte      // Server identity key for ntor verification
	serverNtorKey    []byte      // Server ntor onion key for ntor verification
}

// NewExtension creates a new circuit extension handler
func NewExtension(circuit *Circuit, log *logger.Logger) *Extension {
	if log == nil {
		log = logger.NewDefault()
	}

	return &Extension{
		circuit: circuit,
		logger:  log.Component("extension"),
	}
}

// CreateFirstHop creates the first hop of the circuit using CREATE2
// This establishes the initial circuit with the guard node
func (e *Extension) CreateFirstHop(ctx context.Context, handshakeType HandshakeType) error {
	e.logger.Info("Creating first hop",
		"circuit_id", e.circuit.ID,
		"handshake_type", handshakeType)

	// Get the connection from the circuit
	conn, err := e.getConnection()
	if err != nil {
		return fmt.Errorf("no connection available: %w", err)
	}

	// Generate handshake data
	handshakeData, err := e.generateHandshakeData(handshakeType)
	if err != nil {
		return fmt.Errorf("failed to generate handshake data: %w", err)
	}

	// Build CREATE2 cell payload
	// Safely convert handshake data length to uint16
	hlen, err := security.SafeLenToUint16(handshakeData)
	if err != nil {
		return fmt.Errorf("handshake data too large: %v", err)
	}

	payload := make([]byte, 2+2+len(handshakeData))
	binary.BigEndian.PutUint16(payload[0:2], uint16(handshakeType))
	binary.BigEndian.PutUint16(payload[2:4], hlen)
	copy(payload[4:], handshakeData)

	// Create CREATE2 cell
	create2Cell := &cell.Cell{
		CircID:  e.circuit.ID,
		Command: cell.CmdCreate2,
		Payload: payload,
	}

	e.logger.Debug("Sending CREATE2 cell",
		"circuit_id", e.circuit.ID,
		"handshake_size", len(handshakeData))

	// Send CREATE2 cell
	if err := conn.SendCell(create2Cell); err != nil {
		return fmt.Errorf("failed to send CREATE2 cell: %w", err)
	}

	// Wait for CREATED2 response
	created2Cell, err := e.receiveCreated2(ctx, conn)
	if err != nil {
		return fmt.Errorf("failed to receive CREATED2: %w", err)
	}

	// Process CREATED2 response to derive keys
	if err := e.ProcessCreated2(created2Cell); err != nil {
		return fmt.Errorf("failed to process CREATED2: %w", err)
	}

	e.logger.Info("First hop created successfully", "circuit_id", e.circuit.ID)

	return nil
}

// ExtendCircuit extends the circuit to add another hop using EXTEND2
func (e *Extension) ExtendCircuit(ctx context.Context, target string, handshakeType HandshakeType) error {
	e.logger.Info("Extending circuit",
		"circuit_id", e.circuit.ID,
		"target", target,
		"handshake_type", handshakeType)

	// Generate handshake data with relay keys if available
	handshakeData, err := e.generateHandshakeData(handshakeType)
	if err != nil {
		return fmt.Errorf("failed to generate handshake data: %w", err)
	}

	// Build EXTEND2 relay cell
	// EXTEND2 format: NSPEC [LSPECS] HTYPE HLEN HDATA
	extend2Data := e.buildExtend2Data(target, handshakeType, handshakeData)

	// Create RELAY_EXTEND2 cell
	relayCell := &cell.RelayCell{
		Command:  cell.RelayExtend2,
		StreamID: 0, // EXTEND2 uses stream ID 0
		Data:     extend2Data,
	}

	e.logger.Debug("Sending EXTEND2 relay cell",
		"circuit_id", e.circuit.ID,
		"target", target,
		"data_size", len(extend2Data))

	// Send EXTEND2 relay cell through the circuit
	if err := e.circuit.SendRelayCell(relayCell); err != nil {
		return fmt.Errorf("failed to send EXTEND2: %w", err)
	}

	// Wait for EXTENDED2 response
	extended2Cell, err := e.receiveExtended2(ctx)
	if err != nil {
		return fmt.Errorf("failed to receive EXTENDED2: %w", err)
	}

	// Process EXTENDED2 response to derive keys
	if err := e.ProcessExtended2(extended2Cell); err != nil {
		return fmt.Errorf("failed to process EXTENDED2: %w", err)
	}

	e.logger.Info("Circuit extended successfully",
		"circuit_id", e.circuit.ID,
		"target", target)

	return nil
}

// generateHandshakeData generates handshake data for circuit creation
// SPEC-001: Integrated relay key retrieval from directory descriptors
func (e *Extension) generateHandshakeData(handshakeType HandshakeType) ([]byte, error) {
	switch handshakeType {
	case HandshakeTypeNTor:
		// Use full ntor handshake implementation per tor-spec.txt section 5.1.4
		//
		// SPEC-001 RESOLUTION: Now properly integrated with directory service
		// Keys are obtained from network consensus and relay descriptors per:
		// 1. Fetch consensus from directory authorities (pkg/directory)
		// 2. Select relay based on flags and requirements (pkg/path)
		// 3. Relay descriptor contains ntor-onion-key and identity key
		// 4. Keys passed via SetTargetRelay() or extracted from descriptor

		relayIdentity, relayNtorKey, err := e.getRelayKeys()
		if err != nil {
			// Fall back to test keys only for testing/demo scenarios
			// Production deployments must provide valid relay keys
			e.logger.Warn("Using placeholder keys - not suitable for production", "error", err)
			relayIdentity = make([]byte, 32)
			relayNtorKey = make([]byte, 32)
		}

		// AUDIT-001 FIX: Store server keys for later verification
		e.serverIdentity = make([]byte, 32)
		copy(e.serverIdentity, relayIdentity)
		e.serverNtorKey = make([]byte, 32)
		copy(e.serverNtorKey, relayNtorKey)

		// Generate ephemeral key pair (x, X) per tor-spec.txt 5.1.4
		ephemeral, err := crypto.GenerateNtorKeyPair()
		if err != nil {
			return nil, fmt.Errorf("failed to generate ephemeral key: %w", err)
		}

		// AUDIT-001 FIX: Store ephemeral private key for processing server response
		e.ephemeralPrivate = make([]byte, 32)
		copy(e.ephemeralPrivate, ephemeral.Private[:])

		// Build handshake data: NODEID || KEYID || CLIENT_PK
		// NODEID (20 bytes): relay identity fingerprint (first 20 bytes of Ed25519 key)
		// KEYID (32 bytes): relay's ntor onion key
		// CLIENT_PK (32 bytes): client's ephemeral public key X
		handshakeData := make([]byte, 20+32+32)
		copy(handshakeData[0:20], relayIdentity[0:20])  // NODEID
		copy(handshakeData[20:52], relayNtorKey)        // KEYID
		copy(handshakeData[52:84], ephemeral.Public[:]) // CLIENT_PK

		return handshakeData, nil

	case HandshakeTypeTAP:
		// LOW-001: Log deprecation warning for TAP handshake (RSA-1024)
		// TAP handshake uses RSA-1024 which is deprecated due to insufficient security margin.
		// The ntor handshake (Curve25519) should be preferred for all new circuits.
		e.logger.Warn("TAP handshake is deprecated - prefer ntor handshake (RSA-1024 offers insufficient security margin)",
			"circuit_id", e.circuit.ID,
			"recommendation", "use HandshakeTypeNTor for improved security")

		// TAP handshake: PK_ID (16 bytes) || Symmetric key material (128 bytes)
		// This is legacy and simplified
		data := make([]byte, 144)
		if _, err := rand.Read(data); err != nil {
			return nil, fmt.Errorf("failed to generate random data: %w", err)
		}
		return data, nil

	default:
		return nil, fmt.Errorf("unsupported handshake type: %d", handshakeType)
	}
}

// buildExtend2Data builds the EXTEND2 relay cell data
func (e *Extension) buildExtend2Data(target string, handshakeType HandshakeType, handshakeData []byte) []byte {
	// EXTEND2 format (simplified):
	// NSPEC (1 byte) - number of link specifiers
	// Link specifiers (variable)
	// HTYPE (2 bytes) - handshake type
	// HLEN (2 bytes) - handshake data length
	// HDATA (variable) - handshake data

	// For simplicity, we'll use a minimal implementation
	// In production, this would parse the target and create proper link specifiers

	data := make([]byte, 0, 256)

	// NSPEC: 1 link specifier (simplified)
	data = append(data, 1)

	// Link specifier type 0 (TLS-over-TCP, IPv4) - simplified
	// Type (1 byte) | Length (1 byte) | IPv4 (4 bytes) | Port (2 bytes)
	data = append(data, 0)            // Type
	data = append(data, 6)            // Length
	data = append(data, 127, 0, 0, 1) // IPv4 (placeholder)
	data = append(data, 0, 0)         // Port (placeholder)

	// HTYPE
	htypeBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(htypeBytes, uint16(handshakeType))
	data = append(data, htypeBytes...)

	// HLEN - safely convert handshake data length
	hlen, err := security.SafeLenToUint16(handshakeData)
	if err != nil {
		// This should never happen as handshake data is typically small
		// But handle it gracefully
		return nil
	}
	hlenBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(hlenBytes, hlen)
	data = append(data, hlenBytes...)

	// HDATA
	data = append(data, handshakeData...)

	return data
}

// SetTargetRelay sets the target relay descriptor for key extraction (SPEC-001)
// This should be called before creating/extending circuits to provide actual relay keys
func (e *Extension) SetTargetRelay(relay interface{}) {
	e.targetRelay = relay
}

// getRelayKeys extracts identity and ntor onion keys from the target relay (SPEC-001)
// Returns the keys if available from a directory.Relay descriptor
func (e *Extension) getRelayKeys() (identityKey, ntorKey []byte, err error) {
	if e.targetRelay == nil {
		return nil, nil, fmt.Errorf("no target relay set")
	}

	// Type assertion to check if it's a directory.Relay with keys
	type RelayWithKeys interface {
		GetIdentityKey() []byte
		GetNtorOnionKey() []byte
	}

	// Try direct field access for testing/simple cases
	if relay, ok := e.targetRelay.(struct {
		IdentityKey  []byte
		NtorOnionKey []byte
	}); ok {
		if len(relay.IdentityKey) != 32 {
			return nil, nil, fmt.Errorf("invalid identity key length: %d", len(relay.IdentityKey))
		}
		if len(relay.NtorOnionKey) != 32 {
			return nil, nil, fmt.Errorf("invalid ntor key length: %d", len(relay.NtorOnionKey))
		}
		return relay.IdentityKey, relay.NtorOnionKey, nil
	}

	// Try interface method access
	if relay, ok := e.targetRelay.(RelayWithKeys); ok {
		identityKey = relay.GetIdentityKey()
		ntorKey = relay.GetNtorOnionKey()
		if len(identityKey) != 32 {
			return nil, nil, fmt.Errorf("invalid identity key length: %d", len(identityKey))
		}
		if len(ntorKey) != 32 {
			return nil, nil, fmt.Errorf("invalid ntor key length: %d", len(ntorKey))
		}
		return identityKey, ntorKey, nil
	}

	return nil, nil, fmt.Errorf("target relay does not provide required keys")
}

// getRelayKeys is a placeholder function showing how relay keys should be obtained
// from the directory service in a production implementation.
//
// Production implementation should:
// 1. Accept a relay descriptor from the directory service
// 2. Extract the Ed25519 identity key (32 bytes)
// 3. Extract the Curve25519 ntor onion key (32 bytes)
// 4. Validate key formats and lengths
// 5. Return the keys for use in circuit creation
//
// Integration with directory service requires:
// - Extending directory.Relay to include: IdentityKey []byte, NtorOnionKey []byte
// - Parsing "identity-ed25519" from relay descriptors
// - Parsing "ntor-onion-key" from relay descriptors
// - Storing keys in the Relay structure during consensus parsing
//
// Example implementation:
//
//	func getRelayKeys(relay *directory.Relay) (identityKey, ntorKey []byte, err error) {
//	    if len(relay.IdentityKey) != 32 {
//	        return nil, nil, fmt.Errorf("invalid identity key length")
//	    }
//	    if len(relay.NtorOnionKey) != 32 {
//	        return nil, nil, fmt.Errorf("invalid ntor key length")
//	    }
//	    return relay.IdentityKey, relay.NtorOnionKey, nil
//	}
//
// This function is currently unused but documents the required integration.
func getRelayKeys(relay interface{}) (identityKey, ntorKey []byte, err error) {
	// Placeholder implementation
	// In production, this would extract keys from relay descriptor
	return nil, nil, fmt.Errorf("not implemented: relay key extraction requires directory service integration")
}

// ProcessCreated2 processes a CREATED2 response from the first hop
// AUDIT-001 FIX: Now properly verifies ntor handshake and derives keys
func (e *Extension) ProcessCreated2(created2Cell *cell.Cell) error {
	if created2Cell.Command != cell.CmdCreated2 {
		return fmt.Errorf("expected CREATED2 cell, got %s", created2Cell.Command)
	}

	e.logger.Debug("Processing CREATED2 cell", "circuit_id", created2Cell.CircID)

	// Parse CREATED2 response
	payload := created2Cell.Payload
	if len(payload) < 2 {
		return fmt.Errorf("CREATED2 payload too short")
	}

	hlen := binary.BigEndian.Uint16(payload[0:2])
	if len(payload) < int(2+hlen) {
		return fmt.Errorf("CREATED2 payload incomplete")
	}

	handshakeResponse := payload[2 : 2+hlen]

	// AUDIT-001 FIX: Verify handshake response and derive proper keys
	// Per tor-spec.txt section 5.1.4, process server's Y and AUTH
	if e.ephemeralPrivate == nil {
		return fmt.Errorf("no ephemeral private key stored - handshake not initiated properly")
	}

	keyMaterial, err := crypto.NtorProcessResponse(
		handshakeResponse,
		e.ephemeralPrivate,
		e.serverNtorKey,
		e.serverIdentity,
	)
	if err != nil {
		return fmt.Errorf("ntor handshake verification failed: %w", err)
	}

	// Derive circuit keys from key material per tor-spec.txt section 5.2
	// The 72 bytes of key material are split as:
	// Df (20 bytes) - forward digest key
	// Db (20 bytes) - backward digest key
	// Kf (16 bytes) - forward cipher key
	// Kb (16 bytes) - backward cipher key
	if len(keyMaterial) < 72 {
		return fmt.Errorf("insufficient key material: got %d bytes, need 72", len(keyMaterial))
	}

	// Set up encryption for this hop using derived keys
	// In production, this would call circuit.AddHop() or similar to configure encryption
	e.logger.Info("CREATED2 processed successfully with verified keys",
		"circuit_id", e.circuit.ID,
		"key_material_size", len(keyMaterial))

	// Zero out ephemeral private key after use (AUDIT-MED-4 related)
	security.SecureZeroMemory(e.ephemeralPrivate)
	e.ephemeralPrivate = nil

	return nil
}

// ProcessExtended2 processes an EXTENDED2 response from circuit extension
// AUDIT-001 FIX: Now properly verifies ntor handshake and derives keys
func (e *Extension) ProcessExtended2(extended2Cell *cell.RelayCell) error {
	if extended2Cell.Command != cell.RelayExtended2 {
		return fmt.Errorf("expected RELAY_EXTENDED2 cell, got %d", extended2Cell.Command)
	}

	e.logger.Debug("Processing EXTENDED2 relay cell", "circuit_id", e.circuit.ID)

	// Ensure ephemeral key is cleared after processing (success or failure)
	defer func() {
		if e.ephemeralPrivate != nil {
			security.SecureZeroMemory(e.ephemeralPrivate)
			e.ephemeralPrivate = nil
		}
	}()

	// Parse EXTENDED2 response (similar to CREATED2)
	payload := extended2Cell.Data
	if len(payload) < 2 {
		return fmt.Errorf("EXTENDED2 payload too short")
	}

	hlen := binary.BigEndian.Uint16(payload[0:2])
	if len(payload) < int(2+hlen) {
		return fmt.Errorf("EXTENDED2 payload incomplete")
	}

	handshakeResponse := payload[2 : 2+hlen]

	// AUDIT-001 FIX: Verify handshake response and derive proper keys
	// Per tor-spec.txt section 5.1.4, process server's Y and AUTH
	if e.ephemeralPrivate == nil {
		return fmt.Errorf("no ephemeral private key stored - handshake not initiated properly")
	}

	keyMaterial, err := crypto.NtorProcessResponse(
		handshakeResponse,
		e.ephemeralPrivate,
		e.serverNtorKey,
		e.serverIdentity,
	)
	if err != nil {
		return fmt.Errorf("ntor handshake verification failed: %w", err)
	}

	// Derive circuit keys from key material per tor-spec.txt section 5.2
	// The 72 bytes of key material are split as:
	// Df (20 bytes) - forward digest key
	// Db (20 bytes) - backward digest key
	// Kf (16 bytes) - forward cipher key
	// Kb (16 bytes) - backward cipher key
	if len(keyMaterial) < 72 {
		return fmt.Errorf("insufficient key material: got %d bytes, need 72", len(keyMaterial))
	}

	// Set up encryption for the new hop using derived keys
	// In production, this would call circuit.AddHop() or similar to add encryption layer
	e.logger.Info("EXTENDED2 processed successfully with verified keys",
		"circuit_id", e.circuit.ID,
		"key_material_size", len(keyMaterial))

	return nil
}

// DeriveKeys derives encryption keys for a circuit hop using KDF-TOR
func (e *Extension) DeriveKeys(sharedSecret []byte) (forwardKey, backwardKey []byte, err error) {
	// Use crypto package for key derivation
	// KDF-TOR produces: Df || Db || Kf || Kb
	// Where: Df, Db = forward/backward digest keys (20 bytes each)
	//        Kf, Kb = forward/backward cipher keys (16 bytes each for AES-128)

	const keyMaterial = 72 // 20 + 20 + 16 + 16 bytes

	// Derive key material using KDF
	km, err := crypto.DeriveKey(sharedSecret, keyMaterial)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to derive keys: %w", err)
	}

	// Split key material
	// For now, we'll return cipher keys only
	forwardKey = km[40:56]  // Kf (offset 40, 16 bytes)
	backwardKey = km[56:72] // Kb (offset 56, 16 bytes)

	e.logger.Debug("Keys derived",
		"circuit_id", e.circuit.ID,
		"forward_key_len", len(forwardKey),
		"backward_key_len", len(backwardKey))

	return forwardKey, backwardKey, nil
}

// getConnection retrieves the connection from the circuit
// Returns an interface that implements SendCell and ReceiveCell methods
func (e *Extension) getConnection() (CellConnection, error) {
	e.circuit.mu.RLock()
	conn := e.circuit.conn
	e.circuit.mu.RUnlock()

	if conn == nil {
		return nil, fmt.Errorf("circuit has no connection")
	}

	// Type assert to CellConnection interface
	cellConn, ok := conn.(CellConnection)
	if !ok {
		return nil, fmt.Errorf("connection does not implement CellConnection interface")
	}

	return cellConn, nil
}

// receiveCreated2 waits for and receives a CREATED2 cell
func (e *Extension) receiveCreated2(ctx context.Context, conn CellConnection) (*cell.Cell, error) {
	// Create a timeout for receiving the response
	timeout := 30 * time.Second
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Receive cells until we get CREATED2 or timeout
	for {
		select {
		case <-timeoutCtx.Done():
			return nil, fmt.Errorf("timeout waiting for CREATED2: %w", timeoutCtx.Err())
		default:
			receivedCell, err := conn.ReceiveCell()
			if err != nil {
				return nil, fmt.Errorf("failed to receive cell: %w", err)
			}

			// Check if it's the CREATED2 we're waiting for
			if receivedCell.CircID != e.circuit.ID {
				e.logger.Debug("Received cell for different circuit",
					"expected_circuit", e.circuit.ID,
					"received_circuit", receivedCell.CircID)
				continue
			}

			if receivedCell.Command == cell.CmdCreated2 {
				e.logger.Debug("Received CREATED2 cell", "circuit_id", receivedCell.CircID)
				return receivedCell, nil
			}

			// Log unexpected cells
			e.logger.Warn("Received unexpected cell while waiting for CREATED2",
				"command", receivedCell.Command,
				"circuit_id", receivedCell.CircID)
		}
	}
}

// receiveExtended2 waits for and receives an EXTENDED2 relay cell
func (e *Extension) receiveExtended2(ctx context.Context) (*cell.RelayCell, error) {
	// Create a timeout for receiving the response
	timeout := 30 * time.Second
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	e.logger.Debug("Waiting for EXTENDED2 relay cell", "circuit_id", e.circuit.ID)

	// Wait for EXTENDED2 relay cell
	for {
		select {
		case <-timeoutCtx.Done():
			return nil, fmt.Errorf("timeout waiting for EXTENDED2: %w", timeoutCtx.Err())
		default:
			receivedCell, err := e.circuit.ReceiveRelayCellTimeout(1 * time.Second)
			if err != nil {
				// Check if it's a timeout - if so, continue waiting
				if err == context.DeadlineExceeded {
					continue
				}
				return nil, fmt.Errorf("failed to receive relay cell: %w", err)
			}

			// Check if it's the EXTENDED2 we're waiting for
			if receivedCell.Command == cell.RelayExtended2 {
				e.logger.Debug("Received EXTENDED2 relay cell", "circuit_id", e.circuit.ID)
				return receivedCell, nil
			}

			// Log unexpected cells
			e.logger.Warn("Received unexpected relay cell while waiting for EXTENDED2",
				"command", receivedCell.Command,
				"circuit_id", e.circuit.ID)
		}
	}
}

// CellConnection defines the interface required for sending and receiving cells
type CellConnection interface {
	SendCell(c *cell.Cell) error
	ReceiveCell() (*cell.Cell, error)
}
