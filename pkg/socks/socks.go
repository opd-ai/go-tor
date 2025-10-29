// Package socks provides SOCKS5 proxy server functionality.
// This package implements a SOCKS5 server that routes connections through Tor circuits.
package socks

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/opd-ai/go-tor/pkg/circuit"
	"github.com/opd-ai/go-tor/pkg/logger"
	"github.com/opd-ai/go-tor/pkg/onion"
	"github.com/opd-ai/go-tor/pkg/pool"
	"github.com/opd-ai/go-tor/pkg/stream"
)

const (
	// SOCKS version
	socks5Version = 0x05

	// Authentication methods
	authNone     = 0x00
	authPassword = 0x02
	authNoAccept = 0xFF

	// Address types
	addrIPv4   = 0x01
	addrDomain = 0x03
	addrIPv6   = 0x04

	// Commands
	cmdConnect = 0x01
	cmdBind    = 0x02
	cmdUDP     = 0x03

	// Tor-specific commands for DNS leak prevention (RFC 1928 extensions)
	// These commands allow DNS queries to be routed through Tor circuits
	cmdResolve    = 0xF0 // RESOLVE: DNS hostname to IP address
	cmdResolvePTR = 0xF1 // RESOLVE_PTR: Reverse DNS (IP to hostname)

	// Reply codes
	replySuccess              = 0x00
	replyGeneralFailure       = 0x01
	replyConnectionNotAllowed = 0x02
	replyNetworkUnreachable   = 0x03
	replyHostUnreachable      = 0x04
	replyConnectionRefused    = 0x05
	replyTTLExpired           = 0x06
	replyCommandNotSupported  = 0x07
	replyAddressNotSupported  = 0x08

	// SEC-L006: Default connection limit (configurable via Config)
	defaultMaxConnections = 1000
)

// requestInfo contains parsed SOCKS5 request information
type requestInfo struct {
	cmd        byte   // Command: CONNECT, RESOLVE, or RESOLVE_PTR
	targetAddr string // Target address (host:port for CONNECT, hostname for RESOLVE, IP for RESOLVE_PTR)
}

// Config holds configuration for the SOCKS5 server
type Config struct {
	// MaxConnections limits concurrent SOCKS5 connections
	// SEC-L006: Configurable for resource-constrained embedded systems
	// Set to 0 for unlimited (not recommended for production)
	MaxConnections int

	// DNS leak prevention (ROADMAP Phase 1.2)
	// EnableDNSResolution allows DNS queries through SOCKS5 RESOLVE/RESOLVE_PTR
	// When enabled, clients can perform DNS lookups through Tor circuits
	EnableDNSResolution bool
	// DNSTimeout specifies the timeout for DNS resolution operations
	DNSTimeout time.Duration

	// Circuit isolation configuration
	IsolationLevel      circuit.IsolationLevel // Isolation level to use
	IsolateDestinations bool                   // Isolate by destination
	IsolateSOCKSAuth    bool                   // Isolate by SOCKS5 credentials
	IsolateClientPort   bool                   // Isolate by client port
}

// DefaultConfig returns default SOCKS5 server configuration
func DefaultConfig() *Config {
	return &Config{
		MaxConnections:      defaultMaxConnections,
		EnableDNSResolution: true,                  // DNS leak prevention enabled by default
		DNSTimeout:          30 * time.Second,      // Standard DNS timeout
		IsolationLevel:      circuit.IsolationNone, // Backward compatible default
		IsolateDestinations: false,
		IsolateSOCKSAuth:    false,
		IsolateClientPort:   false,
	}
}

// Server is a SOCKS5 proxy server
// SEC-M001/MED-004: Circuit isolation for different SOCKS5 connections
type Server struct {
	address       string
	listener      net.Listener
	circuitMgr    *circuit.Manager
	circuitPool   *pool.CircuitPool // Optional: for circuit isolation support
	streamMgr     *stream.Manager   // Stream manager for multiplexing
	onionClient   *onion.Client
	logger        *logger.Logger
	config        *Config // SEC-L006: Configurable server settings
	mu            sync.Mutex
	activeConns   map[net.Conn]struct{}
	shutdown      chan struct{}
	shutdownOnce  sync.Once
	closeListener sync.Once
	listenerReady chan struct{} // Signals when listener is ready
}

// NewServer creates a new SOCKS5 proxy server
// SEC-L006: Accepts optional Config for configurable settings
func NewServer(address string, circuitMgr *circuit.Manager, log *logger.Logger) *Server {
	return NewServerWithConfig(address, circuitMgr, log, nil)
}

// NewServerWithConfig creates a new SOCKS5 proxy server with custom configuration
// SEC-L006: Allows customization of connection limits and other settings
func NewServerWithConfig(address string, circuitMgr *circuit.Manager, log *logger.Logger, cfg *Config) *Server {
	if log == nil {
		log = logger.NewDefault()
	}
	if cfg == nil {
		cfg = DefaultConfig()
	}

	return &Server{
		address:       address,
		circuitMgr:    circuitMgr,
		streamMgr:     stream.NewManager(log),
		onionClient:   onion.NewClient(log),
		logger:        log.Component("socks5"),
		config:        cfg,
		activeConns:   make(map[net.Conn]struct{}),
		shutdown:      make(chan struct{}),
		listenerReady: make(chan struct{}),
	}
}

// SetCircuitPool sets the circuit pool for isolated circuit selection
// This should be called after the pool is initialized (usually by the client)
func (s *Server) SetCircuitPool(pool *pool.CircuitPool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.circuitPool = pool
}

// ListenAndServe starts the SOCKS5 server
func (s *Server) ListenAndServe(ctx context.Context) error {
	s.logger.Info("Starting SOCKS5 server", "address", s.address)

	listener, err := net.Listen("tcp", s.address)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	// Use mutex to protect listener assignment
	s.mu.Lock()
	s.listener = listener
	s.mu.Unlock()

	// Signal that listener is ready
	close(s.listenerReady)

	s.logger.Info("SOCKS5 server listening", "address", s.address)

	// Accept connections
	go s.acceptLoop(ctx)

	// Wait for context cancellation
	<-ctx.Done()

	// Shutdown
	return s.Shutdown(context.Background())
}

// acceptLoop accepts incoming connections
func (s *Server) acceptLoop(ctx context.Context) {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.shutdown:
				return
			default:
				s.logger.Error("Failed to accept connection", "error", err)
				continue
			}
		}

		// Check connection limit (SEC-L006: configurable limit)
		s.mu.Lock()
		maxConns := s.config.MaxConnections
		if maxConns > 0 && len(s.activeConns) >= maxConns {
			s.mu.Unlock()
			s.logger.Warn("Connection limit reached, rejecting connection",
				"limit", maxConns, "current", len(s.activeConns), "remote", conn.RemoteAddr())
			if err := conn.Close(); err != nil {
				s.logger.Error("Failed to close rejected connection", "function", "acceptLoop", "error", err)
			}
			continue
		}

		// Track connection
		s.activeConns[conn] = struct{}{}
		s.mu.Unlock()

		// Handle connection
		go s.handleConnection(ctx, conn)
	}
}

// handleConnection handles a SOCKS5 connection
func (s *Server) handleConnection(ctx context.Context, conn net.Conn) {
	defer func() {
		if err := conn.Close(); err != nil {
			s.logger.Error("Failed to close connection", "function", "handleConnection", "error", err)
		}
		s.mu.Lock()
		delete(s.activeConns, conn)
		s.mu.Unlock()
	}()

	// Set read deadline
	if err := conn.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
		s.logger.Error("Failed to set read deadline", "function", "handleConnection", "error", err)
		return
	}

	// Handshake - returns username if password auth was used
	username, err := s.handshake(conn)
	if err != nil {
		s.logger.Error("Handshake failed", "error", err, "remote", conn.RemoteAddr())
		return
	}

	// Read request
	request, err := s.readRequest(conn)
	if err != nil {
		s.logger.Error("Failed to read request", "error", err, "remote", conn.RemoteAddr())
		return
	}

	s.logger.Info("SOCKS5 request", "command", fmt.Sprintf("0x%02X", request.cmd), "target", request.targetAddr, "remote", conn.RemoteAddr(), "username", username)

	// Handle DNS resolution commands
	switch request.cmd {
	case cmdResolve:
		s.handleResolve(ctx, conn, request.targetAddr)
		return
	case cmdResolvePTR:
		s.handleResolvePTR(ctx, conn, request.targetAddr)
		return
	case cmdConnect:
		// Continue with normal CONNECT handling below
	default:
		s.logger.Error("Unsupported command", "command", fmt.Sprintf("0x%02X", request.cmd))
		s.sendReply(conn, replyCommandNotSupported, nil)
		return
	}

	// CONNECT command handling
	targetAddr := request.targetAddr

	// Extract hostname from targetAddr (format: "host:port")
	host := targetAddr
	if idx := strings.LastIndex(targetAddr, ":"); idx != -1 {
		host = targetAddr[:idx]
	}

	// Check if this is an onion address
	isOnion := onion.IsOnionAddress(host)
	if isOnion {
		// Parse and validate the onion address
		addr, err := onion.ParseAddress(host)
		if err != nil {
			s.logger.Warn("Invalid onion address", "address", host, "error", err)
			s.sendReply(conn, replyHostUnreachable, nil)
			return
		}

		s.logger.Info("Onion service connection requested", "address", host)

		// Connect to the onion service using rendezvous protocol
		circuitID, err := s.onionClient.ConnectToOnionService(ctx, addr)
		if err != nil {
			s.logger.Error("Failed to connect to onion service", "address", host, "error", err)
			s.sendReply(conn, replyHostUnreachable, nil)
			return
		}

		s.logger.Info("Successfully connected to onion service",
			"address", host,
			"circuit_id", circuitID)

		// Send success reply
		s.sendReply(conn, replySuccess, conn.LocalAddr())

		// In Phase 8, this would relay data through the rendezvous circuit
		// For Phase 7.3.4, we just log success and close
		s.logger.Debug("Onion service connection established (mock relay)")
		time.Sleep(100 * time.Millisecond)
		return
	}

	// For regular addresses, use circuit isolation if configured
	// Create isolation key based on configuration
	var isolationKey *circuit.IsolationKey
	s.mu.Lock()
	isolationCfg := s.config
	circuitPool := s.circuitPool
	s.mu.Unlock()

	// Build isolation key based on configured isolation level
	if isolationCfg.IsolationLevel != circuit.IsolationNone {
		isolationKey = circuit.NewIsolationKey(isolationCfg.IsolationLevel)

		switch isolationCfg.IsolationLevel {
		case circuit.IsolationDestination:
			if isolationCfg.IsolateDestinations {
				isolationKey = isolationKey.WithDestination(targetAddr)
			}
		case circuit.IsolationCredential:
			if isolationCfg.IsolateSOCKSAuth && username != "" {
				isolationKey = isolationKey.WithCredentials(username)
			}
		case circuit.IsolationPort:
			if isolationCfg.IsolateClientPort {
				if tcpAddr, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
					isolationKey = isolationKey.WithSourcePort(uint16(tcpAddr.Port))
				}
			}
		}

		// Validate the isolation key
		if err := isolationKey.Validate(); err != nil {
			s.logger.Warn("Invalid isolation key, falling back to no isolation",
				"error", err,
				"level", isolationCfg.IsolationLevel)
			isolationKey = nil
		}
	}

	// If circuit pool is available, request an isolated circuit
	var circ *circuit.Circuit
	if circuitPool != nil && isolationKey != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var err error
		circ, err = circuitPool.GetWithIsolation(ctx, isolationKey)
		if err != nil {
			s.logger.Error("Failed to get isolated circuit", "error", err, "isolation_key", isolationKey)
			s.sendReply(conn, replyGeneralFailure, nil)
			return
		}

		s.logger.Info("Using isolated circuit",
			"circuit_id", circ.ID,
			"isolation_key", isolationKey.String(),
			"target", targetAddr)

		// Return circuit to pool when done
		defer circuitPool.Put(circ)
	} else {
		// No circuit pool or no isolation - get any available circuit
		// For now, we'll require a circuit pool
		s.logger.Error("No circuit pool available for connection")
		s.sendReply(conn, replyGeneralFailure, nil)
		return
	}

	// Parse target address and port
	hostStr, portStr, err := net.SplitHostPort(targetAddr)
	if err != nil {
		s.logger.Error("Failed to parse target address", "target", targetAddr, "error", err)
		s.sendReply(conn, replyGeneralFailure, nil)
		return
	}

	var port uint16
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		s.logger.Error("Failed to parse port", "port", portStr, "error", err)
		s.sendReply(conn, replyGeneralFailure, nil)
		return
	}

	// Create a stream
	strm, err := s.streamMgr.CreateStream(circ.ID, hostStr, port)
	if err != nil {
		s.logger.Error("Failed to create stream", "error", err)
		s.sendReply(conn, replyGeneralFailure, nil)
		return
	}
	defer s.streamMgr.RemoveStream(strm.ID)

	s.logger.Info("Created stream",
		"stream_id", strm.ID,
		"circuit_id", circ.ID,
		"target", targetAddr)

	// Update stream state
	strm.SetState(stream.StateConnecting)

	// Open the stream on the circuit (sends RELAY_BEGIN and waits for RELAY_CONNECTED)
	if err := circ.OpenStream(strm.ID, hostStr, port); err != nil {
		s.logger.Error("Failed to open stream", "stream_id", strm.ID, "error", err)
		s.sendReply(conn, replyHostUnreachable, nil)
		return
	}

	// Stream is now connected
	strm.SetState(stream.StateConnected)

	s.logger.Info("Stream connected",
		"stream_id", strm.ID,
		"target", targetAddr)

	// Send SOCKS5 success reply
	s.sendReply(conn, replySuccess, conn.LocalAddr())

	// Relay data bidirectionally between SOCKS client and Tor circuit
	s.relayDataThroughCircuit(ctx, conn, circ, strm)
}

// handshake performs SOCKS5 handshake and returns optional username for isolation
func (s *Server) handshake(conn net.Conn) (string, error) {
	// Read version and methods
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		return "", fmt.Errorf("failed to read handshake header: %w", err)
	}

	version := header[0]
	nmethods := header[1]

	if version != socks5Version {
		return "", fmt.Errorf("unsupported SOCKS version: %d", version)
	}

	// Read methods
	methods := make([]byte, nmethods)
	if _, err := io.ReadFull(conn, methods); err != nil {
		return "", fmt.Errorf("failed to read methods: %w", err)
	}

	// Check which methods are supported
	supportsNoAuth := false
	supportsPassword := false
	for _, method := range methods {
		if method == authNone {
			supportsNoAuth = true
		}
		if method == authPassword {
			supportsPassword = true
		}
	}

	// Prefer password auth for isolation support, fall back to no auth
	var selectedMethod byte
	var username string
	if supportsPassword {
		selectedMethod = authPassword
	} else if supportsNoAuth {
		selectedMethod = authNone
	} else {
		// No acceptable methods
		response := []byte{socks5Version, authNoAccept}
		if _, err := conn.Write(response); err != nil {
			return "", fmt.Errorf("failed to write auth rejection: %w", err)
		}
		return "", fmt.Errorf("no acceptable authentication methods")
	}

	// Send method selection
	response := []byte{socks5Version, selectedMethod}
	if _, err := conn.Write(response); err != nil {
		return "", fmt.Errorf("failed to write method selection: %w", err)
	}

	// Perform username/password authentication if selected
	if selectedMethod == authPassword {
		var err error
		username, err = s.authenticatePassword(conn)
		if err != nil {
			return "", err
		}
	}

	return username, nil
}

// authenticatePassword performs username/password authentication (RFC 1929)
// Returns the username for circuit isolation purposes
func (s *Server) authenticatePassword(conn net.Conn) (string, error) {
	// Read version
	version := make([]byte, 1)
	if _, err := io.ReadFull(conn, version); err != nil {
		return "", fmt.Errorf("failed to read auth version: %w", err)
	}

	if version[0] != 0x01 {
		return "", fmt.Errorf("unsupported auth version: %d", version[0])
	}

	// Read username length
	ulenBuf := make([]byte, 1)
	if _, err := io.ReadFull(conn, ulenBuf); err != nil {
		return "", fmt.Errorf("failed to read username length: %w", err)
	}
	ulen := int(ulenBuf[0])

	// Read username
	username := make([]byte, ulen)
	if _, err := io.ReadFull(conn, username); err != nil {
		return "", fmt.Errorf("failed to read username: %w", err)
	}

	// Read password length
	plenBuf := make([]byte, 1)
	if _, err := io.ReadFull(conn, plenBuf); err != nil {
		return "", fmt.Errorf("failed to read password length: %w", err)
	}
	plen := int(plenBuf[0])

	// Read password (we don't validate it, just use for isolation)
	password := make([]byte, plen)
	if _, err := io.ReadFull(conn, password); err != nil {
		return "", fmt.Errorf("failed to read password: %w", err)
	}

	// Send success response (version=1, status=0 for success)
	response := []byte{0x01, 0x00}
	if _, err := conn.Write(response); err != nil {
		return "", fmt.Errorf("failed to write auth response: %w", err)
	}

	return string(username), nil
}

// readRequest reads a SOCKS5 request and returns command type and target
func (s *Server) readRequest(conn net.Conn) (*requestInfo, error) {
	// Read request header
	header := make([]byte, 4)
	if _, err := io.ReadFull(conn, header); err != nil {
		return nil, fmt.Errorf("failed to read request header: %w", err)
	}

	version := header[0]
	cmd := header[1]
	// reserved := header[2]
	addrType := header[3]

	if version != socks5Version {
		s.sendReply(conn, replyGeneralFailure, nil)
		return nil, fmt.Errorf("unsupported SOCKS version: %d", version)
	}

	// Validate command
	switch cmd {
	case cmdConnect:
		// Always supported
	case cmdResolve, cmdResolvePTR:
		// DNS resolution commands - check if enabled
		if !s.config.EnableDNSResolution {
			s.sendReply(conn, replyCommandNotSupported, nil)
			return nil, fmt.Errorf("DNS resolution disabled (command: 0x%02X)", cmd)
		}
	case cmdBind, cmdUDP:
		// Not supported
		s.sendReply(conn, replyCommandNotSupported, nil)
		return nil, fmt.Errorf("unsupported command: 0x%02X", cmd)
	default:
		s.sendReply(conn, replyCommandNotSupported, nil)
		return nil, fmt.Errorf("unknown command: 0x%02X", cmd)
	}

	// Read address
	var addr string
	switch addrType {
	case addrIPv4:
		ip := make([]byte, 4)
		if _, err := io.ReadFull(conn, ip); err != nil {
			s.sendReply(conn, replyGeneralFailure, nil)
			return nil, fmt.Errorf("failed to read IPv4 address: %w", err)
		}
		addr = net.IP(ip).String()

	case addrDomain:
		domainLen := make([]byte, 1)
		if _, err := io.ReadFull(conn, domainLen); err != nil {
			s.sendReply(conn, replyGeneralFailure, nil)
			return nil, fmt.Errorf("failed to read domain length: %w", err)
		}

		domain := make([]byte, domainLen[0])
		if _, err := io.ReadFull(conn, domain); err != nil {
			s.sendReply(conn, replyGeneralFailure, nil)
			return nil, fmt.Errorf("failed to read domain: %w", err)
		}
		addr = string(domain)

	case addrIPv6:
		ip := make([]byte, 16)
		if _, err := io.ReadFull(conn, ip); err != nil {
			s.sendReply(conn, replyGeneralFailure, nil)
			return nil, fmt.Errorf("failed to read IPv6 address: %w", err)
		}
		addr = net.IP(ip).String()

	default:
		s.sendReply(conn, replyAddressNotSupported, nil)
		return nil, fmt.Errorf("unsupported address type: %d", addrType)
	}

	// Read port
	portBytes := make([]byte, 2)
	if _, err := io.ReadFull(conn, portBytes); err != nil {
		s.sendReply(conn, replyGeneralFailure, nil)
		return nil, fmt.Errorf("failed to read port: %w", err)
	}
	port := binary.BigEndian.Uint16(portBytes)

	// Format target address based on command
	var targetAddr string
	if cmd == cmdResolve {
		// For RESOLVE, we only need the hostname (port is ignored but must be read)
		targetAddr = addr
	} else if cmd == cmdResolvePTR {
		// For RESOLVE_PTR, we need the IP address (port is ignored)
		targetAddr = addr
	} else {
		// For CONNECT and other commands, include port
		targetAddr = fmt.Sprintf("%s:%d", addr, port)
	}

	return &requestInfo{
		cmd:        cmd,
		targetAddr: targetAddr,
	}, nil
}

// sendReply sends a SOCKS5 reply
func (s *Server) sendReply(conn net.Conn, reply byte, bindAddr net.Addr) error {
	// Build reply
	response := make([]byte, 4)
	response[0] = socks5Version
	response[1] = reply
	response[2] = 0x00 // Reserved

	// Add bind address
	if bindAddr != nil {
		host, portStr, err := net.SplitHostPort(bindAddr.String())
		if err != nil {
			// Use default
			response[3] = addrIPv4
			response = append(response, 0, 0, 0, 0, 0, 0)
		} else {
			ip := net.ParseIP(host)
			if ip == nil {
				response[3] = addrIPv4
				response = append(response, 0, 0, 0, 0)
			} else if ip4 := ip.To4(); ip4 != nil {
				response[3] = addrIPv4
				response = append(response, ip4...)
			} else {
				response[3] = addrIPv6
				response = append(response, ip...)
			}

			// Add port
			var port uint16
			if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
				port = 0
			}
			portBytes := make([]byte, 2)
			binary.BigEndian.PutUint16(portBytes, port)
			response = append(response, portBytes...)
		}
	} else {
		// No bind address
		response[3] = addrIPv4
		response = append(response, 0, 0, 0, 0, 0, 0)
	}

	if _, err := conn.Write(response); err != nil {
		return fmt.Errorf("failed to write reply: %w", err)
	}

	return nil
}

// relayDataThroughCircuit relays data bidirectionally between SOCKS client and Tor circuit
// This implements the core stream protocol: reading from the SOCKS connection and sending
// RELAY_DATA cells to the circuit, and vice versa.
func (s *Server) relayDataThroughCircuit(ctx context.Context, socksConn net.Conn, circ *circuit.Circuit, strm *stream.Stream) {
	var wg sync.WaitGroup
	wg.Add(2)

	// SOCKS client -> Tor circuit (RELAY_DATA cells)
	// Read data from the SOCKS connection and send it through the circuit
	go func() {
		defer wg.Done()

		// Maximum relay cell data size (509 bytes payload - 11 bytes relay header)
		maxDataSize := 498
		buf := make([]byte, maxDataSize)

		for {
			// Set read deadline to detect idle connections
			if err := socksConn.SetReadDeadline(time.Now().Add(5 * time.Minute)); err != nil {
				s.logger.Debug("Failed to set read deadline", "error", err)
			}

			n, err := socksConn.Read(buf)
			if err != nil {
				if err != io.EOF {
					s.logger.Debug("SOCKS read error", "stream_id", strm.ID, "error", err)
				}
				// Send RELAY_END to exit node
				endReason := byte(6) // REASON_DONE
				if err := circ.EndStream(strm.ID, endReason); err != nil {
					s.logger.Debug("Failed to send RELAY_END", "stream_id", strm.ID, "error", err)
				}
				return
			}

			if n == 0 {
				continue
			}

			// Send data as RELAY_DATA cell
			if err := circ.WriteToStream(strm.ID, buf[:n]); err != nil {
				s.logger.Error("Failed to send RELAY_DATA", "stream_id", strm.ID, "error", err)
				return
			}

			s.logger.Debug("Sent data to circuit",
				"stream_id", strm.ID,
				"bytes", n)
		}
	}()

	// Tor circuit -> SOCKS client (RELAY_DATA cells)
	// Read relay cells from the circuit and write data to the SOCKS connection
	go func() {
		defer wg.Done()

		for {
			// Read data from circuit for this stream
			data, err := circ.ReadFromStream(ctx, strm.ID)
			if err != nil {
				if err == io.EOF {
					s.logger.Debug("Circuit closed", "stream_id", strm.ID)
				} else {
					s.logger.Debug("Circuit receive error", "stream_id", strm.ID, "error", err)
				}
				// Close SOCKS connection
				if err := socksConn.Close(); err != nil {
					s.logger.Debug("Failed to close SOCKS connection", "stream_id", strm.ID, "error", err)
				}
				return
			}

			// Write to SOCKS client
			if _, err := socksConn.Write(data); err != nil {
				s.logger.Error("Failed to write to SOCKS client", "stream_id", strm.ID, "error", err)
				// Send RELAY_END to exit node
				endReason := byte(6) // REASON_DONE
				if err := circ.EndStream(strm.ID, endReason); err != nil {
					s.logger.Debug("Failed to send RELAY_END", "stream_id", strm.ID, "error", err)
				}
				return
			}

			s.logger.Debug("Sent data to SOCKS client",
				"stream_id", strm.ID,
				"bytes", len(data))
		}
	}()

	// Wait for both goroutines to finish
	wg.Wait()

	s.logger.Info("Data relay finished", "stream_id", strm.ID)
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	var err error
	s.shutdownOnce.Do(func() {
		s.logger.Info("Shutting down SOCKS5 server")
		close(s.shutdown)

		// Close listener
		s.closeListener.Do(func() {
			if s.listener != nil {
				if err := s.listener.Close(); err != nil {
					s.logger.Error("Failed to close listener", "function", "Shutdown", "error", err)
				}
			}
		})

		// Close active connections
		s.mu.Lock()
		for conn := range s.activeConns {
			if err := conn.Close(); err != nil {
				s.logger.Error("Failed to close active connection", "function", "Shutdown", "error", err)
			}
		}
		s.mu.Unlock()

		s.logger.Info("SOCKS5 server shutdown complete")
	})
	return err
}

// Address returns the server address
func (s *Server) Address() string {
	return s.address
}

// handleResolve handles SOCKS5 RESOLVE command (0xF0)
// Resolves a hostname to IP address(es) through the Tor network
func (s *Server) handleResolve(ctx context.Context, conn net.Conn, hostname string) {
	s.logger.Info("DNS RESOLVE request", "hostname", hostname)

	// Set timeout for DNS resolution
	resolveCtx, cancel := context.WithTimeout(ctx, s.config.DNSTimeout)
	defer cancel()

	// For now, we'll use a simplified approach:
	// Create a temporary stream through a circuit and request DNS resolution
	// In a full implementation, this would use Tor's RELAY_RESOLVE cell type

	// Get or create a circuit for the resolution
	s.mu.Lock()
	circuitPool := s.circuitPool
	s.mu.Unlock()

	if circuitPool == nil {
		s.logger.Error("No circuit pool available for DNS resolution")
		s.sendDNSReply(conn, replyGeneralFailure, nil)
		return
	}

	circ, err := circuitPool.Get(resolveCtx)
	if err != nil || circ == nil {
		s.logger.Error("Failed to get circuit for DNS resolution", "error", err)
		s.sendDNSReply(conn, replyGeneralFailure, nil)
		return
	}
	defer circuitPool.Put(circ)

	// For DNS resolution through Tor, we would normally:
	// 1. Send a RELAY_RESOLVE cell through the circuit
	// 2. Wait for RELAY_RESOLVED response
	// 3. Parse the IP addresses from the response

	// Since we don't have direct RELAY_RESOLVE cell support yet,
	// this is a placeholder implementation that accepts the command
	// but returns an error until RELAY_RESOLVE cells are implemented

	s.logger.Warn("DNS RESOLVE accepted but RELAY_RESOLVE cells not yet implemented",
		"hostname", hostname,
		"circuit_id", circ.ID)

	// Send error response indicating feature is not fully implemented
	s.sendDNSReply(conn, replyGeneralFailure, nil)
	s.logger.Info("DNS RESOLVE completed with error (RELAY_RESOLVE cells needed)")
}

// handleResolvePTR handles SOCKS5 RESOLVE_PTR command (0xF1)
// Performs reverse DNS lookup (IP to hostname) through the Tor network
func (s *Server) handleResolvePTR(ctx context.Context, conn net.Conn, ipAddr string) {
	s.logger.Info("DNS RESOLVE_PTR request", "ip", ipAddr)

	// Set timeout for DNS resolution
	resolveCtx, cancel := context.WithTimeout(ctx, s.config.DNSTimeout)
	defer cancel()

	// Validate IP address
	ip := net.ParseIP(ipAddr)
	if ip == nil {
		s.logger.Error("Invalid IP address for RESOLVE_PTR", "ip", ipAddr)
		s.sendDNSReply(conn, replyAddressNotSupported, nil)
		return
	}

	// Get or create a circuit for the resolution
	s.mu.Lock()
	circuitPool := s.circuitPool
	s.mu.Unlock()

	if circuitPool == nil {
		s.logger.Error("No circuit pool available for reverse DNS")
		s.sendDNSReply(conn, replyGeneralFailure, nil)
		return
	}

	circ, err := circuitPool.Get(resolveCtx)
	if err != nil || circ == nil {
		s.logger.Error("Failed to get circuit for reverse DNS", "error", err)
		s.sendDNSReply(conn, replyGeneralFailure, nil)
		return
	}
	defer circuitPool.Put(circ)

	// For reverse DNS through Tor, we would normally:
	// 1. Send a RELAY_RESOLVE cell with PTR flag through the circuit
	// 2. Wait for RELAY_RESOLVED response with hostname
	// 3. Parse the hostname from the response

	s.logger.Warn("DNS RESOLVE_PTR accepted but RELAY_RESOLVE cells not yet implemented",
		"ip", ipAddr,
		"circuit_id", circ.ID)

	// Send error response indicating feature is not fully implemented
	s.sendDNSReply(conn, replyGeneralFailure, nil)
	s.logger.Info("DNS RESOLVE_PTR completed with error (RELAY_RESOLVE cells needed)")
}

// sendDNSReply sends a DNS resolution reply (for RESOLVE/RESOLVE_PTR)
// Format: [version][status][reserved][address_type][address][ttl]
//
// Note: Currently returns only the first address from the addresses slice.
// The Tor SOCKS5 extension for DNS does not have a standard way to return
// multiple addresses. Applications should make multiple RESOLVE requests if needed.
func (s *Server) sendDNSReply(conn net.Conn, status byte, addresses []net.IP) error {
	// Build basic reply header
	response := make([]byte, 4)
	response[0] = socks5Version
	response[1] = status
	response[2] = 0x00 // Reserved

	if status != replySuccess || len(addresses) == 0 {
		// Error response - no address
		response[3] = addrIPv4
		response = append(response, 0, 0, 0, 0) // Null IPv4
		response = append(response, 0, 0, 0, 0) // TTL = 0
	} else {
		// Success response with first address
		// (Multiple addresses would require extensions)
		ip := addresses[0]
		if ip4 := ip.To4(); ip4 != nil {
			response[3] = addrIPv4
			response = append(response, ip4...)
		} else {
			response[3] = addrIPv6
			response = append(response, ip...)
		}
		// Add TTL (4 bytes, big endian) - use 3600 seconds (1 hour) as default
		ttl := uint32(3600)
		ttlBytes := make([]byte, 4)
		binary.BigEndian.PutUint32(ttlBytes, ttl)
		response = append(response, ttlBytes...)
	}

	_, err := conn.Write(response)
	return err
}

// ListenerAddr returns the actual listener address once the server is ready.
// This method blocks until the listener is initialized.
func (s *Server) ListenerAddr() net.Addr {
	// Wait for listener to be ready
	<-s.listenerReady

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.listener != nil {
		return s.listener.Addr()
	}
	return nil
}
