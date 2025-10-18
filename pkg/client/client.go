// Package client provides the high-level Tor client orchestration.
// This package integrates all components (directory, circuit, socks) into a functional client.
package client

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/opd-ai/go-tor/pkg/circuit"
	"github.com/opd-ai/go-tor/pkg/config"
	"github.com/opd-ai/go-tor/pkg/directory"
	"github.com/opd-ai/go-tor/pkg/logger"
	"github.com/opd-ai/go-tor/pkg/path"
	"github.com/opd-ai/go-tor/pkg/socks"
)

// Client represents a Tor client instance
type Client struct {
	config       *config.Config
	logger       *logger.Logger
	directory    *directory.Client
	circuitMgr   *circuit.Manager
	socksServer  *socks.Server
	pathSelector *path.Selector
	guardManager *path.GuardManager

	// Circuit management
	circuits   []*circuit.Circuit
	circuitsMu sync.RWMutex

	// Lifecycle management
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	shutdown     chan struct{}
	shutdownOnce sync.Once
}

// New creates a new Tor client
func New(cfg *config.Config, log *logger.Logger) (*Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}
	if log == nil {
		log = logger.NewDefault()
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Initialize directory client
	dirClient := directory.NewClient(log)

	// Initialize circuit manager
	circuitMgr := circuit.NewManager()

	// Initialize SOCKS5 server
	socksAddr := fmt.Sprintf("127.0.0.1:%d", cfg.SocksPort)
	socksServer := socks.NewServer(socksAddr, circuitMgr, log)

	// Initialize guard manager for persistent guard nodes
	guardMgr, err := path.NewGuardManager(cfg.DataDirectory, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create guard manager: %w", err)
	}

	client := &Client{
		config:       cfg,
		logger:       log.Component("client"),
		directory:    dirClient,
		circuitMgr:   circuitMgr,
		socksServer:  socksServer,
		guardManager: guardMgr,
		circuits:     make([]*circuit.Circuit, 0),
		ctx:          ctx,
		cancel:       cancel,
		shutdown:     make(chan struct{}),
	}

	return client, nil
}

// Start starts the Tor client and all its components
func (c *Client) Start(ctx context.Context) error {
	c.logger.Info("Starting Tor client")

	// Merge contexts - respect both parent context and internal context
	ctx = c.mergeContexts(ctx, c.ctx)

	// Step 1: Fetch network consensus (path selector will do this)
	c.logger.Info("Initializing path selector...")

	// Step 2: Initialize path selector with guard persistence and update consensus
	c.pathSelector = path.NewSelectorWithGuards(c.directory, c.guardManager, c.logger)
	if err := c.pathSelector.UpdateConsensus(ctx); err != nil {
		return fmt.Errorf("failed to update consensus: %w", err)
	}
	c.logger.Info("Path selector initialized")

	// Step 3: Clean up expired guards
	c.guardManager.CleanupExpired()

	// Step 4: Build initial circuits
	c.logger.Info("Building initial circuits...")
	if err := c.buildInitialCircuits(ctx); err != nil {
		return fmt.Errorf("failed to build initial circuits: %w", err)
	}
	c.logger.Info("Initial circuits built successfully")

	// Step 5: Start SOCKS5 proxy server
	c.logger.Info("Starting SOCKS5 proxy server", "port", c.config.SocksPort)
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		if err := c.socksServer.ListenAndServe(ctx); err != nil {
			c.logger.Error("SOCKS5 server error", "error", err)
		}
	}()

	// Step 5: Start circuit maintenance loop
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.maintainCircuits(ctx)
	}()

	c.logger.Info("Tor client started successfully")
	return nil
}

// Stop gracefully stops the Tor client
func (c *Client) Stop() error {
	c.shutdownOnce.Do(func() {
		c.logger.Info("Stopping Tor client...")
		close(c.shutdown)
		c.cancel()
	})

	// Wait for goroutines to finish (with timeout)
	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		c.logger.Info("Tor client stopped successfully")
	case <-time.After(30 * time.Second):
		c.logger.Warn("Shutdown timeout exceeded")
	}

	// Close all circuits
	c.circuitsMu.Lock()
	for _, circ := range c.circuits {
		if err := c.circuitMgr.CloseCircuit(circ.ID); err != nil {
			c.logger.Warn("Failed to close circuit", "circuit_id", circ.ID, "error", err)
		}
	}
	c.circuitsMu.Unlock()

	// Stop SOCKS server
	if err := c.socksServer.Shutdown(context.Background()); err != nil {
		c.logger.Warn("Failed to shutdown SOCKS server", "error", err)
	}

	return nil
}

// buildInitialCircuits builds a pool of circuits for use
func (c *Client) buildInitialCircuits(ctx context.Context) error {
	// Build 3 initial circuits for redundancy
	const initialCircuitCount = 3

	for i := 0; i < initialCircuitCount; i++ {
		if err := c.buildCircuit(ctx); err != nil {
			c.logger.Warn("Failed to build circuit", "attempt", i+1, "error", err)
			// Continue trying - we need at least one circuit
			if i == initialCircuitCount-1 {
				return fmt.Errorf("failed to build any circuits")
			}
		}
	}

	return nil
}

// buildCircuit builds a single circuit
func (c *Client) buildCircuit(ctx context.Context) error {
	// Select path (port 80 for general web traffic)
	selectedPath, err := c.pathSelector.SelectPath(80)
	if err != nil {
		return fmt.Errorf("failed to select path: %w", err)
	}

	c.logger.Info("Building circuit",
		"guard", selectedPath.Guard.Nickname,
		"middle", selectedPath.Middle.Nickname,
		"exit", selectedPath.Exit.Nickname)

	// Create circuit builder
	builder := circuit.NewBuilder(c.circuitMgr, c.logger)

	// Build the circuit with 30 second timeout
	circ, err := builder.BuildCircuit(ctx, selectedPath, 30*time.Second)
	if err != nil {
		return fmt.Errorf("failed to build circuit: %w", err)
	}

	// Confirm the guard node as working (for persistence)
	c.pathSelector.ConfirmGuard(selectedPath.Guard.Fingerprint)

	// Add to circuit pool
	c.circuitsMu.Lock()
	c.circuits = append(c.circuits, circ)
	c.circuitsMu.Unlock()

	c.logger.Info("Circuit built successfully", "circuit_id", circ.ID)
	return nil
}

// maintainCircuits maintains the circuit pool
func (c *Client) maintainCircuits(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.shutdown:
			return
		case <-ticker.C:
			c.checkAndRebuildCircuits(ctx)
		}
	}
}

// checkAndRebuildCircuits checks circuit health and rebuilds if needed
func (c *Client) checkAndRebuildCircuits(ctx context.Context) {
	c.circuitsMu.Lock()
	defer c.circuitsMu.Unlock()

	// Remove failed/closed circuits
	activeCircuits := make([]*circuit.Circuit, 0)
	for _, circ := range c.circuits {
		state := circ.GetState()
		if state == circuit.StateOpen {
			activeCircuits = append(activeCircuits, circ)
		} else {
			c.logger.Info("Removing inactive circuit", "circuit_id", circ.ID, "state", state.String())
		}
	}
	c.circuits = activeCircuits

	// Rebuild if needed
	const minCircuitCount = 2
	if len(c.circuits) < minCircuitCount {
		c.logger.Info("Circuit pool low, rebuilding", "current", len(c.circuits), "min", minCircuitCount)
		// Unlock before building (buildCircuit needs to acquire lock)
		c.circuitsMu.Unlock()

		needed := minCircuitCount - len(c.circuits)
		for i := 0; i < needed; i++ {
			if err := c.buildCircuit(ctx); err != nil {
				c.logger.Warn("Failed to rebuild circuit", "error", err)
			}
		}

		// Re-acquire lock for defer
		c.circuitsMu.Lock()
	}
}

// GetStats returns client statistics
func (c *Client) GetStats() Stats {
	c.circuitsMu.RLock()
	defer c.circuitsMu.RUnlock()

	return Stats{
		ActiveCircuits: len(c.circuits),
		SocksPort:      c.config.SocksPort,
		ControlPort:    c.config.ControlPort,
	}
}

// Stats represents client statistics
type Stats struct {
	ActiveCircuits int
	SocksPort      int
	ControlPort    int
}

// mergeContexts creates a context that respects both parent and child cancellation
func (c *Client) mergeContexts(parent, child context.Context) context.Context {
	ctx, cancel := context.WithCancel(parent)

	go func() {
		select {
		case <-parent.Done():
			cancel()
		case <-child.Done():
			cancel()
		}
	}()

	return ctx
}
