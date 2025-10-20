// Package pool provides resource pooling for performance optimization.
package pool

import (
	"context"
	"sync"
	"time"

	"github.com/opd-ai/go-tor/pkg/circuit"
	"github.com/opd-ai/go-tor/pkg/logger"
)

// CircuitPool manages a pool of pre-built circuits for performance
type CircuitPool struct {
	mu              sync.RWMutex
	circuits        []*circuit.Circuit
	isolatedCircuits map[string][]*circuit.Circuit // Keyed by isolation key
	minCircuits     int
	maxCircuits     int
	buildFunc       CircuitBuilder
	logger          *logger.Logger
	prebuildEnabled bool
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
}

// CircuitBuilder is a function that builds a new circuit
type CircuitBuilder func(ctx context.Context) (*circuit.Circuit, error)

// CircuitPoolConfig holds configuration for the circuit pool
type CircuitPoolConfig struct {
	MinCircuits     int           // Minimum number of circuits to maintain
	MaxCircuits     int           // Maximum number of circuits in the pool
	PrebuildEnabled bool          // Enable automatic prebuilding
	RebuildInterval time.Duration // How often to check and rebuild circuits
}

// DefaultCircuitPoolConfig returns sensible defaults for circuit pooling
func DefaultCircuitPoolConfig() *CircuitPoolConfig {
	return &CircuitPoolConfig{
		MinCircuits:     2,
		MaxCircuits:     10,
		PrebuildEnabled: true,
		RebuildInterval: 30 * time.Second,
	}
}

// NewCircuitPool creates a new circuit pool
func NewCircuitPool(cfg *CircuitPoolConfig, builder CircuitBuilder, log *logger.Logger) *CircuitPool {
	if cfg == nil {
		cfg = DefaultCircuitPoolConfig()
	}
	if log == nil {
		log = logger.NewDefault()
	}

	ctx, cancel := context.WithCancel(context.Background())

	p := &CircuitPool{
		circuits:         make([]*circuit.Circuit, 0, cfg.MaxCircuits),
		isolatedCircuits: make(map[string][]*circuit.Circuit),
		minCircuits:      cfg.MinCircuits,
		maxCircuits:      cfg.MaxCircuits,
		buildFunc:        builder,
		logger:           log.Component("circuit-pool"),
		prebuildEnabled:  cfg.PrebuildEnabled,
		ctx:              ctx,
		cancel:           cancel,
	}

	// Start prebuilding if enabled
	if cfg.PrebuildEnabled {
		p.wg.Add(1)
		go p.prebuildLoop(cfg.RebuildInterval)
	}

	return p
}

// Get retrieves a circuit from the pool
func (p *CircuitPool) Get(ctx context.Context) (*circuit.Circuit, error) {
	return p.GetWithIsolation(ctx, nil)
}

// GetWithIsolation retrieves a circuit from the pool with the specified isolation key
// If isolationKey is nil or has level IsolationNone, uses the default non-isolated pool
func (p *CircuitPool) GetWithIsolation(ctx context.Context, isolationKey *circuit.IsolationKey) (*circuit.Circuit, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Determine which pool to use
	var poolCircuits []*circuit.Circuit
	var poolKey string

	if isolationKey != nil && isolationKey.Level != circuit.IsolationNone {
		poolKey = isolationKey.Key()
		poolCircuits = p.isolatedCircuits[poolKey]
		p.logger.Debug("Looking for isolated circuit", "isolation_key", isolationKey.String(), "pool_size", len(poolCircuits))
	} else {
		poolCircuits = p.circuits
		p.logger.Debug("Looking for non-isolated circuit", "pool_size", len(poolCircuits))
	}

	// Try to get a healthy circuit from the appropriate pool
	for len(poolCircuits) > 0 {
		circ := poolCircuits[0]
		poolCircuits = poolCircuits[1:]

		// Update the pool
		if isolationKey != nil && isolationKey.Level != circuit.IsolationNone {
			p.isolatedCircuits[poolKey] = poolCircuits
		} else {
			p.circuits = poolCircuits
		}

		// Check if circuit is still open
		if circ.GetState() == circuit.StateOpen {
			p.logger.Debug("Retrieved circuit from pool", "circuit_id", circ.ID, "isolation_key", isolationKey)
			return circ, nil
		}

		// Circuit is not open, discard it
		p.logger.Debug("Discarding closed circuit from pool", "circuit_id", circ.ID, "state", circ.GetState())
	}

	// No circuits available, build a new one
	p.logger.Debug("No circuits in pool, building new circuit", "isolation_key", isolationKey)
	circ, err := p.buildFunc(ctx)
	if err != nil {
		return nil, err
	}

	// Set the isolation key on the circuit
	if isolationKey != nil {
		circ.SetIsolationKey(isolationKey)
	}

	return circ, nil
}

// Put returns a circuit to the pool
func (p *CircuitPool) Put(circ *circuit.Circuit) {
	if circ == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Only keep open circuits
	if circ.GetState() != circuit.StateOpen {
		p.logger.Debug("Not returning closed circuit to pool", "circuit_id", circ.ID, "state", circ.GetState())
		return
	}

	// Determine which pool to use based on isolation key
	isolationKey := circ.GetIsolationKey()
	if isolationKey != nil && isolationKey.Level != circuit.IsolationNone {
		poolKey := isolationKey.Key()
		poolCircuits := p.isolatedCircuits[poolKey]

		// Check if we're at capacity for this isolated pool
		if len(poolCircuits) >= p.maxCircuits {
			p.logger.Debug("Isolated circuit pool at capacity, not returning circuit",
				"circuit_id", circ.ID,
				"isolation_key", isolationKey.String())
			return
		}

		p.isolatedCircuits[poolKey] = append(poolCircuits, circ)
		p.logger.Debug("Returned circuit to isolated pool",
			"circuit_id", circ.ID,
			"isolation_key", isolationKey.String(),
			"pool_size", len(p.isolatedCircuits[poolKey]))
	} else {
		// Check if we're at capacity
		if len(p.circuits) >= p.maxCircuits {
			p.logger.Debug("Circuit pool at capacity, not returning circuit", "circuit_id", circ.ID)
			return
		}

		p.circuits = append(p.circuits, circ)
		p.logger.Debug("Returned circuit to pool", "circuit_id", circ.ID, "pool_size", len(p.circuits))
	}
}

// prebuildLoop maintains the minimum number of circuits
func (p *CircuitPool) prebuildLoop(interval time.Duration) {
	defer p.wg.Done()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			p.logger.Debug("Circuit prebuild loop shutting down")
			return
		case <-ticker.C:
			p.ensureMinCircuits()
		}
	}
}

// ensureMinCircuits builds circuits if we're below the minimum
func (p *CircuitPool) ensureMinCircuits() {
	p.mu.RLock()
	currentCount := len(p.circuits)
	p.mu.RUnlock()

	if currentCount >= p.minCircuits {
		return
	}

	needed := p.minCircuits - currentCount
	p.logger.Debug("Prebuilding circuits", "needed", needed, "current", currentCount, "min", p.minCircuits)

	for i := 0; i < needed; i++ {
		// Use a timeout context for building
		ctx, cancel := context.WithTimeout(p.ctx, 30*time.Second)
		circ, err := p.buildFunc(ctx)
		cancel()

		if err != nil {
			p.logger.Warn("Failed to prebuild circuit", "error", err)
			continue
		}

		p.Put(circ)
	}
}

// Stats returns statistics about the circuit pool
func (p *CircuitPool) Stats() CircuitPoolStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := CircuitPoolStats{
		Total:            len(p.circuits),
		MinCircuits:      p.minCircuits,
		MaxCircuits:      p.maxCircuits,
		IsolatedPools:    len(p.isolatedCircuits),
		IsolatedCircuits: 0,
	}

	// Count open circuits in main pool
	for _, circ := range p.circuits {
		if circ.GetState() == circuit.StateOpen {
			stats.Open++
		}
	}

	// Count isolated circuits
	for _, poolCircuits := range p.isolatedCircuits {
		stats.IsolatedCircuits += len(poolCircuits)
		for _, circ := range poolCircuits {
			if circ.GetState() == circuit.StateOpen {
				stats.Open++
			}
		}
	}

	stats.Total += stats.IsolatedCircuits

	return stats
}

// Close closes the circuit pool and cleans up resources
func (p *CircuitPool) Close() error {
	p.cancel()
	p.wg.Wait()

	p.mu.Lock()
	defer p.mu.Unlock()

	// Close all circuits in main pool
	for _, circ := range p.circuits {
		p.logger.Debug("Closing pooled circuit", "circuit_id", circ.ID)
		circ.SetState(circuit.StateClosed)
	}
	p.circuits = nil

	// Close all isolated circuits
	for key, poolCircuits := range p.isolatedCircuits {
		for _, circ := range poolCircuits {
			p.logger.Debug("Closing isolated circuit", "circuit_id", circ.ID, "isolation_key", key)
			circ.SetState(circuit.StateClosed)
		}
		delete(p.isolatedCircuits, key)
	}

	return nil
}

// CircuitPoolStats holds statistics about the circuit pool
type CircuitPoolStats struct {
	Total            int // Total circuits across all pools
	Open             int // Open circuits across all pools
	MinCircuits      int
	MaxCircuits      int
	IsolatedPools    int // Number of isolated circuit pools
	IsolatedCircuits int // Total circuits in isolated pools
}
