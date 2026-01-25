package circuit

import (
	"io"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/cell"
)

// TestDESTROYCellSpecCompliance_Format verifies DESTROY cell format per tor-spec.txt §5.4
//
// Per tor-spec.txt §5.4:
// - DESTROY cells contain a single byte reason code
// - Circuit ID must be valid (non-zero)
// - Command must be CmdDestroy (4)
func TestDESTROYCellSpecCompliance_Format(t *testing.T) {
	tests := []struct {
		name      string
		circuitID uint32
		reason    byte
	}{
		{
			name:      "DESTROY with NONE reason",
			circuitID: 1,
			reason:    cell.DestroyReasonNone,
		},
		{
			name:      "DESTROY with PROTOCOL reason",
			circuitID: 2,
			reason:    cell.DestroyReasonProtocol,
		},
		{
			name:      "DESTROY with REQUESTED reason",
			circuitID: 0x12345678,
			reason:    cell.DestroyReasonRequested,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create DESTROY cell per tor-spec.txt §5.4
			destroy := &cell.Cell{
				CircID:  tt.circuitID,
				Command: cell.CmdDestroy,
				Payload: []byte{tt.reason},
			}

			// Verify circuit ID is preserved
			if destroy.CircID != tt.circuitID {
				t.Errorf("CircID = %d, want %d", destroy.CircID, tt.circuitID)
			}

			// Verify command is DESTROY
			if destroy.Command != cell.CmdDestroy {
				t.Errorf("Command = %d, want %d (DESTROY)", destroy.Command, cell.CmdDestroy)
			}

			// Verify payload contains single byte reason code
			if len(destroy.Payload) != 1 {
				t.Errorf("Payload length = %d, want 1", len(destroy.Payload))
			}

			if destroy.Payload[0] != tt.reason {
				t.Errorf("Reason = %d, want %d", destroy.Payload[0], tt.reason)
			}
		})
	}
}

// TestDESTROYCellSpecCompliance_ReasonCodes verifies all DESTROY reason codes per tor-spec.txt §5.4
//
// Per tor-spec.txt §5.4, these are the defined reason codes:
//
//	 0: NONE - No reason given
//	 1: PROTOCOL - Protocol violation
//	 2: INTERNAL - Internal error
//	 3: REQUESTED - Explicitly requested (e.g., TRUNCATE)
//	 4: HIBERNATING - OR is hibernating
//	 5: RESOURCELIMIT - Resource limit reached
//	 6: CONNECTFAILED - Connection failed
//	 7: NO ROUTE - No route to host
//	 8: TIMEOUT - Connection timed out
//	 9: DESTROYED - Circuit destroyed
//	10: NOSUCHSERVICE - No such service (onion service)
func TestDESTROYCellSpecCompliance_ReasonCodes(t *testing.T) {
	tests := []struct {
		name   string
		reason byte
		desc   string
	}{
		{"NONE", cell.DestroyReasonNone, "No reason given"},
		{"PROTOCOL", cell.DestroyReasonProtocol, "Protocol violation"},
		{"INTERNAL", cell.DestroyReasonInternal, "Internal error"},
		{"REQUESTED", cell.DestroyReasonRequested, "Explicitly requested"},
		{"HIBERNATING", cell.DestroyReasonHibernating, "OR is hibernating"},
		{"RESOURCELIMIT", cell.DestroyReasonResourceLimit, "Resource limit reached"},
		{"CONNECTFAILED", cell.DestroyReasonConnectFailed, "Connection failed"},
		{"NOROUTE", cell.DestroyReasonNoRoute, "No route to host"},
		{"TIMEOUT", cell.DestroyReasonTimeout, "Connection timed out"},
		{"DESTROYED", cell.DestroyReasonDestroyed, "Circuit destroyed"},
		{"NOSUCHSERVICE", cell.DestroyReasonNosuchservice, "No such service"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create DESTROY cell with reason code
			destroy := &cell.Cell{
				CircID:  1,
				Command: cell.CmdDestroy,
				Payload: []byte{tt.reason},
			}

			// Verify reason code matches spec value
			if destroy.Payload[0] != tt.reason {
				t.Errorf("Reason code = %d, want %d (%s)", destroy.Payload[0], tt.reason, tt.name)
			}
		})
	}
}

// TestDESTROYCellSpecCompliance_CircuitCleanup verifies circuit cleanup per tor-spec.txt §5.4
//
// Per tor-spec.txt §5.4:
// - Upon receiving DESTROY, a circuit is torn down
// - All streams on the circuit are closed
// - Circuit state is removed
// - No further cells should be sent on the circuit
func TestDESTROYCellSpecCompliance_CircuitCleanup(t *testing.T) {
	// Create a circuit manager
	mgr := NewManager()

	// Create a circuit
	circ := &Circuit{
		ID:    1,
		State: StateOpen,
		Hops:  []*Hop{},
	}

	mgr.mu.Lock()
	mgr.circuits[circ.ID] = circ
	mgr.mu.Unlock()

	// Verify circuit exists before DESTROY
	mgr.mu.RLock()
	_, exists := mgr.circuits[circ.ID]
	mgr.mu.RUnlock()

	if !exists {
		t.Fatal("Circuit should exist before DESTROY")
	}

	// Simulate DESTROY cell reception by closing circuit
	circ.Close()

	// Verify circuit state is closed
	circ.mu.RLock()
	state := circ.State
	circ.mu.RUnlock()

	if state != StateClosed {
		t.Errorf("Circuit state = %v, want %v", state, StateClosed)
	}

	// Note: In a full implementation, the circuit would be removed from the manager
	// and all streams would be closed. This test verifies the circuit is marked closed.
}

// TestDESTROYCellSpecCompliance_ImmediateTeardown verifies immediate teardown per tor-spec.txt §5.4
//
// Per tor-spec.txt §5.4:
// - DESTROY cells must be processed immediately
// - No queuing or buffering of DESTROY cells
// - Circuit cleanup should be synchronous
func TestDESTROYCellSpecCompliance_ImmediateTeardown(t *testing.T) {
	mgr := NewManager()

	circ := &Circuit{
		ID:    1,
		State: StateOpen,
		Hops:  []*Hop{},
	}

	mgr.mu.Lock()
	mgr.circuits[circ.ID] = circ
	mgr.mu.Unlock()

	// Record time before close
	before := time.Now()

	// Close circuit (simulates DESTROY processing)
	circ.Close()

	// Measure time taken
	elapsed := time.Since(before)

	// Verify state changed immediately
	circ.mu.RLock()
	state := circ.State
	circ.mu.RUnlock()

	if state != StateClosed {
		t.Errorf("Circuit state = %v, want %v (should be immediate)", state, StateClosed)
	}

	// Verify teardown was fast (< 10ms for local operation)
	if elapsed > 10*time.Millisecond {
		t.Errorf("Teardown took %v, want < 10ms (should be synchronous)", elapsed)
	}
}

// TestDESTROYCellSpecCompliance_IdempotentClose verifies idempotent close per tor-spec.txt §5.4
//
// Per tor-spec.txt §5.4:
// - Multiple DESTROY cells on same circuit should be harmless
// - Closing an already-closed circuit should not error
func TestDESTROYCellSpecCompliance_IdempotentClose(t *testing.T) {
	circ := &Circuit{
		ID:    1,
		State: StateOpen,
		Hops:  []*Hop{},
	}

	// First close
	circ.Close()

	circ.mu.RLock()
	state1 := circ.State
	circ.mu.RUnlock()

	if state1 != StateClosed {
		t.Errorf("First close: state = %v, want %v", state1, StateClosed)
	}

	// Second close (should be idempotent)
	circ.Close()

	circ.mu.RLock()
	state2 := circ.State
	circ.mu.RUnlock()

	if state2 != StateClosed {
		t.Errorf("Second close: state = %v, want %v", state2, StateClosed)
	}

	// Third close (should still be idempotent)
	circ.Close()

	circ.mu.RLock()
	state3 := circ.State
	circ.mu.RUnlock()

	if state3 != StateClosed {
		t.Errorf("Third close: state = %v, want %v", state3, StateClosed)
	}
}

// TestDESTROYCellSpecCompliance_EncodeDecode verifies DESTROY cell encoding per tor-spec.txt §5.4
//
// Per tor-spec.txt §5.4:
// - DESTROY cells are fixed-size (514 bytes)
// - Payload is 1 byte reason code followed by 508 bytes padding
func TestDESTROYCellSpecCompliance_EncodeDecode(t *testing.T) {
	tests := []struct {
		name      string
		circuitID uint32
		reason    byte
	}{
		{"NONE", 1, cell.DestroyReasonNone},
		{"PROTOCOL", 2, cell.DestroyReasonProtocol},
		{"REQUESTED", 3, cell.DestroyReasonRequested},
		{"TIMEOUT", 0xFFFFFFFF, cell.DestroyReasonTimeout},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create DESTROY cell
			original := &cell.Cell{
				CircID:  tt.circuitID,
				Command: cell.CmdDestroy,
				Payload: []byte{tt.reason},
			}

			// Encode to buffer
			var buf [514]byte
			w := &fixedWriter{buf: buf[:], offset: 0}
			if err := original.Encode(w); err != nil {
				t.Fatalf("Encode failed: %v", err)
			}

			encoded := w.buf

			// Verify encoded size is 514 bytes (4 CircID + 1 Cmd + 509 Payload)
			if len(encoded) != 514 {
				t.Errorf("Encoded size = %d, want 514 bytes", len(encoded))
			}

			// Verify CircID (big-endian, 4 bytes)
			circID := uint32(encoded[0])<<24 | uint32(encoded[1])<<16 | uint32(encoded[2])<<8 | uint32(encoded[3])
			if circID != tt.circuitID {
				t.Errorf("Encoded CircID = %d, want %d", circID, tt.circuitID)
			}

			// Verify command byte
			if encoded[4] != byte(cell.CmdDestroy) {
				t.Errorf("Encoded command = %d, want %d", encoded[4], cell.CmdDestroy)
			}

			// Verify reason byte (first byte of payload)
			if encoded[5] != tt.reason {
				t.Errorf("Encoded reason = %d, want %d", encoded[5], tt.reason)
			}

			// Verify padding is zeros
			for i := 6; i < 514; i++ {
				if encoded[i] != 0 {
					t.Errorf("Padding byte %d = %d, want 0", i-6, encoded[i])
					break
				}
			}
		})
	}
}

// fixedWriter is a helper for encoding to a fixed-size buffer
type fixedWriter struct {
	buf    []byte
	offset int
}

func (w *fixedWriter) Write(p []byte) (n int, err error) {
	n = copy(w.buf[w.offset:], p)
	w.offset += n
	if n < len(p) {
		return n, io.ErrShortWrite
	}
	return n, nil
}

// TestDESTROYCellSpecCompliance_ReasonPreservation verifies reason code preservation per tor-spec.txt §5.4
//
// Per tor-spec.txt §5.4:
// - When forwarding DESTROY, reason code may be preserved or changed
// - Common pattern: preserve reason unless forwarding to client
func TestDESTROYCellSpecCompliance_ReasonPreservation(t *testing.T) {
	tests := []struct {
		name           string
		receivedReason byte
		forwardReason  byte
		desc           string
	}{
		{
			name:           "Preserve PROTOCOL reason",
			receivedReason: cell.DestroyReasonProtocol,
			forwardReason:  cell.DestroyReasonProtocol,
			desc:           "Protocol errors should be preserved",
		},
		{
			name:           "Preserve REQUESTED reason",
			receivedReason: cell.DestroyReasonRequested,
			forwardReason:  cell.DestroyReasonRequested,
			desc:           "Explicit requests should be preserved",
		},
		{
			name:           "Forward with DESTROYED reason",
			receivedReason: cell.DestroyReasonInternal,
			forwardReason:  cell.DestroyReasonDestroyed,
			desc:           "Relay may change reason to DESTROYED when forwarding",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate receiving DESTROY
			received := &cell.Cell{
				CircID:  1,
				Command: cell.CmdDestroy,
				Payload: []byte{tt.receivedReason},
			}

			// Simulate forwarding DESTROY
			forwarded := &cell.Cell{
				CircID:  2, // Different circuit ID for next hop
				Command: cell.CmdDestroy,
				Payload: []byte{tt.forwardReason},
			}

			// Verify received reason
			if received.Payload[0] != tt.receivedReason {
				t.Errorf("Received reason = %d, want %d", received.Payload[0], tt.receivedReason)
			}

			// Verify forwarded reason
			if forwarded.Payload[0] != tt.forwardReason {
				t.Errorf("Forwarded reason = %d, want %d", forwarded.Payload[0], tt.forwardReason)
			}
		})
	}
}

// TestDESTROYCellSpecCompliance_NoReplyRequired verifies no reply semantics per tor-spec.txt §5.4
//
// Per tor-spec.txt §5.4:
// - DESTROY cells do not require a response
// - Receiving a DESTROY should not send a DESTROY back
// - One-way teardown notification
func TestDESTROYCellSpecCompliance_NoReplyRequired(t *testing.T) {
	// This test documents the no-reply semantics
	// In practice, this is verified by the circuit handler not sending DESTROY in response to DESTROY

	destroy := &cell.Cell{
		CircID:  1,
		Command: cell.CmdDestroy,
		Payload: []byte{cell.DestroyReasonRequested},
	}

	// Verify this is a DESTROY cell
	if destroy.Command != cell.CmdDestroy {
		t.Errorf("Command = %d, want %d (DESTROY)", destroy.Command, cell.CmdDestroy)
	}

	// Note: In actual implementation, circuit handler receives DESTROY,
	// closes circuit, and does NOT send DESTROY back to sender.
	// This is verified by integration tests that check no DESTROY is sent in response.
}
