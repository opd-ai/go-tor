package stream

import (
	"testing"

	"github.com/opd-ai/go-tor/pkg/logger"
)

// TestStreamFlowControlExportedMethods verifies that the exported
// flow control methods work correctly for circuit integration
func TestStreamFlowControlExportedMethods(t *testing.T) {
	log := logger.NewDefault()
	stream := NewStream(1, 100, "example.com", 80, log)

	// Test initial window values
	if got := stream.GetPackageWindow(); got != 500 {
		t.Errorf("Initial package window = %d, want 500", got)
	}
	if got := stream.GetDeliverWindow(); got != 500 {
		t.Errorf("Initial deliver window = %d, want 500", got)
	}

	// Test DecrementPackageWindow (exported)
	for i := 0; i < 10; i++ {
		if err := stream.DecrementPackageWindow(); err != nil {
			t.Fatalf("DecrementPackageWindow() at iteration %d error = %v", i, err)
		}
	}
	if got := stream.GetPackageWindow(); got != 490 {
		t.Errorf("After 10 decrements, package window = %d, want 490", got)
	}

	// Test DecrementDeliverWindow (exported)
	for i := 0; i < 20; i++ {
		if err := stream.DecrementDeliverWindow(); err != nil {
			t.Fatalf("DecrementDeliverWindow() at iteration %d error = %v", i, err)
		}
	}
	if got := stream.GetDeliverWindow(); got != 480 {
		t.Errorf("After 20 decrements, deliver window = %d, want 480", got)
	}

	// Test ShouldSendStreamSendme (exported)
	if stream.ShouldSendStreamSendme() {
		t.Error("ShouldSendStreamSendme() = true after 20 cells, want false")
	}

	// Receive 30 more cells to reach 50 total
	for i := 20; i < 50; i++ {
		if err := stream.DecrementDeliverWindow(); err != nil {
			t.Fatalf("DecrementDeliverWindow() at iteration %d error = %v", i, err)
		}
	}

	// Should trigger SENDME now
	if !stream.ShouldSendStreamSendme() {
		t.Error("ShouldSendStreamSendme() = false after 50 cells, want true")
	}

	// Test RecordStreamSendmeSent (exported)
	stream.RecordStreamSendmeSent()
	if stream.ShouldSendStreamSendme() {
		t.Error("ShouldSendStreamSendme() = true after recording SENDME sent, want false")
	}
	if got := stream.GetDeliverWindow(); got != 500 { // Should increment by 50
		t.Errorf("After RecordStreamSendmeSent(), deliver window = %d, want 500", got)
	}

	// Test IncrementPackageWindow (exported)
	stream.IncrementPackageWindow()
	if got := stream.GetPackageWindow(); got != 540 { // 490 + 50
		t.Errorf("After IncrementPackageWindow(), package window = %d, want 540", got)
	}
}

// TestStreamFlowControlConcurrentAccess tests that flow control methods
// are safe for concurrent access from circuit layer
func TestStreamFlowControlConcurrentAccess(t *testing.T) {
	log := logger.NewDefault()
	stream := NewStream(1, 100, "example.com", 80, log)

	done := make(chan bool)

	// Concurrent decrements
	go func() {
		for i := 0; i < 50; i++ {
			stream.DecrementPackageWindow()
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 50; i++ {
			stream.DecrementDeliverWindow()
		}
		done <- true
	}()

	// Concurrent increments and checks
	go func() {
		for i := 0; i < 10; i++ {
			stream.IncrementPackageWindow()
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 10; i++ {
			stream.ShouldSendStreamSendme()
			stream.RecordStreamSendmeSent()
		}
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 4; i++ {
		<-done
	}

	// Just verify no panic occurred
	// Final values may vary due to race conditions, but that's OK
	// as long as no panic or deadlock occurred
	t.Logf("Final package window: %d", stream.GetPackageWindow())
	t.Logf("Final deliver window: %d", stream.GetDeliverWindow())
}
