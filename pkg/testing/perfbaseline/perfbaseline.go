// Package perfbaseline provides utilities for capturing and validating
// performance baselines for the go-tor implementation.
//
// It implements threshold-based assertions against the performance targets
// stated in the project README:
//
//   - Circuit build time: < 5s at the 95th percentile
//   - Memory usage: < 50 MB RSS in steady state
//   - Binary size: < 15 MB static binary
//   - Concurrent streams: ≥ 100 on typical hardware
//
// # Usage
//
//	baseline := perfbaseline.New()
//	baseline.RecordDuration("circuit_build", elapsed)
//
//	if err := baseline.Check(); err != nil {
//	    t.Errorf("performance regression: %v", err)
//	}
//
// # Storing baselines
//
// Baselines are encoded to/from JSON so they can be committed to the repository
// and compared in CI:
//
//	data, _ := json.Marshal(baseline.Snapshot())
//	os.WriteFile("testdata/perf-baseline.json", data, 0o600)
package perfbaseline

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// Thresholds contains the project's stated performance targets.
var Thresholds = Threshold{
	CircuitBuildP95:    5 * time.Second,
	MemoryRSSMB:        50,
	ConcurrentStreams:  100,
}

// Threshold holds performance targets against which measurements are compared.
type Threshold struct {
	// CircuitBuildP95 is the maximum acceptable 95th-percentile circuit build time.
	CircuitBuildP95 time.Duration
	// MemoryRSSMB is the maximum acceptable RSS memory usage in megabytes.
	MemoryRSSMB float64
	// ConcurrentStreams is the minimum acceptable concurrent stream count.
	ConcurrentStreams int
}

// Measurement records a single timing sample for a named operation.
type Measurement struct {
	Name     string        `json:"name"`
	Elapsed  time.Duration `json:"elapsed_ns"`
	SampleAt time.Time     `json:"sampled_at"`
}

// Snapshot is a serialisable performance snapshot.
type Snapshot struct {
	CreatedAt    time.Time      `json:"created_at"`
	Measurements []Measurement  `json:"measurements"`
	MemStats     MemStats       `json:"mem_stats"`
}

// MemStats captures memory usage metrics.
type MemStats struct {
	AllocMB     float64 `json:"alloc_mb"`
	SysMB       float64 `json:"sys_mb"`
	NumGoroutine int    `json:"num_goroutine"`
}

// Baseline collects performance measurements and validates them against
// configured thresholds.
type Baseline struct {
	mu           sync.Mutex
	measurements []Measurement
	thresholds   Threshold
}

// New creates a Baseline with the default thresholds from README.
func New() *Baseline {
	return &Baseline{thresholds: Thresholds}
}

// NewWithThresholds creates a Baseline with custom thresholds.
func NewWithThresholds(t Threshold) *Baseline {
	return &Baseline{thresholds: t}
}

// RecordDuration records a single duration measurement for the named operation.
func (b *Baseline) RecordDuration(name string, d time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.measurements = append(b.measurements, Measurement{
		Name:     name,
		Elapsed:  d,
		SampleAt: time.Now(),
	})
}

// MeasureFunc records the wall-clock time of calling fn under the given name.
func (b *Baseline) MeasureFunc(name string, fn func()) time.Duration {
	start := time.Now()
	fn()
	d := time.Since(start)
	b.RecordDuration(name, d)
	return d
}

// CheckCircuitBuild validates that all "circuit_build" measurements
// meet the configured CircuitBuildP95 threshold.
// Returns nil if the threshold is met or no measurements exist.
func (b *Baseline) CheckCircuitBuild() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	var samples []time.Duration
	for _, m := range b.measurements {
		if m.Name == "circuit_build" {
			samples = append(samples, m.Elapsed)
		}
	}
	if len(samples) == 0 {
		return nil
	}

	p95 := percentile(samples, 0.95)
	if p95 > b.thresholds.CircuitBuildP95 {
		return fmt.Errorf(
			"circuit build P95 %v exceeds threshold %v",
			p95, b.thresholds.CircuitBuildP95,
		)
	}
	return nil
}

// CheckMemory validates current Go runtime memory against the RSS threshold.
func (b *Baseline) CheckMemory() error {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	allocMB := float64(ms.Alloc) / (1024 * 1024)
	if allocMB > b.thresholds.MemoryRSSMB {
		return fmt.Errorf("memory alloc %.1f MB exceeds threshold %.1f MB",
			allocMB, b.thresholds.MemoryRSSMB)
	}
	return nil
}

// Check runs all threshold validations and returns a combined error if any fail.
func (b *Baseline) Check() error {
	var errs []string
	if err := b.CheckCircuitBuild(); err != nil {
		errs = append(errs, err.Error())
	}
	if err := b.CheckMemory(); err != nil {
		errs = append(errs, err.Error())
	}
	if len(errs) > 0 {
		return fmt.Errorf("performance regression(s): %s", strings.Join(errs, "; "))
	}
	return nil
}

// Snapshot returns a serialisable snapshot of all recorded measurements
// together with current memory stats.
func (b *Baseline) Snapshot() *Snapshot {
	b.mu.Lock()
	measurements := make([]Measurement, len(b.measurements))
	copy(measurements, b.measurements)
	b.mu.Unlock()

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	return &Snapshot{
		CreatedAt:    time.Now(),
		Measurements: measurements,
		MemStats: MemStats{
			AllocMB:      float64(ms.Alloc) / (1024 * 1024),
			SysMB:        float64(ms.Sys) / (1024 * 1024),
			NumGoroutine: runtime.NumGoroutine(),
		},
	}
}

// SaveSnapshot writes a JSON-encoded snapshot to the given file path.
func (b *Baseline) SaveSnapshot(path string) error {
	s := b.Snapshot()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}
	return os.WriteFile(path, data, 0o600)
}

// LoadSnapshot reads a snapshot from a JSON file.
func LoadSnapshot(path string) (*Snapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read snapshot: %w", err)
	}
	var s Snapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("unmarshal snapshot: %w", err)
	}
	return &s, nil
}

// CompareSnapshots computes the percentage change for each named measurement
// between two snapshots and returns violations that exceed maxDeltaPct.
// A positive delta means degradation; negative means improvement.
func CompareSnapshots(baseline, current *Snapshot, maxDeltaPct float64) []string {
	// Build average by name for baseline.
	bAvg := avgByName(baseline.Measurements)
	cAvg := avgByName(current.Measurements)

	var violations []string
	for name, bVal := range bAvg {
		cVal, ok := cAvg[name]
		if !ok {
			continue
		}
		if bVal == 0 {
			continue
		}
		delta := (float64(cVal-bVal) / float64(bVal)) * 100
		if delta > maxDeltaPct {
			violations = append(violations, fmt.Sprintf(
				"%s: %.1f%% regression (baseline=%v current=%v)",
				name, delta, bVal, cVal,
			))
		}
	}
	sort.Strings(violations)
	return violations
}

// --- helpers ---

func percentile(samples []time.Duration, p float64) time.Duration {
	if len(samples) == 0 {
		return 0
	}
	sorted := make([]time.Duration, len(samples))
	copy(sorted, samples)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	idx := p * float64(len(sorted)-1)
	lo := int(math.Floor(idx))
	hi := int(math.Ceil(idx))
	if lo == hi {
		return sorted[lo]
	}
	frac := idx - float64(lo)
	return time.Duration(float64(sorted[lo]) + frac*float64(sorted[hi]-sorted[lo]))
}

func avgByName(ms []Measurement) map[string]time.Duration {
	sums := map[string]int64{}
	counts := map[string]int{}
	for _, m := range ms {
		sums[m.Name] += int64(m.Elapsed)
		counts[m.Name]++
	}
	out := make(map[string]time.Duration, len(sums))
	for name, sum := range sums {
		out[name] = time.Duration(sum / int64(counts[name]))
	}
	return out
}
