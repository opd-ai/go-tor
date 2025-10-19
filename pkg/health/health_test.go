package health

import (
	"context"
	"testing"
	"time"
)

// mockChecker implements Checker for testing
type mockChecker struct {
	name   string
	status Status
	delay  time.Duration
}

func (m *mockChecker) Name() string {
	return m.name
}

func (m *mockChecker) Check(ctx context.Context) ComponentHealth {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	return ComponentHealth{
		Name:        m.name,
		Status:      m.status,
		Message:     "Mock check",
		LastChecked: time.Now(),
	}
}

func TestNewMonitor(t *testing.T) {
	monitor := NewMonitor()
	if monitor == nil {
		t.Fatal("NewMonitor returned nil")
	}
	if monitor.checkers == nil {
		t.Error("checkers map not initialized")
	}
	if monitor.lastChecks == nil {
		t.Error("lastChecks map not initialized")
	}
}

func TestRegisterChecker(t *testing.T) {
	monitor := NewMonitor()
	checker := &mockChecker{name: "test", status: StatusHealthy}

	monitor.RegisterChecker(checker)

	monitor.mu.RLock()
	defer monitor.mu.RUnlock()
	if _, exists := monitor.checkers["test"]; !exists {
		t.Error("Checker not registered")
	}
}

func TestUnregisterChecker(t *testing.T) {
	monitor := NewMonitor()
	checker := &mockChecker{name: "test", status: StatusHealthy}

	monitor.RegisterChecker(checker)
	monitor.UnregisterChecker("test")

	monitor.mu.RLock()
	defer monitor.mu.RUnlock()
	if _, exists := monitor.checkers["test"]; exists {
		t.Error("Checker not unregistered")
	}
}

func TestCheck(t *testing.T) {
	monitor := NewMonitor()
	monitor.RegisterChecker(&mockChecker{name: "component1", status: StatusHealthy})
	monitor.RegisterChecker(&mockChecker{name: "component2", status: StatusHealthy})

	ctx := context.Background()
	result := monitor.Check(ctx)

	if result.Status != StatusHealthy {
		t.Errorf("Expected overall status healthy, got %s", result.Status)
	}
	if len(result.Components) != 2 {
		t.Errorf("Expected 2 components, got %d", len(result.Components))
	}
}

func TestCheckOverallStatus(t *testing.T) {
	tests := []struct {
		name           string
		checkers       []mockChecker
		expectedStatus Status
	}{
		{
			name: "all healthy",
			checkers: []mockChecker{
				{name: "c1", status: StatusHealthy},
				{name: "c2", status: StatusHealthy},
			},
			expectedStatus: StatusHealthy,
		},
		{
			name: "one degraded",
			checkers: []mockChecker{
				{name: "c1", status: StatusHealthy},
				{name: "c2", status: StatusDegraded},
			},
			expectedStatus: StatusDegraded,
		},
		{
			name: "one unhealthy",
			checkers: []mockChecker{
				{name: "c1", status: StatusHealthy},
				{name: "c2", status: StatusUnhealthy},
			},
			expectedStatus: StatusUnhealthy,
		},
		{
			name: "degraded and unhealthy",
			checkers: []mockChecker{
				{name: "c1", status: StatusDegraded},
				{name: "c2", status: StatusUnhealthy},
			},
			expectedStatus: StatusUnhealthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			monitor := NewMonitor()
			for i := range tt.checkers {
				monitor.RegisterChecker(&tt.checkers[i])
			}

			result := monitor.Check(context.Background())
			if result.Status != tt.expectedStatus {
				t.Errorf("Expected status %s, got %s", tt.expectedStatus, result.Status)
			}
		})
	}
}

func TestGetLastCheck(t *testing.T) {
	monitor := NewMonitor()
	monitor.RegisterChecker(&mockChecker{name: "test", status: StatusHealthy})

	// Perform initial check
	ctx := context.Background()
	monitor.Check(ctx)

	// Get last check
	result := monitor.GetLastCheck()
	if len(result.Components) != 1 {
		t.Errorf("Expected 1 component in last check, got %d", len(result.Components))
	}
	if result.Status != StatusHealthy {
		t.Errorf("Expected healthy status, got %s", result.Status)
	}
}

func TestCircuitHealthChecker(t *testing.T) {
	tests := []struct {
		name           string
		stats          CircuitStats
		expectedStatus Status
	}{
		{
			name: "healthy circuits",
			stats: CircuitStats{
				ActiveCircuits: 5,
				MinRequired:    2,
				FailedBuilds:   0,
			},
			expectedStatus: StatusHealthy,
		},
		{
			name: "degraded circuits",
			stats: CircuitStats{
				ActiveCircuits: 1,
				MinRequired:    2,
				FailedBuilds:   2,
			},
			expectedStatus: StatusDegraded,
		},
		{
			name: "unhealthy circuits",
			stats: CircuitStats{
				ActiveCircuits: 0,
				MinRequired:    2,
				FailedBuilds:   5,
			},
			expectedStatus: StatusUnhealthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewCircuitHealthChecker(func() CircuitStats {
				return tt.stats
			})

			result := checker.Check(context.Background())
			if result.Status != tt.expectedStatus {
				t.Errorf("Expected status %s, got %s", tt.expectedStatus, result.Status)
			}
			if result.Name != "circuits" {
				t.Errorf("Expected name 'circuits', got %s", result.Name)
			}
		})
	}
}

func TestConnectionHealthChecker(t *testing.T) {
	tests := []struct {
		name           string
		stats          ConnectionStats
		expectedStatus Status
	}{
		{
			name: "healthy connections",
			stats: ConnectionStats{
				TotalConnections:   10,
				OpenConnections:    8,
				FailedConnections:  2,
				ConnectionAttempts: 10,
			},
			expectedStatus: StatusHealthy,
		},
		{
			name: "degraded connections",
			stats: ConnectionStats{
				TotalConnections:   10,
				OpenConnections:    3,
				FailedConnections:  7,
				ConnectionAttempts: 10,
			},
			expectedStatus: StatusDegraded,
		},
		{
			name: "unhealthy connections",
			stats: ConnectionStats{
				TotalConnections:   5,
				OpenConnections:    0,
				FailedConnections:  5,
				ConnectionAttempts: 5,
			},
			expectedStatus: StatusUnhealthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewConnectionHealthChecker(func() ConnectionStats {
				return tt.stats
			})

			result := checker.Check(context.Background())
			if result.Status != tt.expectedStatus {
				t.Errorf("Expected status %s, got %s", tt.expectedStatus, result.Status)
			}
			if result.Name != "connections" {
				t.Errorf("Expected name 'connections', got %s", result.Name)
			}
		})
	}
}

func TestDirectoryHealthChecker(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name           string
		stats          DirectoryStats
		expectedStatus Status
	}{
		{
			name: "healthy directory",
			stats: DirectoryStats{
				LastConsensusUpdate: now.Add(-1 * time.Hour),
				ConsensusAge:        1 * time.Hour,
				RelayCount:          1000,
				GuardCount:          100,
				ExitCount:           200,
			},
			expectedStatus: StatusHealthy,
		},
		{
			name: "degraded directory - low relay count",
			stats: DirectoryStats{
				LastConsensusUpdate: now.Add(-1 * time.Hour),
				ConsensusAge:        1 * time.Hour,
				RelayCount:          50,
				GuardCount:          10,
				ExitCount:           10,
			},
			expectedStatus: StatusDegraded,
		},
		{
			name: "unhealthy directory - stale consensus",
			stats: DirectoryStats{
				LastConsensusUpdate: now.Add(-4 * time.Hour),
				ConsensusAge:        4 * time.Hour,
				RelayCount:          1000,
				GuardCount:          100,
				ExitCount:           200,
			},
			expectedStatus: StatusUnhealthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewDirectoryHealthChecker(func() DirectoryStats {
				return tt.stats
			})

			result := checker.Check(context.Background())
			if result.Status != tt.expectedStatus {
				t.Errorf("Expected status %s, got %s", tt.expectedStatus, result.Status)
			}
			if result.Name != "directory" {
				t.Errorf("Expected name 'directory', got %s", result.Name)
			}
		})
	}
}

func TestCheckResponseTime(t *testing.T) {
	monitor := NewMonitor()
	// Add a checker with artificial delay
	monitor.RegisterChecker(&mockChecker{
		name:   "slow",
		status: StatusHealthy,
		delay:  50 * time.Millisecond,
	})

	result := monitor.Check(context.Background())
	slowHealth := result.Components["slow"]

	if slowHealth.ResponseTimeMs < 50 {
		t.Errorf("Expected response time >= 50ms, got %dms", slowHealth.ResponseTimeMs)
	}
}
