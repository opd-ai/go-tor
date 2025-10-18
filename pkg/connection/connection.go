// Package connection provides TLS connection handling for Tor relays.
// This package manages connections to Tor relays and handles cell I/O.
package connection

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/opd-ai/go-tor/pkg/cell"
	"github.com/opd-ai/go-tor/pkg/logger"
)

// State represents the connection state
type State int

const (
	// StateConnecting indicates the connection is being established
	StateConnecting State = iota
	// StateHandshaking indicates TLS handshake is in progress
	StateHandshaking
	// StateOpen indicates the connection is ready for use
	StateOpen
	// StateClosed indicates the connection has been closed
	StateClosed
	// StateFailed indicates the connection failed
	StateFailed
)

// String returns a string representation of the state
func (s State) String() string {
	switch s {
	case StateConnecting:
		return "CONNECTING"
	case StateHandshaking:
		return "HANDSHAKING"
	case StateOpen:
		return "OPEN"
	case StateClosed:
		return "CLOSED"
	case StateFailed:
		return "FAILED"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", s)
	}
}

// Connection represents a TLS connection to a Tor relay
type Connection struct {
	address   string
	conn      net.Conn
	tlsConn   *tls.Conn
	state     State
	stateMu   sync.RWMutex
	closeCh   chan struct{}
	closeOnce sync.Once
	sendMu    sync.Mutex
	recvMu    sync.Mutex
	logger    *logger.Logger
}

// Config holds connection configuration
type Config struct {
	Address        string        // Relay address (IP:port)
	Timeout        time.Duration // Connection timeout
	TLSConfig      *tls.Config   // TLS configuration
	LinkProtocolV4 bool          // Use link protocol v4 (4-byte circuit IDs)
}

// DefaultConfig returns a connection config with sensible defaults
func DefaultConfig(address string) *Config {
	return &Config{
		Address:        address,
		Timeout:        30 * time.Second,
		TLSConfig:      &tls.Config{InsecureSkipVerify: true}, // TODO: Implement proper cert validation
		LinkProtocolV4: true,
	}
}

// New creates a new connection to a Tor relay
func New(cfg *Config, log *logger.Logger) *Connection {
	if log == nil {
		log = logger.NewDefault()
	}

	return &Connection{
		address: cfg.Address,
		state:   StateConnecting,
		closeCh: make(chan struct{}),
		logger:  log.With("address", cfg.Address),
	}
}

// Connect establishes a TLS connection to the relay
func (c *Connection) Connect(ctx context.Context, cfg *Config) error {
	c.logger.Debug("Connecting to relay")

	// Create dialer with timeout
	dialer := &net.Dialer{
		Timeout: cfg.Timeout,
	}

	// Establish TCP connection
	conn, err := dialer.DialContext(ctx, "tcp", cfg.Address)
	if err != nil {
		c.setState(StateFailed)
		return fmt.Errorf("failed to connect: %w", err)
	}
	c.conn = conn

	// Upgrade to TLS
	c.setState(StateHandshaking)
	c.logger.Debug("Starting TLS handshake")

	tlsConn := tls.Client(conn, cfg.TLSConfig)
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		conn.Close()
		c.setState(StateFailed)
		return fmt.Errorf("TLS handshake failed: %w", err)
	}
	c.tlsConn = tlsConn

	c.setState(StateOpen)
	c.logger.Info("Connection established")

	return nil
}

// SendCell sends a cell over the connection
func (c *Connection) SendCell(cell *cell.Cell) error {
	c.sendMu.Lock()
	defer c.sendMu.Unlock()

	if c.getState() != StateOpen {
		return fmt.Errorf("connection not open: %s", c.getState())
	}

	select {
	case <-c.closeCh:
		return fmt.Errorf("connection closed")
	default:
	}

	if err := cell.Encode(c.tlsConn); err != nil {
		c.logger.Error("Failed to send cell", "error", err, "command", cell.Command)
		return fmt.Errorf("failed to send cell: %w", err)
	}

	c.logger.Debug("Sent cell", "command", cell.Command, "circuit_id", cell.CircID)
	return nil
}

// ReceiveCell receives a cell from the connection
func (c *Connection) ReceiveCell() (*cell.Cell, error) {
	c.recvMu.Lock()
	defer c.recvMu.Unlock()

	if c.getState() != StateOpen {
		return nil, fmt.Errorf("connection not open: %s", c.getState())
	}

	select {
	case <-c.closeCh:
		return nil, fmt.Errorf("connection closed")
	default:
	}

	receivedCell, err := cell.DecodeCell(c.tlsConn)
	if err != nil {
		if err == io.EOF {
			c.logger.Info("Connection closed by remote")
			c.Close()
			return nil, err
		}
		c.logger.Error("Failed to receive cell", "error", err)
		return nil, fmt.Errorf("failed to receive cell: %w", err)
	}

	c.logger.Debug("Received cell", "command", receivedCell.Command, "circuit_id", receivedCell.CircID)
	return receivedCell, nil
}

// Close closes the connection gracefully
func (c *Connection) Close() error {
	var err error
	c.closeOnce.Do(func() {
		close(c.closeCh)
		c.setState(StateClosed)

		if c.tlsConn != nil {
			if closeErr := c.tlsConn.Close(); closeErr != nil {
				err = fmt.Errorf("failed to close TLS connection: %w", closeErr)
			}
		} else if c.conn != nil {
			if closeErr := c.conn.Close(); closeErr != nil {
				err = fmt.Errorf("failed to close connection: %w", closeErr)
			}
		}

		c.logger.Info("Connection closed")
	})
	return err
}

// IsOpen returns true if the connection is open
func (c *Connection) IsOpen() bool {
	return c.getState() == StateOpen
}

// Address returns the relay address
func (c *Connection) Address() string {
	return c.address
}

// setState sets the connection state
func (c *Connection) setState(state State) {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	c.state = state
}

// getState returns the current connection state
func (c *Connection) getState() State {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	return c.state
}

// GetState returns the current connection state (exported)
func (c *Connection) GetState() State {
	return c.getState()
}
