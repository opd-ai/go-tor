// Package chaos provides chaos engineering testing infrastructure for go-tor.
// It enables testing system resilience through simulated failures, network
// problems, and resource constraints.
//
// Chaos tests help verify that the system:
// - Recovers gracefully from network failures
// - Handles timeouts appropriately
// - Maintains functionality under degraded conditions
// - Properly releases resources during failures
//
// Tests in this package are designed to run with the "integration" build tag:
//
//	go test -tags=integration ./pkg/testing/chaos/...
package chaos

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/opd-ai/go-tor/pkg/logger"
)

// Config holds configuration for chaos testing.
type Config struct {
	FailureRate      float64       // Probability of failure (0.0-1.0)
	LatencyMin       time.Duration // Minimum added latency
	LatencyMax       time.Duration // Maximum added latency
	ConnectionDrops  float64       // Probability of connection drop (0.0-1.0)
	TimeoutDuration  time.Duration // Duration after which operations timeout
	RecoveryInterval time.Duration // Time between recovery attempts
}

// DefaultConfig returns sensible defaults for chaos testing.
func DefaultConfig() *Config {
	return &Config{
		FailureRate:      0.1, // 10% failure rate
		LatencyMin:       10 * time.Millisecond,
		LatencyMax:       100 * time.Millisecond,
		ConnectionDrops:  0.05, // 5% connection drop rate
		TimeoutDuration:  5 * time.Second,
		RecoveryInterval: 100 * time.Millisecond,
	}
}

// AggressiveConfig returns a more aggressive chaos configuration.
func AggressiveConfig() *Config {
	return &Config{
		FailureRate:      0.3, // 30% failure rate
		LatencyMin:       50 * time.Millisecond,
		LatencyMax:       500 * time.Millisecond,
		ConnectionDrops:  0.15, // 15% connection drop rate
		TimeoutDuration:  2 * time.Second,
		RecoveryInterval: 50 * time.Millisecond,
	}
}

// Engine provides chaos engineering capabilities.
type Engine struct {
	mu      sync.RWMutex
	config  *Config
	logger  *logger.Logger
	enabled bool
	paused  bool

	// Statistics
	failuresInjected int64
	latencyInjected  int64
	dropsInjected    int64
	timeoutsInjected int64

	// Random source
	rand *rand.Rand
}

// NewEngine creates a new chaos engine with the given configuration.
func NewEngine(cfg *Config, log *logger.Logger) *Engine {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	if log == nil {
		log = logger.NewDefault()
	}

	return &Engine{
		config:  cfg,
		logger:  log,
		enabled: false,
		rand:    rand.New(rand.NewSource(time.Now().UnixNano())), //nolint:gosec // Cryptographic randomness not needed for chaos testing
	}
}

// Enable turns on chaos injection.
func (e *Engine) Enable() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.enabled = true
	e.logger.Info("Chaos engine enabled", "failureRate", e.config.FailureRate)
}

// Disable turns off chaos injection.
func (e *Engine) Disable() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.enabled = false
	e.logger.Info("Chaos engine disabled")
}

// Pause temporarily suspends chaos injection.
func (e *Engine) Pause() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.paused = true
}

// Resume resumes chaos injection after a pause.
func (e *Engine) Resume() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.paused = false
}

// IsActive returns true if chaos injection is currently active.
func (e *Engine) IsActive() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.enabled && !e.paused
}

// MaybeInjectFailure potentially injects a failure based on the configured rate.
// Returns an error if a failure was injected, nil otherwise.
func (e *Engine) MaybeInjectFailure() error {
	if !e.IsActive() {
		return nil
	}

	e.mu.Lock()
	shouldFail := e.rand.Float64() < e.config.FailureRate
	e.mu.Unlock()

	if shouldFail {
		atomic.AddInt64(&e.failuresInjected, 1)
		e.logger.Debug("Chaos: injecting failure")
		return ErrChaosFailure
	}

	return nil
}

// MaybeInjectLatency potentially adds latency based on the configured settings.
func (e *Engine) MaybeInjectLatency() {
	e.mu.Lock()
	if !e.enabled || e.paused {
		e.mu.Unlock()
		return
	}

	latencyRange := e.config.LatencyMax - e.config.LatencyMin
	latency := e.config.LatencyMin
	if latencyRange > 0 {
		latency += time.Duration(e.rand.Int63n(int64(latencyRange)))
	}
	atomic.AddInt64(&e.latencyInjected, 1)
	e.mu.Unlock()

	e.logger.Debug("Chaos: injecting latency", "duration", latency)
	time.Sleep(latency)
}

// MaybeInjectDrop potentially simulates a connection drop.
// Returns true if a drop was injected.
func (e *Engine) MaybeInjectDrop() bool {
	if !e.IsActive() {
		return false
	}

	e.mu.Lock()
	shouldDrop := e.rand.Float64() < e.config.ConnectionDrops
	e.mu.Unlock()

	if shouldDrop {
		atomic.AddInt64(&e.dropsInjected, 1)
		e.logger.Debug("Chaos: injecting connection drop")
		return true
	}

	return false
}

// WrapContext wraps a context with timeout for chaos testing.
func (e *Engine) WrapContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if !e.IsActive() {
		return ctx, func() {}
	}

	return context.WithTimeout(ctx, e.config.TimeoutDuration)
}

// MaybeTimeout potentially forces an operation to timeout.
func (e *Engine) MaybeTimeout(ctx context.Context) error {
	if !e.IsActive() {
		return nil
	}

	e.mu.Lock()
	// 20% chance to force timeout when chaos is active
	shouldTimeout := e.rand.Float64() < 0.2
	e.mu.Unlock()

	if shouldTimeout {
		atomic.AddInt64(&e.timeoutsInjected, 1)
		e.logger.Debug("Chaos: forcing timeout")
		return context.DeadlineExceeded
	}

	return nil
}

// Stats returns the current chaos injection statistics.
func (e *Engine) Stats() Stats {
	return Stats{
		FailuresInjected: atomic.LoadInt64(&e.failuresInjected),
		LatencyInjected:  atomic.LoadInt64(&e.latencyInjected),
		DropsInjected:    atomic.LoadInt64(&e.dropsInjected),
		TimeoutsInjected: atomic.LoadInt64(&e.timeoutsInjected),
	}
}

// ResetStats resets all statistics to zero.
func (e *Engine) ResetStats() {
	atomic.StoreInt64(&e.failuresInjected, 0)
	atomic.StoreInt64(&e.latencyInjected, 0)
	atomic.StoreInt64(&e.dropsInjected, 0)
	atomic.StoreInt64(&e.timeoutsInjected, 0)
}

// Stats holds chaos injection statistics.
type Stats struct {
	FailuresInjected int64
	LatencyInjected  int64
	DropsInjected    int64
	TimeoutsInjected int64
}

// Total returns the total number of chaos events injected.
func (s Stats) Total() int64 {
	return s.FailuresInjected + s.LatencyInjected + s.DropsInjected + s.TimeoutsInjected
}

// Common chaos-related errors.
var (
	ErrChaosFailure = errors.New("chaos: injected failure")
	ErrChaosDrop    = errors.New("chaos: connection dropped")
	ErrChaosTimeout = errors.New("chaos: operation timed out")
)

// NetworkFaultInjector simulates network-level faults.
type NetworkFaultInjector struct {
	mu          sync.RWMutex
	randMu      sync.Mutex // Separate mutex for rand to allow concurrent config reads
	enabled     bool
	partitioned bool
	packetLoss  float64
	bandwidth   int64 // bytes per second, 0 = unlimited
	logger      *logger.Logger
	rand        *rand.Rand

	// Statistics
	packetsDropped int64
	bytesThrottled int64
}

// NewNetworkFaultInjector creates a new network fault injector.
func NewNetworkFaultInjector(log *logger.Logger) *NetworkFaultInjector {
	if log == nil {
		log = logger.NewDefault()
	}
	return &NetworkFaultInjector{
		logger: log,
		rand:   rand.New(rand.NewSource(time.Now().UnixNano())), //nolint:gosec // G404: Use of weak random number generator is acceptable for network fault simulation
	}
}

// Enable turns on network fault injection.
func (n *NetworkFaultInjector) Enable() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.enabled = true
}

// Disable turns off network fault injection.
func (n *NetworkFaultInjector) Disable() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.enabled = false
}

// SetPacketLoss sets the packet loss rate (0.0-1.0).
func (n *NetworkFaultInjector) SetPacketLoss(rate float64) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.packetLoss = rate
}

// SetBandwidth sets the bandwidth limit in bytes per second.
// 0 means unlimited.
func (n *NetworkFaultInjector) SetBandwidth(bytesPerSecond int64) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.bandwidth = bytesPerSecond
}

// Partition simulates a network partition.
func (n *NetworkFaultInjector) Partition() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.partitioned = true
	n.logger.Info("Network partition simulated")
}

// Heal removes a simulated network partition.
func (n *NetworkFaultInjector) Heal() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.partitioned = false
	n.logger.Info("Network partition healed")
}

// IsPartitioned returns true if the network is currently partitioned.
func (n *NetworkFaultInjector) IsPartitioned() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.partitioned
}

// ShouldDropPacket returns true if the packet should be dropped.
func (n *NetworkFaultInjector) ShouldDropPacket() bool {
	n.mu.RLock()
	enabled := n.enabled
	partitioned := n.partitioned
	packetLoss := n.packetLoss
	n.mu.RUnlock()

	if !enabled {
		return false
	}

	if partitioned {
		atomic.AddInt64(&n.packetsDropped, 1)
		return true
	}

	// Use separate mutex for rand access
	n.randMu.Lock()
	shouldDrop := n.rand.Float64() < packetLoss
	n.randMu.Unlock()

	if shouldDrop {
		atomic.AddInt64(&n.packetsDropped, 1)
		return true
	}

	return false
}

// ThrottleDelay calculates the delay needed to throttle bandwidth.
func (n *NetworkFaultInjector) ThrottleDelay(bytes int64) time.Duration {
	n.mu.RLock()
	defer n.mu.RUnlock()

	if !n.enabled || n.bandwidth <= 0 {
		return 0
	}

	// Calculate delay: bytes / (bytes/second) = seconds
	delay := time.Duration(float64(bytes) / float64(n.bandwidth) * float64(time.Second))
	atomic.AddInt64(&n.bytesThrottled, bytes)
	return delay
}

// Stats returns network fault injection statistics.
func (n *NetworkFaultInjector) Stats() NetworkFaultStats {
	return NetworkFaultStats{
		PacketsDropped: atomic.LoadInt64(&n.packetsDropped),
		BytesThrottled: atomic.LoadInt64(&n.bytesThrottled),
	}
}

// NetworkFaultStats holds network fault statistics.
type NetworkFaultStats struct {
	PacketsDropped int64
	BytesThrottled int64
}

// RelaySimulator simulates Tor relay behavior for testing.
type RelaySimulator struct {
	mu         sync.RWMutex
	running    bool
	healthy    bool
	overloaded bool
	logger     *logger.Logger
	rand       *rand.Rand

	// Behavior configuration
	responseDelay     time.Duration
	failureRate       float64
	overloadThreshold int32

	// Current state
	activeConnections int32
}

// NewRelaySimulator creates a new relay simulator.
func NewRelaySimulator(log *logger.Logger) *RelaySimulator {
	if log == nil {
		log = logger.NewDefault()
	}
	return &RelaySimulator{
		logger:            log,
		healthy:           true,
		overloadThreshold: 100,
		rand:              rand.New(rand.NewSource(time.Now().UnixNano())), //nolint:gosec // G404: Use of weak random number generator is acceptable for relay simulation
	}
}

// Start starts the relay simulator.
func (r *RelaySimulator) Start() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.running = true
	r.healthy = true
	r.logger.Info("Relay simulator started")
}

// Stop stops the relay simulator.
func (r *RelaySimulator) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.running = false
	r.logger.Info("Relay simulator stopped")
}

// SetHealthy sets the relay health status.
func (r *RelaySimulator) SetHealthy(healthy bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.healthy = healthy
}

// IsHealthy returns the current health status.
func (r *RelaySimulator) IsHealthy() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.healthy
}

// SetResponseDelay sets the delay before responding to requests.
func (r *RelaySimulator) SetResponseDelay(delay time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.responseDelay = delay
}

// SetFailureRate sets the probability of operation failures.
func (r *RelaySimulator) SetFailureRate(rate float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failureRate = rate
}

// SimulateRequest simulates a relay request with configured delay and failure rate.
// Returns nil if successful, or an error if the request should fail.
func (r *RelaySimulator) SimulateRequest() error {
	r.mu.Lock()
	delay := r.responseDelay
	shouldFail := r.rand.Float64() < r.failureRate
	r.mu.Unlock()

	// Apply response delay
	if delay > 0 {
		time.Sleep(delay)
	}

	// Check if request should fail based on failure rate
	if shouldFail {
		return errors.New("simulated relay failure")
	}

	return nil
}

// GetResponseDelay returns the current response delay setting.
func (r *RelaySimulator) GetResponseDelay() time.Duration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.responseDelay
}

// GetFailureRate returns the current failure rate setting.
func (r *RelaySimulator) GetFailureRate() float64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.failureRate
}

// AddConnection simulates a new connection.
func (r *RelaySimulator) AddConnection() error {
	connections := atomic.AddInt32(&r.activeConnections, 1)

	r.mu.Lock()
	if connections > r.overloadThreshold {
		r.overloaded = true
	}
	r.mu.Unlock()

	return nil
}

// RemoveConnection simulates removing a connection.
func (r *RelaySimulator) RemoveConnection() {
	connections := atomic.AddInt32(&r.activeConnections, -1)

	r.mu.Lock()
	if connections < r.overloadThreshold/2 {
		r.overloaded = false
	}
	r.mu.Unlock()
}

// IsOverloaded returns true if the relay is overloaded.
func (r *RelaySimulator) IsOverloaded() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.overloaded
}

// ActiveConnections returns the current connection count.
func (r *RelaySimulator) ActiveConnections() int32 {
	return atomic.LoadInt32(&r.activeConnections)
}
