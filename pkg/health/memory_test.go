package health

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestDefaultMemoryThresholds(t *testing.T) {
	thresholds := DefaultMemoryThresholds()

	if thresholds.HighWaterMark != 100*1024*1024 {
		t.Errorf("Expected HighWaterMark to be 100MB, got %d", thresholds.HighWaterMark)
	}
	if thresholds.CriticalMark != 200*1024*1024 {
		t.Errorf("Expected CriticalMark to be 200MB, got %d", thresholds.CriticalMark)
	}
	if thresholds.MaxGoroutines != 10000 {
		t.Errorf("Expected MaxGoroutines to be 10000, got %d", thresholds.MaxGoroutines)
	}
	if thresholds.TargetHeapRatio != 0.9 {
		t.Errorf("Expected TargetHeapRatio to be 0.9, got %f", thresholds.TargetHeapRatio)
	}
}

func TestNewMemoryHealthChecker(t *testing.T) {
	thresholds := DefaultMemoryThresholds()
	checker := NewMemoryHealthChecker(thresholds)

	if checker == nil {
		t.Fatal("NewMemoryHealthChecker returned nil")
	}
	if checker.thresholds.HighWaterMark != thresholds.HighWaterMark {
		t.Error("Thresholds not properly set")
	}
}

func TestNewMemoryHealthCheckerWithDefaults(t *testing.T) {
	checker := NewMemoryHealthCheckerWithDefaults()

	if checker == nil {
		t.Fatal("NewMemoryHealthCheckerWithDefaults returned nil")
	}
	if checker.Name() != "memory" {
		t.Errorf("Expected name 'memory', got '%s'", checker.Name())
	}
}

func TestMemoryHealthCheckerName(t *testing.T) {
	checker := NewMemoryHealthCheckerWithDefaults()
	if checker.Name() != "memory" {
		t.Errorf("Expected name 'memory', got '%s'", checker.Name())
	}
}

func TestMemoryHealthChecker(t *testing.T) {
	tests := []struct {
		name           string
		stats          MemoryStats
		thresholds     MemoryThresholds
		expectedStatus Status
		expectedMsg    string
	}{
		{
			name: "healthy memory",
			stats: MemoryStats{
				HeapAlloc:    50 * 1024 * 1024, // 50 MB
				HeapSys:      100 * 1024 * 1024,
				HeapInuse:    50 * 1024 * 1024,
				HeapIdle:     50 * 1024 * 1024,
				NumGC:        10,
				NumGoroutine: 100,
			},
			thresholds:     DefaultMemoryThresholds(),
			expectedStatus: StatusHealthy,
			expectedMsg:    "Memory usage within normal parameters",
		},
		{
			name: "degraded - high water mark exceeded",
			stats: MemoryStats{
				HeapAlloc:    150 * 1024 * 1024, // 150 MB, exceeds 100 MB high water mark
				HeapSys:      200 * 1024 * 1024,
				HeapInuse:    150 * 1024 * 1024,
				HeapIdle:     50 * 1024 * 1024,
				NumGC:        10,
				NumGoroutine: 100,
			},
			thresholds:     DefaultMemoryThresholds(),
			expectedStatus: StatusDegraded,
			expectedMsg:    "Heap allocation exceeds high water mark",
		},
		{
			name: "unhealthy - critical mark exceeded",
			stats: MemoryStats{
				HeapAlloc:    250 * 1024 * 1024, // 250 MB, exceeds 200 MB critical mark
				HeapSys:      300 * 1024 * 1024,
				HeapInuse:    250 * 1024 * 1024,
				HeapIdle:     50 * 1024 * 1024,
				NumGC:        10,
				NumGoroutine: 100,
			},
			thresholds:     DefaultMemoryThresholds(),
			expectedStatus: StatusUnhealthy,
			expectedMsg:    "Heap allocation exceeds critical threshold",
		},
		{
			name: "unhealthy - too many goroutines",
			stats: MemoryStats{
				HeapAlloc:    50 * 1024 * 1024,
				HeapSys:      100 * 1024 * 1024,
				HeapInuse:    50 * 1024 * 1024,
				HeapIdle:     50 * 1024 * 1024,
				NumGC:        10,
				NumGoroutine: 15000, // Exceeds 10000 threshold
			},
			thresholds:     DefaultMemoryThresholds(),
			expectedStatus: StatusUnhealthy,
			expectedMsg:    "Goroutine count exceeds maximum threshold",
		},
		{
			name: "degraded - high heap ratio",
			stats: MemoryStats{
				HeapAlloc:    95 * 1024 * 1024, // Below high water mark but high ratio
				HeapSys:      100 * 1024 * 1024,
				HeapInuse:    95 * 1024 * 1024,
				HeapIdle:     5 * 1024 * 1024,
				NumGC:        10,
				NumGoroutine: 100,
			},
			thresholds:     DefaultMemoryThresholds(),
			expectedStatus: StatusDegraded,
			expectedMsg:    "Heap usage ratio indicates memory pressure",
		},
		{
			name: "healthy with zero heap sys",
			stats: MemoryStats{
				HeapAlloc:    0,
				HeapSys:      0, // Edge case: no heap allocated yet
				HeapInuse:    0,
				HeapIdle:     0,
				NumGC:        0,
				NumGoroutine: 1,
			},
			thresholds:     DefaultMemoryThresholds(),
			expectedStatus: StatusHealthy,
			expectedMsg:    "Memory usage within normal parameters",
		},
		{
			name: "custom thresholds - lower limits",
			stats: MemoryStats{
				HeapAlloc:    30 * 1024 * 1024, // 30 MB exceeds 25 MB high water
				HeapSys:      50 * 1024 * 1024,
				HeapInuse:    30 * 1024 * 1024,
				HeapIdle:     20 * 1024 * 1024,
				NumGC:        5,
				NumGoroutine: 50,
			},
			thresholds: MemoryThresholds{
				HighWaterMark:   25 * 1024 * 1024, // 25 MB
				CriticalMark:    50 * 1024 * 1024, // 50 MB
				MaxGoroutines:   1000,
				TargetHeapRatio: 0.9,
			},
			expectedStatus: StatusDegraded,
			expectedMsg:    "Heap allocation exceeds high water mark",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewMemoryHealthCheckerWithStatsFunc(tt.thresholds, func() MemoryStats {
				return tt.stats
			})

			result := checker.Check(context.Background())
			if result.Status != tt.expectedStatus {
				t.Errorf("Expected status %s, got %s", tt.expectedStatus, result.Status)
			}
			if result.Message != tt.expectedMsg {
				t.Errorf("Expected message '%s', got '%s'", tt.expectedMsg, result.Message)
			}
			if result.Name != "memory" {
				t.Errorf("Expected name 'memory', got '%s'", result.Name)
			}

			// Verify details are populated
			if result.Details == nil {
				t.Error("Expected details to be populated")
			} else {
				if _, ok := result.Details["heap_alloc_bytes"]; !ok {
					t.Error("Expected heap_alloc_bytes in details")
				}
				if _, ok := result.Details["num_goroutines"]; !ok {
					t.Error("Expected num_goroutines in details")
				}
			}
		})
	}
}

func TestGetRuntimeMemoryStats(t *testing.T) {
	stats := GetRuntimeMemoryStats()

	// Basic sanity checks - these values should be > 0 in a running program
	if stats.NumGoroutine == 0 {
		t.Error("Expected at least 1 goroutine")
	}

	// HeapAlloc should be reasonable (> 0, < 1GB for this test)
	if stats.HeapAlloc == 0 {
		t.Error("Expected HeapAlloc > 0")
	}
	if stats.HeapAlloc > 1024*1024*1024 {
		t.Errorf("HeapAlloc unexpectedly high: %d", stats.HeapAlloc)
	}
}

func TestMemoryPressureLevelString(t *testing.T) {
	tests := []struct {
		level    MemoryPressureLevel
		expected string
	}{
		{MemoryPressureNone, "none"},
		{MemoryPressureModerate, "moderate"},
		{MemoryPressureHigh, "high"},
		{MemoryPressureCritical, "critical"},
		{MemoryPressureLevel(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.level.String() != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, tt.level.String())
			}
		})
	}
}

func TestNewMemoryMonitor(t *testing.T) {
	thresholds := DefaultMemoryThresholds()
	monitor := NewMemoryMonitor(thresholds, time.Second)

	if monitor == nil {
		t.Fatal("NewMemoryMonitor returned nil")
	}
	if monitor.interval != time.Second {
		t.Errorf("Expected interval 1s, got %v", monitor.interval)
	}
}

func TestMemoryMonitorOnPressure(t *testing.T) {
	thresholds := DefaultMemoryThresholds()
	monitor := NewMemoryMonitor(thresholds, time.Second)

	callCount := 0
	monitor.OnPressure(func(level MemoryPressureLevel, stats MemoryStats) {
		callCount++
	})

	if len(monitor.callbacks) != 1 {
		t.Errorf("Expected 1 callback, got %d", len(monitor.callbacks))
	}
}

func TestMemoryMonitorGetCurrentPressureLevel(t *testing.T) {
	tests := []struct {
		name          string
		stats         MemoryStats
		expectedLevel MemoryPressureLevel
	}{
		{
			name: "no pressure",
			stats: MemoryStats{
				HeapAlloc: 10 * 1024 * 1024, // 10 MB - well below 75% of 100 MB
			},
			expectedLevel: MemoryPressureNone,
		},
		{
			name: "moderate pressure",
			stats: MemoryStats{
				HeapAlloc: 80 * 1024 * 1024, // 80 MB - above 75 MB (75% of 100 MB)
			},
			expectedLevel: MemoryPressureModerate,
		},
		{
			name: "high pressure",
			stats: MemoryStats{
				HeapAlloc: 150 * 1024 * 1024, // 150 MB - above 100 MB high water
			},
			expectedLevel: MemoryPressureHigh,
		},
		{
			name: "critical pressure",
			stats: MemoryStats{
				HeapAlloc: 250 * 1024 * 1024, // 250 MB - above 200 MB critical
			},
			expectedLevel: MemoryPressureCritical,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			thresholds := DefaultMemoryThresholds()
			monitor := NewMemoryMonitor(thresholds, time.Second)
			monitor.getStats = func() MemoryStats {
				return tt.stats
			}

			level := monitor.GetCurrentPressureLevel()
			if level != tt.expectedLevel {
				t.Errorf("Expected level %s, got %s", tt.expectedLevel, level)
			}
		})
	}
}

func TestMemoryMonitorStartStop(t *testing.T) {
	thresholds := DefaultMemoryThresholds()
	monitor := NewMemoryMonitor(thresholds, 50*time.Millisecond)

	var mu sync.Mutex
	levels := make([]MemoryPressureLevel, 0)

	// Track pressure level changes
	currentLevel := MemoryPressureNone
	monitor.getStats = func() MemoryStats {
		mu.Lock()
		defer mu.Unlock()
		// Cycle through pressure levels
		switch currentLevel {
		case MemoryPressureNone:
			currentLevel = MemoryPressureModerate
			return MemoryStats{HeapAlloc: 80 * 1024 * 1024}
		case MemoryPressureModerate:
			currentLevel = MemoryPressureHigh
			return MemoryStats{HeapAlloc: 150 * 1024 * 1024}
		default:
			return MemoryStats{HeapAlloc: 150 * 1024 * 1024}
		}
	}

	monitor.OnPressure(func(level MemoryPressureLevel, stats MemoryStats) {
		mu.Lock()
		defer mu.Unlock()
		levels = append(levels, level)
	})

	// Start in goroutine
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	go monitor.Start(ctx)

	// Wait for some callbacks
	time.Sleep(150 * time.Millisecond)

	mu.Lock()
	levelCount := len(levels)
	mu.Unlock()

	// Should have received at least one callback
	if levelCount == 0 {
		t.Error("Expected at least one pressure level callback")
	}
}

func TestMemoryMonitorStopChannel(t *testing.T) {
	thresholds := DefaultMemoryThresholds()
	monitor := NewMemoryMonitor(thresholds, 50*time.Millisecond)

	done := make(chan struct{})

	go func() {
		ctx := context.Background()
		monitor.Start(ctx)
		close(done)
	}()

	// Give it time to start
	time.Sleep(100 * time.Millisecond)

	// Stop should cause Start to return
	monitor.Stop()

	select {
	case <-done:
		// Success - Start returned after Stop
	case <-time.After(time.Second):
		t.Error("Monitor did not stop within timeout")
	}
}

func TestMemoryMonitorStopMultipleTimes(t *testing.T) {
	thresholds := DefaultMemoryThresholds()
	monitor := NewMemoryMonitor(thresholds, 50*time.Millisecond)

	// Stop should be safe to call multiple times without panicking
	monitor.Stop()
	monitor.Stop()
	monitor.Stop()
	// If we get here without panicking, the test passes
}

func TestTriggerGC(t *testing.T) {
	// This is a simple smoke test - we just verify it doesn't panic
	TriggerGC()
}

func TestFreeOSMemory(t *testing.T) {
	// This is a simple smoke test - we just verify it doesn't panic
	FreeOSMemory()
}

func TestMemoryHealthCheckerIntegration(t *testing.T) {
	// Test that the checker works with the health monitor
	monitor := NewMonitor()
	checker := NewMemoryHealthCheckerWithDefaults()
	monitor.RegisterChecker(checker)

	result := monitor.Check(context.Background())

	// Should have the memory component
	memHealth, exists := result.Components["memory"]
	if !exists {
		t.Fatal("Memory component not found in health check results")
	}

	// Should have valid status
	if memHealth.Status != StatusHealthy && memHealth.Status != StatusDegraded && memHealth.Status != StatusUnhealthy {
		t.Errorf("Unexpected status: %s", memHealth.Status)
	}

	// Should have details
	if memHealth.Details == nil {
		t.Error("Expected details to be populated")
	}
}
