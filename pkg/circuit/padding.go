// Package circuit provides circuit management for the Tor protocol.
// This file implements circuit padding for traffic analysis resistance
// per tor-spec.txt §7.1 and padding-spec.txt.
package circuit

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/opd-ai/go-tor/pkg/cell"
)

// CellSender is implemented by types that can send Tor cells.
// This interface is used by PaddingMachine to send padding cells.
type CellSender interface {
	SendCell(*cell.Cell) error
}

// PaddingStrategy defines the padding behavior for a circuit.
type PaddingStrategy int

const (
	// PaddingStrategyNone disables padding entirely.
	PaddingStrategyNone PaddingStrategy = iota

	// PaddingStrategyFixed sends padding at fixed intervals.
	PaddingStrategyFixed

	// PaddingStrategyRandom sends padding at random intervals within a range.
	PaddingStrategyRandom

	// PaddingStrategyAdaptive adjusts padding based on traffic patterns.
	PaddingStrategyAdaptive
)

// String returns a human-readable name for the padding strategy.
func (s PaddingStrategy) String() string {
	switch s {
	case PaddingStrategyNone:
		return "none"
	case PaddingStrategyFixed:
		return "fixed"
	case PaddingStrategyRandom:
		return "random"
	case PaddingStrategyAdaptive:
		return "adaptive"
	default:
		return fmt.Sprintf("unknown(%d)", s)
	}
}

// PaddingConfig configures circuit padding behavior for traffic analysis resistance.
type PaddingConfig struct {
	// Strategy determines how padding cells are scheduled.
	Strategy PaddingStrategy

	// MinInterval is the minimum time between padding cells.
	// For random/adaptive strategies, actual interval is between MinInterval and MaxInterval.
	MinInterval time.Duration

	// MaxInterval is the maximum time between padding cells.
	// For fixed strategy, only MinInterval is used.
	MaxInterval time.Duration

	// IdleTimeout is how long a circuit must be idle before padding begins.
	// This prevents redundant padding during active use.
	IdleTimeout time.Duration

	// DummyTrafficEnabled enables dummy RELAY_DATA cells (vs PADDING cells).
	// Dummy traffic is harder to distinguish from real traffic.
	DummyTrafficEnabled bool

	// BurstSize is the number of padding cells to send in quick succession.
	// For adaptive strategy, this mimics real traffic bursts.
	BurstSize int
}

// DefaultPaddingConfig returns sensible defaults for circuit padding.
// These values provide reasonable traffic analysis resistance without
// excessive bandwidth overhead.
func DefaultPaddingConfig() *PaddingConfig {
	return &PaddingConfig{
		Strategy:            PaddingStrategyRandom,
		MinInterval:         3 * time.Second,
		MaxInterval:         10 * time.Second,
		IdleTimeout:         time.Second,
		DummyTrafficEnabled: false,
		BurstSize:           1,
	}
}

// Validate checks if the padding configuration is valid.
func (c *PaddingConfig) Validate() error {
	if c.MinInterval < 0 {
		return errors.New("MinInterval must be non-negative")
	}
	if c.MaxInterval < 0 {
		return errors.New("MaxInterval must be non-negative")
	}
	if c.MaxInterval > 0 && c.MaxInterval < c.MinInterval {
		return errors.New("MaxInterval must be >= MinInterval")
	}
	if c.IdleTimeout < 0 {
		return errors.New("IdleTimeout must be non-negative")
	}
	if c.BurstSize < 0 {
		return errors.New("BurstSize must be non-negative")
	}
	if c.Strategy < PaddingStrategyNone || c.Strategy > PaddingStrategyAdaptive {
		return fmt.Errorf("invalid padding strategy: %d", c.Strategy)
	}
	return nil
}

// Clone creates a deep copy of the padding configuration.
func (c *PaddingConfig) Clone() *PaddingConfig {
	return &PaddingConfig{
		Strategy:            c.Strategy,
		MinInterval:         c.MinInterval,
		MaxInterval:         c.MaxInterval,
		IdleTimeout:         c.IdleTimeout,
		DummyTrafficEnabled: c.DummyTrafficEnabled,
		BurstSize:           c.BurstSize,
	}
}

// PaddingMachine manages circuit padding for traffic analysis resistance.
// It generates PADDING cells according to the configured strategy.
type PaddingMachine struct {
	circuit *Circuit
	config  *PaddingConfig
	mu      sync.RWMutex

	// Runtime state
	running       atomic.Bool
	stopChan      chan struct{}
	nextPaddingAt time.Time
	trafficBursts int // Track recent traffic bursts for adaptive strategy

	// Metrics
	paddingsSent   atomic.Uint64
	dummyDataSent  atomic.Uint64
	failedPaddings atomic.Uint64
}

// NewPaddingMachine creates a new padding machine for a circuit.
func NewPaddingMachine(circuit *Circuit, config *PaddingConfig) (*PaddingMachine, error) {
	if circuit == nil {
		return nil, errors.New("circuit cannot be nil")
	}
	if config == nil {
		config = DefaultPaddingConfig()
	}
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &PaddingMachine{
		circuit:  circuit,
		config:   config.Clone(),
		stopChan: make(chan struct{}),
	}, nil
}

// Start begins the padding machine background goroutine.
// It runs until Stop is called or the context is cancelled.
func (pm *PaddingMachine) Start(ctx context.Context) error {
	if pm.running.Swap(true) {
		return errors.New("padding machine already running")
	}

	// Reset stop channel if restarting
	pm.mu.Lock()
	pm.stopChan = make(chan struct{})
	pm.mu.Unlock()

	go pm.run(ctx)
	return nil
}

// Stop stops the padding machine.
func (pm *PaddingMachine) Stop() {
	if !pm.running.Swap(false) {
		return // Already stopped
	}
	pm.mu.Lock()
	close(pm.stopChan)
	pm.mu.Unlock()
}

// IsRunning returns whether the padding machine is currently running.
func (pm *PaddingMachine) IsRunning() bool {
	return pm.running.Load()
}

// UpdateConfig updates the padding configuration.
// This can be called while the machine is running.
func (pm *PaddingMachine) UpdateConfig(config *PaddingConfig) error {
	if config == nil {
		return errors.New("config cannot be nil")
	}
	if err := config.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	pm.mu.Lock()
	pm.config = config.Clone()
	pm.mu.Unlock()
	return nil
}

// GetConfig returns a copy of the current padding configuration.
func (pm *PaddingMachine) GetConfig() *PaddingConfig {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.config.Clone()
}

// Stats returns padding machine statistics.
func (pm *PaddingMachine) Stats() PaddingStats {
	return PaddingStats{
		PaddingsSent:   pm.paddingsSent.Load(),
		DummyDataSent:  pm.dummyDataSent.Load(),
		FailedPaddings: pm.failedPaddings.Load(),
	}
}

// PaddingStats contains metrics about padding machine operation.
type PaddingStats struct {
	PaddingsSent   uint64
	DummyDataSent  uint64
	FailedPaddings uint64
}

// run is the main loop for the padding machine.
func (pm *PaddingMachine) run(ctx context.Context) {
	defer pm.running.Store(false)

	// Calculate initial delay
	delay := pm.calculateNextDelay()

	for {
		pm.mu.RLock()
		stopChan := pm.stopChan
		pm.mu.RUnlock()

		select {
		case <-ctx.Done():
			return
		case <-stopChan:
			return
		case <-time.After(delay):
			if pm.shouldSendPadding() {
				pm.sendPadding()
			}
			delay = pm.calculateNextDelay()
		}
	}
}

// calculateNextDelay determines when to send the next padding cell.
func (pm *PaddingMachine) calculateNextDelay() time.Duration {
	pm.mu.RLock()
	config := pm.config
	pm.mu.RUnlock()

	switch config.Strategy {
	case PaddingStrategyNone:
		return time.Hour // Effectively disabled

	case PaddingStrategyFixed:
		return config.MinInterval

	case PaddingStrategyRandom:
		return pm.randomDuration(config.MinInterval, config.MaxInterval)

	case PaddingStrategyAdaptive:
		// Adaptive: shorter intervals during quiet periods,
		// longer when there's real traffic
		if pm.trafficBursts > 0 {
			// Recent real traffic - reduce padding
			pm.trafficBursts--
			// Cap maximum delay at 5 minutes to prevent effectively disabling padding
			maxCap := 5 * time.Minute
			minDelay := config.MaxInterval
			maxDelay := config.MaxInterval * 2
			if maxDelay > maxCap {
				maxDelay = maxCap
			}
			if minDelay > maxCap {
				minDelay = maxCap
			}
			return pm.randomDuration(minDelay, maxDelay)
		}
		// Quiet period - more aggressive padding
		return pm.randomDuration(config.MinInterval, config.MinInterval*2)

	default:
		return config.MinInterval
	}
}

// randomDuration returns a cryptographically random duration between min and max.
func (pm *PaddingMachine) randomDuration(min, max time.Duration) time.Duration {
	if min >= max {
		return min
	}

	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		// Fall back to min on error
		return min
	}

	// Convert to uint64 and scale to range
	n := binary.BigEndian.Uint64(buf[:])
	rangeSize := uint64(max - min)
	return min + time.Duration(n%rangeSize)
}

// shouldSendPadding checks if padding should be sent now.
func (pm *PaddingMachine) shouldSendPadding() bool {
	pm.mu.RLock()
	config := pm.config
	pm.mu.RUnlock()

	// Check strategy
	if config.Strategy == PaddingStrategyNone {
		return false
	}

	// Check circuit state
	if pm.circuit.GetState() != StateOpen {
		return false
	}

	// Check idle timeout
	pm.circuit.mu.RLock()
	lastActivity := pm.circuit.lastActivityTime
	pm.circuit.mu.RUnlock()

	if time.Since(lastActivity) < config.IdleTimeout {
		return false // Circuit is active, don't pad
	}

	return true
}

// sendPadding sends one or more padding cells based on configuration.
func (pm *PaddingMachine) sendPadding() {
	pm.mu.RLock()
	config := pm.config
	pm.mu.RUnlock()

	burstSize := config.BurstSize
	if burstSize < 1 {
		burstSize = 1
	}

	for i := 0; i < burstSize; i++ {
		var err error
		if config.DummyTrafficEnabled {
			err = pm.sendDummyData()
		} else {
			err = pm.sendPaddingCell()
		}

		if err != nil {
			pm.failedPaddings.Add(1)
			return // Stop burst on error
		}
	}
}

// sendPaddingCell sends a PADDING cell on the circuit.
func (pm *PaddingMachine) sendPaddingCell() error {
	// Create a PADDING cell with random payload
	paddingCell := NewPaddingCell(pm.circuit.ID)

	// Send through circuit connection
	pm.circuit.mu.RLock()
	conn := pm.circuit.conn
	pm.circuit.mu.RUnlock()

	if conn == nil {
		return errors.New("circuit has no connection")
	}

	// Type assert to CellSender interface (defined at package level)
	sender, ok := conn.(CellSender)
	if !ok {
		return errors.New("connection does not support SendCell")
	}

	if err := sender.SendCell(paddingCell); err != nil {
		return fmt.Errorf("failed to send padding cell: %w", err)
	}

	pm.paddingsSent.Add(1)
	pm.circuit.RecordPaddingSent()
	return nil
}

// sendDummyData sends a dummy RELAY_DATA cell on the circuit.
// This is harder to distinguish from real traffic than PADDING cells.
//
// WARNING: This uses stream ID 0 which is typically reserved for circuit-level
// control traffic. The exit relay should recognize and drop these cells since
// they don't correspond to any real stream. However, use with caution as this
// may cause confusion with circuit control messages in some implementations.
// For production use, consider using standard PADDING cells instead by setting
// DummyTrafficEnabled to false in PaddingConfig.
func (pm *PaddingMachine) sendDummyData() error {
	// Generate random dummy payload
	dummyData := make([]byte, 498) // Max RELAY_DATA payload size
	if _, err := rand.Read(dummyData); err != nil {
		return fmt.Errorf("failed to generate dummy data: %w", err)
	}

	// Create a RELAY_DATA cell with stream ID 0 (circuit-level)
	// Stream ID 0 is typically used for circuit-level control/padding.
	// Exit relays should drop DATA cells with stream ID 0 since they
	// don't correspond to any established stream.
	dummyCell := cell.NewRelayCell(0, cell.RelayData, dummyData)

	// Send through circuit
	if err := pm.circuit.SendRelayCell(dummyCell); err != nil {
		return fmt.Errorf("failed to send dummy data: %w", err)
	}

	pm.dummyDataSent.Add(1)
	pm.circuit.RecordPaddingSent()
	return nil
}

// RecordTrafficBurst records that real traffic occurred.
// Used by adaptive strategy to reduce padding during active use.
func (pm *PaddingMachine) RecordTrafficBurst() {
	pm.mu.Lock()
	pm.trafficBursts++
	if pm.trafficBursts > 10 {
		pm.trafficBursts = 10 // Cap to prevent overflow
	}
	pm.mu.Unlock()
}

// NewPaddingCell creates a new PADDING cell with random payload.
// Per tor-spec.txt §7.1, PADDING cells are used to defeat traffic analysis.
func NewPaddingCell(circuitID uint32) *cell.Cell {
	// Create random padding payload
	payload := make([]byte, cell.PayloadLen)
	if _, err := rand.Read(payload); err != nil {
		// Log warning but continue with zeros - padding still provides some benefit
		slog.Warn("crypto/rand failure in padding cell generation", "error", err)
	}

	return &cell.Cell{
		CircID:  circuitID,
		Command: cell.CmdPadding,
		Payload: payload,
	}
}

// HandlePaddingCell processes an incoming PADDING cell.
// Per tor-spec.txt §7.1, PADDING cells should be silently discarded.
func HandlePaddingCell(_ *cell.Cell) {
	// PADDING cells are silently ignored per spec
	// No action needed - the cell is simply discarded
}

// AddRandomTimingDelay adds a random delay before sending a cell.
// This helps defeat timing correlation attacks.
func AddRandomTimingDelay(minDelay, maxDelay time.Duration) {
	if minDelay >= maxDelay || maxDelay <= 0 {
		return
	}

	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return
	}

	n := binary.BigEndian.Uint64(buf[:])
	rangeSize := uint64(maxDelay - minDelay)
	delay := minDelay + time.Duration(n%rangeSize)

	time.Sleep(delay)
}
