// Package control provides Tor control protocol functionality.
// This package implements a subset of the Tor control protocol relevant to client operations.
// See: https://spec.torproject.org/control-spec
package control

import (
	"bufio"
	"context"
	"crypto/subtle"
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
	password     string // Control password (empty = no auth required)

	// Connection management
	conns   map[net.Conn]*connection
	connsMu sync.RWMutex

	// Event management
	dispatcher *EventDispatcher

	// Authentication rate limiting (per-IP)
	authAttempts   map[string]*authRateLimiter
	authAttemptsMu sync.Mutex

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// authRateLimiter tracks authentication attempts per IP
type authRateLimiter struct {
	attempts  int
	lastTime  time.Time
	backoffMs int
}

// ClientInfoGetter provides access to client information for control commands
type ClientInfoGetter interface {
	GetStats() StatsProvider
	GetConfig() ConfigProvider
}

// StatsProvider provides statistics information
type StatsProvider interface {
	GetActiveCircuits() int
	GetSocksPort() int
	GetControlPort() int
	GetCircuitBuilds() int64
	GetCircuitBuildSuccess() int64
	GetCircuitBuildFailure() int64
	GetGuardsActive() int
	GetGuardsConfirmed() int
	GetUptimeSeconds() int64
	GetConnectionAttempts() int64
	GetDataDir() string
}

// ConfigProvider provides access to configuration values
type ConfigProvider interface {
	GetConfigValue(key string) (string, bool)
	SetConfigValue(key, value string) error
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
	return NewServerWithPassword(address, clientGetter, "", log)
}

// NewServerWithPassword creates a new control protocol server with authentication
func NewServerWithPassword(address string, clientGetter ClientInfoGetter, password string, log *logger.Logger) *Server {
	ctx, cancel := context.WithCancel(context.Background())

	return &Server{
		address:      address,
		logger:       log.Component("control"),
		clientGetter: clientGetter,
		password:     password,
		conns:        make(map[net.Conn]*connection),
		dispatcher:   NewEventDispatcher(),
		authAttempts: make(map[string]*authRateLimiter),
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

	// Close listener (AUDIT-013)
	if s.listener != nil {
		if err := s.listener.Close(); err != nil {
			s.logger.Error("Failed to close control protocol listener", "error", err)
		}
	}

	// Close all connections (AUDIT-013)
	s.connsMu.Lock()
	for conn := range s.conns {
		if err := conn.Close(); err != nil {
			s.logger.Error("Failed to close control protocol connection", "error", err)
		}
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

		// Set read deadline (AUDIT-014)
		if err := netConn.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
			s.logger.Error("Failed to set read deadline", "error", err)
			return
		}

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
		// Handle Close error (AUDIT-013)
		if err := conn.conn.Close(); err != nil {
			s.logger.Error("Failed to close connection on QUIT", "error", err)
		}
	case "PROTOCOLINFO":
		s.handleProtocolInfo(conn, args)
	default:
		conn.writeReply(510, fmt.Sprintf("Unrecognized command %q", cmd))
	}
}

// handleAuthenticate handles AUTHENTICATE command
func (s *Server) handleAuthenticate(conn *connection, args []string) {
	// If no password is configured, accept any authentication
	if s.password == "" {
		conn.mu.Lock()
		conn.authenticated = true
		conn.mu.Unlock()
		conn.writeReply(250, "OK")
		s.logger.Info("Client authenticated (no password required)", "remote", conn.conn.RemoteAddr())
		return
	}

	// Password authentication required
	if len(args) == 0 {
		conn.writeReply(515, "Authentication failed: password required")
		s.logger.Warn("Authentication failed: no password provided", "remote", conn.conn.RemoteAddr())
		return
	}

	// Get password from command (may be quoted)
	password := strings.Join(args, " ")
	password = strings.Trim(password, `"`)

	// Extract IP address (without port) for rate limiting
	remoteIP := conn.conn.RemoteAddr().String()
	if host, _, err := net.SplitHostPort(remoteIP); err == nil {
		remoteIP = host
	}

	// Check rate limiting before attempting authentication
	if !s.checkAuthRateLimit(remoteIP) {
		conn.writeReply(515, "Authentication failed: too many attempts, try again later")
		s.logger.Warn("Authentication rate limited", "remote", remoteIP)
		return
	}

	// Validate password using constant-time comparison to prevent timing attacks
	// Convert strings to byte slices for constant-time comparison
	passwordMatch := subtle.ConstantTimeCompare([]byte(password), []byte(s.password)) == 1

	if !passwordMatch {
		s.recordFailedAuth(remoteIP)
		conn.writeReply(515, "Authentication failed: incorrect password")
		s.logger.Warn("Authentication failed: incorrect password", "remote", remoteIP)
		return
	}

	// Reset rate limiter on successful authentication
	s.resetAuthRateLimit(remoteIP)

	// Authentication successful
	conn.mu.Lock()
	conn.authenticated = true
	conn.mu.Unlock()
	conn.writeReply(250, "OK")
	s.logger.Info("Client authenticated", "remote", conn.conn.RemoteAddr())
}

// handleProtocolInfo handles PROTOCOLINFO command
func (s *Server) handleProtocolInfo(conn *connection, args []string) {
	// No authentication required for PROTOCOLINFO per control-spec.txt
	authMethods := "NULL"
	if s.password != "" {
		authMethods = "HASHEDPASSWORD"
	}

	conn.writeDataReply([]string{
		"250-PROTOCOLINFO 1",
		fmt.Sprintf("250-AUTH METHODS=%s", authMethods),
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

	// Circuit statistics
	case "status/circuits":
		return fmt.Sprintf("%d", stats.GetActiveCircuits()), true
	case "status/circuit-builds":
		return fmt.Sprintf("%d", stats.GetCircuitBuilds()), true
	case "status/circuit-build-success":
		return fmt.Sprintf("%d", stats.GetCircuitBuildSuccess()), true
	case "status/circuit-build-failure":
		return fmt.Sprintf("%d", stats.GetCircuitBuildFailure()), true

	// Guard statistics
	case "status/guards/active":
		return fmt.Sprintf("%d", stats.GetGuardsActive()), true
	case "status/guards/confirmed":
		return fmt.Sprintf("%d", stats.GetGuardsConfirmed()), true

	// Connection statistics
	case "status/connection-attempts":
		return fmt.Sprintf("%d", stats.GetConnectionAttempts()), true

	// System information
	case "status/uptime":
		return fmt.Sprintf("%d", stats.GetUptimeSeconds()), true
	case "config-file":
		// Return data directory path (closest equivalent for client)
		return stats.GetDataDir(), true
	case "config-text":
		// Not implemented - would require full config serialization
		return "", false

	// Port information
	case "net/listeners/socks":
		return fmt.Sprintf("127.0.0.1:%d", stats.GetSocksPort()), true
	case "net/listeners/control":
		return fmt.Sprintf("127.0.0.1:%d", stats.GetControlPort()), true

	// Help information
	case "info/names":
		return s.getInfoNames(), true

	default:
		return "", false
	}
}

// getInfoNames returns a list of all supported GETINFO keys
func (s *Server) getInfoNames() string {
	keys := []string{
		"version",
		"traffic/read",
		"traffic/written",
		"status/circuit-established",
		"status/enough-dir-info",
		"status/circuits",
		"status/circuit-builds",
		"status/circuit-build-success",
		"status/circuit-build-failure",
		"status/guards/active",
		"status/guards/confirmed",
		"status/connection-attempts",
		"status/uptime",
		"config-file",
		"net/listeners/socks",
		"net/listeners/control",
		"info/names",
	}
	return strings.Join(keys, " ")
}

// handleGetConf handles GETCONF command per control-spec.txt §3.1
func (s *Server) handleGetConf(conn *connection, args []string) {
	if !conn.authenticated {
		conn.writeReply(514, "Authentication required")
		return
	}

	if len(args) == 0 {
		conn.writeReply(552, "Missing argument")
		return
	}

	configProvider := s.clientGetter.GetConfig()
	if configProvider == nil {
		// Fallback to empty values if config is not available
		var replies []string
		for _, key := range args {
			replies = append(replies, fmt.Sprintf("250-%s=", key))
		}
		if len(replies) > 0 {
			replies[len(replies)-1] = strings.Replace(replies[len(replies)-1], "250-", "250 ", 1)
		}
		conn.writeDataReply(replies)
		return
	}

	var replies []string
	for _, key := range args {
		value, exists := configProvider.GetConfigValue(key)
		if !exists {
			// Per control-spec.txt, unknown config keys return empty value
			replies = append(replies, fmt.Sprintf("250-%s=", key))
		} else {
			replies = append(replies, fmt.Sprintf("250-%s=%s", key, value))
		}
	}

	if len(replies) > 0 {
		replies[len(replies)-1] = strings.Replace(replies[len(replies)-1], "250-", "250 ", 1)
	}

	conn.writeDataReply(replies)
}

// handleSetConf handles SETCONF command per control-spec.txt §3.1
func (s *Server) handleSetConf(conn *connection, args []string) {
	if !conn.authenticated {
		conn.writeReply(514, "Authentication required")
		return
	}

	if len(args) == 0 {
		conn.writeReply(552, "Missing argument")
		return
	}

	configProvider := s.clientGetter.GetConfig()
	if configProvider == nil {
		// If config is not available, just acknowledge
		conn.writeReply(250, "OK")
		return
	}

	// Parse and set configuration values
	for _, arg := range args {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) != 2 {
			conn.writeReply(552, fmt.Sprintf("Invalid argument: %s", arg))
			return
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if err := configProvider.SetConfigValue(key, value); err != nil {
			conn.writeReply(553, fmt.Sprintf("Failed to set %s: %v", key, err))
			return
		}
	}

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
	// Handle WriteString error (AUDIT-012)
	// Note: Errors are intentionally ignored here as they indicate a broken connection
	// which will be detected on the next read. Logging would require access to the server's logger.
	_, _ = c.writer.WriteString(line)
	// Handle Flush error (AUDIT-012)
	_ = c.writer.Flush()
}

// writeDataReply writes a multi-line reply
func (c *connection) writeDataReply(lines []string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, line := range lines {
		// Handle WriteString error (AUDIT-012)
		// Note: Errors are intentionally ignored here as they indicate a broken connection
		// which will be detected on the next read. Logging would require access to the server's logger.
		_, _ = c.writer.WriteString(line + "\r\n")
	}
	// Handle Flush error (AUDIT-012)
	_ = c.writer.Flush()
}

// checkAuthRateLimit checks if authentication attempts should be rate limited
// Returns true if authentication attempt is allowed, false if rate limited
func (s *Server) checkAuthRateLimit(remoteIP string) bool {
	s.authAttemptsMu.Lock()
	defer s.authAttemptsMu.Unlock()

	limiter, exists := s.authAttempts[remoteIP]
	if !exists {
		// First attempt, allow
		return true
	}

	// Check if backoff period has elapsed
	elapsed := time.Since(limiter.lastTime)
	backoffDuration := time.Duration(limiter.backoffMs) * time.Millisecond

	if elapsed < backoffDuration {
		// Still in backoff period
		return false
	}

	// Backoff period elapsed, allow new attempt
	return true
}

// recordFailedAuth records a failed authentication attempt
func (s *Server) recordFailedAuth(remoteIP string) {
	s.authAttemptsMu.Lock()
	defer s.authAttemptsMu.Unlock()

	limiter, exists := s.authAttempts[remoteIP]
	if !exists {
		limiter = &authRateLimiter{
			attempts:  1,
			lastTime:  time.Now(),
			backoffMs: 1000, // 1 second initial backoff
		}
		s.authAttempts[remoteIP] = limiter
		return
	}

	// Update attempt count and backoff with exponential backoff
	limiter.attempts++
	limiter.lastTime = time.Now()

	// Exponential backoff: 1s, 2s, 4s, 8s, ..., max 60s
	limiter.backoffMs = limiter.backoffMs * 2
	if limiter.backoffMs > 60000 {
		limiter.backoffMs = 60000
	}
}

// resetAuthRateLimit resets rate limiting for an IP on successful authentication
func (s *Server) resetAuthRateLimit(remoteIP string) {
	s.authAttemptsMu.Lock()
	defer s.authAttemptsMu.Unlock()

	delete(s.authAttempts, remoteIP)
}
