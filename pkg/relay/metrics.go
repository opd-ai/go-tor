package relay

import (
	"sync/atomic"
	"time"
)

// RelayMetrics tracks relay-specific operational metrics
type RelayMetrics struct {
	// Circuit metrics
	CircuitsCreated     *Counter   // Total circuits created (server-side)
	CircuitsDestroyed   *Counter   // Total circuits destroyed
	CircuitsExtended    *Counter   // Total circuit extensions processed
	ActiveCircuits      *Gauge     // Currently active circuits
	CircuitCreationTime *Histogram // Time to process CREATE2 cells

	// Connection metrics
	ConnectionsAccepted *Counter   // Total connections accepted
	ConnectionsRejected *Counter   // Total connections rejected
	ConnectionsClosed   *Counter   // Total connections closed
	ActiveConnections   *Gauge     // Currently active connections
	ConnectionDuration  *Histogram // Connection lifetime distribution

	// Cell forwarding metrics
	CellsReceived        *Counter   // Total cells received from clients
	CellsForwarded       *Counter   // Total cells forwarded to next hop
	CellsDropped         *Counter   // Total cells dropped (errors, rate limits)
	RelayEarlyViolations *Counter   // RELAY_EARLY limit violations detected
	CellForwardingTime   *Histogram // Time to forward a cell

	// Bandwidth metrics
	BytesReceived    *Counter // Total bytes received from clients
	BytesTransmitted *Counter // Total bytes transmitted to next hop
	BandwidthUsage   *Gauge   // Current bandwidth usage (bytes/sec)

	// Rate limiting metrics
	RateLimitedCircuits    *Counter   // Circuits rejected due to rate limiting
	RateLimitedConnections *Counter   // Connections rejected due to rate limiting
	RateLimitedCells       *Counter   // Cells delayed due to rate limiting
	RateLimitWaitTime      *Histogram // Wait time for rate-limited operations

	// DoS protection metrics
	DoSConnectionsRejected *Counter // Connections rejected by DoS protection
	DoSCircuitsRejected    *Counter // Circuits rejected by DoS protection
	DoSEventsDetected      *Counter // Potential DoS events detected

	// Exit policy metrics
	ExitAttemptsBlocked  *Counter // Exit attempts blocked by policy
	ExitPolicyViolations *Counter // Exit policy violations detected

	// Descriptor publishing metrics
	DescriptorsPublished     *Counter   // Successful descriptor publications
	DescriptorPublishFailed  *Counter   // Failed descriptor publications
	DescriptorPublishTime    *Histogram // Time to publish descriptor
	DescriptorPublishRetries *Counter   // Descriptor publish retry attempts

	// Error metrics
	HandshakeErrors    *Counter // TLS/link protocol handshake errors
	CellDecodingErrors *Counter // Cell decoding errors
	ProtocolErrors     *Counter // Protocol violations detected
	ExtensionErrors    *Counter // Circuit extension errors

	// System metrics
	Uptime    *Gauge
	startTime time.Time
}

// NewRelayMetrics creates a new relay metrics instance
func NewRelayMetrics() *RelayMetrics {
	return &RelayMetrics{
		// Circuit metrics
		CircuitsCreated:     NewCounter(),
		CircuitsDestroyed:   NewCounter(),
		CircuitsExtended:    NewCounter(),
		ActiveCircuits:      NewGauge(),
		CircuitCreationTime: NewHistogram(),

		// Connection metrics
		ConnectionsAccepted: NewCounter(),
		ConnectionsRejected: NewCounter(),
		ConnectionsClosed:   NewCounter(),
		ActiveConnections:   NewGauge(),
		ConnectionDuration:  NewHistogram(),

		// Cell forwarding metrics
		CellsReceived:        NewCounter(),
		CellsForwarded:       NewCounter(),
		CellsDropped:         NewCounter(),
		RelayEarlyViolations: NewCounter(),
		CellForwardingTime:   NewHistogram(),

		// Bandwidth metrics
		BytesReceived:    NewCounter(),
		BytesTransmitted: NewCounter(),
		BandwidthUsage:   NewGauge(),

		// Rate limiting metrics
		RateLimitedCircuits:    NewCounter(),
		RateLimitedConnections: NewCounter(),
		RateLimitedCells:       NewCounter(),
		RateLimitWaitTime:      NewHistogram(),

		// DoS protection metrics
		DoSConnectionsRejected: NewCounter(),
		DoSCircuitsRejected:    NewCounter(),
		DoSEventsDetected:      NewCounter(),

		// Exit policy metrics
		ExitAttemptsBlocked:  NewCounter(),
		ExitPolicyViolations: NewCounter(),

		// Descriptor publishing metrics
		DescriptorsPublished:     NewCounter(),
		DescriptorPublishFailed:  NewCounter(),
		DescriptorPublishTime:    NewHistogram(),
		DescriptorPublishRetries: NewCounter(),

		// Error metrics
		HandshakeErrors:    NewCounter(),
		CellDecodingErrors: NewCounter(),
		ProtocolErrors:     NewCounter(),
		ExtensionErrors:    NewCounter(),

		// System metrics
		Uptime:    NewGauge(),
		startTime: time.Now(),
	}
}

// UpdateUptime updates the uptime metric
func (m *RelayMetrics) UpdateUptime() {
	uptime := time.Since(m.startTime).Seconds()
	m.Uptime.Set(int(uptime))
}

// Snapshot returns a snapshot of all relay metrics
func (m *RelayMetrics) Snapshot() RelayMetricsSnapshot {
	return RelayMetricsSnapshot{
		// Circuit metrics
		CircuitsCreated:   m.CircuitsCreated.Value(),
		CircuitsDestroyed: m.CircuitsDestroyed.Value(),
		CircuitsExtended:  m.CircuitsExtended.Value(),
		ActiveCircuits:    m.ActiveCircuits.Value(),

		// Connection metrics
		ConnectionsAccepted: m.ConnectionsAccepted.Value(),
		ConnectionsRejected: m.ConnectionsRejected.Value(),
		ConnectionsClosed:   m.ConnectionsClosed.Value(),
		ActiveConnections:   m.ActiveConnections.Value(),

		// Cell forwarding metrics
		CellsReceived:        m.CellsReceived.Value(),
		CellsForwarded:       m.CellsForwarded.Value(),
		CellsDropped:         m.CellsDropped.Value(),
		RelayEarlyViolations: m.RelayEarlyViolations.Value(),

		// Bandwidth metrics
		BytesReceived:    m.BytesReceived.Value(),
		BytesTransmitted: m.BytesTransmitted.Value(),
		BandwidthUsage:   m.BandwidthUsage.Value(),

		// Rate limiting metrics
		RateLimitedCircuits:    m.RateLimitedCircuits.Value(),
		RateLimitedConnections: m.RateLimitedConnections.Value(),
		RateLimitedCells:       m.RateLimitedCells.Value(),

		// DoS protection metrics
		DoSConnectionsRejected: m.DoSConnectionsRejected.Value(),
		DoSCircuitsRejected:    m.DoSCircuitsRejected.Value(),
		DoSEventsDetected:      m.DoSEventsDetected.Value(),

		// Exit policy metrics
		ExitAttemptsBlocked:  m.ExitAttemptsBlocked.Value(),
		ExitPolicyViolations: m.ExitPolicyViolations.Value(),

		// Descriptor publishing metrics
		DescriptorsPublished:    m.DescriptorsPublished.Value(),
		DescriptorPublishFailed: m.DescriptorPublishFailed.Value(),

		// Error metrics
		HandshakeErrors:    m.HandshakeErrors.Value(),
		CellDecodingErrors: m.CellDecodingErrors.Value(),
		ProtocolErrors:     m.ProtocolErrors.Value(),
		ExtensionErrors:    m.ExtensionErrors.Value(),

		// System metrics
		Uptime: m.Uptime.Value(),
	}
}

// RelayMetricsSnapshot is a point-in-time snapshot of relay metrics
type RelayMetricsSnapshot struct {
	// Circuit metrics
	CircuitsCreated   int64
	CircuitsDestroyed int64
	CircuitsExtended  int64
	ActiveCircuits    int

	// Connection metrics
	ConnectionsAccepted int64
	ConnectionsRejected int64
	ConnectionsClosed   int64
	ActiveConnections   int

	// Cell forwarding metrics
	CellsReceived        int64
	CellsForwarded       int64
	CellsDropped         int64
	RelayEarlyViolations int64

	// Bandwidth metrics
	BytesReceived    int64
	BytesTransmitted int64
	BandwidthUsage   int

	// Rate limiting metrics
	RateLimitedCircuits    int64
	RateLimitedConnections int64
	RateLimitedCells       int64

	// DoS protection metrics
	DoSConnectionsRejected int64
	DoSCircuitsRejected    int64
	DoSEventsDetected      int64

	// Exit policy metrics
	ExitAttemptsBlocked  int64
	ExitPolicyViolations int64

	// Descriptor publishing metrics
	DescriptorsPublished    int64
	DescriptorPublishFailed int64

	// Error metrics
	HandshakeErrors    int64
	CellDecodingErrors int64
	ProtocolErrors     int64
	ExtensionErrors    int64

	// System metrics
	Uptime int
}

// Counter is a thread-safe counter
type Counter struct {
	value int64
}

// NewCounter creates a new counter
func NewCounter() *Counter {
	return &Counter{}
}

// Inc increments the counter
func (c *Counter) Inc() {
	atomic.AddInt64(&c.value, 1)
}

// Add adds a value to the counter
func (c *Counter) Add(delta int64) {
	atomic.AddInt64(&c.value, delta)
}

// Value returns the current counter value
func (c *Counter) Value() int64 {
	return atomic.LoadInt64(&c.value)
}

// Gauge is a thread-safe gauge that can go up or down
type Gauge struct {
	value int64
}

// NewGauge creates a new gauge
func NewGauge() *Gauge {
	return &Gauge{}
}

// Set sets the gauge to a specific value
func (g *Gauge) Set(value int) {
	atomic.StoreInt64(&g.value, int64(value))
}

// Inc increments the gauge
func (g *Gauge) Inc() {
	atomic.AddInt64(&g.value, 1)
}

// Dec decrements the gauge
func (g *Gauge) Dec() {
	atomic.AddInt64(&g.value, -1)
}

// Value returns the current gauge value
func (g *Gauge) Value() int {
	return int(atomic.LoadInt64(&g.value))
}

// Histogram tracks distribution of values
type Histogram struct {
	// Simple histogram using buckets
	// For production, consider using a more sophisticated implementation
	count int64
	sum   int64
}

// NewHistogram creates a new histogram
func NewHistogram() *Histogram {
	return &Histogram{}
}

// Observe records a value
func (h *Histogram) Observe(value int64) {
	atomic.AddInt64(&h.count, 1)
	atomic.AddInt64(&h.sum, value)
}

// Count returns the number of observations
func (h *Histogram) Count() int64 {
	return atomic.LoadInt64(&h.count)
}

// Sum returns the sum of all observations
func (h *Histogram) Sum() int64 {
	return atomic.LoadInt64(&h.sum)
}

// Average returns the average of all observations
func (h *Histogram) Average() float64 {
	count := atomic.LoadInt64(&h.count)
	if count == 0 {
		return 0
	}
	sum := atomic.LoadInt64(&h.sum)
	return float64(sum) / float64(count)
}
