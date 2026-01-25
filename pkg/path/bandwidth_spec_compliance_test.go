// Package path_test contains specification compliance tests for bandwidth-weighted relay selection
package path

import (
	"testing"

	"github.com/opd-ai/go-tor/pkg/directory"
)

// TestBandwidthWeightingSpecCompliance_Algorithm verifies the bandwidth-weighted selection algorithm
// per path-spec.txt §2.2: "Relays are selected with probability proportional to their bandwidth"
func TestBandwidthWeightingSpecCompliance_Algorithm(t *testing.T) {
	tests := []struct {
		name        string
		relays      []*directory.Relay
		description string
	}{
		{
			name: "equal bandwidth relays",
			relays: []*directory.Relay{
				{Nickname: "Relay1", Bandwidth: 1000},
				{Nickname: "Relay2", Bandwidth: 1000},
				{Nickname: "Relay3", Bandwidth: 1000},
			},
			description: "equal bandwidth should result in uniform distribution",
		},
		{
			name: "proportional bandwidth relays",
			relays: []*directory.Relay{
				{Nickname: "SmallBW", Bandwidth: 100},
				{Nickname: "MediumBW", Bandwidth: 500},
				{Nickname: "LargeBW", Bandwidth: 1000},
			},
			description: "selection probability proportional to bandwidth",
		},
		{
			name: "single dominant relay",
			relays: []*directory.Relay{
				{Nickname: "Tiny1", Bandwidth: 10},
				{Nickname: "Tiny2", Bandwidth: 10},
				{Nickname: "Huge", Bandwidth: 10000},
			},
			description: "high bandwidth relay should dominate selection",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify selection algorithm per path-spec.txt §2.2
			counts := make(map[int]int)
			const iterations = 10000

			for i := 0; i < iterations; i++ {
				idx, err := weightedRandomIndex(tt.relays)
				if err != nil {
					t.Fatalf("weightedRandomIndex() error: %v", err)
				}
				if idx < 0 || idx >= len(tt.relays) {
					t.Fatalf("invalid index %d for %d relays", idx, len(tt.relays))
				}
				counts[idx]++
			}

			// Verify each relay was selected at least once (with high probability)
			// For equal bandwidth, expect roughly equal distribution
			// For proportional bandwidth, expect proportional distribution
			for i := range tt.relays {
				if counts[i] == 0 {
					// Only fail if relay has non-zero bandwidth
					if tt.relays[i].Bandwidth > 0 {
						t.Errorf("relay %s (bw=%d) never selected in %d iterations",
							tt.relays[i].Nickname, tt.relays[i].Bandwidth, iterations)
					}
				}
			}

			// Verify total selections equals iterations
			total := 0
			for _, count := range counts {
				total += count
			}
			if total != iterations {
				t.Errorf("total selections (%d) != iterations (%d)", total, iterations)
			}
		})
	}
}

// TestBandwidthWeightingSpecCompliance_Proportionality verifies selection is proportional to bandwidth
// per path-spec.txt §2.2: "probability proportional to their bandwidth"
func TestBandwidthWeightingSpecCompliance_Proportionality(t *testing.T) {
	tests := []struct {
		name            string
		relays          []*directory.Relay
		expectedRatios  map[int]float64 // expected selection ratio (0.0 - 1.0)
		toleranceMargin float64         // acceptable variance (e.g., 0.05 = ±5%)
	}{
		{
			name: "1:1 ratio",
			relays: []*directory.Relay{
				{Nickname: "Relay1", Bandwidth: 5000},
				{Nickname: "Relay2", Bandwidth: 5000},
			},
			expectedRatios: map[int]float64{
				0: 0.50, // 50%
				1: 0.50, // 50%
			},
			toleranceMargin: 0.05, // ±5%
		},
		{
			name: "1:2 ratio",
			relays: []*directory.Relay{
				{Nickname: "SmallBW", Bandwidth: 1000},
				{Nickname: "LargeBW", Bandwidth: 2000},
			},
			expectedRatios: map[int]float64{
				0: 0.333, // 33.3%
				1: 0.667, // 66.7%
			},
			toleranceMargin: 0.05,
		},
		{
			name: "1:4:5 ratio",
			relays: []*directory.Relay{
				{Nickname: "Small", Bandwidth: 1000},
				{Nickname: "Medium", Bandwidth: 4000},
				{Nickname: "Large", Bandwidth: 5000},
			},
			expectedRatios: map[int]float64{
				0: 0.10, // 10%
				1: 0.40, // 40%
				2: 0.50, // 50%
			},
			toleranceMargin: 0.05,
		},
		{
			name: "1:9 ratio (10x difference)",
			relays: []*directory.Relay{
				{Nickname: "Tiny", Bandwidth: 100},
				{Nickname: "Huge", Bandwidth: 900},
			},
			expectedRatios: map[int]float64{
				0: 0.10, // 10%
				1: 0.90, // 90%
			},
			toleranceMargin: 0.05,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			counts := make(map[int]int)
			const iterations = 100000 // Large sample for statistical accuracy

			for i := 0; i < iterations; i++ {
				idx, err := weightedRandomIndex(tt.relays)
				if err != nil {
					t.Fatalf("weightedRandomIndex() error: %v", err)
				}
				counts[idx]++
			}

			// Verify proportionality for each relay
			for idx, expectedRatio := range tt.expectedRatios {
				actualRatio := float64(counts[idx]) / float64(iterations)
				variance := actualRatio - expectedRatio

				if variance < -tt.toleranceMargin || variance > tt.toleranceMargin {
					t.Errorf("relay %s: expected %.1f%%, got %.1f%% (variance %.1f%%, tolerance ±%.1f%%)",
						tt.relays[idx].Nickname,
						expectedRatio*100,
						actualRatio*100,
						variance*100,
						tt.toleranceMargin*100,
					)
				}
			}
		})
	}
}

// TestBandwidthWeightingSpecCompliance_ZeroBandwidth verifies fallback behavior
// per path-spec.txt §2.2: when bandwidth is unknown, use uniform random
func TestBandwidthWeightingSpecCompliance_ZeroBandwidth(t *testing.T) {
	tests := []struct {
		name          string
		relays        []*directory.Relay
		expectUniform bool
		description   string
	}{
		{
			name: "all zero bandwidth",
			relays: []*directory.Relay{
				{Nickname: "Relay1", Bandwidth: 0},
				{Nickname: "Relay2", Bandwidth: 0},
				{Nickname: "Relay3", Bandwidth: 0},
			},
			expectUniform: true,
			description:   "uniform random when all bandwidth is zero",
		},
		{
			name: "mixed zero and non-zero bandwidth",
			relays: []*directory.Relay{
				{Nickname: "ZeroBW", Bandwidth: 0},
				{Nickname: "NonZeroBW", Bandwidth: 1000},
			},
			expectUniform: false,
			description:   "non-zero bandwidth relay should be heavily favored",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			counts := make(map[int]int)
			const iterations = 10000

			for i := 0; i < iterations; i++ {
				idx, err := weightedRandomIndex(tt.relays)
				if err != nil {
					t.Fatalf("weightedRandomIndex() error: %v", err)
				}
				counts[idx]++
			}

			if tt.expectUniform {
				// Verify uniform distribution (each relay selected roughly equally)
				expectedCount := iterations / len(tt.relays)
				tolerance := float64(iterations) * 0.10 // ±10% variance

				for i, relay := range tt.relays {
					actualCount := counts[i]
					variance := float64(actualCount - expectedCount)

					if variance < -tolerance || variance > tolerance {
						t.Errorf("relay %s: expected ~%d selections, got %d (variance %.1f, tolerance ±%.1f)",
							relay.Nickname, expectedCount, actualCount, variance, tolerance)
					}
				}
			} else {
				// Verify non-zero bandwidth relay is heavily favored
				// In the mixed case, non-zero bandwidth relay should be selected 100% of time
				for i, relay := range tt.relays {
					if relay.Bandwidth > 0 {
						if counts[i] != iterations {
							t.Errorf("relay %s (bw=%d) should be selected 100%% of time, got %.1f%%",
								relay.Nickname, relay.Bandwidth, float64(counts[i])/float64(iterations)*100)
						}
					} else {
						if counts[i] != 0 {
							t.Errorf("relay %s (bw=0) should never be selected, got %d selections",
								relay.Nickname, counts[i])
						}
					}
				}
			}
		})
	}
}

// TestBandwidthWeightingSpecCompliance_LargeValues verifies algorithm handles realistic bandwidth values
// per path-spec.txt §2.2: bandwidth values can be very large (MB/s or KB/s)
func TestBandwidthWeightingSpecCompliance_LargeValues(t *testing.T) {
	tests := []struct {
		name        string
		relays      []*directory.Relay
		description string
	}{
		{
			name: "realistic consensus bandwidth values (KB/s)",
			relays: []*directory.Relay{
				{Nickname: "SlowRelay", Bandwidth: 1_024},       // 1 KB/s
				{Nickname: "FastRelay", Bandwidth: 10_240},      // 10 KB/s
				{Nickname: "VeryFastRelay", Bandwidth: 102_400}, // 100 KB/s
			},
			description: "typical bandwidth range in consensus",
		},
		{
			name: "high-bandwidth relays (MB/s)",
			relays: []*directory.Relay{
				{Nickname: "LowBW", Bandwidth: 1_000_000},    // 1 MB/s
				{Nickname: "MedBW", Bandwidth: 10_000_000},   // 10 MB/s
				{Nickname: "HighBW", Bandwidth: 100_000_000}, // 100 MB/s
			},
			description: "high-bandwidth relay range",
		},
		{
			name: "extreme bandwidth values",
			relays: []*directory.Relay{
				{Nickname: "Tiny", Bandwidth: 1},
				{Nickname: "Normal", Bandwidth: 1_000_000},
				{Nickname: "Massive", Bandwidth: 1_000_000_000}, // 1 GB/s
			},
			description: "extreme bandwidth range",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const iterations = 10000

			for i := 0; i < iterations; i++ {
				idx, err := weightedRandomIndex(tt.relays)
				if err != nil {
					t.Fatalf("iteration %d: weightedRandomIndex() error: %v", i, err)
				}

				if idx < 0 || idx >= len(tt.relays) {
					t.Fatalf("iteration %d: invalid index %d for %d relays", i, idx, len(tt.relays))
				}
			}

			// If we reach here, algorithm handled large values correctly
			t.Logf("Successfully handled %d iterations with bandwidth range %d - %d",
				iterations, tt.relays[0].Bandwidth, tt.relays[len(tt.relays)-1].Bandwidth)
		})
	}
}

// TestBandwidthWeightingSpecCompliance_GuardSelection verifies guards use bandwidth weighting
// per path-spec.txt §2.2: guard selection should be bandwidth-weighted
func TestBandwidthWeightingSpecCompliance_GuardSelection(t *testing.T) {
	selector := NewSelector(nil, nil)

	relays := []*directory.Relay{
		{
			Nickname:    "LowBWGuard",
			Address:     "192.168.1.1",
			ORPort:      9001,
			Flags:       []string{"Guard", "Running", "Valid", "Stable"},
			Bandwidth:   100,
			Fingerprint: "0000000000000000000000000000000000000001",
		},
		{
			Nickname:    "HighBWGuard",
			Address:     "192.168.1.2",
			ORPort:      9001,
			Flags:       []string{"Guard", "Running", "Valid", "Stable"},
			Bandwidth:   10000, // 100x higher bandwidth
			Fingerprint: "0000000000000000000000000000000000000002",
		},
	}

	selector.mu.Lock()
	selector.guards = relays
	selector.relays = relays
	selector.mu.Unlock()

	// Select guards multiple times and verify bandwidth weighting
	counts := make(map[string]int)
	const iterations = 1000

	for i := 0; i < iterations; i++ {
		guard, err := selector.selectGuard()
		if err != nil {
			t.Fatalf("iteration %d: selectGuard() error: %v", i, err)
		}
		counts[guard.Nickname]++
	}

	// Verify high bandwidth guard was selected significantly more
	// With 100:1 bandwidth ratio, expect ~99:1 selection ratio
	highBWCount := counts["HighBWGuard"]
	lowBWCount := counts["LowBWGuard"]

	// High bandwidth guard should be selected at least 90% of the time
	highBWPercentage := float64(highBWCount) / float64(iterations)
	if highBWPercentage < 0.90 {
		t.Errorf("High bandwidth guard should be selected ~99%% of time, got %.1f%% (high=%d, low=%d)",
			highBWPercentage*100, highBWCount, lowBWCount)
	}

	t.Logf("Guard selection with 100:1 bandwidth ratio: HighBW=%.1f%%, LowBW=%.1f%%",
		float64(highBWCount)/float64(iterations)*100,
		float64(lowBWCount)/float64(iterations)*100)
}

// TestBandwidthWeightingSpecCompliance_ExitSelection verifies exits use bandwidth weighting
// per path-spec.txt §2.2: exit selection should be bandwidth-weighted
func TestBandwidthWeightingSpecCompliance_ExitSelection(t *testing.T) {
	selector := NewSelector(nil, nil)

	relays := []*directory.Relay{
		{
			Nickname:    "LowBWExit",
			Address:     "192.168.2.1",
			ORPort:      9001,
			Flags:       []string{"Exit", "Running", "Valid", "Fast"},
			Bandwidth:   500,
			Fingerprint: "0000000000000000000000000000000000000003",
		},
		{
			Nickname:    "HighBWExit",
			Address:     "192.168.2.2",
			ORPort:      9001,
			Flags:       []string{"Exit", "Running", "Valid", "Fast"},
			Bandwidth:   5000, // 10x higher bandwidth
			Fingerprint: "0000000000000000000000000000000000000004",
		},
	}

	selector.mu.Lock()
	selector.relays = relays
	selector.mu.Unlock()

	// Select exits multiple times and verify bandwidth weighting
	counts := make(map[string]int)
	const iterations = 1000

	for i := 0; i < iterations; i++ {
		exit, err := selector.selectExit(0, nil)
		if err != nil {
			t.Fatalf("iteration %d: selectExit() error: %v", i, err)
		}
		counts[exit.Nickname]++
	}

	// Verify high bandwidth exit was selected significantly more
	// With 10:1 bandwidth ratio, expect ~90:10 selection ratio
	highBWCount := counts["HighBWExit"]
	lowBWCount := counts["LowBWExit"]

	// High bandwidth exit should be selected at least 80% of the time
	highBWPercentage := float64(highBWCount) / float64(iterations)
	if highBWPercentage < 0.80 {
		t.Errorf("High bandwidth exit should be selected ~90%% of time, got %.1f%% (high=%d, low=%d)",
			highBWPercentage*100, highBWCount, lowBWCount)
	}

	t.Logf("Exit selection with 10:1 bandwidth ratio: HighBW=%.1f%%, LowBW=%.1f%%",
		float64(highBWCount)/float64(iterations)*100,
		float64(lowBWCount)/float64(iterations)*100)
}

// TestBandwidthWeightingSpecCompliance_MiddleSelection verifies middle relays use bandwidth weighting
// per path-spec.txt §2.2: middle relay selection should be bandwidth-weighted
func TestBandwidthWeightingSpecCompliance_MiddleSelection(t *testing.T) {
	// Create guards and exits for path construction
	guard := &directory.Relay{
		Nickname:    "Guard",
		Address:     "192.168.0.1",
		ORPort:      9001,
		Flags:       []string{"Guard", "Running", "Valid", "Stable"},
		Bandwidth:   1000,
		Fingerprint: "0000000000000000000000000000000000000010",
	}

	exit := &directory.Relay{
		Nickname:    "Exit",
		Address:     "192.168.0.2",
		ORPort:      9001,
		Flags:       []string{"Exit", "Running", "Valid", "Fast"},
		Bandwidth:   1000,
		Fingerprint: "0000000000000000000000000000000000000011",
	}

	// Create middle relay candidates with different bandwidths
	relays := []*directory.Relay{
		guard,
		exit,
		{
			Nickname:    "LowBWMiddle",
			Address:     "192.168.3.1",
			ORPort:      9001,
			Flags:       []string{"Running", "Valid", "Fast"},
			Bandwidth:   200,
			Fingerprint: "0000000000000000000000000000000000000005",
		},
		{
			Nickname:    "HighBWMiddle",
			Address:     "192.168.3.2",
			ORPort:      9001,
			Flags:       []string{"Running", "Valid", "Fast"},
			Bandwidth:   2000, // 10x higher bandwidth
			Fingerprint: "0000000000000000000000000000000000000006",
		},
	}

	selector := NewSelector(nil, nil)

	selector.mu.Lock()
	selector.relays = relays
	selector.mu.Unlock()

	// Select middle relays multiple times and verify bandwidth weighting
	counts := make(map[string]int)
	const iterations = 1000

	for i := 0; i < iterations; i++ {
		middle, err := selector.selectMiddle(guard, exit)
		if err != nil {
			t.Fatalf("iteration %d: selectMiddle() error: %v", i, err)
		}
		counts[middle.Nickname]++
	}

	// Verify high bandwidth middle relay was selected significantly more
	// With 10:1 bandwidth ratio, expect ~90:10 selection ratio
	highBWCount := counts["HighBWMiddle"]
	lowBWCount := counts["LowBWMiddle"]

	// High bandwidth middle should be selected at least 80% of the time
	highBWPercentage := float64(highBWCount) / float64(iterations)
	if highBWPercentage < 0.80 {
		t.Errorf("High bandwidth middle should be selected ~90%% of time, got %.1f%% (high=%d, low=%d)",
			highBWPercentage*100, highBWCount, lowBWCount)
	}

	t.Logf("Middle selection with 10:1 bandwidth ratio: HighBW=%.1f%%, LowBW=%.1f%%",
		float64(highBWCount)/float64(iterations)*100,
		float64(lowBWCount)/float64(iterations)*100)
}

// TestBandwidthWeightingSpecCompliance_EmptyRelayList verifies error handling
func TestBandwidthWeightingSpecCompliance_EmptyRelayList(t *testing.T) {
	_, err := weightedRandomIndex([]*directory.Relay{})
	if err == nil {
		t.Error("weightedRandomIndex() with empty list should return error")
	}
}
