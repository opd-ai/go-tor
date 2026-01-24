package path

import (
	"testing"

	"github.com/opd-ai/go-tor/pkg/directory"
)

func TestWeightedRandomIndex(t *testing.T) {
	tests := []struct {
		name        string
		relays      []*directory.Relay
		expectError bool
		description string
	}{
		{
			name:        "empty list",
			relays:      []*directory.Relay{},
			expectError: true,
			description: "should error on empty list",
		},
		{
			name: "single relay with bandwidth",
			relays: []*directory.Relay{
				{Nickname: "Relay1", Bandwidth: 1000},
			},
			expectError: false,
			description: "should always select the only relay",
		},
		{
			name: "single relay without bandwidth",
			relays: []*directory.Relay{
				{Nickname: "Relay1", Bandwidth: 0},
			},
			expectError: false,
			description: "should fallback to uniform random for zero bandwidth",
		},
		{
			name: "multiple relays with different bandwidths",
			relays: []*directory.Relay{
				{Nickname: "Relay1", Bandwidth: 1000},
				{Nickname: "Relay2", Bandwidth: 5000},
				{Nickname: "Relay3", Bandwidth: 2000},
			},
			expectError: false,
			description: "should use weighted selection",
		},
		{
			name: "multiple relays all zero bandwidth",
			relays: []*directory.Relay{
				{Nickname: "Relay1", Bandwidth: 0},
				{Nickname: "Relay2", Bandwidth: 0},
				{Nickname: "Relay3", Bandwidth: 0},
			},
			expectError: false,
			description: "should fallback to uniform random",
		},
		{
			name: "multiple relays mixed zero and non-zero bandwidth",
			relays: []*directory.Relay{
				{Nickname: "Relay1", Bandwidth: 0},
				{Nickname: "Relay2", Bandwidth: 5000},
				{Nickname: "Relay3", Bandwidth: 0},
			},
			expectError: false,
			description: "should weight only non-zero bandwidth relays",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx, err := weightedRandomIndex(tt.relays)

			if tt.expectError {
				if err == nil {
					t.Errorf("weightedRandomIndex() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("weightedRandomIndex() unexpected error: %v", err)
				return
			}

			if idx < 0 || idx >= len(tt.relays) {
				t.Errorf("weightedRandomIndex() returned invalid index %d for %d relays",
					idx, len(tt.relays))
			}
		})
	}
}

func TestWeightedRandomIndexDistribution(t *testing.T) {
	// Test that higher bandwidth relays are selected more frequently
	relays := []*directory.Relay{
		{Nickname: "LowBW", Bandwidth: 100},     // 10% of total
		{Nickname: "HighBW", Bandwidth: 900},    // 90% of total
	}

	const iterations = 1000
	selectedCount := map[int]int{0: 0, 1: 0}

	for i := 0; i < iterations; i++ {
		idx, err := weightedRandomIndex(relays)
		if err != nil {
			t.Fatalf("weightedRandomIndex() error: %v", err)
		}
		selectedCount[idx]++
	}

	// HighBW relay should be selected significantly more often
	// Allow some variance, but expect ~9:1 ratio
	lowBWCount := selectedCount[0]
	highBWCount := selectedCount[1]

	// High bandwidth relay should be selected at least 7x more (allow variance)
	if highBWCount < lowBWCount*7 {
		t.Errorf("Bandwidth weighting appears incorrect: LowBW=%d, HighBW=%d (expected ~90%% HighBW)",
			lowBWCount, highBWCount)
	}

	t.Logf("Distribution over %d iterations: LowBW=%d (%.1f%%), HighBW=%d (%.1f%%)",
		iterations,
		lowBWCount, float64(lowBWCount)/float64(iterations)*100,
		highBWCount, float64(highBWCount)/float64(iterations)*100)
}

func TestWeightedRandomIndexFallbackToUniform(t *testing.T) {
	// Test that zero-bandwidth relays fall back to uniform distribution
	relays := []*directory.Relay{
		{Nickname: "Relay1", Bandwidth: 0},
		{Nickname: "Relay2", Bandwidth: 0},
		{Nickname: "Relay3", Bandwidth: 0},
	}

	const iterations = 300
	selectedCount := map[int]int{0: 0, 1: 0, 2: 0}

	for i := 0; i < iterations; i++ {
		idx, err := weightedRandomIndex(relays)
		if err != nil {
			t.Fatalf("weightedRandomIndex() error: %v", err)
		}
		selectedCount[idx]++
	}

	// Each relay should be selected roughly equally (within reasonable variance)
	// With 300 iterations and 3 relays, expect ~100 each, allow 50-150 range
	for i := 0; i < 3; i++ {
		count := selectedCount[i]
		if count < 50 || count > 150 {
			t.Errorf("Uniform distribution appears skewed: Relay%d selected %d times (expected ~100)",
				i, count)
		}
	}

	t.Logf("Uniform distribution over %d iterations: Relay0=%d, Relay1=%d, Relay2=%d",
		iterations, selectedCount[0], selectedCount[1], selectedCount[2])
}

func TestWeightedRandomIndexLargeValues(t *testing.T) {
	// Test with realistic large bandwidth values (MB/s)
	relays := []*directory.Relay{
		{Nickname: "Relay1", Bandwidth: 10_000_000},  // 10 MB/s
		{Nickname: "Relay2", Bandwidth: 100_000_000}, // 100 MB/s
		{Nickname: "Relay3", Bandwidth: 50_000_000},  // 50 MB/s
	}

	// Should not overflow and should select valid indices
	for i := 0; i < 100; i++ {
		idx, err := weightedRandomIndex(relays)
		if err != nil {
			t.Fatalf("weightedRandomIndex() error on iteration %d: %v", i, err)
		}
		if idx < 0 || idx >= len(relays) {
			t.Errorf("weightedRandomIndex() returned invalid index %d", idx)
		}
	}
}

func TestBandwidthWeightedPathSelection(t *testing.T) {
	// Integration test: verify that path selection uses bandwidth weighting
	selector := NewSelector(nil, nil)

	// Create test relays with different bandwidths in different subnets
	testRelays := []*directory.Relay{
		{
			Nickname:    "HighBWGuard",
			Fingerprint: "AAAA",
			Address:     "10.1.1.1",
			Flags:       []string{"Guard", "Running", "Valid", "Stable"},
			Bandwidth:   100_000_000, // 100 MB/s
		},
		{
			Nickname:    "LowBWGuard",
			Fingerprint: "BBBB",
			Address:     "10.2.1.1",
			Flags:       []string{"Guard", "Running", "Valid", "Stable"},
			Bandwidth:   1_000_000, // 1 MB/s
		},
		{
			Nickname:    "MiddleRelay1",
			Fingerprint: "CCCC",
			Address:     "10.3.1.1",
			Flags:       []string{"Running", "Valid"},
			Bandwidth:   50_000_000,
		},
		{
			Nickname:    "MiddleRelay2",
			Fingerprint: "DDDD",
			Address:     "10.4.1.1",
			Flags:       []string{"Running", "Valid"},
			Bandwidth:   5_000_000,
		},
		{
			Nickname:    "ExitRelay1",
			Fingerprint: "EEEE",
			Address:     "10.5.1.1",
			Flags:       []string{"Exit", "Running", "Valid"},
			Bandwidth:   80_000_000,
		},
		{
			Nickname:    "ExitRelay2",
			Fingerprint: "FFFF",
			Address:     "10.6.1.1",
			Flags:       []string{"Exit", "Running", "Valid"},
			Bandwidth:   8_000_000,
		},
	}

	selector.mu.Lock()
	selector.guards = []*directory.Relay{testRelays[0], testRelays[1]}
	selector.relays = testRelays
	selector.mu.Unlock()

	// Select multiple paths and verify bandwidth weighting is applied
	guardSelections := map[string]int{
		"HighBWGuard": 0,
		"LowBWGuard":  0,
	}

	const iterations = 50
	for i := 0; i < iterations; i++ {
		path, err := selector.SelectPath(80) // Port 80 for exit
		if err != nil {
			t.Fatalf("SelectPath() error: %v", err)
		}

		if path == nil || path.Guard == nil {
			t.Fatal("SelectPath() returned nil path or guard")
		}

		guardSelections[path.Guard.Nickname]++
	}

	// HighBWGuard should be selected more often (100x bandwidth)
	highCount := guardSelections["HighBWGuard"]
	lowCount := guardSelections["LowBWGuard"]

	// Expect at least 80% selection rate for high bandwidth guard
	if highCount < iterations*8/10 {
		t.Errorf("Bandwidth weighting not applied: HighBW=%d, LowBW=%d (expected >80%% HighBW)",
			highCount, lowCount)
	}

	t.Logf("Guard selection over %d paths: HighBW=%d (%.1f%%), LowBW=%d (%.1f%%)",
		iterations,
		highCount, float64(highCount)/float64(iterations)*100,
		lowCount, float64(lowCount)/float64(iterations)*100)
}
