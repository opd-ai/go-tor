package health

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// stateChecker is a checker whose status can be changed atomically.
type stateChecker struct {
	name      string
	statusPtr atomic.Value
}

func newStateChecker(name string, s Status) *stateChecker {
	sc := &stateChecker{name: name}
	sc.statusPtr.Store(s)
	return sc
}

func (s *stateChecker) Name() string { return s.name }

func (s *stateChecker) Check(_ context.Context) ComponentHealth {
	return ComponentHealth{
		Name:        s.name,
		Status:      s.statusPtr.Load().(Status),
		LastChecked: time.Now(),
	}
}

func (s *stateChecker) SetStatus(st Status) { s.statusPtr.Store(st) }

func TestStateTransition_FullCycle(t *testing.T) {
	m := NewMonitor()
	sc := newStateChecker("comp", StatusHealthy)
	m.RegisterChecker(sc)
	ctx := context.Background()

	transitions := []Status{
		StatusHealthy, StatusDegraded, StatusUnhealthy, StatusHealthy,
	}
	for _, expected := range transitions {
		sc.SetStatus(expected)
		got := m.Check(ctx)
		if got.Status != expected {
			t.Fatalf("expected %s, got %s", expected, got.Status)
		}
	}
}

func TestStateTransition_WorstStatusWins(t *testing.T) {
	tests := []struct {
		name     string
		statuses []Status
		want     Status
	}{
		{"all healthy", []Status{StatusHealthy, StatusHealthy}, StatusHealthy},
		{"one degraded", []Status{StatusHealthy, StatusDegraded}, StatusDegraded},
		{"degraded and unhealthy", []Status{StatusDegraded, StatusUnhealthy}, StatusUnhealthy},
		{"healthy and unhealthy", []Status{StatusHealthy, StatusUnhealthy}, StatusUnhealthy},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMonitor()
			for i, s := range tt.statuses {
				m.RegisterChecker(newStateChecker(
					tt.name+string(rune('A'+i)), s,
				))
			}
			got := m.Check(context.Background())
			if got.Status != tt.want {
				t.Errorf("got %s, want %s", got.Status, tt.want)
			}
		})
	}
}

func TestStateTransition_NoCheckers(t *testing.T) {
	m := NewMonitor()
	got := m.Check(context.Background())
	if got.Status != StatusHealthy {
		t.Errorf("expected healthy with no checkers, got %s", got.Status)
	}
	if len(got.Components) != 0 {
		t.Errorf("expected 0 components, got %d", len(got.Components))
	}
}

func TestStateTransition_ConcurrentRegisterUnregister(t *testing.T) {
	m := NewMonitor()
	ctx := context.Background()
	sc := newStateChecker("stable", StatusHealthy)
	m.RegisterChecker(sc)

	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			m.RegisterChecker(newStateChecker("transient", StatusDegraded))
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			m.UnregisterChecker("transient")
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			m.Check(ctx)
		}
	}()
	wg.Wait()
}

func TestStateTransition_ConcurrentChecks(t *testing.T) {
	m := NewMonitor()
	sc := newStateChecker("svc", StatusHealthy)
	m.RegisterChecker(sc)
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				result := m.Check(ctx)
				if result.Status != StatusHealthy {
					t.Errorf("unexpected status %s", result.Status)
				}
			}
		}()
	}
	wg.Wait()
}

func TestStateTransition_StatusStringRepresentations(t *testing.T) {
	tests := []struct {
		status Status
		want   string
	}{
		{StatusHealthy, "healthy"},
		{StatusDegraded, "degraded"},
		{StatusUnhealthy, "unhealthy"},
	}
	for _, tt := range tests {
		if string(tt.status) != tt.want {
			t.Errorf("Status %v != %q", tt.status, tt.want)
		}
	}
}

func TestStateTransition_GetLastCheckBeforeAnyChecks(t *testing.T) {
	m := NewMonitor()
	got := m.GetLastCheck()
	if got.Status != StatusHealthy {
		t.Errorf("expected healthy, got %s", got.Status)
	}
	if len(got.Components) != 0 {
		t.Errorf("expected empty components, got %d", len(got.Components))
	}
	if got.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

func TestStateTransition_CheckerInterfaceSatisfaction(t *testing.T) {
	// Verify all built-in types implement Checker.
	var _ Checker = &CircuitHealthChecker{}
	var _ Checker = &ConnectionHealthChecker{}
	var _ Checker = &DirectoryHealthChecker{}
	var _ Checker = &MemoryHealthChecker{}
}

func TestStateTransition_CachedMonitorExpiration(t *testing.T) {
	var callCount atomic.Int32
	cfg := CacheConfig{Enabled: true, TTL: 20 * time.Millisecond}
	cm := NewCachedMonitor(cfg)

	checker := &countingChecker{
		name:  "cached",
		count: &callCount,
	}
	cm.RegisterChecker(checker)
	ctx := context.Background()

	cm.Check(ctx)
	if callCount.Load() != 1 {
		t.Fatalf("expected 1 call, got %d", callCount.Load())
	}

	// Within TTL - should use cache
	cm.Check(ctx)
	if callCount.Load() != 1 {
		t.Fatalf("expected cached (1 call), got %d", callCount.Load())
	}

	// Wait for expiration
	time.Sleep(25 * time.Millisecond)
	cm.Check(ctx)
	if callCount.Load() != 2 {
		t.Fatalf("expected 2 calls after expiry, got %d", callCount.Load())
	}
}

func TestStateTransition_CachedMonitorInvalidateThenCheck(t *testing.T) {
	var callCount atomic.Int32
	cfg := CacheConfig{Enabled: true, TTL: 5 * time.Second}
	cm := NewCachedMonitor(cfg)

	checker := &countingChecker{name: "inv", count: &callCount}
	cm.RegisterChecker(checker)
	ctx := context.Background()

	cm.Check(ctx)
	if callCount.Load() != 1 {
		t.Fatalf("expected 1, got %d", callCount.Load())
	}

	cm.InvalidateCache()
	cm.Check(ctx)
	if callCount.Load() != 2 {
		t.Fatalf("expected 2 after invalidate, got %d", callCount.Load())
	}
}

func TestStateTransition_DependencyCascadeRequired(t *testing.T) {
	dm := NewDependencyAwareMonitor()
	compA := newStateChecker("compA", StatusHealthy)
	compB := newStateChecker("compB", StatusUnhealthy)
	dm.Monitor.RegisterChecker(compA)
	dm.Monitor.RegisterChecker(compB)
	dm.RegisterDependency("compA", Dependency{
		Name: "compB", Relation: DependencyRequired,
	})

	result := dm.CheckWithDependencies(context.Background())
	a := result["compA"]
	if a.Status != StatusUnhealthy {
		t.Errorf("expected compA unhealthy due to dep, got %s", a.Status)
	}
}

func TestStateTransition_DependencyCascadeOptionalDegraded(t *testing.T) {
	dm := NewDependencyAwareMonitor()
	compA := newStateChecker("compA", StatusHealthy)
	compB := newStateChecker("compB", StatusUnhealthy)
	dm.Monitor.RegisterChecker(compA)
	dm.Monitor.RegisterChecker(compB)
	dm.RegisterDependency("compA", Dependency{
		Name: "compB", Relation: DependencyOptional,
	})

	result := dm.CheckWithDependencies(context.Background())
	a := result["compA"]
	// Optional unhealthy dep should NOT make parent unhealthy
	if a.Status == StatusUnhealthy {
		t.Errorf("optional dep should not make parent unhealthy")
	}
}

func TestStateTransition_MemoryPressureLevelCycle(t *testing.T) {
	thresholds := MemoryThresholds{
		HighWaterMark: 100,
		CriticalMark:  200,
		MaxGoroutines: 10000,
	}
	var currentAlloc atomic.Uint64
	currentAlloc.Store(10)

	getStats := func() MemoryStats {
		return MemoryStats{
			HeapAlloc: currentAlloc.Load(),
			HeapSys:   1000,
		}
	}

	var mu sync.Mutex
	var transitions []MemoryPressureLevel

	mon := &MemoryMonitor{
		thresholds: thresholds,
		getStats:   getStats,
		callbacks:  make([]MemoryPressureCallback, 0),
		interval:   10 * time.Millisecond,
		stopCh:     make(chan struct{}),
		lastLevel:  MemoryPressureNone,
	}
	mon.OnPressure(func(level MemoryPressureLevel, _ MemoryStats) {
		mu.Lock()
		transitions = append(transitions, level)
		mu.Unlock()
	})

	ctx, cancel := context.WithCancel(context.Background())
	go mon.Start(ctx)

	// Normal → High
	time.Sleep(15 * time.Millisecond)
	currentAlloc.Store(150)
	time.Sleep(25 * time.Millisecond)

	// High → Critical
	currentAlloc.Store(250)
	time.Sleep(25 * time.Millisecond)

	// Critical → Normal
	currentAlloc.Store(10)
	time.Sleep(25 * time.Millisecond)

	cancel()
	time.Sleep(15 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(transitions) < 3 {
		t.Fatalf("expected >=3 transitions, got %d: %v", len(transitions), transitions)
	}
}

func TestStateTransition_MemoryCheckerLevelTransitions(t *testing.T) {
	thresholds := MemoryThresholds{
		HighWaterMark:   100,
		CriticalMark:    200,
		MaxGoroutines:   10000,
		TargetHeapRatio: 0.9,
	}

	tests := []struct {
		name      string
		heapAlloc uint64
		wantStat  Status
	}{
		{"normal", 50, StatusHealthy},
		{"high", 150, StatusDegraded},
		{"critical", 250, StatusUnhealthy},
		{"back to normal", 50, StatusHealthy},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewMemoryHealthCheckerWithStatsFunc(thresholds, func() MemoryStats {
				return MemoryStats{HeapAlloc: tt.heapAlloc, HeapSys: 1000}
			})
			got := checker.Check(context.Background())
			if got.Status != tt.wantStat {
				t.Errorf("got %s, want %s", got.Status, tt.wantStat)
			}
		})
	}
}

func TestStateTransition_MemoryPressureLevelStrings(t *testing.T) {
	tests := []struct {
		level MemoryPressureLevel
		want  string
	}{
		{MemoryPressureNone, "none"},
		{MemoryPressureModerate, "moderate"},
		{MemoryPressureHigh, "high"},
		{MemoryPressureCritical, "critical"},
		{MemoryPressureLevel(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.level.String(); got != tt.want {
			t.Errorf("level %d: got %q, want %q", tt.level, got, tt.want)
		}
	}
}

// countingChecker tracks invocation count for cache tests.
type countingChecker struct {
	name  string
	count *atomic.Int32
}

func (c *countingChecker) Name() string { return c.name }

func (c *countingChecker) Check(_ context.Context) ComponentHealth {
	c.count.Add(1)
	return ComponentHealth{
		Name:        c.name,
		Status:      StatusHealthy,
		LastChecked: time.Now(),
	}
}
