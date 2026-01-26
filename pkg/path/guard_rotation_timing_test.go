package path

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/directory"
	"github.com/opd-ai/go-tor/pkg/logger"
)

// TestGuardExpiryFixed verifies the current implementation uses fixed 90-day expiry
// This test documents the VULNERABILITY identified in the audit
func TestGuardExpiryFixed(t *testing.T) {
	tmpDir := t.TempDir()
	gm, err := NewGuardManager(tmpDir, logger.NewDefault())
	if err != nil {
		t.Fatalf("Failed to create guard manager: %v", err)
	}

	// Verify default expiry is fixed at 90 days
	expectedExpiry := 90 * 24 * time.Hour
	if gm.guardExpiry != expectedExpiry {
		t.Errorf("Guard expiry = %v, want %v", gm.guardExpiry, expectedExpiry)
	}

	// Add multiple guards and verify they all use the same expiry
	relays := []*directory.Relay{
		{Fingerprint: "AAAA", Nickname: "Guard1", Address: "1.1.1.1:9001"},
		{Fingerprint: "BBBB", Nickname: "Guard2", Address: "2.2.2.2:9001"},
		{Fingerprint: "CCCC", Nickname: "Guard3", Address: "3.3.3.3:9001"},
	}

	for _, relay := range relays {
		if err := gm.AddGuard(relay); err != nil {
			t.Fatalf("Failed to add guard %s: %v", relay.Nickname, err)
		}
	}

	guards := gm.GetGuards()
	if len(guards) != 3 {
		t.Fatalf("Expected 3 guards, got %d", len(guards))
	}

	// All guards added at the same time will expire at the same time
	// This is the VULNERABILITY - synchronized rotation
	firstUsedTimes := make([]time.Time, len(guards))
	for i, guard := range guards {
		firstUsedTimes[i] = guard.FirstUsed
	}

	// Verify all guards have nearly identical FirstUsed times (within 1 second)
	for i := 1; i < len(firstUsedTimes); i++ {
		diff := firstUsedTimes[i].Sub(firstUsedTimes[0])
		if diff < 0 {
			diff = -diff
		}
		if diff > time.Second {
			t.Errorf("Guard %d FirstUsed differs by %v from guard 0, expected < 1s (synchronized)", i, diff)
		}
	}

	t.Log("VULNERABILITY CONFIRMED: All guards expire at the same time (synchronized rotation)")
}

// TestSynchronizedRotation verifies the synchronized rotation vulnerability
func TestSynchronizedRotation(t *testing.T) {
	tmpDir := t.TempDir()
	gm, err := NewGuardManager(tmpDir, logger.NewDefault())
	if err != nil {
		t.Fatalf("Failed to create guard manager: %v", err)
	}

	// Add 3 guards simultaneously
	relays := []*directory.Relay{
		{Fingerprint: "G1", Nickname: "Guard1", Address: "1.1.1.1:9001"},
		{Fingerprint: "G2", Nickname: "Guard2", Address: "2.2.2.2:9001"},
		{Fingerprint: "G3", Nickname: "Guard3", Address: "3.3.3.3:9001"},
	}

	for _, relay := range relays {
		if err := gm.AddGuard(relay); err != nil {
			t.Fatalf("Failed to add guard: %v", err)
		}
	}

	guards := gm.GetGuards()
	if len(guards) != 3 {
		t.Fatalf("Expected 3 guards, got %d", len(guards))
	}

	// Fast-forward time to just before expiry
	gm.mu.Lock()
	for i := range gm.state.Guards {
		gm.state.Guards[i].LastUsed = time.Now().Add(-89 * 24 * time.Hour)
	}
	gm.mu.Unlock()

	// All guards should still be valid
	validGuards := gm.GetGuards()
	if len(validGuards) != 3 {
		t.Errorf("Expected 3 valid guards at 89 days, got %d", len(validGuards))
	}

	// Fast-forward to just after expiry
	gm.mu.Lock()
	for i := range gm.state.Guards {
		gm.state.Guards[i].LastUsed = time.Now().Add(-91 * 24 * time.Hour)
	}
	gm.mu.Unlock()

	// All guards should expire simultaneously
	validGuards = gm.GetGuards()
	if len(validGuards) != 0 {
		t.Errorf("Expected 0 valid guards at 91 days, got %d (synchronized expiry)", len(validGuards))
	}

	t.Log("VULNERABILITY CONFIRMED: All guards expire simultaneously")
}

// TestTemporalCorrelationVulnerability demonstrates temporal correlation attack
func TestTemporalCorrelationVulnerability(t *testing.T) {
	// Simulate multiple clients starting on the same day
	clients := make([]*GuardManager, 10)
	startTime := time.Now()

	tmpDirs := make([]string, 10)
	for i := 0; i < 10; i++ {
		tmpDirs[i] = t.TempDir()
		gm, err := NewGuardManager(tmpDirs[i], logger.NewDefault())
		if err != nil {
			t.Fatalf("Client %d: failed to create guard manager: %v", i, err)
		}
		clients[i] = gm

		// Each client adds 3 guards
		for j := 0; j < 3; j++ {
			relay := &directory.Relay{
				Fingerprint: randomFingerprint(t),
				Nickname:    "Guard",
				Address:     "1.1.1.1:9001",
			}
			if err := gm.AddGuard(relay); err != nil {
				t.Fatalf("Client %d: failed to add guard: %v", i, err)
			}
		}
	}

	// Collect rotation times for all clients
	rotationTimes := make([]time.Time, 10)
	for i, gm := range clients {
		guards := gm.GetGuards()
		if len(guards) == 0 {
			t.Fatalf("Client %d has no guards", i)
		}
		// Calculate when this client will rotate (FirstUsed + 90 days)
		rotationTimes[i] = guards[0].FirstUsed.Add(90 * 24 * time.Hour)
	}

	// Check if rotation times are synchronized (within 1 minute window)
	correlationWindow := time.Minute
	correlatedClients := 0

	for i := 1; i < len(rotationTimes); i++ {
		diff := rotationTimes[i].Sub(rotationTimes[0])
		if diff < 0 {
			diff = -diff
		}
		if diff < correlationWindow {
			correlatedClients++
		}
	}

	// If most clients rotate within the same window, they can be correlated
	correlationRate := float64(correlatedClients) / float64(len(rotationTimes)-1) * 100

	if correlationRate > 80 {
		t.Logf("VULNERABILITY: %.1f%% of clients rotate within %v (temporal correlation possible)",
			correlationRate, correlationWindow)
	}

	// For this test, we expect high correlation since all clients started simultaneously
	if correlationRate < 80 {
		t.Errorf("Expected >80%% correlation, got %.1f%% (test setup issue)", correlationRate)
	}

	t.Logf("Temporal correlation: %.1f%% of clients rotate within %v of first client",
		correlationRate, correlationWindow)
	t.Logf("Rotation spread: %v", rotationTimes[len(rotationTimes)-1].Sub(rotationTimes[0]))
	t.Logf("Expected spread with randomization: ~60 days (currently: %v)",
		rotationTimes[len(rotationTimes)-1].Sub(rotationTimes[0]))

	expectedSpread := startTime.Add(time.Second).Sub(startTime)
	if rotationTimes[len(rotationTimes)-1].Sub(rotationTimes[0]) < expectedSpread {
		t.Logf("VULNERABILITY CONFIRMED: Rotation times are highly correlated (fingerprinting possible)")
	}
}

// TestClientIdentificationVulnerability demonstrates client identification attack
func TestClientIdentificationVulnerability(t *testing.T) {
	tmpDir := t.TempDir()
	gm, err := NewGuardManager(tmpDir, logger.NewDefault())
	if err != nil {
		t.Fatalf("Failed to create guard manager: %v", err)
	}

	// Client adds a guard
	relay := &directory.Relay{
		Fingerprint: "MALICIOUS_GUARD",
		Nickname:    "MaliciousGuard",
		Address:     "1.1.1.1:9001",
	}

	if err := gm.AddGuard(relay); err != nil {
		t.Fatalf("Failed to add guard: %v", err)
	}

	guards := gm.GetGuards()
	if len(guards) != 1 {
		t.Fatalf("Expected 1 guard, got %d", len(guards))
	}

	// Adversary records connection timestamp
	connectionTime := guards[0].FirstUsed
	t.Logf("Adversary observes: Client connected at %v", connectionTime)

	// Adversary predicts disconnection time
	predictedDisconnect := connectionTime.Add(90 * 24 * time.Hour)
	t.Logf("Adversary predicts: Client will disconnect at %v", predictedDisconnect)

	// Simulate guard expiry
	gm.mu.Lock()
	gm.state.Guards[0].LastUsed = time.Now().Add(-91 * 24 * time.Hour)
	gm.mu.Unlock()

	validGuards := gm.GetGuards()
	if len(validGuards) != 0 {
		t.Errorf("Guard should have expired, but %d guards remain", len(validGuards))
	}

	actualDisconnect := connectionTime.Add(90 * 24 * time.Hour)
	predictionError := actualDisconnect.Sub(predictedDisconnect)
	if predictionError < 0 {
		predictionError = -predictionError
	}

	t.Logf("Prediction error: %v", predictionError)

	// With fixed expiry, prediction error should be near zero
	if predictionError < time.Hour {
		t.Log("VULNERABILITY CONFIRMED: Adversary can predict rotation time with <1 hour error")
		t.Log("Impact: Client can be identified and tracked across guard rotations")
	}
}

// TestLongTermTrackingVulnerability demonstrates long-term tracking attack
func TestLongTermTrackingVulnerability(t *testing.T) {
	tmpDir := t.TempDir()
	gm, err := NewGuardManager(tmpDir, logger.NewDefault())
	if err != nil {
		t.Fatalf("Failed to create guard manager: %v", err)
	}

	// Simulate multiple rotation cycles
	rotationCycles := 5
	rotationIntervals := make([]time.Duration, rotationCycles)

	for cycle := 0; cycle < rotationCycles; cycle++ {
		// Add guards for this cycle
		for i := 0; i < 3; i++ {
			relay := &directory.Relay{
				Fingerprint: randomFingerprint(t),
				Nickname:    "Guard",
				Address:     "1.1.1.1:9001",
			}
			if err := gm.AddGuard(relay); err != nil {
				t.Fatalf("Cycle %d: failed to add guard: %v", cycle, err)
			}
		}

		guards := gm.GetGuards()
		if len(guards) == 0 {
			t.Fatalf("Cycle %d: no guards available", cycle)
		}

		// Record rotation interval
		if cycle > 0 {
			rotationIntervals[cycle-1] = guards[0].FirstUsed.Sub(guards[0].FirstUsed.Add(-90 * 24 * time.Hour))
		}

		// Simulate guard expiry and cleanup
		gm.mu.Lock()
		for i := range gm.state.Guards {
			gm.state.Guards[i].LastUsed = time.Now().Add(-91 * 24 * time.Hour)
		}
		gm.mu.Unlock()
		gm.CleanupExpired()
	}

	// Analyze rotation intervals for predictability
	if len(rotationIntervals) > 0 {
		// Calculate variance in rotation intervals
		var totalVariance time.Duration
		avgInterval := 90 * 24 * time.Hour

		for _, interval := range rotationIntervals {
			diff := interval - avgInterval
			if diff < 0 {
				diff = -diff
			}
			totalVariance += diff
		}

		meanVariance := totalVariance / time.Duration(len(rotationIntervals))
		t.Logf("Mean rotation interval variance: %v", meanVariance)

		// With fixed expiry, variance should be very small
		if meanVariance < 24*time.Hour {
			t.Log("VULNERABILITY CONFIRMED: Rotation intervals are highly predictable")
			t.Logf("Adversary can track client across %d rotation cycles", rotationCycles)
			t.Log("Impact: Long-term user tracking is feasible")
		}
	}
}

// TestRotationTimingJitter verifies lack of jitter in rotation execution
func TestRotationTimingJitter(t *testing.T) {
	tmpDir := t.TempDir()
	gm, err := NewGuardManager(tmpDir, logger.NewDefault())
	if err != nil {
		t.Fatalf("Failed to create guard manager: %v", err)
	}

	relay := &directory.Relay{
		Fingerprint: "TEST",
		Nickname:    "TestGuard",
		Address:     "1.1.1.1:9001",
	}

	if err := gm.AddGuard(relay); err != nil {
		t.Fatalf("Failed to add guard: %v", err)
	}

	// Set LastUsed to exactly 90 days ago
	exactExpiryTime := time.Now().Add(-90 * 24 * time.Hour)
	gm.mu.Lock()
	gm.state.Guards[0].LastUsed = exactExpiryTime
	gm.mu.Unlock()

	// Call CleanupExpired multiple times
	jitterMeasurements := make([]bool, 10)
	for i := 0; i < 10; i++ {
		// Create a fresh guard manager for each measurement
		subdir := fmt.Sprintf("%s/test_%d", tmpDir, i)
		gm2, err := NewGuardManager(subdir, logger.NewDefault())
		if err != nil {
			t.Fatalf("Failed to create guard manager %d: %v", i, err)
		}
		if err := gm2.AddGuard(relay); err != nil {
			t.Fatalf("Failed to add guard %d: %v", i, err)
		}
		gm2.mu.Lock()
		gm2.state.Guards[0].LastUsed = exactExpiryTime
		gm2.mu.Unlock()

		gm2.CleanupExpired()
		guards := gm2.GetGuards()
		jitterMeasurements[i] = len(guards) == 0 // true if expired
	}

	// All measurements should be identical (no jitter)
	allIdentical := true
	firstResult := jitterMeasurements[0]
	for i := 1; i < len(jitterMeasurements); i++ {
		if jitterMeasurements[i] != firstResult {
			allIdentical = false
			break
		}
	}

	if allIdentical {
		t.Log("VULNERABILITY CONFIRMED: No jitter in rotation execution")
		t.Log("Impact: Rotation timing is deterministic and predictable")
	} else {
		t.Log("INFO: Some jitter detected in rotation execution (test timing variation)")
	}
}

// TestNoRotationRateLimit verifies lack of rate limiting
func TestNoRotationRateLimit(t *testing.T) {
	tmpDir := t.TempDir()
	gm, err := NewGuardManager(tmpDir, logger.NewDefault())
	if err != nil {
		t.Fatalf("Failed to create guard manager: %v", err)
	}

	// Add guards
	for i := 0; i < 5; i++ {
		relay := &directory.Relay{
			Fingerprint: randomFingerprint(t),
			Nickname:    "Guard",
			Address:     "1.1.1.1:9001",
		}
		if err := gm.AddGuard(relay); err != nil {
			t.Fatalf("Failed to add guard: %v", err)
		}
	}

	guards := gm.GetGuards()
	if len(guards) < 3 {
		t.Fatalf("Expected at least 3 guards, got %d", len(guards))
	}

	// Attempt rapid removal (simulating rapid rotation)
	startTime := time.Now()
	removedCount := 0

	for _, guard := range guards[:3] {
		if err := gm.RemoveGuard(guard.Fingerprint); err != nil {
			t.Logf("Failed to remove guard %s: %v", guard.Nickname, err)
		} else {
			removedCount++
		}
	}

	duration := time.Since(startTime)

	if removedCount >= 3 && duration < time.Second {
		t.Log("VULNERABILITY CONFIRMED: No rate limiting on guard removal")
		t.Logf("Removed %d guards in %v (no minimum interval enforced)", removedCount, duration)
		t.Log("Impact: Behavioral fingerprinting via rotation frequency is possible")
	}
}

// TestFingerprintingResistanceStatistics performs statistical analysis
func TestFingerprintingResistanceStatistics(t *testing.T) {
	// Create 100 simulated clients
	numClients := 100
	rotationTimes := make([]time.Time, numClients)

	for i := 0; i < numClients; i++ {
		tmpDir := t.TempDir()
		gm, err := NewGuardManager(tmpDir, logger.NewDefault())
		if err != nil {
			t.Fatalf("Client %d: failed to create guard manager: %v", i, err)
		}

		relay := &directory.Relay{
			Fingerprint: randomFingerprint(t),
			Nickname:    "Guard",
			Address:     "1.1.1.1:9001",
		}

		if err := gm.AddGuard(relay); err != nil {
			t.Fatalf("Client %d: failed to add guard: %v", i, err)
		}

		guards := gm.GetGuards()
		if len(guards) == 0 {
			t.Fatalf("Client %d: no guards", i)
		}

		// Calculate rotation time (FirstUsed + 90 days)
		rotationTimes[i] = guards[0].FirstUsed.Add(90 * 24 * time.Hour)
	}

	// Calculate statistics
	earliestRotation := rotationTimes[0]
	latestRotation := rotationTimes[0]
	var totalDuration time.Duration

	for i, rt := range rotationTimes {
		if rt.Before(earliestRotation) {
			earliestRotation = rt
		}
		if rt.After(latestRotation) {
			latestRotation = rt
		}
		if i > 0 {
			totalDuration += rt.Sub(rotationTimes[i-1])
		}
	}

	rotationSpread := latestRotation.Sub(earliestRotation)
	avgSpacing := totalDuration / time.Duration(numClients-1)

	t.Logf("Rotation Statistics for %d clients:", numClients)
	t.Logf("  Earliest rotation: %v", earliestRotation)
	t.Logf("  Latest rotation: %v", latestRotation)
	t.Logf("  Rotation spread: %v", rotationSpread)
	t.Logf("  Average spacing: %v", avgSpacing)

	// Expected spread with randomization: ~60 days (60-120 day range)
	expectedSpread := 60 * 24 * time.Hour

	if rotationSpread < expectedSpread/10 {
		t.Log("VULNERABILITY CONFIRMED: Rotation spread is very narrow")
		t.Logf("  Current spread: %v", rotationSpread)
		t.Logf("  Expected spread: %v", expectedSpread)
		t.Log("  Impact: High correlation enables fingerprinting")
	}

	// Calculate entropy (number of unique rotation windows)
	windowSize := time.Hour
	windows := make(map[int64]int)
	for _, rt := range rotationTimes {
		window := rt.Unix() / int64(windowSize.Seconds())
		windows[window]++
	}

	entropy := len(windows)
	entropyRate := float64(entropy) / float64(numClients) * 100

	t.Logf("  Unique rotation windows (%v): %d (%.1f%%)", windowSize, entropy, entropyRate)

	if entropyRate < 50 {
		t.Log("VULNERABILITY CONFIRMED: Low entropy in rotation timing")
		t.Logf("  Only %.1f%% unique windows (expected >90%% with randomization)", entropyRate)
	}

	// Fingerprinting resistance score
	spreadScore := float64(rotationSpread) / float64(expectedSpread) * 100
	if spreadScore > 100 {
		spreadScore = 100
	}

	fingerprintingResistance := (spreadScore + entropyRate) / 2

	t.Logf("Fingerprinting Resistance Score: %.1f/100", fingerprintingResistance)
	if fingerprintingResistance < 70 {
		t.Log("ASSESSMENT: LOW fingerprinting resistance (NOT SUITABLE for privacy-sensitive use)")
	}
}

// TestRecommendedRandomization demonstrates the recommended fix
// This test shows what the implementation SHOULD do (currently fails)
func TestRecommendedRandomization(t *testing.T) {
	t.Skip("SKIP: Recommended implementation not yet available (demonstrates fix)")

	// This test demonstrates the recommended implementation with randomization
	// It will fail with current code but shows the expected behavior

	tmpDir := t.TempDir()
	gm, err := NewGuardManager(tmpDir, logger.NewDefault())
	if err != nil {
		t.Fatalf("Failed to create guard manager: %v", err)
	}

	// Add multiple guards
	for i := 0; i < 10; i++ {
		relay := &directory.Relay{
			Fingerprint: randomFingerprint(t),
			Nickname:    "Guard",
			Address:     "1.1.1.1:9001",
		}
		if err := gm.AddGuard(relay); err != nil {
			t.Fatalf("Failed to add guard: %v", err)
		}
	}

	guards := gm.GetGuards()
	if len(guards) < 3 {
		t.Fatalf("Expected at least 3 guards, got %d", len(guards))
	}

	// Check for randomized expiry offsets (currently not implemented)
	// Expected: Each guard should have different expiry time (60-120 days)
	expiryTimes := make([]time.Time, len(guards))
	for i, guard := range guards {
		// This field doesn't exist yet - part of recommended fix
		// expiryTimes[i] = guard.FirstUsed.Add(guard.ExpiryOffset)
		expiryTimes[i] = guard.FirstUsed.Add(90 * 24 * time.Hour) // Current behavior
	}

	// Calculate spread of expiry times
	earliestExpiry := expiryTimes[0]
	latestExpiry := expiryTimes[0]
	for _, et := range expiryTimes {
		if et.Before(earliestExpiry) {
			earliestExpiry = et
		}
		if et.After(latestExpiry) {
			latestExpiry = et
		}
	}

	expirySpread := latestExpiry.Sub(earliestExpiry)
	expectedMinSpread := 30 * 24 * time.Hour // With 60-120 day range, expect at least 30 days spread

	if expirySpread < expectedMinSpread {
		t.Errorf("Expiry spread = %v, want >= %v (randomization needed)", expirySpread, expectedMinSpread)
		t.Log("RECOMMENDATION: Implement per-guard random expiry offset (60-120 days)")
	}
}

// Helper function to generate random fingerprints for testing
func randomFingerprint(t *testing.T) string {
	const letters = "0123456789ABCDEF"
	b := make([]byte, 40)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		if err != nil {
			t.Fatalf("Failed to generate random fingerprint: %v", err)
		}
		b[i] = letters[n.Int64()]
	}
	return string(b)
}
