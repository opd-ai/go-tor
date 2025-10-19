# go-tor Phase 2 Implementation: Complete Analysis and Deliverables

## 1. Analysis Summary (150-250 words)

The go-tor project is a pure Go implementation of a Tor client designed for embedded systems. The codebase analysis revealed that Phase 1 (Foundation) was complete with comprehensive cell encoding/decoding, circuit management types, cryptographic primitives, configuration system, and structured logging - all with ~90% test coverage.

The code maturity is mid-stage: the foundation is solid with excellent test coverage and follows Go best practices, but the application cannot yet connect to the Tor network. The main application shell exists but lacks the core protocol implementation needed for actual network functionality.

The repository structure is well-organized with clear package separation, consistent error handling patterns, comprehensive testing, and excellent documentation. All existing tests pass, the code builds successfully, and follows idiomatic Go conventions. The README clearly indicates Phase 2 (Core Protocol) as the next logical development phase.

Based on this analysis, the identified next step is implementing Phase 2: Core Protocol, which includes TLS connection handling, protocol handshake/version negotiation, cell I/O, and directory client functionality. This is the critical missing piece that will enable actual network connectivity and prepare the codebase for circuit building in Phase 3.

## 2. Proposed Next Phase (100-150 words)

**Selected Phase: Phase 2 - Core Protocol Implementation**

**Rationale**: Phase 2 is the logical next step as it provides the networking foundation required for all subsequent phases. Without the ability to connect to Tor relays and negotiate the protocol, the application cannot function as a Tor client. This phase directly addresses the project's current limitation stated in the README: "The client is not yet functional."

**Expected Outcomes**:
- Establish TLS connections to Tor relays
- Negotiate protocol versions (link protocol v3-5)
- Send and receive cells over connections
- Fetch network consensus from directory authorities
- Extract relay information for path selection

**Benefits**: Enables network connectivity, validates existing cell/circuit abstractions with real data, and provides the foundation for circuit building (Phase 3).

**Scope Boundaries**: Focus on connection establishment and basic protocol negotiation; circuit building, stream handling, and SOCKS proxy are explicitly out of scope for this phase.

## 3. Implementation Plan (200-300 words)

**Detailed Breakdown of Changes**:

1. **Connection Package** (`pkg/connection`):
   - Implement TLS connection wrapper with state management
   - Add thread-safe cell send/receive operations
   - Implement connection lifecycle (connect, send, receive, close)
   - Add proper timeout and error handling
   - Support both v3 and v4 link protocols

2. **Protocol Package** (`pkg/protocol`):
   - Implement version negotiation (VERSIONS cell exchange)
   - Add NETINFO cell exchange for connection metadata
   - Implement version selection algorithm (highest mutual version)
   - Add handshake orchestration with timeout

3. **Directory Package** (`pkg/directory`):
   - Implement HTTP client for directory authorities
   - Add consensus document parsing (relay entries)
   - Extract relay information (nickname, address, ports, flags)
   - Add relay filtering by flags (Guard, Exit, etc.)
   - Implement fallback to multiple authorities

4. **Testing**:
   - Comprehensive unit tests for all new packages
   - Mock TLS servers for connection testing
   - Mock consensus data for directory testing
   - Test error cases, timeouts, and edge conditions

5. **Documentation**:
   - Update README with Phase 2 completion
   - Update ARCHITECTURE.md with new packages
   - Create PHASE2.md guide with examples
   - Add example demo program

**Files to Create**:
- `pkg/connection/connection.go` and `connection_test.go`
- `pkg/protocol/protocol.go` and `protocol_test.go`
- `pkg/directory/directory.go` and `directory_test.go`
- `examples/phase2-demo/main.go`
- `docs/PHASE2.md`

**Technical Approach**:
- Use Go's standard `crypto/tls` for TLS connections
- Leverage existing `pkg/cell` for cell encoding/decoding
- HTTP client for directory protocol (simpler than custom protocol)
- State machines for connection lifecycle
- Context-based cancellation and timeouts throughout

**Design Decisions**:
- InsecureSkipVerify for TLS initially (proper cert validation is Phase 5)
- HTTP-based directory client (fallback directories, not full directory protocol)
- Simplified NETINFO cell (minimal required information)
- Link protocol v4 as default (4-byte circuit IDs)

**Potential Risks**:
- Network connectivity required for testing directory client
- Real Tor relays may reject connections without proper certificates
- Consensus parsing may encounter unexpected formats

## 4. Code Implementation

### pkg/connection/connection.go

```go
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
```

### pkg/protocol/protocol.go

```go
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
)

// Protocol versions supported by this implementation
const (
	MinLinkProtocolVersion = 3
	MaxLinkProtocolVersion = 5
	PreferredVersion       = 4 // Link protocol v4 uses 4-byte circuit IDs
)

// Handshake performs the Tor protocol handshake on a connection
type Handshake struct {
	conn              *connection.Connection
	negotiatedVersion int
	logger            *logger.Logger
}

// NewHandshake creates a new handshake instance
func NewHandshake(conn *connection.Connection, log *logger.Logger) *Handshake {
	if log == nil {
		log = logger.NewDefault()
	}
	return &Handshake{
		conn:   conn,
		logger: log,
	}
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
	// Set a timeout for receiving
	timer := time.NewTimer(30 * time.Second)
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
	timestamp := uint32(time.Now().Unix())
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
	timer := time.NewTimer(30 * time.Second)
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
```

### pkg/directory/directory.go

```go
// Package directory provides Tor directory protocol functionality.
// This package handles fetching and parsing directory consensus documents and router descriptors.
package directory

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/opd-ai/go-tor/pkg/logger"
)

// Default directory authority addresses (hardcoded fallback directories)
var DefaultAuthorities = []string{
	"https://194.109.206.212/tor/status-vote/current/consensus.z", // gabelmoo
	"https://131.188.40.189/tor/status-vote/current/consensus.z",  // moria1
	"https://128.31.0.34:9131/tor/status-vote/current/consensus.z", // tor26
}

// Relay represents a Tor relay from the consensus
type Relay struct {
	Nickname    string
	Fingerprint string
	Address     string
	ORPort      int
	DirPort     int
	Flags       []string
	Published   time.Time
}

// Client provides directory protocol operations
type Client struct {
	httpClient *http.Client
	logger     *logger.Logger
	authorities []string
}

// NewClient creates a new directory client
func NewClient(log *logger.Logger) *Client {
	if log == nil {
		log = logger.NewDefault()
	}
	
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger:      log.Component("directory"),
		authorities: DefaultAuthorities,
	}
}

// FetchConsensus fetches the network consensus from directory authorities
func (c *Client) FetchConsensus(ctx context.Context) ([]*Relay, error) {
	c.logger.Info("Fetching network consensus")

	// Try each authority until one succeeds
	var lastErr error
	for _, authority := range c.authorities {
		relays, err := c.fetchFromAuthority(ctx, authority)
		if err != nil {
			c.logger.Warn("Failed to fetch from authority", "authority", authority, "error", err)
			lastErr = err
			continue
		}
		
		c.logger.Info("Successfully fetched consensus", "relays", len(relays), "authority", authority)
		return relays, nil
	}

	return nil, fmt.Errorf("failed to fetch consensus from any authority: %w", lastErr)
}

// fetchFromAuthority fetches consensus from a specific authority
func (c *Client) fetchFromAuthority(ctx context.Context, authorityURL string) ([]*Relay, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", authorityURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch consensus: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse the consensus document
	relays, err := c.parseConsensus(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse consensus: %w", err)
	}

	return relays, nil
}

// parseConsensus parses a consensus document and extracts relay information
func (c *Client) parseConsensus(r io.Reader) ([]*Relay, error) {
	var relays []*Relay
	scanner := bufio.NewScanner(r)
	
	var currentRelay *Relay
	
	for scanner.Scan() {
		line := scanner.Text()
		
		// Parse "r" lines (router status entries)
		if strings.HasPrefix(line, "r ") {
			if currentRelay != nil {
				relays = append(relays, currentRelay)
			}
			
			parts := strings.Fields(line)
			if len(parts) < 9 {
				continue // Skip malformed entries
			}
			
			currentRelay = &Relay{
				Nickname:    parts[1],
				Fingerprint: parts[2],
				Address:     parts[6],
			}
			
			// Parse ORPort
			fmt.Sscanf(parts[7], "%d", &currentRelay.ORPort)
			// Parse DirPort
			fmt.Sscanf(parts[8], "%d", &currentRelay.DirPort)
		}
		
		// Parse "s" lines (flags)
		if strings.HasPrefix(line, "s ") && currentRelay != nil {
			flags := strings.Fields(line[2:]) // Skip "s "
			currentRelay.Flags = flags
		}
	}
	
	// Add the last relay
	if currentRelay != nil {
		relays = append(relays, currentRelay)
	}
	
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading consensus: %w", err)
	}
	
	return relays, nil
}

// HasFlag checks if a relay has a specific flag
func (r *Relay) HasFlag(flag string) bool {
	for _, f := range r.Flags {
		if f == flag {
			return true
		}
	}
	return false
}

// IsGuard returns true if the relay is a guard
func (r *Relay) IsGuard() bool {
	return r.HasFlag("Guard")
}

// IsExit returns true if the relay is an exit
func (r *Relay) IsExit() bool {
	return r.HasFlag("Exit")
}

// IsStable returns true if the relay is stable
func (r *Relay) IsStable() bool {
	return r.HasFlag("Stable")
}

// IsRunning returns true if the relay is running
func (r *Relay) IsRunning() bool {
	return r.HasFlag("Running")
}

// IsValid returns true if the relay is valid
func (r *Relay) IsValid() bool {
	return r.HasFlag("Valid")
}

// String returns a string representation of the relay
func (r *Relay) String() string {
	return fmt.Sprintf("%s (%s:%d)", r.Nickname, r.Address, r.ORPort)
}
```

## 5. Testing & Usage

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests for new packages with verbose output
go test -v ./pkg/connection ./pkg/protocol ./pkg/directory

# Run with race detector
go test -race ./pkg/connection ./pkg/protocol ./pkg/directory

# Run with coverage
go test -cover ./pkg/connection ./pkg/protocol ./pkg/directory

# Generate coverage report
go test -coverprofile=coverage.out ./pkg/connection ./pkg/protocol ./pkg/directory
go tool cover -html=coverage.out -o coverage.html
```

### Building

```bash
# Build main client
make build

# Build demo program
go build ./examples/phase2-demo

# Build for multiple platforms
make build-all
```

### Running the Demo

```bash
# Build and run the Phase 2 demo
cd examples/phase2-demo
go build
./phase2-demo
```

**Expected Output**:
```
time=... level=INFO msg="go-tor Phase 2 Demo: Core Protocol Implementation"
time=... level=INFO msg="This demo fetches consensus and connects to a Tor relay"
time=... level=INFO msg="Step 1: Fetching Tor network consensus..." component=directory
time=... level=INFO msg="Successfully fetched consensus" component=directory relays=7000+ authority=...
time=... level=INFO msg="Found guard relays" count=5

Sample Guard Relays:
  1. RelayNickname (IP:Port)
     Flags: [Fast Guard Running Stable Valid]
  ...
```

### Example Usage in Code

```go
package main

import (
	"context"
	"log"
	
	"github.com/opd-ai/go-tor/pkg/connection"
	"github.com/opd-ai/go-tor/pkg/directory"
	"github.com/opd-ai/go-tor/pkg/logger"
	"github.com/opd-ai/go-tor/pkg/protocol"
)

func main() {
	// Setup
	ctx := context.Background()
	logger := logger.NewDefault()
	
	// Fetch consensus
	dirClient := directory.NewClient(logger)
	relays, err := dirClient.FetchConsensus(ctx)
	if err != nil {
		log.Fatal(err)
	}
	
	// Find a guard relay
	var guardRelay *directory.Relay
	for _, r := range relays {
		if r.IsGuard() && r.IsRunning() {
			guardRelay = r
			break
		}
	}
	
	// Connect to relay
	cfg := connection.DefaultConfig(fmt.Sprintf("%s:%d", guardRelay.Address, guardRelay.ORPort))
	conn := connection.New(cfg, logger)
	if err := conn.Connect(ctx, cfg); err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	
	// Perform handshake
	handshake := protocol.NewHandshake(conn, logger)
	if err := handshake.PerformHandshake(ctx); err != nil {
		log.Fatal(err)
	}
	
	log.Printf("Connected! Protocol version: %d", handshake.NegotiatedVersion())
}
```

## 6. Integration Notes (100-150 words)

**Integration with Existing Code**:
The Phase 2 implementation seamlessly integrates with existing Phase 1 components. The `connection` package uses `pkg/cell` for cell encoding/decoding, leverages `pkg/logger` for structured logging, and prepares data structures for `pkg/circuit` to build circuits in Phase 3.

**Configuration Changes**:
No configuration changes are required. The existing `Config` struct in `pkg/config` already supports the necessary parameters. Future additions may include directory caching and connection pool settings.

**No Breaking Changes**:
All Phase 1 APIs remain unchanged. Phase 2 adds new packages without modifying existing interfaces.

**Migration Steps**:
None required - this is additive functionality. Applications can adopt Phase 2 features incrementally.

**Backward Compatibility**:
Fully maintained. All existing tests pass, and the build remains compatible with Go 1.21+.

**Next Integration Point**:
Phase 3 will integrate these components to build complete circuits using CREATE2/CREATED2 cells and implement path selection algorithms using the relay information from the directory client.
