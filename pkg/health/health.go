// Package health provides health check and monitoring capabilities for the Tor client.
// This package implements health checks for circuits, connections, and overall system status.
package health

import (
	"context"
	"sync"
	"time"
)

// Status represents the health status of a component
type Status string

const (
	// StatusHealthy indicates the component is functioning normally
	StatusHealthy Status = "healthy"
	// StatusDegraded indicates the component is functioning but with reduced capacity
	StatusDegraded Status = "degraded"
	// StatusUnhealthy indicates the component is not functioning properly
	StatusUnhealthy Status = "unhealthy"
)

// ComponentHealth represents the health of a single component
type ComponentHealth struct {
	Name           string                 `json:"name"`
	Status         Status                 `json:"status"`
	Message        string                 `json:"message,omitempty"`
	LastChecked    time.Time              `json:"last_checked"`
	Details        map[string]interface{} `json:"details,omitempty"`
	ResponseTimeMs int64                  `json:"response_time_ms,omitempty"`
}

// OverallHealth represents the overall health of the Tor client
type OverallHealth struct {
	Status     Status                     `json:"status"`
	Components map[string]ComponentHealth `json:"components"`
	Timestamp  time.Time                  `json:"timestamp"`
	Uptime     time.Duration              `json:"uptime"`
}

// Checker defines the interface for health checks
type Checker interface {
	// Check performs a health check and returns the result
	Check(ctx context.Context) ComponentHealth
	// Name returns the name of the component being checked
	Name() string
}

// Monitor manages health checks for various components
type Monitor struct {
	mu         sync.RWMutex
	checkers   map[string]Checker
	lastChecks map[string]ComponentHealth
	startTime  time.Time
}

// NewMonitor creates a new health monitor
func NewMonitor() *Monitor {
	return &Monitor{
		checkers:   make(map[string]Checker),
		lastChecks: make(map[string]ComponentHealth),
		startTime:  time.Now(),
	}
}

// RegisterChecker registers a health checker for a component
func (m *Monitor) RegisterChecker(checker Checker) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.checkers[checker.Name()] = checker
}

// UnregisterChecker removes a health checker
func (m *Monitor) UnregisterChecker(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.checkers, name)
}

// Check performs health checks on all registered components
func (m *Monitor) Check(ctx context.Context) OverallHealth {
	m.mu.Lock()
	checkers := make([]Checker, 0, len(m.checkers))
	for _, checker := range m.checkers {
		checkers = append(checkers, checker)
	}
	m.mu.Unlock()

	// Perform checks concurrently
	resultsCh := make(chan ComponentHealth, len(checkers))
	for _, checker := range checkers {
		go func(c Checker) {
			startTime := time.Now()
			health := c.Check(ctx)
			health.ResponseTimeMs = time.Since(startTime).Milliseconds()
			resultsCh <- health
		}(checker)
	}

	// Collect results
	components := make(map[string]ComponentHealth)
	for i := 0; i < len(checkers); i++ {
		health := <-resultsCh
		components[health.Name] = health
	}

	// Update last checks cache
	m.mu.Lock()
	m.lastChecks = components
	m.mu.Unlock()

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
		Uptime:     time.Since(m.startTime),
	}
}

// GetLastCheck returns the last health check result
func (m *Monitor) GetLastCheck() OverallHealth {
	m.mu.RLock()
	defer m.mu.RUnlock()

	components := make(map[string]ComponentHealth)
	for name, health := range m.lastChecks {
		components[name] = health
	}

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
		Uptime:     time.Since(m.startTime),
	}
}

// CircuitHealthChecker checks the health of circuits
type CircuitHealthChecker struct {
	getStats func() CircuitStats
}

// CircuitStats contains circuit statistics for health checking
type CircuitStats struct {
	ActiveCircuits int
	MinRequired    int
	FailedBuilds   int
	AverageAge     time.Duration
	MaxAge         time.Duration
}

// NewCircuitHealthChecker creates a new circuit health checker
func NewCircuitHealthChecker(getStats func() CircuitStats) *CircuitHealthChecker {
	return &CircuitHealthChecker{
		getStats: getStats,
	}
}

// Name returns the checker name
func (c *CircuitHealthChecker) Name() string {
	return "circuits"
}

// Check performs the health check
func (c *CircuitHealthChecker) Check(ctx context.Context) ComponentHealth {
	stats := c.getStats()

	health := ComponentHealth{
		Name:        c.Name(),
		LastChecked: time.Now(),
		Details: map[string]interface{}{
			"active_circuits": stats.ActiveCircuits,
			"min_required":    stats.MinRequired,
			"failed_builds":   stats.FailedBuilds,
			"average_age":     stats.AverageAge.String(),
			"max_age":         stats.MaxAge.String(),
		},
	}

	// Determine status based on circuit count
	if stats.ActiveCircuits == 0 {
		health.Status = StatusUnhealthy
		health.Message = "No active circuits available"
	} else if stats.ActiveCircuits < stats.MinRequired {
		health.Status = StatusDegraded
		health.Message = "Circuit count below minimum threshold"
	} else {
		health.Status = StatusHealthy
		health.Message = "Circuits functioning normally"
	}

	return health
}

// ConnectionHealthChecker checks the health of connections
type ConnectionHealthChecker struct {
	getStats func() ConnectionStats
}

// ConnectionStats contains connection statistics for health checking
type ConnectionStats struct {
	TotalConnections   int
	OpenConnections    int
	FailedConnections  int
	AverageLatency     time.Duration
	ConnectionAttempts int
}

// NewConnectionHealthChecker creates a new connection health checker
func NewConnectionHealthChecker(getStats func() ConnectionStats) *ConnectionHealthChecker {
	return &ConnectionHealthChecker{
		getStats: getStats,
	}
}

// Name returns the checker name
func (c *ConnectionHealthChecker) Name() string {
	return "connections"
}

// Check performs the health check
func (c *ConnectionHealthChecker) Check(ctx context.Context) ComponentHealth {
	stats := c.getStats()

	health := ComponentHealth{
		Name:        c.Name(),
		LastChecked: time.Now(),
		Details: map[string]interface{}{
			"total_connections":   stats.TotalConnections,
			"open_connections":    stats.OpenConnections,
			"failed_connections":  stats.FailedConnections,
			"average_latency":     stats.AverageLatency.String(),
			"connection_attempts": stats.ConnectionAttempts,
		},
	}

	// Determine status based on connection health
	if stats.OpenConnections == 0 && stats.ConnectionAttempts > 0 {
		health.Status = StatusUnhealthy
		health.Message = "No open connections available"
	} else if stats.FailedConnections > stats.OpenConnections {
		health.Status = StatusDegraded
		health.Message = "High connection failure rate"
	} else {
		health.Status = StatusHealthy
		health.Message = "Connections functioning normally"
	}

	return health
}

// DirectoryHealthChecker checks the health of directory services
type DirectoryHealthChecker struct {
	getStats func() DirectoryStats
}

// DirectoryStats contains directory statistics for health checking
type DirectoryStats struct {
	LastConsensusUpdate time.Time
	ConsensusAge        time.Duration
	RelayCount          int
	GuardCount          int
	ExitCount           int
}

// NewDirectoryHealthChecker creates a new directory health checker
func NewDirectoryHealthChecker(getStats func() DirectoryStats) *DirectoryHealthChecker {
	return &DirectoryHealthChecker{
		getStats: getStats,
	}
}

// Name returns the checker name
func (d *DirectoryHealthChecker) Name() string {
	return "directory"
}

// Check performs the health check
func (d *DirectoryHealthChecker) Check(ctx context.Context) ComponentHealth {
	stats := d.getStats()

	health := ComponentHealth{
		Name:        d.Name(),
		LastChecked: time.Now(),
		Details: map[string]interface{}{
			"last_update":   stats.LastConsensusUpdate.Format(time.RFC3339),
			"consensus_age": stats.ConsensusAge.String(),
			"relay_count":   stats.RelayCount,
			"guard_count":   stats.GuardCount,
			"exit_count":    stats.ExitCount,
		},
	}

	// Consensus should be updated at least every 3 hours
	if stats.ConsensusAge > 3*time.Hour {
		health.Status = StatusUnhealthy
		health.Message = "Directory consensus is stale"
	} else if stats.RelayCount < 100 {
		health.Status = StatusDegraded
		health.Message = "Low relay count in consensus"
	} else {
		health.Status = StatusHealthy
		health.Message = "Directory consensus is current"
	}

	return health
}
