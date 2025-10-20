// Package protocol provides core Tor protocol functionality.
// This package implements version negotiation and link protocol handshake.
package protocol

import (
	"context"
	"fmt"
	"time"

	"github.com/opd-ai/go-tor/pkg/cell"
	"github.com/opd-ai/go-tor/pkg/connection"
	"github.com/opd-ai/go-tor/pkg/logger"
	"github.com/opd-ai/go-tor/pkg/security"
)

// Protocol versions supported by this implementation
const (
	MinLinkProtocolVersion = 3
	MaxLinkProtocolVersion = 5
	PreferredVersion       = 4 // Link protocol v4 uses 4-byte circuit IDs

	// DefaultHandshakeTimeout is the default timeout for protocol handshake (SEC-009)
	DefaultHandshakeTimeout = 10 * time.Second
)

// Handshake performs the Tor protocol handshake on a connection
type Handshake struct {
	conn              *connection.Connection
	negotiatedVersion int
	logger            *logger.Logger
	timeout           time.Duration // Configurable handshake timeout (SEC-009)
}

// NewHandshake creates a new handshake instance
func NewHandshake(conn *connection.Connection, log *logger.Logger) *Handshake {
	if log == nil {
		log = logger.NewDefault()
	}
	return &Handshake{
		conn:    conn,
		logger:  log,
		timeout: DefaultHandshakeTimeout, // Use default, can be overridden with SetTimeout
	}
}

// SetTimeout sets the handshake timeout (SEC-009)
// This allows configuring shorter timeouts for embedded systems
func (h *Handshake) SetTimeout(timeout time.Duration) {
	h.timeout = timeout
}

// PerformHandshake performs the version negotiation handshake
func (h *Handshake) PerformHandshake(ctx context.Context) error {
	h.logger.Info("Starting protocol handshake")

	// Send VERSIONS cell
	if err := h.sendVersions(); err != nil {
		return fmt.Errorf("failed to send VERSIONS: %w", err)
	}

	// Receive VERSIONS response
	if err := h.receiveVersions(ctx); err != nil {
		return fmt.Errorf("failed to receive VERSIONS: %w", err)
	}

	// Send NETINFO cell
	if err := h.sendNetinfo(); err != nil {
		return fmt.Errorf("failed to send NETINFO: %w", err)
	}

	// Receive NETINFO response
	if err := h.receiveNetinfo(ctx); err != nil {
		return fmt.Errorf("failed to receive NETINFO: %w", err)
	}

	h.logger.Info("Protocol handshake complete", "version", h.negotiatedVersion)
	return nil
}

// sendVersions sends a VERSIONS cell with supported versions
func (h *Handshake) sendVersions() error {
	// VERSIONS cell payload: 2 bytes per version (big-endian)
	versions := []uint16{
		MinLinkProtocolVersion,
		PreferredVersion,
		MaxLinkProtocolVersion,
	}

	payload := make([]byte, len(versions)*2)
	for i, v := range versions {
		payload[i*2] = byte(v >> 8)
		payload[i*2+1] = byte(v)
	}

	versionsCell := cell.NewCell(0, cell.CmdVersions)
	versionsCell.Payload = payload

	h.logger.Debug("Sending VERSIONS cell", "versions", versions)
	return h.conn.SendCell(versionsCell)
}

// receiveVersions receives and processes the VERSIONS response
func (h *Handshake) receiveVersions(ctx context.Context) error {
	// Set a timeout for receiving (SEC-009: configurable timeout)
	timer := time.NewTimer(h.timeout)
	defer timer.Stop()

	cellCh := make(chan *cell.Cell, 1)
	errCh := make(chan error, 1)

	go func() {
		receivedCell, err := h.conn.ReceiveCell()
		if err != nil {
			errCh <- err
			return
		}
		cellCh <- receivedCell
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return fmt.Errorf("timeout waiting for VERSIONS response")
	case err := <-errCh:
		return err
	case receivedCell := <-cellCh:
		if receivedCell.Command != cell.CmdVersions {
			return fmt.Errorf("expected VERSIONS cell, got %s", receivedCell.Command)
		}

		// Parse versions from payload
		if len(receivedCell.Payload)%2 != 0 {
			return fmt.Errorf("invalid VERSIONS payload length: %d", len(receivedCell.Payload))
		}

		var versions []int
		for i := 0; i < len(receivedCell.Payload); i += 2 {
			version := int(receivedCell.Payload[i])<<8 | int(receivedCell.Payload[i+1])
			versions = append(versions, version)
		}

		h.logger.Debug("Received VERSIONS cell", "versions", versions)

		// Select highest mutually supported version
		h.negotiatedVersion = h.selectVersion(versions)
		if h.negotiatedVersion == 0 {
			return fmt.Errorf("no compatible protocol version")
		}

		h.logger.Info("Negotiated protocol version", "version", h.negotiatedVersion)
		return nil
	}
}

// selectVersion selects the highest mutually supported version
func (h *Handshake) selectVersion(remoteVersions []int) int {
	for v := MaxLinkProtocolVersion; v >= MinLinkProtocolVersion; v-- {
		for _, remote := range remoteVersions {
			if remote == v {
				return v
			}
		}
	}
	return 0
}

// sendNetinfo sends a NETINFO cell
func (h *Handshake) sendNetinfo() error {
	// Simplified NETINFO cell for now
	// Format: timestamp (4 bytes) + other address (various) + this address (various)
	payload := make([]byte, 512) // Use fixed size, will be padded

	// Timestamp (current time in seconds since epoch)
	// Safely convert to uint32 (will fail if timestamp exceeds uint32 max in year 2106)
	now := time.Now()
	timestamp, err := security.SafeUnixToUint32(now)
	if err != nil {
		// Log warning but continue with 0 timestamp if conversion fails
		h.logger.Warn("Failed to convert timestamp to uint32, using 0", "error", err)
		timestamp = 0
	}
	payload[0] = byte(timestamp >> 24)
	payload[1] = byte(timestamp >> 16)
	payload[2] = byte(timestamp >> 8)
	payload[3] = byte(timestamp)

	// For simplicity, we'll use minimal address info
	// Other address type: 0x04 (IPv4), 4 bytes, 0.0.0.0
	payload[4] = 0x04 // IPv4
	payload[5] = 4    // 4 bytes
	// payload[6:10] already zeros

	// Number of this addresses: 0
	payload[10] = 0

	netinfoCell := cell.NewCell(0, cell.CmdNetinfo)
	netinfoCell.Payload = payload[:11]

	h.logger.Debug("Sending NETINFO cell")
	return h.conn.SendCell(netinfoCell)
}

// receiveNetinfo receives and validates the NETINFO response
func (h *Handshake) receiveNetinfo(ctx context.Context) error {
	timer := time.NewTimer(h.timeout)
	defer timer.Stop()

	cellCh := make(chan *cell.Cell, 1)
	errCh := make(chan error, 1)

	go func() {
		receivedCell, err := h.conn.ReceiveCell()
		if err != nil {
			errCh <- err
			return
		}
		cellCh <- receivedCell
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return fmt.Errorf("timeout waiting for NETINFO response")
	case err := <-errCh:
		return err
	case receivedCell := <-cellCh:
		if receivedCell.Command != cell.CmdNetinfo {
			return fmt.Errorf("expected NETINFO cell, got %s", receivedCell.Command)
		}

		h.logger.Debug("Received NETINFO cell")
		// For now, just validate we received it
		// Full parsing would extract timestamp and addresses
		return nil
	}
}

// NegotiatedVersion returns the negotiated protocol version
func (h *Handshake) NegotiatedVersion() int {
	return h.negotiatedVersion
}
