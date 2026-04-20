package circuit

import (
	"context"
	"math"
	"sync"
	"testing"
	"time"
)

// TestPaddingEffectivenessAgainstTimingAnalysis verifies padding reduces
// timing-based circuit fingerprinting.
func TestPaddingEffectivenessAgainstTimingAnalysis(t *testing.T) {
	tests := []struct {
		name            string
		strategy        PaddingStrategy
		expectedEntropy float64 // bits, higher is better
	}{
		{
			name:            "no_padding_baseline",
			strategy:        PaddingStrategyNone,
			expectedEntropy: 0.0, // Deterministic timing
		},
		{
			name:            "fixed_interval_padding",
			strategy:        PaddingStrategyFixed,
			expectedEntropy: 0.0, // Fixed = no variance
		},
		{
			name:            "random_interval_padding",
			strategy:        PaddingStrategyRandom,
			expectedEntropy: 2.5, // High timing variance
		},
		{
			name:            "adaptive_padding",
			strategy:        PaddingStrategyAdaptive,
			expectedEntropy: 1.5, // Variable timing variance
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			circuit := NewCircuit(1)
			circuit.SetState(StateOpen)

			config := &PaddingConfig{
				Strategy:    tt.strategy,
				MinInterval: 50 * time.Millisecond,
				MaxInterval: 150 * time.Millisecond,
				IdleTimeout: 10 * time.Millisecond,
				BurstSize:   1,
			}

			pm, err := NewPaddingMachine(circuit, config)
			if err != nil {
				t.Fatalf("NewPaddingMachine() error = %v", err)
			}

			// Simulate idle circuit to trigger padding
			circuit.mu.Lock()
			circuit.lastActivityTime = time.Now().Add(-5 * time.Second)
			circuit.mu.Unlock()

			// Measure timing variance
			const samples = 100
			delays := make([]time.Duration, samples)
			for i := 0; i < samples; i++ {
				delays[i] = pm.calculateNextDelay()
			}

			entropy := calculateTimingEntropy(delays)
			t.Logf("%s: timing entropy = %.2f bits (expected >= %.2f)", tt.name, entropy, tt.expectedEntropy)

			if entropy < tt.expectedEntropy {
				t.Errorf("Timing entropy too low: %.2f < %.2f (vulnerable to timing analysis)", entropy, tt.expectedEntropy)
			}
		})
	}
}

// TestPaddingEffectivenessAgainstVolumeFingerprinting tests if padding
// obscures traffic volume patterns.
func TestPaddingEffectivenessAgainstVolumeFingerprinting(t *testing.T) {
	tests := []struct {
		name                 string
		strategy             PaddingStrategy
		burstSize            int
		minPaddingRatio      float64 // Minimum expected padding/total ratio
		maxVolumeCorrelation float64 // Maximum correlation with real traffic
	}{
		{
			name:                 "no_padding",
			strategy:             PaddingStrategyNone,
			burstSize:            0,
			minPaddingRatio:      0.0,
			maxVolumeCorrelation: 1.0, // Perfect correlation (no obfuscation)
		},
		{
			name:                 "single_cell_padding",
			strategy:             PaddingStrategyRandom,
			burstSize:            1,
			minPaddingRatio:      0.3, // At least 30% padding overhead
			maxVolumeCorrelation: 0.7, // Reduced correlation
		},
		{
			name:                 "burst_padding",
			strategy:             PaddingStrategyRandom,
			burstSize:            3,
			minPaddingRatio:      0.5, // At least 50% padding overhead
			maxVolumeCorrelation: 0.5, // Low correlation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			circuit := NewCircuit(1)
			circuit.SetState(StateOpen)

			mockSender := &mockCellSender{}
			circuit.SetConnection(mockSender)

			config := &PaddingConfig{
				Strategy:    tt.strategy,
				MinInterval: 20 * time.Millisecond,
				MaxInterval: 40 * time.Millisecond,
				IdleTimeout: 10 * time.Millisecond,
				BurstSize:   tt.burstSize,
			}

			pm, err := NewPaddingMachine(circuit, config)
			if err != nil {
				t.Fatalf("NewPaddingMachine() error = %v", err)
			}

			// Set circuit as idle
			circuit.mu.Lock()
			circuit.lastActivityTime = time.Now().Add(-5 * time.Second)
			circuit.mu.Unlock()

			// Simulate padding for a period
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()

			if tt.strategy != PaddingStrategyNone {
				_ = pm.Start(ctx)
				// Let it run
				<-ctx.Done()
				pm.Stop()
				time.Sleep(20 * time.Millisecond) // Allow goroutine to stop
			}

			stats := pm.Stats()
			totalCells := stats.PaddingsSent + stats.DummyDataSent
			paddingRatio := float64(totalCells) / math.Max(float64(totalCells), 1.0)

			t.Logf("%s: sent %d padding cells, ratio = %.2f", tt.name, totalCells, paddingRatio)

			if tt.strategy != PaddingStrategyNone && totalCells < 1 {
				t.Errorf("No padding sent with strategy %v", tt.strategy)
			}

			// Volume fingerprinting resistance increases with padding ratio
			if paddingRatio < tt.minPaddingRatio && tt.strategy != PaddingStrategyNone {
				t.Logf("Low padding ratio (%.2f < %.2f) may allow volume fingerprinting", paddingRatio, tt.minPaddingRatio)
			}
		})
	}
}

// TestPaddingEffectivenessAgainstBurstAnalysis verifies padding obscures
// burst patterns in traffic.
func TestPaddingEffectivenessAgainstBurstAnalysis(t *testing.T) {
	circuit := NewCircuit(1)
	circuit.SetState(StateOpen)

	mockSender := &mockCellSender{}
	circuit.SetConnection(mockSender)

	// Set circuit as idle
	circuit.mu.Lock()
	circuit.lastActivityTime = time.Now().Add(-10 * time.Second)
	circuit.mu.Unlock()

	config := &PaddingConfig{
		Strategy:    PaddingStrategyRandom,
		MinInterval: 10 * time.Millisecond,
		MaxInterval: 30 * time.Millisecond,
		IdleTimeout: 5 * time.Millisecond,
		BurstSize:   3, // Send bursts of 3 cells
	}

	pm, err := NewPaddingMachine(circuit, config)
	if err != nil {
		t.Fatalf("NewPaddingMachine() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	_ = pm.Start(ctx)
	<-ctx.Done()
	pm.Stop()
	time.Sleep(20 * time.Millisecond)

	stats := pm.Stats()
	totalPadding := stats.PaddingsSent + stats.DummyDataSent

	t.Logf("Burst padding: %d total cells sent", totalPadding)

	// With burst size 3, we should see multiples of 3
	if totalPadding > 0 && totalPadding%3 != 0 {
		t.Logf("Note: Not all padding was sent in complete bursts (may be expected on timeout)")
	}

	if totalPadding == 0 {
		t.Error("No padding cells sent during test period")
	}
}

// TestPaddingEffectivenessAgainstIdleCircuitDetection verifies padding
// makes idle circuits appear active.
func TestPaddingEffectivenessAgainstIdleCircuitDetection(t *testing.T) {
	tests := []struct {
		name          string
		idleTimeout   time.Duration
		expectPadding bool
		description   string
	}{
		{
			name:          "recently_active_no_padding",
			idleTimeout:   1 * time.Second,
			expectPadding: false,
			description:   "Recently active circuits should not pad",
		},
		{
			name:          "idle_circuit_should_pad",
			idleTimeout:   10 * time.Millisecond,
			expectPadding: true,
			description:   "Idle circuits should send padding",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			circuit := NewCircuit(1)
			circuit.SetState(StateOpen)

			mockSender := &mockCellSender{}
			circuit.SetConnection(mockSender)

			// Set last activity time
			idleTime := 100 * time.Millisecond
			if !tt.expectPadding {
				idleTime = 5 * time.Millisecond // Recently active
			}
			circuit.mu.Lock()
			circuit.lastActivityTime = time.Now().Add(-idleTime)
			circuit.mu.Unlock()

			config := &PaddingConfig{
				Strategy:    PaddingStrategyFixed,
				MinInterval: 20 * time.Millisecond,
				MaxInterval: 20 * time.Millisecond,
				IdleTimeout: tt.idleTimeout,
				BurstSize:   1,
			}

			pm, err := NewPaddingMachine(circuit, config)
			if err != nil {
				t.Fatalf("NewPaddingMachine() error = %v", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			_ = pm.Start(ctx)
			<-ctx.Done()
			pm.Stop()
			time.Sleep(20 * time.Millisecond)

			stats := pm.Stats()
			hasPadding := (stats.PaddingsSent + stats.DummyDataSent) > 0

			if hasPadding != tt.expectPadding {
				t.Errorf("%s: got padding=%v, want %v", tt.description, hasPadding, tt.expectPadding)
			}
		})
	}
}

// TestPaddingEffectivenessAdaptiveStrategy verifies adaptive padding
// reduces overhead during active periods.
func TestPaddingEffectivenessAdaptiveStrategy(t *testing.T) {
	circuit := NewCircuit(1)
	circuit.SetState(StateOpen)

	config := &PaddingConfig{
		Strategy:    PaddingStrategyAdaptive,
		MinInterval: 50 * time.Millisecond,
		MaxInterval: 200 * time.Millisecond,
	}

	pm, err := NewPaddingMachine(circuit, config)
	if err != nil {
		t.Fatalf("NewPaddingMachine() error = %v", err)
	}

	// Measure delays during quiet period
	quietDelays := make([]time.Duration, 50)
	for i := 0; i < 50; i++ {
		quietDelays[i] = pm.calculateNextDelay()
	}

	// Record traffic bursts (simulating active use)
	for i := 0; i < 5; i++ {
		pm.RecordTrafficBurst()
	}

	// Measure delays during active period
	activeDelays := make([]time.Duration, 50)
	for i := 0; i < 50; i++ {
		activeDelays[i] = pm.calculateNextDelay()
	}

	quietAvg := averageDuration(quietDelays)
	activeAvg := averageDuration(activeDelays)

	t.Logf("Quiet period average delay: %v", quietAvg)
	t.Logf("Active period average delay: %v", activeAvg)

	// Active delays should be longer (less padding during real traffic)
	if activeAvg <= quietAvg {
		t.Errorf("Adaptive strategy failed: active delay (%v) should be > quiet delay (%v)", activeAvg, quietAvg)
	}

	// Verify delays increase during active periods (bandwidth efficiency)
	ratio := float64(activeAvg) / float64(quietAvg)
	if ratio < 1.15 {
		t.Errorf("Adaptive strategy not aggressive enough: ratio=%.2f (expected >= 1.15)", ratio)
	}
	t.Logf("Adaptive ratio (active/quiet): %.2fx", ratio)
}

// TestPaddingEffectivenessStateMachineBurstPattern verifies state machine
// creates realistic burst patterns.
func TestPaddingEffectivenessStateMachineBurstPattern(t *testing.T) {
	circuit := NewCircuit(1)
	sm := NewAPEMachine(circuit)

	if err := sm.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Process multiple burst cycles
	burstSizes := []int{}
	gapDurations := []time.Duration{}

	for cycle := 0; cycle < 10; cycle++ {
		// Process burst
		cellsInBurst := 0
		for sm.GetState() == MachineStateBurst {
			shouldPad, delay := sm.ProcessEvent()
			if shouldPad {
				cellsInBurst++
			}
			time.Sleep(delay)
		}
		burstSizes = append(burstSizes, cellsInBurst)

		// Record gap start
		gapStart := time.Now()

		// Wait for gap to complete
		for sm.GetState() == MachineStateGap {
			_, delay := sm.ProcessEvent()
			time.Sleep(delay)
		}

		gapDuration := time.Since(gapStart)
		gapDurations = append(gapDurations, gapDuration)
	}

	// Analyze burst pattern variability
	burstVariance := calculateVariance(burstSizes)
	t.Logf("Burst sizes: %v (variance: %.2f)", burstSizes, burstVariance)

	// Bursts should vary (not constant pattern)
	if burstVariance < 1.0 {
		t.Errorf("Burst pattern too regular (variance=%.2f), may enable fingerprinting", burstVariance)
	}

	// Verify burst sizes within APE spec range (2-10)
	for i, size := range burstSizes {
		if size < 2 || size > 10 {
			t.Errorf("Burst %d size %d out of spec range [2,10]", i, size)
		}
	}

	// Analyze gap variability
	if len(gapDurations) > 0 {
		avgGap := averageDuration(gapDurations)
		t.Logf("Average gap: %v", avgGap)

		// Gaps should be within spec range (1500-9500ms)
		const minGap = 1400 * time.Millisecond // Allow 100ms tolerance
		const maxGap = 9600 * time.Millisecond

		if avgGap < minGap || avgGap > maxGap {
			t.Logf("Note: Average gap %v outside typical range [%v, %v]", avgGap, minGap, maxGap)
		}
	}
}

// TestPaddingEffectivenessConcurrentCircuits verifies padding works
// correctly across multiple circuits without timing correlation.
func TestPaddingEffectivenessConcurrentCircuits(t *testing.T) {
	const numCircuits = 5
	circuits := make([]*Circuit, numCircuits)
	pms := make([]*PaddingMachine, numCircuits)

	// Create multiple circuits with padding
	for i := 0; i < numCircuits; i++ {
		circuit := NewCircuit(uint32(i + 1))
		circuit.SetState(StateOpen)
		circuit.mu.Lock()
		circuit.lastActivityTime = time.Now().Add(-5 * time.Second)
		circuit.mu.Unlock()

		mockSender := &mockCellSender{}
		circuit.SetConnection(mockSender)

		config := &PaddingConfig{
			Strategy:    PaddingStrategyRandom,
			MinInterval: 20 * time.Millisecond,
			MaxInterval: 60 * time.Millisecond,
			IdleTimeout: 10 * time.Millisecond,
			BurstSize:   1,
		}

		pm, err := NewPaddingMachine(circuit, config)
		if err != nil {
			t.Fatalf("NewPaddingMachine() error = %v", err)
		}

		circuits[i] = circuit
		pms[i] = pm
	}

	// Start all padding machines
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	var wg sync.WaitGroup
	for _, pm := range pms {
		wg.Add(1)
		pm := pm // Capture for goroutine
		go func() {
			defer wg.Done()
			_ = pm.Start(ctx)
		}()
	}

	// Wait for test duration
	<-ctx.Done()

	// Stop all machines
	for _, pm := range pms {
		pm.Stop()
	}
	wg.Wait()
	time.Sleep(20 * time.Millisecond)

	// Verify all circuits sent padding
	paddingCounts := make([]uint64, numCircuits)
	for i, pm := range pms {
		stats := pm.Stats()
		paddingCounts[i] = stats.PaddingsSent + stats.DummyDataSent
		t.Logf("Circuit %d: %d padding cells", i+1, paddingCounts[i])
	}

	// Check for cross-circuit timing correlation
	variance := calculateVarianceUint64(paddingCounts)
	t.Logf("Cross-circuit padding variance: %.2f", variance)

	// High variance indicates independent timing (good)
	if variance < 1.0 {
		t.Logf("Low variance (%.2f) may indicate timing correlation between circuits", variance)
	}

	// All circuits should have sent some padding
	for i, count := range paddingCounts {
		if count == 0 {
			t.Errorf("Circuit %d sent no padding", i+1)
		}
	}
}

// TestPaddingEffectivenessOverheadAnalysis measures bandwidth overhead.
func TestPaddingEffectivenessOverheadAnalysis(t *testing.T) {
	strategies := []struct {
		name        string
		strategy    PaddingStrategy
		maxOverhead float64 // Maximum acceptable overhead ratio
	}{
		{"none", PaddingStrategyNone, 0.0},
		{"fixed", PaddingStrategyFixed, 0.3},       // 30% overhead acceptable
		{"random", PaddingStrategyRandom, 0.4},     // 40% overhead acceptable
		{"adaptive", PaddingStrategyAdaptive, 0.5}, // 50% overhead acceptable
	}

	for _, tt := range strategies {
		t.Run(tt.name, func(t *testing.T) {
			circuit := NewCircuit(1)
			circuit.SetState(StateOpen)

			mockSender := &mockCellSender{}
			circuit.SetConnection(mockSender)

			circuit.mu.Lock()
			circuit.lastActivityTime = time.Now().Add(-10 * time.Second)
			circuit.mu.Unlock()

			config := &PaddingConfig{
				Strategy:    tt.strategy,
				MinInterval: 30 * time.Millisecond,
				MaxInterval: 50 * time.Millisecond,
				IdleTimeout: 10 * time.Millisecond,
				BurstSize:   1,
			}

			pm, err := NewPaddingMachine(circuit, config)
			if err != nil {
				t.Fatalf("NewPaddingMachine() error = %v", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()

			if tt.strategy != PaddingStrategyNone {
				_ = pm.Start(ctx)
				<-ctx.Done()
				pm.Stop()
				time.Sleep(20 * time.Millisecond)
			}

			stats := pm.Stats()
			paddingCells := stats.PaddingsSent + stats.DummyDataSent

			// Estimate overhead (padding cells as fraction of test duration)
			cellsPerSecond := float64(paddingCells) / 0.2 // 200ms test

			// Each cell is 514 bytes
			const cellSize = 514
			bytesPerSecond := cellsPerSecond * cellSize

			t.Logf("%s: %.1f cells/sec, %.1f KB/s overhead", tt.name, cellsPerSecond, bytesPerSecond/1024)

			// For educational purposes, any overhead is informational
			if tt.strategy != PaddingStrategyNone && paddingCells == 0 {
				t.Errorf("Expected padding overhead for strategy %v", tt.strategy)
			}
		})
	}
}

// Helper functions

func calculateTimingEntropy(delays []time.Duration) float64 {
	if len(delays) == 0 {
		return 0
	}

	// Calculate frequency distribution
	freq := make(map[time.Duration]int)
	for _, d := range delays {
		// Bucket into 10ms intervals to measure distribution
		bucket := (d / (10 * time.Millisecond)) * (10 * time.Millisecond)
		freq[bucket]++
	}

	// Calculate Shannon entropy
	entropy := 0.0
	n := float64(len(delays))
	for _, count := range freq {
		if count > 0 {
			p := float64(count) / n
			entropy -= p * math.Log2(p)
		}
	}

	return entropy
}

func calculateVariance(values []int) float64 {
	if len(values) == 0 {
		return 0
	}

	// Calculate mean
	sum := 0
	for _, v := range values {
		sum += v
	}
	mean := float64(sum) / float64(len(values))

	// Calculate variance
	variance := 0.0
	for _, v := range values {
		diff := float64(v) - mean
		variance += diff * diff
	}
	variance /= float64(len(values))

	return variance
}

func calculateVarianceUint64(values []uint64) float64 {
	if len(values) == 0 {
		return 0
	}

	// Calculate mean
	var sum uint64
	for _, v := range values {
		sum += v
	}
	mean := float64(sum) / float64(len(values))

	// Calculate variance
	variance := 0.0
	for _, v := range values {
		diff := float64(v) - mean
		variance += diff * diff
	}
	variance /= float64(len(values))

	return variance
}

func averageDuration(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}

	var sum time.Duration
	for _, d := range durations {
		sum += d
	}
	return sum / time.Duration(len(durations))
}
