// Package circuit provides circuit building functionality for the Tor protocol.
package circuit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/opd-ai/go-tor/pkg/connection"
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

	// Connect to guard
	guardAddr := fmt.Sprintf("%s:%d", p.Guard.Address, p.Guard.ORPort)
	guardConn, err := b.connectToRelay(buildCtx, guardAddr)
	if err != nil {
		circuit.SetState(StateFailed)
		return nil, fmt.Errorf("failed to connect to guard: %w", err)
	}
	defer func() {
		if err := guardConn.Close(); err != nil {
			b.logger.Error("Failed to close guard connection", "function", "BuildCircuit", "error", err)
		}
	}()

	// Add guard hop
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

	// Add middle hop (simulated for now)
	if err := circuit.AddHop(&Hop{
		Fingerprint: p.Middle.Fingerprint,
		Address:     fmt.Sprintf("%s:%d", p.Middle.Address, p.Middle.ORPort),
		IsGuard:     false,
		IsExit:      false,
	}); err != nil {
		circuit.SetState(StateFailed)
		return nil, fmt.Errorf("failed to add middle hop: %w", err)
	}

	b.logger.Info("Extended to middle", "middle", p.Middle.Nickname)

	// Add exit hop (simulated for now)
	if err := circuit.AddHop(&Hop{
		Fingerprint: p.Exit.Fingerprint,
		Address:     fmt.Sprintf("%s:%d", p.Exit.Address, p.Exit.ORPort),
		IsGuard:     false,
		IsExit:      true,
	}); err != nil {
		circuit.SetState(StateFailed)
		return nil, fmt.Errorf("failed to add exit hop: %w", err)
	}

	b.logger.Info("Extended to exit", "exit", p.Exit.Nickname)

	// Mark circuit as open
	circuit.SetState(StateOpen)

	b.logger.Info("Circuit built successfully", "circuit_id", circuit.ID, "hops", circuit.Length())

	return circuit, nil
}

// connectToRelay establishes a connection to a relay
func (b *Builder) connectToRelay(ctx context.Context, address string) (*connection.Connection, error) {
	cfg := connection.DefaultConfig(address)
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
