// Package stream provides Tor stream management for multiplexing connections over circuits.
//
// # Flow Control
//
// This package implements stream-level flow control per tor-spec.txt §7.4.
// Both circuit-level and stream-level flow control use a sliding window protocol
// to prevent buffer exhaustion.
//
// Stream-level flow control parameters:
//   - Initial window size: 500 cells
//   - SENDME threshold: 50 cells (send SENDME every 50 DATA cells received)
//   - SENDME increment: 50 cells (each SENDME increases window by 50)
//
// The package window tracks outgoing cells (cells we can send), while the
// deliver window tracks incoming cells (cells we can receive). When either
// window is exhausted, data transmission is blocked until a SENDME is received.
//
// For circuit-level flow control, see pkg/circuit/circuit.go.
package stream

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/opd-ai/go-tor/pkg/circuit"
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
	ID           uint16
	CircuitID    uint32
	Target       string
	Port         uint16
	State        State
	IsolationKey *circuit.IsolationKey // Isolation key for this stream
	CreatedAt    time.Time
	sendQueue    chan []byte
	recvQueue    chan []byte
	closeChan    chan struct{}
	closeOnce    sync.Once
	mu           sync.RWMutex
	logger       *logger.Logger
	// Flow control per tor-spec.txt §7.4
	packageWindow  int // Stream-level package window (cells we can send)
	deliverWindow  int // Stream-level deliver window (cells we can receive)
	sendmeReceived int // Count of DATA cells received (for sending SENDME)
	sendmeSent     int // Count of SENDME cells sent
	// Backpressure state for memory management
	backpressure   *BackpressureState // Optional backpressure controller
	sendBufferSize int                // Current send buffer size in bytes
	recvBufferSize int                // Current recv buffer size in bytes
}

// NewStream creates a new stream
func NewStream(id uint16, circuitID uint32, target string, port uint16, log *logger.Logger) *Stream {
	if log == nil {
		log = logger.NewDefault()
	}

	return &Stream{
		ID:             id,
		CircuitID:      circuitID,
		Target:         target,
		Port:           port,
		State:          StateNew,
		CreatedAt:      time.Now(),
		sendQueue:      make(chan []byte, 32),
		recvQueue:      make(chan []byte, 32),
		closeChan:      make(chan struct{}),
		logger:         log.Component("stream"),
		packageWindow:  500, // tor-spec.txt §7.4: Initial stream window is 500
		deliverWindow:  500, // tor-spec.txt §7.4: Initial stream window is 500
		sendmeReceived: 0,
		sendmeSent:     0,
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

	// Check backpressure before attempting to send
	if s.backpressure != nil {
		s.mu.Lock()
		potentialSize := s.sendBufferSize + len(data)
		isPaused := s.backpressure.CheckSendBuffer(potentialSize)
		s.mu.Unlock()

		if isPaused {
			return fmt.Errorf("send buffer full (backpressure applied)")
		}
	}

	select {
	case s.sendQueue <- data:
		// Successfully queued - now update buffer size
		if s.backpressure != nil {
			s.mu.Lock()
			s.sendBufferSize += len(data)
			s.mu.Unlock()
		}
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
		// Update buffer size and check if backpressure can be released
		if s.backpressure != nil {
			s.mu.Lock()
			s.recvBufferSize -= len(data)
			if s.recvBufferSize < 0 {
				s.recvBufferSize = 0
			}
			s.backpressure.CheckRecvBuffer(s.recvBufferSize)
			s.mu.Unlock()
		}
		return data, nil
	case <-s.closeChan:
		return nil, io.EOF
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// ReceiveData delivers received data to the stream (called by circuit layer)
func (s *Stream) ReceiveData(data []byte) error {
	// Check backpressure before accepting data
	if s.backpressure != nil {
		s.mu.Lock()
		potentialSize := s.recvBufferSize + len(data)
		isPaused := s.backpressure.CheckRecvBuffer(potentialSize)
		s.mu.Unlock()

		if isPaused {
			return fmt.Errorf("receive buffer full (backpressure applied)")
		}
	}

	select {
	case s.recvQueue <- data:
		// Successfully queued - now update buffer size
		if s.backpressure != nil {
			s.mu.Lock()
			s.recvBufferSize += len(data)
			s.mu.Unlock()
		}
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
		// Update buffer size and check if backpressure can be released
		if s.backpressure != nil {
			s.mu.Lock()
			s.sendBufferSize -= len(data)
			if s.sendBufferSize < 0 {
				s.sendBufferSize = 0
			}
			s.backpressure.CheckSendBuffer(s.sendBufferSize)
			s.mu.Unlock()
		}
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

	if err := stream.Close(); err != nil {
		m.logger.Error("Failed to close stream during removal", "function", "RemoveStream", "stream_id", streamID, "error", err)
	}
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
			if err := stream.Close(); err != nil {
				m.logger.Error("Failed to close stream during shutdown", "function", "Close", "stream_id", id, "error", err)
			}
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

// SetIsolationKey sets the isolation key for a stream
func (s *Stream) SetIsolationKey(key *circuit.IsolationKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.IsolationKey = key
}

// GetIsolationKey returns the isolation key for a stream
func (s *Stream) GetIsolationKey() *circuit.IsolationKey {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.IsolationKey
}

// DecrementPackageWindow decrements the stream-level package window
// Returns an error if the window is exhausted
func (s *Stream) DecrementPackageWindow() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.packageWindow <= 0 {
		return fmt.Errorf("stream package window exhausted: cannot send more cells until SENDME received")
	}

	s.packageWindow--
	return nil
}

// IncrementPackageWindow increments the stream-level package window
// This is called when we receive a SENDME cell for this stream
func (s *Stream) IncrementPackageWindow() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Per tor-spec.txt §7.4, each stream SENDME increments the window by 50
	s.packageWindow += 50
	s.sendmeSent++
}

// DecrementDeliverWindow decrements the stream-level deliver window
// Returns an error if the window is exhausted
func (s *Stream) DecrementDeliverWindow() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.deliverWindow <= 0 {
		return fmt.Errorf("stream deliver window exhausted: cannot receive more cells until SENDME sent")
	}

	s.deliverWindow--
	s.sendmeReceived++

	return nil
}

// ShouldSendStreamSendme checks if we should send a stream-level SENDME
// Per tor-spec.txt §7.4, send SENDME every 50 cells received
func (s *Stream) ShouldSendStreamSendme() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.sendmeReceived >= 50
}

// RecordStreamSendmeSent records that a stream-level SENDME was sent
// This resets the received counter and increments the deliver window
func (s *Stream) RecordStreamSendmeSent() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sendmeReceived = 0
	s.deliverWindow += 50 // Increment our deliver window
}

// GetPackageWindow returns the current package window (for testing)
func (s *Stream) GetPackageWindow() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.packageWindow
}

// GetDeliverWindow returns the current deliver window (for testing)
func (s *Stream) GetDeliverWindow() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.deliverWindow
}

// SetBackpressure attaches a backpressure controller to this stream
func (s *Stream) SetBackpressure(bp *BackpressureState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.backpressure = bp
}

// GetBackpressure returns the current backpressure state
func (s *Stream) GetBackpressure() *BackpressureState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.backpressure
}

// GetSendBufferSize returns the current send buffer size in bytes
func (s *Stream) GetSendBufferSize() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sendBufferSize
}

// GetRecvBufferSize returns the current receive buffer size in bytes
func (s *Stream) GetRecvBufferSize() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.recvBufferSize
}
