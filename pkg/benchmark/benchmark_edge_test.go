package benchmark

import (
	"testing"
	"time"
)

func TestFormatBytes_EdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		bytes uint64
		want  string
	}{
		{"zero", 0, "0 B"},
		{"one byte", 1, "1 B"},
		{"max below KiB", 1023, "1023 B"},
		{"exact KiB", 1024, "1.0 KiB"},
		{"exact MiB", 1024 * 1024, "1.0 MiB"},
		{"exact GiB", 1024 * 1024 * 1024, "1.0 GiB"},
		{"exact TiB", 1024 * 1024 * 1024 * 1024, "1.0 TiB"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatBytes(tt.bytes)
			if got != tt.want {
				t.Errorf("FormatBytes(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestLatencyTracker_SingleElement(t *testing.T) {
	tracker := NewLatencyTracker(1)
	tracker.Record(42 * time.Millisecond)

	tests := []struct {
		name string
		fn   func() time.Duration
		want time.Duration
	}{
		{"count check via max", tracker.Max, 42 * time.Millisecond},
		{"p0", func() time.Duration { return tracker.Percentile(0.0) }, 42 * time.Millisecond},
		{"p50", func() time.Duration { return tracker.Percentile(0.5) }, 42 * time.Millisecond},
		{"p100", func() time.Duration { return tracker.Percentile(1.0) }, 42 * time.Millisecond},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fn()
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLatencyTracker_NegativePercentile(t *testing.T) {
	tracker := NewLatencyTracker(5)
	tracker.Record(10 * time.Millisecond)
	tracker.Record(20 * time.Millisecond)
	tracker.Record(30 * time.Millisecond)

	// Negative percentile should clamp to index 0
	got := tracker.Percentile(-1.0)
	if got != 10*time.Millisecond {
		t.Errorf("got %v, want 10ms", got)
	}
}

func TestLatencyTracker_ZeroCapacity(t *testing.T) {
	tracker := NewLatencyTracker(0)
	tracker.Record(5 * time.Millisecond)
	if tracker.Count() != 1 {
		t.Errorf("expected count 1, got %d", tracker.Count())
	}
}

func TestNewSuite_ResultsInitiallyEmpty(t *testing.T) {
	suite := NewSuite(nil)
	results := suite.Results()
	if results == nil {
		t.Error("expected non-nil results slice")
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestSuite_AddMultipleResults(t *testing.T) {
	suite := NewSuite(nil)
	tests := []struct {
		name    string
		success bool
	}{
		{"bench1", true},
		{"bench2", false},
		{"bench3", true},
	}
	for _, tt := range tests {
		suite.addResult(Result{Name: tt.name, Success: tt.success})
	}
	results := suite.Results()
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	for i, tt := range tests {
		if results[i].Name != tt.name {
			t.Errorf("result[%d].Name = %q, want %q", i, results[i].Name, tt.name)
		}
		if results[i].Success != tt.success {
			t.Errorf("result[%d].Success = %v, want %v", i, results[i].Success, tt.success)
		}
	}
}

func TestGetMemorySnapshot_NonZero(t *testing.T) {
	snap := GetMemorySnapshot()
	checks := []struct {
		name string
		val  uint64
	}{
		{"Alloc", snap.Alloc},
		{"TotalAlloc", snap.TotalAlloc},
		{"Sys", snap.Sys},
		{"HeapAlloc", snap.HeapAlloc},
		{"HeapSys", snap.HeapSys},
	}
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			if c.val == 0 {
				t.Errorf("%s should be non-zero", c.name)
			}
		})
	}
}

func TestQuickSort_EdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		input []time.Duration
	}{
		{"single element", []time.Duration{1 * time.Second}},
		{"already sorted", []time.Duration{1, 2, 3, 4, 5}},
		{"reverse sorted", []time.Duration{5, 4, 3, 2, 1}},
		{"duplicates", []time.Duration{3, 1, 3, 1, 3}},
		{"two elements", []time.Duration{2, 1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			arr := make([]time.Duration, len(tt.input))
			copy(arr, tt.input)
			quickSort(arr, 0, len(arr)-1)
			for i := 1; i < len(arr); i++ {
				if arr[i] < arr[i-1] {
					t.Errorf("not sorted at %d: %v > %v", i, arr[i-1], arr[i])
				}
			}
		})
	}
}
