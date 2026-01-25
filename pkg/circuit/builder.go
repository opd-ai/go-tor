// Package circuit provides circuit building functionality for the Tor protocol.
package circuit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/opd-ai/go-tor/pkg/connection"
	"github.com/opd-ai/go-tor/pkg/directory"
	"github.com/opd-ai/go-tor/pkg/logger"
	"github.com/opd-ai/go-tor/pkg/path"
)

// Builder constructs Tor circuits through the network
type Builder struct {
	logger  *logger.Logger
	manager *Manager
	mu      sync.Mutex
}

// NewBuilder creates a new circuit builder
func NewBuilder(manager *Manager, log *logger.Logger) *Builder {
	if log == nil {
		log = logger.NewDefault()
	}

	return &Builder{
		logger:  log.Component("builder"),
		manager: manager,
	}
}

// BuildCircuit builds a complete 3-hop circuit using the provided path
func (b *Builder) BuildCircuit(ctx context.Context, p *path.Path, timeout time.Duration) (*Circuit, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.logger.Info("Building circuit",
		"guard", p.Guard.Nickname,
		"middle", p.Middle.Nickname,
		"exit", p.Exit.Nickname)

	// Create the circuit
	circuit, err := b.manager.CreateCircuit()
	if err != nil {
		return nil, fmt.Errorf("failed to create circuit: %w", err)
	}

	// Build with timeout
	buildCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Connect to guard with certificate pinning
	guardAddr := fmt.Sprintf("%s:%d", p.Guard.Address, p.Guard.ORPort)
	guardConn, err := b.connectToRelay(buildCtx, guardAddr, p.Guard)
	if err != nil {
		circuit.SetState(StateFailed)
		return nil, fmt.Errorf("failed to connect to guard: %w", err)
	}

	// Store connection in circuit for cell I/O
	circuit.SetConnection(guardConn)

	// Add guard hop structure
	if err := circuit.AddHop(&Hop{
		Fingerprint: p.Guard.Fingerprint,
		Address:     guardAddr,
		IsGuard:     true,
		IsExit:      false,
	}); err != nil {
		circuit.SetState(StateFailed)
		return nil, fmt.Errorf("failed to add guard hop: %w", err)
	}

	b.logger.Info("Connected to guard", "guard", p.Guard.Nickname)

	// Create extension handler for this circuit
	ext := NewExtension(circuit, b.logger)

	// Set relay keys from guard for first hop (SPEC-001)
	ext.SetTargetRelay(p.Guard)

	// Create first hop using CREATE2 protocol
	if err := ext.CreateFirstHop(buildCtx, HandshakeTypeNTor); err != nil {
		circuit.SetState(StateFailed)
		return nil, fmt.Errorf("failed to create first hop: %w", err)
	}

	b.logger.Info("First hop created with ntor handshake", "guard", p.Guard.Nickname)

	// Extend to middle relay using EXTEND2 protocol
	ext.SetTargetRelay(p.Middle)
	middleAddr := fmt.Sprintf("%s:%d", p.Middle.Address, p.Middle.ORPort)
	if err := ext.ExtendCircuit(buildCtx, middleAddr, HandshakeTypeNTor); err != nil {
		circuit.SetState(StateFailed)
		return nil, fmt.Errorf("failed to extend to middle hop: %w", err)
	}

	b.logger.Info("Extended to middle", "middle", p.Middle.Nickname)

	// Extend to exit relay using EXTEND2 protocol
	ext.SetTargetRelay(p.Exit)
	exitAddr := fmt.Sprintf("%s:%d", p.Exit.Address, p.Exit.ORPort)
	if err := ext.ExtendCircuit(buildCtx, exitAddr, HandshakeTypeNTor); err != nil {
		circuit.SetState(StateFailed)
		return nil, fmt.Errorf("failed to extend to exit hop: %w", err)
	}

	b.logger.Info("Extended to exit", "exit", p.Exit.Nickname)

	// Mark circuit as open
	circuit.SetState(StateOpen)

	b.logger.Info("Circuit built successfully", "circuit_id", circuit.ID, "hops", circuit.Length())

	return circuit, nil
}

// connectToRelay establishes a connection to a relay with certificate pinning.
// This implements enhanced certificate validation per tor-spec.txt §2 by:
// 1. Setting expected Ed25519 identity from directory consensus
// 2. Setting expected RSA fingerprint from directory consensus
// 3. Enabling CERTS cell validation in the link protocol handshake
// This prevents MITM attacks where an adversary presents a valid self-signed
// certificate for a different relay's identity.
func (b *Builder) connectToRelay(ctx context.Context, address string, relay *directory.Relay) (*connection.Connection, error) {
	cfg := connection.DefaultConfig(address)
	
	// AUDIT-004: Enhanced certificate pinning with relay identity from consensus
	if relay != nil {
		// Set expected Ed25519 identity key from consensus (32 bytes)
		if len(relay.IdentityKey) == 32 {
			cfg.ExpectedIdentity = relay.IdentityKey
			b.logger.Debug("Certificate pinning enabled",
				"relay", relay.Nickname,
				"fingerprint", relay.Fingerprint)
		}
		
		// Set expected RSA fingerprint from consensus
		if relay.Fingerprint != "" {
			cfg.ExpectedFingerprint = relay.Fingerprint
		}
		
		// Enable strict CERTS validation mode for defense-in-depth
		// This will fail the handshake if CERTS validation fails
		cfg.RequireCERTS = true
	}
	
	conn := connection.New(cfg, b.logger)

	if err := conn.Connect(ctx, cfg); err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	// Wait for connection to be ready
	select {
	case <-ctx.Done():
		if err := conn.Close(); err != nil {
			b.logger.Error("Failed to close connection on context cancellation", "function", "connectToRelay", "error", err)
		}
		return nil, ctx.Err()
	case <-time.After(100 * time.Millisecond):
		// Connection established
	}

	return conn, nil
}
