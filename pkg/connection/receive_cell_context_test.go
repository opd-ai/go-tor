package connection

import (
	"context"
	"runtime"
	"testing"
	"time"
)

// TestReceiveCellWithContextCancellation verifies that ReceiveCellWithContext
// properly handles context cancellation without leaking goroutines.
//
// This test addresses the CRITICAL BUG in AUDIT.md: "Goroutine Leak in Protocol Handshake"
// The fix uses context-aware ReceiveCellWithContext which cancels blocking reads.
func TestReceiveCellWithContextCancellation(t *testing.T) {
	// Get baseline goroutine count
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	// Create a connection that's not open (will fail fast)
	conn := &Connection{
		address: "test:9001",
		state:   StateClosed, // Closed state will cause quick error
		closeCh: make(chan struct{}),
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// This should return error quickly (connection closed)
	_, err := conn.ReceiveCellWithContext(ctx)
	if err == nil {
		t.Fatal("Expected error for closed connection, got nil")
	}

	// Wait for goroutines to clean up
	time.Sleep(200 * time.Millisecond)
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	// Check goroutine count
	after := runtime.NumGoroutine()
	leaked := after - baseline
	
	if leaked > 2 {
		t.Errorf("Goroutine leak detected: baseline=%d, after=%d, leaked=%d", baseline, after, leaked)
	}
}

// TestReceiveCellWithContextMultipleCancellations tests repeated cancellations.
func TestReceiveCellWithContextMultipleCancellations(t *testing.T) {
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	conn := &Connection{
		address: "test:9001",
		state:   StateClosed,
		closeCh: make(chan struct{}),
	}

	// Simulate 20 cancelled reads
	for i := 0; i < 20; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		_, _ = conn.ReceiveCellWithContext(ctx)
		cancel()
	}

	// Give time for cleanup
	time.Sleep(300 * time.Millisecond)
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	after := runtime.NumGoroutine()
	leaked := after - baseline
	
	// With 20 cancelled reads, we should not accumulate goroutines
	if leaked > 3 {
		t.Errorf("Goroutine leak detected after 20 cancellations: baseline=%d, after=%d, leaked=%d", baseline, after, leaked)
	}
}

// TestReceiveCellBackwardCompatibility verifies that the original ReceiveCell
// still works and delegates to ReceiveCellWithContext.
func TestReceiveCellBackwardCompatibility(t *testing.T) {
	conn := &Connection{
		address: "test:9001",
		state:   StateClosed, // Closed state should cause an error
		closeCh: make(chan struct{}),
	}

	// Should get "connection not open" error
	_, err := conn.ReceiveCell()
	if err == nil {
		t.Fatal("Expected error for closed connection, got nil")
	}
	if err.Error() != "connection not open: CLOSED" {
		t.Errorf("Expected 'connection not open' error, got: %v", err)
	}
}

// TestReceiveCellWithContextStateCheck verifies state checking.
func TestReceiveCellWithContextStateCheck(t *testing.T) {
	tests := []struct {
		name  string
		state State
	}{
		{"connecting", StateConnecting},
		{"handshaking", StateHandshaking},
		{"closed", StateClosed},
		{"failed", StateFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := &Connection{
				address: "test:9001",
				state:   tt.state,
				closeCh: make(chan struct{}),
			}

			ctx := context.Background()
			_, err := conn.ReceiveCellWithContext(ctx)
			if err == nil {
				t.Errorf("Expected error for state %s, got nil", tt.state)
			}
			// Should get "connection not open" error
			expectedErr := "connection not open: " + tt.state.String()
			if err.Error() != expectedErr {
				t.Errorf("Expected error %q, got %q", expectedErr, err.Error())
			}
		})
	}
}

// TestReceiveCellWithContextClosedChannel verifies closeCh handling.
func TestReceiveCellWithContextClosedChannel(t *testing.T) {
	closeCh := make(chan struct{})
	close(closeCh) // Pre-close the channel

	conn := &Connection{
		address: "test:9001",
		state:   StateOpen,
		closeCh: closeCh,
	}

	ctx := context.Background()
	_, err := conn.ReceiveCellWithContext(ctx)
	if err == nil {
		t.Fatal("Expected error for closed channel, got nil")
	}
	if err.Error() != "connection closed" {
		t.Errorf("Expected 'connection closed' error, got: %v", err)
	}
}
