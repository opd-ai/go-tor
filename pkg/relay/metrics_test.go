package relay

import (
	"testing"
	"time"
)

func TestNewRelayMetrics(t *testing.T) {
	metrics := NewRelayMetrics()

	if metrics == nil {
		t.Fatal("Expected non-nil metrics")
	}

	// Check all metrics are initialized
	if metrics.CircuitsCreated == nil {
		t.Error("CircuitsCreated not initialized")
	}
	if metrics.ConnectionsAccepted == nil {
		t.Error("ConnectionsAccepted not initialized")
	}
	if metrics.CellsReceived == nil {
		t.Error("CellsReceived not initialized")
	}
	if metrics.BytesReceived == nil {
		t.Error("BytesReceived not initialized")
	}
	if metrics.RateLimitedCircuits == nil {
		t.Error("RateLimitedCircuits not initialized")
	}
	if metrics.DoSConnectionsRejected == nil {
		t.Error("DoSConnectionsRejected not initialized")
	}
	if metrics.ExitAttemptsBlocked == nil {
		t.Error("ExitAttemptsBlocked not initialized")
	}
	if metrics.DescriptorsPublished == nil {
		t.Error("DescriptorsPublished not initialized")
	}
	if metrics.HandshakeErrors == nil {
		t.Error("HandshakeErrors not initialized")
	}
	if metrics.Uptime == nil {
		t.Error("Uptime not initialized")
	}
}

func TestRelayMetrics_CircuitMetrics(t *testing.T) {
	metrics := NewRelayMetrics()

	// Increment circuit metrics
	metrics.CircuitsCreated.Inc()
	metrics.CircuitsCreated.Inc()
	metrics.CircuitsExtended.Inc()
	metrics.ActiveCircuits.Inc()

	if metrics.CircuitsCreated.Value() != 2 {
		t.Errorf("Expected 2 circuits created, got %d", metrics.CircuitsCreated.Value())
	}

	if metrics.CircuitsExtended.Value() != 1 {
		t.Errorf("Expected 1 circuit extended, got %d", metrics.CircuitsExtended.Value())
	}

	if metrics.ActiveCircuits.Value() != 1 {
		t.Errorf("Expected 1 active circuit, got %d", metrics.ActiveCircuits.Value())
	}

	// Test histogram
	metrics.CircuitCreationTime.Observe(100)
	metrics.CircuitCreationTime.Observe(200)

	if metrics.CircuitCreationTime.Count() != 2 {
		t.Errorf("Expected 2 observations, got %d", metrics.CircuitCreationTime.Count())
	}

	if metrics.CircuitCreationTime.Sum() != 300 {
		t.Errorf("Expected sum of 300, got %d", metrics.CircuitCreationTime.Sum())
	}

	avg := metrics.CircuitCreationTime.Average()
	if avg != 150.0 {
		t.Errorf("Expected average of 150.0, got %f", avg)
	}
}

func TestRelayMetrics_ConnectionMetrics(t *testing.T) {
	metrics := NewRelayMetrics()

	metrics.ConnectionsAccepted.Add(10)
	metrics.ConnectionsRejected.Add(2)
	metrics.ConnectionsClosed.Add(5)
	metrics.ActiveConnections.Set(5)

	if metrics.ConnectionsAccepted.Value() != 10 {
		t.Errorf("Expected 10 connections accepted, got %d", metrics.ConnectionsAccepted.Value())
	}

	if metrics.ConnectionsRejected.Value() != 2 {
		t.Errorf("Expected 2 connections rejected, got %d", metrics.ConnectionsRejected.Value())
	}

	if metrics.ActiveConnections.Value() != 5 {
		t.Errorf("Expected 5 active connections, got %d", metrics.ActiveConnections.Value())
	}
}

func TestRelayMetrics_CellForwardingMetrics(t *testing.T) {
	metrics := NewRelayMetrics()

	metrics.CellsReceived.Add(1000)
	metrics.CellsForwarded.Add(980)
	metrics.CellsDropped.Add(20)
	metrics.RelayEarlyViolations.Inc()

	if metrics.CellsReceived.Value() != 1000 {
		t.Errorf("Expected 1000 cells received, got %d", metrics.CellsReceived.Value())
	}

	if metrics.CellsForwarded.Value() != 980 {
		t.Errorf("Expected 980 cells forwarded, got %d", metrics.CellsForwarded.Value())
	}

	if metrics.CellsDropped.Value() != 20 {
		t.Errorf("Expected 20 cells dropped, got %d", metrics.CellsDropped.Value())
	}

	if metrics.RelayEarlyViolations.Value() != 1 {
		t.Errorf("Expected 1 RELAY_EARLY violation, got %d", metrics.RelayEarlyViolations.Value())
	}
}

func TestRelayMetrics_BandwidthMetrics(t *testing.T) {
	metrics := NewRelayMetrics()

	metrics.BytesReceived.Add(1024 * 1024)        // 1 MB
	metrics.BytesTransmitted.Add(2 * 1024 * 1024) // 2 MB
	metrics.BandwidthUsage.Set(1000000)           // 1 MB/s

	if metrics.BytesReceived.Value() != 1024*1024 {
		t.Errorf("Expected 1 MB received, got %d", metrics.BytesReceived.Value())
	}

	if metrics.BytesTransmitted.Value() != 2*1024*1024 {
		t.Errorf("Expected 2 MB transmitted, got %d", metrics.BytesTransmitted.Value())
	}

	if metrics.BandwidthUsage.Value() != 1000000 {
		t.Errorf("Expected 1 MB/s bandwidth, got %d", metrics.BandwidthUsage.Value())
	}
}

func TestRelayMetrics_RateLimitingMetrics(t *testing.T) {
	metrics := NewRelayMetrics()

	metrics.RateLimitedCircuits.Inc()
	metrics.RateLimitedConnections.Inc()
	metrics.RateLimitedCells.Add(50)
	metrics.RateLimitWaitTime.Observe(100)

	if metrics.RateLimitedCircuits.Value() != 1 {
		t.Errorf("Expected 1 rate limited circuit, got %d", metrics.RateLimitedCircuits.Value())
	}

	if metrics.RateLimitedCells.Value() != 50 {
		t.Errorf("Expected 50 rate limited cells, got %d", metrics.RateLimitedCells.Value())
	}
}

func TestRelayMetrics_DoSProtectionMetrics(t *testing.T) {
	metrics := NewRelayMetrics()

	metrics.DoSConnectionsRejected.Add(10)
	metrics.DoSCircuitsRejected.Add(5)
	metrics.DoSEventsDetected.Inc()

	if metrics.DoSConnectionsRejected.Value() != 10 {
		t.Errorf("Expected 10 DoS connections rejected, got %d", metrics.DoSConnectionsRejected.Value())
	}

	if metrics.DoSCircuitsRejected.Value() != 5 {
		t.Errorf("Expected 5 DoS circuits rejected, got %d", metrics.DoSCircuitsRejected.Value())
	}

	if metrics.DoSEventsDetected.Value() != 1 {
		t.Errorf("Expected 1 DoS event detected, got %d", metrics.DoSEventsDetected.Value())
	}
}

func TestRelayMetrics_ExitPolicyMetrics(t *testing.T) {
	metrics := NewRelayMetrics()

	metrics.ExitAttemptsBlocked.Add(100)
	metrics.ExitPolicyViolations.Inc()

	if metrics.ExitAttemptsBlocked.Value() != 100 {
		t.Errorf("Expected 100 exit attempts blocked, got %d", metrics.ExitAttemptsBlocked.Value())
	}

	if metrics.ExitPolicyViolations.Value() != 1 {
		t.Errorf("Expected 1 exit policy violation, got %d", metrics.ExitPolicyViolations.Value())
	}
}

func TestRelayMetrics_DescriptorMetrics(t *testing.T) {
	metrics := NewRelayMetrics()

	metrics.DescriptorsPublished.Add(10)
	metrics.DescriptorPublishFailed.Inc()
	metrics.DescriptorPublishTime.Observe(5000)
	metrics.DescriptorPublishRetries.Add(3)

	if metrics.DescriptorsPublished.Value() != 10 {
		t.Errorf("Expected 10 descriptors published, got %d", metrics.DescriptorsPublished.Value())
	}

	if metrics.DescriptorPublishFailed.Value() != 1 {
		t.Errorf("Expected 1 failed publish, got %d", metrics.DescriptorPublishFailed.Value())
	}

	if metrics.DescriptorPublishRetries.Value() != 3 {
		t.Errorf("Expected 3 retries, got %d", metrics.DescriptorPublishRetries.Value())
	}
}

func TestRelayMetrics_ErrorMetrics(t *testing.T) {
	metrics := NewRelayMetrics()

	metrics.HandshakeErrors.Inc()
	metrics.CellDecodingErrors.Add(5)
	metrics.ProtocolErrors.Inc()
	metrics.ExtensionErrors.Add(2)

	if metrics.HandshakeErrors.Value() != 1 {
		t.Errorf("Expected 1 handshake error, got %d", metrics.HandshakeErrors.Value())
	}

	if metrics.CellDecodingErrors.Value() != 5 {
		t.Errorf("Expected 5 cell decoding errors, got %d", metrics.CellDecodingErrors.Value())
	}

	if metrics.ProtocolErrors.Value() != 1 {
		t.Errorf("Expected 1 protocol error, got %d", metrics.ProtocolErrors.Value())
	}

	if metrics.ExtensionErrors.Value() != 2 {
		t.Errorf("Expected 2 extension errors, got %d", metrics.ExtensionErrors.Value())
	}
}

func TestRelayMetrics_UpdateUptime(t *testing.T) {
	metrics := NewRelayMetrics()

	// Initial uptime should be near 0
	metrics.UpdateUptime()
	uptime := metrics.Uptime.Value()
	if uptime != 0 {
		t.Logf("Initial uptime: %d seconds (expected ~0)", uptime)
	}

	// Wait a bit and update
	time.Sleep(100 * time.Millisecond)
	metrics.UpdateUptime()
	uptime2 := metrics.Uptime.Value()

	if uptime2 < uptime {
		t.Errorf("Uptime should increase, got %d -> %d", uptime, uptime2)
	}
}

func TestRelayMetrics_Snapshot(t *testing.T) {
	metrics := NewRelayMetrics()

	// Set some values
	metrics.CircuitsCreated.Add(10)
	metrics.ConnectionsAccepted.Add(20)
	metrics.CellsReceived.Add(1000)
	metrics.ActiveCircuits.Set(5)
	metrics.ActiveConnections.Set(8)

	// Take snapshot
	snapshot := metrics.Snapshot()

	// Verify snapshot values
	if snapshot.CircuitsCreated != 10 {
		t.Errorf("Expected 10 circuits created in snapshot, got %d", snapshot.CircuitsCreated)
	}

	if snapshot.ConnectionsAccepted != 20 {
		t.Errorf("Expected 20 connections accepted in snapshot, got %d", snapshot.ConnectionsAccepted)
	}

	if snapshot.CellsReceived != 1000 {
		t.Errorf("Expected 1000 cells received in snapshot, got %d", snapshot.CellsReceived)
	}

	if snapshot.ActiveCircuits != 5 {
		t.Errorf("Expected 5 active circuits in snapshot, got %d", snapshot.ActiveCircuits)
	}

	if snapshot.ActiveConnections != 8 {
		t.Errorf("Expected 8 active connections in snapshot, got %d", snapshot.ActiveConnections)
	}

	// Modify metrics after snapshot
	metrics.CircuitsCreated.Inc()

	// Snapshot should be immutable
	if snapshot.CircuitsCreated != 10 {
		t.Errorf("Snapshot should be immutable, got %d", snapshot.CircuitsCreated)
	}

	// New snapshot should reflect changes
	snapshot2 := metrics.Snapshot()
	if snapshot2.CircuitsCreated != 11 {
		t.Errorf("Expected 11 circuits created in new snapshot, got %d", snapshot2.CircuitsCreated)
	}
}

func TestCounter(t *testing.T) {
	counter := NewCounter()

	if counter.Value() != 0 {
		t.Errorf("Expected initial value of 0, got %d", counter.Value())
	}

	counter.Inc()
	if counter.Value() != 1 {
		t.Errorf("Expected value of 1 after Inc(), got %d", counter.Value())
	}

	counter.Add(10)
	if counter.Value() != 11 {
		t.Errorf("Expected value of 11 after Add(10), got %d", counter.Value())
	}
}

func TestGauge(t *testing.T) {
	gauge := NewGauge()

	if gauge.Value() != 0 {
		t.Errorf("Expected initial value of 0, got %d", gauge.Value())
	}

	gauge.Set(10)
	if gauge.Value() != 10 {
		t.Errorf("Expected value of 10 after Set(10), got %d", gauge.Value())
	}

	gauge.Inc()
	if gauge.Value() != 11 {
		t.Errorf("Expected value of 11 after Inc(), got %d", gauge.Value())
	}

	gauge.Dec()
	if gauge.Value() != 10 {
		t.Errorf("Expected value of 10 after Dec(), got %d", gauge.Value())
	}
}

func TestHistogram(t *testing.T) {
	hist := NewHistogram()

	if hist.Count() != 0 {
		t.Errorf("Expected initial count of 0, got %d", hist.Count())
	}

	if hist.Sum() != 0 {
		t.Errorf("Expected initial sum of 0, got %d", hist.Sum())
	}

	if hist.Average() != 0 {
		t.Errorf("Expected initial average of 0, got %f", hist.Average())
	}

	hist.Observe(100)
	hist.Observe(200)
	hist.Observe(300)

	if hist.Count() != 3 {
		t.Errorf("Expected count of 3, got %d", hist.Count())
	}

	if hist.Sum() != 600 {
		t.Errorf("Expected sum of 600, got %d", hist.Sum())
	}

	avg := hist.Average()
	if avg != 200.0 {
		t.Errorf("Expected average of 200.0, got %f", avg)
	}
}
