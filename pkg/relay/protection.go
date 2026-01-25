package relay

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// ProtectionManager implements DoS protection for relay operations
type ProtectionManager struct {
	// Connection tracking per IP
	connCounts    map[string]*ipConnectionTracker
	connMu        sync.RWMutex
	maxConnsPerIP int

	// Circuit tracking per connection
	circuitCounts      map[string]*connCircuitTracker
	circuitMu          sync.RWMutex
	maxCircuitsPerConn int

	// Global limits
	maxTotalConnections int64
	totalConnections    int64

	// Cleanup
	cleanupInterval time.Duration
	lastCleanup     time.Time
	cleanupMu       sync.Mutex

	// Metrics
	metrics *RelayMetrics
}

// ipConnectionTracker tracks connections from a single IP
type ipConnectionTracker struct {
	count      int32
	lastAccess time.Time
	mu         sync.Mutex
}

// connCircuitTracker tracks circuits on a single connection
type connCircuitTracker struct {
	count      int32
	lastAccess time.Time
	mu         sync.Mutex
}

// ProtectionConfig holds DoS protection configuration
type ProtectionConfig struct {
	MaxConnectionsPerIP int           // Maximum connections per IP address
	MaxCircuitsPerConn  int           // Maximum circuits per connection
	MaxTotalConnections int           // Global connection limit
	CleanupInterval     time.Duration // Interval for cleanup of stale trackers
	Metrics             *RelayMetrics
}

// DefaultProtectionConfig returns sensible defaults
func DefaultProtectionConfig() *ProtectionConfig {
	return &ProtectionConfig{
		MaxConnectionsPerIP: 10,   // 10 concurrent connections per IP
		MaxCircuitsPerConn:  1000, // 1000 circuits per connection
		MaxTotalConnections: 5000, // 5000 total connections
		CleanupInterval:     5 * time.Minute,
	}
}

// NewProtectionManager creates a new DoS protection manager
func NewProtectionManager(cfg *ProtectionConfig) *ProtectionManager {
	if cfg == nil {
		cfg = DefaultProtectionConfig()
	}

	return &ProtectionManager{
		connCounts:          make(map[string]*ipConnectionTracker),
		maxConnsPerIP:       cfg.MaxConnectionsPerIP,
		circuitCounts:       make(map[string]*connCircuitTracker),
		maxCircuitsPerConn:  cfg.MaxCircuitsPerConn,
		maxTotalConnections: int64(cfg.MaxTotalConnections),
		cleanupInterval:     cfg.CleanupInterval,
		lastCleanup:         time.Now(),
		metrics:             cfg.Metrics,
	}
}

// AllowConnection checks if a new connection should be accepted
func (pm *ProtectionManager) AllowConnection(remoteAddr string) error {
	// Parse IP from address
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		// If parsing fails, use full address
		host = remoteAddr
	}

	// Check global connection limit
	current := atomic.LoadInt64(&pm.totalConnections)
	if pm.maxTotalConnections > 0 && current >= pm.maxTotalConnections {
		if pm.metrics != nil {
			pm.metrics.DoSConnectionsRejected.Inc()
		}
		return fmt.Errorf("global connection limit reached (%d)", pm.maxTotalConnections)
	}

	// Check per-IP connection limit
	pm.connMu.Lock()
	tracker, exists := pm.connCounts[host]
	if !exists {
		tracker = &ipConnectionTracker{
			lastAccess: time.Now(),
		}
		pm.connCounts[host] = tracker
	}
	pm.connMu.Unlock()

	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	if pm.maxConnsPerIP > 0 && int(tracker.count) >= pm.maxConnsPerIP {
		if pm.metrics != nil {
			pm.metrics.DoSConnectionsRejected.Inc()
		}
		return fmt.Errorf("connection limit per IP exceeded for %s (%d)", host, pm.maxConnsPerIP)
	}

	// Allow connection
	tracker.count++
	tracker.lastAccess = time.Now()
	atomic.AddInt64(&pm.totalConnections, 1)

	if pm.metrics != nil {
		pm.metrics.ActiveConnections.Set(int(atomic.LoadInt64(&pm.totalConnections)))
	}

	return nil
}

// ReleaseConnection decrements connection count for an IP
func (pm *ProtectionManager) ReleaseConnection(remoteAddr string) {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}

	pm.connMu.RLock()
	tracker, exists := pm.connCounts[host]
	pm.connMu.RUnlock()

	if exists {
		tracker.mu.Lock()
		if tracker.count > 0 {
			tracker.count--
		}
		tracker.lastAccess = time.Now()
		tracker.mu.Unlock()
	}

	current := atomic.AddInt64(&pm.totalConnections, -1)
	if current < 0 {
		atomic.StoreInt64(&pm.totalConnections, 0)
		current = 0
	}

	if pm.metrics != nil {
		pm.metrics.ActiveConnections.Set(int(current))
	}

	// Periodic cleanup
	pm.maybeCleanup()
}

// AllowCircuit checks if a new circuit can be created on the connection
func (pm *ProtectionManager) AllowCircuit(remoteAddr string) error {
	pm.circuitMu.Lock()
	tracker, exists := pm.circuitCounts[remoteAddr]
	if !exists {
		tracker = &connCircuitTracker{
			lastAccess: time.Now(),
		}
		pm.circuitCounts[remoteAddr] = tracker
	}
	pm.circuitMu.Unlock()

	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	if pm.maxCircuitsPerConn > 0 && int(tracker.count) >= pm.maxCircuitsPerConn {
		if pm.metrics != nil {
			pm.metrics.DoSCircuitsRejected.Inc()
		}
		return fmt.Errorf("circuit limit per connection exceeded for %s (%d)", remoteAddr, pm.maxCircuitsPerConn)
	}

	tracker.count++
	tracker.lastAccess = time.Now()

	if pm.metrics != nil {
		pm.metrics.ActiveCircuits.Inc()
	}

	return nil
}

// ReleaseCircuit decrements circuit count for a connection
func (pm *ProtectionManager) ReleaseCircuit(remoteAddr string) {
	pm.circuitMu.RLock()
	tracker, exists := pm.circuitCounts[remoteAddr]
	pm.circuitMu.RUnlock()

	if exists {
		tracker.mu.Lock()
		if tracker.count > 0 {
			tracker.count--
		}
		tracker.lastAccess = time.Now()
		tracker.mu.Unlock()
	}

	if pm.metrics != nil {
		pm.metrics.ActiveCircuits.Dec()
	}
}

// maybeCleanup performs periodic cleanup of stale trackers
func (pm *ProtectionManager) maybeCleanup() {
	pm.cleanupMu.Lock()
	defer pm.cleanupMu.Unlock()

	if time.Since(pm.lastCleanup) < pm.cleanupInterval {
		return
	}

	now := time.Now()
	staleThreshold := 10 * time.Minute

	// Cleanup connection trackers
	pm.connMu.Lock()
	for ip, tracker := range pm.connCounts {
		tracker.mu.Lock()
		if tracker.count == 0 && now.Sub(tracker.lastAccess) > staleThreshold {
			delete(pm.connCounts, ip)
		}
		tracker.mu.Unlock()
	}
	pm.connMu.Unlock()

	// Cleanup circuit trackers
	pm.circuitMu.Lock()
	for addr, tracker := range pm.circuitCounts {
		tracker.mu.Lock()
		if tracker.count == 0 && now.Sub(tracker.lastAccess) > staleThreshold {
			delete(pm.circuitCounts, addr)
		}
		tracker.mu.Unlock()
	}
	pm.circuitMu.Unlock()

	pm.lastCleanup = now
}

// Stats returns current protection statistics
func (pm *ProtectionManager) Stats() ProtectionStats {
	pm.connMu.RLock()
	ipTrackers := len(pm.connCounts)
	pm.connMu.RUnlock()

	pm.circuitMu.RLock()
	connTrackers := len(pm.circuitCounts)
	pm.circuitMu.RUnlock()

	return ProtectionStats{
		TotalConnections:    int(atomic.LoadInt64(&pm.totalConnections)),
		MaxTotalConnections: int(pm.maxTotalConnections),
		TrackedIPs:          ipTrackers,
		TrackedConnections:  connTrackers,
		MaxConnsPerIP:       pm.maxConnsPerIP,
		MaxCircuitsPerConn:  pm.maxCircuitsPerConn,
	}
}

// ProtectionStats contains DoS protection statistics
type ProtectionStats struct {
	TotalConnections    int // Current total connections
	MaxTotalConnections int // Maximum allowed total connections
	TrackedIPs          int // Number of IPs being tracked
	TrackedConnections  int // Number of connections being tracked for circuits
	MaxConnsPerIP       int // Maximum connections allowed per IP
	MaxCircuitsPerConn  int // Maximum circuits allowed per connection
}
