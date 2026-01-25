// Package onion - Stream Handling for Onion Services
// This file implements stream management for onion service hosting (Task 9.3.1)
// Following tor-spec.txt §6 for RELAY_BEGIN/DATA/END protocol
package onion

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/opd-ai/go-tor/pkg/cell"
	"github.com/opd-ai/go-tor/pkg/logger"
)

// ServiceStream represents an active stream connection from client to service
type ServiceStream struct {
	StreamID   uint16
	CircuitID  uint32
	TargetPort int
	TargetAddr string
	conn       net.Conn // Connection to local service backend
	circuit    CircuitInterface
	logger     *logger.Logger
	metrics    MetricsCollector
	ctx        context.Context
	cancel     context.CancelFunc
	closeOnce  sync.Once
	mu         sync.RWMutex
	closed     bool
}

// ServiceStreamManager manages streams for an onion service
type ServiceStreamManager struct {
	service *Service
	streams map[uint16]*ServiceStream
	mu      sync.RWMutex
	logger  *logger.Logger
	metrics MetricsCollector
}

// NewServiceStreamManager creates a new stream manager for a service
func NewServiceStreamManager(service *Service, log *logger.Logger) *ServiceStreamManager {
	if log == nil {
		log = logger.NewDefault()
	}

	var metrics MetricsCollector
	if service != nil && service.config != nil {
		metrics = service.config.Metrics
	}

	return &ServiceStreamManager{
		service: service,
		streams: make(map[uint16]*ServiceStream),
		logger:  log.Component("service-stream-manager"),
		metrics: metrics,
	}
}

// HandleRelayBegin handles an incoming RELAY_BEGIN cell from a client
//
// RELAY_BEGIN cell format (tor-spec.txt §6.2):
//
//	ADDRPORT [nul-terminated string]
//	FLAGS [4 bytes]
//
// ADDRPORT format: "host:port" or just "host" for implicit port 0
//
// Returns error if the stream cannot be established
func (sm *ServiceStreamManager) HandleRelayBegin(circuitID uint32, streamID uint16, data []byte, circuit CircuitInterface) error {
	sm.logger.Info("Received RELAY_BEGIN",
		"circuit_id", circuitID,
		"stream_id", streamID,
		"data_len", len(data))

	// Parse ADDRPORT (nul-terminated string)
	addrPortEnd := 0
	for i, b := range data {
		if b == 0 {
			addrPortEnd = i
			break
		}
	}

	if addrPortEnd == 0 {
		sm.logger.Warn("Invalid RELAY_BEGIN: no null terminator")
		return sm.sendRelayEnd(circuit, circuitID, streamID, cell.EndReasonProtocol)
	}

	addrPort := string(data[:addrPortEnd])
	sm.logger.Debug("RELAY_BEGIN address", "addr", addrPort)

	// Parse "host:port" or just "host"
	_, targetPort, err := parseAddrPort(addrPort)
	if err != nil {
		sm.logger.Warn("Invalid RELAY_BEGIN address format",
			"addr", addrPort,
			"error", err)
		return sm.sendRelayEnd(circuit, circuitID, streamID, cell.EndReasonProtocol)
	}

	// Map virtual port to local service target
	targetAddr, ok := sm.service.config.Ports[targetPort]
	if !ok {
		sm.logger.Warn("No service configured for port",
			"port", targetPort,
			"configured_ports", len(sm.service.config.Ports))
		return sm.sendRelayEnd(circuit, circuitID, streamID, cell.EndReasonExitPolicy)
	}

	sm.logger.Info("Mapping stream to local service",
		"stream_id", streamID,
		"virtual_port", targetPort,
		"target", targetAddr)

	// Connect to local service backend
	conn, err := sm.connectToBackend(targetAddr)
	if err != nil {
		sm.logger.Error("Failed to connect to backend",
			"target", targetAddr,
			"error", err)
		return sm.sendRelayEnd(circuit, circuitID, streamID, cell.EndReasonConnRefused)
	}

	// Create stream context
	ctx, cancel := context.WithCancel(sm.service.ctx)

	// Create service stream
	stream := &ServiceStream{
		StreamID:   streamID,
		CircuitID:  circuitID,
		TargetPort: targetPort,
		TargetAddr: targetAddr,
		conn:       conn,
		circuit:    circuit,
		logger:     sm.logger,
		metrics:    sm.metrics,
		ctx:        ctx,
		cancel:     cancel,
	}

	// Register stream
	sm.mu.Lock()
	sm.streams[streamID] = stream
	sm.mu.Unlock()

	// Record stream creation
	if sm.metrics != nil {
		sm.metrics.RecordOnionServiceStream(true)
	}

	// Send RELAY_CONNECTED response
	if err := sm.sendRelayConnected(circuit, circuitID, streamID); err != nil {
		sm.logger.Error("Failed to send RELAY_CONNECTED", "error", err)
		stream.Close()
		conn.Close()
		return err
	}

	// Start bidirectional forwarding
	go stream.forwardToCircuit()
	go stream.forwardFromCircuit()

	sm.logger.Info("Stream established",
		"stream_id", streamID,
		"target", targetAddr)

	return nil
}

// HandleRelayData handles incoming RELAY_DATA cell and forwards to backend
func (sm *ServiceStreamManager) HandleRelayData(streamID uint16, data []byte) error {
	sm.mu.RLock()
	stream, exists := sm.streams[streamID]
	sm.mu.RUnlock()

	if !exists {
		sm.logger.Warn("RELAY_DATA for unknown stream", "stream_id", streamID)
		return fmt.Errorf("unknown stream %d", streamID)
	}

	// Forward to backend connection
	n, err := stream.conn.Write(data)
	if err != nil {
		sm.logger.Error("Failed to write to backend",
			"stream_id", streamID,
			"error", err)
		stream.Close()
		return err
	}

	// Record data transferred
	if sm.metrics != nil {
		sm.metrics.RecordOnionServiceStreamData(int64(n))
	}

	return nil
}

// HandleRelayEnd handles RELAY_END cell from client
func (sm *ServiceStreamManager) HandleRelayEnd(streamID uint16) error {
	sm.mu.RLock()
	stream, exists := sm.streams[streamID]
	sm.mu.RUnlock()

	if !exists {
		sm.logger.Debug("RELAY_END for unknown stream", "stream_id", streamID)
		return nil
	}

	sm.logger.Info("Received RELAY_END", "stream_id", streamID)
	stream.Close()

	// Remove from manager
	sm.mu.Lock()
	delete(sm.streams, streamID)
	sm.mu.Unlock()

	// Record stream closure
	if sm.metrics != nil {
		sm.metrics.RecordOnionServiceStream(false)
	}

	return nil
}

// connectToBackend establishes connection to local service
func (sm *ServiceStreamManager) connectToBackend(targetAddr string) (net.Conn, error) {
	// Connect with timeout
	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
	}

	conn, err := dialer.Dial("tcp", targetAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to dial backend: %w", err)
	}

	return conn, nil
}

// sendRelayConnected sends RELAY_CONNECTED cell to client
//
// RELAY_CONNECTED format (tor-spec.txt §6.2):
//
//	IPv4 address [4 bytes] (0.0.0.0 for hostname)
//	TTL [4 bytes] (0 for no TTL)
func (sm *ServiceStreamManager) sendRelayConnected(circuit CircuitInterface, circuitID uint32, streamID uint16) error {
	// Build RELAY_CONNECTED payload: IPv4 (4) + TTL (4) = 8 bytes
	payload := make([]byte, 8)
	// Use 0.0.0.0 for hostname-based connections
	// TTL is 0 (not applicable)

	relayCell, err := cell.NewRelayCell(streamID, cell.RelayConnected, payload)
	if err != nil {
		return fmt.Errorf("failed to create RELAY_CONNECTED: %w", err)
	}

	if err := circuit.SendRelayCell(relayCell); err != nil {
		return fmt.Errorf("failed to send RELAY_CONNECTED: %w", err)
	}

	sm.logger.Debug("Sent RELAY_CONNECTED", "stream_id", streamID)
	return nil
}

// sendRelayEnd sends RELAY_END cell to client
//
// RELAY_END format (tor-spec.txt §6.3):
//
//	Reason [1 byte]
func (sm *ServiceStreamManager) sendRelayEnd(circuit CircuitInterface, circuitID uint32, streamID uint16, reason byte) error {
	payload := []byte{reason}

	relayCell, err := cell.NewRelayCell(streamID, cell.RelayEnd, payload)
	if err != nil {
		return fmt.Errorf("failed to create RELAY_END: %w", err)
	}

	if err := circuit.SendRelayCell(relayCell); err != nil {
		return fmt.Errorf("failed to send RELAY_END: %w", err)
	}

	sm.logger.Debug("Sent RELAY_END",
		"stream_id", streamID,
		"reason", reason)
	return nil
}

// forwardToCircuit reads from backend and sends to circuit
func (s *ServiceStream) forwardToCircuit() {
	defer s.Close()

	buffer := make([]byte, 498) // tor-spec.txt: max RELAY_DATA payload
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		// Set read deadline to allow checking context
		if err := s.conn.SetReadDeadline(time.Now().Add(1 * time.Second)); err != nil {
			s.logger.Error("Failed to set read deadline", "error", err)
			return
		}

		n, err := s.conn.Read(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// Timeout is expected, continue
				continue
			}
			if err != io.EOF {
				s.logger.Error("Backend read error",
					"stream_id", s.StreamID,
					"error", err)
			}
			return
		}

		if n > 0 {
			// Send RELAY_DATA to client
			relayCell, err := cell.NewRelayCell(s.StreamID, cell.RelayData, buffer[:n])
			if err != nil {
				s.logger.Error("Failed to create RELAY_DATA", "error", err)
				return
			}

			if err := s.circuit.SendRelayCell(relayCell); err != nil {
				s.logger.Error("Failed to send RELAY_DATA",
					"stream_id", s.StreamID,
					"error", err)
				return
			}

			// Record data transferred
			if s.metrics != nil {
				s.metrics.RecordOnionServiceStreamData(int64(n))
			}
		}
	}
}

// forwardFromCircuit receives data from circuit and writes to backend
// (Data is delivered via HandleRelayData)
func (s *ServiceStream) forwardFromCircuit() {
	// This goroutine mainly monitors context for cancellation
	// Actual data forwarding happens in HandleRelayData
	<-s.ctx.Done()
	s.Close()
}

// Close closes the stream and backend connection
func (s *ServiceStream) Close() error {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			return
		}
		s.closed = true
		s.mu.Unlock()

		if s.cancel != nil {
			s.cancel()
		}

		if s.conn != nil {
			s.conn.Close()
		}

		if s.logger != nil {
			s.logger.Info("Stream closed",
				"stream_id", s.StreamID,
				"target", s.TargetAddr)
		}
	})
	return nil
}

// GetActiveStreamCount returns the number of active streams
func (sm *ServiceStreamManager) GetActiveStreamCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.streams)
}

// CloseAll closes all active streams
func (sm *ServiceStreamManager) CloseAll() {
	sm.mu.Lock()
	streams := make([]*ServiceStream, 0, len(sm.streams))
	for _, s := range sm.streams {
		streams = append(streams, s)
	}
	sm.streams = make(map[uint16]*ServiceStream)
	sm.mu.Unlock()

	for _, s := range streams {
		s.Close()
	}

	sm.logger.Info("All streams closed", "count", len(streams))
}

// parseAddrPort parses "host:port" or "host" from RELAY_BEGIN
func parseAddrPort(addrPort string) (host string, port int, err error) {
	// Try to split on ':'
	var portStr string
	host, portStr, err = net.SplitHostPort(addrPort)
	if err != nil {
		// No port specified, use entire string as host
		host = addrPort
		port = 0
		return host, port, nil
	}

	// Parse port
	var p uint64
	if portStr != "" {
		p, err = parsePort(portStr)
		if err != nil {
			return "", 0, fmt.Errorf("invalid port: %w", err)
		}
	}

	return host, int(p), nil
}

// parsePort parses a port number string
func parsePort(s string) (uint64, error) {
	if len(s) == 0 {
		return 0, fmt.Errorf("empty port")
	}

	var port uint64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid port character: %c", c)
		}
		port = port*10 + uint64(c-'0')
		if port > 65535 {
			return 0, fmt.Errorf("port out of range: %d", port)
		}
	}

	return port, nil
}

// ParseRelayBeginCell parses a RELAY_BEGIN cell payload
//
// Returns the target address/port string and flags
func ParseRelayBeginCell(payload []byte) (addrPort string, flags uint32, err error) {
	// Find null terminator
	nullIdx := -1
	for i, b := range payload {
		if b == 0 {
			nullIdx = i
			break
		}
	}

	if nullIdx == -1 {
		return "", 0, fmt.Errorf("no null terminator in RELAY_BEGIN")
	}

	addrPort = string(payload[:nullIdx])

	// Parse flags if present (4 bytes after null)
	if len(payload) >= nullIdx+1+4 {
		flags = binary.BigEndian.Uint32(payload[nullIdx+1 : nullIdx+5])
	}

	return addrPort, flags, nil
}
