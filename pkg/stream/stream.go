// Package stream provides Tor stream management for multiplexing connections over circuits.
package stream

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/opd-ai/go-tor/pkg/logger"
)

// State represents the current state of a stream
type State int

const (
	// StateNew indicates the stream is newly created
	StateNew State = iota
	// StateConnecting indicates the stream is connecting
	StateConnecting
	// StateConnected indicates the stream is connected and ready
	StateConnected
	// StateClosed indicates the stream has been closed
	StateClosed
	// StateFailed indicates the stream failed
	StateFailed
)

// String returns a string representation of the state
func (s State) String() string {
	switch s {
	case StateNew:
		return "NEW"
	case StateConnecting:
		return "CONNECTING"
	case StateConnected:
		return "CONNECTED"
	case StateClosed:
		return "CLOSED"
	case StateFailed:
		return "FAILED"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", s)
	}
}

// Stream represents a single connection multiplexed over a circuit
type Stream struct {
	ID        uint16
	CircuitID uint32
	Target    string
	Port      uint16
	State     State
	CreatedAt time.Time
	sendQueue chan []byte
	recvQueue chan []byte
	closeChan chan struct{}
	closeOnce sync.Once
	mu        sync.RWMutex
	logger    *logger.Logger
}

// NewStream creates a new stream
func NewStream(id uint16, circuitID uint32, target string, port uint16, log *logger.Logger) *Stream {
	if log == nil {
		log = logger.NewDefault()
	}

	return &Stream{
		ID:        id,
		CircuitID: circuitID,
		Target:    target,
		Port:      port,
		State:     StateNew,
		CreatedAt: time.Now(),
		sendQueue: make(chan []byte, 32),
		recvQueue: make(chan []byte, 32),
		closeChan: make(chan struct{}),
		logger:    log.Component("stream"),
	}
}

// SetState updates the stream state
func (s *Stream) SetState(state State) {
	s.mu.Lock()
	defer s.mu.Unlock()
	oldState := s.State
	s.State = state
	s.logger.Debug("Stream state transition",
		"stream_id", s.ID,
		"old_state", oldState,
		"new_state", state)
}

// GetState returns the current stream state
func (s *Stream) GetState() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.State
}

// Send queues data to be sent on the stream
func (s *Stream) Send(data []byte) error {
	if s.GetState() != StateConnected {
		return fmt.Errorf("stream not connected: state=%s", s.GetState())
	}

	select {
	case s.sendQueue <- data:
		return nil
	case <-s.closeChan:
		return io.EOF
	default:
		return fmt.Errorf("send queue full")
	}
}

// Receive reads data from the stream
func (s *Stream) Receive(ctx context.Context) ([]byte, error) {
	select {
	case data := <-s.recvQueue:
		return data, nil
	case <-s.closeChan:
		return nil, io.EOF
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// ReceiveData delivers received data to the stream (called by circuit layer)
func (s *Stream) ReceiveData(data []byte) error {
	select {
	case s.recvQueue <- data:
		return nil
	case <-s.closeChan:
		return io.EOF
	default:
		return fmt.Errorf("receive queue full")
	}
}

// SendData retrieves data to be sent (called by circuit layer)
func (s *Stream) SendData(ctx context.Context) ([]byte, error) {
	select {
	case data := <-s.sendQueue:
		return data, nil
	case <-s.closeChan:
		return nil, io.EOF
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Close closes the stream
func (s *Stream) Close() error {
	s.closeOnce.Do(func() {
		close(s.closeChan)
		s.SetState(StateClosed)
		s.logger.Info("Stream closed",
			"stream_id", s.ID,
			"circuit_id", s.CircuitID)
	})
	return nil
}

// Manager manages multiple streams across circuits
type Manager struct {
	streams   map[uint16]*Stream
	nextID    uint16
	mu        sync.RWMutex
	logger    *logger.Logger
	closeChan chan struct{}
	closeOnce sync.Once
}

// NewManager creates a new stream manager
func NewManager(log *logger.Logger) *Manager {
	if log == nil {
		log = logger.NewDefault()
	}

	return &Manager{
		streams:   make(map[uint16]*Stream),
		nextID:    1,
		logger:    log.Component("stream-manager"),
		closeChan: make(chan struct{}),
	}
}

// CreateStream creates a new stream for a target
func (m *Manager) CreateStream(circuitID uint32, target string, port uint16) (*Stream, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	select {
	case <-m.closeChan:
		return nil, fmt.Errorf("manager closed")
	default:
	}

	// Allocate stream ID
	streamID := m.nextID
	m.nextID++
	if m.nextID == 0 {
		m.nextID = 1 // Skip 0
	}

	stream := NewStream(streamID, circuitID, target, port, m.logger)
	m.streams[streamID] = stream

	m.logger.Info("Stream created",
		"stream_id", streamID,
		"circuit_id", circuitID,
		"target", target,
		"port", port)

	return stream, nil
}

// GetStream retrieves a stream by ID
func (m *Manager) GetStream(streamID uint16) (*Stream, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stream, exists := m.streams[streamID]
	if !exists {
		return nil, fmt.Errorf("stream not found: %d", streamID)
	}

	return stream, nil
}

// RemoveStream removes a stream from management
func (m *Manager) RemoveStream(streamID uint16) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	stream, exists := m.streams[streamID]
	if !exists {
		return fmt.Errorf("stream not found: %d", streamID)
	}

	stream.Close()
	delete(m.streams, streamID)

	m.logger.Info("Stream removed", "stream_id", streamID)

	return nil
}

// GetStreamsForCircuit returns all streams on a circuit
func (m *Manager) GetStreamsForCircuit(circuitID uint32) []*Stream {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var streams []*Stream
	for _, stream := range m.streams {
		if stream.CircuitID == circuitID {
			streams = append(streams, stream)
		}
	}

	return streams
}

// Close closes all streams and the manager
func (m *Manager) Close() error {
	m.closeOnce.Do(func() {
		close(m.closeChan)

		m.mu.Lock()
		defer m.mu.Unlock()

		for id, stream := range m.streams {
			// Best-effort close during shutdown - errors are logged by the stream itself
			stream.Close() // nolint:errcheck
			delete(m.streams, id)
		}

		m.logger.Info("Stream manager closed")
	})

	return nil
}

// Count returns the number of active streams
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.streams)
}
