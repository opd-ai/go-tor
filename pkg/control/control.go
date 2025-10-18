// Package control provides Tor control protocol functionality.
// This package implements a subset of the Tor control protocol relevant to client operations.
// See: https://spec.torproject.org/control-spec
package control

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/opd-ai/go-tor/pkg/logger"
)

// Server represents a Tor control protocol server
type Server struct {
	address      string
	listener     net.Listener
	logger       *logger.Logger
	clientGetter ClientInfoGetter

	// Connection management
	conns   map[net.Conn]*connection
	connsMu sync.RWMutex

	// Event management
	dispatcher *EventDispatcher

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// ClientInfoGetter provides access to client information for control commands
type ClientInfoGetter interface {
	GetStats() StatsProvider
}

// StatsProvider provides statistics information
type StatsProvider interface {
	GetActiveCircuits() int
	GetSocksPort() int
	GetControlPort() int
}

// connection represents a single control protocol connection
type connection struct {
	conn          net.Conn
	reader        *bufio.Reader
	writer        *bufio.Writer
	authenticated bool
	events        map[string]bool // subscribed events
	mu            sync.Mutex
}

// NewServer creates a new control protocol server
func NewServer(address string, clientGetter ClientInfoGetter, log *logger.Logger) *Server {
	ctx, cancel := context.WithCancel(context.Background())

	return &Server{
		address:      address,
		logger:       log.Component("control"),
		clientGetter: clientGetter,
		conns:        make(map[net.Conn]*connection),
		dispatcher:   NewEventDispatcher(),
		ctx:          ctx,
		cancel:       cancel,
	}
}

// GetEventDispatcher returns the event dispatcher for publishing events
func (s *Server) GetEventDispatcher() *EventDispatcher {
	return s.dispatcher
}

// Start starts the control protocol server
func (s *Server) Start() error {
	listener, err := net.Listen("tcp", s.address)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.address, err)
	}

	s.listener = listener
	s.logger.Info("Control protocol server listening", "address", s.address)

	// Accept connections in background
	s.wg.Add(1)
	go s.acceptLoop()

	return nil
}

// Stop stops the control protocol server
func (s *Server) Stop() error {
	s.logger.Info("Stopping control protocol server")

	// Cancel context
	s.cancel()

	// Close listener
	if s.listener != nil {
		s.listener.Close()
	}

	// Close all connections
	s.connsMu.Lock()
	for conn := range s.conns {
		conn.Close()
	}
	s.connsMu.Unlock()

	// Wait for goroutines
	s.wg.Wait()

	s.logger.Info("Control protocol server stopped")
	return nil
}

// acceptLoop accepts incoming connections
func (s *Server) acceptLoop() {
	defer s.wg.Done()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return
			default:
				s.logger.Warn("Failed to accept connection", "error", err)
				continue
			}
		}

		s.logger.Info("New control connection", "remote", conn.RemoteAddr())

		// Handle connection in background
		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

// handleConnection handles a single control protocol connection
func (s *Server) handleConnection(netConn net.Conn) {
	defer s.wg.Done()
	defer netConn.Close()

	// Create connection state
	conn := &connection{
		conn:          netConn,
		reader:        bufio.NewReader(netConn),
		writer:        bufio.NewWriter(netConn),
		authenticated: false,
		events:        make(map[string]bool),
	}

	// Register connection
	s.connsMu.Lock()
	s.conns[netConn] = conn
	s.connsMu.Unlock()

	// Unregister on exit
	defer func() {
		// Unsubscribe from events
		s.dispatcher.Unsubscribe(conn)
		
		s.connsMu.Lock()
		delete(s.conns, netConn)
		s.connsMu.Unlock()
	}()

	// Send greeting
	conn.writeReply(250, "OK")

	// Process commands
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		// Set read deadline
		netConn.SetReadDeadline(time.Now().Add(30 * time.Second))

		// Read command
		line, err := conn.reader.ReadString('\n')
		if err != nil {
			if err.Error() != "EOF" {
				s.logger.Debug("Connection read error", "error", err)
			}
			return
		}

		// Parse and handle command
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		s.logger.Debug("Control command received", "command", line)
		s.handleCommand(conn, line)
	}
}

// handleCommand processes a control protocol command
func (s *Server) handleCommand(conn *connection, line string) {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		conn.writeReply(500, "Syntax error: empty command")
		return
	}

	cmd := strings.ToUpper(parts[0])
	args := parts[1:]

	switch cmd {
	case "AUTHENTICATE":
		s.handleAuthenticate(conn, args)
	case "GETINFO":
		s.handleGetInfo(conn, args)
	case "GETCONF":
		s.handleGetConf(conn, args)
	case "SETCONF":
		s.handleSetConf(conn, args)
	case "SETEVENTS":
		s.handleSetEvents(conn, args)
	case "QUIT":
		conn.writeReply(250, "closing connection")
		conn.conn.Close()
	case "PROTOCOLINFO":
		s.handleProtocolInfo(conn, args)
	default:
		conn.writeReply(510, fmt.Sprintf("Unrecognized command %q", cmd))
	}
}

// handleAuthenticate handles AUTHENTICATE command
func (s *Server) handleAuthenticate(conn *connection, args []string) {
	// For now, accept any authentication (including no password)
	// In production, this should validate a cookie or password
	conn.mu.Lock()
	conn.authenticated = true
	conn.mu.Unlock()

	conn.writeReply(250, "OK")
	s.logger.Info("Client authenticated", "remote", conn.conn.RemoteAddr())
}

// handleProtocolInfo handles PROTOCOLINFO command
func (s *Server) handleProtocolInfo(conn *connection, args []string) {
	// No authentication required for PROTOCOLINFO
	conn.writeDataReply([]string{
		"250-PROTOCOLINFO 1",
		"250-AUTH METHODS=NULL",
		"250-VERSION Tor=\"go-tor-0.1.0\"",
		"250 OK",
	})
}

// handleGetInfo handles GETINFO command
func (s *Server) handleGetInfo(conn *connection, args []string) {
	if !conn.authenticated {
		conn.writeReply(514, "Authentication required")
		return
	}

	if len(args) == 0 {
		conn.writeReply(552, "Missing argument")
		return
	}

	// Get client stats
	stats := s.clientGetter.GetStats()

	var replies []string
	for _, key := range args {
		value, ok := s.getInfoValue(key, stats)
		if !ok {
			conn.writeReply(552, fmt.Sprintf("Unrecognized key %q", key))
			return
		}
		replies = append(replies, fmt.Sprintf("250-%s=%s", key, value))
	}

	// Last reply without dash
	if len(replies) > 0 {
		replies[len(replies)-1] = strings.Replace(replies[len(replies)-1], "250-", "250 ", 1)
	}

	conn.writeDataReply(replies)
}

// getInfoValue gets the value for a GETINFO key
func (s *Server) getInfoValue(key string, stats StatsProvider) (string, bool) {
	switch key {
	case "version":
		return "go-tor 0.1.0", true
	case "traffic/read", "traffic/written":
		return "0", true
	case "status/circuit-established":
		// Check if we have any circuits
		if stats.GetActiveCircuits() > 0 {
			return "1", true
		}
		return "0", true
	case "status/enough-dir-info":
		return "1", true
	default:
		return "", false
	}
}

// handleGetConf handles GETCONF command
func (s *Server) handleGetConf(conn *connection, args []string) {
	if !conn.authenticated {
		conn.writeReply(514, "Authentication required")
		return
	}

	if len(args) == 0 {
		conn.writeReply(552, "Missing argument")
		return
	}

	// Return dummy values for now
	var replies []string
	for _, key := range args {
		replies = append(replies, fmt.Sprintf("250-%s=", key))
	}

	if len(replies) > 0 {
		replies[len(replies)-1] = strings.Replace(replies[len(replies)-1], "250-", "250 ", 1)
	}

	conn.writeDataReply(replies)
}

// handleSetConf handles SETCONF command
func (s *Server) handleSetConf(conn *connection, args []string) {
	if !conn.authenticated {
		conn.writeReply(514, "Authentication required")
		return
	}

	// For now, just acknowledge
	conn.writeReply(250, "OK")
}

// handleSetEvents handles SETEVENTS command
func (s *Server) handleSetEvents(conn *connection, args []string) {
	if !conn.authenticated {
		conn.writeReply(514, "Authentication required")
		return
	}

	conn.mu.Lock()
	// Clear existing events
	conn.events = make(map[string]bool)

	// Register new events with connection and dispatcher
	var eventTypes []EventType
	for _, event := range args {
		eventUpper := strings.ToUpper(event)
		conn.events[eventUpper] = true
		eventTypes = append(eventTypes, EventType(eventUpper))
	}
	conn.mu.Unlock()

	// Update dispatcher subscriptions
	s.dispatcher.Subscribe(conn, eventTypes)

	conn.writeReply(250, "OK")
	s.logger.Debug("Events subscribed", "events", args, "remote", conn.conn.RemoteAddr())
}

// writeReply writes a simple reply
func (c *connection) writeReply(code int, message string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	line := fmt.Sprintf("%d %s\r\n", code, message)
	c.writer.WriteString(line)
	c.writer.Flush()
}

// writeDataReply writes a multi-line reply
func (c *connection) writeDataReply(lines []string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, line := range lines {
		c.writer.WriteString(line + "\r\n")
	}
	c.writer.Flush()
}
