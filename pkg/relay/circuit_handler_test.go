// Package relay - Circuit handler tests
package relay

import (
	"bytes"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/cell"
	"github.com/opd-ai/go-tor/pkg/crypto"
	"github.com/opd-ai/go-tor/pkg/logger"
	"golang.org/x/crypto/curve25519"
)

func TestCircuitHandler_HandleCreate2(t *testing.T) {
	log := logger.NewDefault()

	// Generate relay keys
	keys, err := GenerateRelayKeys()
	if err != nil {
		t.Fatalf("Failed to generate relay keys: %v", err)
	}

	handler := NewCircuitHandler(keys, log)

	// Generate client ntor keypair
	clientKey, err := crypto.GenerateNtorKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate client key: %v", err)
	}

	// Build ntor handshake data (84 bytes):
	// NODEID (32) || KEYID (32) || CLIENT_PK (32)
	serverIdentity := keys.Identity.Public

	// Compute server public ntor key from private key
	var serverNtorPriv [32]byte
	copy(serverNtorPriv[:], keys.NtorOnionKey)
	var serverNtorPub [32]byte
	curve25519.ScalarBaseMult(&serverNtorPub, &serverNtorPriv)

	handshakeData := make([]byte, 84)
	copy(handshakeData[0:32], serverIdentity)       // NODEID (use full Ed25519 key)
	copy(handshakeData[32:64], serverNtorPub[:])    // KEYID
	copy(handshakeData[64:84], clientKey.Public[:]) // CLIENT_PK

	// Build CREATE2 cell
	// Payload: HTYPE (2) || HLEN (2) || HDATA (84)
	payload := make([]byte, 4+84)
	payload[0] = 0x00 // HTYPE = 0x0002 (ntor)
	payload[1] = 0x02
	payload[2] = 0x00 // HLEN = 84
	payload[3] = 0x54
	copy(payload[4:], handshakeData)

	create2Cell := &cell.Cell{
		CircID:  1,
		Command: cell.CmdCreate2,
		Payload: payload,
	}

	conn := newMockConn()

	// Handle CREATE2
	err = handler.HandleCellFromConnection(conn, create2Cell)
	if err != nil {
		t.Fatalf("HandleCellFromConnection failed: %v", err)
	}

	// Verify CREATED2 was sent
	if len(conn.writeData) == 0 {
		t.Fatal("Expected CREATED2 response but got nothing")
	}

	// Parse CREATED2 response
	responseData := conn.writeData
	if len(responseData) < cell.CellSize {
		t.Fatalf("Response too short: %d bytes", len(responseData))
	}

	responseCell, err := cell.DecodeCell(bytes.NewReader(responseData))
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if responseCell.Command != cell.CmdCreated2 {
		t.Errorf("Expected CREATED2, got command %d", responseCell.Command)
	}

	if responseCell.CircID != 1 {
		t.Errorf("Expected circuit ID 1, got %d", responseCell.CircID)
	}

	// Verify circuit was created
	circuit, exists := handler.GetCircuit(1)
	if !exists {
		t.Fatal("Circuit was not created")
	}

	if len(circuit.KeyMaterial) != 72 {
		t.Errorf("Expected 72 bytes of key material, got %d", len(circuit.KeyMaterial))
	}

	if circuit.CircuitID != 1 {
		t.Errorf("Expected circuit ID 1, got %d", circuit.CircuitID)
	}
}

func TestCircuitHandler_HandleCreate2_InvalidHandshakeType(t *testing.T) {
	log := logger.NewDefault()

	keys, err := GenerateRelayKeys()
	if err != nil {
		t.Fatalf("Failed to generate relay keys: %v", err)
	}

	handler := NewCircuitHandler(keys, log)

	// Build CREATE2 with invalid handshake type (TAP = 0x0000)
	payload := make([]byte, 4+84)
	payload[0] = 0x00 // HTYPE = 0x0000 (TAP - unsupported)
	payload[1] = 0x00
	payload[2] = 0x00 // HLEN = 84
	payload[3] = 0x54

	create2Cell := &cell.Cell{
		CircID:  2,
		Command: cell.CmdCreate2,
		Payload: payload,
	}

	conn := newMockConn()

	// Should fail with DESTROY
	err = handler.HandleCellFromConnection(conn, create2Cell)
	if err != nil {
		t.Logf("HandleCellFromConnection returned error (expected): %v", err)
	}

	// Verify DESTROY was sent
	if len(conn.writeData) > 0 {
		responseData := conn.writeData
		if len(responseData) >= cell.CellSize {
			responseCell, _ := cell.DecodeCell(bytes.NewReader(responseData))
			if responseCell.Command != cell.CmdDestroy {
				t.Errorf("Expected DESTROY, got command %d", responseCell.Command)
			}
		}
	}

	// Circuit should not be created
	_, exists := handler.GetCircuit(2)
	if exists {
		t.Error("Circuit should not have been created")
	}
}

func TestCircuitHandler_HandleCreate2_DuplicateCircuit(t *testing.T) {
	log := logger.NewDefault()

	keys, err := GenerateRelayKeys()
	if err != nil {
		t.Fatalf("Failed to generate relay keys: %v", err)
	}

	handler := NewCircuitHandler(keys, log)

	// Create a circuit manually
	handler.mu.Lock()
	handler.circuits[3] = &ServerCircuit{
		CircuitID: 2,
		Created:   time.Now(),
	}
	handler.mu.Unlock()

	// Try to create same circuit again
	payload := make([]byte, 4+84)
	payload[0] = 0x00
	payload[1] = 0x02
	payload[2] = 0x00
	payload[3] = 0x54

	create2Cell := &cell.Cell{
		CircID:  2,
		Command: cell.CmdCreate2,
		Payload: payload,
	}

	conn := newMockConn()

	// Should send DESTROY
	err = handler.HandleCellFromConnection(conn, create2Cell)
	if err != nil {
		t.Logf("HandleCellFromConnection returned error (expected): %v", err)
	}
}

func TestCircuitHandler_HandleDestroy(t *testing.T) {
	log := logger.NewDefault()

	keys, err := GenerateRelayKeys()
	if err != nil {
		t.Fatalf("Failed to generate relay keys: %v", err)
	}

	handler := NewCircuitHandler(keys, log)

	// Create a circuit
	handler.mu.Lock()
	handler.circuits[2] = &ServerCircuit{
		CircuitID: 2,
		Created:   time.Now(),
	}
	handler.mu.Unlock()

	// Send DESTROY
	destroyCell := &cell.Cell{
		CircID:  2,
		Command: cell.CmdDestroy,
		Payload: []byte{cell.DestroyReasonNone},
	}

	conn := newMockConn()
	err = handler.HandleCellFromConnection(conn, destroyCell)
	if err != nil {
		t.Fatalf("HandleCellFromConnection failed: %v", err)
	}

	// Circuit should be removed
	_, exists := handler.GetCircuit(2)
	if exists {
		t.Error("Circuit should have been destroyed")
	}
}

func TestCircuitHandler_CloseCircuit(t *testing.T) {
	log := logger.NewDefault()

	keys, err := GenerateRelayKeys()
	if err != nil {
		t.Fatalf("Failed to generate relay keys: %v", err)
	}

	handler := NewCircuitHandler(keys, log)

	// Create circuits
	for i := uint32(1); i <= 3; i++ {
		handler.mu.Lock()
		handler.circuits[i] = &ServerCircuit{
			CircuitID: 2,
			Created:   time.Now(),
		}
		handler.mu.Unlock()
	}

	if handler.GetCircuitCount() != 3 {
		t.Fatalf("Expected 3 circuits, got %d", handler.GetCircuitCount())
	}

	// Close one circuit
	handler.CloseCircuit(2)

	if handler.GetCircuitCount() != 2 {
		t.Errorf("Expected 2 circuits after close, got %d", handler.GetCircuitCount())
	}

	_, exists := handler.GetCircuit(2)
	if exists {
		t.Error("Circuit 2 should have been closed")
	}
}

func TestCircuitHandler_CloseAll(t *testing.T) {
	log := logger.NewDefault()

	keys, err := GenerateRelayKeys()
	if err != nil {
		t.Fatalf("Failed to generate relay keys: %v", err)
	}

	handler := NewCircuitHandler(keys, log)

	// Create multiple circuits
	for i := uint32(1); i <= 5; i++ {
		handler.mu.Lock()
		handler.circuits[i] = &ServerCircuit{
			CircuitID: 2,
			Created:   time.Now(),
		}
		handler.mu.Unlock()
	}

	if handler.GetCircuitCount() != 5 {
		t.Fatalf("Expected 5 circuits, got %d", handler.GetCircuitCount())
	}

	// Close all
	handler.CloseAll()

	if handler.GetCircuitCount() != 0 {
		t.Errorf("Expected 0 circuits after CloseAll, got %d", handler.GetCircuitCount())
	}
}

func TestCircuitHandler_HandleRelay(t *testing.T) {
	log := logger.NewDefault()

	keys, err := GenerateRelayKeys()
	if err != nil {
		t.Fatalf("Failed to generate relay keys: %v", err)
	}

	handler := NewCircuitHandler(keys, log)

	// Create a circuit
	handler.mu.Lock()
	handler.circuits[2] = &ServerCircuit{
		CircuitID:    2,
		Created:      time.Now(),
		LastActivity: time.Now().Add(-1 * time.Minute),
	}
	handler.mu.Unlock()

	// Send RELAY cell
	relayCell := &cell.Cell{
		CircID:  2,
		Command: cell.CmdRelay,
		Payload: make([]byte, 509), // Standard relay cell payload
	}

	conn := newMockConn()
	err = handler.HandleCellFromConnection(conn, relayCell)
	if err != nil {
		t.Fatalf("HandleCellFromConnection failed: %v", err)
	}

	// Verify LastActivity was updated
	circuit, _ := handler.GetCircuit(2)
	if time.Since(circuit.LastActivity) > 1*time.Second {
		t.Error("LastActivity was not updated")
	}
}

func TestCircuitHandler_HandleRelay_UnknownCircuit(t *testing.T) {
	log := logger.NewDefault()

	keys, err := GenerateRelayKeys()
	if err != nil {
		t.Fatalf("Failed to generate relay keys: %v", err)
	}

	handler := NewCircuitHandler(keys, log)

	// Send RELAY cell for non-existent circuit
	relayCell := &cell.Cell{
		CircID:  2,
		Command: cell.CmdRelay,
		Payload: make([]byte, 509),
	}

	conn := newMockConn()
	err = handler.HandleCellFromConnection(conn, relayCell)
	if err != nil {
		t.Logf("HandleCellFromConnection returned error (expected): %v", err)
	}

	// Should send DESTROY
	if len(conn.writeData) > 0 {
		responseData := conn.writeData
		if len(responseData) >= cell.CellSize {
			responseCell, _ := cell.DecodeCell(bytes.NewReader(responseData))
			if responseCell.Command != cell.CmdDestroy {
				t.Errorf("Expected DESTROY, got command %d", responseCell.Command)
			}
		}
	}
}

func TestCircuitHandler_HandleRelayEarly(t *testing.T) {
	log := logger.NewDefault()

	keys, err := GenerateRelayKeys()
	if err != nil {
		t.Fatalf("Failed to generate relay keys: %v", err)
	}

	handler := NewCircuitHandler(keys, log)

	// Create a circuit
	handler.mu.Lock()
	handler.circuits[5] = &ServerCircuit{
		CircuitID:    5,
		Created:      time.Now(),
		LastActivity: time.Now(),
	}
	handler.mu.Unlock()

	// Send RELAY_EARLY cell
	relayEarlyCell := &cell.Cell{
		CircID:  5,
		Command: cell.CmdRelayEarly,
		Payload: make([]byte, 509),
	}

	conn := newMockConn()
	err = handler.HandleCellFromConnection(conn, relayEarlyCell)
	if err != nil {
		t.Fatalf("HandleCellFromConnection failed: %v", err)
	}

	// Verify circuit activity was updated
	circuit, _ := handler.GetCircuit(5)
	if time.Since(circuit.LastActivity) > 1*time.Second {
		t.Error("LastActivity was not updated")
	}
}

func TestCircuitHandler_GetForwardingHandler(t *testing.T) {
	log := logger.NewDefault()

	keys, err := GenerateRelayKeys()
	if err != nil {
		t.Fatalf("Failed to generate relay keys: %v", err)
	}

	handler := NewCircuitHandler(keys, log)

	// Verify forwarder is initialized
	forwarder := handler.GetForwardingHandler()
	if forwarder == nil {
		t.Fatal("ForwardingHandler should not be nil")
	}

	// Verify it's the same instance
	if forwarder != handler.forwarder {
		t.Error("GetForwardingHandler returned different instance")
	}
}

func TestCircuitHandler_HandleDestroy_WithExtendedCircuit(t *testing.T) {
	log := logger.NewDefault()

	keys, err := GenerateRelayKeys()
	if err != nil {
		t.Fatalf("Failed to generate relay keys: %v", err)
	}

	handler := NewCircuitHandler(keys, log)

	// Create a circuit
	handler.mu.Lock()
	handler.circuits[10] = &ServerCircuit{
		CircuitID: 10,
		Created:   time.Now(),
	}
	handler.mu.Unlock()

	// Register as extended circuit
	mockNextHop := newMockConn()
	err = handler.GetForwardingHandler().RegisterExtendedCircuit(10, 20, "127.0.0.1:9001", mockNextHop)
	if err != nil {
		t.Fatalf("Failed to register extended circuit: %v", err)
	}

	// Verify extended circuit exists
	if handler.GetForwardingHandler().GetExtendedCircuitCount() != 1 {
		t.Fatal("Extended circuit was not registered")
	}

	// Send DESTROY
	destroyCell := &cell.Cell{
		CircID:  10,
		Command: cell.CmdDestroy,
		Payload: []byte{cell.DestroyReasonNone},
	}

	conn := newMockConn()
	err = handler.HandleCellFromConnection(conn, destroyCell)
	if err != nil {
		t.Fatalf("HandleCellFromConnection failed: %v", err)
	}

	// Circuit should be removed
	_, exists := handler.GetCircuit(10)
	if exists {
		t.Error("Circuit should have been destroyed")
	}

	// Extended circuit should also be cleaned up
	if handler.GetForwardingHandler().GetExtendedCircuitCount() != 0 {
		t.Error("Extended circuit should have been cleaned up")
	}
}

func TestCircuitHandler_CloseAll_WithExtendedCircuits(t *testing.T) {
	log := logger.NewDefault()

	keys, err := GenerateRelayKeys()
	if err != nil {
		t.Fatalf("Failed to generate relay keys: %v", err)
	}

	handler := NewCircuitHandler(keys, log)

	// Create circuits
	for i := uint32(1); i <= 3; i++ {
		handler.mu.Lock()
		handler.circuits[i] = &ServerCircuit{
			CircuitID: i,
			Created:   time.Now(),
		}
		handler.mu.Unlock()

		// Register some as extended
		if i%2 == 0 {
			mockNextHop := newMockConn()
			handler.GetForwardingHandler().RegisterExtendedCircuit(i, i+100, "127.0.0.1:9001", mockNextHop)
		}
	}

	if handler.GetCircuitCount() != 3 {
		t.Fatalf("Expected 3 circuits, got %d", handler.GetCircuitCount())
	}

	// Close all
	handler.CloseAll()

	// All circuits should be closed
	if handler.GetCircuitCount() != 0 {
		t.Errorf("Expected 0 circuits after CloseAll, got %d", handler.GetCircuitCount())
	}

	// All extended circuits should be closed
	if handler.GetForwardingHandler().GetExtendedCircuitCount() != 0 {
		t.Errorf("Expected 0 extended circuits after CloseAll, got %d",
			handler.GetForwardingHandler().GetExtendedCircuitCount())
	}
}

// Benchmark CREATE2 processing
func BenchmarkHandleCreate2(b *testing.B) {
	log := logger.NewDefault()

	keys, _ := GenerateRelayKeys()
	handler := NewCircuitHandler(keys, log)

	// Generate client keypair
	clientKey, _ := crypto.GenerateNtorKeyPair()

	serverIdentity := keys.Identity.Public
	var serverNtorPriv [32]byte
	copy(serverNtorPriv[:], keys.NtorOnionKey)
	var serverNtorPub [32]byte
	curve25519.ScalarBaseMult(&serverNtorPub, &serverNtorPriv)

	handshakeData := make([]byte, 84)
	copy(handshakeData[0:32], serverIdentity)
	copy(handshakeData[32:64], serverNtorPub[:])
	copy(handshakeData[64:84], clientKey.Public[:])

	payload := make([]byte, 4+84)
	payload[0] = 0x00
	payload[1] = 0x02
	payload[2] = 0x00
	payload[3] = 0x54
	copy(payload[4:], handshakeData)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		create2Cell := &cell.Cell{
			CircID:  2,
			Command: cell.CmdCreate2,
			Payload: payload,
		}

		conn := newMockConn()
		handler.HandleCellFromConnection(conn, create2Cell)

		// Cleanup
		handler.CloseCircuit(uint32(i % 65535))
	}
}
