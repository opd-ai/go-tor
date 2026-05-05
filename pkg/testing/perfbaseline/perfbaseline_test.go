package perfbaseline_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/testing/perfbaseline"
)

func TestNew_DefaultThresholds(t *testing.T) {
	b := perfbaseline.New()
	if b == nil {
		t.Fatal("New returned nil")
	}
}

func TestRecordDuration(t *testing.T) {
	b := perfbaseline.New()
	b.RecordDuration("circuit_build", 100*time.Millisecond)
	b.RecordDuration("circuit_build", 200*time.Millisecond)

	snap := b.Snapshot()
	if len(snap.Measurements) != 2 {
		t.Errorf("expected 2 measurements, got %d", len(snap.Measurements))
	}
}

func TestMeasureFunc(t *testing.T) {
	b := perfbaseline.New()
	d := b.MeasureFunc("op", func() {
		time.Sleep(5 * time.Millisecond)
	})
	if d < 5*time.Millisecond {
		t.Errorf("MeasureFunc returned %v, expected >= 5ms", d)
	}
	snap := b.Snapshot()
	if len(snap.Measurements) != 1 {
		t.Errorf("expected 1 measurement, got %d", len(snap.Measurements))
	}
}

func TestCheckCircuitBuild_Pass(t *testing.T) {
	b := perfbaseline.New()
	// Record build times well under the 5s threshold
	for i := 0; i < 20; i++ {
		b.RecordDuration("circuit_build", 500*time.Millisecond)
	}
	if err := b.CheckCircuitBuild(); err != nil {
		t.Errorf("unexpected circuit build failure: %v", err)
	}
}

func TestCheckCircuitBuild_Empty(t *testing.T) {
	b := perfbaseline.New()
	// No measurements – should pass.
	if err := b.CheckCircuitBuild(); err != nil {
		t.Errorf("empty baseline should pass: %v", err)
	}
}

func TestCheckCircuitBuild_Fail(t *testing.T) {
	threshold := perfbaseline.Threshold{
		CircuitBuildP95:   500 * time.Millisecond,
		MemoryAllocMB:     50,
		ConcurrentStreams: 100,
	}
	b := perfbaseline.NewWithThresholds(threshold)
	// Record build times above 500ms threshold
	for i := 0; i < 20; i++ {
		b.RecordDuration("circuit_build", 2*time.Second)
	}
	if err := b.CheckCircuitBuild(); err == nil {
		t.Error("expected circuit build threshold violation")
	}
}

func TestCheckMemory_Pass(t *testing.T) {
	// Use an intentionally high threshold (500 MB) so this always passes
	// regardless of test environment heap size.
	b := perfbaseline.NewWithThresholds(perfbaseline.Threshold{
		CircuitBuildP95: 5 * time.Second,
		MemoryAllocMB:   500,
	})
	if err := b.CheckMemory(); err != nil {
		t.Errorf("unexpected memory check failure with 500 MB threshold: %v", err)
	}
}

func TestCheckMemory_Fail(t *testing.T) {
	// Use a 0.001 MB (1 KB) threshold so any real process will exceed it.
	b := perfbaseline.NewWithThresholds(perfbaseline.Threshold{
		CircuitBuildP95: 5 * time.Second,
		MemoryAllocMB:   0.001,
	})
	if err := b.CheckMemory(); err == nil {
		t.Error("expected memory check to fail with 0.001 MB threshold")
	}
}

func TestCheck_Combined(t *testing.T) {
	// Use a high memory threshold so Check always passes on circuit_build alone.
	b := perfbaseline.NewWithThresholds(perfbaseline.Threshold{
		CircuitBuildP95: 5 * time.Second,
		MemoryAllocMB:   500,
	})
	b.RecordDuration("circuit_build", 100*time.Millisecond)
	if err := b.Check(); err != nil {
		t.Errorf("unexpected Check failure: %v", err)
	}
}

func TestSnapshot_Fields(t *testing.T) {
	b := perfbaseline.New()
	b.RecordDuration("op_a", 10*time.Millisecond)
	b.RecordDuration("op_b", 20*time.Millisecond)

	snap := b.Snapshot()
	if snap.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if len(snap.Measurements) != 2 {
		t.Errorf("expected 2 measurements, got %d", len(snap.Measurements))
	}
	if snap.MemStats.SysMB <= 0 {
		t.Error("SysMB should be positive")
	}
	if snap.MemStats.NumGoroutine <= 0 {
		t.Error("NumGoroutine should be positive")
	}
}

func TestSaveLoadSnapshot(t *testing.T) {
	b := perfbaseline.New()
	b.RecordDuration("circuit_build", 300*time.Millisecond)
	b.RecordDuration("memory_check", 10*time.Millisecond)

	path := filepath.Join(t.TempDir(), "baseline.json")
	if err := b.SaveSnapshot(path); err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}

	// File should contain valid JSON.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var snap perfbaseline.Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if len(snap.Measurements) != 2 {
		t.Errorf("expected 2 measurements, got %d", len(snap.Measurements))
	}

	// Reload with LoadSnapshot.
	loaded, err := perfbaseline.LoadSnapshot(path)
	if err != nil {
		t.Fatalf("LoadSnapshot: %v", err)
	}
	if len(loaded.Measurements) != 2 {
		t.Errorf("loaded: expected 2 measurements, got %d", len(loaded.Measurements))
	}
}

func TestLoadSnapshot_Missing(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nonexistent-baseline.json")
	_, err := perfbaseline.LoadSnapshot(missing)
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestCompareSnapshots_NoRegression(t *testing.T) {
	base := perfbaseline.New()
	for i := 0; i < 10; i++ {
		base.RecordDuration("circuit_build", 300*time.Millisecond)
	}

	current := perfbaseline.New()
	for i := 0; i < 10; i++ {
		current.RecordDuration("circuit_build", 320*time.Millisecond) // ~6.7% increase
	}

	// Allow up to 10% regression.
	violations := perfbaseline.CompareSnapshots(base.Snapshot(), current.Snapshot(), 10.0)
	if len(violations) > 0 {
		t.Errorf("unexpected violations: %v", violations)
	}
}

func TestCompareSnapshots_Regression(t *testing.T) {
	base := perfbaseline.New()
	for i := 0; i < 10; i++ {
		base.RecordDuration("circuit_build", 300*time.Millisecond)
	}

	current := perfbaseline.New()
	for i := 0; i < 10; i++ {
		current.RecordDuration("circuit_build", 1*time.Second) // ~233% increase
	}

	violations := perfbaseline.CompareSnapshots(base.Snapshot(), current.Snapshot(), 10.0)
	if len(violations) == 0 {
		t.Error("expected regression violation but got none")
	}
}

func TestCompareSnapshots_MissingMetric(t *testing.T) {
	base := perfbaseline.New()
	base.RecordDuration("circuit_build", 300*time.Millisecond)

	current := perfbaseline.New()
	current.RecordDuration("other_op", 100*time.Millisecond)

	// No shared metric → no violations.
	violations := perfbaseline.CompareSnapshots(base.Snapshot(), current.Snapshot(), 0.0)
	if len(violations) > 0 {
		t.Errorf("unexpected violations for non-overlapping metrics: %v", violations)
	}
}

// TestReadmeThresholds verifies the project's stated README targets are encoded
// in the default Thresholds variable.
func TestReadmeThresholds(t *testing.T) {
	th := perfbaseline.Thresholds
	if th.CircuitBuildP95 != 5*time.Second {
		t.Errorf("CircuitBuildP95: got %v, want 5s (README)", th.CircuitBuildP95)
	}
	if th.MemoryAllocMB != 50 {
		t.Errorf("MemoryAllocMB: got %v, want 50 (README)", th.MemoryAllocMB)
	}
	if th.ConcurrentStreams < 100 {
		t.Errorf("ConcurrentStreams: got %d, want >= 100 (README)", th.ConcurrentStreams)
	}
}
