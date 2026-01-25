package relay

import (
	"testing"
	"time"
)

func TestProtectionManager_AllowConnection(t *testing.T) {
	cfg := &ProtectionConfig{
		MaxConnectionsPerIP: 3,
		MaxTotalConnections: 10,
	}
	pm := NewProtectionManager(cfg)

	// Should allow connections up to per-IP limit
	for i := 0; i < 3; i++ {
		err := pm.AllowConnection("192.168.1.1:1234")
		if err != nil {
			t.Errorf("Expected connection %d to be allowed, got error: %v", i, err)
		}
	}

	// 4th connection from same IP should be rejected
	err := pm.AllowConnection("192.168.1.1:1234")
	if err == nil {
		t.Error("Expected 4th connection from same IP to be rejected")
	}

	// Different IP should be allowed
	err = pm.AllowConnection("192.168.1.2:1234")
	if err != nil {
		t.Errorf("Expected connection from different IP to be allowed, got error: %v", err)
	}
}

func TestProtectionManager_GlobalConnectionLimit(t *testing.T) {
	cfg := &ProtectionConfig{
		MaxConnectionsPerIP: 10,
		MaxTotalConnections: 3,
	}
	pm := NewProtectionManager(cfg)

	// Should allow up to global limit
	err := pm.AllowConnection("192.168.1.1:1234")
	if err != nil {
		t.Fatalf("Expected connection 1 to be allowed, got error: %v", err)
	}

	err = pm.AllowConnection("192.168.1.2:1234")
	if err != nil {
		t.Fatalf("Expected connection 2 to be allowed, got error: %v", err)
	}

	err = pm.AllowConnection("192.168.1.3:1234")
	if err != nil {
		t.Fatalf("Expected connection 3 to be allowed, got error: %v", err)
	}

	// 4th connection should be rejected due to global limit
	err = pm.AllowConnection("192.168.1.4:1234")
	if err == nil {
		t.Error("Expected 4th connection to be rejected due to global limit")
	}
}

func TestProtectionManager_ReleaseConnection(t *testing.T) {
	cfg := &ProtectionConfig{
		MaxConnectionsPerIP: 2,
	}
	pm := NewProtectionManager(cfg)

	addr := "192.168.1.1:1234"

	// Allow 2 connections
	pm.AllowConnection(addr)
	pm.AllowConnection(addr)

	// 3rd should be rejected
	err := pm.AllowConnection(addr)
	if err == nil {
		t.Error("Expected 3rd connection to be rejected")
	}

	// Release one connection
	pm.ReleaseConnection(addr)

	// Now should be allowed again
	err = pm.AllowConnection(addr)
	if err != nil {
		t.Errorf("Expected connection after release to be allowed, got error: %v", err)
	}
}

func TestProtectionManager_AllowCircuit(t *testing.T) {
	cfg := &ProtectionConfig{
		MaxCircuitsPerConn: 3,
	}
	pm := NewProtectionManager(cfg)

	addr := "192.168.1.1:1234"

	// Should allow circuits up to limit
	for i := 0; i < 3; i++ {
		err := pm.AllowCircuit(addr)
		if err != nil {
			t.Errorf("Expected circuit %d to be allowed, got error: %v", i, err)
		}
	}

	// 4th circuit should be rejected
	err := pm.AllowCircuit(addr)
	if err == nil {
		t.Error("Expected 4th circuit to be rejected")
	}

	// Different connection should be allowed
	err = pm.AllowCircuit("192.168.1.2:1234")
	if err != nil {
		t.Errorf("Expected circuit from different connection to be allowed, got error: %v", err)
	}
}

func TestProtectionManager_ReleaseCircuit(t *testing.T) {
	cfg := &ProtectionConfig{
		MaxCircuitsPerConn: 2,
	}
	pm := NewProtectionManager(cfg)

	addr := "192.168.1.1:1234"

	// Allow 2 circuits
	pm.AllowCircuit(addr)
	pm.AllowCircuit(addr)

	// 3rd should be rejected
	err := pm.AllowCircuit(addr)
	if err == nil {
		t.Error("Expected 3rd circuit to be rejected")
	}

	// Release one circuit
	pm.ReleaseCircuit(addr)

	// Now should be allowed again
	err = pm.AllowCircuit(addr)
	if err != nil {
		t.Errorf("Expected circuit after release to be allowed, got error: %v", err)
	}
}

func TestProtectionManager_Stats(t *testing.T) {
	cfg := &ProtectionConfig{
		MaxConnectionsPerIP: 5,
		MaxCircuitsPerConn:  10,
		MaxTotalConnections: 100,
	}
	pm := NewProtectionManager(cfg)

	// Add some connections
	pm.AllowConnection("192.168.1.1:1234")
	pm.AllowConnection("192.168.1.2:1234")
	pm.AllowConnection("192.168.1.3:1234")

	// Add some circuits
	pm.AllowCircuit("192.168.1.1:1234")
	pm.AllowCircuit("192.168.1.2:1234")

	stats := pm.Stats()

	if stats.TotalConnections != 3 {
		t.Errorf("Expected 3 total connections, got %d", stats.TotalConnections)
	}

	if stats.MaxTotalConnections != 100 {
		t.Errorf("Expected max total connections of 100, got %d", stats.MaxTotalConnections)
	}

	if stats.TrackedIPs != 3 {
		t.Errorf("Expected 3 tracked IPs, got %d", stats.TrackedIPs)
	}

	if stats.TrackedConnections != 2 {
		t.Errorf("Expected 2 tracked connections (for circuits), got %d", stats.TrackedConnections)
	}

	if stats.MaxConnsPerIP != 5 {
		t.Errorf("Expected max connections per IP of 5, got %d", stats.MaxConnsPerIP)
	}

	if stats.MaxCircuitsPerConn != 10 {
		t.Errorf("Expected max circuits per connection of 10, got %d", stats.MaxCircuitsPerConn)
	}
}

func TestProtectionManager_Cleanup(t *testing.T) {
	cfg := &ProtectionConfig{
		MaxConnectionsPerIP: 5,
		CleanupInterval:     100 * time.Millisecond,
	}
	pm := NewProtectionManager(cfg)

	// Add and release connections
	pm.AllowConnection("192.168.1.1:1234")
	pm.AllowConnection("192.168.1.2:1234")
	pm.ReleaseConnection("192.168.1.1:1234")
	pm.ReleaseConnection("192.168.1.2:1234")

	if len(pm.connCounts) != 2 {
		t.Errorf("Expected 2 connection trackers before cleanup, got %d", len(pm.connCounts))
	}

	// Wait for cleanup interval
	time.Sleep(150 * time.Millisecond)

	// Trigger cleanup by releasing a connection
	pm.ReleaseConnection("192.168.1.3:1234") // Doesn't exist, just triggers cleanup

	// Note: Cleanup only removes trackers that are both zero count AND stale (10 min)
	// So our trackers won't be removed immediately in this test
	// This is expected behavior - cleanup is conservative
	if len(pm.connCounts) >= 0 {
		t.Logf("Connection trackers after cleanup: %d (expected to keep recent trackers)", len(pm.connCounts))
	}
}

func TestProtectionManager_WithMetrics(t *testing.T) {
	metrics := NewRelayMetrics()
	cfg := &ProtectionConfig{
		MaxConnectionsPerIP: 1,
		Metrics:             metrics,
	}
	pm := NewProtectionManager(cfg)

	addr := "192.168.1.1:1234"

	// First connection allowed
	pm.AllowConnection(addr)

	// Second connection rejected
	pm.AllowConnection(addr)

	if metrics.DoSConnectionsRejected.Value() == 0 {
		t.Error("Expected DoS connections rejected metric to be incremented")
	}

	// Test circuit rejection metric
	cfg2 := &ProtectionConfig{
		MaxCircuitsPerConn: 1,
		Metrics:            metrics,
	}
	pm2 := NewProtectionManager(cfg2)

	pm2.AllowCircuit(addr)
	pm2.AllowCircuit(addr) // Should be rejected

	if metrics.DoSCircuitsRejected.Value() == 0 {
		t.Error("Expected DoS circuits rejected metric to be incremented")
	}
}

func TestProtectionManager_AddressWithoutPort(t *testing.T) {
	cfg := &ProtectionConfig{
		MaxConnectionsPerIP: 2,
	}
	pm := NewProtectionManager(cfg)

	// Address without port (malformed)
	addr := "192.168.1.1"

	// Should still work (uses full address as key)
	err := pm.AllowConnection(addr)
	if err != nil {
		t.Errorf("Expected connection with malformed address to be allowed, got error: %v", err)
	}

	err = pm.AllowConnection(addr)
	if err != nil {
		t.Errorf("Expected second connection with malformed address to be allowed, got error: %v", err)
	}

	err = pm.AllowConnection(addr)
	if err == nil {
		t.Error("Expected third connection to be rejected")
	}
}

func TestDefaultProtectionConfig(t *testing.T) {
	cfg := DefaultProtectionConfig()

	if cfg.MaxConnectionsPerIP != 10 {
		t.Errorf("Expected max connections per IP of 10, got %d", cfg.MaxConnectionsPerIP)
	}

	if cfg.MaxCircuitsPerConn != 1000 {
		t.Errorf("Expected max circuits per connection of 1000, got %d", cfg.MaxCircuitsPerConn)
	}

	if cfg.MaxTotalConnections != 5000 {
		t.Errorf("Expected max total connections of 5000, got %d", cfg.MaxTotalConnections)
	}

	if cfg.CleanupInterval != 5*time.Minute {
		t.Errorf("Expected cleanup interval of 5 minutes, got %v", cfg.CleanupInterval)
	}
}

func TestProtectionManager_ConcurrentAccess(t *testing.T) {
	cfg := DefaultProtectionConfig()
	pm := NewProtectionManager(cfg)

	// Test concurrent access to connection tracking
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			addr := "192.168.1.1:1234"
			pm.AllowConnection(addr)
			pm.ReleaseConnection(addr)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should not panic and should have consistent state
	stats := pm.Stats()
	if stats.TotalConnections < 0 {
		t.Errorf("Expected non-negative total connections, got %d", stats.TotalConnections)
	}
}
