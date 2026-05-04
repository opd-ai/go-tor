// Package connection provides connection-level padding for traffic analysis resistance.
// This implements link-level padding per padding-spec.txt, complementing circuit-level padding.
package connection

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/opd-ai/go-tor/pkg/cell"
)

// ConnectionPaddingStrategy defines padding behavior for a connection.
type ConnectionPaddingStrategy int

const (
	// ConnectionPaddingNone disables connection-level padding.
	ConnectionPaddingNone ConnectionPaddingStrategy = iota

	// ConnectionPaddingFixed sends padding at fixed intervals.
	ConnectionPaddingFixed

	// ConnectionPaddingRandom sends padding at random intervals within a range.
	ConnectionPaddingRandom

	// ConnectionPaddingAdaptive adjusts padding based on connection activity.
	ConnectionPaddingAdaptive
)

// String returns a human-readable name for the padding strategy.
func (s ConnectionPaddingStrategy) String() string {
	switch s {
	case ConnectionPaddingNone:
		return "none"
	case ConnectionPaddingFixed:
		return "fixed"
	case ConnectionPaddingRandom:
		return "random"
	case ConnectionPaddingAdaptive:
		return "adaptive"
	default:
		return fmt.Sprintf("unknown(%d)", s)
	}
}

// ConnectionPaddingConfig configures connection-level padding.
type ConnectionPaddingConfig struct {
	// Strategy determines how padding cells are scheduled.
	Strategy ConnectionPaddingStrategy

	// MinInterval is the minimum time between padding cells.
	MinInterval time.Duration

	// MaxInterval is the maximum time between padding cells.
	MaxInterval time.Duration

	// IdleTimeout is how long a connection must be idle before padding begins.
	IdleTimeout time.Duration

	// UseVariableLength enables VPADDING cells instead of fixed PADDING cells.
	// VPADDING cells can have variable payload sizes, making traffic analysis harder.
	UseVariableLength bool
}

// DefaultConnectionPaddingConfig returns sensible defaults for connection padding.
func DefaultConnectionPaddingConfig() *ConnectionPaddingConfig {
	return &ConnectionPaddingConfig{
		Strategy:          ConnectionPaddingRandom,
		MinInterval:       5 * time.Second,
		MaxInterval:       15 * time.Second,
		IdleTimeout:       2 * time.Second,
		UseVariableLength: false,
	}
}

// Validate checks if the padding configuration is valid.
func (c *ConnectionPaddingConfig) Validate() error {
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
	if c.Strategy < ConnectionPaddingNone || c.Strategy > ConnectionPaddingAdaptive {
		return fmt.Errorf("invalid padding strategy: %d", c.Strategy)
	}
	return nil
}

// Clone creates a deep copy of the padding configuration.
func (c *ConnectionPaddingConfig) Clone() *ConnectionPaddingConfig {
	return &ConnectionPaddingConfig{
		Strategy:          c.Strategy,
		MinInterval:       c.MinInterval,
		MaxInterval:       c.MaxInterval,
		IdleTimeout:       c.IdleTimeout,
		UseVariableLength: c.UseVariableLength,
	}
}

// ConnectionPaddingMachine manages connection-level padding for traffic analysis resistance.
type ConnectionPaddingMachine struct {
	conn   *Connection
	config *ConnectionPaddingConfig
	mu     sync.RWMutex

	// Runtime state
	running          atomic.Bool
	stopChan         chan struct{}
	lastActivityTime time.Time
	activityBursts   int // Track recent activity for adaptive strategy

	// Metrics
	paddingsSent   atomic.Uint64
	vpaddingsSent  atomic.Uint64
	failedPaddings atomic.Uint64
}

// NewConnectionPaddingMachine creates a new connection padding machine.
func NewConnectionPaddingMachine(conn *Connection, config *ConnectionPaddingConfig) (*ConnectionPaddingMachine, error) {
	if conn == nil {
		return nil, errors.New("connection cannot be nil")
	}
	if config == nil {
		config = DefaultConnectionPaddingConfig()
	}
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &ConnectionPaddingMachine{
		conn:             conn,
		config:           config.Clone(),
		stopChan:         make(chan struct{}),
		lastActivityTime: time.Now(),
	}, nil
}

// Start begins the padding machine background goroutine.
func (pm *ConnectionPaddingMachine) Start(ctx context.Context) error {
	if pm.running.Swap(true) {
		return errors.New("padding machine already running")
	}

	pm.mu.Lock()
	pm.stopChan = make(chan struct{})
	pm.mu.Unlock()

	go pm.run(ctx)
	return nil
}

// Stop stops the padding machine.
func (pm *ConnectionPaddingMachine) Stop() {
	if !pm.running.Swap(false) {
		return
	}
	pm.mu.Lock()
	close(pm.stopChan)
	pm.mu.Unlock()
}

// IsRunning returns whether the padding machine is currently running.
func (pm *ConnectionPaddingMachine) IsRunning() bool {
	return pm.running.Load()
}

// UpdateConfig updates the padding configuration.
func (pm *ConnectionPaddingMachine) UpdateConfig(config *ConnectionPaddingConfig) error {
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
func (pm *ConnectionPaddingMachine) GetConfig() *ConnectionPaddingConfig {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.config.Clone()
}

// RecordActivity records that activity occurred on the connection.
func (pm *ConnectionPaddingMachine) RecordActivity() {
	pm.mu.Lock()
	pm.lastActivityTime = time.Now()
	pm.activityBursts++
	if pm.activityBursts > 10 {
		pm.activityBursts = 10
	}
	pm.mu.Unlock()
}

// Stats returns padding machine statistics.
func (pm *ConnectionPaddingMachine) Stats() ConnectionPaddingStats {
	return ConnectionPaddingStats{
		PaddingsSent:   pm.paddingsSent.Load(),
		VPaddingsSent:  pm.vpaddingsSent.Load(),
		FailedPaddings: pm.failedPaddings.Load(),
	}
}

// ConnectionPaddingStats contains metrics about padding operation.
type ConnectionPaddingStats struct {
	PaddingsSent   uint64
	VPaddingsSent  uint64
	FailedPaddings uint64
}

// run is the main loop for the padding machine.
func (pm *ConnectionPaddingMachine) run(ctx context.Context) {
	defer pm.running.Store(false)

	delay := pm.calculateNextDelay()

	for {
		if !pm.running.Load() {
			return
		}

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
func (pm *ConnectionPaddingMachine) calculateNextDelay() time.Duration {
	pm.mu.RLock()
	config := pm.config
	pm.mu.RUnlock()

	switch config.Strategy {
	case ConnectionPaddingNone:
		return time.Hour

	case ConnectionPaddingFixed:
		return config.MinInterval

	case ConnectionPaddingRandom:
		return pm.randomDuration(config.MinInterval, config.MaxInterval)

	case ConnectionPaddingAdaptive:
		// Adaptive: reduce padding during active periods
		pm.mu.Lock()
		inBurst := pm.activityBursts > 0 && time.Since(pm.lastActivityTime) < config.MaxInterval
		if inBurst {
			pm.activityBursts--
		}
		pm.mu.Unlock()

		if inBurst {
			return pm.randomDuration(config.MaxInterval, config.MaxInterval*2)
		}
		// Quiet period - more aggressive padding
		return pm.randomDuration(config.MinInterval, config.MinInterval*2)

	default:
		return config.MinInterval
	}
}

// randomDuration returns a cryptographically random duration between min and max.
func (pm *ConnectionPaddingMachine) randomDuration(min, max time.Duration) time.Duration {
	if min >= max {
		return min
	}

	rangeSize := uint64(max - min)
	if rangeSize == 0 {
		return min
	}

	// Use rejection sampling to avoid modulo bias
	maxVal := ^uint64(0)
	limit := maxVal - (maxVal % rangeSize)

	for {
		var buf [8]byte
		if _, err := rand.Read(buf[:]); err != nil {
			return min
		}

		n := binary.BigEndian.Uint64(buf[:])
		if n < limit {
			return min + time.Duration(n%rangeSize)
		}
	}
}

// shouldSendPadding checks if padding should be sent now.
func (pm *ConnectionPaddingMachine) shouldSendPadding() bool {
	pm.mu.RLock()
	config := pm.config
	lastActivity := pm.lastActivityTime
	pm.mu.RUnlock()

	if config.Strategy == ConnectionPaddingNone {
		return false
	}

	if pm.conn.GetState() != StateOpen {
		return false
	}

	if time.Since(lastActivity) < config.IdleTimeout {
		return false
	}

	return true
}

// sendPadding sends a padding cell on the connection.
func (pm *ConnectionPaddingMachine) sendPadding() {
	pm.mu.RLock()
	config := pm.config
	pm.mu.RUnlock()

	var err error
	if config.UseVariableLength {
		err = pm.sendVPaddingCell()
	} else {
		err = pm.sendPaddingCell()
	}

	if err != nil {
		pm.failedPaddings.Add(1)
	}
}

// sendPaddingCell sends a fixed-size PADDING cell.
func (pm *ConnectionPaddingMachine) sendPaddingCell() error {
	payload := make([]byte, cell.PayloadLen)
	if _, err := rand.Read(payload); err != nil {
		return fmt.Errorf("failed to generate padding: %w", err)
	}

	paddingCell := &cell.Cell{
		CircID:  0, // Circuit ID 0 for connection-level padding
		Command: cell.CmdPadding,
		Payload: payload,
	}

	if err := pm.conn.SendCell(paddingCell); err != nil {
		return fmt.Errorf("failed to send padding cell: %w", err)
	}

	pm.paddingsSent.Add(1)
	return nil
}

// sendVPaddingCell sends a variable-length VPADDING cell.
func (pm *ConnectionPaddingMachine) sendVPaddingCell() error {
	// VPADDING cells can have variable payload sizes
	// Use random size between 100 and PayloadLen for variety
	minSize := 100
	maxSize := cell.PayloadLen
	size := pm.randomRange(minSize, maxSize)

	payload := make([]byte, size)
	if _, err := rand.Read(payload); err != nil {
		return fmt.Errorf("failed to generate vpadding: %w", err)
	}

	vpaddingCell := &cell.Cell{
		CircID:  0, // Circuit ID 0 for connection-level padding
		Command: cell.CmdVPadding,
		Payload: payload,
	}

	if err := pm.conn.SendCell(vpaddingCell); err != nil {
		return fmt.Errorf("failed to send vpadding cell: %w", err)
	}

	pm.vpaddingsSent.Add(1)
	return nil
}

// randomRange returns a random integer in [min, max].
func (pm *ConnectionPaddingMachine) randomRange(min, max int) int {
	if min >= max {
		return min
	}
	rangeSize := uint32(max - min + 1)
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return min
	}
	n := binary.BigEndian.Uint32(buf[:])
	return min + int(n%rangeSize)
}

// HandlePaddingCell processes an incoming PADDING cell (connection-level).
// Per tor-spec.txt §7.1, PADDING cells should be silently discarded.
func HandleConnectionPaddingCell(_ *cell.Cell) {
	// PADDING/VPADDING cells are silently ignored per spec
}
