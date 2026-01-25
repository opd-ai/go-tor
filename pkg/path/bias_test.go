package path

import (
	"testing"
	"time"
)

func TestDefaultThresholds(t *testing.T) {
	thresholds := DefaultThresholds()

	if thresholds.UseSuccessMin != 20 {
		t.Errorf("expected UseSuccessMin=20, got %d", thresholds.UseSuccessMin)
	}
	if thresholds.UseSuccessRate != 0.7 {
		t.Errorf("expected UseSuccessRate=0.7, got %f", thresholds.UseSuccessRate)
	}
	if thresholds.BuildTimeoutCount != 3 {
		t.Errorf("expected BuildTimeoutCount=3, got %d", thresholds.BuildTimeoutCount)
	}
	if thresholds.SuccessCount != 20 {
		t.Errorf("expected SuccessCount=20, got %d", thresholds.SuccessCount)
	}
}

func TestCircuitOutcomeString(t *testing.T) {
	tests := []struct {
		outcome  CircuitOutcome
		expected string
	}{
		{OutcomeBuildSuccess, "BUILD_SUCCESS"},
		{OutcomeBuildTimeout, "BUILD_TIMEOUT"},
		{OutcomeBuildFailed, "BUILD_FAILED"},
		{OutcomeUseSuccess, "USE_SUCCESS"},
		{OutcomeUseFailed, "USE_FAILED"},
	}

	for _, tt := range tests {
		if got := tt.outcome.String(); got != tt.expected {
			t.Errorf("outcome %d: expected %q, got %q", tt.outcome, tt.expected, got)
		}
	}
}

func TestNewBiasDetector(t *testing.T) {
	thresholds := DefaultThresholds()
	bd := NewBiasDetector(thresholds)

	if bd == nil {
		t.Fatal("expected non-nil detector")
	}

	if bd.thresholds.UseSuccessMin != thresholds.UseSuccessMin {
		t.Error("thresholds not set correctly")
	}

	if len(bd.guardStats) != 0 {
		t.Errorf("expected empty guard stats, got %d", len(bd.guardStats))
	}

	if len(bd.alerts) != 0 {
		t.Errorf("expected empty alerts, got %d", len(bd.alerts))
	}
}

func TestRecordBuildSuccess(t *testing.T) {
	bd := NewBiasDetector(DefaultThresholds())

	fingerprint := "test-guard-fp"
	alerts := bd.RecordOutcome(1, fingerprint, OutcomeBuildSuccess)

	if len(alerts) != 0 {
		t.Errorf("expected no alerts, got %d", len(alerts))
	}

	stats, err := bd.GetGuardStats(fingerprint)
	if err != nil {
		t.Fatalf("failed to get stats: %v", err)
	}

	if stats.TotalBuilds != 1 {
		t.Errorf("expected TotalBuilds=1, got %d", stats.TotalBuilds)
	}
	if stats.BuildSuccesses != 1 {
		t.Errorf("expected BuildSuccesses=1, got %d", stats.BuildSuccesses)
	}
	if stats.ConsecutiveTimeouts != 0 {
		t.Errorf("expected ConsecutiveTimeouts=0, got %d", stats.ConsecutiveTimeouts)
	}
}

func TestRecordBuildTimeout(t *testing.T) {
	bd := NewBiasDetector(DefaultThresholds())

	fingerprint := "test-guard-fp"
	alerts := bd.RecordOutcome(1, fingerprint, OutcomeBuildTimeout)

	if len(alerts) != 0 {
		t.Errorf("expected no alerts for single timeout, got %d", len(alerts))
	}

	stats, err := bd.GetGuardStats(fingerprint)
	if err != nil {
		t.Fatalf("failed to get stats: %v", err)
	}

	if stats.TotalBuilds != 1 {
		t.Errorf("expected TotalBuilds=1, got %d", stats.TotalBuilds)
	}
	if stats.BuildTimeouts != 1 {
		t.Errorf("expected BuildTimeouts=1, got %d", stats.BuildTimeouts)
	}
	if stats.ConsecutiveTimeouts != 1 {
		t.Errorf("expected ConsecutiveTimeouts=1, got %d", stats.ConsecutiveTimeouts)
	}
}

func TestConsecutiveTimeoutAlert(t *testing.T) {
	bd := NewBiasDetector(DefaultThresholds())

	fingerprint := "test-guard-fp"

	// First timeout - no alert
	alerts := bd.RecordOutcome(1, fingerprint, OutcomeBuildTimeout)
	if len(alerts) != 0 {
		t.Errorf("expected no alerts for timeout 1, got %d", len(alerts))
	}

	// Second timeout - no alert
	alerts = bd.RecordOutcome(2, fingerprint, OutcomeBuildTimeout)
	if len(alerts) != 0 {
		t.Errorf("expected no alerts for timeout 2, got %d", len(alerts))
	}

	// Third timeout - should trigger alert
	alerts = bd.RecordOutcome(3, fingerprint, OutcomeBuildTimeout)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert for timeout 3, got %d", len(alerts))
	}

	alert := alerts[0]
	if alert.Type != "CONSECUTIVE_TIMEOUTS" {
		t.Errorf("expected alert type CONSECUTIVE_TIMEOUTS, got %s", alert.Type)
	}
	if alert.Fingerprint != fingerprint {
		t.Errorf("expected fingerprint %s, got %s", fingerprint, alert.Fingerprint)
	}

	// Success should reset counter
	alerts = bd.RecordOutcome(4, fingerprint, OutcomeBuildSuccess)
	if len(alerts) != 0 {
		t.Errorf("expected no alerts after success, got %d", len(alerts))
	}

	stats, _ := bd.GetGuardStats(fingerprint)
	if stats.ConsecutiveTimeouts != 0 {
		t.Errorf("expected ConsecutiveTimeouts=0 after success, got %d", stats.ConsecutiveTimeouts)
	}
}

func TestUseSuccessRateAlert(t *testing.T) {
	thresholds := BiasThresholds{
		UseSuccessMin:     5,
		UseSuccessRate:    0.7,
		BuildTimeoutCount: 3,
		SuccessCount:      20,
	}
	bd := NewBiasDetector(thresholds)

	fingerprint := "test-guard-fp"

	// Record 5 uses: 2 successes, 3 failures (40% success rate < 70% threshold)
	bd.RecordOutcome(1, fingerprint, OutcomeUseSuccess)
	bd.RecordOutcome(2, fingerprint, OutcomeUseFailed)
	bd.RecordOutcome(3, fingerprint, OutcomeUseFailed)
	bd.RecordOutcome(4, fingerprint, OutcomeUseSuccess)
	alerts := bd.RecordOutcome(5, fingerprint, OutcomeUseFailed)

	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert for low use success, got %d", len(alerts))
	}

	alert := alerts[0]
	if alert.Type != "LOW_USE_SUCCESS" {
		t.Errorf("expected alert type LOW_USE_SUCCESS, got %s", alert.Type)
	}

	stats := alert.Stats
	if stats.TotalUses != 5 {
		t.Errorf("expected TotalUses=5, got %d", stats.TotalUses)
	}
	if stats.UseSuccesses != 2 {
		t.Errorf("expected UseSuccesses=2, got %d", stats.UseSuccesses)
	}
	if stats.UseFailures != 3 {
		t.Errorf("expected UseFailures=3, got %d", stats.UseFailures)
	}
}

func TestBuildSuccessRateAlert(t *testing.T) {
	thresholds := BiasThresholds{
		UseSuccessMin:     5,
		UseSuccessRate:    0.7,
		BuildTimeoutCount: 10, // Set high to avoid timeout alerts
		SuccessCount:      20,
	}
	bd := NewBiasDetector(thresholds)

	fingerprint := "test-guard-fp"

	// Record 5 builds: 2 successes, 3 failures (40% success rate < 70% threshold)
	bd.RecordOutcome(1, fingerprint, OutcomeBuildSuccess)
	bd.RecordOutcome(2, fingerprint, OutcomeBuildFailed)
	bd.RecordOutcome(3, fingerprint, OutcomeBuildFailed)
	bd.RecordOutcome(4, fingerprint, OutcomeBuildSuccess)
	alerts := bd.RecordOutcome(5, fingerprint, OutcomeBuildFailed)

	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert for low build success, got %d", len(alerts))
	}

	alert := alerts[0]
	if alert.Type != "LOW_BUILD_SUCCESS" {
		t.Errorf("expected alert type LOW_BUILD_SUCCESS, got %s", alert.Type)
	}
}

func TestGetGuardStatsNonexistent(t *testing.T) {
	bd := NewBiasDetector(DefaultThresholds())

	_, err := bd.GetGuardStats("nonexistent-fp")
	if err == nil {
		t.Error("expected error for nonexistent guard, got nil")
	}
}

func TestGetAllGuardStats(t *testing.T) {
	bd := NewBiasDetector(DefaultThresholds())

	bd.RecordOutcome(1, "guard1", OutcomeBuildSuccess)
	bd.RecordOutcome(2, "guard2", OutcomeBuildSuccess)
	bd.RecordOutcome(3, "guard3", OutcomeBuildSuccess)

	allStats := bd.GetAllGuardStats()

	if len(allStats) != 3 {
		t.Errorf("expected 3 guards, got %d", len(allStats))
	}

	for _, fp := range []string{"guard1", "guard2", "guard3"} {
		if _, exists := allStats[fp]; !exists {
			t.Errorf("expected stats for %s", fp)
		}
	}
}

func TestGetAlerts(t *testing.T) {
	bd := NewBiasDetector(DefaultThresholds())

	fingerprint := "test-guard-fp"

	// Generate 3 consecutive timeout alerts
	bd.RecordOutcome(1, fingerprint, OutcomeBuildTimeout)
	bd.RecordOutcome(2, fingerprint, OutcomeBuildTimeout)
	bd.RecordOutcome(3, fingerprint, OutcomeBuildTimeout)

	alerts := bd.GetAlerts(10)
	if len(alerts) != 1 {
		t.Errorf("expected 1 alert, got %d", len(alerts))
	}

	// Test limit
	bd.RecordOutcome(4, fingerprint, OutcomeBuildTimeout)
	bd.RecordOutcome(5, fingerprint, OutcomeBuildTimeout)

	alerts = bd.GetAlerts(1)
	if len(alerts) != 1 {
		t.Errorf("expected 1 alert with limit, got %d", len(alerts))
	}
}

func TestClearGuard(t *testing.T) {
	bd := NewBiasDetector(DefaultThresholds())

	fingerprint := "test-guard-fp"
	bd.RecordOutcome(1, fingerprint, OutcomeBuildSuccess)

	stats, err := bd.GetGuardStats(fingerprint)
	if err != nil {
		t.Fatalf("expected stats before clear: %v", err)
	}
	if stats.TotalBuilds != 1 {
		t.Errorf("expected TotalBuilds=1, got %d", stats.TotalBuilds)
	}

	bd.ClearGuard(fingerprint)

	_, err = bd.GetGuardStats(fingerprint)
	if err == nil {
		t.Error("expected error after clear, got nil")
	}
}

func TestReset(t *testing.T) {
	bd := NewBiasDetector(DefaultThresholds())

	bd.RecordOutcome(1, "guard1", OutcomeBuildSuccess)
	bd.RecordOutcome(2, "guard2", OutcomeBuildSuccess)
	bd.RecordOutcome(3, "guard1", OutcomeBuildTimeout)
	bd.RecordOutcome(4, "guard1", OutcomeBuildTimeout)
	bd.RecordOutcome(5, "guard1", OutcomeBuildTimeout)

	allStats := bd.GetAllGuardStats()
	if len(allStats) != 2 {
		t.Errorf("expected 2 guards before reset, got %d", len(allStats))
	}

	alerts := bd.GetAlerts(10)
	if len(alerts) == 0 {
		t.Error("expected alerts before reset")
	}

	bd.Reset()

	allStats = bd.GetAllGuardStats()
	if len(allStats) != 0 {
		t.Errorf("expected 0 guards after reset, got %d", len(allStats))
	}

	alerts = bd.GetAlerts(10)
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts after reset, got %d", len(alerts))
	}
}

func TestBiasGetStats(t *testing.T) {
	bd := NewBiasDetector(DefaultThresholds())

	bd.RecordOutcome(1, "guard1", OutcomeBuildSuccess)
	bd.RecordOutcome(2, "guard1", OutcomeBuildTimeout)
	bd.RecordOutcome(3, "guard2", OutcomeBuildFailed)
	bd.RecordOutcome(4, "guard1", OutcomeUseSuccess)
	bd.RecordOutcome(5, "guard2", OutcomeUseFailed)

	stats := bd.GetStats()

	if stats.TotalGuards != 2 {
		t.Errorf("expected TotalGuards=2, got %d", stats.TotalGuards)
	}
	if stats.TotalBuilds != 3 {
		t.Errorf("expected TotalBuilds=3, got %d", stats.TotalBuilds)
	}
	if stats.BuildSuccesses != 1 {
		t.Errorf("expected BuildSuccesses=1, got %d", stats.BuildSuccesses)
	}
	if stats.BuildTimeouts != 1 {
		t.Errorf("expected BuildTimeouts=1, got %d", stats.BuildTimeouts)
	}
	if stats.BuildFailures != 1 {
		t.Errorf("expected BuildFailures=1, got %d", stats.BuildFailures)
	}
	if stats.TotalUses != 2 {
		t.Errorf("expected TotalUses=2, got %d", stats.TotalUses)
	}
	if stats.UseSuccesses != 1 {
		t.Errorf("expected UseSuccesses=1, got %d", stats.UseSuccesses)
	}
	if stats.UseFailures != 1 {
		t.Errorf("expected UseFailures=1, got %d", stats.UseFailures)
	}
}

func TestIsBiased(t *testing.T) {
	thresholds := BiasThresholds{
		UseSuccessMin:     5, // Require 5 samples before checking rates
		UseSuccessRate:    0.7,
		BuildTimeoutCount: 2,
		SuccessCount:      20,
	}
	bd := NewBiasDetector(thresholds)

	fingerprint := "test-guard-fp"

	// Initially not biased (no data)
	if bd.IsBiased(fingerprint) {
		t.Error("expected not biased initially")
	}

	// After 2 consecutive timeouts, should be biased
	bd.RecordOutcome(1, fingerprint, OutcomeBuildTimeout)
	bd.RecordOutcome(2, fingerprint, OutcomeBuildTimeout)

	if !bd.IsBiased(fingerprint) {
		t.Error("expected biased after 2 consecutive timeouts")
	}

	// Success should clear consecutive timeout counter
	// Now we have 1 success out of 3 builds, but threshold is 5, so rate not checked yet
	bd.RecordOutcome(3, fingerprint, OutcomeBuildSuccess)

	if bd.IsBiased(fingerprint) {
		t.Error("expected not biased after success (consecutive timeouts cleared, not enough samples for rate)")
	}

	// Add more successes to get above minimum sample size with good rate
	bd.RecordOutcome(7, fingerprint, OutcomeBuildSuccess)
	bd.RecordOutcome(8, fingerprint, OutcomeBuildSuccess)
	bd.RecordOutcome(9, fingerprint, OutcomeBuildSuccess)

	// Now we have 4 successes out of 6 builds (67%), just below 70% threshold
	// This should trigger bias
	if !bd.IsBiased(fingerprint) {
		t.Error("expected biased due to low build success rate (67% < 70%)")
	}

	// Add one more success to get above threshold
	bd.RecordOutcome(10, fingerprint, OutcomeBuildSuccess)

	// Now we have 5 successes out of 7 builds (71%), above 70%
	if bd.IsBiased(fingerprint) {
		t.Error("expected not biased with good build rate")
	}

	// Low use success rate should cause bias (1 success, 4 failures = 20% < 70%)
	bd.RecordOutcome(11, fingerprint, OutcomeUseSuccess)
	bd.RecordOutcome(12, fingerprint, OutcomeUseFailed)
	bd.RecordOutcome(13, fingerprint, OutcomeUseFailed)
	bd.RecordOutcome(14, fingerprint, OutcomeUseFailed)
	bd.RecordOutcome(15, fingerprint, OutcomeUseFailed)

	if !bd.IsBiased(fingerprint) {
		t.Error("expected biased due to low use success rate")
	}
}

func TestRecordRingBuffer(t *testing.T) {
	thresholds := BiasThresholds{
		UseSuccessMin:     3,
		UseSuccessRate:    0.7,
		BuildTimeoutCount: 3,
		SuccessCount:      5, // Small buffer for testing
	}
	bd := NewBiasDetector(thresholds)

	fingerprint := "test-guard-fp"

	// Add more records than buffer size
	for i := 1; i <= 10; i++ {
		bd.RecordOutcome(uint32(i), fingerprint, OutcomeBuildSuccess)
	}

	stats := bd.GetStats()
	if stats.RecordCount != 5 {
		t.Errorf("expected RecordCount=5 (buffer size), got %d", stats.RecordCount)
	}
}

func TestAlertRingBuffer(t *testing.T) {
	bd := NewBiasDetector(DefaultThresholds())
	bd.maxAlerts = 2 // Set small max for testing

	fingerprint := "test-guard-fp"

	// Generate 3 alerts
	bd.RecordOutcome(1, fingerprint, OutcomeBuildTimeout)
	bd.RecordOutcome(2, fingerprint, OutcomeBuildTimeout)
	bd.RecordOutcome(3, fingerprint, OutcomeBuildTimeout)

	bd.RecordOutcome(4, fingerprint, OutcomeBuildSuccess)

	bd.RecordOutcome(5, fingerprint, OutcomeBuildTimeout)
	bd.RecordOutcome(6, fingerprint, OutcomeBuildTimeout)
	bd.RecordOutcome(7, fingerprint, OutcomeBuildTimeout)

	alerts := bd.GetAlerts(10)
	if len(alerts) != 2 {
		t.Errorf("expected 2 alerts (max buffer), got %d", len(alerts))
	}
}

func TestGuardStatsTimestamp(t *testing.T) {
	bd := NewBiasDetector(DefaultThresholds())

	fingerprint := "test-guard-fp"

	before := time.Now()
	bd.RecordOutcome(1, fingerprint, OutcomeBuildSuccess)
	after := time.Now()

	stats, _ := bd.GetGuardStats(fingerprint)

	if stats.LastSeen.Before(before) || stats.LastSeen.After(after) {
		t.Errorf("LastSeen timestamp not in expected range")
	}
}

func TestMultipleGuardsIsolation(t *testing.T) {
	bd := NewBiasDetector(DefaultThresholds())

	// Record different outcomes for different guards
	bd.RecordOutcome(1, "guard1", OutcomeBuildSuccess)
	bd.RecordOutcome(2, "guard2", OutcomeBuildTimeout)
	bd.RecordOutcome(3, "guard3", OutcomeBuildFailed)

	stats1, _ := bd.GetGuardStats("guard1")
	stats2, _ := bd.GetGuardStats("guard2")
	stats3, _ := bd.GetGuardStats("guard3")

	if stats1.BuildSuccesses != 1 || stats1.BuildTimeouts != 0 {
		t.Error("guard1 stats incorrect")
	}
	if stats2.BuildTimeouts != 1 || stats2.BuildSuccesses != 0 {
		t.Error("guard2 stats incorrect")
	}
	if stats3.BuildFailures != 1 || stats3.BuildSuccesses != 0 {
		t.Error("guard3 stats incorrect")
	}
}
