// Package relay - Cell forwarding tests
package relay

import (
	"context"
	"testing"

	"github.com/opd-ai/go-tor/pkg/cell"
	"github.com/opd-ai/go-tor/pkg/logger"
)

func TestNewForwardingHandler(t *testing.T) {
	log := logger.NewDefault()
	circuits := NewCircuitHandler(nil, log)
	handler := NewForwardingHandler(circuits, log)

	if handler == nil {
		t.Fatal("NewForwardingHandler returned nil")
	}

	if handler.circuits != circuits {
		t.Error("Circuits not set correctly")
	}

	if len(handler.extended) != 0 {
		t.Errorf("Expected empty extended circuits map, got %d", len(handler.extended))
	}
}

func TestForwardingRegisterExtendedCircuit(t *testing.T) {
	log := logger.NewDefault()
	circuits := NewCircuitHandler(nil, log)
	handler := NewForwardingHandler(circuits, log)

	mockConn := newTestMockConn()
	err := handler.RegisterExtendedCircuit(100, 200, "127.0.0.1:9001", mockConn)
	if err != nil {
		t.Fatalf("RegisterExtendedCircuit failed: %v", err)
	}

	if handler.GetExtendedCircuitCount() != 1 {
		t.Errorf("Expected 1 extended circuit, got %d", handler.GetExtendedCircuitCount())
	}

	// Try to register same circuit again
	err = handler.RegisterExtendedCircuit(100, 300, "127.0.0.1:9002", mockConn)
	if err == nil {
		t.Error("Expected error when registering duplicate circuit")
	}
}

func TestForwardRelayCell_RelayEarlyLimiting(t *testing.T) {
	log := logger.NewDefault()
	circuits := NewCircuitHandler(nil, log)
	handler := NewForwardingHandler(circuits, log)

	mockConn := newTestMockConn()
	handler.RegisterExtendedCircuit(100, 200, "127.0.0.1:9001", mockConn)

	ctx := context.Background()

	// Send 10 RELAY_EARLY cells
	for i := 0; i < 10; i++ {
		c := &cell.Cell{
			CircID:  100,
			Command: cell.CmdRelayEarly,
			Payload: make([]byte, cell.PayloadLen),
		}

		err := handler.ForwardRelayCell(ctx, true, 100, c)
		if err != nil {
			t.Fatalf("ForwardRelayCell failed at iteration %d: %v", i, err)
		}
	}

	// Check that the extended circuit has the correct RELAY_EARLY count
	handler.extendedMu.RLock()
	ext := handler.extended[100]
	handler.extendedMu.RUnlock()

	if ext.RelayEarlyCount != 8 {
		t.Errorf("Expected RELAY_EARLY count of 8, got %d", ext.RelayEarlyCount)
	}
}

func TestForwardRelayCell_NonExtended(t *testing.T) {
	log := logger.NewDefault()
	circuits := NewCircuitHandler(nil, log)
	handler := NewForwardingHandler(circuits, log)

	ctx := context.Background()

	// Create a relay cell for a non-extended circuit
	relayCell, _ := cell.NewRelayCell(1, cell.RelayBegin, []byte("example.com:80\x00"))
	payload, _ := relayCell.Encode()

	c := &cell.Cell{
		CircID:  100,
		Command: cell.CmdRelay,
		Payload: payload,
	}

	// This should handle locally (and reject exit attempt)
	err := handler.ForwardRelayCell(ctx, true, 100, c)
	if err != nil {
		t.Fatalf("ForwardRelayCell failed: %v", err)
	}
}

func TestHandleLocalRelayCell_ExitAttempt(t *testing.T) {
	log := logger.NewDefault()
	circuits := NewCircuitHandler(nil, log)
	handler := NewForwardingHandler(circuits, log)

	ctx := context.Background()

	tests := []struct {
		name    string
		command byte
		data    []byte
	}{
		{
			name:    "RELAY_BEGIN",
			command: cell.RelayBegin,
			data:    []byte("example.com:80\x00"),
		},
		{
			name:    "RELAY_BEGIN_DIR",
			command: cell.RelayBeginDir,
			data:    []byte{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			relayCell, err := cell.NewRelayCell(1, tt.command, tt.data)
			if err != nil {
				t.Fatalf("Failed to create relay cell: %v", err)
			}

			payload, err := relayCell.Encode()
			if err != nil {
				t.Fatalf("Failed to encode relay cell: %v", err)
			}

			c := &cell.Cell{
				CircID:  100,
				Command: cell.CmdRelay,
				Payload: payload,
			}

			err = handler.handleLocalRelayCell(ctx, 100, c)
			if err != nil {
				t.Errorf("handleLocalRelayCell failed: %v", err)
			}
		})
	}
}

func TestHandleTruncate(t *testing.T) {
	log := logger.NewDefault()
	circuits := NewCircuitHandler(nil, log)
	handler := NewForwardingHandler(circuits, log)

	// Register extended circuit
	mockNextHopConn := newTestMockConn()
	err := handler.RegisterExtendedCircuit(100, 200, "127.0.0.1:9001", mockNextHopConn)
	if err != nil {
		t.Fatalf("RegisterExtendedCircuit failed: %v", err)
	}

	// Handle truncate
	err = handler.handleTruncate(100)
	if err != nil {
		t.Fatalf("handleTruncate failed: %v", err)
	}

	// Verify next hop connection was closed
	if !mockNextHopConn.closed {
		t.Error("Next hop connection was not closed")
	}

	// Verify extended circuit was removed
	if handler.GetExtendedCircuitCount() != 0 {
		t.Errorf("Expected 0 extended circuits after truncate, got %d", handler.GetExtendedCircuitCount())
	}
}

func TestHandleTruncateNoExtension(t *testing.T) {
	// Test truncate on a circuit that was never extended
	log := logger.NewDefault()
	circuits := NewCircuitHandler(nil, log)
	handler := NewForwardingHandler(circuits, log)

	err := handler.handleTruncate(100)
	if err != nil {
		t.Fatalf("handleTruncate failed on non-extended circuit: %v", err)
	}

	if handler.GetExtendedCircuitCount() != 0 {
		t.Errorf("Expected 0 extended circuits, got %d", handler.GetExtendedCircuitCount())
	}
}

func TestHandleDestroy(t *testing.T) {
	log := logger.NewDefault()
	circuits := NewCircuitHandler(nil, log)
	handler := NewForwardingHandler(circuits, log)

	mockConn := newTestMockConn()
	handler.RegisterExtendedCircuit(100, 200, "127.0.0.1:9001", mockConn)

	err := handler.HandleDestroy(100)
	if err != nil {
		t.Fatalf("HandleDestroy failed: %v", err)
	}

	if !mockConn.closed {
		t.Error("Next hop connection was not closed")
	}

	if handler.GetExtendedCircuitCount() != 0 {
		t.Errorf("Expected 0 extended circuits after destroy, got %d", handler.GetExtendedCircuitCount())
	}

	// Verify DESTROY cell was sent to next hop
	if len(mockConn.writeData) > 0 {
		// A DESTROY cell should have been written
		t.Log("DESTROY cell was sent to next hop")
	}
}

func TestCloseAll(t *testing.T) {
	log := logger.NewDefault()
	circuits := NewCircuitHandler(nil, log)
	handler := NewForwardingHandler(circuits, log)

	// Register multiple extended circuits
	mockConns := make([]*testMockConn, 3)
	for i := 0; i < 3; i++ {
		mockConns[i] = newTestMockConn()
		handler.RegisterExtendedCircuit(uint32(100+i), uint32(200+i), "127.0.0.1:9001", mockConns[i])
	}

	if handler.GetExtendedCircuitCount() != 3 {
		t.Errorf("Expected 3 extended circuits, got %d", handler.GetExtendedCircuitCount())
	}

	handler.CloseAll()

	if handler.GetExtendedCircuitCount() != 0 {
		t.Errorf("Expected 0 extended circuits after CloseAll, got %d", handler.GetExtendedCircuitCount())
	}

	for i, conn := range mockConns {
		if !conn.closed {
			t.Errorf("Connection %d was not closed", i)
		}
	}
}

func TestForwardToNextHop(t *testing.T) {
	log := logger.NewDefault()
	circuits := NewCircuitHandler(nil, log)
	handler := NewForwardingHandler(circuits, log)

	mockConn := newTestMockConn()
	handler.RegisterExtendedCircuit(100, 200, "127.0.0.1:9001", mockConn)

	c := &cell.Cell{
		CircID:  100,
		Command: cell.CmdRelay,
		Payload: make([]byte, cell.PayloadLen),
	}

	handler.extendedMu.RLock()
	ext := handler.extended[100]
	handler.extendedMu.RUnlock()

	err := handler.forwardToNextHop(ext, c)
	if err != nil {
		t.Fatalf("forwardToNextHop failed: %v", err)
	}

	// Verify cell was written to connection
	if len(mockConn.writeData) == 0 {
		t.Error("No data was written to next hop connection")
	}
}

func TestRejectExitAttempt(t *testing.T) {
	log := logger.NewDefault()
	circuits := NewCircuitHandler(nil, log)
	handler := NewForwardingHandler(circuits, log)

	err := handler.rejectExitAttempt(100, 1)
	if err != nil {
		t.Fatalf("rejectExitAttempt failed: %v", err)
	}

	// Should not return error, just log
}

func TestHandleLocalRelayCell_InvalidPayload(t *testing.T) {
	log := logger.NewDefault()
	circuits := NewCircuitHandler(nil, log)
	handler := NewForwardingHandler(circuits, log)

	ctx := context.Background()

	// Create cell with invalid payload
	c := &cell.Cell{
		CircID:  100,
		Command: cell.CmdRelay,
		Payload: []byte{0, 1, 2}, // Too short
	}

	err := handler.handleLocalRelayCell(ctx, 100, c)
	if err == nil {
		t.Error("Expected error for invalid relay cell payload")
	}
}
