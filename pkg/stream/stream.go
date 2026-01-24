// Package stream provides Tor stream management for multiplexing connections over circuits.
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
	// Flow control per tor-spec.txt §7.4 (stream-level windows)
	packageWindow  int // Stream-level package window (cells we can send)
	deliverWindow  int // Stream-level deliver window (cells we can receive)
	sendmeReceived int // Count of DATA cells received (for sending SENDME)
	sendmeSent     int // Count of SENDME cells sent
}

// NewStream creates a new stream
func NewStream(id uint16, circuitID uint32, target string, port uint16, log *logger.Logger) *Stream {
	if log == nil {
		log = logger.NewDefault()
	}

	return &Stream{
		ID:            id,
		CircuitID:     circuitID,
		Target:        target,
		Port:          port,
		State:         StateNew,
		CreatedAt:     time.Now(),
		sendQueue:     make(chan []byte, 32),
		recvQueue:     make(chan []byte, 32),
		closeChan:     make(chan struct{}),
		logger:        log.Component("stream"),
		packageWindow: 500, // tor-spec.txt §7.4: Initial stream window is 500
		deliverWindow: 500, // tor-spec.txt §7.4: Initial stream window is 500
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

// DecrementDeliverWindow decrements the stream's deliver window for received DATA cells
// Implements circuit.StreamFlowController interface
func (m *Manager) DecrementDeliverWindow(streamID uint16) error {
	stream, err := m.GetStream(streamID)
	if err != nil {
		return err
	}
	return stream.decrementDeliverWindow()
}

// ShouldSendStreamSendme checks if we should send a stream-level SENDME
// Implements circuit.StreamFlowController interface
func (m *Manager) ShouldSendStreamSendme(streamID uint16) bool {
	stream, err := m.GetStream(streamID)
	if err != nil {
		return false
	}
	return stream.shouldSendStreamSendme()
}

// SendmePrepare prepares to send a stream-level SENDME
// Implements circuit.StreamFlowController interface
func (m *Manager) SendmePrepare(streamID uint16) []byte {
	stream, err := m.GetStream(streamID)
	if err != nil {
		return []byte{}
	}
	return stream.SendmePrepare()
}

// IncrementPackageWindow increments the stream's package window when SENDME received
// Implements circuit.StreamFlowController interface
func (m *Manager) IncrementPackageWindow(streamID uint16) {
	stream, err := m.GetStream(streamID)
	if err != nil {
		return
	}
	stream.incrementPackageWindow()
}

// DecrementPackageWindow decrements the stream's package window before sending DATA cells
// Implements circuit.StreamFlowController interface
func (m *Manager) DecrementPackageWindow(streamID uint16) error {
	stream, err := m.GetStream(streamID)
	if err != nil {
		return err
	}
	return stream.decrementPackageWindow()
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

// decrementPackageWindow decrements the stream-level package window
// Returns an error if the window is exhausted
func (s *Stream) decrementPackageWindow() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.packageWindow <= 0 {
		return fmt.Errorf("stream %d package window exhausted: cannot send more cells until SENDME received", s.ID)
	}

	s.packageWindow--
	return nil
}

// incrementPackageWindow increments the stream-level package window
// This is called when we receive a stream-level SENDME cell
func (s *Stream) incrementPackageWindow() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Per tor-spec.txt §7.4, each SENDME increments the window by 50
	s.packageWindow += 50
}

// decrementDeliverWindow decrements the stream-level deliver window
// Returns an error if the window is exhausted
func (s *Stream) decrementDeliverWindow() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.deliverWindow <= 0 {
		return fmt.Errorf("stream %d deliver window exhausted: cannot receive more cells until SENDME sent", s.ID)
	}

	s.deliverWindow--
	s.sendmeReceived++

	return nil
}

// shouldSendStreamSendme checks if we should send a stream-level SENDME
// Per tor-spec.txt §7.4, send SENDME every 50 cells received
func (s *Stream) shouldSendStreamSendme() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.sendmeReceived >= 50
}

// SendmePrepare prepares to send a stream-level SENDME and returns the data
// This is called by the circuit layer to build the SENDME cell
func (s *Stream) SendmePrepare() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sendmeReceived = 0
	s.sendmeSent++
	s.deliverWindow += 50 // Increment our deliver window

	// Per tor-spec.txt §7.4, stream-level SENDME has empty payload
	return []byte{}
}

// GetPackageWindow returns the current package window (for testing/debugging)
func (s *Stream) GetPackageWindow() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.packageWindow
}

// GetDeliverWindow returns the current deliver window (for testing/debugging)
func (s *Stream) GetDeliverWindow() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.deliverWindow
}
