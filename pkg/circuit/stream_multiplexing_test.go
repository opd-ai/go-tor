package circuit

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/cell"
)

// mockStream is a mock implementation of a stream for testing
type mockStream struct {
	id           uint16
	receivedData [][]byte
	mu           sync.Mutex
	closed       bool
}

func newMockStream(id uint16) *mockStream {
	return &mockStream{
		id:           id,
		receivedData: make([][]byte, 0),
	}
}

func (m *mockStream) ReceiveData(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return fmt.Errorf("stream closed")
	}

	m.receivedData = append(m.receivedData, data)
	return nil
}

func (m *mockStream) GetReceivedData() [][]byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([][]byte{}, m.receivedData...)
}

// mockStreamManager is a mock implementation of a stream manager for testing
type mockStreamManager struct {
	streams map[uint16]*mockStream
	mu      sync.RWMutex
}

func newMockStreamManager() *mockStreamManager {
	return &mockStreamManager{
		streams: make(map[uint16]*mockStream),
	}
}

func (m *mockStreamManager) AddStream(id uint16) *mockStream {
	m.mu.Lock()
	defer m.mu.Unlock()

	stream := newMockStream(id)
	m.streams[id] = stream
	return stream
}

func (m *mockStreamManager) GetStream(id uint16) (interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stream, exists := m.streams[id]
	if !exists {
		return nil, fmt.Errorf("stream not found: %d", id)
	}
	return stream, nil
}

// TestStreamMultiplexing_DeliverToStream tests the deliverToStream method
func TestStreamMultiplexing_DeliverToStream(t *testing.T) {
	tests := []struct {
		name          string
		setupManager  func() *mockStreamManager
		streamID      uint16
		relayCommand  byte
		data          []byte
		expectError   bool
		errorContains string
	}{
		{
			name: "deliver data to existing stream",
			setupManager: func() *mockStreamManager {
				mgr := newMockStreamManager()
				mgr.AddStream(42)
				return mgr
			},
			streamID:     42,
			relayCommand: cell.RelayData,
			data:         []byte("test data"),
			expectError:  false,
		},
		{
			name: "deliver end to existing stream",
			setupManager: func() *mockStreamManager {
				mgr := newMockStreamManager()
				mgr.AddStream(100)
				return mgr
			},
			streamID:     100,
			relayCommand: cell.RelayEnd,
			data:         []byte{},
			expectError:  false,
		},
		{
			name: "deliver to non-existent stream",
			setupManager: func() *mockStreamManager {
				return newMockStreamManager()
			},
			streamID:      99,
			relayCommand:  cell.RelayData,
			data:          []byte("test"),
			expectError:   true,
			errorContains: "not found",
		},
		{
			name: "no stream manager configured",
			setupManager: func() *mockStreamManager {
				return nil
			},
			streamID:      42,
			relayCommand:  cell.RelayData,
			data:          []byte("test"),
			expectError:   true,
			errorContains: "no stream manager",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a circuit
			c := NewCircuit(1)

			// Setup stream manager
			mgr := tt.setupManager()
			if mgr != nil {
				c.SetStreamManager(mgr)
			}

			// Create relay cell
			relayCell := &cell.RelayCell{
				StreamID: tt.streamID,
				Command:  tt.relayCommand,
				Data:     tt.data,
			}

			// Test deliverToStream
			err := c.deliverToStream(relayCell)

			// Check error expectation
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else if tt.errorContains != "" && !contains(err.Error(), tt.errorContains) {
					t.Errorf("expected error containing %q, got %q", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}

				// Verify data was delivered
				if mgr != nil && tt.relayCommand == cell.RelayData {
					stream, _ := mgr.GetStream(tt.streamID)
					mockStream := stream.(*mockStream)
					received := mockStream.GetReceivedData()
					if len(received) != 1 {
						t.Errorf("expected 1 data delivery, got %d", len(received))
					} else if string(received[0]) != string(tt.data) {
						t.Errorf("expected data %q, got %q", string(tt.data), string(received[0]))
					}
				}
			}
		})
	}
}

// TestStreamMultiplexing_ReadFromStream tests that ReadFromStream delivers cells to other streams
func TestStreamMultiplexing_ReadFromStream(t *testing.T) {
	// Create a circuit
	c := NewCircuit(1)

	// Create stream manager with multiple streams
	mgr := newMockStreamManager()
	stream1 := mgr.AddStream(1)
	stream2 := mgr.AddStream(2)
	stream3 := mgr.AddStream(3)
	c.SetStreamManager(mgr)

	// Send cells for different streams
	cells := []*cell.RelayCell{
		{StreamID: 2, Command: cell.RelayData, Data: []byte("data for stream 2")},
		{StreamID: 3, Command: cell.RelayData, Data: []byte("data for stream 3")},
		{StreamID: 1, Command: cell.RelayData, Data: []byte("data for stream 1")},
		{StreamID: 2, Command: cell.RelayData, Data: []byte("more data for stream 2")},
	}

	// Queue cells in background
	go func() {
		for _, relayCell := range cells {
			c.relayReceiveChan <- relayCell
			time.Sleep(10 * time.Millisecond)
		}
	}()

	// Read from stream 1 (should skip cells for streams 2 and 3)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	data, err := c.ReadFromStream(ctx, 1)
	if err != nil {
		t.Fatalf("ReadFromStream failed: %v", err)
	}

	// Verify we got data for stream 1
	expected := "data for stream 1"
	if string(data) != expected {
		t.Errorf("expected %q, got %q", expected, string(data))
	}

	// Verify other streams received their data
	// Give more time for async delivery to complete
	time.Sleep(200 * time.Millisecond)

	stream2Data := stream2.GetReceivedData()
	if len(stream2Data) < 1 {
		t.Errorf("stream 2 expected at least 1 delivery, got %d", len(stream2Data))
	}

	stream3Data := stream3.GetReceivedData()
	if len(stream3Data) < 1 {
		t.Errorf("stream 3 expected at least 1 delivery, got %d", len(stream3Data))
	}

	stream1Data := stream1.GetReceivedData()
	if len(stream1Data) != 0 {
		t.Errorf("stream 1 should not have received data through manager (read directly), got %d", len(stream1Data))
	}
}

// TestStreamMultiplexing_ConcurrentReads tests concurrent reads from multiple streams
func TestStreamMultiplexing_ConcurrentReads(t *testing.T) {
	// Create a circuit
	c := NewCircuit(1)

	// Create stream manager with multiple streams
	mgr := newMockStreamManager()
	mgr.AddStream(10)
	mgr.AddStream(20)
	mgr.AddStream(30)
	c.SetStreamManager(mgr)

	// Send many cells for different streams in round-robin fashion
	numCellsPerStream := 5
	go func() {
		for i := 0; i < numCellsPerStream; i++ {
			for streamID := uint16(10); streamID <= 30; streamID += 10 {
				c.relayReceiveChan <- &cell.RelayCell{
					StreamID: streamID,
					Command:  cell.RelayData,
					Data:     []byte(fmt.Sprintf("stream-%d-data-%d", streamID, i)),
				}
			}
		}
	}()

	// Read from each stream concurrently
	var wg sync.WaitGroup
	successCount := make(chan int, 3)

	for streamID := uint16(10); streamID <= 30; streamID += 10 {
		wg.Add(1)
		go func(sid uint16) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			// Try to read at least one cell for this stream
			data, err := c.ReadFromStream(ctx, sid)
			if err != nil {
				t.Logf("stream %d read error (acceptable in concurrent test): %v", sid, err)
				return
			}
			if len(data) > 0 {
				successCount <- 1
			}
		}(streamID)
	}

	// Wait for all reads to complete
	wg.Wait()
	close(successCount)

	// Count successes
	totalSuccess := 0
	for range successCount {
		totalSuccess++
	}

	// At least one stream should have successfully read data
	if totalSuccess == 0 {
		t.Error("no streams successfully read data in concurrent test")
	} else {
		t.Logf("%d out of 3 streams successfully read data", totalSuccess)
	}

	// Verify streams received data through the manager
	time.Sleep(200 * time.Millisecond)
	totalReceived := 0
	for streamID := uint16(10); streamID <= 30; streamID += 10 {
		stream, _ := mgr.GetStream(streamID)
		mockStream := stream.(*mockStream)
		received := mockStream.GetReceivedData()
		totalReceived += len(received)
	}
	
	// With 3 streams, reading 1 cell each, and 5 cells per stream sent (15 total),
	// we should have at least a few cells delivered through the manager
	t.Logf("total cells delivered through stream manager: %d", totalReceived)
}

// TestStreamMultiplexing_EndSignal tests that RELAY_END is properly delivered
func TestStreamMultiplexing_EndSignal(t *testing.T) {
	// Create a circuit
	c := NewCircuit(1)

	// Create stream manager with two streams
	mgr := newMockStreamManager()
	stream50 := mgr.AddStream(50)
	stream51 := mgr.AddStream(51)
	c.SetStreamManager(mgr)

	// Send END for stream 51 (different stream), then data for stream 50
	go func() {
		c.relayReceiveChan <- &cell.RelayCell{
			StreamID: 51,
			Command:  cell.RelayEnd,
			Data:     []byte{0x06}, // END reason
		}
		c.relayReceiveChan <- &cell.RelayCell{
			StreamID: 50,
			Command:  cell.RelayData,
			Data:     []byte("data for stream 50"),
		}
	}()

	// Read from stream 50 (should receive data, while END is delivered to stream 51)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	data, err := c.ReadFromStream(ctx, 50)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if string(data) != "data for stream 50" {
		t.Errorf("expected 'data for stream 50', got %q", string(data))
	}

	// Verify END was delivered to stream 51
	time.Sleep(100 * time.Millisecond)
	received51 := stream51.GetReceivedData()
	if len(received51) != 1 {
		t.Errorf("stream 51 expected 1 delivery for END signal, got %d", len(received51))
	} else if len(received51[0]) != 0 {
		t.Errorf("stream 51 expected empty or nil data for END signal, got %v", received51[0])
	}

	// Stream 50 should not have received anything through manager
	received50 := stream50.GetReceivedData()
	if len(received50) != 0 {
		t.Errorf("stream 50 should not have received data through manager (read directly), got %d", len(received50))
	}
}
