// Package onion - Introduction Point Protocol Implementation
// Implements circuit retry logic, health monitoring, and rotation per rend-spec-v3.txt §3.1
package onion

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/opd-ai/go-tor/pkg/circuit"
	"github.com/opd-ai/go-tor/pkg/logger"
)

const (
	// Default retry parameters
	defaultMaxRetries     = 3
	defaultBaseRetryDelay = 2 * time.Second
	defaultMaxRetryDelay  = 30 * time.Second

	// Health check parameters
	defaultHealthCheckInterval = 30 * time.Second
	defaultRotationInterval    = 24 * time.Hour
	defaultCircuitTimeout      = 30 * time.Second
)

// IntroPointHealth tracks the health status of an introduction point
type IntroPointHealth struct {
	mu sync.RWMutex

	CircuitID        uint32
	LastChecked      time.Time
	LastSuccess      time.Time
	FailureCount     int
	ConsecutiveFails int
	Healthy          bool
}

// IntroPointManager manages introduction point lifecycle
type IntroPointManager struct {
	mu sync.RWMutex

	service         *Service
	health          map[uint32]*IntroPointHealth // circuit ID -> health
	healthCheckStop chan struct{}
	logger          *logger.Logger
}

// NewIntroPointManager creates a new introduction point manager
func NewIntroPointManager(service *Service, log *logger.Logger) *IntroPointManager {
	return &IntroPointManager{
		service:         service,
		health:          make(map[uint32]*IntroPointHealth),
		healthCheckStop: make(chan struct{}),
		logger:          log.Component("intro-manager"),
	}
}

// BuildIntroCircuitWithRetry builds an introduction point circuit with exponential backoff retry
func (m *IntroPointManager) BuildIntroCircuitWithRetry(ctx context.Context, relay *HSDirectory) (*circuit.Circuit, error) {
	var lastErr error
	delay := defaultBaseRetryDelay

	for attempt := 0; attempt <= defaultMaxRetries; attempt++ {
		if attempt > 0 {
			m.logger.Debug("Retrying circuit build",
				"attempt", attempt,
				"delay", delay,
				"relay", relay.Fingerprint)

			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}

			// Exponential backoff with jitter
			delay = time.Duration(float64(delay) * 2.0)
			if delay > defaultMaxRetryDelay {
				delay = defaultMaxRetryDelay
			}
		}

		circ, err := m.buildIntroCircuit(ctx, relay)
		if err != nil {
			lastErr = err
			m.logger.Warn("Circuit build failed",
				"attempt", attempt+1,
				"relay", relay.Fingerprint,
				"error", err)
			continue
		}

		m.logger.Info("Circuit build succeeded",
			"attempts", attempt+1,
			"relay", relay.Fingerprint,
			"circuit", circ.ID)
		return circ, nil
	}

	return nil, fmt.Errorf("failed after %d attempts: %w", defaultMaxRetries+1, lastErr)
}

// buildIntroCircuit builds a single introduction point circuit (internal helper)
func (m *IntroPointManager) buildIntroCircuit(ctx context.Context, relay *HSDirectory) (*circuit.Circuit, error) {
	if m.service.config.PathSelector == nil || m.service.config.CircuitBuilder == nil {
		return nil, fmt.Errorf("path selector or circuit builder not configured")
	}

	selectedPath, err := m.service.config.PathSelector.SelectPath(0)
	if err != nil {
		return nil, fmt.Errorf("failed to select path: %w", err)
	}

	buildCtx, cancel := context.WithTimeout(ctx, defaultCircuitTimeout)
	defer cancel()

	circ, err := m.service.config.CircuitBuilder.BuildCircuit(buildCtx, selectedPath, defaultCircuitTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to build circuit: %w", err)
	}

	return circ, nil
}

// RegisterIntroPoint registers an introduction point for health monitoring
func (m *IntroPointManager) RegisterIntroPoint(circuitID uint32) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	m.health[circuitID] = &IntroPointHealth{
		CircuitID:        circuitID,
		LastChecked:      now,
		LastSuccess:      now,
		FailureCount:     0,
		ConsecutiveFails: 0,
		Healthy:          true,
	}

	m.logger.Debug("Registered introduction point for monitoring",
		"circuit", circuitID)
}

// UnregisterIntroPoint removes an introduction point from health monitoring
func (m *IntroPointManager) UnregisterIntroPoint(circuitID uint32) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.health, circuitID)
	m.logger.Debug("Unregistered introduction point",
		"circuit", circuitID)
}

// RecordSuccess records a successful interaction with an introduction point
func (m *IntroPointManager) RecordSuccess(circuitID uint32) {
	m.mu.Lock()
	defer m.mu.Unlock()

	h, ok := m.health[circuitID]
	if !ok {
		return
	}

	now := time.Now()
	h.LastSuccess = now
	h.LastChecked = now
	h.ConsecutiveFails = 0
	h.Healthy = true
}

// RecordFailure records a failure with an introduction point
func (m *IntroPointManager) RecordFailure(circuitID uint32) {
	m.mu.Lock()
	defer m.mu.Unlock()

	h, ok := m.health[circuitID]
	if !ok {
		return
	}

	h.LastChecked = time.Now()
	h.FailureCount++
	h.ConsecutiveFails++

	// Mark unhealthy after 3 consecutive failures
	if h.ConsecutiveFails >= 3 {
		h.Healthy = false
		m.logger.Warn("Introduction point marked unhealthy",
			"circuit", circuitID,
			"consecutive_failures", h.ConsecutiveFails)
	}
}

// IsHealthy checks if an introduction point is healthy
func (m *IntroPointManager) IsHealthy(circuitID uint32) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	h, ok := m.health[circuitID]
	if !ok {
		return false
	}

	return h.Healthy
}

// GetUnhealthyIntroPoints returns list of unhealthy introduction point circuit IDs
func (m *IntroPointManager) GetUnhealthyIntroPoints() []uint32 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var unhealthy []uint32
	for circuitID, h := range m.health {
		if !h.Healthy {
			unhealthy = append(unhealthy, circuitID)
		}
	}

	return unhealthy
}

// GetStaleIntroPoints returns introduction points that need rotation (older than 24h)
func (m *IntroPointManager) GetStaleIntroPoints() []uint32 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var stale []uint32
	now := time.Now()

	for circuitID, h := range m.health {
		if now.Sub(h.LastSuccess) > defaultRotationInterval {
			stale = append(stale, circuitID)
		}
	}

	return stale
}

// StartHealthChecking starts background health checking
func (m *IntroPointManager) StartHealthChecking(ctx context.Context) {
	ticker := time.NewTicker(defaultHealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.healthCheckStop:
			return
		case <-ticker.C:
			m.performHealthCheck()
		}
	}
}

// StopHealthChecking stops background health checking
func (m *IntroPointManager) StopHealthChecking() {
	close(m.healthCheckStop)
}

// performHealthCheck checks health of all introduction points
func (m *IntroPointManager) performHealthCheck() {
	m.mu.Lock()
	circuitIDs := make([]uint32, 0, len(m.health))
	for id := range m.health {
		circuitIDs = append(circuitIDs, id)
	}

	// Update last checked time for all circuits
	now := time.Now()
	for _, id := range circuitIDs {
		if h, ok := m.health[id]; ok {
			h.LastChecked = now
		}
	}
	m.mu.Unlock()

	// Check for unhealthy or stale intro points
	unhealthy := m.GetUnhealthyIntroPoints()
	stale := m.GetStaleIntroPoints()

	if len(unhealthy) > 0 {
		m.logger.Warn("Found unhealthy introduction points",
			"count", len(unhealthy),
			"circuits", unhealthy)
	}

	if len(stale) > 0 {
		m.logger.Info("Found stale introduction points needing rotation",
			"count", len(stale),
			"circuits", stale)
	}
}

// CalculateBackoffDelay calculates exponential backoff delay
func CalculateBackoffDelay(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}

	delay := defaultBaseRetryDelay * time.Duration(math.Pow(2, float64(attempt)))
	if delay > defaultMaxRetryDelay {
		return defaultMaxRetryDelay
	}

	return delay
}
