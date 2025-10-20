// Package metrics provides comprehensive operational metrics for the Tor client.
// This package tracks circuit, connection, stream, and system-level metrics
// for observability and monitoring.
package metrics

import (
	"sync"
	"sync/atomic"
	"time"
)

// Metrics provides a comprehensive metrics collection for the Tor client
type Metrics struct {
	// Circuit metrics
	CircuitBuilds       *Counter
	CircuitBuildSuccess *Counter
	CircuitBuildFailure *Counter
	CircuitBuildTime    *Histogram
	ActiveCircuits      *Gauge

	// Connection metrics
	ConnectionAttempts *Counter
	ConnectionSuccess  *Counter
	ConnectionFailures *Counter
	ConnectionRetries  *Counter
	TLSHandshakeTime   *Histogram
	ActiveConnections  *Gauge

	// Stream metrics
	StreamsCreated *Counter
	StreamsClosed  *Counter
	StreamFailures *Counter
	ActiveStreams  *Gauge
	StreamData     *Counter // bytes transferred

	// Guard metrics
	GuardsActive    *Gauge
	GuardsConfirmed *Gauge

	// SOCKS metrics
	SocksConnections *Counter
	SocksRequests    *Counter
	SocksErrors      *Counter

	// Circuit isolation metrics
	IsolatedCircuits *Gauge   // Total isolated circuits
	IsolationKeys    *Gauge   // Number of unique isolation keys
	IsolationHits    *Counter // Circuit reused from isolated pool
	IsolationMisses  *Counter // New circuit built for isolation

	// System metrics
	Uptime      *Gauge
	startTime   time.Time
	startTimeMu sync.RWMutex
}

// New creates a new metrics instance
func New() *Metrics {
	now := time.Now()
	return &Metrics{
		// Circuit metrics
		CircuitBuilds:       NewCounter(),
		CircuitBuildSuccess: NewCounter(),
		CircuitBuildFailure: NewCounter(),
		CircuitBuildTime:    NewHistogram(),
		ActiveCircuits:      NewGauge(),

		// Connection metrics
		ConnectionAttempts: NewCounter(),
		ConnectionSuccess:  NewCounter(),
		ConnectionFailures: NewCounter(),
		ConnectionRetries:  NewCounter(),
		TLSHandshakeTime:   NewHistogram(),
		ActiveConnections:  NewGauge(),

		// Stream metrics
		StreamsCreated: NewCounter(),
		StreamsClosed:  NewCounter(),
		StreamFailures: NewCounter(),
		ActiveStreams:  NewGauge(),
		StreamData:     NewCounter(),

		// Guard metrics
		GuardsActive:    NewGauge(),
		GuardsConfirmed: NewGauge(),

		// SOCKS metrics
		SocksConnections: NewCounter(),
		SocksRequests:    NewCounter(),
		SocksErrors:      NewCounter(),

		// Circuit isolation metrics
		IsolatedCircuits: NewGauge(),
		IsolationKeys:    NewGauge(),
		IsolationHits:    NewCounter(),
		IsolationMisses:  NewCounter(),

		// System metrics
		Uptime:    NewGauge(),
		startTime: now,
	}
}

// RecordCircuitBuild records a circuit build attempt and its duration
func (m *Metrics) RecordCircuitBuild(success bool, duration time.Duration) {
	m.CircuitBuilds.Inc()
	if success {
		m.CircuitBuildSuccess.Inc()
	} else {
		m.CircuitBuildFailure.Inc()
	}
	m.CircuitBuildTime.Observe(duration)
}

// RecordConnection records a connection attempt and its outcome
func (m *Metrics) RecordConnection(success bool, retries int64) {
	m.ConnectionAttempts.Inc()
	if success {
		m.ConnectionSuccess.Inc()
	} else {
		m.ConnectionFailures.Inc()
	}
	m.ConnectionRetries.Add(retries)
}

// RecordTLSHandshake records TLS handshake duration
func (m *Metrics) RecordTLSHandshake(duration time.Duration) {
	m.TLSHandshakeTime.Observe(duration)
}

// UpdateUptime updates the uptime metric
func (m *Metrics) UpdateUptime() {
	m.startTimeMu.RLock()
	defer m.startTimeMu.RUnlock()
	m.Uptime.Set(int64(time.Since(m.startTime).Seconds()))
}

// Snapshot returns a point-in-time snapshot of all metrics
func (m *Metrics) Snapshot() *Snapshot {
	m.UpdateUptime()
	return &Snapshot{
		// Circuit metrics
		CircuitBuilds:       m.CircuitBuilds.Value(),
		CircuitBuildSuccess: m.CircuitBuildSuccess.Value(),
		CircuitBuildFailure: m.CircuitBuildFailure.Value(),
		CircuitBuildTimeAvg: m.CircuitBuildTime.Mean(),
		CircuitBuildTimeP95: m.CircuitBuildTime.Percentile(0.95),
		ActiveCircuits:      m.ActiveCircuits.Value(),

		// Connection metrics
		ConnectionAttempts: m.ConnectionAttempts.Value(),
		ConnectionSuccess:  m.ConnectionSuccess.Value(),
		ConnectionFailures: m.ConnectionFailures.Value(),
		ConnectionRetries:  m.ConnectionRetries.Value(),
		TLSHandshakeAvg:    m.TLSHandshakeTime.Mean(),
		TLSHandshakeP95:    m.TLSHandshakeTime.Percentile(0.95),
		ActiveConnections:  m.ActiveConnections.Value(),

		// Stream metrics
		StreamsCreated: m.StreamsCreated.Value(),
		StreamsClosed:  m.StreamsClosed.Value(),
		StreamFailures: m.StreamFailures.Value(),
		ActiveStreams:  m.ActiveStreams.Value(),
		StreamData:     m.StreamData.Value(),

		// Guard metrics
		GuardsActive:    m.GuardsActive.Value(),
		GuardsConfirmed: m.GuardsConfirmed.Value(),

		// SOCKS metrics
		SocksConnections: m.SocksConnections.Value(),
		SocksRequests:    m.SocksRequests.Value(),
		SocksErrors:      m.SocksErrors.Value(),

		// Circuit isolation metrics
		IsolatedCircuits: m.IsolatedCircuits.Value(),
		IsolationKeys:    m.IsolationKeys.Value(),
		IsolationHits:    m.IsolationHits.Value(),
		IsolationMisses:  m.IsolationMisses.Value(),

		// System metrics
		UptimeSeconds: m.Uptime.Value(),
	}
}

// Snapshot represents a point-in-time snapshot of metrics
type Snapshot struct {
	// Circuit metrics
	CircuitBuilds       int64
	CircuitBuildSuccess int64
	CircuitBuildFailure int64
	CircuitBuildTimeAvg time.Duration
	CircuitBuildTimeP95 time.Duration
	ActiveCircuits      int64

	// Connection metrics
	ConnectionAttempts int64
	ConnectionSuccess  int64
	ConnectionFailures int64
	ConnectionRetries  int64
	TLSHandshakeAvg    time.Duration
	TLSHandshakeP95    time.Duration
	ActiveConnections  int64

	// Stream metrics
	StreamsCreated int64
	StreamsClosed  int64
	StreamFailures int64
	ActiveStreams  int64
	StreamData     int64 // bytes

	// Guard metrics
	GuardsActive    int64
	GuardsConfirmed int64

	// SOCKS metrics
	SocksConnections int64
	SocksRequests    int64
	SocksErrors      int64

	// Circuit isolation metrics
	IsolatedCircuits int64
	IsolationKeys    int64
	IsolationHits    int64
	IsolationMisses  int64

	// System metrics
	UptimeSeconds int64
}

// Counter is a monotonically increasing counter
type Counter struct {
	value int64
}

// NewCounter creates a new counter
func NewCounter() *Counter {
	return &Counter{}
}

// Inc increments the counter by 1
func (c *Counter) Inc() {
	atomic.AddInt64(&c.value, 1)
}

// Add adds n to the counter
func (c *Counter) Add(n int64) {
	atomic.AddInt64(&c.value, n)
}

// Value returns the current counter value
func (c *Counter) Value() int64 {
	return atomic.LoadInt64(&c.value)
}

// Gauge is a value that can go up or down
type Gauge struct {
	value int64
}

// NewGauge creates a new gauge
func NewGauge() *Gauge {
	return &Gauge{}
}

// Set sets the gauge to a specific value
func (g *Gauge) Set(value int64) {
	atomic.StoreInt64(&g.value, value)
}

// Inc increments the gauge by 1
func (g *Gauge) Inc() {
	atomic.AddInt64(&g.value, 1)
}

// Dec decrements the gauge by 1
func (g *Gauge) Dec() {
	atomic.AddInt64(&g.value, -1)
}

// Add adds n to the gauge
func (g *Gauge) Add(n int64) {
	atomic.AddInt64(&g.value, n)
}

// Value returns the current gauge value
func (g *Gauge) Value() int64 {
	return atomic.LoadInt64(&g.value)
}

// Histogram tracks distribution of durations
type Histogram struct {
	observations []time.Duration
	mu           sync.RWMutex
}

// NewHistogram creates a new histogram
func NewHistogram() *Histogram {
	return &Histogram{
		observations: make([]time.Duration, 0, 1000),
	}
}

// Observe adds a new observation to the histogram
func (h *Histogram) Observe(d time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Keep last 1000 observations to prevent unbounded memory growth
	if len(h.observations) >= 1000 {
		h.observations = h.observations[1:]
	}
	h.observations = append(h.observations, d)
}

// Mean returns the mean of all observations
func (h *Histogram) Mean() time.Duration {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.observations) == 0 {
		return 0
	}

	var sum time.Duration
	for _, d := range h.observations {
		sum += d
	}
	return sum / time.Duration(len(h.observations))
}

// Percentile returns the nth percentile (0.0 to 1.0)
func (h *Histogram) Percentile(p float64) time.Duration {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.observations) == 0 {
		return 0
	}

	// Simple percentile calculation - sort observations
	sorted := make([]time.Duration, len(h.observations))
	copy(sorted, h.observations)

	// Bubble sort (fine for our limited observation window)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	index := int(float64(len(sorted)-1) * p)
	return sorted[index]
}

// Count returns the number of observations
func (h *Histogram) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.observations)
}
