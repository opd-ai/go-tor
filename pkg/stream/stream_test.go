package stream

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/logger"
)

func TestNewStream(t *testing.T) {
	log := logger.NewDefault()
	stream := NewStream(1, 100, "example.com", 80, log)

	if stream.ID != 1 {
		t.Errorf("Expected stream ID 1, got %d", stream.ID)
	}
	if stream.CircuitID != 100 {
		t.Errorf("Expected circuit ID 100, got %d", stream.CircuitID)
	}
	if stream.Target != "example.com" {
		t.Errorf("Expected target example.com, got %s", stream.Target)
	}
	if stream.Port != 80 {
		t.Errorf("Expected port 80, got %d", stream.Port)
	}
	if stream.State != StateNew {
		t.Errorf("Expected state NEW, got %s", stream.State)
	}
}

func TestStreamStateTransitions(t *testing.T) {
	log := logger.NewDefault()
	stream := NewStream(1, 100, "example.com", 80, log)

	states := []State{StateConnecting, StateConnected, StateClosed}
	for _, state := range states {
		stream.SetState(state)
		if stream.GetState() != state {
			t.Errorf("Expected state %s, got %s", state, stream.GetState())
		}
	}
}

func TestStreamSendReceive(t *testing.T) {
	log := logger.NewDefault()
	stream := NewStream(1, 100, "example.com", 80, log)
	stream.SetState(StateConnected)

	testData := []byte("Hello, Tor!")

	// Test send
	if err := stream.Send(testData); err != nil {
		t.Fatalf("Failed to send data: %v", err)
	}

	// Test receive from send queue (simulating circuit layer)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	data, err := stream.SendData(ctx)
	if err != nil {
		t.Fatalf("Failed to receive from send queue: %v", err)
	}

	if string(data) != string(testData) {
		t.Errorf("Expected data %s, got %s", testData, data)
	}
}

func TestStreamReceiveData(t *testing.T) {
	log := logger.NewDefault()
	stream := NewStream(1, 100, "example.com", 80, log)
	stream.SetState(StateConnected)

	testData := []byte("Data from circuit")

	// Simulate circuit layer delivering data
	if err := stream.ReceiveData(testData); err != nil {
		t.Fatalf("Failed to deliver data: %v", err)
	}

	// Application receives data
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	data, err := stream.Receive(ctx)
	if err != nil {
		t.Fatalf("Failed to receive data: %v", err)
	}

	if string(data) != string(testData) {
		t.Errorf("Expected data %s, got %s", testData, data)
	}
}

func TestStreamSendBeforeConnected(t *testing.T) {
	log := logger.NewDefault()
	stream := NewStream(1, 100, "example.com", 80, log)

	// Try to send before connected
	err := stream.Send([]byte("data"))
	if err == nil {
		t.Error("Expected error when sending on non-connected stream")
	}
}

func TestStreamClose(t *testing.T) {
	log := logger.NewDefault()
	stream := NewStream(1, 100, "example.com", 80, log)
	stream.SetState(StateConnected)

	// Close stream
	if err := stream.Close(); err != nil {
		t.Fatalf("Failed to close stream: %v", err)
	}

	if stream.GetState() != StateClosed {
		t.Errorf("Expected state CLOSED, got %s", stream.GetState())
	}

	// Try to receive after close
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := stream.Receive(ctx)
	if err != io.EOF {
		t.Errorf("Expected EOF after close, got %v", err)
	}
}

func TestNewManager(t *testing.T) {
	log := logger.NewDefault()
	mgr := NewManager(log)

	if mgr == nil {
		t.Fatal("Expected manager to be created")
	}

	if mgr.Count() != 0 {
		t.Errorf("Expected 0 streams, got %d", mgr.Count())
	}
}

func TestManagerCreateStream(t *testing.T) {
	log := logger.NewDefault()
	mgr := NewManager(log)

	stream, err := mgr.CreateStream(100, "example.com", 80)
	if err != nil {
		t.Fatalf("Failed to create stream: %v", err)
	}

	if stream.ID == 0 {
		t.Error("Expected non-zero stream ID")
	}

	if mgr.Count() != 1 {
		t.Errorf("Expected 1 stream, got %d", mgr.Count())
	}
}

func TestManagerGetStream(t *testing.T) {
	log := logger.NewDefault()
	mgr := NewManager(log)

	stream1, err := mgr.CreateStream(100, "example.com", 80)
	if err != nil {
		t.Fatalf("Failed to create stream: %v", err)
	}

	stream2, err := mgr.GetStream(stream1.ID)
	if err != nil {
		t.Fatalf("Failed to get stream: %v", err)
	}

	if stream1.ID != stream2.ID {
		t.Errorf("Expected same stream, got IDs %d and %d", stream1.ID, stream2.ID)
	}
}

func TestManagerGetNonExistentStream(t *testing.T) {
	log := logger.NewDefault()
	mgr := NewManager(log)

	_, err := mgr.GetStream(999)
	if err == nil {
		t.Error("Expected error when getting non-existent stream")
	}
}

func TestManagerRemoveStream(t *testing.T) {
	log := logger.NewDefault()
	mgr := NewManager(log)

	stream, err := mgr.CreateStream(100, "example.com", 80)
	if err != nil {
		t.Fatalf("Failed to create stream: %v", err)
	}

	if err := mgr.RemoveStream(stream.ID); err != nil {
		t.Fatalf("Failed to remove stream: %v", err)
	}

	if mgr.Count() != 0 {
		t.Errorf("Expected 0 streams after removal, got %d", mgr.Count())
	}
}

func TestManagerGetStreamsForCircuit(t *testing.T) {
	log := logger.NewDefault()
	mgr := NewManager(log)

	// Create streams on different circuits
	mgr.CreateStream(100, "example1.com", 80)
	mgr.CreateStream(100, "example2.com", 443)
	mgr.CreateStream(200, "example3.com", 80)

	streams := mgr.GetStreamsForCircuit(100)
	if len(streams) != 2 {
		t.Errorf("Expected 2 streams on circuit 100, got %d", len(streams))
	}

	streams = mgr.GetStreamsForCircuit(200)
	if len(streams) != 1 {
		t.Errorf("Expected 1 stream on circuit 200, got %d", len(streams))
	}
}

func TestManagerClose(t *testing.T) {
	log := logger.NewDefault()
	mgr := NewManager(log)

	// Create some streams
	mgr.CreateStream(100, "example1.com", 80)
	mgr.CreateStream(100, "example2.com", 443)

	// Close manager
	if err := mgr.Close(); err != nil {
		t.Fatalf("Failed to close manager: %v", err)
	}

	// Should not be able to create streams after close
	_, err := mgr.CreateStream(100, "example3.com", 80)
	if err == nil {
		t.Error("Expected error when creating stream after manager closed")
	}
}

func TestManagerConcurrentOperations(t *testing.T) {
	log := logger.NewDefault()
	mgr := NewManager(log)

	// Create streams concurrently
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(n int) {
			_, err := mgr.CreateStream(uint32(n%3), "example.com", 80)
			if err != nil {
				t.Errorf("Failed to create stream: %v", err)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	if mgr.Count() != 10 {
		t.Errorf("Expected 10 streams, got %d", mgr.Count())
	}
}

func TestStateString(t *testing.T) {
	tests := []struct {
		state    State
		expected string
	}{
		{StateNew, "NEW"},
		{StateConnecting, "CONNECTING"},
		{StateConnected, "CONNECTED"},
		{StateClosed, "CLOSED"},
		{StateFailed, "FAILED"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.state.String() != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, tt.state.String())
			}
		})
	}
}

// TestStreamFlowControlWindows tests stream-level flow control window management
func TestStreamFlowControlWindows(t *testing.T) {
	log := logger.NewDefault()
	stream := NewStream(1, 100, "example.com", 80, log)

	// Test initial window values per tor-spec.txt §7.4
	if stream.GetPackageWindow() != 500 {
		t.Errorf("Initial packageWindow = %v, want 500", stream.GetPackageWindow())
	}
	if stream.GetDeliverWindow() != 500 {
		t.Errorf("Initial deliverWindow = %v, want 500", stream.GetDeliverWindow())
	}

	// Test decrementPackageWindow
	err := stream.decrementPackageWindow()
	if err != nil {
		t.Errorf("decrementPackageWindow() error = %v", err)
	}
	if stream.GetPackageWindow() != 499 {
		t.Errorf("packageWindow after decrement = %v, want 499", stream.GetPackageWindow())
	}

	// Test incrementPackageWindow (adds 50 per tor-spec.txt §7.4)
	stream.incrementPackageWindow()
	if stream.GetPackageWindow() != 549 {
		t.Errorf("packageWindow after increment = %v, want 549", stream.GetPackageWindow())
	}

	// Test decrementDeliverWindow
	err = stream.decrementDeliverWindow()
	if err != nil {
		t.Errorf("decrementDeliverWindow() error = %v", err)
	}
	if stream.GetDeliverWindow() != 499 {
		t.Errorf("deliverWindow after decrement = %v, want 499", stream.GetDeliverWindow())
	}

	// Test window exhaustion
	stream.packageWindow = 0
	err = stream.decrementPackageWindow()
	if err == nil {
		t.Error("decrementPackageWindow() should error when window exhausted")
	}

	stream.deliverWindow = 0
	err = stream.decrementDeliverWindow()
	if err == nil {
		t.Error("decrementDeliverWindow() should error when window exhausted")
	}
}

// TestStreamShouldSendSendme tests shouldSendStreamSendme
func TestStreamShouldSendSendme(t *testing.T) {
	log := logger.NewDefault()
	stream := NewStream(1, 100, "example.com", 80, log)

	// Initially should not need SENDME
	if stream.shouldSendStreamSendme() {
		t.Error("shouldSendStreamSendme() = true initially, want false")
	}

	// Decrement deliver window by 50 cells
	for i := 0; i < 50; i++ {
		stream.decrementDeliverWindow()
	}

	// Should now need SENDME
	if !stream.shouldSendStreamSendme() {
		t.Error("shouldSendStreamSendme() = false after 50 cells, want true")
	}
}

// TestStreamSendmePrepare tests SendmePrepare
func TestStreamSendmePrepare(t *testing.T) {
	log := logger.NewDefault()
	stream := NewStream(1, 100, "example.com", 80, log)

	// Simulate receiving 50 DATA cells
	for i := 0; i < 50; i++ {
		stream.decrementDeliverWindow()
	}

	initialDeliverWindow := stream.GetDeliverWindow()

	// Prepare SENDME
	data := stream.SendmePrepare()

	// Verify SENDME data is empty per tor-spec.txt §7.4
	if len(data) != 0 {
		t.Errorf("SendmePrepare() returned non-empty data: %v", data)
	}

	// Verify sendmeReceived counter was reset
	if stream.sendmeReceived != 0 {
		t.Errorf("sendmeReceived = %v after SendmePrepare, want 0", stream.sendmeReceived)
	}

	// Verify sendmeSent counter was incremented
	if stream.sendmeSent != 1 {
		t.Errorf("sendmeSent = %v after SendmePrepare, want 1", stream.sendmeSent)
	}

	// Verify deliver window was incremented by 50
	expectedWindow := initialDeliverWindow + 50
	if stream.GetDeliverWindow() != expectedWindow {
		t.Errorf("deliverWindow = %v after SendmePrepare, want %v", stream.GetDeliverWindow(), expectedWindow)
	}
}

// TestManagerFlowControlInterface tests that Manager implements StreamFlowController
func TestManagerFlowControlInterface(t *testing.T) {
	log := logger.NewDefault()
	mgr := NewManager(log)

	// Create a stream
	stream, err := mgr.CreateStream(100, "example.com", 80)
	if err != nil {
		t.Fatalf("CreateStream() error = %v", err)
	}

	// Test DecrementDeliverWindow
	err = mgr.DecrementDeliverWindow(stream.ID)
	if err != nil {
		t.Errorf("DecrementDeliverWindow() error = %v", err)
	}

	// Test ShouldSendStreamSendme
	for i := 0; i < 49; i++ {
		mgr.DecrementDeliverWindow(stream.ID)
	}
	if !mgr.ShouldSendStreamSendme(stream.ID) {
		t.Error("ShouldSendStreamSendme() = false after 50 cells, want true")
	}

	// Test SendmePrepare
	data := mgr.SendmePrepare(stream.ID)
	if len(data) != 0 {
		t.Errorf("SendmePrepare() returned non-empty data: %v", data)
	}

	// Test IncrementPackageWindow
	initialWindow := stream.GetPackageWindow()
	mgr.IncrementPackageWindow(stream.ID)
	if stream.GetPackageWindow() != initialWindow+50 {
		t.Errorf("packageWindow = %v after IncrementPackageWindow, want %v", stream.GetPackageWindow(), initialWindow+50)
	}

	// Test DecrementPackageWindow
	err = mgr.DecrementPackageWindow(stream.ID)
	if err != nil {
		t.Errorf("DecrementPackageWindow() error = %v", err)
	}

	// Test with non-existent stream
	err = mgr.DecrementDeliverWindow(9999)
	if err == nil {
		t.Error("DecrementDeliverWindow() should error for non-existent stream")
	}

	// Clean up
	mgr.Close()
}
