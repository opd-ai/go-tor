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

	// Replay protection metrics (SECURITY-001)
	ReplayAttemptsDetected *Counter // Total replay attempts detected
	ReplayForwardAttempts  *Counter // Replay attempts in forward direction
	ReplayBackwardAttempts *Counter // Replay attempts in backward direction
	OutOfOrderCells        *Counter // Cells received out of order (not replays)

	// Rate limiting metrics (Phase 2.3)
	RateLimitedConnections *Counter   // Connections rejected due to rate limiting
	RateLimitedCircuits    *Counter   // TODO: Reserved for future circuit rate limiting
	RateLimitWaitTime      *Histogram // TODO: Reserved for wait-based rate limiting
	BackpressurePauses     *Counter   // TODO: Reserved for future backpressure implementation
	BackpressureResumes    *Counter   // TODO: Reserved for future backpressure implementation

	// Connection pool metrics (Phase 3.3)
	PoolConnectionsCreated *Counter // New connections created (not reused from pool)
	PoolConnectionsReused  *Counter // Connections successfully reused from pool
	PoolConnectionsClosed  *Counter // Connections closed/removed from pool
	PoolSize               *Gauge   // Current number of connections in pool
	PoolHealthCheckFailed  *Counter // Health checks that detected unhealthy connections

	// Memory metrics (AUDIT LOW-007)
	MemoryHeapAlloc      *Gauge   // Bytes of allocated heap objects
	MemoryHeapSys        *Gauge   // Bytes obtained from OS for heap
	MemoryHeapInuse      *Gauge   // Bytes in in-use heap spans
	MemoryNumGoroutines  *Gauge   // Number of goroutines currently running
	MemoryPressureEvents *Counter // Number of memory pressure events detected

	// Crash recovery checkpoint metrics (AUDIT LOW-008)
	CheckpointsSaved     *Counter // Number of successful checkpoint saves
	CheckpointsFailed    *Counter // Number of failed checkpoint saves
	CheckpointsLoaded    *Counter // Number of successful checkpoint loads
	CheckpointRecoveries *Counter // Number of recovery operations from backup

	// Path diversity metrics (Phase 3.4)
	PathDiversityAnalyzed    *Counter   // Total paths analyzed for diversity
	PathDiversityScore       *Histogram // Distribution of path diversity scores (0.0-1.0)
	PathDiversityLow         *Counter   // Paths with low diversity (potential security concern)
	PathDiversityMedium      *Counter   // Paths with medium diversity
	PathDiversityHigh        *Counter   // Paths with high diversity
	PathDiversityExcellent   *Counter   // Paths with excellent diversity
	PathDiversityRejected    *Counter   // Paths rejected due to insufficient diversity
	UniqueASNsObserved       *Gauge     // Number of unique ASNs observed across all relays
	UniqueCountriesObserved  *Gauge     // Number of unique countries observed across all relays
	PathDiversityAvgScore    *Gauge     // Running average diversity score (scaled 0-1000)

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

		// Replay protection metrics (SECURITY-001)
		ReplayAttemptsDetected: NewCounter(),
		ReplayForwardAttempts:  NewCounter(),
		ReplayBackwardAttempts: NewCounter(),
		OutOfOrderCells:        NewCounter(),

		// Rate limiting metrics (Phase 2.3)
		RateLimitedConnections: NewCounter(),
		RateLimitedCircuits:    NewCounter(),
		RateLimitWaitTime:      NewHistogram(),
		BackpressurePauses:     NewCounter(),
		BackpressureResumes:    NewCounter(),

		// Connection pool metrics (Phase 3.3)
		PoolConnectionsCreated: NewCounter(),
		PoolConnectionsReused:  NewCounter(),
		PoolConnectionsClosed:  NewCounter(),
		PoolSize:               NewGauge(),
		PoolHealthCheckFailed:  NewCounter(),

		// Memory metrics (AUDIT LOW-007)
		MemoryHeapAlloc:      NewGauge(),
		MemoryHeapSys:        NewGauge(),
		MemoryHeapInuse:      NewGauge(),
		MemoryNumGoroutines:  NewGauge(),
		MemoryPressureEvents: NewCounter(),

		// Crash recovery checkpoint metrics (AUDIT LOW-008)
		CheckpointsSaved:     NewCounter(),
		CheckpointsFailed:    NewCounter(),
		CheckpointsLoaded:    NewCounter(),
		CheckpointRecoveries: NewCounter(),

		// Path diversity metrics (Phase 3.4)
		PathDiversityAnalyzed:   NewCounter(),
		PathDiversityScore:      NewHistogram(),
		PathDiversityLow:        NewCounter(),
		PathDiversityMedium:     NewCounter(),
		PathDiversityHigh:       NewCounter(),
		PathDiversityExcellent:  NewCounter(),
		PathDiversityRejected:   NewCounter(),
		UniqueASNsObserved:      NewGauge(),
		UniqueCountriesObserved: NewGauge(),
		PathDiversityAvgScore:   NewGauge(),

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

// RecordReplayAttempt records a detected replay attempt
// isForward: true for client→exit direction, false for exit→client
func (m *Metrics) RecordReplayAttempt(isForward bool) {
	m.ReplayAttemptsDetected.Inc()
	if isForward {
		m.ReplayForwardAttempts.Inc()
	} else {
		m.ReplayBackwardAttempts.Inc()
	}
}

// RecordOutOfOrderCell records a cell received out of order
func (m *Metrics) RecordOutOfOrderCell() {
	m.OutOfOrderCells.Inc()
}

// RecordRateLimitedConnection records a connection rejected due to rate limiting
func (m *Metrics) RecordRateLimitedConnection() {
	m.RateLimitedConnections.Inc()
}

// RecordRateLimitedCircuit records a circuit creation rejected due to rate limiting
func (m *Metrics) RecordRateLimitedCircuit() {
	m.RateLimitedCircuits.Inc()
}

// RecordRateLimitWait records time spent waiting for rate limiter
func (m *Metrics) RecordRateLimitWait(duration time.Duration) {
	m.RateLimitWaitTime.Observe(duration)
}

// RecordBackpressure records a backpressure event
// pause: true when backpressure is applied, false when released
func (m *Metrics) RecordBackpressure(pause bool) {
	if pause {
		m.BackpressurePauses.Inc()
	} else {
		m.BackpressureResumes.Inc()
	}
}

// RecordPoolConnectionCreated records when a new connection is created in the pool
func (m *Metrics) RecordPoolConnectionCreated() {
	m.PoolConnectionsCreated.Inc()
}

// RecordPoolConnectionReused records when a connection is successfully reused from the pool
func (m *Metrics) RecordPoolConnectionReused() {
	m.PoolConnectionsReused.Inc()
}

// RecordPoolConnectionClosed records when a connection is closed/removed from the pool
func (m *Metrics) RecordPoolConnectionClosed() {
	m.PoolConnectionsClosed.Inc()
}

// SetPoolSize sets the current number of connections in the pool
func (m *Metrics) SetPoolSize(size int64) {
	m.PoolSize.Set(size)
}

// RecordPoolHealthCheckFailed records when a health check detects an unhealthy connection
func (m *Metrics) RecordPoolHealthCheckFailed() {
	m.PoolHealthCheckFailed.Inc()
}

// UpdateMemoryMetrics updates all memory-related metrics
func (m *Metrics) UpdateMemoryMetrics(heapAlloc, heapSys, heapInuse uint64, numGoroutines int) {
	m.MemoryHeapAlloc.Set(int64(heapAlloc))
	m.MemoryHeapSys.Set(int64(heapSys))
	m.MemoryHeapInuse.Set(int64(heapInuse))
	m.MemoryNumGoroutines.Set(int64(numGoroutines))
}

// RecordMemoryPressureEvent records when a memory pressure event is detected
func (m *Metrics) RecordMemoryPressureEvent() {
	m.MemoryPressureEvents.Inc()
}

// RecordCheckpointSaved records a successful checkpoint save
func (m *Metrics) RecordCheckpointSaved() {
	m.CheckpointsSaved.Inc()
}

// RecordCheckpointFailed records a failed checkpoint save
func (m *Metrics) RecordCheckpointFailed() {
	m.CheckpointsFailed.Inc()
}

// RecordCheckpointLoaded records a successful checkpoint load
func (m *Metrics) RecordCheckpointLoaded() {
	m.CheckpointsLoaded.Inc()
}

// RecordCheckpointRecovery records when state was recovered from a backup checkpoint
func (m *Metrics) RecordCheckpointRecovery() {
	m.CheckpointRecoveries.Inc()
}

// DiversityLevel represents path diversity categories for metrics
type DiversityLevel int

const (
	// DiversityLevelUnknown indicates unknown diversity
	DiversityLevelUnknown DiversityLevel = iota
	// DiversityLevelLow indicates low path diversity
	DiversityLevelLow
	// DiversityLevelMedium indicates medium path diversity
	DiversityLevelMedium
	// DiversityLevelHigh indicates high path diversity
	DiversityLevelHigh
	// DiversityLevelExcellent indicates excellent path diversity
	DiversityLevelExcellent
)

// RecordPathDiversity records a path diversity analysis result
// score: diversity score from 0.0 to 1.0
// level: categorized diversity level
func (m *Metrics) RecordPathDiversity(score float64, level DiversityLevel) {
	m.PathDiversityAnalyzed.Inc()
	// Record score as nanoseconds for histogram compatibility (0.0-1.0 -> 0-1000ms)
	m.PathDiversityScore.Observe(time.Duration(score * 1000 * float64(time.Millisecond)))

	// Record by level
	switch level {
	case DiversityLevelLow:
		m.PathDiversityLow.Inc()
	case DiversityLevelMedium:
		m.PathDiversityMedium.Inc()
	case DiversityLevelHigh:
		m.PathDiversityHigh.Inc()
	case DiversityLevelExcellent:
		m.PathDiversityExcellent.Inc()
	}

	// Update average score (scaled 0-1000 for integer representation)
	m.PathDiversityAvgScore.Set(int64(score * 1000))
}

// RecordPathDiversityRejected records when a path was rejected due to low diversity
func (m *Metrics) RecordPathDiversityRejected() {
	m.PathDiversityRejected.Inc()
}

// UpdatePathDiversityObservations updates the count of unique ASNs and countries observed
func (m *Metrics) UpdatePathDiversityObservations(uniqueASNs, uniqueCountries int) {
	m.UniqueASNsObserved.Set(int64(uniqueASNs))
	m.UniqueCountriesObserved.Set(int64(uniqueCountries))
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

		// Replay protection metrics (SECURITY-001)
		ReplayAttemptsDetected: m.ReplayAttemptsDetected.Value(),
		ReplayForwardAttempts:  m.ReplayForwardAttempts.Value(),
		ReplayBackwardAttempts: m.ReplayBackwardAttempts.Value(),
		OutOfOrderCells:        m.OutOfOrderCells.Value(),

		// Rate limiting metrics (Phase 2.3)
		RateLimitedConnections: m.RateLimitedConnections.Value(),
		RateLimitedCircuits:    m.RateLimitedCircuits.Value(),
		RateLimitWaitTimeAvg:   m.RateLimitWaitTime.Mean(),
		BackpressurePauses:     m.BackpressurePauses.Value(),
		BackpressureResumes:    m.BackpressureResumes.Value(),

		// Connection pool metrics (Phase 3.3)
		PoolConnectionsCreated: m.PoolConnectionsCreated.Value(),
		PoolConnectionsReused:  m.PoolConnectionsReused.Value(),
		PoolConnectionsClosed:  m.PoolConnectionsClosed.Value(),
		PoolSize:               m.PoolSize.Value(),
		PoolHealthCheckFailed:  m.PoolHealthCheckFailed.Value(),

		// Memory metrics (AUDIT LOW-007)
		MemoryHeapAlloc:      m.MemoryHeapAlloc.Value(),
		MemoryHeapSys:        m.MemoryHeapSys.Value(),
		MemoryHeapInuse:      m.MemoryHeapInuse.Value(),
		MemoryNumGoroutines:  m.MemoryNumGoroutines.Value(),
		MemoryPressureEvents: m.MemoryPressureEvents.Value(),

		// Crash recovery checkpoint metrics (AUDIT LOW-008)
		CheckpointsSaved:     m.CheckpointsSaved.Value(),
		CheckpointsFailed:    m.CheckpointsFailed.Value(),
		CheckpointsLoaded:    m.CheckpointsLoaded.Value(),
		CheckpointRecoveries: m.CheckpointRecoveries.Value(),

		// Path diversity metrics (Phase 3.4)
		PathDiversityAnalyzed:   m.PathDiversityAnalyzed.Value(),
		PathDiversityScoreAvg:   m.PathDiversityScore.Mean(),
		PathDiversityScoreP95:   m.PathDiversityScore.Percentile(0.95),
		PathDiversityLow:        m.PathDiversityLow.Value(),
		PathDiversityMedium:     m.PathDiversityMedium.Value(),
		PathDiversityHigh:       m.PathDiversityHigh.Value(),
		PathDiversityExcellent:  m.PathDiversityExcellent.Value(),
		PathDiversityRejected:   m.PathDiversityRejected.Value(),
		UniqueASNsObserved:      m.UniqueASNsObserved.Value(),
		UniqueCountriesObserved: m.UniqueCountriesObserved.Value(),
		PathDiversityAvgScore:   m.PathDiversityAvgScore.Value(),

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

	// Replay protection metrics (SECURITY-001)
	ReplayAttemptsDetected int64 // Total replay attempts detected
	ReplayForwardAttempts  int64 // Replay attempts in forward direction
	ReplayBackwardAttempts int64 // Replay attempts in backward direction
	OutOfOrderCells        int64 // Cells received out of order

	// Rate limiting metrics (Phase 2.3)
	RateLimitedConnections int64         // Connections rejected due to rate limiting
	RateLimitedCircuits    int64         // Circuit creations rejected due to rate limiting
	RateLimitWaitTimeAvg   time.Duration // Average time spent waiting for rate limiter
	BackpressurePauses     int64         // Number of times backpressure was applied
	BackpressureResumes    int64         // Number of times backpressure was released

	// Connection pool metrics (Phase 3.3)
	PoolConnectionsCreated int64 // New connections created (not reused from pool)
	PoolConnectionsReused  int64 // Connections successfully reused from pool
	PoolConnectionsClosed  int64 // Connections closed/removed from pool
	PoolSize               int64 // Current number of connections in pool
	PoolHealthCheckFailed  int64 // Health checks that detected unhealthy connections

	// Memory metrics (AUDIT LOW-007)
	MemoryHeapAlloc      int64 // Bytes of allocated heap objects
	MemoryHeapSys        int64 // Bytes obtained from OS for heap
	MemoryHeapInuse      int64 // Bytes in in-use heap spans
	MemoryNumGoroutines  int64 // Number of goroutines currently running
	MemoryPressureEvents int64 // Number of memory pressure events detected

	// Crash recovery checkpoint metrics (AUDIT LOW-008)
	CheckpointsSaved     int64 // Number of successful checkpoint saves
	CheckpointsFailed    int64 // Number of failed checkpoint saves
	CheckpointsLoaded    int64 // Number of successful checkpoint loads
	CheckpointRecoveries int64 // Number of recovery operations from backup

	// Path diversity metrics (Phase 3.4)
	PathDiversityAnalyzed   int64         // Total paths analyzed for diversity
	PathDiversityScoreAvg   time.Duration // Average diversity score (using Duration for histogram compatibility)
	PathDiversityScoreP95   time.Duration // P95 diversity score
	PathDiversityLow        int64         // Paths with low diversity (potential security concern)
	PathDiversityMedium     int64         // Paths with medium diversity
	PathDiversityHigh       int64         // Paths with high diversity
	PathDiversityExcellent  int64         // Paths with excellent diversity
	PathDiversityRejected   int64         // Paths rejected due to insufficient diversity
	UniqueASNsObserved      int64         // Number of unique ASNs observed across all relays
	UniqueCountriesObserved int64         // Number of unique countries observed across all relays
	PathDiversityAvgScore   int64         // Running average diversity score (scaled 0-1000)

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
