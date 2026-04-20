// Package stream provides stress tests for stream multiplexing
// under concurrent load conditions.
//
// These tests verify that the stream manager correctly handles
// concurrent creation, use, and teardown of multiple streams
// within the same circuit, as required for Tor's multiplexed
// stream model (tor-spec.txt §6).
//
// Compliance: tor-spec.txt §6 (Stream Multiplexing)
package stream

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/logger"
)

// TestManagerConcurrentStreamCreation verifies that multiple
// goroutines can create streams concurrently without races.
func TestManagerConcurrentStreamCreation(t *testing.T) {
	log := logger.NewDefault()
	mgr := NewManager(log)
	defer mgr.Close()

	const numGoroutines = 20
	const streamsPerGoroutine = 5
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(circuitID uint32) {
			defer wg.Done()
			for j := 0; j < streamsPerGoroutine; j++ {
				_, err := mgr.CreateStream(circuitID, "example.com", 80)
				if err != nil {
					t.Errorf("CreateStream: %v", err)
				}
			}
		}(uint32(i + 1))
	}

	wg.Wait()

	count := mgr.Count()
	expected := numGoroutines * streamsPerGoroutine
	if count != expected {
		t.Errorf("stream count = %d, want %d", count, expected)
	}
}

// TestManagerConcurrentCreateRemove verifies that concurrent
// creates and removes don't corrupt the manager state.
func TestManagerConcurrentCreateRemove(t *testing.T) {
	log := logger.NewDefault()
	mgr := NewManager(log)
	defer mgr.Close()

	var wg sync.WaitGroup
	streamIDs := make(chan uint16, 50)

	// Create streams
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				s, err := mgr.CreateStream(1, "example.com", 443)
				if err != nil {
					t.Errorf("CreateStream: %v", err)
					return
				}
				streamIDs <- s.ID
			}
		}()
	}

	// Remove streams concurrently
	wg.Add(1)
	go func() {
		defer wg.Done()
		removed := 0
		for id := range streamIDs {
			if err := mgr.RemoveStream(id); err != nil {
				t.Errorf("RemoveStream(%d): %v", id, err)
			}
			removed++
			if removed >= 50 {
				return
			}
		}
	}()

	wg.Wait()
	close(streamIDs)
}

// TestManagerGetStreamsForCircuitMux verifies that circuit-scoped
// stream queries work correctly.
func TestManagerGetStreamsForCircuitMux(t *testing.T) {
	log := logger.NewDefault()
	mgr := NewManager(log)
	defer mgr.Close()

	// Create streams on different circuits
	for i := 0; i < 5; i++ {
		mgr.CreateStream(1, "example.com", 80)
	}
	for i := 0; i < 3; i++ {
		mgr.CreateStream(2, "other.com", 443)
	}

	circuit1Streams := mgr.GetStreamsForCircuit(1)
	circuit2Streams := mgr.GetStreamsForCircuit(2)
	circuit3Streams := mgr.GetStreamsForCircuit(3) // Nonexistent

	if len(circuit1Streams) != 5 {
		t.Errorf("circuit 1 streams = %d, want 5", len(circuit1Streams))
	}
	if len(circuit2Streams) != 3 {
		t.Errorf("circuit 2 streams = %d, want 3", len(circuit2Streams))
	}
	if len(circuit3Streams) != 0 {
		t.Errorf("circuit 3 streams = %d, want 0", len(circuit3Streams))
	}
}

// TestStreamSendReceiveDataConcurrent verifies that sending and
// receiving data on a stream works concurrently.
func TestStreamSendReceiveDataConcurrent(t *testing.T) {
	log := logger.NewDefault()
	s := NewStream(1, 100, "example.com", 80, log)

	var wg sync.WaitGroup

	// Writer: send multiple data chunks
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			data := []byte("test data chunk")
			if err := s.ReceiveData(data); err != nil {
				// May fail if stream closes, that's OK
				return
			}
		}
	}()

	// Reader: consume data chunks with timeout
	wg.Add(1)
	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		for i := 0; i < 10; i++ {
			_, err := s.Receive(ctx)
			if err != nil {
				return
			}
		}
	}()

	wg.Wait()
	s.Close()
}

// TestStreamStateTransitionsMux verifies stream state machine
// transitions.
func TestStreamStateTransitionsMux(t *testing.T) {
	log := logger.NewDefault()
	s := NewStream(1, 100, "example.com", 80, log)

	// Initial state
	if s.GetState() != StateNew {
		t.Errorf("initial state = %v, want Init", s.GetState())
	}

	// Transition to Connected
	s.SetState(StateConnected)
	if s.GetState() != StateConnected {
		t.Errorf("state = %v, want Connected", s.GetState())
	}

	// Close
	s.Close()
	if s.GetState() != StateClosed {
		t.Errorf("state after close = %v, want Closed", s.GetState())
	}
}

// TestStreamStateString verifies human-readable state names.
func TestStreamStateString(t *testing.T) {
	states := []State{StateNew, StateConnected, StateClosed}
	for _, state := range states {
		s := state.String()
		if s == "" {
			t.Errorf("State(%d).String() is empty", state)
		}
	}
}

// TestManagerCloseAllStreams verifies that closing the manager
// closes all open streams.
func TestManagerCloseAllStreams(t *testing.T) {
	log := logger.NewDefault()
	mgr := NewManager(log)

	// Create several streams
	streams := make([]*Stream, 5)
	for i := 0; i < 5; i++ {
		s, err := mgr.CreateStream(1, "example.com", 80)
		if err != nil {
			t.Fatal(err)
		}
		streams[i] = s
	}

	// Close manager
	if err := mgr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// All streams should be closed
	for i, s := range streams {
		if s.GetState() != StateClosed {
			t.Errorf("stream %d: state = %v, want Closed", i, s.GetState())
		}
	}

	// Count should be 0
	if mgr.Count() != 0 {
		t.Errorf("count = %d after close, want 0", mgr.Count())
	}
}

// TestManagerGetNonexistentStream verifies getting a stream
// that doesn't exist.
func TestManagerGetNonexistentStream(t *testing.T) {
	log := logger.NewDefault()
	mgr := NewManager(log)
	defer mgr.Close()

	_, err := mgr.GetStream(9999)
	if err == nil {
		t.Error("expected error for nonexistent stream")
	}
}

// TestManagerRemoveNonexistentStream verifies removing a stream
// that doesn't exist.
func TestManagerRemoveNonexistentStream(t *testing.T) {
	log := logger.NewDefault()
	mgr := NewManager(log)
	defer mgr.Close()

	err := mgr.RemoveStream(9999)
	if err == nil {
		t.Error("expected error for nonexistent stream")
	}
}

// TestStreamSendOnClosedStream verifies that sending data
// on a closed stream fails.
func TestStreamSendOnClosedStream(t *testing.T) {
	log := logger.NewDefault()
	s := NewStream(1, 100, "example.com", 80, log)
	s.Close()

	err := s.Send([]byte("data"))
	if err == nil {
		t.Error("expected error sending on closed stream")
	}
}

// TestStreamDoubleClose verifies that closing a stream twice
// is safe.
func TestStreamDoubleClose(t *testing.T) {
	log := logger.NewDefault()
	s := NewStream(1, 100, "example.com", 80, log)

	if err := s.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}

	// Second close should be safe
	err := s.Close()
	if err != nil {
		// Some implementations return an error, some don't
		t.Logf("second close returned: %v (acceptable)", err)
	}
}

// TestStreamFlowControlWindows verifies flow control window
// operations.
func TestStreamFlowControlWindows(t *testing.T) {
	log := logger.NewDefault()
	s := NewStream(1, 100, "example.com", 80, log)

	// Decrement should work initially
	for i := 0; i < 5; i++ {
		if err := s.DecrementPackageWindow(); err != nil {
			t.Fatalf("decrement %d: %v", i, err)
		}
	}

	// Increment after decrement
	s.IncrementPackageWindow()
}

// TestManagerNilLogger verifies manager creation with nil logger.
func TestManagerNilLogger(t *testing.T) {
	mgr := NewManager(nil)
	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}
	mgr.Close()
}
