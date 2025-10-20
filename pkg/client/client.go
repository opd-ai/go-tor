// Package client provides the high-level Tor client orchestration.
// This package integrates all components (directory, circuit, socks) into a functional client.
package client

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/opd-ai/go-tor/pkg/autoconfig"
	"github.com/opd-ai/go-tor/pkg/circuit"
	"github.com/opd-ai/go-tor/pkg/config"
	"github.com/opd-ai/go-tor/pkg/control"
	"github.com/opd-ai/go-tor/pkg/directory"
	"github.com/opd-ai/go-tor/pkg/logger"
	"github.com/opd-ai/go-tor/pkg/metrics"
	"github.com/opd-ai/go-tor/pkg/path"
	"github.com/opd-ai/go-tor/pkg/socks"
)

// Client represents a Tor client instance
type Client struct {
	config        *config.Config
	logger        *logger.Logger
	directory     *directory.Client
	circuitMgr    *circuit.Manager
	socksServer   *socks.Server
	controlServer *control.Server
	pathSelector  *path.Selector
	guardManager  *path.GuardManager
	metrics       *metrics.Metrics

	// Circuit management
	circuits   []*circuit.Circuit
	circuitsMu sync.RWMutex

	// Bandwidth tracking (for BW events)
	bytesRead    uint64
	bytesWritten uint64
	bwMu         sync.Mutex

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

	// Ensure data directory exists with proper permissions
	if err := autoconfig.EnsureDataDir(cfg.DataDirectory); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	// Cleanup any temporary files from previous runs
	if err := autoconfig.CleanupTempFiles(cfg.DataDirectory); err != nil {
		log.Warn("Failed to cleanup temporary files", "error", err)
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
		cancel() // Clean up context on error
		return nil, fmt.Errorf("failed to create guard manager: %w", err)
	}

	client := &Client{
		config:       cfg,
		logger:       log.Component("client"),
		directory:    dirClient,
		circuitMgr:   circuitMgr,
		socksServer:  socksServer,
		guardManager: guardMgr,
		metrics:      metrics.New(),
		circuits:     make([]*circuit.Circuit, 0),
		ctx:          ctx,
		cancel:       cancel,
		shutdown:     make(chan struct{}),
	}

	// Initialize control protocol server
	controlAddr := fmt.Sprintf("127.0.0.1:%d", cfg.ControlPort)
	client.controlServer = control.NewServer(controlAddr, &clientStatsAdapter{client: client}, log)

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

	// Publish NS and NEWDESC events for the new consensus
	if relays := c.pathSelector.GetRelays(); len(relays) > 0 {
		c.publishNewDescEvents(relays)
		c.publishConsensusEvents(relays)
	}

	// Step 3: Clean up expired guards
	c.guardManager.CleanupExpired()

	// Step 3.5: Update guard metrics
	guardStats := c.guardManager.GetStats()
	c.metrics.GuardsActive.Set(int64(guardStats.TotalGuards))
	c.metrics.GuardsConfirmed.Set(int64(guardStats.ConfirmedGuards))

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

	// Step 6: Start control protocol server
	c.logger.Info("Starting control protocol server", "port", c.config.ControlPort)
	if err := c.controlServer.Start(); err != nil {
		return fmt.Errorf("failed to start control server: %w", err)
	}

	// Step 7: Start circuit maintenance loop
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.maintainCircuits(ctx)
	}()

	// Step 8: Start bandwidth monitoring (publishes BW events)
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.monitorBandwidth(ctx)
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

	// Stop control server
	if err := c.controlServer.Stop(); err != nil {
		c.logger.Warn("Failed to stop control server", "error", err)
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

	// Track circuit build time
	startTime := time.Now()

	// Build the circuit with configured timeout
	circ, err := builder.BuildCircuit(ctx, selectedPath, c.config.CircuitBuildTimeout)
	buildDuration := time.Since(startTime)

	// Record metrics
	c.metrics.RecordCircuitBuild(err == nil, buildDuration)

	if err != nil {
		// Publish circuit failure event
		if circ != nil {
			c.PublishEvent(&control.CircuitEvent{
				CircuitID:   circ.ID,
				Status:      "FAILED",
				Purpose:     "GENERAL",
				TimeCreated: startTime,
			})
		}
		return fmt.Errorf("failed to build circuit: %w", err)
	}

	// Publish circuit built event
	path := fmt.Sprintf("%s~%s,%s~%s,%s~%s",
		selectedPath.Guard.Fingerprint, selectedPath.Guard.Nickname,
		selectedPath.Middle.Fingerprint, selectedPath.Middle.Nickname,
		selectedPath.Exit.Fingerprint, selectedPath.Exit.Nickname)

	c.PublishEvent(&control.CircuitEvent{
		CircuitID:   circ.ID,
		Status:      "BUILT",
		Path:        path,
		Purpose:     "GENERAL",
		TimeCreated: startTime,
	})

	// Confirm the guard node as working (for persistence)
	c.pathSelector.ConfirmGuard(selectedPath.Guard.Fingerprint)

	// Publish GUARD event for confirmed guard
	c.PublishEvent(&control.GuardEvent{
		GuardType: "ENTRY",
		Name:      fmt.Sprintf("$%s~%s", selectedPath.Guard.Fingerprint, selectedPath.Guard.Nickname),
		Status:    "GOOD",
	})

	// Add to circuit pool
	c.circuitsMu.Lock()
	c.circuits = append(c.circuits, circ)
	c.metrics.ActiveCircuits.Set(int64(len(c.circuits)))
	c.circuitsMu.Unlock()

	c.logger.Info("Circuit built successfully", "circuit_id", circ.ID, "duration", buildDuration)
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

	// Remove failed/closed circuits and enforce max circuit age
	activeCircuits := make([]*circuit.Circuit, 0)
	maxAge := c.config.MaxCircuitDirtiness
	for _, circ := range c.circuits {
		state := circ.GetState()
		age := circ.Age()

		// Remove circuits that are not open or too old
		if state != circuit.StateOpen {
			c.logger.Info("Removing inactive circuit", "circuit_id", circ.ID, "state", state.String())
			continue
		}

		if age > maxAge {
			c.logger.Info("Removing old circuit", "circuit_id", circ.ID, "age", age, "max_age", maxAge)
			// Close the old circuit
			circ.SetState(circuit.StateClosed)
			if err := c.circuitMgr.CloseCircuit(circ.ID); err != nil {
				c.logger.Warn("Failed to close old circuit", "circuit_id", circ.ID, "error", err)
			}
			// Publish circuit closed event
			c.PublishEvent(&control.CircuitEvent{
				CircuitID:   circ.ID,
				Status:      "CLOSED",
				Purpose:     "GENERAL",
				TimeCreated: circ.CreatedAt,
			})
			continue
		}

		activeCircuits = append(activeCircuits, circ)
	}
	c.circuits = activeCircuits
	c.metrics.ActiveCircuits.Set(int64(len(c.circuits)))

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

	c.circuitsMu.Unlock()
}

// GetStats returns client statistics
func (c *Client) GetStats() Stats {
	c.circuitsMu.RLock()
	defer c.circuitsMu.RUnlock()

	// Get guard statistics
	guardStats := c.guardManager.GetStats()

	// Get metrics snapshot
	metricsSnap := c.metrics.Snapshot()

	return Stats{
		ActiveCircuits:      len(c.circuits),
		SocksPort:           c.config.SocksPort,
		ControlPort:         c.config.ControlPort,
		CircuitBuilds:       metricsSnap.CircuitBuilds,
		CircuitBuildSuccess: metricsSnap.CircuitBuildSuccess,
		CircuitBuildFailure: metricsSnap.CircuitBuildFailure,
		CircuitBuildTimeAvg: metricsSnap.CircuitBuildTimeAvg,
		CircuitBuildTimeP95: metricsSnap.CircuitBuildTimeP95,
		GuardsActive:        guardStats.TotalGuards,
		GuardsConfirmed:     guardStats.ConfirmedGuards,
		ConnectionAttempts:  metricsSnap.ConnectionAttempts,
		ConnectionRetries:   metricsSnap.ConnectionRetries,
		UptimeSeconds:       metricsSnap.UptimeSeconds,
	}
}

// Stats represents client statistics
type Stats struct {
	// Basic stats
	ActiveCircuits int
	SocksPort      int
	ControlPort    int

	// Circuit metrics
	CircuitBuilds       int64
	CircuitBuildSuccess int64
	CircuitBuildFailure int64
	CircuitBuildTimeAvg time.Duration
	CircuitBuildTimeP95 time.Duration

	// Guard metrics
	GuardsActive    int
	GuardsConfirmed int

	// Connection metrics
	ConnectionAttempts int64
	ConnectionRetries  int64

	// System metrics
	UptimeSeconds int64
}

// GetActiveCircuits returns the number of active circuits
func (s Stats) GetActiveCircuits() int {
	return s.ActiveCircuits
}

// GetSocksPort returns the SOCKS proxy port
func (s Stats) GetSocksPort() int {
	return s.SocksPort
}

// GetControlPort returns the control protocol port
func (s Stats) GetControlPort() int {
	return s.ControlPort
}

// PublishEvent publishes an event to the control protocol
func (c *Client) PublishEvent(event control.Event) {
	if c.controlServer != nil {
		c.controlServer.GetEventDispatcher().Dispatch(event)
	}
}

// publishConsensusEvents publishes NS events for relays in the consensus
func (c *Client) publishConsensusEvents(relays []*directory.Relay) {
	// Only publish a subset to avoid flooding - publish events for guards and exits
	count := 0
	maxEvents := 50 // Limit to avoid overwhelming subscribers

	for _, relay := range relays {
		if count >= maxEvents {
			break
		}

		// Only publish for guards and exits (most interesting nodes)
		// Use short-circuit evaluation to avoid redundant method calls
		if !(relay.IsGuard() || relay.IsExit()) {
			continue
		}

		c.PublishEvent(&control.NSEvent{
			LongName:    fmt.Sprintf("$%s~%s", relay.Fingerprint, relay.Nickname),
			Fingerprint: fmt.Sprintf("$%s", relay.Fingerprint),
			Published:   relay.Published.Format(time.RFC3339),
			IP:          relay.Address,
			ORPort:      relay.ORPort,
			DirPort:     relay.DirPort,
			Flags:       relay.Flags,
		})
		count++
	}

	c.logger.Debug("Published NS events", "count", count)
}

// publishNewDescEvents publishes NEWDESC events for new relay descriptors
func (c *Client) publishNewDescEvents(relays []*directory.Relay) {
	// Collect fingerprints for NEWDESC event
	descriptors := make([]string, 0, len(relays))

	// Limit to avoid huge events
	maxDescriptors := 100
	for i, relay := range relays {
		if i >= maxDescriptors {
			break
		}
		descriptors = append(descriptors, fmt.Sprintf("$%s~%s", relay.Fingerprint, relay.Nickname))
	}

	if len(descriptors) > 0 {
		c.PublishEvent(&control.NewDescEvent{
			Descriptors: descriptors,
		})
		c.logger.Debug("Published NEWDESC event", "count", len(descriptors))
	}
}

// monitorBandwidth periodically publishes BW events
func (c *Client) monitorBandwidth(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second) // BW events every second
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.shutdown:
			return
		case <-ticker.C:
			c.publishBandwidthEvent()
		}
	}
}

// publishBandwidthEvent publishes a bandwidth usage event
func (c *Client) publishBandwidthEvent() {
	c.bwMu.Lock()
	bytesRead := c.bytesRead
	bytesWritten := c.bytesWritten
	c.bwMu.Unlock()

	c.PublishEvent(&control.BWEvent{
		BytesRead:    bytesRead,
		BytesWritten: bytesWritten,
	})
}

// RecordBytesRead records bytes read (called by stream/circuit layers)
func (c *Client) RecordBytesRead(n uint64) {
	c.bwMu.Lock()
	c.bytesRead += n
	c.bwMu.Unlock()
}

// RecordBytesWritten records bytes written (called by stream/circuit layers)
func (c *Client) RecordBytesWritten(n uint64) {
	c.bwMu.Lock()
	c.bytesWritten += n
	c.bwMu.Unlock()
}

// clientStatsAdapter adapts Client to control.ClientInfoGetter
type clientStatsAdapter struct {
	client *Client
}

func (a *clientStatsAdapter) GetStats() control.StatsProvider {
	return a.client.GetStats()
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
