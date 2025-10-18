// Package socks provides SOCKS5 proxy server functionality.
// This package implements a SOCKS5 server that routes connections through Tor circuits.
package socks

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/opd-ai/go-tor/pkg/circuit"
	"github.com/opd-ai/go-tor/pkg/logger"
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
)

// Server is a SOCKS5 proxy server
type Server struct {
	address       string
	listener      net.Listener
	circuitMgr    *circuit.Manager
	logger        *logger.Logger
	mu            sync.Mutex
	activeConns   map[net.Conn]struct{}
	shutdown      chan struct{}
	shutdownOnce  sync.Once
	closeListener sync.Once
}

// NewServer creates a new SOCKS5 proxy server
func NewServer(address string, circuitMgr *circuit.Manager, log *logger.Logger) *Server {
	if log == nil {
		log = logger.NewDefault()
	}

	return &Server{
		address:     address,
		circuitMgr:  circuitMgr,
		logger:      log.Component("socks5"),
		activeConns: make(map[net.Conn]struct{}),
		shutdown:    make(chan struct{}),
	}
}

// ListenAndServe starts the SOCKS5 server
func (s *Server) ListenAndServe(ctx context.Context) error {
	s.logger.Info("Starting SOCKS5 server", "address", s.address)

	listener, err := net.Listen("tcp", s.address)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	s.listener = listener

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

		// Track connection
		s.mu.Lock()
		s.activeConns[conn] = struct{}{}
		s.mu.Unlock()

		// Handle connection
		go s.handleConnection(ctx, conn)
	}
}

// handleConnection handles a SOCKS5 connection
func (s *Server) handleConnection(ctx context.Context, conn net.Conn) {
	defer func() {
		conn.Close()
		s.mu.Lock()
		delete(s.activeConns, conn)
		s.mu.Unlock()
	}()

	// Set read deadline
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))

	// Handshake
	if err := s.handshake(conn); err != nil {
		s.logger.Error("Handshake failed", "error", err, "remote", conn.RemoteAddr())
		return
	}

	// Read request
	targetAddr, err := s.readRequest(conn)
	if err != nil {
		s.logger.Error("Failed to read request", "error", err, "remote", conn.RemoteAddr())
		return
	}

	s.logger.Info("SOCKS5 request", "target", targetAddr, "remote", conn.RemoteAddr())

	// For now, return success without actually routing through Tor
	// In a full implementation, this would:
	// 1. Select or create a circuit
	// 2. Open a stream through the circuit
	// 3. Relay data between client and stream
	s.sendReply(conn, replySuccess, conn.LocalAddr())

	s.logger.Info("Connection established", "target", targetAddr)

	// Relay data (simplified - just close for now)
	// In production: relay between conn and circuit stream
	time.Sleep(100 * time.Millisecond)
}

// handshake performs SOCKS5 handshake
func (s *Server) handshake(conn net.Conn) error {
	// Read version and methods
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		return fmt.Errorf("failed to read handshake header: %w", err)
	}

	version := header[0]
	nmethods := header[1]

	if version != socks5Version {
		return fmt.Errorf("unsupported SOCKS version: %d", version)
	}

	// Read methods
	methods := make([]byte, nmethods)
	if _, err := io.ReadFull(conn, methods); err != nil {
		return fmt.Errorf("failed to read methods: %w", err)
	}

	// We only support no authentication
	supportsNoAuth := false
	for _, method := range methods {
		if method == authNone {
			supportsNoAuth = true
			break
		}
	}

	// Send method selection
	response := []byte{socks5Version, authNone}
	if !supportsNoAuth {
		response[1] = authNoAccept
	}

	if _, err := conn.Write(response); err != nil {
		return fmt.Errorf("failed to write method selection: %w", err)
	}

	if !supportsNoAuth {
		return fmt.Errorf("no acceptable authentication methods")
	}

	return nil
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
			fmt.Sscanf(portStr, "%d", &port)
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
				s.listener.Close()
			}
		})

		// Close active connections
		s.mu.Lock()
		for conn := range s.activeConns {
			conn.Close()
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
