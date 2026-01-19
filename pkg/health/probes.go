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

	// ProbeStartup checks if the application has started successfully.
	// Used during initial startup to allow slow-starting containers.
	ProbeStartup ProbeType = "startup"
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

	m.mu.Lock()
	defer m.mu.Unlock()

	// Get all checkers
	m.Monitor.mu.RLock()
	checkers := make([]Checker, 0, len(m.Monitor.checkers))
	for _, checker := range m.Monitor.checkers {
		checkers = append(checkers, checker)
	}
	m.Monitor.mu.RUnlock()

	// Check each component with caching
	components := make(map[string]ComponentHealth)
	for _, checker := range checkers {
		name := checker.Name()

		// Check cache
		if cached, ok := m.cache[name]; ok {
			if time.Since(cached.timestamp) < m.cacheConfig.TTL {
				components[name] = cached.result
				continue
			}
		}

		// Perform fresh check
		startTime := time.Now()
		health := checker.Check(ctx)
		health.ResponseTimeMs = time.Since(startTime).Milliseconds()

		// Update cache
		m.cache[name] = &cachedResult{
			result:    health,
			timestamp: time.Now(),
		}
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

// CheckLiveness performs liveness checks on all components with caching.
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

	allHealthy := true
	var failedComponent string

	for _, checker := range checkers {
		name := checker.Name()
		healthy := true

		// Check if component supports liveness checks
		if lc, ok := checker.(LivenessChecker); ok {
			// Check cache
			if m.cacheConfig.Enabled {
				m.mu.RLock()
				if cached, exists := m.livenessCache[name]; exists {
					if time.Since(cached.timestamp) < ttl {
						healthy = cached.healthy
						m.mu.RUnlock()
						if !healthy {
							allHealthy = false
							failedComponent = name
						}
						continue
					}
				}
				m.mu.RUnlock()
			}

			// Perform fresh check
			healthy = lc.CheckLiveness(ctx)

			// Update cache
			if m.cacheConfig.Enabled {
				m.mu.Lock()
				m.livenessCache[name] = &cachedProbeResult{
					healthy:   healthy,
					timestamp: time.Now(),
				}
				m.mu.Unlock()
			}
		} else if pc, ok := checker.(ProbeChecker); ok {
			// Check cache
			if m.cacheConfig.Enabled {
				m.mu.RLock()
				if cached, exists := m.livenessCache[name]; exists {
					if time.Since(cached.timestamp) < ttl {
						healthy = cached.healthy
						m.mu.RUnlock()
						if !healthy {
							allHealthy = false
							failedComponent = name
						}
						continue
					}
				}
				m.mu.RUnlock()
			}

			// Perform fresh check
			healthy = pc.CheckLiveness(ctx)

			// Update cache
			if m.cacheConfig.Enabled {
				m.mu.Lock()
				m.livenessCache[name] = &cachedProbeResult{
					healthy:   healthy,
					timestamp: time.Now(),
				}
				m.mu.Unlock()
			}
		} else {
			// Default to regular health check for liveness
			result := checker.Check(ctx)
			healthy = result.Status != StatusUnhealthy
		}

		if !healthy {
			allHealthy = false
			failedComponent = name
		}
	}

	msg := "All components are alive"
	if !allHealthy {
		msg = "Component " + failedComponent + " failed liveness check"
	}

	return ProbeResult{
		Type:      ProbeLiveness,
		Healthy:   allHealthy,
		Message:   msg,
		Timestamp: time.Now(),
		Duration:  time.Since(start).Milliseconds(),
	}
}

// CheckReadiness performs readiness checks on all components with caching.
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

	allReady := true
	var notReadyComponent string

	for _, checker := range checkers {
		name := checker.Name()
		ready := true

		// Check if component supports readiness checks
		if rc, ok := checker.(ReadinessChecker); ok {
			// Check cache
			if m.cacheConfig.Enabled {
				m.mu.RLock()
				if cached, exists := m.readinessCache[name]; exists {
					if time.Since(cached.timestamp) < ttl {
						ready = cached.healthy
						m.mu.RUnlock()
						if !ready {
							allReady = false
							notReadyComponent = name
						}
						continue
					}
				}
				m.mu.RUnlock()
			}

			// Perform fresh check
			ready = rc.CheckReadiness(ctx)

			// Update cache
			if m.cacheConfig.Enabled {
				m.mu.Lock()
				m.readinessCache[name] = &cachedProbeResult{
					healthy:   ready,
					timestamp: time.Now(),
				}
				m.mu.Unlock()
			}
		} else if pc, ok := checker.(ProbeChecker); ok {
			// Check cache
			if m.cacheConfig.Enabled {
				m.mu.RLock()
				if cached, exists := m.readinessCache[name]; exists {
					if time.Since(cached.timestamp) < ttl {
						ready = cached.healthy
						m.mu.RUnlock()
						if !ready {
							allReady = false
							notReadyComponent = name
						}
						continue
					}
				}
				m.mu.RUnlock()
			}

			// Perform fresh check
			ready = pc.CheckReadiness(ctx)

			// Update cache
			if m.cacheConfig.Enabled {
				m.mu.Lock()
				m.readinessCache[name] = &cachedProbeResult{
					healthy:   ready,
					timestamp: time.Now(),
				}
				m.mu.Unlock()
			}
		} else {
			// Default: healthy or degraded = ready
			result := checker.Check(ctx)
			ready = result.Status == StatusHealthy || result.Status == StatusDegraded
		}

		if !ready {
			allReady = false
			notReadyComponent = name
		}
	}

	msg := "All components are ready"
	if !allReady {
		msg = "Component " + notReadyComponent + " is not ready"
	}

	return ProbeResult{
		Type:      ProbeReadiness,
		Healthy:   allReady,
		Message:   msg,
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
func (m *CachedMonitor) GetCacheConfig() CacheConfig {
	return m.cacheConfig
}

// SetCacheConfig updates the cache configuration.
// This method is not thread-safe and should only be called before starting the monitor.
func (m *CachedMonitor) SetCacheConfig(config CacheConfig) {
	m.cacheConfig = config
}
