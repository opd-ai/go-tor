package relay

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/opd-ai/go-tor/pkg/logger"
)

// ORListener listens for incoming OR (Onion Router) connections
type ORListener struct {
	address  string
	keys     *RelayKeys
	listener net.Listener
	logger   *logger.Logger

	// Connection management
	connsMu     sync.RWMutex
	connections map[string]*ORConnection
	stopCh      chan struct{}
	wg          sync.WaitGroup

	// Configuration
	maxConnections int
	readTimeout    time.Duration
	writeTimeout   time.Duration
}

// ORListenerConfig holds configuration for the OR listener
type ORListenerConfig struct {
	Address        string        // Address to listen on (e.g., ":9001")
	Keys           *RelayKeys    // Relay identity keys
	MaxConnections int           // Maximum concurrent connections (0 = unlimited)
	ReadTimeout    time.Duration // Per-connection read timeout
	WriteTimeout   time.Duration // Per-connection write timeout
}

// DefaultORListenerConfig returns default configuration
func DefaultORListenerConfig(address string, keys *RelayKeys) *ORListenerConfig {
	return &ORListenerConfig{
		Address:        address,
		Keys:           keys,
		MaxConnections: 1000,
		ReadTimeout:    60 * time.Second,
		WriteTimeout:   60 * time.Second,
	}
}

// NewORListener creates a new OR listener
func NewORListener(cfg *ORListenerConfig, log *logger.Logger) (*ORListener, error) {
	if cfg.Keys == nil {
		return nil, fmt.Errorf("relay keys are required")
	}

	if log == nil {
		log = logger.NewDefault()
	}

	return &ORListener{
		address:        cfg.Address,
		keys:           cfg.Keys,
		logger:         log,
		connections:    make(map[string]*ORConnection),
		stopCh:         make(chan struct{}),
		maxConnections: cfg.MaxConnections,
		readTimeout:    cfg.ReadTimeout,
		writeTimeout:   cfg.WriteTimeout,
	}, nil
}

// Start begins listening for OR connections
func (l *ORListener) Start(ctx context.Context) error {
	// Create TLS config for server
	tlsConfig, err := l.createServerTLSConfig()
	if err != nil {
		return fmt.Errorf("failed to create TLS config: %w", err)
	}

	// Start TCP listener
	listener, err := net.Listen("tcp", l.address)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", l.address, err)
	}

	// Wrap with TLS
	l.listener = tls.NewListener(listener, tlsConfig)
	l.logger.Info("OR listener started", "address", l.address, "fingerprint", l.keys.Fingerprint())

	// Accept connections in goroutine
	l.wg.Add(1)
	go l.acceptLoop(ctx)

	return nil
}

// createServerTLSConfig creates a TLS config for the OR server
// Per tor-spec.txt §1.1, the server presents a self-signed certificate
func (l *ORListener) createServerTLSConfig() (*tls.Config, error) {
	// Parse certificate
	cert, err := x509.ParseCertificate(l.keys.TLSCert)
	if err != nil {
		return nil, fmt.Errorf("failed to parse TLS certificate: %w", err)
	}

	// Create tls.Certificate from DER cert and private key
	tlsCert := tls.Certificate{
		Certificate: [][]byte{l.keys.TLSCert},
		PrivateKey:  l.keys.RSAPrivate,
		Leaf:        cert,
	}

	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		MinVersion:   tls.VersionTLS12,
		// Use secure cipher suites (AEAD with forward secrecy)
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
		},
		// Don't require client certificates
		ClientAuth: tls.NoClientCert,
	}, nil
}

// acceptLoop accepts incoming connections
func (l *ORListener) acceptLoop(ctx context.Context) {
	defer l.wg.Done()

	for {
		select {
		case <-ctx.Done():
			l.logger.Info("Accept loop stopping (context cancelled)")
			return
		case <-l.stopCh:
			l.logger.Info("Accept loop stopping (stop signal)")
			return
		default:
		}

		conn, err := l.listener.Accept()
		if err != nil {
			// Check if listener was closed
			select {
			case <-l.stopCh:
				return
			case <-ctx.Done():
				return
			default:
			}

			l.logger.Warn("Failed to accept connection", "error", err)
			time.Sleep(10 * time.Millisecond) // Brief pause to prevent tight loop on errors
			continue
		}

		// Check connection limit
		if l.maxConnections > 0 {
			l.connsMu.RLock()
			connCount := len(l.connections)
			l.connsMu.RUnlock()

			if connCount >= l.maxConnections {
				l.logger.Warn("Connection limit reached, rejecting", "remote", conn.RemoteAddr())
				conn.Close()
				continue
			}
		}

		// Handle connection
		l.wg.Add(1)
		go l.handleConnection(ctx, conn)
	}
}

// handleConnection handles a single OR connection
func (l *ORListener) handleConnection(ctx context.Context, rawConn net.Conn) {
	defer l.wg.Done()

	remoteAddr := rawConn.RemoteAddr().String()
	l.logger.Info("New OR connection", "remote", remoteAddr)

	// Ensure cleanup
	defer func() {
		rawConn.Close()
		l.logger.Info("OR connection closed", "remote", remoteAddr)
	}()

	// Complete TLS handshake
	if tlsConn, ok := rawConn.(*tls.Conn); ok {
		if err := tlsConn.Handshake(); err != nil {
			l.logger.Warn("TLS handshake failed", "remote", remoteAddr, "error", err)
			return
		}
	}

	// Perform link protocol handshake
	linkHandler := NewLinkProtocolHandler(l.keys, l.logger)
	serverConn, err := linkHandler.HandleConnection(ctx, rawConn)
	if err != nil {
		l.logger.Warn("Link protocol handshake failed", "remote", remoteAddr, "error", err)
		return
	}

	// Create full OR connection wrapper
	orConn := &ORConnection{
		conn:       serverConn.conn,
		remoteAddr: remoteAddr,
		logger:     l.logger,
	}

	// Register connection
	l.connsMu.Lock()
	l.connections[remoteAddr] = orConn
	l.connsMu.Unlock()

	// Ensure cleanup from registry
	defer func() {
		l.connsMu.Lock()
		delete(l.connections, remoteAddr)
		l.connsMu.Unlock()
	}()

	l.logger.Info("Link protocol handshake complete", "remote", remoteAddr, "version", serverConn.negotiatedVersion)

	// Keep connection alive until context cancels or stop signal
	// TODO: Handle circuit management (Task 10.1.3)
	select {
	case <-ctx.Done():
		return
	case <-l.stopCh:
		return
	}
}

// Stop stops the OR listener and closes all connections
func (l *ORListener) Stop() error {
	l.logger.Info("Stopping OR listener")

	// Signal stop
	close(l.stopCh)

	// Close listener
	if l.listener != nil {
		l.listener.Close()
	}

	// Close all active connections
	l.connsMu.Lock()
	for _, conn := range l.connections {
		conn.Close()
	}
	l.connsMu.Unlock()

	// Wait for goroutines
	l.wg.Wait()

	l.logger.Info("OR listener stopped")
	return nil
}

// ConnectionCount returns the number of active connections
func (l *ORListener) ConnectionCount() int {
	l.connsMu.RLock()
	defer l.connsMu.RUnlock()
	return len(l.connections)
}

// ORConnection represents a single OR connection
type ORConnection struct {
	conn         net.Conn
	remoteAddr   string
	logger       *logger.Logger
	readTimeout  time.Duration
	writeTimeout time.Duration
	mu           sync.Mutex
}

// Close closes the connection
func (c *ORConnection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// RemoteAddr returns the remote address
func (c *ORConnection) RemoteAddr() string {
	return c.remoteAddr
}
