package metrics

import (
	"sync"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	m := New()
	if m == nil {
		t.Fatal("New() returned nil")
	}

	// Check all metrics are initialized
	if m.CircuitBuilds == nil {
		t.Error("CircuitBuilds not initialized")
	}
	if m.ActiveCircuits == nil {
		t.Error("ActiveCircuits not initialized")
	}
	if m.CircuitBuildTime == nil {
		t.Error("CircuitBuildTime not initialized")
	}
}

func TestCounter(t *testing.T) {
	c := NewCounter()

	if c.Value() != 0 {
		t.Errorf("initial value = %d, want 0", c.Value())
	}

	c.Inc()
	if c.Value() != 1 {
		t.Errorf("after Inc() = %d, want 1", c.Value())
	}

	c.Add(5)
	if c.Value() != 6 {
		t.Errorf("after Add(5) = %d, want 6", c.Value())
	}
}

func TestCounterConcurrency(t *testing.T) {
	c := NewCounter()
	const goroutines = 100
	const increments = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < increments; j++ {
				c.Inc()
			}
		}()
	}

	wg.Wait()

	expected := int64(goroutines * increments)
	if c.Value() != expected {
		t.Errorf("concurrent increments = %d, want %d", c.Value(), expected)
	}
}

func TestGauge(t *testing.T) {
	g := NewGauge()

	if g.Value() != 0 {
		t.Errorf("initial value = %d, want 0", g.Value())
	}

	g.Set(42)
	if g.Value() != 42 {
		t.Errorf("after Set(42) = %d, want 42", g.Value())
	}

	g.Inc()
	if g.Value() != 43 {
		t.Errorf("after Inc() = %d, want 43", g.Value())
	}

	g.Dec()
	if g.Value() != 42 {
		t.Errorf("after Dec() = %d, want 42", g.Value())
	}

	g.Add(10)
	if g.Value() != 52 {
		t.Errorf("after Add(10) = %d, want 52", g.Value())
	}
}

func TestGaugeConcurrency(t *testing.T) {
	g := NewGauge()
	const goroutines = 50

	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	// Half increment, half decrement
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			g.Inc()
		}()
		go func() {
			defer wg.Done()
			g.Dec()
		}()
	}

	wg.Wait()

	// Should net to 0
	if g.Value() != 0 {
		t.Errorf("concurrent inc/dec = %d, want 0", g.Value())
	}
}

func TestHistogram(t *testing.T) {
	h := NewHistogram()

	if h.Count() != 0 {
		t.Errorf("initial count = %d, want 0", h.Count())
	}

	// Add observations
	observations := []time.Duration{
		1 * time.Second,
		2 * time.Second,
		3 * time.Second,
		4 * time.Second,
		5 * time.Second,
	}

	for _, d := range observations {
		h.Observe(d)
	}

	if h.Count() != 5 {
		t.Errorf("count = %d, want 5", h.Count())
	}

	// Mean should be 3 seconds
	mean := h.Mean()
	expected := 3 * time.Second
	if mean != expected {
		t.Errorf("mean = %v, want %v", mean, expected)
	}

	// P95 should be close to 5 seconds (95th percentile of 5 items)
	// For 5 items, index = floor(4 * 0.95) = 3, which is the 4th item (4 seconds)
	p95 := h.Percentile(0.95)
	if p95 != 4*time.Second {
		t.Errorf("p95 = %v, want %v", p95, 4*time.Second)
	}

	// P50 (median) should be 3 seconds
	p50 := h.Percentile(0.50)
	if p50 != 3*time.Second {
		t.Errorf("p50 = %v, want %v", p50, 3*time.Second)
	}
}

func TestHistogramBoundedSize(t *testing.T) {
	h := NewHistogram()

	// Add more than 1000 observations
	for i := 0; i < 1500; i++ {
		h.Observe(time.Duration(i) * time.Millisecond)
	}

	// Should only keep last 1000
	if h.Count() != 1000 {
		t.Errorf("count = %d, want 1000", h.Count())
	}
}

func TestHistogramEmptyStats(t *testing.T) {
	h := NewHistogram()

	if h.Mean() != 0 {
		t.Errorf("mean of empty histogram = %v, want 0", h.Mean())
	}

	if h.Percentile(0.95) != 0 {
		t.Errorf("p95 of empty histogram = %v, want 0", h.Percentile(0.95))
	}
}

func TestRecordCircuitBuild(t *testing.T) {
	m := New()

	// Record successful build
	m.RecordCircuitBuild(true, 2*time.Second)

	if m.CircuitBuilds.Value() != 1 {
		t.Errorf("circuit builds = %d, want 1", m.CircuitBuilds.Value())
	}
	if m.CircuitBuildSuccess.Value() != 1 {
		t.Errorf("circuit build success = %d, want 1", m.CircuitBuildSuccess.Value())
	}
	if m.CircuitBuildFailure.Value() != 0 {
		t.Errorf("circuit build failure = %d, want 0", m.CircuitBuildFailure.Value())
	}

	// Record failed build
	m.RecordCircuitBuild(false, 1*time.Second)

	if m.CircuitBuilds.Value() != 2 {
		t.Errorf("circuit builds = %d, want 2", m.CircuitBuilds.Value())
	}
	if m.CircuitBuildSuccess.Value() != 1 {
		t.Errorf("circuit build success = %d, want 1", m.CircuitBuildSuccess.Value())
	}
	if m.CircuitBuildFailure.Value() != 1 {
		t.Errorf("circuit build failure = %d, want 1", m.CircuitBuildFailure.Value())
	}
}

func TestRecordConnection(t *testing.T) {
	m := New()

	// Record successful connection with retries
	m.RecordConnection(true, 2)

	if m.ConnectionAttempts.Value() != 1 {
		t.Errorf("connection attempts = %d, want 1", m.ConnectionAttempts.Value())
	}
	if m.ConnectionSuccess.Value() != 1 {
		t.Errorf("connection success = %d, want 1", m.ConnectionSuccess.Value())
	}
	if m.ConnectionRetries.Value() != 2 {
		t.Errorf("connection retries = %d, want 2", m.ConnectionRetries.Value())
	}

	// Record failed connection
	m.RecordConnection(false, 3)

	if m.ConnectionAttempts.Value() != 2 {
		t.Errorf("connection attempts = %d, want 2", m.ConnectionAttempts.Value())
	}
	if m.ConnectionFailures.Value() != 1 {
		t.Errorf("connection failures = %d, want 1", m.ConnectionFailures.Value())
	}
	if m.ConnectionRetries.Value() != 5 {
		t.Errorf("connection retries = %d, want 5", m.ConnectionRetries.Value())
	}
}

func TestRecordTLSHandshake(t *testing.T) {
	m := New()

	m.RecordTLSHandshake(100 * time.Millisecond)
	m.RecordTLSHandshake(200 * time.Millisecond)

	if m.TLSHandshakeTime.Count() != 2 {
		t.Errorf("TLS handshake count = %d, want 2", m.TLSHandshakeTime.Count())
	}

	mean := m.TLSHandshakeTime.Mean()
	expected := 150 * time.Millisecond
	if mean != expected {
		t.Errorf("TLS handshake mean = %v, want %v", mean, expected)
	}
}

func TestUpdateUptime(t *testing.T) {
	m := New()

	// Wait a bit
	time.Sleep(1100 * time.Millisecond)

	m.UpdateUptime()

	uptime := m.Uptime.Value()
	if uptime < 1 {
		t.Errorf("uptime = %d seconds, want >= 1", uptime)
	}
}

func TestSnapshot(t *testing.T) {
	m := New()

	// Record some metrics
	m.RecordCircuitBuild(true, 2*time.Second)
	m.RecordCircuitBuild(false, 1*time.Second)
	m.RecordConnection(true, 1)
	m.ActiveCircuits.Set(3)
	m.GuardsActive.Set(2)
	m.SocksConnections.Inc()

	// Get snapshot
	snap := m.Snapshot()

	if snap.CircuitBuilds != 2 {
		t.Errorf("snapshot circuit builds = %d, want 2", snap.CircuitBuilds)
	}
	if snap.CircuitBuildSuccess != 1 {
		t.Errorf("snapshot circuit build success = %d, want 1", snap.CircuitBuildSuccess)
	}
	if snap.CircuitBuildFailure != 1 {
		t.Errorf("snapshot circuit build failure = %d, want 1", snap.CircuitBuildFailure)
	}
	if snap.ActiveCircuits != 3 {
		t.Errorf("snapshot active circuits = %d, want 3", snap.ActiveCircuits)
	}
	if snap.GuardsActive != 2 {
		t.Errorf("snapshot guards active = %d, want 2", snap.GuardsActive)
	}
	if snap.SocksConnections != 1 {
		t.Errorf("snapshot socks connections = %d, want 1", snap.SocksConnections)
	}
	// Uptime might be 0 if snapshot is taken immediately
	// Just check it's non-negative
	if snap.UptimeSeconds < 0 {
		t.Errorf("snapshot uptime = %d, want >= 0", snap.UptimeSeconds)
	}
}

func TestSnapshotIndependence(t *testing.T) {
	m := New()

	m.CircuitBuilds.Inc()
	snap1 := m.Snapshot()

	m.CircuitBuilds.Inc()
	snap2 := m.Snapshot()

	if snap1.CircuitBuilds != 1 {
		t.Errorf("snap1 circuit builds = %d, want 1", snap1.CircuitBuilds)
	}
	if snap2.CircuitBuilds != 2 {
		t.Errorf("snap2 circuit builds = %d, want 2", snap2.CircuitBuilds)
	}
}
