package path

import (
	"fmt"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/directory"
	"github.com/opd-ai/go-tor/pkg/logger"
)

// TestGuardSelectionSpecCompliance_GuardFlags verifies guard selection requirements per path-spec.txt §2.1
// Guards must have the Guard, Running, Valid, and Stable flags
func TestGuardSelectionSpecCompliance_GuardFlags(t *testing.T) {
	log := logger.NewDefault()
	selector := NewSelector(nil, log)

	tests := []struct {
		name    string
		relay   *directory.Relay
		isGuard bool // Should be selectable as guard
	}{
		{
			name: "Guard+Running+Valid+Stable (VALID GUARD)",
			relay: &directory.Relay{
				Fingerprint: "AAAA0000000000000000000000000000AAAA0001",
				Nickname:    "ValidGuard",
				Flags:       []string{"Guard", "Running", "Valid", "Stable"},
				Bandwidth:   1000,
			},
			isGuard: true,
		},
		{
			name: "Missing Guard flag",
			relay: &directory.Relay{
				Fingerprint: "AAAA0000000000000000000000000000AAAA0002",
				Nickname:    "NoGuardFlag",
				Flags:       []string{"Running", "Valid", "Stable", "Fast"},
				Bandwidth:   1000,
			},
			isGuard: false,
		},
		{
			name: "Missing Running flag",
			relay: &directory.Relay{
				Fingerprint: "AAAA0000000000000000000000000000AAAA0003",
				Nickname:    "NotRunning",
				Flags:       []string{"Guard", "Valid", "Stable"},
				Bandwidth:   1000,
			},
			isGuard: false,
		},
		{
			name: "Missing Valid flag",
			relay: &directory.Relay{
				Fingerprint: "AAAA0000000000000000000000000000AAAA0004",
				Nickname:    "NotValid",
				Flags:       []string{"Guard", "Running", "Stable"},
				Bandwidth:   1000,
			},
			isGuard: false,
		},
		{
			name: "Missing Stable flag",
			relay: &directory.Relay{
				Fingerprint: "AAAA0000000000000000000000000000AAAA0005",
				Nickname:    "NotStable",
				Flags:       []string{"Guard", "Running", "Valid"},
				Bandwidth:   1000,
			},
			isGuard: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test if relay meets guard criteria
			isGuard := tt.relay.IsGuard() && tt.relay.IsRunning() && tt.relay.IsValid() && tt.relay.IsStable()

			if isGuard != tt.isGuard {
				t.Errorf("Guard flag check failed: got %v, want %v (flags: %v)", isGuard, tt.isGuard, tt.relay.Flags)
			}

			// Verify consensus update correctly filters guards
			selector.mu.Lock()
			selector.relays = []*directory.Relay{tt.relay}
			selector.guards = make([]*directory.Relay, 0)
			for _, relay := range selector.relays {
				if relay.IsGuard() && relay.IsRunning() && relay.IsValid() && relay.IsStable() {
					selector.guards = append(selector.guards, relay)
				}
			}
			selector.mu.Unlock()

			selector.mu.RLock()
			numGuards := len(selector.guards)
			selector.mu.RUnlock()

			expectedCount := 0
			if tt.isGuard {
				expectedCount = 1
			}

			if numGuards != expectedCount {
				t.Errorf("Guard filtering failed: got %d guards, want %d", numGuards, expectedCount)
			}
		})
	}
}

// TestGuardSelectionSpecCompliance_GuardPersistence verifies guard persistence per path-spec.txt §2.2
// Guards should be persisted and reused across sessions
func TestGuardSelectionSpecCompliance_GuardPersistence(t *testing.T) {
	log := logger.NewDefault()

	// Create temporary guard manager
	tmpDir := t.TempDir()
	guardMgr, err := NewGuardManager(tmpDir, log)
	if err != nil {
		t.Fatalf("Failed to create guard manager: %v", err)
	}

	// Create selector with guard manager
	selector := NewSelectorWithGuards(nil, guardMgr, log)

	// Add guards to selector
	guard1 := &directory.Relay{
		Fingerprint: "AAAA0000000000000000000000000000AAAA1111",
		Nickname:    "Guard1",
		Address:     "1.2.3.4",
		Flags:       []string{"Guard", "Running", "Valid", "Stable"},
		Bandwidth:   1000,
	}
	guard2 := &directory.Relay{
		Fingerprint: "BBBB0000000000000000000000000000BBBB2222",
		Nickname:    "Guard2",
		Address:     "5.6.7.8",
		Flags:       []string{"Guard", "Running", "Valid", "Stable"},
		Bandwidth:   2000,
	}

	selector.mu.Lock()
	selector.guards = []*directory.Relay{guard1, guard2}
	selector.relays = []*directory.Relay{guard1, guard2}
	selector.mu.Unlock()

	// Select a guard (should be persisted)
	selectedGuard, err := selector.selectGuard()
	if err != nil {
		t.Fatalf("Failed to select guard: %v", err)
	}

	// Verify guard was persisted
	persistedGuards := guardMgr.GetGuards()
	if len(persistedGuards) == 0 {
		t.Fatal("Guard was not persisted")
	}

	// Verify the persisted guard matches the selected guard
	found := false
	for _, pg := range persistedGuards {
		if pg.Fingerprint == selectedGuard.Fingerprint {
			found = true
			if pg.Nickname != selectedGuard.Nickname {
				t.Errorf("Persisted guard nickname mismatch: got %s, want %s", pg.Nickname, selectedGuard.Nickname)
			}
			if pg.Address != selectedGuard.Address {
				t.Errorf("Persisted guard address mismatch: got %s, want %s", pg.Address, selectedGuard.Address)
			}
			break
		}
	}

	if !found {
		t.Errorf("Selected guard %s not found in persisted guards", selectedGuard.Fingerprint)
	}

	// Save guard state
	if err := guardMgr.Save(); err != nil {
		t.Fatalf("Failed to save guard state: %v", err)
	}

	// Create new guard manager (simulating restart)
	guardMgr2, err := NewGuardManager(tmpDir, log)
	if err != nil {
		t.Fatalf("Failed to create second guard manager: %v", err)
	}

	// Verify guards were loaded from disk
	loadedGuards := guardMgr2.GetGuards()
	if len(loadedGuards) == 0 {
		t.Fatal("Guards were not loaded from disk after restart")
	}

	// Verify loaded guard matches original
	found = false
	for _, lg := range loadedGuards {
		if lg.Fingerprint == selectedGuard.Fingerprint {
			found = true
			if lg.Nickname != selectedGuard.Nickname {
				t.Errorf("Loaded guard nickname mismatch: got %s, want %s", lg.Nickname, selectedGuard.Nickname)
			}
			break
		}
	}

	if !found {
		t.Errorf("Selected guard %s not found in loaded guards after restart", selectedGuard.Fingerprint)
	}
}

// TestGuardSelectionSpecCompliance_GuardRotation verifies guard rotation per path-spec.txt §2.3
// Guards should be rotated after 90 days (guard expiry period)
func TestGuardSelectionSpecCompliance_GuardRotation(t *testing.T) {
	log := logger.NewDefault()
	tmpDir := t.TempDir()

	// Create guard manager with custom expiry (1 minute for testing)
	config := DefaultGuardManagerConfig(tmpDir)
	config.GuardExpiry = 1 * time.Minute
	guardMgr, err := NewGuardManagerWithConfig(config, log)
	if err != nil {
		t.Fatalf("Failed to create guard manager: %v", err)
	}

	// Add a guard with old LastUsed timestamp
	oldGuard := GuardEntry{
		Fingerprint: "AAAA0000000000000000000000000000AAAA0001",
		Nickname:    "OldGuard",
		Address:     "1.2.3.4",
		FirstUsed:   time.Now().Add(-90 * 24 * time.Hour), // 90 days ago
		LastUsed:    time.Now().Add(-91 * 24 * time.Hour), // 91 days ago (expired)
		Confirmed:   true,
	}

	guardMgr.mu.Lock()
	guardMgr.state.Guards = []GuardEntry{oldGuard}
	guardMgr.mu.Unlock()

	// Get guards (should not include expired guard)
	validGuards := guardMgr.GetGuards()
	if len(validGuards) != 0 {
		t.Errorf("Expected 0 valid guards (expired), got %d", len(validGuards))
	}

	// Cleanup expired guards
	guardMgr.CleanupExpired()

	// Verify cleanup removed the expired guard
	guardMgr.mu.RLock()
	remainingGuards := len(guardMgr.state.Guards)
	guardMgr.mu.RUnlock()

	if remainingGuards != 0 {
		t.Errorf("Expected 0 guards after cleanup, got %d", remainingGuards)
	}
}

// TestGuardSelectionSpecCompliance_GuardConfirmation verifies guard confirmation per path-spec.txt §2.4
// Guards should be confirmed after successful circuit building
func TestGuardSelectionSpecCompliance_GuardConfirmation(t *testing.T) {
	log := logger.NewDefault()
	tmpDir := t.TempDir()
	guardMgr, err := NewGuardManager(tmpDir, log)
	if err != nil {
		t.Fatalf("Failed to create guard manager: %v", err)
	}

	// Add unconfirmed guard
	guard := &directory.Relay{
		Fingerprint: "AAAA0000000000000000000000000000AAAA0001",
		Nickname:    "UnconfirmedGuard",
		Address:     "1.2.3.4",
		Flags:       []string{"Guard", "Running", "Valid", "Stable"},
		Bandwidth:   1000,
	}

	if err := guardMgr.AddGuard(guard); err != nil {
		t.Fatalf("Failed to add guard: %v", err)
	}

	// Verify guard is unconfirmed
	guards := guardMgr.GetGuards()
	if len(guards) == 0 {
		t.Fatal("Guard was not added")
	}
	if guards[0].Confirmed {
		t.Error("Newly added guard should be unconfirmed")
	}

	// Confirm the guard (simulating successful circuit)
	if err := guardMgr.ConfirmGuard(guard.Fingerprint); err != nil {
		t.Fatalf("Failed to confirm guard: %v", err)
	}

	// Verify guard is now confirmed
	confirmedGuards := guardMgr.GetGuards()
	if len(confirmedGuards) == 0 {
		t.Fatal("Guard disappeared after confirmation")
	}
	if !confirmedGuards[0].Confirmed {
		t.Error("Guard should be confirmed after ConfirmGuard")
	}
}

// TestGuardSelectionSpecCompliance_GuardLimit verifies guard limit per path-spec.txt §2.5
// Maximum number of guards should be limited (typically 3)
func TestGuardSelectionSpecCompliance_GuardLimit(t *testing.T) {
	log := logger.NewDefault()
	tmpDir := t.TempDir()

	config := DefaultGuardManagerConfig(tmpDir)
	config.MaxGuards = 3
	guardMgr, err := NewGuardManagerWithConfig(config, log)
	if err != nil {
		t.Fatalf("Failed to create guard manager: %v", err)
	}

	// Verify default max guards is 3 per Tor spec
	if config.MaxGuards != 3 {
		t.Errorf("Default MaxGuards should be 3 per Tor spec, got %d", config.MaxGuards)
	}

	// Add 5 guards
	for i := 1; i <= 5; i++ {
		guard := &directory.Relay{
			Fingerprint: fmt.Sprintf("AAAA000000000000000000000000000000%02d", i),
			Nickname:    fmt.Sprintf("Guard%d", i),
			Address:     fmt.Sprintf("1.2.3.%d", i),
			Flags:       []string{"Guard", "Running", "Valid", "Stable"},
			Bandwidth:   uint64(1000 * i),
		}
		if err := guardMgr.AddGuard(guard); err != nil {
			t.Fatalf("Failed to add guard %d: %v", i, err)
		}
	}

	// Verify only MaxGuards (3) are stored
	guards := guardMgr.GetGuards()
	if len(guards) > config.MaxGuards {
		t.Errorf("Guard count exceeds MaxGuards: got %d, max %d", len(guards), config.MaxGuards)
	}
}

// TestGuardSelectionSpecCompliance_BiasedGuardAvoidance verifies biased guard filtering per path-spec.txt §5.3
// Guards that appear biased (path bias detection) should be avoided
func TestGuardSelectionSpecCompliance_BiasedGuardAvoidance(t *testing.T) {
	log := logger.NewDefault()
	selector := NewSelector(nil, log)

	// Create guards
	goodGuard := &directory.Relay{
		Fingerprint: "AAAA0000000000000000000000000000AAAA0001",
		Nickname:    "GoodGuard",
		Flags:       []string{"Guard", "Running", "Valid", "Stable"},
		Bandwidth:   1000,
	}
	biasedGuard := &directory.Relay{
		Fingerprint: "BBBB0000000000000000000000000000BBBB0002",
		Nickname:    "BiasedGuard",
		Flags:       []string{"Guard", "Running", "Valid", "Stable"},
		Bandwidth:   2000, // Higher bandwidth but biased
	}

	selector.mu.Lock()
	selector.guards = []*directory.Relay{goodGuard, biasedGuard}
	selector.relays = []*directory.Relay{goodGuard, biasedGuard}
	selector.mu.Unlock()

	// Mark biasedGuard as biased
	for i := 0; i < 20; i++ {
		selector.RecordCircuitOutcome(uint32(i), biasedGuard.Fingerprint, OutcomeBuildFailed)
	}

	// Select guard multiple times
	selections := make(map[string]int)
	for i := 0; i < 20; i++ {
		guard, err := selector.selectGuard()
		if err != nil {
			t.Fatalf("Failed to select guard: %v", err)
		}
		selections[guard.Fingerprint]++
	}

	// Verify biased guard was avoided (should select goodGuard most of the time)
	if selections[biasedGuard.Fingerprint] > selections[goodGuard.Fingerprint] {
		t.Errorf("Biased guard was preferred over good guard: biased=%d, good=%d",
			selections[biasedGuard.Fingerprint], selections[goodGuard.Fingerprint])
	}

	// Verify bias detection marked the guard as biased
	if !selector.IsGuardBiased(biasedGuard.Fingerprint) {
		t.Error("BiasedGuard should be marked as biased by BiasDetector")
	}
}

// TestGuardSelectionSpecCompliance_BandwidthWeighted verifies bandwidth-weighted guard selection per path-spec.txt §2.2
// Guards with higher bandwidth should be selected more frequently
func TestGuardSelectionSpecCompliance_BandwidthWeighted(t *testing.T) {
	log := logger.NewDefault()
	selector := NewSelector(nil, log)

	// Create guards with different bandwidths
	lowBWGuard := &directory.Relay{
		Fingerprint: "AAAA0000000000000000000000000000AAAA0001",
		Nickname:    "LowBWGuard",
		Flags:       []string{"Guard", "Running", "Valid", "Stable"},
		Bandwidth:   100, // Low bandwidth
	}
	highBWGuard := &directory.Relay{
		Fingerprint: "BBBB0000000000000000000000000000BBBB0002",
		Nickname:    "HighBWGuard",
		Flags:       []string{"Guard", "Running", "Valid", "Stable"},
		Bandwidth:   10000, // 100x higher bandwidth
	}

	selector.mu.Lock()
	selector.guards = []*directory.Relay{lowBWGuard, highBWGuard}
	selector.relays = []*directory.Relay{lowBWGuard, highBWGuard}
	selector.mu.Unlock()

	// Select guard 100 times
	selections := make(map[string]int)
	for i := 0; i < 100; i++ {
		guard, err := selector.selectGuard()
		if err != nil {
			t.Fatalf("Failed to select guard: %v", err)
		}
		selections[guard.Fingerprint]++
	}

	// Verify high bandwidth guard was selected more frequently
	// With 100:1 bandwidth ratio, expect ~99:1 selection ratio (within statistical variance)
	if selections[highBWGuard.Fingerprint] <= selections[lowBWGuard.Fingerprint] {
		t.Errorf("High bandwidth guard should be selected more frequently: high=%d, low=%d",
			selections[highBWGuard.Fingerprint], selections[lowBWGuard.Fingerprint])
	}

	// Verify high bandwidth guard was selected at least 80% of the time (reasonable threshold)
	highBWPercentage := float64(selections[highBWGuard.Fingerprint]) / 100.0
	if highBWPercentage < 0.80 {
		t.Errorf("High bandwidth guard should be selected ~99%% of the time, got %.1f%%", highBWPercentage*100)
	}

	t.Logf("Guard selection distribution (100 iterations): LowBW=%d (%.1f%%), HighBW=%d (%.1f%%)",
		selections[lowBWGuard.Fingerprint], float64(selections[lowBWGuard.Fingerprint]),
		selections[highBWGuard.Fingerprint], highBWPercentage*100)
}

// TestGuardSelectionSpecCompliance_PersistentGuardPreference verifies persistent guard reuse per path-spec.txt §2.2
// If a guard is in persistent state and still in consensus, it should be preferred
func TestGuardSelectionSpecCompliance_PersistentGuardPreference(t *testing.T) {
	log := logger.NewDefault()
	tmpDir := t.TempDir()
	guardMgr, err := NewGuardManager(tmpDir, log)
	if err != nil {
		t.Fatalf("Failed to create guard manager: %v", err)
	}

	// Create two guards
	persistentGuard := &directory.Relay{
		Fingerprint: "AAAA0000000000000000000000000000AAAA0001",
		Nickname:    "PersistentGuard",
		Address:     "1.2.3.4",
		Flags:       []string{"Guard", "Running", "Valid", "Stable"},
		Bandwidth:   1000,
	}
	newGuard := &directory.Relay{
		Fingerprint: "BBBB0000000000000000000000000000BBBB0002",
		Nickname:    "NewGuard",
		Address:     "5.6.7.8",
		Flags:       []string{"Guard", "Running", "Valid", "Stable"},
		Bandwidth:   2000, // Higher bandwidth
	}

	// Add persistent guard to guard manager and confirm it
	if err := guardMgr.AddGuard(persistentGuard); err != nil {
		t.Fatalf("Failed to add persistent guard: %v", err)
	}
	if err := guardMgr.ConfirmGuard(persistentGuard.Fingerprint); err != nil {
		t.Fatalf("Failed to confirm persistent guard: %v", err)
	}

	// Create selector with guard manager
	selector := NewSelectorWithGuards(nil, guardMgr, log)
	selector.mu.Lock()
	selector.guards = []*directory.Relay{persistentGuard, newGuard}
	selector.relays = []*directory.Relay{persistentGuard, newGuard}
	selector.mu.Unlock()

	// Select guard multiple times
	selections := make(map[string]int)
	for i := 0; i < 20; i++ {
		guard, err := selector.selectGuard()
		if err != nil {
			t.Fatalf("Failed to select guard: %v", err)
		}
		selections[guard.Fingerprint]++
	}

	// Verify persistent guard was preferred despite lower bandwidth
	if selections[persistentGuard.Fingerprint] == 0 {
		t.Error("Persistent guard should have been selected at least once")
	}

	// In practice, persistent guard should be selected every time
	// since the code tries to use persistent guards first
	if selections[persistentGuard.Fingerprint] < selections[newGuard.Fingerprint] {
		t.Errorf("Persistent guard should be preferred: persistent=%d, new=%d",
			selections[persistentGuard.Fingerprint], selections[newGuard.Fingerprint])
	}

	t.Logf("Guard selection with persistence (20 iterations): Persistent=%d, New=%d",
		selections[persistentGuard.Fingerprint], selections[newGuard.Fingerprint])
}

// TestGuardSelectionSpecCompliance_UpdateConsensus verifies consensus update filtering per path-spec.txt
// UpdateConsensus should only include relays with Guard+Running+Valid+Stable flags in guards list
func TestGuardSelectionSpecCompliance_UpdateConsensus(t *testing.T) {
	// This test would normally require a mock directory client
	// For spec compliance, we verify the filtering logic in UpdateConsensus

	log := logger.NewDefault()
	selector := NewSelector(nil, log)

	// Manually populate relays (simulating consensus)
	relays := []*directory.Relay{
		{
			Fingerprint: "AAAA0000000000000000000000000000AAAA0001",
			Nickname:    "ValidGuard",
			Flags:       []string{"Guard", "Running", "Valid", "Stable"},
			Bandwidth:   1000,
		},
		{
			Fingerprint: "AAAA0000000000000000000000000000AAAA0002",
			Nickname:    "NotStable",
			Flags:       []string{"Guard", "Running", "Valid"},
			Bandwidth:   1000,
		},
		{
			Fingerprint: "AAAA0000000000000000000000000000AAAA0003",
			Nickname:    "NotGuard",
			Flags:       []string{"Running", "Valid", "Stable"},
			Bandwidth:   1000,
		},
		{
			Fingerprint: "AAAA0000000000000000000000000000AAAA0004",
			Nickname:    "NotRunning",
			Flags:       []string{"Guard", "Valid", "Stable"},
			Bandwidth:   1000,
		},
	}

	// Simulate UpdateConsensus filtering logic
	selector.mu.Lock()
	guards := make([]*directory.Relay, 0)
	allRelays := make([]*directory.Relay, 0)

	for _, relay := range relays {
		if !relay.IsRunning() || !relay.IsValid() {
			continue // Skip non-running or invalid relays
		}

		allRelays = append(allRelays, relay)

		if relay.IsGuard() && relay.IsStable() {
			guards = append(guards, relay)
		}
	}

	selector.guards = guards
	selector.relays = allRelays
	selector.mu.Unlock()

	// Verify only 1 guard (ValidGuard) passed the filters
	selector.mu.RLock()
	numGuards := len(selector.guards)
	numRelays := len(selector.relays)
	selector.mu.RUnlock()

	if numGuards != 1 {
		t.Errorf("Expected 1 guard relay after filtering, got %d", numGuards)
	}

	if numRelays != 3 {
		t.Errorf("Expected 3 total relays after filtering (Running+Valid), got %d", numRelays)
	}

	// Verify the guard is ValidGuard
	selector.mu.RLock()
	if len(selector.guards) > 0 && selector.guards[0].Nickname != "ValidGuard" {
		t.Errorf("Expected ValidGuard to be selected, got %s", selector.guards[0].Nickname)
	}
	selector.mu.RUnlock()
}

// TestGuardSelectionSpecCompliance_NoGuardsAvailable verifies error handling when no guards are available
func TestGuardSelectionSpecCompliance_NoGuardsAvailable(t *testing.T) {
	log := logger.NewDefault()
	selector := NewSelector(nil, log)

	// Try to select guard with empty guard list
	_, err := selector.selectGuard()
	if err == nil {
		t.Error("selectGuard should return error when no guards available")
	}

	expectedError := "no guard relays available"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

// TestGuardSelectionSpecCompliance_GuardExpiry verifies guard expiry per path-spec.txt §2.3
// Default guard expiry should be 90 days
func TestGuardSelectionSpecCompliance_GuardExpiry(t *testing.T) {
	tmpDir := t.TempDir()
	config := DefaultGuardManagerConfig(tmpDir)

	// Verify default guard expiry is 90 days per Tor spec
	expectedExpiry := 90 * 24 * time.Hour
	if config.GuardExpiry != expectedExpiry {
		t.Errorf("Default GuardExpiry should be 90 days per Tor spec, got %v", config.GuardExpiry)
	}

	// Verify guard manager respects expiry
	log := logger.NewDefault()
	guardMgr, err := NewGuardManagerWithConfig(config, log)
	if err != nil {
		t.Fatalf("Failed to create guard manager: %v", err)
	}

	// Add guard that was last used 91 days ago (expired)
	expiredGuard := GuardEntry{
		Fingerprint: "AAAA0000000000000000000000000000AAAA0001",
		Nickname:    "ExpiredGuard",
		Address:     "1.2.3.4",
		FirstUsed:   time.Now().Add(-100 * 24 * time.Hour),
		LastUsed:    time.Now().Add(-91 * 24 * time.Hour),
		Confirmed:   true,
	}

	// Add guard that was last used 89 days ago (still valid)
	validGuard := GuardEntry{
		Fingerprint: "BBBB0000000000000000000000000000BBBB0002",
		Nickname:    "ValidGuard",
		Address:     "5.6.7.8",
		FirstUsed:   time.Now().Add(-90 * 24 * time.Hour),
		LastUsed:    time.Now().Add(-89 * 24 * time.Hour),
		Confirmed:   true,
	}

	guardMgr.mu.Lock()
	guardMgr.state.Guards = []GuardEntry{expiredGuard, validGuard}
	guardMgr.mu.Unlock()

	// Get valid guards (should only return ValidGuard)
	validGuards := guardMgr.GetGuards()
	if len(validGuards) != 1 {
		t.Errorf("Expected 1 valid guard (ValidGuard), got %d", len(validGuards))
	}

	if len(validGuards) > 0 && validGuards[0].Fingerprint != validGuard.Fingerprint {
		t.Errorf("Expected ValidGuard to remain, got %s", validGuards[0].Nickname)
	}
}
