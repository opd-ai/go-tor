// Package metrics provides enhanced histogram implementation with
// percentile calculations, time window aggregation, and retention policies.
package metrics

import (
	"math"
	"sort"
	"sync"
	"time"
)

// HistogramOptions configures histogram behavior
type HistogramOptions struct {
	// MaxObservations is the maximum number of observations to keep
	// Default: 1000
	MaxObservations int

	// TimeWindow is the duration to keep observations (0 = unlimited)
	// When set, observations older than this duration are discarded
	TimeWindow time.Duration

	// BucketCount is the number of buckets for aggregation
	// Default: 20
	BucketCount int

	// EnableAggregation enables time window aggregation
	EnableAggregation bool
}

// DefaultHistogramOptions returns default histogram configuration
func DefaultHistogramOptions() HistogramOptions {
	return HistogramOptions{
		MaxObservations:   1000,
		TimeWindow:        5 * time.Minute,
		BucketCount:       20,
		EnableAggregation: true,
	}
}

// TimedObservation represents an observation with timestamp
type TimedObservation struct {
	Value     time.Duration
	Timestamp time.Time
}

// EnhancedHistogram provides advanced histogram functionality with
// percentile calculation, time window aggregation, and retention policies
type EnhancedHistogram struct {
	observations []TimedObservation
	options      HistogramOptions
	mu           sync.RWMutex
	lastCleanup  time.Time
}

// NewEnhancedHistogram creates a new enhanced histogram with specified options
func NewEnhancedHistogram(opts HistogramOptions) *EnhancedHistogram {
	if opts.MaxObservations <= 0 {
		opts.MaxObservations = 1000
	}
	if opts.BucketCount <= 0 {
		opts.BucketCount = 20
	}

	return &EnhancedHistogram{
		observations: make([]TimedObservation, 0, opts.MaxObservations),
		options:      opts,
		lastCleanup:  time.Now(),
	}
}

// Observe adds a new observation to the histogram
func (h *EnhancedHistogram) Observe(d time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()
	h.observations = append(h.observations, TimedObservation{
		Value:     d,
		Timestamp: now,
	})

	// Apply retention policy if needed
	h.applyRetentionLocked(now)
}

// applyRetentionLocked applies retention policies (caller must hold lock)
func (h *EnhancedHistogram) applyRetentionLocked(now time.Time) {
	// Only cleanup periodically to avoid performance hit
	if now.Sub(h.lastCleanup) < time.Minute {
		return
	}
	h.lastCleanup = now

	// Remove old observations based on time window
	if h.options.TimeWindow > 0 {
		cutoff := now.Add(-h.options.TimeWindow)
		newObs := make([]TimedObservation, 0, len(h.observations))
		for _, obs := range h.observations {
			if obs.Timestamp.After(cutoff) {
				newObs = append(newObs, obs)
			}
		}
		h.observations = newObs
	}

	// Keep only last N observations
	if len(h.observations) > h.options.MaxObservations {
		h.observations = h.observations[len(h.observations)-h.options.MaxObservations:]
	}
}

// P50 returns the 50th percentile (median)
func (h *EnhancedHistogram) P50() time.Duration {
	return h.Percentile(0.50)
}

// P95 returns the 95th percentile
func (h *EnhancedHistogram) P95() time.Duration {
	return h.Percentile(0.95)
}

// P99 returns the 99th percentile
func (h *EnhancedHistogram) P99() time.Duration {
	return h.Percentile(0.99)
}

// Percentile returns the nth percentile (0.0 to 1.0)
func (h *EnhancedHistogram) Percentile(p float64) time.Duration {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.observations) == 0 {
		return 0
	}

	// Extract values and sort (copy to avoid modifying original)
	values := make([]time.Duration, len(h.observations))
	for i, obs := range h.observations {
		values[i] = obs.Value
	}

	sort.Slice(values, func(i, j int) bool {
		return values[i] < values[j]
	})

	// Calculate percentile index
	index := int(float64(len(values)-1) * p)
	if index < 0 {
		index = 0
	}
	if index >= len(values) {
		index = len(values) - 1
	}

	return values[index]
}

// Mean returns the mean of all observations
func (h *EnhancedHistogram) Mean() time.Duration {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.observations) == 0 {
		return 0
	}

	var sum time.Duration
	for _, obs := range h.observations {
		sum += obs.Value
	}
	return sum / time.Duration(len(h.observations))
}

// Count returns the number of observations
func (h *EnhancedHistogram) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.observations)
}

// Min returns the minimum observation
func (h *EnhancedHistogram) Min() time.Duration {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.observations) == 0 {
		return 0
	}

	min := h.observations[0].Value
	for _, obs := range h.observations[1:] {
		if obs.Value < min {
			min = obs.Value
		}
	}
	return min
}

// Max returns the maximum observation
func (h *EnhancedHistogram) Max() time.Duration {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.observations) == 0 {
		return 0
	}

	max := h.observations[0].Value
	for _, obs := range h.observations[1:] {
		if obs.Value > max {
			max = obs.Value
		}
	}
	return max
}

// Snapshot returns a statistical snapshot of the histogram
func (h *EnhancedHistogram) Snapshot() HistogramSnapshot {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.observations) == 0 {
		return HistogramSnapshot{}
	}

	// Extract and sort values
	values := make([]time.Duration, len(h.observations))
	for i, obs := range h.observations {
		values[i] = obs.Value
	}
	sort.Slice(values, func(i, j int) bool {
		return values[i] < values[j]
	})

	// Calculate statistics
	var sum time.Duration
	for _, v := range values {
		sum += v
	}

	return HistogramSnapshot{
		Count:  len(values),
		Mean:   sum / time.Duration(len(values)),
		Min:    values[0],
		Max:    values[len(values)-1],
		P50:    values[int(float64(len(values)-1)*0.50)],
		P95:    values[int(float64(len(values)-1)*0.95)],
		P99:    values[int(float64(len(values)-1)*0.99)],
		P999:   values[int(float64(len(values)-1)*0.999)],
		StdDev: h.calculateStdDevLocked(values, sum/time.Duration(len(values))),
	}
}

// calculateStdDevLocked calculates standard deviation (caller must hold read lock)
func (h *EnhancedHistogram) calculateStdDevLocked(values []time.Duration, mean time.Duration) time.Duration {
	if len(values) == 0 {
		return 0
	}

	var sumSquares float64
	for _, v := range values {
		diff := float64(v - mean)
		sumSquares += diff * diff
	}

	variance := sumSquares / float64(len(values))
	// Standard deviation is square root of variance
	stdDev := math.Sqrt(variance)
	return time.Duration(stdDev)
}

// HistogramSnapshot represents a point-in-time statistical snapshot
type HistogramSnapshot struct {
	Count  int
	Mean   time.Duration
	Min    time.Duration
	Max    time.Duration
	P50    time.Duration // Median
	P95    time.Duration
	P99    time.Duration
	P999   time.Duration
	StdDev time.Duration
}

// AggregatedHistogram provides time-window based aggregation
type AggregatedHistogram struct {
	windows map[time.Time]*EnhancedHistogram
	mu      sync.RWMutex

	windowDuration time.Duration
	maxWindows     int
	histOptions    HistogramOptions
}

// NewAggregatedHistogram creates a histogram with time-based aggregation
// windowDuration: duration of each aggregation window (e.g., 1 minute)
// maxWindows: maximum number of windows to keep
func NewAggregatedHistogram(windowDuration time.Duration, maxWindows int, opts HistogramOptions) *AggregatedHistogram {
	if maxWindows <= 0 {
		maxWindows = 60 // Default: 60 windows
	}

	return &AggregatedHistogram{
		windows:        make(map[time.Time]*EnhancedHistogram),
		windowDuration: windowDuration,
		maxWindows:     maxWindows,
		histOptions:    opts,
	}
}

// Observe adds an observation to the current time window
func (a *AggregatedHistogram) Observe(d time.Duration) {
	now := time.Now()
	windowKey := now.Truncate(a.windowDuration)

	a.mu.Lock()
	defer a.mu.Unlock()

	// Get or create histogram for this window
	hist, exists := a.windows[windowKey]
	if !exists {
		hist = NewEnhancedHistogram(a.histOptions)
		a.windows[windowKey] = hist
	}

	hist.Observe(d)

	// Clean up old windows
	a.cleanupOldWindowsLocked(now)
}

// cleanupOldWindowsLocked removes windows exceeding maxWindows (caller must hold lock)
func (a *AggregatedHistogram) cleanupOldWindowsLocked(now time.Time) {
	if len(a.windows) <= a.maxWindows {
		return
	}

	// Find oldest windows to remove
	cutoff := now.Add(-time.Duration(a.maxWindows) * a.windowDuration)

	for key := range a.windows {
		if key.Before(cutoff) {
			delete(a.windows, key)
		}
	}
}

// AggregateAll returns aggregated statistics across all windows
func (a *AggregatedHistogram) AggregateAll() HistogramSnapshot {
	a.mu.RLock()
	// Collect histogram references while holding the lock
	histograms := make([]*EnhancedHistogram, 0, len(a.windows))
	for _, hist := range a.windows {
		histograms = append(histograms, hist)
	}
	a.mu.RUnlock()

	// Now access histograms without holding the aggregated lock
	var allValues []time.Duration
	for _, hist := range histograms {
		hist.mu.RLock()
		for _, obs := range hist.observations {
			allValues = append(allValues, obs.Value)
		}
		hist.mu.RUnlock()
	}

	if len(allValues) == 0 {
		return HistogramSnapshot{}
	}

	sort.Slice(allValues, func(i, j int) bool {
		return allValues[i] < allValues[j]
	})

	var sum time.Duration
	for _, v := range allValues {
		sum += v
	}
	mean := sum / time.Duration(len(allValues))

	return HistogramSnapshot{
		Count: len(allValues),
		Mean:  mean,
		Min:   allValues[0],
		Max:   allValues[len(allValues)-1],
		P50:   allValues[int(float64(len(allValues)-1)*0.50)],
		P95:   allValues[int(float64(len(allValues)-1)*0.95)],
		P99:   allValues[int(float64(len(allValues)-1)*0.99)],
		P999:  allValues[int(float64(len(allValues)-1)*0.999)],
	}
}

// GetWindow returns histogram for a specific time window
func (a *AggregatedHistogram) GetWindow(t time.Time) *EnhancedHistogram {
	a.mu.RLock()
	defer a.mu.RUnlock()

	windowKey := t.Truncate(a.windowDuration)
	return a.windows[windowKey]
}

// WindowCount returns the number of active windows
func (a *AggregatedHistogram) WindowCount() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.windows)
}
