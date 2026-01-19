package health

import (
	"context"
	"testing"
	"time"
)

// mockProbeChecker implements ProbeChecker for testing
type mockProbeChecker struct {
	name        string
	status      Status
	livenessOK  bool
	readinessOK bool
	checkDelay  time.Duration
}

func (m *mockProbeChecker) Name() string {
	return m.name
}

func (m *mockProbeChecker) Check(ctx context.Context) ComponentHealth {
	if m.checkDelay > 0 {
		time.Sleep(m.checkDelay)
	}
	return ComponentHealth{
		Name:        m.name,
		Status:      m.status,
		Message:     "Mock check",
		LastChecked: time.Now(),
	}
}

func (m *mockProbeChecker) CheckLiveness(ctx context.Context) bool {
	return m.livenessOK
}

func (m *mockProbeChecker) CheckReadiness(ctx context.Context) bool {
	return m.readinessOK
}

func TestDefaultCacheConfig(t *testing.T) {
	config := DefaultCacheConfig()

	if !config.Enabled {
		t.Error("Expected caching to be enabled by default")
	}
	if config.TTL != 5*time.Second {
		t.Errorf("Expected TTL 5s, got %v", config.TTL)
	}
	if config.LivenessTTL != 1*time.Second {
		t.Errorf("Expected LivenessTTL 1s, got %v", config.LivenessTTL)
	}
	if config.ReadinessTTL != 3*time.Second {
		t.Errorf("Expected ReadinessTTL 3s, got %v", config.ReadinessTTL)
	}
}

func TestNewCachedMonitor(t *testing.T) {
	config := DefaultCacheConfig()
	monitor := NewCachedMonitor(config)

	if monitor == nil {
		t.Fatal("NewCachedMonitor returned nil")
	}
	if monitor.Monitor == nil {
		t.Error("Embedded Monitor is nil")
	}
	if !monitor.cacheConfig.Enabled {
		t.Error("Cache config not properly set")
	}
}

func TestCachedMonitorCheck(t *testing.T) {
	config := DefaultCacheConfig()
	config.TTL = 100 * time.Millisecond
	monitor := NewCachedMonitor(config)

	checker := &mockProbeChecker{
		name:   "test",
		status: StatusHealthy,
	}
	monitor.RegisterChecker(checker)

	// First check should execute
	ctx := context.Background()
	result1 := monitor.Check(ctx)
	if result1.Status != StatusHealthy {
		t.Errorf("Expected healthy status, got %s", result1.Status)
	}

	// Second check should use cache (within TTL)
	result2 := monitor.Check(ctx)
	if result2.Status != StatusHealthy {
		t.Errorf("Expected healthy status from cache, got %s", result2.Status)
	}

	// Wait for cache to expire
	time.Sleep(150 * time.Millisecond)

	// Third check should execute fresh
	result3 := monitor.Check(ctx)
	if result3.Status != StatusHealthy {
		t.Errorf("Expected healthy status after cache expiry, got %s", result3.Status)
	}
}

func TestCachedMonitorCheckCacheDisabled(t *testing.T) {
	config := CacheConfig{
		Enabled: false,
		TTL:     time.Hour, // Even with long TTL, should not cache
	}
	monitor := NewCachedMonitor(config)

	checker := &mockChecker{
		name:   "counter",
		status: StatusHealthy,
	}
	monitor.RegisterChecker(checker)

	// Checks should not be cached - verify by running multiple checks
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		result := monitor.Check(ctx)
		if result.Status != StatusHealthy {
			t.Errorf("Check %d: expected healthy, got %s", i+1, result.Status)
		}
	}
}

func TestCachedMonitorCheckLiveness(t *testing.T) {
	config := DefaultCacheConfig()
	config.LivenessTTL = 100 * time.Millisecond
	monitor := NewCachedMonitor(config)

	checker := &mockProbeChecker{
		name:        "liveness-test",
		status:      StatusHealthy,
		livenessOK:  true,
		readinessOK: true,
	}
	monitor.RegisterChecker(checker)

	ctx := context.Background()
	result := monitor.CheckLiveness(ctx)

	if result.Type != ProbeLiveness {
		t.Errorf("Expected liveness probe type, got %s", result.Type)
	}
	if !result.Healthy {
		t.Error("Expected healthy liveness result")
	}
	if result.Duration < 0 {
		t.Error("Expected non-negative duration")
	}
}

func TestCachedMonitorCheckLivenessFailure(t *testing.T) {
	config := DefaultCacheConfig()
	monitor := NewCachedMonitor(config)

	checker := &mockProbeChecker{
		name:        "failing",
		status:      StatusHealthy,
		livenessOK:  false, // Liveness fails
		readinessOK: true,
	}
	monitor.RegisterChecker(checker)

	ctx := context.Background()
	result := monitor.CheckLiveness(ctx)

	if result.Healthy {
		t.Error("Expected unhealthy liveness result")
	}
	if result.Message == "" {
		t.Error("Expected failure message")
	}
}

func TestCachedMonitorCheckReadiness(t *testing.T) {
	config := DefaultCacheConfig()
	config.ReadinessTTL = 100 * time.Millisecond
	monitor := NewCachedMonitor(config)

	checker := &mockProbeChecker{
		name:        "readiness-test",
		status:      StatusHealthy,
		livenessOK:  true,
		readinessOK: true,
	}
	monitor.RegisterChecker(checker)

	ctx := context.Background()
	result := monitor.CheckReadiness(ctx)

	if result.Type != ProbeReadiness {
		t.Errorf("Expected readiness probe type, got %s", result.Type)
	}
	if !result.Healthy {
		t.Error("Expected healthy readiness result")
	}
}

func TestCachedMonitorCheckReadinessFailure(t *testing.T) {
	config := DefaultCacheConfig()
	monitor := NewCachedMonitor(config)

	checker := &mockProbeChecker{
		name:        "not-ready",
		status:      StatusHealthy,
		livenessOK:  true,
		readinessOK: false, // Readiness fails
	}
	monitor.RegisterChecker(checker)

	ctx := context.Background()
	result := monitor.CheckReadiness(ctx)

	if result.Healthy {
		t.Error("Expected not-ready result")
	}
}

func TestCachedMonitorLivenessCaching(t *testing.T) {
	config := DefaultCacheConfig()
	config.LivenessTTL = 100 * time.Millisecond
	monitor := NewCachedMonitor(config)

	checker := &mockProbeChecker{
		name:        "cache-test",
		status:      StatusHealthy,
		livenessOK:  true,
		readinessOK: true,
	}
	monitor.RegisterChecker(checker)

	ctx := context.Background()

	// First check
	result1 := monitor.CheckLiveness(ctx)
	if !result1.Healthy {
		t.Error("First check should be healthy")
	}

	// Second check (should use cache)
	result2 := monitor.CheckLiveness(ctx)
	if !result2.Healthy {
		t.Error("Cached check should be healthy")
	}

	// Wait for cache expiry
	time.Sleep(150 * time.Millisecond)

	// Third check (cache expired)
	result3 := monitor.CheckLiveness(ctx)
	if !result3.Healthy {
		t.Error("Fresh check should be healthy")
	}
}

func TestCachedMonitorInvalidateCache(t *testing.T) {
	config := DefaultCacheConfig()
	config.TTL = time.Hour // Long TTL
	monitor := NewCachedMonitor(config)

	checker := &mockProbeChecker{
		name:        "invalidate-test",
		status:      StatusHealthy,
		livenessOK:  true,
		readinessOK: true,
	}
	monitor.RegisterChecker(checker)

	ctx := context.Background()

	// Perform checks to populate cache
	monitor.Check(ctx)
	monitor.CheckLiveness(ctx)
	monitor.CheckReadiness(ctx)

	// Invalidate cache
	monitor.InvalidateCache()

	// Verify caches are empty
	monitor.mu.RLock()
	if len(monitor.cache) != 0 {
		t.Error("Cache should be empty after invalidation")
	}
	if len(monitor.livenessCache) != 0 {
		t.Error("Liveness cache should be empty after invalidation")
	}
	if len(monitor.readinessCache) != 0 {
		t.Error("Readiness cache should be empty after invalidation")
	}
	monitor.mu.RUnlock()
}

func TestCachedMonitorInvalidateCacheFor(t *testing.T) {
	config := DefaultCacheConfig()
	config.TTL = time.Hour
	monitor := NewCachedMonitor(config)

	checker1 := &mockProbeChecker{name: "comp1", status: StatusHealthy, livenessOK: true, readinessOK: true}
	checker2 := &mockProbeChecker{name: "comp2", status: StatusHealthy, livenessOK: true, readinessOK: true}
	monitor.RegisterChecker(checker1)
	monitor.RegisterChecker(checker2)

	ctx := context.Background()

	// Populate caches
	monitor.Check(ctx)
	monitor.CheckLiveness(ctx)
	monitor.CheckReadiness(ctx)

	// Invalidate only comp1
	monitor.InvalidateCacheFor("comp1")

	// Verify comp1 is invalidated but comp2 is still cached
	monitor.mu.RLock()
	if _, exists := monitor.cache["comp1"]; exists {
		t.Error("comp1 should be invalidated")
	}
	if _, exists := monitor.cache["comp2"]; !exists {
		t.Error("comp2 should still be cached")
	}
	monitor.mu.RUnlock()
}

func TestCachedMonitorSetCacheConfig(t *testing.T) {
	config := DefaultCacheConfig()
	monitor := NewCachedMonitor(config)

	newConfig := CacheConfig{
		Enabled: false,
		TTL:     time.Minute,
	}
	monitor.SetCacheConfig(newConfig)

	result := monitor.GetCacheConfig()
	if result.Enabled {
		t.Error("Expected disabled caching")
	}
	if result.TTL != time.Minute {
		t.Errorf("Expected TTL 1m, got %v", result.TTL)
	}
}

func TestProbeTypeConstants(t *testing.T) {
	if ProbeLiveness != "liveness" {
		t.Errorf("Unexpected ProbeLiveness value: %s", ProbeLiveness)
	}
	if ProbeReadiness != "readiness" {
		t.Errorf("Unexpected ProbeReadiness value: %s", ProbeReadiness)
	}
	if ProbeStartup != "startup" {
		t.Errorf("Unexpected ProbeStartup value: %s", ProbeStartup)
	}
}

// Test that regular checkers (not implementing probe interfaces) still work
func TestCachedMonitorRegularCheckerFallback(t *testing.T) {
	config := DefaultCacheConfig()
	monitor := NewCachedMonitor(config)

	// Regular checker (not a ProbeChecker)
	regularChecker := &mockChecker{
		name:   "regular",
		status: StatusHealthy,
	}
	monitor.RegisterChecker(regularChecker)

	ctx := context.Background()

	// Liveness should fall back to regular health check
	livenessResult := monitor.CheckLiveness(ctx)
	if !livenessResult.Healthy {
		t.Error("Regular healthy checker should pass liveness")
	}

	// Readiness should fall back to regular health check
	readinessResult := monitor.CheckReadiness(ctx)
	if !readinessResult.Healthy {
		t.Error("Regular healthy checker should pass readiness")
	}
}

func TestCachedMonitorUnhealthyRegularCheckerFallback(t *testing.T) {
	config := DefaultCacheConfig()
	monitor := NewCachedMonitor(config)

	// Regular unhealthy checker
	unhealthyChecker := &mockChecker{
		name:   "unhealthy",
		status: StatusUnhealthy,
	}
	monitor.RegisterChecker(unhealthyChecker)

	ctx := context.Background()

	// Liveness should fail for unhealthy component
	livenessResult := monitor.CheckLiveness(ctx)
	if livenessResult.Healthy {
		t.Error("Unhealthy checker should fail liveness")
	}

	// Readiness should also fail for unhealthy component
	readinessResult := monitor.CheckReadiness(ctx)
	if readinessResult.Healthy {
		t.Error("Unhealthy checker should fail readiness")
	}
}

func TestCachedMonitorDegradedRegularCheckerFallback(t *testing.T) {
	config := DefaultCacheConfig()
	monitor := NewCachedMonitor(config)

	// Regular degraded checker
	degradedChecker := &mockChecker{
		name:   "degraded",
		status: StatusDegraded,
	}
	monitor.RegisterChecker(degradedChecker)

	ctx := context.Background()

	// Liveness should pass for degraded (it's still alive)
	livenessResult := monitor.CheckLiveness(ctx)
	if !livenessResult.Healthy {
		t.Error("Degraded checker should pass liveness")
	}

	// Readiness should pass for degraded (can still serve traffic)
	readinessResult := monitor.CheckReadiness(ctx)
	if !readinessResult.Healthy {
		t.Error("Degraded checker should pass readiness")
	}
}

func TestProbeResult(t *testing.T) {
	result := ProbeResult{
		Type:      ProbeLiveness,
		Healthy:   true,
		Message:   "test message",
		Timestamp: time.Now(),
		Duration:  42,
	}

	if result.Type != ProbeLiveness {
		t.Errorf("Unexpected type: %s", result.Type)
	}
	if !result.Healthy {
		t.Error("Expected healthy")
	}
	if result.Message != "test message" {
		t.Errorf("Unexpected message: %s", result.Message)
	}
	if result.Duration != 42 {
		t.Errorf("Unexpected duration: %d", result.Duration)
	}
}
