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

// Config holds configuration for the SOCKS5 server
type Config struct {
	// MaxConnections limits concurrent SOCKS5 connections
	// SEC-L006: Configurable for resource-constrained embedded systems
	// Set to 0 for unlimited (not recommended for production)
	MaxConnections int

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
	targetAddr, err := s.readRequest(conn)
	if err != nil {
		s.logger.Error("Failed to read request", "error", err, "remote", conn.RemoteAddr())
		return
	}

	s.logger.Info("SOCKS5 request", "target", targetAddr, "remote", conn.RemoteAddr(), "username", username)

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
	if circuitPool != nil && isolationKey != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		circ, err := circuitPool.GetWithIsolation(ctx, isolationKey)
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
	}

	// Send success reply
	s.sendReply(conn, replySuccess, conn.LocalAddr())

	s.logger.Info("Connection established", "target", targetAddr, "username", username, "isolation", isolationKey)

	// Relay data (simplified - just close for now)
	// In production: relay between conn and circuit stream using the isolated circuit
	time.Sleep(100 * time.Millisecond)
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

// readRequest reads a SOCKS5 request
func (s *Server) readRequest(conn net.Conn) (string, error) {
	// Read request header
	header := make([]byte, 4)
	if _, err := io.ReadFull(conn, header); err != nil {
		return "", fmt.Errorf("failed to read request header: %w", err)
	}

	version := header[0]
	cmd := header[1]
	// reserved := header[2]
	addrType := header[3]

	if version != socks5Version {
		s.sendReply(conn, replyGeneralFailure, nil)
		return "", fmt.Errorf("unsupported SOCKS version: %d", version)
	}

	if cmd != cmdConnect {
		s.sendReply(conn, replyCommandNotSupported, nil)
		return "", fmt.Errorf("unsupported command: %d", cmd)
	}

	// Read address
	var addr string
	switch addrType {
	case addrIPv4:
		ip := make([]byte, 4)
		if _, err := io.ReadFull(conn, ip); err != nil {
			s.sendReply(conn, replyGeneralFailure, nil)
			return "", fmt.Errorf("failed to read IPv4 address: %w", err)
		}
		addr = net.IP(ip).String()

	case addrDomain:
		domainLen := make([]byte, 1)
		if _, err := io.ReadFull(conn, domainLen); err != nil {
			s.sendReply(conn, replyGeneralFailure, nil)
			return "", fmt.Errorf("failed to read domain length: %w", err)
		}

		domain := make([]byte, domainLen[0])
		if _, err := io.ReadFull(conn, domain); err != nil {
			s.sendReply(conn, replyGeneralFailure, nil)
			return "", fmt.Errorf("failed to read domain: %w", err)
		}
		addr = string(domain)

	case addrIPv6:
		ip := make([]byte, 16)
		if _, err := io.ReadFull(conn, ip); err != nil {
			s.sendReply(conn, replyGeneralFailure, nil)
			return "", fmt.Errorf("failed to read IPv6 address: %w", err)
		}
		addr = net.IP(ip).String()

	default:
		s.sendReply(conn, replyAddressNotSupported, nil)
		return "", fmt.Errorf("unsupported address type: %d", addrType)
	}

	// Read port
	portBytes := make([]byte, 2)
	if _, err := io.ReadFull(conn, portBytes); err != nil {
		s.sendReply(conn, replyGeneralFailure, nil)
		return "", fmt.Errorf("failed to read port: %w", err)
	}
	port := binary.BigEndian.Uint16(portBytes)

	return fmt.Sprintf("%s:%d", addr, port), nil
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
