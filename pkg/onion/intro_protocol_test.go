// Package onion - Tests for Introduction Point Protocol
package onion

import (
	"context"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/logger"
)

func TestIntroPointManager_RegisterUnregister(t *testing.T) {
	service := &Service{
		logger: logger.NewDefault(),
	}
	manager := NewIntroPointManager(service, logger.NewDefault())

	// Register an intro point
	manager.RegisterIntroPoint(1001)

	// Check it's registered and healthy
	if !manager.IsHealthy(1001) {
		t.Error("Newly registered intro point should be healthy")
	}

	// Unregister
	manager.UnregisterIntroPoint(1001)

	// Check it's no longer tracked
	if manager.IsHealthy(1001) {
		t.Error("Unregistered intro point should not be tracked")
	}
}

func TestIntroPointManager_RecordSuccess(t *testing.T) {
	service := &Service{
		logger: logger.NewDefault(),
	}
	manager := NewIntroPointManager(service, logger.NewDefault())

	manager.RegisterIntroPoint(1001)

	// Record initial failure
	manager.RecordFailure(1001)

	// Record success should reset consecutive failures
	manager.RecordSuccess(1001)

	if !manager.IsHealthy(1001) {
		t.Error("Intro point should be healthy after success")
	}

	manager.mu.RLock()
	h := manager.health[1001]
	manager.mu.RUnlock()

	if h.ConsecutiveFails != 0 {
		t.Errorf("ConsecutiveFails should be 0 after success, got %d", h.ConsecutiveFails)
	}

	if h.FailureCount != 1 {
		t.Errorf("FailureCount should still be 1 (total), got %d", h.FailureCount)
	}
}

func TestIntroPointManager_RecordFailure(t *testing.T) {
	service := &Service{
		logger: logger.NewDefault(),
	}
	manager := NewIntroPointManager(service, logger.NewDefault())

	manager.RegisterIntroPoint(1001)

	// Should be healthy initially
	if !manager.IsHealthy(1001) {
		t.Error("Should be healthy initially")
	}

	// Record 2 failures - should still be healthy
	manager.RecordFailure(1001)
	manager.RecordFailure(1001)

	if !manager.IsHealthy(1001) {
		t.Error("Should be healthy after 2 consecutive failures")
	}

	// Record 3rd failure - should become unhealthy
	manager.RecordFailure(1001)

	if manager.IsHealthy(1001) {
		t.Error("Should be unhealthy after 3 consecutive failures")
	}

	manager.mu.RLock()
	h := manager.health[1001]
	manager.mu.RUnlock()

	if h.ConsecutiveFails != 3 {
		t.Errorf("ConsecutiveFails should be 3, got %d", h.ConsecutiveFails)
	}

	if h.FailureCount != 3 {
		t.Errorf("FailureCount should be 3, got %d", h.FailureCount)
	}
}

func TestIntroPointManager_GetUnhealthyIntroPoints(t *testing.T) {
	service := &Service{
		logger: logger.NewDefault(),
	}
	manager := NewIntroPointManager(service, logger.NewDefault())

	// Register multiple intro points
	manager.RegisterIntroPoint(1001)
	manager.RegisterIntroPoint(1002)
	manager.RegisterIntroPoint(1003)

	// Mark 1001 and 1003 as unhealthy
	for i := 0; i < 3; i++ {
		manager.RecordFailure(1001)
		manager.RecordFailure(1003)
	}

	unhealthy := manager.GetUnhealthyIntroPoints()

	if len(unhealthy) != 2 {
		t.Errorf("Expected 2 unhealthy intro points, got %d", len(unhealthy))
	}

	// Verify the unhealthy ones are in the list
	unhealthyMap := make(map[uint32]bool)
	for _, id := range unhealthy {
		unhealthyMap[id] = true
	}

	if !unhealthyMap[1001] || !unhealthyMap[1003] {
		t.Error("Unhealthy list should contain 1001 and 1003")
	}

	if unhealthyMap[1002] {
		t.Error("Unhealthy list should not contain 1002")
	}
}

func TestIntroPointManager_GetStaleIntroPoints(t *testing.T) {
	service := &Service{
		logger: logger.NewDefault(),
	}
	manager := NewIntroPointManager(service, logger.NewDefault())

	// Register intro points
	manager.RegisterIntroPoint(1001)
	manager.RegisterIntroPoint(1002)

	// Make 1001 stale by setting LastSuccess to old time
	manager.mu.Lock()
	manager.health[1001].LastSuccess = time.Now().Add(-25 * time.Hour)
	manager.mu.Unlock()

	stale := manager.GetStaleIntroPoints()

	if len(stale) != 1 {
		t.Errorf("Expected 1 stale intro point, got %d", len(stale))
	}

	if len(stale) > 0 && stale[0] != 1001 {
		t.Errorf("Expected stale intro point 1001, got %d", stale[0])
	}
}

func TestIntroPointManager_HealthChecking(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping health checking test in short mode")
	}

	service := &Service{
		logger: logger.NewDefault(),
		config: &ServiceConfig{},
	}
	manager := NewIntroPointManager(service, logger.NewDefault())

	manager.RegisterIntroPoint(1001)

	// Record old last checked time
	manager.mu.Lock()
	oldChecked := manager.health[1001].LastChecked
	manager.mu.Unlock()

	// Manually trigger health check
	manager.performHealthCheck()

	manager.mu.RLock()
	newChecked := manager.health[1001].LastChecked
	manager.mu.RUnlock()

	// LastChecked should have been updated
	if !newChecked.After(oldChecked) {
		t.Errorf("LastChecked should be updated after health check: old=%v new=%v", oldChecked, newChecked)
	}
}

func TestCalculateBackoffDelay(t *testing.T) {
	tests := []struct {
		name     string
		attempt  int
		expected time.Duration
	}{
		{
			name:     "first retry",
			attempt:  0,
			expected: 2 * time.Second,
		},
		{
			name:     "second retry",
			attempt:  1,
			expected: 4 * time.Second,
		},
		{
			name:     "third retry",
			attempt:  2,
			expected: 8 * time.Second,
		},
		{
			name:     "fourth retry",
			attempt:  3,
			expected: 16 * time.Second,
		},
		{
			name:     "max delay capped",
			attempt:  10,
			expected: 30 * time.Second, // capped at max
		},
		{
			name:     "negative attempt",
			attempt:  -1,
			expected: 2 * time.Second, // treated as 0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delay := CalculateBackoffDelay(tt.attempt)
			if delay != tt.expected {
				t.Errorf("CalculateBackoffDelay(%d) = %v, want %v",
					tt.attempt, delay, tt.expected)
			}
		})
	}
}

func TestIntroPointManager_BuildIntroCircuitWithRetry_NoBuilder(t *testing.T) {
	service := &Service{
		logger: logger.NewDefault(),
		config: &ServiceConfig{},
	}
	manager := NewIntroPointManager(service, logger.NewDefault())

	relay := &HSDirectory{
		Fingerprint: "test-relay",
	}

	ctx := context.Background()
	_, err := manager.BuildIntroCircuitWithRetry(ctx, relay)

	if err == nil {
		t.Error("Expected error when circuit builder not configured")
	}

	if err != nil && err.Error() != "failed after 4 attempts: path selector or circuit builder not configured" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestIntroPointManager_BuildIntroCircuitWithRetry_ContextCanceled(t *testing.T) {
	service := &Service{
		logger: logger.NewDefault(),
		config: &ServiceConfig{},
	}
	manager := NewIntroPointManager(service, logger.NewDefault())

	relay := &HSDirectory{
		Fingerprint: "test-relay",
	}

	// Cancel context immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := manager.BuildIntroCircuitWithRetry(ctx, relay)

	if err == nil {
		t.Error("Expected error when context canceled")
	}
}

func TestIntroPointHealth_InitialState(t *testing.T) {
	service := &Service{
		logger: logger.NewDefault(),
	}
	manager := NewIntroPointManager(service, logger.NewDefault())

	circuitID := uint32(2001)
	manager.RegisterIntroPoint(circuitID)

	manager.mu.RLock()
	h := manager.health[circuitID]
	manager.mu.RUnlock()

	if h.CircuitID != circuitID {
		t.Errorf("CircuitID = %d, want %d", h.CircuitID, circuitID)
	}

	if h.FailureCount != 0 {
		t.Errorf("FailureCount = %d, want 0", h.FailureCount)
	}

	if h.ConsecutiveFails != 0 {
		t.Errorf("ConsecutiveFails = %d, want 0", h.ConsecutiveFails)
	}

	if !h.Healthy {
		t.Error("Initial health should be true")
	}

	now := time.Now()
	if h.LastChecked.After(now) || h.LastSuccess.After(now) {
		t.Error("Timestamps should not be in the future")
	}
}

func TestIntroPointManager_MultipleFailuresAndRecovery(t *testing.T) {
	service := &Service{
		logger: logger.NewDefault(),
	}
	manager := NewIntroPointManager(service, logger.NewDefault())

	manager.RegisterIntroPoint(1001)

	// Fail 5 times
	for i := 0; i < 5; i++ {
		manager.RecordFailure(1001)
	}

	manager.mu.RLock()
	h1 := manager.health[1001]
	manager.mu.RUnlock()

	if h1.FailureCount != 5 {
		t.Errorf("FailureCount = %d, want 5", h1.FailureCount)
	}

	if h1.ConsecutiveFails != 5 {
		t.Errorf("ConsecutiveFails = %d, want 5", h1.ConsecutiveFails)
	}

	if h1.Healthy {
		t.Error("Should be unhealthy after 5 failures")
	}

	// One success should recover health
	manager.RecordSuccess(1001)

	manager.mu.RLock()
	h2 := manager.health[1001]
	manager.mu.RUnlock()

	if !h2.Healthy {
		t.Error("Should be healthy after success")
	}

	if h2.ConsecutiveFails != 0 {
		t.Errorf("ConsecutiveFails = %d, want 0 after success", h2.ConsecutiveFails)
	}

	if h2.FailureCount != 5 {
		t.Errorf("FailureCount = %d, want 5 (total count persists)", h2.FailureCount)
	}
}
