// Package stream provides Tor stream management for multiplexing connections over circuits.
package stream

import (
	"testing"
)

// TestStreamFlowControlInitialization tests initial window values
func TestStreamFlowControlInitialization(t *testing.T) {
	stream := NewStream(1, 100, "example.com", 443, nil)

	// Per tor-spec.txt §7.4, initial stream window is 500
	if got := stream.GetPackageWindow(); got != 500 {
		t.Errorf("Initial packageWindow = %d, want 500", got)
	}

	if got := stream.GetDeliverWindow(); got != 500 {
		t.Errorf("Initial deliverWindow = %d, want 500", got)
	}
}

// TestStreamPackageWindowDecrement tests package window decrement
func TestStreamPackageWindowDecrement(t *testing.T) {
	stream := NewStream(1, 100, "example.com", 443, nil)

	// Decrement once
	if err := stream.DecrementPackageWindow(); err != nil {
		t.Fatalf("decrementPackageWindow() error = %v", err)
	}

	if got := stream.GetPackageWindow(); got != 499 {
		t.Errorf("packageWindow after decrement = %d, want 499", got)
	}
}

// TestStreamPackageWindowExhaustion tests package window exhaustion
func TestStreamPackageWindowExhaustion(t *testing.T) {
	stream := NewStream(1, 100, "example.com", 443, nil)

	// Exhaust the package window
	for i := 0; i < 500; i++ {
		if err := stream.DecrementPackageWindow(); err != nil {
			t.Fatalf("decrementPackageWindow() at iteration %d error = %v", i, err)
		}
	}

	// Next decrement should fail
	if err := stream.DecrementPackageWindow(); err == nil {
		t.Error("decrementPackageWindow() should fail when window exhausted")
	}
}

// TestStreamPackageWindowIncrement tests package window increment
func TestStreamPackageWindowIncrement(t *testing.T) {
	stream := NewStream(1, 100, "example.com", 443, nil)

	// Decrement to lower the window
	for i := 0; i < 100; i++ {
		stream.DecrementPackageWindow()
	}

	// Per tor-spec.txt §7.4, each SENDME increments by 50
	stream.IncrementPackageWindow()

	if got := stream.GetPackageWindow(); got != 450 {
		t.Errorf("packageWindow after increment = %d, want 450 (500 - 100 + 50)", got)
	}
}

// TestStreamDeliverWindowDecrement tests deliver window decrement
func TestStreamDeliverWindowDecrement(t *testing.T) {
	stream := NewStream(1, 100, "example.com", 443, nil)

	// Decrement once
	if err := stream.DecrementDeliverWindow(); err != nil {
		t.Fatalf("decrementDeliverWindow() error = %v", err)
	}

	if got := stream.GetDeliverWindow(); got != 499 {
		t.Errorf("deliverWindow after decrement = %d, want 499", got)
	}
}

// TestStreamDeliverWindowExhaustion tests deliver window exhaustion
func TestStreamDeliverWindowExhaustion(t *testing.T) {
	stream := NewStream(1, 100, "example.com", 443, nil)

	// Exhaust the deliver window
	for i := 0; i < 500; i++ {
		if err := stream.DecrementDeliverWindow(); err != nil {
			t.Fatalf("decrementDeliverWindow() at iteration %d error = %v", i, err)
		}
	}

	// Next decrement should fail
	if err := stream.DecrementDeliverWindow(); err == nil {
		t.Error("decrementDeliverWindow() should fail when window exhausted")
	}
}

// TestStreamShouldSendSendme tests SENDME trigger logic
func TestStreamShouldSendSendme(t *testing.T) {
	stream := NewStream(1, 100, "example.com", 443, nil)

	// Initially should not need SENDME
	if stream.ShouldSendStreamSendme() {
		t.Error("shouldSendStreamSendme() = true initially, want false")
	}

	// Receive 49 cells - should not trigger SENDME
	for i := 0; i < 49; i++ {
		stream.DecrementDeliverWindow()
	}

	if stream.ShouldSendStreamSendme() {
		t.Error("shouldSendStreamSendme() = true after 49 cells, want false")
	}

	// Receive 50th cell - should trigger SENDME
	stream.DecrementDeliverWindow()

	if !stream.ShouldSendStreamSendme() {
		t.Error("shouldSendStreamSendme() = false after 50 cells, want true")
	}
}

// TestStreamRecordSendmeSent tests SENDME sent recording
func TestStreamRecordSendmeSent(t *testing.T) {
	stream := NewStream(1, 100, "example.com", 443, nil)

	// Receive 50 cells
	for i := 0; i < 50; i++ {
		stream.DecrementDeliverWindow()
	}

	// Record SENDME sent
	stream.RecordStreamSendmeSent()

	// Counter should be reset
	if stream.ShouldSendStreamSendme() {
		t.Error("shouldSendStreamSendme() = true after recording SENDME, want false")
	}

	// Deliver window should be incremented by 50
	if got := stream.GetDeliverWindow(); got != 500 {
		t.Errorf("deliverWindow after SENDME = %d, want 500 (500 - 50 + 50)", got)
	}
}

// TestStreamFlowControlConcurrency tests concurrent window operations
func TestStreamFlowControlConcurrency(t *testing.T) {
	stream := NewStream(1, 100, "example.com", 443, nil)

	done := make(chan bool)

	// Concurrent decrements
	go func() {
		for i := 0; i < 100; i++ {
			stream.DecrementPackageWindow()
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			stream.DecrementDeliverWindow()
		}
		done <- true
	}()

	// Wait for both goroutines
	<-done
	<-done

	// Verify final state
	if got := stream.GetPackageWindow(); got != 400 {
		t.Errorf("packageWindow after concurrent decrements = %d, want 400", got)
	}

	if got := stream.GetDeliverWindow(); got != 400 {
		t.Errorf("deliverWindow after concurrent decrements = %d, want 400", got)
	}
}

// TestStreamFlowControlMultipleSendmeCycles tests multiple SENDME cycles
func TestStreamFlowControlMultipleSendmeCycles(t *testing.T) {
	stream := NewStream(1, 100, "example.com", 443, nil)

	// Simulate 3 SENDME cycles
	for cycle := 0; cycle < 3; cycle++ {
		// Receive 50 cells
		for i := 0; i < 50; i++ {
			if err := stream.DecrementDeliverWindow(); err != nil {
				t.Fatalf("Cycle %d: decrementDeliverWindow() error = %v", cycle, err)
			}
		}

		// Should trigger SENDME
		if !stream.ShouldSendStreamSendme() {
			t.Errorf("Cycle %d: shouldSendStreamSendme() = false, want true", cycle)
		}

		// Record SENDME
		stream.RecordStreamSendmeSent()
	}

	// After 3 cycles (150 cells received, 150 window restored)
	if got := stream.GetDeliverWindow(); got != 500 {
		t.Errorf("deliverWindow after 3 SENDME cycles = %d, want 500", got)
	}
}

// TestStreamWindowRecoveryFromExhaustion tests recovery from window exhaustion
func TestStreamWindowRecoveryFromExhaustion(t *testing.T) {
	stream := NewStream(1, 100, "example.com", 443, nil)

	// Exhaust package window
	for i := 0; i < 500; i++ {
		stream.DecrementPackageWindow()
	}

	// Verify exhaustion
	if err := stream.DecrementPackageWindow(); err == nil {
		t.Error("Window should be exhausted")
	}

	// Receive SENDME to recover
	stream.IncrementPackageWindow()

	// Should be able to send again
	if err := stream.DecrementPackageWindow(); err != nil {
		t.Errorf("decrementPackageWindow() after SENDME error = %v, want nil", err)
	}

	if got := stream.GetPackageWindow(); got != 49 {
		t.Errorf("packageWindow after recovery = %d, want 49 (0 + 50 - 1)", got)
	}
}
