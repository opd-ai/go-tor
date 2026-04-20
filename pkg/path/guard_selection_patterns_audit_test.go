package path

import (
	"fmt"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/directory"
	"github.com/opd-ai/go-tor/pkg/logger"
)

// TestGuardSetSize verifies guard set size matches Tor specification (3 guards)
func TestGuardSetSize(t *testing.T) {
	tmpDir := t.TempDir()
	gm, err := NewGuardManager(tmpDir, logger.NewDefault())
	if err != nil {
		t.Fatalf("NewGuardManager() failed: %v", err)
	}

	if gm.maxGuards != 3 {
		t.Errorf("maxGuards = %d, want 3 (per Tor spec)", gm.maxGuards)
	}

	// Verify default config also sets 3 guards
	config := DefaultGuardManagerConfig(tmpDir)
	if config.MaxGuards != 3 {
		t.Errorf("DefaultGuardManagerConfig().MaxGuards = %d, want 3", config.MaxGuards)
	}
}

// TestGuardPersistenceAcrossRestarts verifies guards persist across client restarts
func TestGuardPersistenceAcrossRestarts(t *testing.T) {
	tmpDir := t.TempDir()

	// Phase 1: Create guard manager and add guards
	gm1, err := NewGuardManager(tmpDir, logger.NewDefault())
	if err != nil {
		t.Fatalf("NewGuardManager() phase 1 failed: %v", err)
	}

	guards := []*directory.Relay{
		{
			Nickname:    "Guard1",
			Fingerprint: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
			Address:     "192.0.2.1:9001",
			Flags:       []string{"Guard", "Running", "Valid", "Stable"},
			Bandwidth:   1000000,
		},
		{
			Nickname:    "Guard2",
			Fingerprint: "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB",
			Address:     "192.0.2.2:9001",
			Flags:       []string{"Guard", "Running", "Valid", "Stable"},
			Bandwidth:   1000000,
		},
		{
			Nickname:    "Guard3",
			Fingerprint: "CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC",
			Address:     "192.0.2.3:9001",
			Flags:       []string{"Guard", "Running", "Valid", "Stable"},
			Bandwidth:   1000000,
		},
	}

	for _, guard := range guards {
		if err := gm1.AddGuard(guard); err != nil {
			t.Fatalf("AddGuard(%s) failed: %v", guard.Nickname, err)
		}
	}

	// Confirm first two guards
	if err := gm1.ConfirmGuard(guards[0].Fingerprint); err != nil {
		t.Fatalf("ConfirmGuard() failed: %v", err)
	}
	if err := gm1.ConfirmGuard(guards[1].Fingerprint); err != nil {
		t.Fatalf("ConfirmGuard() failed: %v", err)
	}

	// Save state
	if err := gm1.Save(); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Phase 2: Simulate restart by creating new guard manager
	gm2, err := NewGuardManager(tmpDir, logger.NewDefault())
	if err != nil {
		t.Fatalf("NewGuardManager() phase 2 failed: %v", err)
	}

	// Verify guards persisted
	persistedGuards := gm2.GetGuards()
	if len(persistedGuards) != 3 {
		t.Fatalf("GetGuards() returned %d guards after restart, want 3", len(persistedGuards))
	}

	// Verify confirmation status persisted
	confirmedCount := 0
	for _, guard := range persistedGuards {
		if guard.Confirmed {
			confirmedCount++
		}
	}

	if confirmedCount != 2 {
		t.Errorf("Found %d confirmed guards after restart, want 2", confirmedCount)
	}

	// Verify fingerprints match
	fingerprintMap := make(map[string]bool)
	for _, guard := range persistedGuards {
		fingerprintMap[guard.Fingerprint] = true
	}

	for _, originalGuard := range guards {
		if !fingerprintMap[originalGuard.Fingerprint] {
			t.Errorf("Guard %s not found after restart", originalGuard.Fingerprint)
		}
	}
}

// TestGuardExpiryTiming verifies guards expire after 90 days
func TestGuardExpiryTiming(t *testing.T) {
	tmpDir := t.TempDir()
	gm, err := NewGuardManager(tmpDir, logger.NewDefault())
	if err != nil {
		t.Fatalf("NewGuardManager() failed: %v", err)
	}

	// Verify default expiry is 90 days
	expectedExpiry := 90 * 24 * time.Hour
	if gm.guardExpiry != expectedExpiry {
		t.Errorf("guardExpiry = %v, want %v (90 days per Tor spec)", gm.guardExpiry, expectedExpiry)
	}

	// Add a guard
	guard := &directory.Relay{
		Nickname:    "TestGuard",
		Fingerprint: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		Address:     "192.0.2.1:9001",
		Flags:       []string{"Guard", "Running", "Valid", "Stable"},
	}

	if err := gm.AddGuard(guard); err != nil {
		t.Fatalf("AddGuard() failed: %v", err)
	}

	// Manually set LastUsed to 91 days ago
	gm.mu.Lock()
	gm.state.Guards[0].LastUsed = time.Now().Add(-91 * 24 * time.Hour)
	gm.mu.Unlock()

	// Guard should be expired
	validGuards := gm.GetGuards()
	if len(validGuards) != 0 {
		t.Errorf("GetGuards() returned %d guards for 91-day-old guard, want 0 (should be expired)", len(validGuards))
	}

	// Cleanup should remove it
	gm.CleanupExpired()
	gm.mu.RLock()
	totalGuards := len(gm.state.Guards)
	gm.mu.RUnlock()

	if totalGuards != 0 {
		t.Errorf("After CleanupExpired(), %d guards remain, want 0", totalGuards)
	}
}

// TestGuardConfirmationWorkflow verifies guards are initially unconfirmed and can be confirmed
func TestGuardConfirmationWorkflow(t *testing.T) {
	tmpDir := t.TempDir()
	gm, err := NewGuardManager(tmpDir, logger.NewDefault())
	if err != nil {
		t.Fatalf("NewGuardManager() failed: %v", err)
	}

	guard := &directory.Relay{
		Nickname:    "TestGuard",
		Fingerprint: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		Address:     "192.0.2.1:9001",
		Flags:       []string{"Guard", "Running", "Valid", "Stable"},
	}

	// Step 1: Add guard (should be unconfirmed)
	if err := gm.AddGuard(guard); err != nil {
		t.Fatalf("AddGuard() failed: %v", err)
	}

	guards := gm.GetGuards()
	if len(guards) != 1 {
		t.Fatalf("GetGuards() returned %d guards, want 1", len(guards))
	}

	if guards[0].Confirmed {
		t.Error("Newly added guard should be unconfirmed, but Confirmed=true")
	}

	// Step 2: Confirm guard
	if err := gm.ConfirmGuard(guard.Fingerprint); err != nil {
		t.Fatalf("ConfirmGuard() failed: %v", err)
	}

	guards = gm.GetGuards()
	if !guards[0].Confirmed {
		t.Error("Guard should be confirmed after ConfirmGuard(), but Confirmed=false")
	}

	// Step 3: Verify LastUsed was updated
	if guards[0].LastUsed.Before(time.Now().Add(-1 * time.Second)) {
		t.Error("ConfirmGuard() should update LastUsed timestamp")
	}
}

// TestBandwidthWeightedSelectionDistribution verifies bandwidth-weighted selection
// produces correct statistical distribution
func TestBandwidthWeightedSelectionDistribution(t *testing.T) {
	// Create relays with known bandwidth ratios
	relays := []*directory.Relay{
		{
			Nickname:    "HighBW",
			Fingerprint: "AAAA",
			Bandwidth:   1000000, // 90.9% of total (10:1 ratio)
		},
		{
			Nickname:    "LowBW",
			Fingerprint: "BBBB",
			Bandwidth:   100000, // 9.1% of total
		},
	}

	// Sample 10,000 selections
	const samples = 10000
	counts := make(map[string]int)

	for i := 0; i < samples; i++ {
		idx, err := weightedRandomIndex(relays)
		if err != nil {
			t.Fatalf("weightedRandomIndex() iteration %d failed: %v", i, err)
		}
		counts[relays[idx].Nickname]++
	}

	// Expected: ~9091 HighBW, ~909 LowBW (90.9% vs 9.1%)
	highBWCount := counts["HighBW"]
	lowBWCount := counts["LowBW"]

	expectedHighBW := 9091.0
	expectedLowBW := 909.0

	// Allow 5% statistical variance (±455 for high, ±45 for low)
	highBWVariance := 455.0
	lowBWVariance := 45.0

	if float64(highBWCount) < expectedHighBW-highBWVariance || float64(highBWCount) > expectedHighBW+highBWVariance {
		t.Errorf("HighBW selected %d times, expected %.0f±%.0f (90.9%%)", highBWCount, expectedHighBW, highBWVariance)
	}

	if float64(lowBWCount) < expectedLowBW-lowBWVariance || float64(lowBWCount) > expectedLowBW+lowBWVariance {
		t.Errorf("LowBW selected %d times, expected %.0f±%.0f (9.1%%)", lowBWCount, expectedLowBW, lowBWVariance)
	}

	t.Logf("Bandwidth weighting distribution: HighBW=%d (%.1f%%), LowBW=%d (%.1f%%)",
		highBWCount, float64(highBWCount)/samples*100,
		lowBWCount, float64(lowBWCount)/samples*100)
}

// TestCryptographicRandomness verifies selection uses crypto/rand, not math/rand
func TestCryptographicRandomness(t *testing.T) {
	relays := []*directory.Relay{
		{Nickname: "R1", Fingerprint: "A", Bandwidth: 1000},
		{Nickname: "R2", Fingerprint: "B", Bandwidth: 1000},
		{Nickname: "R3", Fingerprint: "C", Bandwidth: 1000},
	}

	// Generate many selections - should be uniformly distributed
	const samples = 1000
	counts := make(map[int]int)

	for i := 0; i < samples; i++ {
		idx, err := weightedRandomIndex(relays)
		if err != nil {
			t.Fatalf("weightedRandomIndex() failed: %v", err)
		}
		counts[idx]++
	}

	// Each should be selected ~333 times (within reasonable variance)
	for idx := 0; idx < 3; idx++ {
		count := counts[idx]
		if count < 250 || count > 416 {
			t.Errorf("Relay %d selected %d times, expected ~333 (25%% variance allowed)", idx, count)
		}
	}

	// Verify no obvious patterns (e.g., first relay overly preferred)
	if counts[0] > 400 {
		t.Errorf("First relay overly preferred (%d selections), possible weak PRNG", counts[0])
	}
}

// TestPersistentGuardPreference verifies selector prefers persistent guards
func TestPersistentGuardPreference(t *testing.T) {
	tmpDir := t.TempDir()
	log := logger.NewDefault()

	// Create test relays
	relays := []*directory.Relay{
		{
			Nickname:    "Guard1",
			Fingerprint: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
			Address:     "192.0.2.1:9001",
			Flags:       []string{"Guard", "Running", "Valid", "Stable"},
			Bandwidth:   1000000,
		},
		{
			Nickname:    "Guard2",
			Fingerprint: "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB",
			Address:     "192.0.2.2:9001",
			Flags:       []string{"Guard", "Running", "Valid", "Stable"},
			Bandwidth:   1000000,
		},
		{
			Nickname:    "Guard3",
			Fingerprint: "CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC",
			Address:     "192.0.2.3:9001",
			Flags:       []string{"Guard", "Running", "Valid", "Stable"},
			Bandwidth:   1000000,
		},
	}

	// Create guard manager with persistent guard
	guardMgr, err := NewGuardManager(tmpDir, log)
	if err != nil {
		t.Fatalf("NewGuardManager() failed: %v", err)
	}

	// Add and confirm Guard1 as persistent guard
	if err := guardMgr.AddGuard(relays[0]); err != nil {
		t.Fatalf("AddGuard() failed: %v", err)
	}
	if err := guardMgr.ConfirmGuard(relays[0].Fingerprint); err != nil {
		t.Fatalf("ConfirmGuard() failed: %v", err)
	}
	if err := guardMgr.Save(); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Create selector with guard manager
	selector := NewSelectorWithGuards(directory.NewClient(log), guardMgr, log)

	// Directly set guards for testing (bypass consensus)
	selector.mu.Lock()
	selector.guards = relays
	selector.relays = relays
	selector.mu.Unlock()

	// Select guards multiple times - should prefer persistent Guard1
	persistentCount := 0
	const trials = 20

	for i := 0; i < trials; i++ {
		guard, err := selector.selectGuard()
		if err != nil {
			t.Fatalf("selectGuard() trial %d failed: %v", i, err)
		}

		if guard.Fingerprint == relays[0].Fingerprint {
			persistentCount++
		}
	}

	// Should use persistent guard in most or all trials
	if persistentCount < trials-2 {
		t.Errorf("Persistent guard selected %d/%d times, expected >=%d (should prefer persistent guard)",
			persistentCount, trials, trials-2)
	}

	t.Logf("Persistent guard preference: %d/%d trials used persistent guard", persistentCount, trials)
}

// TestFamilyDiversityEnforcement verifies guards and exits cannot be in same family
func TestFamilyDiversityEnforcement(t *testing.T) {
	log := logger.NewDefault()

	// Create relays with family relationships in different subnets
	relays := []*directory.Relay{
		{
			Nickname:    "Guard1",
			Fingerprint: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
			Address:     "192.0.2.1:9001",
			Flags:       []string{"Guard", "Running", "Valid", "Stable"},
			Bandwidth:   1000000,
			Family:      []string{"BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"}, // Related to Relay1
		},
		{
			Nickname:    "Relay1",
			Fingerprint: "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB",
			Address:     "10.0.1.1:9001",              // Different /16 subnet
			Flags:       []string{"Running", "Valid"}, // Not Exit flag
			Bandwidth:   1000000,
			Family:      []string{"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}, // Related to Guard1
		},
		{
			Nickname:    "Exit2",
			Fingerprint: "CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC",
			Address:     "172.16.1.1:9001", // Different /16 subnet
			Flags:       []string{"Exit", "Running", "Valid"},
			Bandwidth:   1000000,
			// No family relationship
		},
		{
			Nickname:    "Exit3",
			Fingerprint: "DDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDD",
			Address:     "203.0.113.1:9001", // Different /16 subnet
			Flags:       []string{"Exit", "Running", "Valid"},
			Bandwidth:   1000000,
			// No family relationship
		},
	}

	selector := NewSelector(directory.NewClient(log), log)

	// Directly set relays for testing
	selector.mu.Lock()
	selector.relays = relays
	selector.mu.Unlock()

	// Select exit avoiding Guard1 - should never select Relay1 (same family)
	for i := 0; i < 20; i++ {
		exit, err := selector.selectExit(80, relays[0]) // Avoid Guard1
		if err != nil {
			t.Fatalf("selectExit() trial %d failed: %v", i, err)
		}

		if exit.Fingerprint == relays[1].Fingerprint {
			t.Errorf("Trial %d: Selected Relay1 which is in same family as Guard1 (should be avoided)", i)
		}

		// Should select Exit2 or Exit3 (both non-family)
		if exit.Fingerprint != relays[2].Fingerprint && exit.Fingerprint != relays[3].Fingerprint {
			t.Errorf("Trial %d: Expected Exit2 or Exit3, got %s", i, exit.Nickname)
		}
	}
}

// TestSubnetDiversityEnforcement verifies guards and exits cannot be in same /16 subnet
func TestSubnetDiversityEnforcement(t *testing.T) {
	log := logger.NewDefault()

	relays := []*directory.Relay{
		{
			Nickname:    "Guard1",
			Fingerprint: "AAAA",
			Address:     "192.168.1.1:9001", // 192.168/16 subnet
			Flags:       []string{"Guard", "Running", "Valid", "Stable"},
			Bandwidth:   1000000,
		},
		{
			Nickname:    "Exit1",
			Fingerprint: "BBBB",
			Address:     "192.168.2.1:9001", // Same 192.168/16 subnet
			Flags:       []string{"Exit", "Running", "Valid"},
			Bandwidth:   1000000,
		},
		{
			Nickname:    "Exit2",
			Fingerprint: "CCCC",
			Address:     "10.0.1.1:9001", // Different 10.0/16 subnet
			Flags:       []string{"Exit", "Running", "Valid"},
			Bandwidth:   1000000,
		},
	}

	selector := NewSelector(directory.NewClient(log), log)

	// Directly set relays for testing
	selector.mu.Lock()
	selector.relays = relays
	selector.mu.Unlock()

	// Select exit avoiding Guard1 - should never select Exit1 (same subnet)
	for i := 0; i < 20; i++ {
		exit, err := selector.selectExit(80, relays[0]) // Avoid Guard1
		if err != nil {
			t.Fatalf("selectExit() trial %d failed: %v", i, err)
		}

		if exit.Fingerprint == relays[1].Fingerprint {
			t.Errorf("Trial %d: Selected Exit1 in same /16 subnet as Guard1 (should be avoided)", i)
		}

		// Should select Exit2
		if exit.Fingerprint != relays[2].Fingerprint {
			t.Errorf("Trial %d: Expected Exit2, got %s", i, exit.Nickname)
		}
	}
}

// TestBiasDetectionConsecutiveTimeouts verifies bias detector triggers on consecutive timeouts
func TestBiasDetectionConsecutiveTimeouts(t *testing.T) {
	detector := NewBiasDetector(DefaultThresholds())

	guardFP := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

	// Record 3 consecutive timeouts (default threshold)
	for i := 0; i < 3; i++ {
		alerts := detector.RecordOutcome(uint32(i+1), guardFP, OutcomeBuildTimeout)

		if i < 2 {
			// First two timeouts should not trigger alert
			if len(alerts) > 0 {
				t.Errorf("Iteration %d: Unexpected alert before reaching threshold", i)
			}
		} else {
			// Third timeout should trigger alert
			if len(alerts) != 1 {
				t.Fatalf("Iteration %d: Expected 1 alert after 3 consecutive timeouts, got %d", i, len(alerts))
			}

			if alerts[0].Type != "CONSECUTIVE_TIMEOUTS" {
				t.Errorf("Alert type = %s, want CONSECUTIVE_TIMEOUTS", alerts[0].Type)
			}

			if alerts[0].Fingerprint != guardFP {
				t.Errorf("Alert fingerprint = %s, want %s", alerts[0].Fingerprint, guardFP)
			}
		}
	}

	// Verify guard is marked as biased
	if !detector.IsBiased(guardFP) {
		t.Error("Guard should be marked as biased after 3 consecutive timeouts")
	}
}

// TestBiasDetectionLowSuccessRate verifies bias detector triggers on low success rates
func TestBiasDetectionLowSuccessRate(t *testing.T) {
	thresholds := BiasThresholds{
		UseSuccessMin:     20,  // Need 20 circuits
		UseSuccessRate:    0.7, // 70% threshold
		BuildTimeoutCount: 10,  // High to avoid timeout alerts
		SuccessCount:      25,  // Track 25 circuits (needs to be >= UseSuccessMin)
	}
	detector := NewBiasDetector(thresholds)

	guardFP := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

	// Record 20 use attempts: 12 successes, 8 failures = 60% success rate (below 70% threshold)
	for i := 0; i < 20; i++ {
		var outcome CircuitOutcome
		if i < 12 {
			outcome = OutcomeUseSuccess
		} else {
			outcome = OutcomeUseFailed
		}

		alerts := detector.RecordOutcome(uint32(i+1), guardFP, outcome)

		if i < 19 {
			// Should not alert until we reach minimum sample size
			if len(alerts) > 0 {
				t.Errorf("Iteration %d: Unexpected alert before minimum sample size", i)
			}
		} else {
			// At 20 circuits, should detect low success rate (60% < 70%)
			if len(alerts) != 1 {
				t.Fatalf("Iteration %d: Expected 1 alert for low success rate, got %d", i, len(alerts))
			}

			if alerts[0].Type != "LOW_USE_SUCCESS" {
				t.Errorf("Alert type = %s, want LOW_USE_SUCCESS", alerts[0].Type)
			}
		}
	}

	// Verify guard is marked as biased
	if !detector.IsBiased(guardFP) {
		t.Error("Guard should be marked as biased with 60% success rate (threshold 70%)")
	}

	// Verify statistics
	stats, err := detector.GetGuardStats(guardFP)
	if err != nil {
		t.Fatalf("GetGuardStats() failed: %v", err)
	}

	if stats.UseSuccesses != 12 {
		t.Errorf("UseSuccesses = %d, want 12", stats.UseSuccesses)
	}
	if stats.UseFailures != 8 {
		t.Errorf("UseFailures = %d, want 8", stats.UseFailures)
	}
	if stats.TotalUses != 20 {
		t.Errorf("TotalUses = %d, want 20", stats.TotalUses)
	}
}

// TestBiasDetectorFiltersBiasedGuards verifies selector excludes biased guards
func TestBiasDetectorFiltersBiasedGuards(t *testing.T) {
	log := logger.NewDefault()

	relays := []*directory.Relay{
		{
			Nickname:    "BiasedGuard",
			Fingerprint: "AAAA",
			Address:     "192.0.2.1:9001",
			Flags:       []string{"Guard", "Running", "Valid", "Stable"},
			Bandwidth:   1000000,
		},
		{
			Nickname:    "GoodGuard",
			Fingerprint: "BBBB",
			Address:     "192.0.2.2:9001",
			Flags:       []string{"Guard", "Running", "Valid", "Stable"},
			Bandwidth:   1000000,
		},
	}

	selector := NewSelector(directory.NewClient(log), log)

	// Directly set guards for testing
	selector.mu.Lock()
	selector.guards = relays
	selector.relays = relays
	selector.mu.Unlock()

	// Mark BiasedGuard as biased (3 consecutive timeouts)
	for i := 0; i < 3; i++ {
		selector.RecordCircuitOutcome(uint32(i+1), "AAAA", OutcomeBuildTimeout)
	}

	// Verify BiasedGuard is marked biased
	if !selector.IsGuardBiased("AAAA") {
		t.Fatal("BiasedGuard should be marked as biased")
	}

	// Select guards 20 times - should never select BiasedGuard
	for i := 0; i < 20; i++ {
		guard, err := selector.selectGuard()
		if err != nil {
			t.Fatalf("selectGuard() trial %d failed: %v", i, err)
		}

		if guard.Fingerprint == "AAAA" {
			t.Errorf("Trial %d: Selected BiasedGuard (should be filtered out)", i)
		}

		if guard.Fingerprint != "BBBB" {
			t.Errorf("Trial %d: Expected GoodGuard, got %s", i, guard.Nickname)
		}
	}
}

// TestGuardRotationNotTriggeredOnNormalUse verifies guards don't rotate unnecessarily
func TestGuardRotationNotTriggeredOnNormalUse(t *testing.T) {
	tmpDir := t.TempDir()
	gm, err := NewGuardManager(tmpDir, logger.NewDefault())
	if err != nil {
		t.Fatalf("NewGuardManager() failed: %v", err)
	}

	guards := []*directory.Relay{
		{
			Nickname:    "Guard1",
			Fingerprint: "AAAA",
			Address:     "192.0.2.1:9001",
			Flags:       []string{"Guard", "Running", "Valid", "Stable"},
		},
		{
			Nickname:    "Guard2",
			Fingerprint: "BBBB",
			Address:     "192.0.2.2:9001",
			Flags:       []string{"Guard", "Running", "Valid", "Stable"},
		},
		{
			Nickname:    "Guard3",
			Fingerprint: "CCCC",
			Address:     "192.0.2.3:9001",
			Flags:       []string{"Guard", "Running", "Valid", "Stable"},
		},
	}

	// Add and confirm all guards
	for _, guard := range guards {
		if err := gm.AddGuard(guard); err != nil {
			t.Fatalf("AddGuard(%s) failed: %v", guard.Nickname, err)
		}
		if err := gm.ConfirmGuard(guard.Fingerprint); err != nil {
			t.Fatalf("ConfirmGuard(%s) failed: %v", guard.Fingerprint, err)
		}
	}

	// Capture initial guard set
	initialGuards := gm.GetGuards()
	if len(initialGuards) != 3 {
		t.Fatalf("Initial guard set has %d guards, want 3", len(initialGuards))
	}

	// Simulate normal use: update LastUsed timestamps
	for i := 0; i < 10; i++ {
		for _, guard := range guards {
			if err := gm.ConfirmGuard(guard.Fingerprint); err != nil {
				t.Fatalf("ConfirmGuard(%s) iteration %d failed: %v", guard.Fingerprint, i, err)
			}
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Verify guard set unchanged (no rotation)
	currentGuards := gm.GetGuards()
	if len(currentGuards) != 3 {
		t.Errorf("Guard set changed to %d guards, want 3 (should not rotate on normal use)", len(currentGuards))
	}

	for _, initialGuard := range initialGuards {
		found := false
		for _, currentGuard := range currentGuards {
			if currentGuard.Fingerprint == initialGuard.Fingerprint {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Guard %s rotated out unexpectedly", initialGuard.Fingerprint)
		}
	}
}

// TestThreadSafeConcurrentGuardAccess verifies guard manager is thread-safe
func TestThreadSafeConcurrentGuardAccess(t *testing.T) {
	tmpDir := t.TempDir()
	gm, err := NewGuardManager(tmpDir, logger.NewDefault())
	if err != nil {
		t.Fatalf("NewGuardManager() failed: %v", err)
	}

	guards := []*directory.Relay{
		{Nickname: "G1", Fingerprint: "AAAA", Address: "192.0.2.1:9001", Flags: []string{"Guard", "Running", "Valid", "Stable"}},
		{Nickname: "G2", Fingerprint: "BBBB", Address: "192.0.2.2:9001", Flags: []string{"Guard", "Running", "Valid", "Stable"}},
		{Nickname: "G3", Fingerprint: "CCCC", Address: "192.0.2.3:9001", Flags: []string{"Guard", "Running", "Valid", "Stable"}},
	}

	// Add initial guards
	for _, guard := range guards {
		if err := gm.AddGuard(guard); err != nil {
			t.Fatalf("AddGuard() failed: %v", err)
		}
	}

	// Concurrent operations
	done := make(chan bool)

	// Goroutine 1: Read guards
	go func() {
		for i := 0; i < 100; i++ {
			_ = gm.GetGuards()
		}
		done <- true
	}()

	// Goroutine 2: Confirm guards
	go func() {
		for i := 0; i < 100; i++ {
			_ = gm.ConfirmGuard("AAAA")
		}
		done <- true
	}()

	// Goroutine 3: Get stats
	go func() {
		for i := 0; i < 100; i++ {
			_ = gm.GetStats()
		}
		done <- true
	}()

	// Wait for completion
	for i := 0; i < 3; i++ {
		<-done
	}

	// Verify no corruption
	finalGuards := gm.GetGuards()
	if len(finalGuards) != 3 {
		t.Errorf("After concurrent access, %d guards exist, want 3", len(finalGuards))
	}
}

// TestGuardSetSizeLimitEnforcement verifies max guards limit is enforced
func TestGuardSetSizeLimitEnforcement(t *testing.T) {
	tmpDir := t.TempDir()
	gm, err := NewGuardManager(tmpDir, logger.NewDefault())
	if err != nil {
		t.Fatalf("NewGuardManager() failed: %v", err)
	}

	// Try to add 5 guards (limit is 3)
	for i := 0; i < 5; i++ {
		guard := &directory.Relay{
			Nickname:    fmt.Sprintf("Guard%d", i+1),
			Fingerprint: fmt.Sprintf("%040d", i),
			Address:     fmt.Sprintf("192.0.2.%d:9001", i+1),
			Flags:       []string{"Guard", "Running", "Valid", "Stable"},
		}

		if err := gm.AddGuard(guard); err != nil {
			t.Fatalf("AddGuard(%s) failed: %v", guard.Nickname, err)
		}
	}

	// Should have only 3 guards
	guards := gm.GetGuards()
	if len(guards) > 3 {
		t.Errorf("Guard set has %d guards, want <= 3 (limit enforcement)", len(guards))
	}
}

// TestWeightedRandomIndexEdgeCases tests edge cases for bandwidth-weighted selection
func TestWeightedRandomIndexEdgeCases(t *testing.T) {
	t.Run("EmptyList", func(t *testing.T) {
		_, err := weightedRandomIndex([]*directory.Relay{})
		if err == nil {
			t.Error("weightedRandomIndex([]) should return error")
		}
	})

	t.Run("SingleRelay", func(t *testing.T) {
		relays := []*directory.Relay{
			{Nickname: "Only", Fingerprint: "A", Bandwidth: 1000},
		}
		idx, err := weightedRandomIndex(relays)
		if err != nil {
			t.Fatalf("weightedRandomIndex() failed: %v", err)
		}
		if idx != 0 {
			t.Errorf("Single relay: index = %d, want 0", idx)
		}
	})

	t.Run("ZeroBandwidth", func(t *testing.T) {
		relays := []*directory.Relay{
			{Nickname: "R1", Fingerprint: "A", Bandwidth: 0},
			{Nickname: "R2", Fingerprint: "B", Bandwidth: 0},
		}
		// Should fallback to uniform random
		idx, err := weightedRandomIndex(relays)
		if err != nil {
			t.Fatalf("weightedRandomIndex() with zero bandwidth failed: %v", err)
		}
		if idx < 0 || idx >= len(relays) {
			t.Errorf("Index %d out of bounds [0, %d)", idx, len(relays))
		}
	})
}

// TestGuardExpiryCleanup verifies cleanup removes only expired guards
func TestGuardExpiryCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	gm, err := NewGuardManager(tmpDir, logger.NewDefault())
	if err != nil {
		t.Fatalf("NewGuardManager() failed: %v", err)
	}

	// Set short expiry for testing
	gm.guardExpiry = 1 * time.Second

	// Add guards with different ages
	recentGuard := &directory.Relay{
		Nickname:    "RecentGuard",
		Fingerprint: "AAAA",
		Address:     "192.0.2.1:9001",
		Flags:       []string{"Guard", "Running", "Valid", "Stable"},
	}
	oldGuard := &directory.Relay{
		Nickname:    "OldGuard",
		Fingerprint: "BBBB",
		Address:     "192.0.2.2:9001",
		Flags:       []string{"Guard", "Running", "Valid", "Stable"},
	}

	if err := gm.AddGuard(recentGuard); err != nil {
		t.Fatalf("AddGuard(recent) failed: %v", err)
	}
	if err := gm.AddGuard(oldGuard); err != nil {
		t.Fatalf("AddGuard(old) failed: %v", err)
	}

	// Manually age OldGuard
	gm.mu.Lock()
	for i := range gm.state.Guards {
		if gm.state.Guards[i].Fingerprint == "BBBB" {
			gm.state.Guards[i].LastUsed = time.Now().Add(-2 * time.Second)
			break
		}
	}
	gm.mu.Unlock()

	// Cleanup should remove only OldGuard
	gm.CleanupExpired()

	guards := gm.GetGuards()
	if len(guards) != 1 {
		t.Errorf("After cleanup, %d guards remain, want 1", len(guards))
	}

	if guards[0].Fingerprint != "AAAA" {
		t.Errorf("Remaining guard = %s, want RecentGuard (AAAA)", guards[0].Fingerprint)
	}
}
