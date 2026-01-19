// Package health provides health check and monitoring capabilities for the Tor client.
package health

import (
	"context"
	"sync"
	"time"
)

// ProbeType represents the type of health probe.
type ProbeType string

const (
	// ProbeLiveness checks if the application is running.
	// If liveness fails, the application should be restarted.
	ProbeLiveness ProbeType = "liveness"

	// ProbeReadiness checks if the application is ready to receive traffic.
	// If readiness fails, the application should be removed from load balancing.
	ProbeReadiness ProbeType = "readiness"
)

// ProbeResult represents the result of a probe check.
type ProbeResult struct {
	Type      ProbeType `json:"type"`
	Healthy   bool      `json:"healthy"`
	Message   string    `json:"message,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Duration  int64     `json:"duration_ms"`
}

// ProbeChecker defines the interface for components that support probe checks.
type ProbeChecker interface {
	Checker
	// CheckLiveness performs a liveness check.
	// Returns true if the component is alive and functioning.
	CheckLiveness(ctx context.Context) bool
	// CheckReadiness performs a readiness check.
	// Returns true if the component is ready to receive traffic.
	CheckReadiness(ctx context.Context) bool
}

// LivenessChecker is a simpler interface for components that only need liveness checks.
type LivenessChecker interface {
	Checker
	// CheckLiveness performs a liveness check.
	CheckLiveness(ctx context.Context) bool
}

// ReadinessChecker is a simpler interface for components that only need readiness checks.
type ReadinessChecker interface {
	Checker
	// CheckReadiness performs a readiness check.
	CheckReadiness(ctx context.Context) bool
}

// CacheConfig configures health check result caching.
type CacheConfig struct {
	// Enabled determines if caching is enabled.
	Enabled bool
	// TTL is the duration for which cached results are valid.
	TTL time.Duration
	// LivenessTTL is the TTL for liveness checks (defaults to TTL if zero).
	LivenessTTL time.Duration
	// ReadinessTTL is the TTL for readiness checks (defaults to TTL if zero).
	ReadinessTTL time.Duration
}

// DefaultCacheConfig returns sensible defaults for health check caching.
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		Enabled:      true,
		TTL:          5 * time.Second,
		LivenessTTL:  1 * time.Second, // Liveness checks should be fresher
		ReadinessTTL: 3 * time.Second, // Readiness can be slightly stale
	}
}

// cachedResult stores a cached health check result.
type cachedResult struct {
	result    ComponentHealth
	timestamp time.Time
}

// cachedProbeResult stores a cached probe result.
type cachedProbeResult struct {
	healthy   bool
	timestamp time.Time
}

// CachedMonitor wraps Monitor with caching capabilities.
type CachedMonitor struct {
	*Monitor
	cacheConfig    CacheConfig
	mu             sync.RWMutex
	cache          map[string]*cachedResult
	livenessCache  map[string]*cachedProbeResult
	readinessCache map[string]*cachedProbeResult
}

// NewCachedMonitor creates a new cached health monitor.
func NewCachedMonitor(config CacheConfig) *CachedMonitor {
	return &CachedMonitor{
		Monitor:        NewMonitor(),
		cacheConfig:    config,
		cache:          make(map[string]*cachedResult),
		livenessCache:  make(map[string]*cachedProbeResult),
		readinessCache: make(map[string]*cachedProbeResult),
	}
}

// Check performs health checks with caching.
func (m *CachedMonitor) Check(ctx context.Context) OverallHealth {
	if !m.cacheConfig.Enabled {
		return m.Monitor.Check(ctx)
	}

	// Get all checkers first (short lock on Monitor)
	m.Monitor.mu.RLock()
	checkers := make([]Checker, 0, len(m.Monitor.checkers))
	for _, checker := range m.Monitor.checkers {
		checkers = append(checkers, checker)
	}
	m.Monitor.mu.RUnlock()

	// Check each component with caching
	components := make(map[string]ComponentHealth)
	for _, checker := range checkers {
		// Check context cancellation
		if ctx.Err() != nil {
			break
		}

		name := checker.Name()

		// Check cache (short read lock)
		m.mu.RLock()
		cached, ok := m.cache[name]
		cacheValid := ok && time.Since(cached.timestamp) < m.cacheConfig.TTL
		if cacheValid {
			components[name] = cached.result
		}
		m.mu.RUnlock()

		if cacheValid {
			continue
		}

		// Perform fresh check (no lock held during potentially slow operation)
		startTime := time.Now()
		health := checker.Check(ctx)
		health.ResponseTimeMs = time.Since(startTime).Milliseconds()

		// Update cache (short write lock)
		m.mu.Lock()
		m.cache[name] = &cachedResult{
			result:    health,
			timestamp: time.Now(),
		}
		m.mu.Unlock()

		components[name] = health
	}

	// Determine overall status
	overallStatus := StatusHealthy
	for _, health := range components {
		if health.Status == StatusUnhealthy {
			overallStatus = StatusUnhealthy
			break
		} else if health.Status == StatusDegraded && overallStatus == StatusHealthy {
			overallStatus = StatusDegraded
		}
	}

	return OverallHealth{
		Status:     overallStatus,
		Components: components,
		Timestamp:  time.Now(),
		Uptime:     time.Since(m.Monitor.startTime),
	}
}

// probeCheckResult holds the result of a single probe check.
type probeCheckResult struct {
	name    string
	healthy bool
}

// checkProbeWithCache is a helper that checks cache and performs a probe check.
// It returns the health status and whether a failure was found.
func (m *CachedMonitor) checkProbeWithCache(
	ctx context.Context,
	checker Checker,
	cache map[string]*cachedProbeResult,
	ttl time.Duration,
	checkFunc func(ctx context.Context) bool,
) (healthy bool, fromCache bool) {
	name := checker.Name()

	// Check cache (short read lock)
	if m.cacheConfig.Enabled {
		m.mu.RLock()
		if cached, exists := cache[name]; exists {
			if time.Since(cached.timestamp) < ttl {
				m.mu.RUnlock()
				return cached.healthy, true
			}
		}
		m.mu.RUnlock()
	}

	// Perform fresh check (no lock held)
	healthy = checkFunc(ctx)

	// Update cache (short write lock)
	if m.cacheConfig.Enabled {
		m.mu.Lock()
		cache[name] = &cachedProbeResult{
			healthy:   healthy,
			timestamp: time.Now(),
		}
		m.mu.Unlock()
	}

	return healthy, false
}

// CheckLiveness performs liveness checks on all components with caching.
// Returns early on first failure for efficiency.
func (m *CachedMonitor) CheckLiveness(ctx context.Context) ProbeResult {
	start := time.Now()

	// Get all checkers
	m.Monitor.mu.RLock()
	checkers := make([]Checker, 0, len(m.Monitor.checkers))
	for _, checker := range m.Monitor.checkers {
		checkers = append(checkers, checker)
	}
	m.Monitor.mu.RUnlock()

	ttl := m.cacheConfig.LivenessTTL
	if ttl == 0 {
		ttl = m.cacheConfig.TTL
	}

	var failedComponent string

	for _, checker := range checkers {
		// Check context cancellation
		if ctx.Err() != nil {
			return ProbeResult{
				Type:      ProbeLiveness,
				Healthy:   false,
				Message:   "Context cancelled during liveness check",
				Timestamp: time.Now(),
				Duration:  time.Since(start).Milliseconds(),
			}
		}

		name := checker.Name()
		var healthy bool

		// Check if component supports liveness checks
		if lc, ok := checker.(LivenessChecker); ok {
			healthy, _ = m.checkProbeWithCache(ctx, checker, m.livenessCache, ttl, lc.CheckLiveness)
		} else if pc, ok := checker.(ProbeChecker); ok {
			healthy, _ = m.checkProbeWithCache(ctx, checker, m.livenessCache, ttl, pc.CheckLiveness)
		} else {
			// Default to regular health check for liveness
			result := checker.Check(ctx)
			healthy = result.Status != StatusUnhealthy
		}

		if !healthy {
			failedComponent = name
			// Early exit on first failure
			return ProbeResult{
				Type:      ProbeLiveness,
				Healthy:   false,
				Message:   "Component " + failedComponent + " failed liveness check",
				Timestamp: time.Now(),
				Duration:  time.Since(start).Milliseconds(),
			}
		}
	}

	return ProbeResult{
		Type:      ProbeLiveness,
		Healthy:   true,
		Message:   "All components are alive",
		Timestamp: time.Now(),
		Duration:  time.Since(start).Milliseconds(),
	}
}

// CheckReadiness performs readiness checks on all components with caching.
// Returns early on first failure for efficiency.
func (m *CachedMonitor) CheckReadiness(ctx context.Context) ProbeResult {
	start := time.Now()

	// Get all checkers
	m.Monitor.mu.RLock()
	checkers := make([]Checker, 0, len(m.Monitor.checkers))
	for _, checker := range m.Monitor.checkers {
		checkers = append(checkers, checker)
	}
	m.Monitor.mu.RUnlock()

	ttl := m.cacheConfig.ReadinessTTL
	if ttl == 0 {
		ttl = m.cacheConfig.TTL
	}

	var notReadyComponent string

	for _, checker := range checkers {
		// Check context cancellation
		if ctx.Err() != nil {
			return ProbeResult{
				Type:      ProbeReadiness,
				Healthy:   false,
				Message:   "Context cancelled during readiness check",
				Timestamp: time.Now(),
				Duration:  time.Since(start).Milliseconds(),
			}
		}

		name := checker.Name()
		var ready bool

		// Check if component supports readiness checks
		if rc, ok := checker.(ReadinessChecker); ok {
			ready, _ = m.checkProbeWithCache(ctx, checker, m.readinessCache, ttl, rc.CheckReadiness)
		} else if pc, ok := checker.(ProbeChecker); ok {
			ready, _ = m.checkProbeWithCache(ctx, checker, m.readinessCache, ttl, pc.CheckReadiness)
		} else {
			// Default: healthy or degraded = ready
			result := checker.Check(ctx)
			ready = result.Status == StatusHealthy || result.Status == StatusDegraded
		}

		if !ready {
			notReadyComponent = name
			// Early exit on first failure
			return ProbeResult{
				Type:      ProbeReadiness,
				Healthy:   false,
				Message:   "Component " + notReadyComponent + " is not ready",
				Timestamp: time.Now(),
				Duration:  time.Since(start).Milliseconds(),
			}
		}
	}

	return ProbeResult{
		Type:      ProbeReadiness,
		Healthy:   true,
		Message:   "All components are ready",
		Timestamp: time.Now(),
		Duration:  time.Since(start).Milliseconds(),
	}
}

// InvalidateCache clears all cached health check results.
func (m *CachedMonitor) InvalidateCache() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cache = make(map[string]*cachedResult)
	m.livenessCache = make(map[string]*cachedProbeResult)
	m.readinessCache = make(map[string]*cachedProbeResult)
}

// InvalidateCacheFor clears cached results for a specific component.
func (m *CachedMonitor) InvalidateCacheFor(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.cache, name)
	delete(m.livenessCache, name)
	delete(m.readinessCache, name)
}

// GetCacheConfig returns the current cache configuration.
// This method is thread-safe.
func (m *CachedMonitor) GetCacheConfig() CacheConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cacheConfig
}

// SetCacheConfig updates the cache configuration.
// This method is thread-safe.
func (m *CachedMonitor) SetCacheConfig(config CacheConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cacheConfig = config
}
