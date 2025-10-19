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
		circuits:        make([]*circuit.Circuit, 0, cfg.MaxCircuits),
		minCircuits:     cfg.MinCircuits,
		maxCircuits:     cfg.MaxCircuits,
		buildFunc:       builder,
		logger:          log.Component("circuit-pool"),
		prebuildEnabled: cfg.PrebuildEnabled,
		ctx:             ctx,
		cancel:          cancel,
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
	p.mu.Lock()
	defer p.mu.Unlock()

	// Try to get a healthy circuit from the pool
	for len(p.circuits) > 0 {
		circ := p.circuits[0]
		p.circuits = p.circuits[1:]

		// Check if circuit is still open
		if circ.GetState() == circuit.StateOpen {
			p.logger.Debug("Retrieved circuit from pool", "circuit_id", circ.ID)
			return circ, nil
		}

		// Circuit is not open, discard it
		p.logger.Debug("Discarding closed circuit from pool", "circuit_id", circ.ID, "state", circ.GetState())
	}

	// No circuits available, build a new one
	p.logger.Debug("No circuits in pool, building new circuit")
	return p.buildFunc(ctx)
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

	// Check if we're at capacity
	if len(p.circuits) >= p.maxCircuits {
		p.logger.Debug("Circuit pool at capacity, not returning circuit", "circuit_id", circ.ID)
		return
	}

	p.circuits = append(p.circuits, circ)
	p.logger.Debug("Returned circuit to pool", "circuit_id", circ.ID, "pool_size", len(p.circuits))
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
		Total:       len(p.circuits),
		MinCircuits: p.minCircuits,
		MaxCircuits: p.maxCircuits,
	}

	for _, circ := range p.circuits {
		if circ.GetState() == circuit.StateOpen {
			stats.Open++
		}
	}

	return stats
}

// Close closes the circuit pool and cleans up resources
func (p *CircuitPool) Close() error {
	p.cancel()
	p.wg.Wait()

	p.mu.Lock()
	defer p.mu.Unlock()

	// Close all circuits
	for _, circ := range p.circuits {
		p.logger.Debug("Closing pooled circuit", "circuit_id", circ.ID)
		circ.SetState(circuit.StateClosed)
	}
	p.circuits = nil

	return nil
}

// CircuitPoolStats holds statistics about the circuit pool
type CircuitPoolStats struct {
	Total       int
	Open        int
	MinCircuits int
	MaxCircuits int
}
