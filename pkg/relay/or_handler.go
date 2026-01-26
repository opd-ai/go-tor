package relay

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/binary"
	"fmt"
	"net"
	"time"

	"github.com/opd-ai/go-tor/pkg/cell"
	"github.com/opd-ai/go-tor/pkg/logger"
	"github.com/opd-ai/go-tor/pkg/security"
)

// ServerORConnection holds server-side OR connection state
// This extends the basic ORConnection with protocol state
type ServerORConnection struct {
	conn              net.Conn
	remoteAddr        string
	negotiatedVersion int
	authenticated     bool
}

// LinkProtocolHandler handles the server-side link protocol handshake
// Following tor-spec.txt §1-2 (server-side)
type LinkProtocolHandler struct {
	keys   *RelayKeys
	logger *logger.Logger
}

// NewLinkProtocolHandler creates a new link protocol handler
func NewLinkProtocolHandler(keys *RelayKeys, log *logger.Logger) *LinkProtocolHandler {
	if log == nil {
		log = logger.NewDefault()
	}
	return &LinkProtocolHandler{
		keys:   keys,
		logger: log,
	}
}

// HandleConnection performs the server-side link protocol handshake
// Per tor-spec.txt §2, the server:
// 1. Receives VERSIONS from client
// 2. Sends VERSIONS response
// 3. Sends CERTS cell with identity certificates
// 4. Sends AUTH_CHALLENGE (optional)
// 5. Sends NETINFO
// 6. Receives NETINFO from client
func (h *LinkProtocolHandler) HandleConnection(ctx context.Context, conn net.Conn) (*ServerORConnection, error) {
	orConn := &ServerORConnection{
		conn:       conn,
		remoteAddr: conn.RemoteAddr().String(),
	}

	// Step 1: Receive VERSIONS from client
	clientVersions, err := h.receiveVersions(ctx, conn)
	if err != nil {
		return nil, fmt.Errorf("failed to receive client VERSIONS: %w", err)
	}

	// Step 2: Negotiate protocol version
	version := h.selectVersion(clientVersions)
	if version == 0 {
		return nil, fmt.Errorf("no compatible protocol version")
	}
	orConn.negotiatedVersion = version
	h.logger.Info("Negotiated protocol version", "version", version, "client_versions", clientVersions)

	// Send VERSIONS response
	if err := h.sendVersions(conn); err != nil {
		return nil, fmt.Errorf("failed to send VERSIONS: %w", err)
	}

	// Step 3: Send CERTS cell
	if err := h.sendCerts(conn); err != nil {
		return nil, fmt.Errorf("failed to send CERTS: %w", err)
	}

	// Step 4: Send AUTH_CHALLENGE (optional, not implemented yet)
	// This would be needed for client authentication

	// Step 5: Send NETINFO
	if err := h.sendNetinfo(conn); err != nil {
		return nil, fmt.Errorf("failed to send NETINFO: %w", err)
	}

	// Step 6: Receive NETINFO from client
	if err := h.receiveNetinfo(ctx, conn); err != nil {
		return nil, fmt.Errorf("failed to receive client NETINFO: %w", err)
	}

	h.logger.Info("Link protocol handshake complete", "version", version, "remote", conn.RemoteAddr())
	return orConn, nil
}

// receiveVersions receives and parses a VERSIONS cell from the client
func (h *LinkProtocolHandler) receiveVersions(ctx context.Context, conn net.Conn) ([]int, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Read cell with context
	cellData, err := h.readCellWithContext(ctx, conn)
	if err != nil {
		return nil, err
	}

	if cellData.Command != cell.CmdVersions {
		return nil, fmt.Errorf("expected VERSIONS cell, got %s", cellData.Command)
	}

	// Parse versions from payload (2 bytes per version, big-endian)
	if len(cellData.Payload)%2 != 0 {
		return nil, fmt.Errorf("invalid VERSIONS payload length: %d", len(cellData.Payload))
	}

	var versions []int
	for i := 0; i < len(cellData.Payload); i += 2 {
		version := int(cellData.Payload[i])<<8 | int(cellData.Payload[i+1])
		versions = append(versions, version)
	}

	h.logger.Debug("Received VERSIONS cell", "versions", versions)
	return versions, nil
}

// selectVersion selects the highest mutually supported version
func (h *LinkProtocolHandler) selectVersion(clientVersions []int) int {
	// We support versions 3-5
	supportedVersions := []int{5, 4, 3}

	for _, supported := range supportedVersions {
		for _, client := range clientVersions {
			if client == supported {
				return supported
			}
		}
	}
	return 0
}

// sendVersions sends a VERSIONS cell to the client
func (h *LinkProtocolHandler) sendVersions(conn net.Conn) error {
	// Send versions 3, 4, 5
	versions := []uint16{3, 4, 5}
	payload := make([]byte, len(versions)*2)
	for i, v := range versions {
		binary.BigEndian.PutUint16(payload[i*2:], v)
	}

	versionsCell := cell.NewCell(0, cell.CmdVersions)
	versionsCell.Payload = payload

	h.logger.Debug("Sending VERSIONS cell", "versions", versions)
	return h.writeCell(conn, versionsCell)
}

// sendCerts sends a CERTS cell with the relay's identity certificates
// Per tor-spec.txt §4.2
func (h *LinkProtocolHandler) sendCerts(conn net.Conn) error {
	// Build CERTS cell payload
	// Format: N (1 byte) || N times: (CertType (1) || CLEN (2) || Certificate (CLEN))

	var payload []byte

	// Count certificates we'll send
	numCerts := 0
	var certs []struct {
		certType byte
		certData []byte
	}

	// Cert 1: TLS link certificate (type 1)
	if h.keys.TLSCert != nil {
		certs = append(certs, struct {
			certType byte
			certData []byte
		}{0x01, h.keys.TLSCert})
		numCerts++
	}

	// Cert 2: RSA identity certificate (type 2)
	// Use the same TLS cert for RSA identity (simplified approach)
	if h.keys.TLSCert != nil {
		certs = append(certs, struct {
			certType byte
			certData []byte
		}{0x02, h.keys.TLSCert})
		numCerts++
	}

	// Cert 4: Ed25519 signing key certificate (type 4)
	// Generate a simple Ed25519 cert per cert-spec.txt
	if h.keys.Ed25519Private != nil {
		ed25519Cert, err := h.buildEd25519SigningCert()
		if err != nil {
			h.logger.Warn("Failed to build Ed25519 signing cert", "error", err)
		} else {
			certs = append(certs, struct {
				certType byte
				certData []byte
			}{0x04, ed25519Cert})
			numCerts++
		}
	}

	// Build payload
	payload = append(payload, byte(numCerts))
	for _, cert := range certs {
		payload = append(payload, cert.certType)
		certLen := uint16(len(cert.certData))
		payload = append(payload, byte(certLen>>8), byte(certLen))
		payload = append(payload, cert.certData...)
	}

	certsCell := cell.NewCell(0, cell.CmdCerts)
	certsCell.Payload = payload

	h.logger.Debug("Sending CERTS cell", "num_certs", numCerts)
	return h.writeCell(conn, certsCell)
}

// buildEd25519SigningCert builds a simple Ed25519 signing certificate
// Per cert-spec.txt §2.1
func (h *LinkProtocolHandler) buildEd25519SigningCert() ([]byte, error) {
	// Build minimal Ed25519 certificate
	// Format: Version (1) || CertType (1) || ExpirationDate (4) || CertKeyType (1) ||
	//         CertifiedKey (32) || NumExtensions (1) || Signature (64)

	var cert []byte

	// Version 1
	cert = append(cert, 0x01)

	// CertType 4 (Ed25519 signing key)
	cert = append(cert, 0x04)

	// Expiration date (4 bytes, hours since epoch)
	expiresAt := time.Now().Add(365 * 24 * time.Hour) // 1 year
	hoursSinceEpoch := uint32(expiresAt.Unix() / 3600)
	cert = append(cert,
		byte(hoursSinceEpoch>>24),
		byte(hoursSinceEpoch>>16),
		byte(hoursSinceEpoch>>8),
		byte(hoursSinceEpoch))

	// CertKeyType 1 (Ed25519 key)
	cert = append(cert, 0x01)

	// Certified key (32 bytes Ed25519 public key)
	cert = append(cert, h.keys.Ed25519Public...)

	// Number of extensions (0)
	cert = append(cert, 0x00)

	// Sign the certificate body with the identity key
	signature := ed25519.Sign(h.keys.Ed25519Private, cert)
	cert = append(cert, signature...)

	return cert, nil
}

// sendNetinfo sends a NETINFO cell to the client
func (h *LinkProtocolHandler) sendNetinfo(conn net.Conn) error {
	// NETINFO format per tor-spec.txt §4.5:
	// Timestamp (4) || OtherAddress || NumAddresses (1) || Addresses

	var payload []byte

	// Timestamp (current time in seconds since epoch)
	now := time.Now()
	timestamp, err := security.SafeUnixToUint32(now)
	if err != nil {
		h.logger.Warn("Failed to convert timestamp, using 0", "error", err)
		timestamp = 0
	}
	payload = append(payload,
		byte(timestamp>>24),
		byte(timestamp>>16),
		byte(timestamp>>8),
		byte(timestamp))

	// OtherAddress (client's address as we see it)
	// Type 0x04 (IPv4) || Length || Address
	if tcpAddr, ok := conn.RemoteAddr().(*net.TCPAddr); ok && tcpAddr.IP.To4() != nil {
		payload = append(payload, 0x04) // IPv4
		payload = append(payload, 4)    // 4 bytes
		payload = append(payload, tcpAddr.IP.To4()...)
	} else {
		// Unknown address type, use 0.0.0.0
		payload = append(payload, 0x04, 4, 0, 0, 0, 0)
	}

	// NumAddresses (our addresses) - for simplicity, send 0
	payload = append(payload, 0)

	netinfoCell := cell.NewCell(0, cell.CmdNetinfo)
	netinfoCell.Payload = payload

	h.logger.Debug("Sending NETINFO cell")
	return h.writeCell(conn, netinfoCell)
}

// receiveNetinfo receives and validates the client's NETINFO cell
func (h *LinkProtocolHandler) receiveNetinfo(ctx context.Context, conn net.Conn) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cellData, err := h.readCellWithContext(ctx, conn)
	if err != nil {
		return err
	}

	if cellData.Command != cell.CmdNetinfo {
		return fmt.Errorf("expected NETINFO cell, got %s", cellData.Command)
	}

	h.logger.Debug("Received NETINFO cell")
	// For now, just validate we received it
	// Full parsing would extract timestamp and addresses
	return nil
}

// readCellWithContext reads a cell from the connection with context cancellation
func (h *LinkProtocolHandler) readCellWithContext(ctx context.Context, conn net.Conn) (*cell.Cell, error) {
	// Read header (CircID + Command)
	header := make([]byte, 5) // 4 bytes CircID + 1 byte Command

	// Create a channel for the read result
	type readResult struct {
		n   int
		err error
	}
	resultCh := make(chan readResult, 1)

	go func() {
		n, err := conn.Read(header)
		resultCh <- readResult{n, err}
	}()

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("read cancelled: %w", ctx.Err())
	case result := <-resultCh:
		if result.err != nil {
			return nil, result.err
		}
		if result.n != 5 {
			return nil, fmt.Errorf("incomplete header read: %d bytes", result.n)
		}
	}

	// Parse header
	circID := binary.BigEndian.Uint32(header[0:4])
	command := cell.Command(header[4])

	// Determine payload size
	var payloadLen int
	if command.IsVariableLength() {
		// Read 2-byte length field
		lenBytes := make([]byte, 2)
		if _, err := conn.Read(lenBytes); err != nil {
			return nil, fmt.Errorf("failed to read payload length: %w", err)
		}
		payloadLen = int(binary.BigEndian.Uint16(lenBytes))
	} else {
		// Fixed-size cell
		payloadLen = cell.PayloadLen
	}

	// Read payload
	payload := make([]byte, payloadLen)
	if payloadLen > 0 {
		if _, err := conn.Read(payload); err != nil {
			return nil, fmt.Errorf("failed to read payload: %w", err)
		}
	}

	return &cell.Cell{
		CircID:  circID,
		Command: command,
		Payload: payload,
	}, nil
}

// writeCell writes a cell to the connection
func (h *LinkProtocolHandler) writeCell(conn net.Conn, c *cell.Cell) error {
	// Use a buffer to encode the cell
	var buf bytes.Buffer
	if err := c.Encode(&buf); err != nil {
		return fmt.Errorf("failed to encode cell: %w", err)
	}

	_, err := conn.Write(buf.Bytes())
	return err
}

// ReceiveCell receives a cell from the server OR connection with context
func (s *ServerORConnection) ReceiveCell(ctx context.Context) (*cell.Cell, error) {
	// Read header (CircID + Command)
	header := make([]byte, 5) // 4 bytes CircID + 1 byte Command

	// Create a channel for the read result
	type readResult struct {
		n   int
		err error
	}
	resultCh := make(chan readResult, 1)

	go func() {
		n, err := s.conn.Read(header)
		resultCh <- readResult{n, err}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-resultCh:
		if result.err != nil {
			return nil, result.err
		}
		if result.n != 5 {
			return nil, fmt.Errorf("incomplete header read: %d bytes", result.n)
		}
	}

	// Parse header
	circID := binary.BigEndian.Uint32(header[0:4])
	command := cell.Command(header[4])

	// Determine payload size
	var payloadLen int
	if command.IsVariableLength() {
		// Read 2-byte length field
		lenBytes := make([]byte, 2)
		if _, err := s.conn.Read(lenBytes); err != nil {
			return nil, fmt.Errorf("failed to read payload length: %w", err)
		}
		payloadLen = int(binary.BigEndian.Uint16(lenBytes))
	} else {
		// Fixed-size cell
		payloadLen = cell.PayloadLen
	}

	// Read payload
	var payload []byte
	if payloadLen > 0 {
		payload = make([]byte, payloadLen)
		if _, err := s.conn.Read(payload); err != nil {
			return nil, fmt.Errorf("failed to read payload: %w", err)
		}
	}

	// Create cell with payload
	return &cell.Cell{
		CircID:  circID,
		Command: command,
		Payload: payload,
	}, nil
}
