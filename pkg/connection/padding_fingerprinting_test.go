// Package connection provides connection-level padding fingerprinting resistance tests.
// This test suite evaluates the effectiveness of connection padding against
// traffic analysis and connection fingerprinting attacks.
package connection

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/cell"
	"github.com/opd-ai/go-tor/pkg/logger"
)

// TestConnectionPaddingFingerprintingResistance evaluates padding effectiveness
// against connection fingerprinting attacks based on timing patterns.
func TestConnectionPaddingFingerprintingResistance(t *testing.T) {
	t.Run("timing entropy", func(t *testing.T) {
		testTimingEntropyFingerprinting(t)
	})
	t.Run("connection duration", func(t *testing.T) {
		testConnectionDurationFingerprinting(t)
	})
	t.Run("idle period detection", func(t *testing.T) {
		testIdlePeriodDetectionResistance(t)
	})
	t.Run("burst pattern", func(t *testing.T) {
		testBurstPatternFingerprinting(t)
	})
	t.Run("cell size uniformity", func(t *testing.T) {
		testCellSizeUniformity(t)
	})
	t.Run("cross-connection correlation", func(t *testing.T) {
		testCrossConnectionCorrelation(t)
	})
	t.Run("strategy distinguishability", func(t *testing.T) {
		testStrategyDistinguishability(t)
	})
}

// testTimingEntropyFingerprinting verifies that padding intervals have
// sufficient entropy to prevent timing-based fingerprinting.
func testTimingEntropyFingerprinting(t *testing.T) {
	strategies := []struct {
		name           string
		strategy       ConnectionPaddingStrategy
		minInterval    time.Duration
		maxInterval    time.Duration
		minEntropyBits float64
		maxCorrelation float64
	}{
		{
			name:           "random strategy",
			strategy:       ConnectionPaddingRandom,
			minInterval:    100 * time.Millisecond,
			maxInterval:    500 * time.Millisecond,
			minEntropyBits: 4.0, // Expect at least 4 bits entropy
			maxCorrelation: 0.3, // Low correlation between successive intervals
		},
		{
			name:           "adaptive strategy",
			strategy:       ConnectionPaddingAdaptive,
			minInterval:    100 * time.Millisecond,
			maxInterval:    500 * time.Millisecond,
			minEntropyBits: 3.5, // Slightly lower due to adaptive behavior
			maxCorrelation: 0.4,
		},
		{
			name:           "fixed strategy (low entropy)",
			strategy:       ConnectionPaddingFixed,
			minInterval:    200 * time.Millisecond,
			maxInterval:    200 * time.Millisecond,
			minEntropyBits: 0.0, // Expected: zero entropy (deterministic)
			maxCorrelation: 1.0, // Perfect correlation
		},
	}

	for _, tc := range strategies {
		t.Run(tc.name, func(t *testing.T) {
			conn := &Connection{
				address: "127.0.0.1:9001",
				state:   StateOpen,
				logger:  logger.NewDefault(),
			}

			config := &ConnectionPaddingConfig{
				Strategy:    tc.strategy,
				MinInterval: tc.minInterval,
				MaxInterval: tc.maxInterval,
				IdleTimeout: 0, // No idle timeout for testing
			}

			pm, _ := NewConnectionPaddingMachine(conn, config)

			// Collect 1000 delay samples
			samples := make([]time.Duration, 1000)
			for i := 0; i < 1000; i++ {
				samples[i] = pm.calculateNextDelay()
			}

			// Calculate Shannon entropy
			entropy := calculateDurationEntropy(samples, tc.minInterval, tc.maxInterval)

			t.Logf("Strategy: %s, Entropy: %.2f bits (threshold: %.2f bits)",
				tc.strategy, entropy, tc.minEntropyBits)

			if tc.strategy == ConnectionPaddingFixed {
				// Fixed strategy should have zero entropy
				if entropy > 0.1 {
					t.Errorf("Fixed strategy has unexpected entropy: %.2f bits (expected ~0)", entropy)
				}
			} else {
				// Random/adaptive strategies should have high entropy
				if entropy < tc.minEntropyBits {
					t.Errorf("Insufficient timing entropy: %.2f bits (min: %.2f bits)", entropy, tc.minEntropyBits)
				}
			}

			// Calculate autocorrelation (lag-1)
			correlation := calculateAutocorrelation(samples)
			t.Logf("Autocorrelation: %.4f (threshold: %.2f)", correlation, tc.maxCorrelation)

			if math.Abs(correlation) > tc.maxCorrelation {
				t.Errorf("High autocorrelation detected: %.4f (max: %.2f)", correlation, tc.maxCorrelation)
			}
		})
	}
}

// testConnectionDurationFingerprinting verifies that padding obscures
// connection duration patterns that could fingerprint applications.
func testConnectionDurationFingerprinting(t *testing.T) {
	scenarios := []struct {
		name                string
		realDuration        time.Duration
		paddingConfig       *ConnectionPaddingConfig
		expectedObfuscation float64 // Minimum variance introduced (%)
	}{
		{
			name:         "no padding baseline",
			realDuration: 1 * time.Second,
			paddingConfig: &ConnectionPaddingConfig{
				Strategy: ConnectionPaddingNone,
			},
			expectedObfuscation: 0.0, // No obfuscation
		},
		{
			name:         "random padding",
			realDuration: 1 * time.Second,
			paddingConfig: &ConnectionPaddingConfig{
				Strategy:    ConnectionPaddingRandom,
				MinInterval: 50 * time.Millisecond,
				MaxInterval: 150 * time.Millisecond,
				IdleTimeout: 10 * time.Millisecond,
			},
			expectedObfuscation: 3.0, // At least 3% variance
		},
	}

	for _, tc := range scenarios {
		t.Run(tc.name, func(t *testing.T) {
			// Simulate 100 connections with same real duration
			durations := make([]time.Duration, 100)
			for i := 0; i < 100; i++ {
				durations[i] = simulateConnectionWithPadding(tc.realDuration, tc.paddingConfig)
			}

			// Calculate coefficient of variation
			mean := calculateMean(durations)
			stddev := calculateStdDev(durations, mean)
			cv := (stddev / float64(mean)) * 100.0 // Percentage

			t.Logf("Duration variance: %.2f%% (threshold: %.2f%%)", cv, tc.expectedObfuscation)

			if tc.paddingConfig.Strategy == ConnectionPaddingNone {
				// No padding should have minimal variance
				if cv > 1.0 {
					t.Errorf("Unexpected variance with no padding: %.2f%%", cv)
				}
			} else {
				// Padding should introduce significant variance
				if cv < tc.expectedObfuscation {
					t.Errorf("Insufficient duration obfuscation: %.2f%% (min: %.2f%%)",
						cv, tc.expectedObfuscation)
				}
			}
		})
	}
}

// testIdlePeriodDetectionResistance verifies padding prevents detection
// of idle periods which could reveal application behavior.
func testIdlePeriodDetectionResistance(t *testing.T) {
	conn := &Connection{
		address: "127.0.0.1:9001",
		state:   StateOpen,
		logger:  logger.NewDefault(),
	}

	testCases := []struct {
		name          string
		config        *ConnectionPaddingConfig
		idleDuration  time.Duration
		shouldPad     bool
		minPaddingCnt int
	}{
		{
			name: "padding masks idle period",
			config: &ConnectionPaddingConfig{
				Strategy:    ConnectionPaddingRandom,
				MinInterval: 50 * time.Millisecond,
				MaxInterval: 100 * time.Millisecond,
				IdleTimeout: 10 * time.Millisecond,
			},
			idleDuration:  500 * time.Millisecond,
			shouldPad:     true,
			minPaddingCnt: 3, // Expect multiple padding cells during idle
		},
		{
			name: "no padding exposes idle period",
			config: &ConnectionPaddingConfig{
				Strategy: ConnectionPaddingNone,
			},
			idleDuration:  500 * time.Millisecond,
			shouldPad:     false,
			minPaddingCnt: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pm, _ := NewConnectionPaddingMachine(conn, tc.config)

			// Simulate idle period by checking how many times padding would be sent
			paddingCount := 0
			elapsed := time.Duration(0)

			// Set lastActivityTime to past to ensure we're beyond IdleTimeout
			pm.mu.Lock()
			pm.lastActivityTime = time.Now().Add(-1 * time.Second)
			pm.mu.Unlock()

			for elapsed < tc.idleDuration {
				if pm.shouldSendPadding() {
					paddingCount++
				}
				// Advance by delay
				delay := pm.calculateNextDelay()
				if delay > tc.idleDuration {
					delay = tc.idleDuration
				}
				elapsed += delay
			}

			t.Logf("Idle period: %v, Padding cells: %d (min expected: %d)",
				tc.idleDuration, paddingCount, tc.minPaddingCnt)

			if tc.shouldPad {
				if paddingCount < tc.minPaddingCnt {
					t.Errorf("Insufficient padding during idle: %d (min: %d)",
						paddingCount, tc.minPaddingCnt)
				}
			} else {
				if paddingCount > 0 {
					t.Errorf("Unexpected padding with None strategy: %d", paddingCount)
				}
			}
		})
	}
}

// testBurstPatternFingerprinting verifies padding prevents fingerprinting
// via burst detection (sudden increases in cell transmission).
func testBurstPatternFingerprinting(t *testing.T) {
	conn := &Connection{
		address: "127.0.0.1:9001",
		state:   StateOpen,
		logger:  logger.NewDefault(),
	}

	config := &ConnectionPaddingConfig{
		Strategy:    ConnectionPaddingAdaptive,
		MinInterval: 50 * time.Millisecond,
		MaxInterval: 200 * time.Millisecond,
		IdleTimeout: 10 * time.Millisecond,
	}

	pm, _ := NewConnectionPaddingMachine(conn, config)

	// Simulate activity burst
	for i := 0; i < 10; i++ {
		pm.RecordActivity()
	}

	// Measure delay during burst
	burstDelay := pm.calculateNextDelay()

	// Wait for burst to decay
	time.Sleep(300 * time.Millisecond)

	// Measure delay after burst
	quietDelay := pm.calculateNextDelay()

	t.Logf("Burst delay: %v, Quiet delay: %v", burstDelay, quietDelay)

	// Adaptive strategy should reduce padding during bursts
	if burstDelay <= quietDelay {
		t.Errorf("Adaptive padding not working: burst delay (%v) <= quiet delay (%v)",
			burstDelay, quietDelay)
	}

	// Verify burst reduces padding frequency (longer delays during active periods)
	if burstDelay < config.MaxInterval {
		t.Errorf("Burst delay too short: %v (should be >= MaxInterval: %v)",
			burstDelay, config.MaxInterval)
	}
}

// testCellSizeUniformity verifies PADDING cells have uniform size distribution
// to prevent size-based fingerprinting.
func testCellSizeUniformity(t *testing.T) {
	testCases := []struct {
		name            string
		useVariableLen  bool
		expectedEntropy float64 // Shannon entropy in bits
	}{
		{
			name:            "fixed PADDING cells (uniform)",
			useVariableLen:  false,
			expectedEntropy: 0.0, // All same size
		},
		{
			name:            "variable VPADDING cells",
			useVariableLen:  true,
			expectedEntropy: 5.0, // Expect significant variety
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			conn := &Connection{
				address: "127.0.0.1:9001",
				state:   StateOpen,
				logger:  logger.NewDefault(),
			}

			config := &ConnectionPaddingConfig{
				Strategy:          ConnectionPaddingRandom,
				MinInterval:       100 * time.Millisecond,
				MaxInterval:       200 * time.Millisecond,
				UseVariableLength: tc.useVariableLen,
			}

			pm, _ := NewConnectionPaddingMachine(conn, config)

			// Generate 1000 padding sizes
			sizes := make([]int, 1000)
			for i := 0; i < 1000; i++ {
				if tc.useVariableLen {
					sizes[i] = pm.randomRange(100, cell.PayloadLen)
				} else {
					sizes[i] = cell.PayloadLen
				}
			}

			// Calculate size entropy
			entropy := calculateIntEntropy(sizes)
			t.Logf("Cell size entropy: %.2f bits (expected: %.2f bits)", entropy, tc.expectedEntropy)

			if tc.useVariableLen {
				if entropy < tc.expectedEntropy {
					t.Errorf("Insufficient size entropy: %.2f bits (min: %.2f bits)",
						entropy, tc.expectedEntropy)
				}
			} else {
				if entropy > 0.1 {
					t.Errorf("Unexpected entropy for fixed-size cells: %.2f bits", entropy)
				}
			}
		})
	}
}

// testCrossConnectionCorrelation verifies padding prevents correlation
// between multiple connections from the same client.
func testCrossConnectionCorrelation(t *testing.T) {
	config := &ConnectionPaddingConfig{
		Strategy:    ConnectionPaddingRandom,
		MinInterval: 100 * time.Millisecond,
		MaxInterval: 500 * time.Millisecond,
		IdleTimeout: 10 * time.Millisecond,
	}

	// Create 10 independent connections with same config
	numConns := 10
	connections := make([]*ConnectionPaddingMachine, numConns)
	samples := make([][]time.Duration, numConns)

	for i := 0; i < numConns; i++ {
		conn := &Connection{
			address: fmt.Sprintf("127.0.0.1:900%d", i),
			state:   StateOpen,
			logger:  logger.NewDefault(),
		}
		connections[i], _ = NewConnectionPaddingMachine(conn, config)

		// Collect 100 delay samples per connection
		samples[i] = make([]time.Duration, 100)
		for j := 0; j < 100; j++ {
			samples[i][j] = connections[i].calculateNextDelay()
		}
	}

	// Calculate pairwise correlation between connections
	correlations := make([]float64, 0)
	for i := 0; i < numConns; i++ {
		for j := i + 1; j < numConns; j++ {
			corr := calculatePearsonCorrelation(samples[i], samples[j])
			correlations = append(correlations, corr)
		}
	}

	// Calculate mean correlation
	meanCorr := 0.0
	for _, c := range correlations {
		meanCorr += math.Abs(c)
	}
	meanCorr /= float64(len(correlations))

	t.Logf("Mean cross-connection correlation: %.4f (threshold: 0.2)", meanCorr)

	// Expect low correlation (independent randomness)
	if meanCorr > 0.2 {
		t.Errorf("High cross-connection correlation: %.4f (max: 0.2)", meanCorr)
	}
}

// testStrategyDistinguishability verifies that different padding strategies
// are difficult to distinguish via traffic analysis.
func testStrategyDistinguishability(t *testing.T) {
	strategies := []struct {
		name     string
		strategy ConnectionPaddingStrategy
	}{
		{"random", ConnectionPaddingRandom},
		{"adaptive", ConnectionPaddingAdaptive},
	}

	// Collect timing samples for each strategy
	strategySamples := make(map[string][]time.Duration)

	for _, s := range strategies {
		conn := &Connection{
			address: "127.0.0.1:9001",
			state:   StateOpen,
			logger:  logger.NewDefault(),
		}

		config := &ConnectionPaddingConfig{
			Strategy:    s.strategy,
			MinInterval: 100 * time.Millisecond,
			MaxInterval: 500 * time.Millisecond,
			IdleTimeout: 10 * time.Millisecond,
		}

		pm, _ := NewConnectionPaddingMachine(conn, config)

		samples := make([]time.Duration, 500)
		for i := 0; i < 500; i++ {
			samples[i] = pm.calculateNextDelay()
		}
		strategySamples[s.name] = samples
	}

	// Calculate Kolmogorov-Smirnov distance between distributions
	ksDistance := calculateKSDistance(
		strategySamples["random"],
		strategySamples["adaptive"],
	)

	t.Logf("KS distance (random vs adaptive): %.4f (threshold: 0.8)", ksDistance)

	// Note: Adaptive strategy is intentionally different from random (it adapts based on activity).
	// Some distinguishability is acceptable - we just want to ensure the distributions overlap significantly.
	// A KS distance < 0.8 means they share >20% of their distribution.
	if ksDistance > 0.8 {
		t.Errorf("Strategies too distinguishable: KS distance %.4f (max: 0.8)", ksDistance)
	}
}

// Helper functions

// calculateDurationEntropy calculates Shannon entropy of duration samples in bits.
func calculateDurationEntropy(samples []time.Duration, min, max time.Duration) float64 {
	if min >= max {
		return 0.0
	}

	// Discretize into 100 bins
	numBins := 100
	binSize := (max - min) / time.Duration(numBins)
	if binSize == 0 {
		return 0.0
	}

	bins := make([]int, numBins)
	for _, sample := range samples {
		if sample < min {
			sample = min
		}
		if sample > max {
			sample = max
		}
		binIndex := int((sample - min) / binSize)
		if binIndex >= numBins {
			binIndex = numBins - 1
		}
		bins[binIndex]++
	}

	// Calculate Shannon entropy
	entropy := 0.0
	total := float64(len(samples))
	for _, count := range bins {
		if count > 0 {
			p := float64(count) / total
			entropy -= p * math.Log2(p)
		}
	}

	return entropy
}

// calculateIntEntropy calculates Shannon entropy of integer samples.
func calculateIntEntropy(samples []int) float64 {
	// Count frequencies
	freq := make(map[int]int)
	for _, val := range samples {
		freq[val]++
	}

	// Calculate entropy
	entropy := 0.0
	total := float64(len(samples))
	for _, count := range freq {
		if count > 0 {
			p := float64(count) / total
			entropy -= p * math.Log2(p)
		}
	}

	return entropy
}

// calculateAutocorrelation calculates lag-1 autocorrelation coefficient.
func calculateAutocorrelation(samples []time.Duration) float64 {
	if len(samples) < 2 {
		return 0.0
	}

	// Convert to float64 array
	vals := make([]float64, len(samples))
	for i, d := range samples {
		vals[i] = float64(d)
	}

	// Calculate mean
	mean := 0.0
	for _, v := range vals {
		mean += v
	}
	mean /= float64(len(vals))

	// Calculate variance and covariance
	variance := 0.0
	covariance := 0.0
	for i := 0; i < len(vals)-1; i++ {
		variance += (vals[i] - mean) * (vals[i] - mean)
		covariance += (vals[i] - mean) * (vals[i+1] - mean)
	}
	variance += (vals[len(vals)-1] - mean) * (vals[len(vals)-1] - mean)

	if variance == 0 {
		return 0.0
	}

	return covariance / variance
}

// calculateMean calculates mean of duration samples.
func calculateMean(samples []time.Duration) time.Duration {
	if len(samples) == 0 {
		return 0
	}
	sum := time.Duration(0)
	for _, d := range samples {
		sum += d
	}
	return sum / time.Duration(len(samples))
}

// calculateStdDev calculates standard deviation of duration samples.
func calculateStdDev(samples []time.Duration, mean time.Duration) float64 {
	if len(samples) == 0 {
		return 0
	}
	variance := 0.0
	for _, d := range samples {
		diff := float64(d - mean)
		variance += diff * diff
	}
	variance /= float64(len(samples))
	return math.Sqrt(variance)
}

// calculatePearsonCorrelation calculates Pearson correlation coefficient.
func calculatePearsonCorrelation(x, y []time.Duration) float64 {
	if len(x) != len(y) || len(x) == 0 {
		return 0.0
	}

	// Convert to float64
	xf := make([]float64, len(x))
	yf := make([]float64, len(y))
	for i := range x {
		xf[i] = float64(x[i])
		yf[i] = float64(y[i])
	}

	// Calculate means
	meanX, meanY := 0.0, 0.0
	for i := range xf {
		meanX += xf[i]
		meanY += yf[i]
	}
	meanX /= float64(len(xf))
	meanY /= float64(len(yf))

	// Calculate correlation
	numerator := 0.0
	denomX := 0.0
	denomY := 0.0
	for i := range xf {
		dx := xf[i] - meanX
		dy := yf[i] - meanY
		numerator += dx * dy
		denomX += dx * dx
		denomY += dy * dy
	}

	if denomX == 0 || denomY == 0 {
		return 0.0
	}

	return numerator / math.Sqrt(denomX*denomY)
}

// calculateKSDistance calculates Kolmogorov-Smirnov distance between two distributions.
func calculateKSDistance(x, y []time.Duration) float64 {
	// Convert to sorted float64 arrays
	xf := make([]float64, len(x))
	yf := make([]float64, len(y))
	for i := range x {
		xf[i] = float64(x[i])
		yf[i] = float64(y[i])
	}
	sort.Float64s(xf)
	sort.Float64s(yf)

	// Calculate empirical CDFs and max distance
	maxDist := 0.0
	i, j := 0, 0
	for i < len(xf) && j < len(yf) {
		cdfX := float64(i+1) / float64(len(xf))
		cdfY := float64(j+1) / float64(len(yf))
		dist := math.Abs(cdfX - cdfY)
		if dist > maxDist {
			maxDist = dist
		}

		if xf[i] < yf[j] {
			i++
		} else {
			j++
		}
	}

	return maxDist
}

// simulateConnectionWithPadding simulates a connection with padding and returns total duration.
func simulateConnectionWithPadding(realDuration time.Duration, config *ConnectionPaddingConfig) time.Duration {
	conn := &Connection{
		address: "127.0.0.1:9001",
		state:   StateOpen,
		logger:  logger.NewDefault(),
	}

	pm, _ := NewConnectionPaddingMachine(conn, config)

	// Simulate connection with padding overhead
	paddingCells := 0
	elapsed := time.Duration(0)

	for elapsed < realDuration {
		delay := pm.calculateNextDelay()
		elapsed += delay

		// Each padding cell adds a small overhead to connection duration
		if pm.shouldSendPadding() {
			paddingCells++
		}
	}

	// Add variance based on number of padding cells sent
	// Each padding cell adds random jitter (0-5ms) to total duration
	if config.Strategy != ConnectionPaddingNone && paddingCells > 0 {
		jitterPerCell := time.Millisecond * time.Duration(pm.randomRange(0, 5))
		elapsed += jitterPerCell * time.Duration(paddingCells)
	}

	return elapsed
}

// TestConnectionPaddingConcurrentFingerprinting verifies padding machine
// is thread-safe under concurrent fingerprinting attempts.
func TestConnectionPaddingConcurrentFingerprinting(t *testing.T) {
	conn := &Connection{
		address: "127.0.0.1:9001",
		state:   StateOpen,
		logger:  logger.NewDefault(),
	}

	config := &ConnectionPaddingConfig{
		Strategy:    ConnectionPaddingRandom,
		MinInterval: 50 * time.Millisecond,
		MaxInterval: 200 * time.Millisecond,
		IdleTimeout: 10 * time.Millisecond,
	}

	pm, _ := NewConnectionPaddingMachine(conn, config)

	// Start padding machine
	ctx := context.Background()
	if err := pm.Start(ctx); err != nil {
		t.Fatalf("Failed to start padding machine: %v", err)
	}
	defer pm.Stop()

	// Simulate concurrent fingerprinting attempts
	var wg sync.WaitGroup
	numGoroutines := 10
	samplesPerGoroutine := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < samplesPerGoroutine; j++ {
				_ = pm.calculateNextDelay()
				pm.RecordActivity()
				_ = pm.GetConfig()
				_ = pm.Stats()
			}
		}()
	}

	wg.Wait()

	// Verify no data races (test passes if no panic)
	stats := pm.Stats()
	t.Logf("Concurrent test complete. Stats: %+v", stats)
}

// TestConnectionPaddingStrategyTransitions verifies fingerprinting resistance
// is maintained during configuration changes.
func TestConnectionPaddingStrategyTransitions(t *testing.T) {
	conn := &Connection{
		address: "127.0.0.1:9001",
		state:   StateOpen,
		logger:  logger.NewDefault(),
	}

	pm, _ := NewConnectionPaddingMachine(conn, &ConnectionPaddingConfig{
		Strategy:    ConnectionPaddingRandom,
		MinInterval: 100 * time.Millisecond,
		MaxInterval: 300 * time.Millisecond,
	})

	// Collect samples before transition
	samplesBefore := make([]time.Duration, 100)
	for i := 0; i < 100; i++ {
		samplesBefore[i] = pm.calculateNextDelay()
	}

	// Transition to adaptive strategy
	newConfig := &ConnectionPaddingConfig{
		Strategy:    ConnectionPaddingAdaptive,
		MinInterval: 100 * time.Millisecond,
		MaxInterval: 300 * time.Millisecond,
	}
	if err := pm.UpdateConfig(newConfig); err != nil {
		t.Fatalf("Failed to update config: %v", err)
	}

	// Collect samples after transition
	samplesAfter := make([]time.Duration, 100)
	for i := 0; i < 100; i++ {
		samplesAfter[i] = pm.calculateNextDelay()
	}

	// Verify both distributions have sufficient entropy
	entropyBefore := calculateDurationEntropy(samplesBefore, 100*time.Millisecond, 300*time.Millisecond)
	entropyAfter := calculateDurationEntropy(samplesAfter, 100*time.Millisecond, 300*time.Millisecond)

	t.Logf("Entropy before: %.2f bits, after: %.2f bits", entropyBefore, entropyAfter)

	if entropyBefore < 3.0 {
		t.Errorf("Insufficient entropy before transition: %.2f bits", entropyBefore)
	}
	if entropyAfter < 3.0 {
		t.Errorf("Insufficient entropy after transition: %.2f bits", entropyAfter)
	}

	// Verify transition didn't introduce highly detectable pattern
	// Note: Some change is expected since adaptive behaves differently than random.
	// We just want to ensure the transition isn't creating an obvious fingerprint.
	ksDistance := calculateKSDistance(samplesBefore, samplesAfter)
	t.Logf("KS distance (before vs after): %.4f (threshold: 0.6)", ksDistance)

	if ksDistance > 0.6 {
		t.Errorf("Strategy transition too detectable: KS distance %.4f", ksDistance)
	}
}
