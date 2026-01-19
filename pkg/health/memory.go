// Package health provides health check and monitoring capabilities for the Tor client.
package health

import (
	"context"
	"runtime"
	"runtime/debug"
	"sync"
	"time"
)

// MemoryStats contains memory statistics for health checking.
// These values are derived from runtime.MemStats.
type MemoryStats struct {
	// HeapAlloc is bytes of allocated heap objects.
	HeapAlloc uint64
	// HeapSys is bytes obtained from the OS for the heap.
	HeapSys uint64
	// HeapInuse is bytes in in-use spans.
	HeapInuse uint64
	// HeapIdle is bytes in idle (unused) spans.
	HeapIdle uint64
	// HeapReleased is bytes of physical memory returned to the OS.
	HeapReleased uint64
	// NumGC is the number of completed GC cycles.
	NumGC uint32
	// LastGC is the time the last garbage collection finished.
	LastGC time.Time
	// GCPauseTotal is the cumulative GC pause time.
	GCPauseTotal time.Duration
	// NumGoroutine is the number of goroutines that currently exist.
	NumGoroutine int
}

// MemoryThresholds defines thresholds for memory pressure monitoring.
// When exceeded, the system may take action to reduce memory usage.
type MemoryThresholds struct {
	// HighWaterMark is the heap allocation threshold in bytes that triggers degraded status.
	// Default: 100MB
	HighWaterMark uint64
	// CriticalMark is the heap allocation threshold in bytes that triggers unhealthy status.
	// Default: 200MB
	CriticalMark uint64
	// MaxGoroutines is the threshold for number of goroutines.
	// Default: 10000
	MaxGoroutines int
	// TargetHeapRatio is the target ratio of HeapAlloc/HeapSys (0.0-1.0).
	// Higher values indicate potential memory pressure.
	// Default: 0.9
	TargetHeapRatio float64
}

// DefaultMemoryThresholds returns sensible defaults for memory thresholds.
// These defaults are tuned for embedded deployments with limited resources.
func DefaultMemoryThresholds() MemoryThresholds {
	return MemoryThresholds{
		HighWaterMark:   100 * 1024 * 1024, // 100 MB
		CriticalMark:    200 * 1024 * 1024, // 200 MB
		MaxGoroutines:   10000,
		TargetHeapRatio: 0.9,
	}
}

// MemoryHealthChecker checks the health of memory usage.
// It monitors heap allocation, goroutine count, and garbage collection metrics.
type MemoryHealthChecker struct {
	thresholds MemoryThresholds
	getStats   func() MemoryStats
}

// NewMemoryHealthChecker creates a new memory health checker with custom thresholds.
func NewMemoryHealthChecker(thresholds MemoryThresholds) *MemoryHealthChecker {
	return &MemoryHealthChecker{
		thresholds: thresholds,
		getStats:   GetRuntimeMemoryStats,
	}
}

// NewMemoryHealthCheckerWithDefaults creates a memory health checker with default thresholds.
func NewMemoryHealthCheckerWithDefaults() *MemoryHealthChecker {
	return NewMemoryHealthChecker(DefaultMemoryThresholds())
}

// NewMemoryHealthCheckerWithStatsFunc creates a memory health checker with a custom stats function.
// This is useful for testing where you want to provide mock memory stats.
func NewMemoryHealthCheckerWithStatsFunc(thresholds MemoryThresholds, getStats func() MemoryStats) *MemoryHealthChecker {
	return &MemoryHealthChecker{
		thresholds: thresholds,
		getStats:   getStats,
	}
}

// Name returns the checker name
func (m *MemoryHealthChecker) Name() string {
	return "memory"
}

// Check performs the health check
func (m *MemoryHealthChecker) Check(ctx context.Context) ComponentHealth {
	stats := m.getStats()

	// Calculate heap usage ratio
	var heapRatio float64
	if stats.HeapSys > 0 {
		heapRatio = float64(stats.HeapAlloc) / float64(stats.HeapSys)
	}

	health := ComponentHealth{
		Name:        m.Name(),
		LastChecked: time.Now(),
		Details: map[string]interface{}{
			"heap_alloc_bytes":    stats.HeapAlloc,
			"heap_sys_bytes":      stats.HeapSys,
			"heap_inuse_bytes":    stats.HeapInuse,
			"heap_idle_bytes":     stats.HeapIdle,
			"heap_released_bytes": stats.HeapReleased,
			"heap_ratio":          heapRatio,
			"num_gc":              stats.NumGC,
			"gc_pause_total_ns":   stats.GCPauseTotal.Nanoseconds(),
			"num_goroutines":      stats.NumGoroutine,
		},
	}

	// Determine status based on memory thresholds
	// Priority: Critical > Degraded > Healthy

	// Check for critical memory conditions first
	if stats.HeapAlloc >= m.thresholds.CriticalMark {
		health.Status = StatusUnhealthy
		health.Message = "Heap allocation exceeds critical threshold"
		return health
	}

	if stats.NumGoroutine >= m.thresholds.MaxGoroutines {
		health.Status = StatusUnhealthy
		health.Message = "Goroutine count exceeds maximum threshold"
		return health
	}

	// Check for degraded conditions
	if stats.HeapAlloc >= m.thresholds.HighWaterMark {
		health.Status = StatusDegraded
		health.Message = "Heap allocation exceeds high water mark"
		return health
	}

	if heapRatio >= m.thresholds.TargetHeapRatio {
		health.Status = StatusDegraded
		health.Message = "Heap usage ratio indicates memory pressure"
		return health
	}

	// All checks passed
	health.Status = StatusHealthy
	health.Message = "Memory usage within normal parameters"
	return health
}

// GetRuntimeMemoryStats returns current memory statistics from the Go runtime.
func GetRuntimeMemoryStats() MemoryStats {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return MemoryStats{
		HeapAlloc:    memStats.HeapAlloc,
		HeapSys:      memStats.HeapSys,
		HeapInuse:    memStats.HeapInuse,
		HeapIdle:     memStats.HeapIdle,
		HeapReleased: memStats.HeapReleased,
		NumGC:        memStats.NumGC,
		LastGC:       time.Unix(0, int64(memStats.LastGC)),
		GCPauseTotal: time.Duration(memStats.PauseTotalNs),
		NumGoroutine: runtime.NumGoroutine(),
	}
}

// MemoryPressureCallback is a function called when memory pressure is detected.
type MemoryPressureCallback func(level MemoryPressureLevel, stats MemoryStats)

// MemoryPressureLevel indicates the severity of memory pressure.
type MemoryPressureLevel int

const (
	// MemoryPressureNone indicates no memory pressure.
	MemoryPressureNone MemoryPressureLevel = iota
	// MemoryPressureModerate indicates moderate memory pressure (approaching high water mark).
	MemoryPressureModerate
	// MemoryPressureHigh indicates high memory pressure (exceeds high water mark).
	MemoryPressureHigh
	// MemoryPressureCritical indicates critical memory pressure (exceeds critical mark).
	MemoryPressureCritical
)

// String returns a string representation of the memory pressure level.
func (l MemoryPressureLevel) String() string {
	switch l {
	case MemoryPressureNone:
		return "none"
	case MemoryPressureModerate:
		return "moderate"
	case MemoryPressureHigh:
		return "high"
	case MemoryPressureCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// MemoryMonitor provides continuous memory pressure monitoring with callbacks.
type MemoryMonitor struct {
	thresholds MemoryThresholds
	getStats   func() MemoryStats
	callbacks  []MemoryPressureCallback
	interval   time.Duration
	stopCh     chan struct{}
	stopOnce   sync.Once
	lastLevel  MemoryPressureLevel
	mu         sync.RWMutex // Protects callbacks and lastLevel
}

// NewMemoryMonitor creates a new memory monitor with the specified thresholds and check interval.
func NewMemoryMonitor(thresholds MemoryThresholds, interval time.Duration) *MemoryMonitor {
	return &MemoryMonitor{
		thresholds: thresholds,
		getStats:   GetRuntimeMemoryStats,
		callbacks:  make([]MemoryPressureCallback, 0),
		interval:   interval,
		stopCh:     make(chan struct{}),
		lastLevel:  MemoryPressureNone,
	}
}

// OnPressure registers a callback to be called when memory pressure changes.
// This method is safe to call concurrently.
func (m *MemoryMonitor) OnPressure(callback MemoryPressureCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callbacks = append(m.callbacks, callback)
}

// Start begins continuous memory monitoring. This is a blocking call that runs until Stop is called.
func (m *MemoryMonitor) Start(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			stats := m.getStats()
			level := m.calculatePressureLevel(stats)

			// Only notify on level changes
			m.mu.Lock()
			if level != m.lastLevel {
				// Copy callbacks to avoid holding lock during callback execution
				callbacks := make([]MemoryPressureCallback, len(m.callbacks))
				copy(callbacks, m.callbacks)
				m.lastLevel = level
				m.mu.Unlock()

				// Execute callbacks without holding the lock
				for _, callback := range callbacks {
					callback(level, stats)
				}
			} else {
				m.mu.Unlock()
			}
		}
	}
}

// Stop stops the memory monitor. This method is safe to call multiple times.
func (m *MemoryMonitor) Stop() {
	m.stopOnce.Do(func() {
		close(m.stopCh)
	})
}

// GetCurrentPressureLevel returns the current memory pressure level without triggering callbacks.
// This method is safe to call concurrently.
func (m *MemoryMonitor) GetCurrentPressureLevel() MemoryPressureLevel {
	stats := m.getStats()
	return m.calculatePressureLevel(stats)
}

// calculatePressureLevel determines the memory pressure level based on current stats.
func (m *MemoryMonitor) calculatePressureLevel(stats MemoryStats) MemoryPressureLevel {
	if stats.HeapAlloc >= m.thresholds.CriticalMark {
		return MemoryPressureCritical
	}

	if stats.HeapAlloc >= m.thresholds.HighWaterMark {
		return MemoryPressureHigh
	}

	// Moderate pressure at 75% of high water mark
	moderateThreshold := uint64(float64(m.thresholds.HighWaterMark) * 0.75)
	if stats.HeapAlloc >= moderateThreshold {
		return MemoryPressureModerate
	}

	return MemoryPressureNone
}

// TriggerGC triggers a garbage collection cycle.
// This can be called when memory pressure is detected to attempt to free memory.
func TriggerGC() {
	runtime.GC()
}

// FreeOSMemory returns unused memory to the operating system.
// This runs a GC cycle first, then aggressively returns free memory to the OS.
// This is more aggressive than TriggerGC and should be used sparingly,
// as it may cause performance overhead when memory needs to be re-acquired.
func FreeOSMemory() {
	debug.FreeOSMemory()
}
