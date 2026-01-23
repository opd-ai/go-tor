// Package metrics provides enhanced histogram implementation tests
package metrics

import (
	"testing"
	"time"
)

func TestEnhancedHistogram_BasicOperations(t *testing.T) {
	opts := HistogramOptions{
		MaxObservations:   100,
		TimeWindow:        0, // No time window
		EnableAggregation: false,
	}
	h := NewEnhancedHistogram(opts)

	// Test empty histogram
	if count := h.Count(); count != 0 {
		t.Errorf("Expected count 0 for empty histogram, got %d", count)
	}

	// Add observations
	observations := []time.Duration{
		100 * time.Millisecond,
		200 * time.Millisecond,
		300 * time.Millisecond,
		400 * time.Millisecond,
		500 * time.Millisecond,
	}

	for _, d := range observations {
		h.Observe(d)
	}

	// Test count
	if count := h.Count(); count != 5 {
		t.Errorf("Expected count 5, got %d", count)
	}

	// Test mean
	expectedMean := 300 * time.Millisecond
	if mean := h.Mean(); mean != expectedMean {
		t.Errorf("Expected mean %v, got %v", expectedMean, mean)
	}

	// Test min/max
	if min := h.Min(); min != 100*time.Millisecond {
		t.Errorf("Expected min 100ms, got %v", min)
	}
	if max := h.Max(); max != 500*time.Millisecond {
		t.Errorf("Expected max 500ms, got %v", max)
	}
}

func TestEnhancedHistogram_Percentiles(t *testing.T) {
	opts := DefaultHistogramOptions()
	opts.TimeWindow = 0 // Disable time window for this test
	h := NewEnhancedHistogram(opts)

	// Add 100 observations (1ms to 100ms)
	for i := 1; i <= 100; i++ {
		h.Observe(time.Duration(i) * time.Millisecond)
	}

	tests := []struct {
		name     string
		method   func() time.Duration
		expected time.Duration
		tolerance time.Duration
	}{
		{"P50", h.P50, 50 * time.Millisecond, 5 * time.Millisecond},
		{"P95", h.P95, 95 * time.Millisecond, 5 * time.Millisecond},
		{"P99", h.P99, 99 * time.Millisecond, 5 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := tt.method()
			diff := actual - tt.expected
			if diff < 0 {
				diff = -diff
			}
			if diff > tt.tolerance {
				t.Errorf("%s = %v, want %v (±%v)", tt.name, actual, tt.expected, tt.tolerance)
			}
		})
	}
}

func TestEnhancedHistogram_Snapshot(t *testing.T) {
	opts := DefaultHistogramOptions()
	opts.TimeWindow = 0
	h := NewEnhancedHistogram(opts)

	// Add observations
	for i := 1; i <= 100; i++ {
		h.Observe(time.Duration(i) * time.Millisecond)
	}

	snap := h.Snapshot()

	if snap.Count != 100 {
		t.Errorf("Expected count 100, got %d", snap.Count)
	}

	if snap.Min != 1*time.Millisecond {
		t.Errorf("Expected min 1ms, got %v", snap.Min)
	}

	if snap.Max != 100*time.Millisecond {
		t.Errorf("Expected max 100ms, got %v", snap.Max)
	}

	// Check percentiles are in reasonable range
	if snap.P50 < 40*time.Millisecond || snap.P50 > 60*time.Millisecond {
		t.Errorf("P50 out of expected range: %v", snap.P50)
	}

	if snap.P95 < 90*time.Millisecond || snap.P95 > 100*time.Millisecond {
		t.Errorf("P95 out of expected range: %v", snap.P95)
	}

	if snap.P99 < 95*time.Millisecond || snap.P99 > 100*time.Millisecond {
		t.Errorf("P99 out of expected range: %v", snap.P99)
	}
}

func TestEnhancedHistogram_MaxObservations(t *testing.T) {
	opts := HistogramOptions{
		MaxObservations: 10,
		TimeWindow:      0,
	}
	h := NewEnhancedHistogram(opts)

	// Add more observations than max
	for i := 1; i <= 20; i++ {
		h.Observe(time.Duration(i) * time.Millisecond)
	}

	// Force cleanup
	h.mu.Lock()
	h.lastCleanup = time.Time{} // Reset to force cleanup
	h.applyRetentionLocked(time.Now())
	count := len(h.observations)
	h.mu.Unlock()

	if count > 10 {
		t.Errorf("Expected max 10 observations, got %d", count)
	}
}

func TestEnhancedHistogram_TimeWindow(t *testing.T) {
	opts := HistogramOptions{
		MaxObservations: 1000,
		TimeWindow:      100 * time.Millisecond,
	}
	h := NewEnhancedHistogram(opts)

	// Add observations
	h.Observe(50 * time.Millisecond)
	h.Observe(100 * time.Millisecond)

	// Wait for observations to expire
	time.Sleep(150 * time.Millisecond)

	// Add new observation to trigger cleanup
	h.Observe(150 * time.Millisecond)

	// Force cleanup
	h.mu.Lock()
	h.lastCleanup = time.Time{}
	h.applyRetentionLocked(time.Now())
	count := len(h.observations)
	h.mu.Unlock()

	// Only the last observation should remain
	if count != 1 {
		t.Errorf("Expected 1 observation after time window expiry, got %d", count)
	}
}

func TestEnhancedHistogram_EmptyHistogram(t *testing.T) {
	opts := DefaultHistogramOptions()
	h := NewEnhancedHistogram(opts)

	// Test all methods on empty histogram
	if h.Count() != 0 {
		t.Error("Expected count 0 for empty histogram")
	}

	if h.Mean() != 0 {
		t.Error("Expected mean 0 for empty histogram")
	}

	if h.Min() != 0 {
		t.Error("Expected min 0 for empty histogram")
	}

	if h.Max() != 0 {
		t.Error("Expected max 0 for empty histogram")
	}

	if h.P50() != 0 {
		t.Error("Expected P50 0 for empty histogram")
	}

	snap := h.Snapshot()
	if snap.Count != 0 {
		t.Error("Expected snapshot count 0 for empty histogram")
	}
}

func TestAggregatedHistogram_BasicOperations(t *testing.T) {
	opts := DefaultHistogramOptions()
	windowDuration := 100 * time.Millisecond
	maxWindows := 5

	agg := NewAggregatedHistogram(windowDuration, maxWindows, opts)

	// Add observations
	agg.Observe(100 * time.Millisecond)
	agg.Observe(200 * time.Millisecond)
	agg.Observe(300 * time.Millisecond)

	// Verify window count
	if count := agg.WindowCount(); count != 1 {
		t.Errorf("Expected 1 window, got %d", count)
	}

	// Get aggregated snapshot
	snap := agg.AggregateAll()
	if snap.Count != 3 {
		t.Errorf("Expected 3 observations, got %d", snap.Count)
	}

	expectedMean := 200 * time.Millisecond
	if snap.Mean != expectedMean {
		t.Errorf("Expected mean %v, got %v", expectedMean, snap.Mean)
	}
}

func TestAggregatedHistogram_MultipleWindows(t *testing.T) {
	opts := DefaultHistogramOptions()
	opts.TimeWindow = 0 // Disable time window for window-based histogram
	windowDuration := 50 * time.Millisecond
	maxWindows := 3

	agg := NewAggregatedHistogram(windowDuration, maxWindows, opts)

	// Add observations in first window
	agg.Observe(100 * time.Millisecond)

	// Wait and add observations in second window
	time.Sleep(60 * time.Millisecond)
	agg.Observe(200 * time.Millisecond)

	// Wait and add observations in third window
	time.Sleep(60 * time.Millisecond)
	agg.Observe(300 * time.Millisecond)

	// Should have multiple windows
	if count := agg.WindowCount(); count < 2 {
		t.Errorf("Expected at least 2 windows, got %d", count)
	}

	// Aggregate all windows
	snap := agg.AggregateAll()
	if snap.Count != 3 {
		t.Errorf("Expected 3 total observations, got %d", snap.Count)
	}
}

func TestAggregatedHistogram_WindowCleanup(t *testing.T) {
	opts := DefaultHistogramOptions()
	opts.TimeWindow = 0
	windowDuration := 50 * time.Millisecond
	maxWindows := 2

	agg := NewAggregatedHistogram(windowDuration, maxWindows, opts)

	// Add observations in multiple windows
	for i := 0; i < 5; i++ {
		agg.Observe(time.Duration(i*100) * time.Millisecond)
		time.Sleep(60 * time.Millisecond)
	}

	// Window count should not exceed maxWindows
	count := agg.WindowCount()
	if count > maxWindows {
		t.Errorf("Expected max %d windows, got %d", maxWindows, count)
	}
}

func TestAggregatedHistogram_EmptyHistogram(t *testing.T) {
	opts := DefaultHistogramOptions()
	agg := NewAggregatedHistogram(100*time.Millisecond, 5, opts)

	snap := agg.AggregateAll()
	if snap.Count != 0 {
		t.Error("Expected count 0 for empty aggregated histogram")
	}

	if agg.WindowCount() != 0 {
		t.Error("Expected 0 windows for empty aggregated histogram")
	}
}

func TestDefaultHistogramOptions(t *testing.T) {
	opts := DefaultHistogramOptions()

	if opts.MaxObservations <= 0 {
		t.Error("Default MaxObservations should be positive")
	}

	if opts.BucketCount <= 0 {
		t.Error("Default BucketCount should be positive")
	}

	if opts.TimeWindow <= 0 {
		t.Error("Default TimeWindow should be positive")
	}

	if !opts.EnableAggregation {
		t.Error("Default EnableAggregation should be true")
	}
}

func TestEnhancedHistogram_ConcurrentAccess(t *testing.T) {
	opts := DefaultHistogramOptions()
	h := NewEnhancedHistogram(opts)

	// Concurrent writes and reads
	done := make(chan bool)

	// Writer goroutines
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				h.Observe(time.Duration(id*100+j) * time.Microsecond)
			}
			done <- true
		}(i)
	}

	// Reader goroutines
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_ = h.Mean()
				_ = h.P50()
				_ = h.P95()
				_ = h.P99()
				_ = h.Count()
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 15; i++ {
		<-done
	}

	// Verify histogram has data
	if h.Count() == 0 {
		t.Error("Expected histogram to have observations after concurrent access")
	}
}

func BenchmarkEnhancedHistogram_Observe(b *testing.B) {
	opts := DefaultHistogramOptions()
	h := NewEnhancedHistogram(opts)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h.Observe(time.Duration(i) * time.Microsecond)
	}
}

func BenchmarkEnhancedHistogram_P95(b *testing.B) {
	opts := DefaultHistogramOptions()
	h := NewEnhancedHistogram(opts)

	// Populate histogram
	for i := 0; i < 1000; i++ {
		h.Observe(time.Duration(i) * time.Microsecond)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = h.P95()
	}
}

func BenchmarkEnhancedHistogram_Snapshot(b *testing.B) {
	opts := DefaultHistogramOptions()
	h := NewEnhancedHistogram(opts)

	// Populate histogram
	for i := 0; i < 1000; i++ {
		h.Observe(time.Duration(i) * time.Microsecond)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = h.Snapshot()
	}
}

func BenchmarkAggregatedHistogram_Observe(b *testing.B) {
	opts := DefaultHistogramOptions()
	agg := NewAggregatedHistogram(time.Minute, 60, opts)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		agg.Observe(time.Duration(i) * time.Microsecond)
	}
}

func BenchmarkAggregatedHistogram_AggregateAll(b *testing.B) {
	opts := DefaultHistogramOptions()
	agg := NewAggregatedHistogram(time.Minute, 60, opts)

	// Populate with data
	for i := 0; i < 1000; i++ {
		agg.Observe(time.Duration(i) * time.Microsecond)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = agg.AggregateAll()
	}
}
