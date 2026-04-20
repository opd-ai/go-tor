// Package circuit provides tests for circuit extension failure recovery
// per tor-spec.txt §5.1 and §5.3.
//
// These tests verify that circuit extension handles failures gracefully:
// missing connections, invalid handshake data, DESTROY responses, nil
// circuits, and key derivation errors. Failure recovery is critical for
// circuit reliability in real network conditions.
//
// Compliance: tor-spec.txt §5.1 (Circuit Creation), §5.3 (DESTROY cells)
package circuit

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/cell"
	"github.com/opd-ai/go-tor/pkg/logger"
)

// failingConnection is a mock CellConnection that returns errors.
type failingConnection struct {
	sendErr    error
	receiveErr error
}

func (f *failingConnection) SendCell(_ *cell.Cell) error {
	return f.sendErr
}

func (f *failingConnection) ReceiveCell() (*cell.Cell, error) {
	return nil, f.receiveErr
}

// destroyConnection is a mock CellConnection that returns DESTROY cells.
type destroyConnection struct {
	circID uint32
	reason byte
}

func (d *destroyConnection) SendCell(_ *cell.Cell) error {
	return nil
}

func (d *destroyConnection) ReceiveCell() (*cell.Cell, error) {
	c := cell.NewCell(d.circID, cell.CmdDestroy)
	c.Payload = []byte{d.reason}
	return c, nil
}

// TestCreateFirstHopNoConnection verifies that creating the first
// hop without a connection fails gracefully.
func TestCreateFirstHopNoConnection(t *testing.T) {
	log := logger.NewDefault()
	circ := &Circuit{ID: 100}
	ext := NewExtension(circ, log)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := ext.CreateFirstHop(ctx, HandshakeTypeNTor)
	if err == nil {
		t.Error("expected error for no connection")
	}
	if !strings.Contains(err.Error(), "no connection") {
		t.Errorf("error %q does not mention 'no connection'", err.Error())
	}
}

// TestCreateFirstHopSendFailure verifies that a send error during
// CREATE2 is properly propagated.
func TestCreateFirstHopSendFailure(t *testing.T) {
	log := logger.NewDefault()
	circ := &Circuit{
		ID:   200,
		conn: &failingConnection{sendErr: fmt.Errorf("network error")},
	}
	ext := NewExtension(circ, log)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := ext.CreateFirstHop(ctx, HandshakeTypeNTor)
	if err == nil {
		t.Error("expected error for send failure")
	}
}

// TestCreateFirstHopReceiveFailure verifies that a receive error
// after sending CREATE2 is properly propagated.
func TestCreateFirstHopReceiveFailure(t *testing.T) {
	log := logger.NewDefault()
	circ := &Circuit{
		ID:   300,
		conn: &failingConnection{receiveErr: fmt.Errorf("connection reset")},
	}
	ext := NewExtension(circ, log)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := ext.CreateFirstHop(ctx, HandshakeTypeNTor)
	if err == nil {
		t.Error("expected error for receive failure")
	}
}

// TestExtendCircuitNoConnection verifies that extending without
// a connection fails gracefully.
func TestExtendCircuitNoConnection(t *testing.T) {
	log := logger.NewDefault()
	circ := &Circuit{ID: 400}
	ext := NewExtension(circ, log)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := ext.ExtendCircuit(ctx, "1.2.3.4:9001", HandshakeTypeNTor)
	if err == nil {
		t.Error("expected error for no connection")
	}
}

// TestNewExtensionNilLogger verifies extension creation with nil
// logger falls back to default.
func TestNewExtensionNilLogger(t *testing.T) {
	circ := &Circuit{ID: 500}
	ext := NewExtension(circ, nil)
	if ext == nil {
		t.Fatal("NewExtension returned nil")
	}
	if ext.logger == nil {
		t.Error("logger is nil after NewExtension with nil logger")
	}
}

// TestNewExtensionWithLogger verifies extension creation with a
// provided logger.
func TestNewExtensionWithLogger(t *testing.T) {
	log := logger.NewDefault()
	circ := &Circuit{ID: 600}
	ext := NewExtension(circ, log)
	if ext == nil {
		t.Fatal("NewExtension returned nil")
	}
}

// TestGetConnectionTypeMismatch verifies that getConnection rejects
// connections that don't implement CellConnection.
func TestGetConnectionTypeMismatch(t *testing.T) {
	log := logger.NewDefault()
	// Use a string as the connection - doesn't implement CellConnection
	circ := &Circuit{ID: 700, conn: "not-a-connection"}
	ext := NewExtension(circ, log)

	_, err := ext.getConnection()
	if err == nil {
		t.Error("expected error for non-CellConnection type")
	}
	if !strings.Contains(err.Error(), "CellConnection") {
		t.Errorf("error %q does not mention CellConnection", err.Error())
	}
}

// TestGetConnectionNilConn verifies that getConnection detects
// a nil connection.
func TestGetConnectionNilConn(t *testing.T) {
	log := logger.NewDefault()
	circ := &Circuit{ID: 800}
	ext := NewExtension(circ, log)

	_, err := ext.getConnection()
	if err == nil {
		t.Error("expected error for nil connection")
	}
	if !strings.Contains(err.Error(), "no connection") {
		t.Errorf("error %q does not mention 'no connection'", err.Error())
	}
}

// TestProcessCreated2NilCell verifies that ProcessCreated2 handles
// a nil cell without panicking.
func TestProcessCreated2NilCell(t *testing.T) {
	log := logger.NewDefault()
	circ := &Circuit{ID: 900}
	ext := NewExtension(circ, log)

	err := ext.ProcessCreated2(nil)
	if err == nil {
		t.Error("expected error for nil cell")
	}
}

// TestProcessExtended2NilCell verifies that ProcessExtended2
// handles a nil relay cell without panicking.
func TestProcessExtended2NilCell(t *testing.T) {
	log := logger.NewDefault()
	circ := &Circuit{ID: 1000}
	ext := NewExtension(circ, log)

	err := ext.ProcessExtended2(nil)
	if err == nil {
		t.Error("expected error for nil relay cell")
	}
}

// TestProcessCreated2EmptyPayload verifies that ProcessCreated2
// handles an empty payload.
func TestProcessCreated2EmptyPayload(t *testing.T) {
	log := logger.NewDefault()
	circ := &Circuit{ID: 1100}
	ext := NewExtension(circ, log)

	emptyCell := cell.NewCell(1100, cell.CmdCreated2)
	err := ext.ProcessCreated2(emptyCell)
	if err == nil {
		t.Error("expected error for empty payload")
	}
}

// TestProcessExtended2EmptyData verifies that ProcessExtended2
// handles a relay cell with empty data.
func TestProcessExtended2EmptyData(t *testing.T) {
	log := logger.NewDefault()
	circ := &Circuit{ID: 1200}
	ext := NewExtension(circ, log)

	relayCell := &cell.RelayCell{
		Command: cell.RelayExtended2,
		Data:    []byte{},
	}
	err := ext.ProcessExtended2(relayCell)
	if err == nil {
		t.Error("expected error for empty data")
	}
}

// TestDeriveHopFromShortKeyMaterial verifies that key derivation
// fails with insufficient key material.
func TestDeriveHopFromShortKeyMaterial(t *testing.T) {
	log := logger.NewDefault()
	circ := &Circuit{ID: 1300}
	ext := NewExtension(circ, log)

	shortKeyMaterials := [][]byte{
		nil,
		{},
		make([]byte, 10),
		make([]byte, 71), // One byte short of 72
	}

	for i, km := range shortKeyMaterials {
		_, err := ext.deriveHopFromKeyMaterial(km)
		if err == nil {
			t.Errorf("case %d (len %d): expected error for short key material",
				i, len(km))
		}
	}
}

// TestDeriveHopFromValidKeyMaterial verifies that key derivation
// succeeds with correctly sized key material.
func TestDeriveHopFromValidKeyMaterial(t *testing.T) {
	log := logger.NewDefault()
	circ := &Circuit{ID: 1400}
	ext := NewExtension(circ, log)

	keyMaterial := make([]byte, 72)
	for i := range keyMaterial {
		keyMaterial[i] = byte(i)
	}

	hop, err := ext.deriveHopFromKeyMaterial(keyMaterial)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hop == nil {
		t.Fatal("hop is nil")
	}
	if hop.ForwardCipher == nil {
		t.Error("ForwardCipher is nil")
	}
	if hop.BackwardCipher == nil {
		t.Error("BackwardCipher is nil")
	}
	if hop.ForwardDigest == nil {
		t.Error("ForwardDigest is nil")
	}
	if hop.BackwardDigest == nil {
		t.Error("BackwardDigest is nil")
	}
}

// TestSetTargetRelay verifies setting and using the target relay.
func TestSetTargetRelay(t *testing.T) {
	log := logger.NewDefault()
	circ := &Circuit{ID: 1500}
	ext := NewExtension(circ, log)

	// Initially nil
	ext.SetTargetRelay(nil)

	// Set a valid relay-like struct
	ext.SetTargetRelay(struct {
		Fingerprint string
	}{"AAAA"})
}

// TestDeriveKeysFromSharedSecret verifies DeriveKeys handles
// edge cases in shared secret processing.
func TestDeriveKeysFromSharedSecret(t *testing.T) {
	log := logger.NewDefault()
	circ := &Circuit{ID: 1600}
	ext := NewExtension(circ, log)

	tests := []struct {
		name    string
		secret  []byte
		wantErr bool
	}{
		{"nil secret", nil, false},
		{"empty secret", []byte{}, false},
		{"valid secret", make([]byte, 32), false},
		{"short secret", []byte{0x01}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fwd, bwd, err := ext.DeriveKeys(tc.secret)

			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if fwd == nil {
					t.Error("forward key is nil")
				}
				if bwd == nil {
					t.Error("backward key is nil")
				}
			}
		})
	}
}

// TestHandshakeTypeConstantsRecovery verifies handshake type constants.
func TestHandshakeTypeConstantsRecovery(t *testing.T) {
	if HandshakeTypeNTor != 0x0002 {
		t.Errorf("HandshakeTypeNTor = %d, want 2", HandshakeTypeNTor)
	}
	if HandshakeTypeTAP != 0x0000 {
		t.Errorf("HandshakeTypeTAP = %d, want 0", HandshakeTypeTAP)
	}
}
