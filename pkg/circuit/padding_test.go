package circuit

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/cell"
)

func TestPaddingStrategyString(t *testing.T) {
	tests := []struct {
		strategy PaddingStrategy
		expected string
	}{
		{PaddingStrategyNone, "none"},
		{PaddingStrategyFixed, "fixed"},
		{PaddingStrategyRandom, "random"},
		{PaddingStrategyAdaptive, "adaptive"},
		{PaddingStrategy(99), "unknown(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.strategy.String(); got != tt.expected {
				t.Errorf("String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDefaultPaddingConfig(t *testing.T) {
	config := DefaultPaddingConfig()

	if config.Strategy != PaddingStrategyRandom {
		t.Errorf("Strategy = %v, want %v", config.Strategy, PaddingStrategyRandom)
	}
	if config.MinInterval != 3*time.Second {
		t.Errorf("MinInterval = %v, want %v", config.MinInterval, 3*time.Second)
	}
	if config.MaxInterval != 10*time.Second {
		t.Errorf("MaxInterval = %v, want %v", config.MaxInterval, 10*time.Second)
	}
	if config.IdleTimeout != time.Second {
		t.Errorf("IdleTimeout = %v, want %v", config.IdleTimeout, time.Second)
	}
	if config.DummyTrafficEnabled {
		t.Error("DummyTrafficEnabled = true, want false")
	}
	if config.BurstSize != 1 {
		t.Errorf("BurstSize = %v, want 1", config.BurstSize)
	}
}

func TestPaddingConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *PaddingConfig
		wantErr bool
	}{
		{
			name:    "valid_default",
			config:  DefaultPaddingConfig(),
			wantErr: false,
		},
		{
			name: "valid_none_strategy",
			config: &PaddingConfig{
				Strategy:    PaddingStrategyNone,
				MinInterval: 0,
				MaxInterval: 0,
				IdleTimeout: 0,
				BurstSize:   0,
			},
			wantErr: false,
		},
		{
			name: "invalid_negative_min_interval",
			config: &PaddingConfig{
				Strategy:    PaddingStrategyFixed,
				MinInterval: -1 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "invalid_negative_max_interval",
			config: &PaddingConfig{
				Strategy:    PaddingStrategyFixed,
				MinInterval: time.Second,
				MaxInterval: -1 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "invalid_max_less_than_min",
			config: &PaddingConfig{
				Strategy:    PaddingStrategyRandom,
				MinInterval: 10 * time.Second,
				MaxInterval: 5 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "invalid_negative_idle_timeout",
			config: &PaddingConfig{
				Strategy:    PaddingStrategyFixed,
				IdleTimeout: -1 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "invalid_negative_burst_size",
			config: &PaddingConfig{
				Strategy:  PaddingStrategyFixed,
				BurstSize: -1,
			},
			wantErr: true,
		},
		{
			name: "invalid_strategy",
			config: &PaddingConfig{
				Strategy: PaddingStrategy(99),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPaddingConfigClone(t *testing.T) {
	original := &PaddingConfig{
		Strategy:            PaddingStrategyAdaptive,
		MinInterval:         5 * time.Second,
		MaxInterval:         15 * time.Second,
		IdleTimeout:         2 * time.Second,
		DummyTrafficEnabled: true,
		BurstSize:           3,
	}

	clone := original.Clone()

	// Verify values match
	if clone.Strategy != original.Strategy {
		t.Errorf("Strategy mismatch: got %v, want %v", clone.Strategy, original.Strategy)
	}
	if clone.MinInterval != original.MinInterval {
		t.Errorf("MinInterval mismatch: got %v, want %v", clone.MinInterval, original.MinInterval)
	}
	if clone.MaxInterval != original.MaxInterval {
		t.Errorf("MaxInterval mismatch: got %v, want %v", clone.MaxInterval, original.MaxInterval)
	}

	// Verify independence
	clone.Strategy = PaddingStrategyNone
	if original.Strategy == PaddingStrategyNone {
		t.Error("Clone is not independent from original")
	}
}

func TestNewPaddingMachine(t *testing.T) {
	circuit := NewCircuit(1)
	config := DefaultPaddingConfig()

	// Valid creation
	pm, err := NewPaddingMachine(circuit, config)
	if err != nil {
		t.Fatalf("NewPaddingMachine() error = %v", err)
	}
	if pm == nil {
		t.Fatal("NewPaddingMachine() returned nil")
	}

	// Nil circuit should fail
	_, err = NewPaddingMachine(nil, config)
	if err == nil {
		t.Error("NewPaddingMachine(nil, config) should return error")
	}

	// Nil config uses default
	pm, err = NewPaddingMachine(circuit, nil)
	if err != nil {
		t.Fatalf("NewPaddingMachine(circuit, nil) error = %v", err)
	}
	if pm.config.Strategy != PaddingStrategyRandom {
		t.Error("Nil config should use default")
	}

	// Invalid config should fail
	invalidConfig := &PaddingConfig{
		MinInterval: -1 * time.Second,
	}
	_, err = NewPaddingMachine(circuit, invalidConfig)
	if err == nil {
		t.Error("NewPaddingMachine with invalid config should return error")
	}
}

func TestPaddingMachineStartStop(t *testing.T) {
	circuit := NewCircuit(1)
	circuit.SetState(StateOpen)
	config := &PaddingConfig{
		Strategy:    PaddingStrategyNone, // Disabled to avoid sending
		MinInterval: time.Hour,           // Long interval
	}

	pm, err := NewPaddingMachine(circuit, config)
	if err != nil {
		t.Fatalf("NewPaddingMachine() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start
	if err := pm.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if !pm.IsRunning() {
		t.Error("IsRunning() = false after Start()")
	}

	// Starting again should fail
	if err := pm.Start(ctx); err == nil {
		t.Error("Start() again should return error")
	}

	// Stop
	pm.Stop()
	time.Sleep(10 * time.Millisecond) // Allow goroutine to exit
	if pm.IsRunning() {
		t.Error("IsRunning() = true after Stop()")
	}

	// Stopping again should be safe
	pm.Stop() // Should not panic
}

func TestPaddingMachineUpdateConfig(t *testing.T) {
	circuit := NewCircuit(1)
	pm, err := NewPaddingMachine(circuit, DefaultPaddingConfig())
	if err != nil {
		t.Fatalf("NewPaddingMachine() error = %v", err)
	}

	// Update to new config
	newConfig := &PaddingConfig{
		Strategy:    PaddingStrategyAdaptive,
		MinInterval: 7 * time.Second,
		MaxInterval: 20 * time.Second,
	}
	if err := pm.UpdateConfig(newConfig); err != nil {
		t.Fatalf("UpdateConfig() error = %v", err)
	}

	// Verify update
	got := pm.GetConfig()
	if got.Strategy != PaddingStrategyAdaptive {
		t.Errorf("Strategy = %v, want %v", got.Strategy, PaddingStrategyAdaptive)
	}

	// Nil config should fail
	if err := pm.UpdateConfig(nil); err == nil {
		t.Error("UpdateConfig(nil) should return error")
	}

	// Invalid config should fail
	invalidConfig := &PaddingConfig{Strategy: PaddingStrategy(99)}
	if err := pm.UpdateConfig(invalidConfig); err == nil {
		t.Error("UpdateConfig with invalid config should return error")
	}
}

func TestPaddingMachineStats(t *testing.T) {
	circuit := NewCircuit(1)
	pm, err := NewPaddingMachine(circuit, DefaultPaddingConfig())
	if err != nil {
		t.Fatalf("NewPaddingMachine() error = %v", err)
	}

	stats := pm.Stats()
	if stats.PaddingsSent != 0 {
		t.Errorf("Initial PaddingsSent = %d, want 0", stats.PaddingsSent)
	}
	if stats.DummyDataSent != 0 {
		t.Errorf("Initial DummyDataSent = %d, want 0", stats.DummyDataSent)
	}
	if stats.FailedPaddings != 0 {
		t.Errorf("Initial FailedPaddings = %d, want 0", stats.FailedPaddings)
	}
}

func TestPaddingMachineRecordTrafficBurst(t *testing.T) {
	circuit := NewCircuit(1)
	config := &PaddingConfig{
		Strategy:    PaddingStrategyAdaptive,
		MinInterval: time.Second,
		MaxInterval: 5 * time.Second,
	}
	pm, err := NewPaddingMachine(circuit, config)
	if err != nil {
		t.Fatalf("NewPaddingMachine() error = %v", err)
	}

	// Record bursts
	for i := 0; i < 5; i++ {
		pm.RecordTrafficBurst()
	}

	pm.mu.RLock()
	bursts := pm.trafficBursts
	pm.mu.RUnlock()

	if bursts != 5 {
		t.Errorf("trafficBursts = %d, want 5", bursts)
	}

	// Check cap at 10
	for i := 0; i < 20; i++ {
		pm.RecordTrafficBurst()
	}

	pm.mu.RLock()
	bursts = pm.trafficBursts
	pm.mu.RUnlock()

	if bursts > 10 {
		t.Errorf("trafficBursts = %d, should be capped at 10", bursts)
	}
}

func TestNewPaddingCell(t *testing.T) {
	circuitID := uint32(42)
	paddingCell := NewPaddingCell(circuitID)

	if paddingCell.CircID != circuitID {
		t.Errorf("CircID = %d, want %d", paddingCell.CircID, circuitID)
	}
	if paddingCell.Command != cell.CmdPadding {
		t.Errorf("Command = %v, want %v", paddingCell.Command, cell.CmdPadding)
	}
	if len(paddingCell.Payload) != cell.PayloadLen {
		t.Errorf("Payload length = %d, want %d", len(paddingCell.Payload), cell.PayloadLen)
	}

	// Verify payload is random (not all zeros)
	allZero := true
	for _, b := range paddingCell.Payload {
		if b != 0 {
			allZero = false
			break
		}
	}
	// Note: There's a tiny chance this could fail if rand produces all zeros
	// but that's astronomically unlikely for 509 bytes
	if allZero {
		t.Error("Payload should be random, got all zeros")
	}
}

func TestHandlePaddingCell(t *testing.T) {
	// HandlePaddingCell should not panic
	paddingCell := NewPaddingCell(1)
	HandlePaddingCell(paddingCell) // Should complete without error
}

func TestAddRandomTimingDelay(t *testing.T) {
	// Test with valid range
	start := time.Now()
	AddRandomTimingDelay(10*time.Millisecond, 50*time.Millisecond)
	elapsed := time.Since(start)

	if elapsed < 10*time.Millisecond {
		t.Errorf("Delay too short: %v < 10ms", elapsed)
	}
	if elapsed > 100*time.Millisecond { // Allow some slack
		t.Errorf("Delay too long: %v > 100ms", elapsed)
	}

	// Test with invalid range (should return immediately)
	start = time.Now()
	AddRandomTimingDelay(50*time.Millisecond, 10*time.Millisecond)
	elapsed = time.Since(start)

	if elapsed > 10*time.Millisecond {
		t.Errorf("Invalid range should not delay: %v", elapsed)
	}

	// Test with zero max
	start = time.Now()
	AddRandomTimingDelay(0, 0)
	elapsed = time.Since(start)

	if elapsed > 10*time.Millisecond {
		t.Errorf("Zero range should not delay: %v", elapsed)
	}
}

func TestPaddingMachineShouldSendPadding(t *testing.T) {
	circuit := NewCircuit(1)

	tests := []struct {
		name         string
		state        State
		strategy     PaddingStrategy
		idleTimeout  time.Duration
		lastActivity time.Duration // How long ago
		expected     bool
	}{
		{
			name:         "circuit_not_open",
			state:        StateBuilding,
			strategy:     PaddingStrategyFixed,
			idleTimeout:  time.Second,
			lastActivity: 2 * time.Second,
			expected:     false,
		},
		{
			name:         "strategy_none",
			state:        StateOpen,
			strategy:     PaddingStrategyNone,
			idleTimeout:  time.Second,
			lastActivity: 2 * time.Second,
			expected:     false,
		},
		{
			name:         "circuit_active",
			state:        StateOpen,
			strategy:     PaddingStrategyFixed,
			idleTimeout:  2 * time.Second,
			lastActivity: time.Second, // Active within idle timeout
			expected:     false,
		},
		{
			name:         "should_pad",
			state:        StateOpen,
			strategy:     PaddingStrategyFixed,
			idleTimeout:  time.Second,
			lastActivity: 2 * time.Second, // Idle beyond timeout
			expected:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			circuit.SetState(tt.state)
			circuit.mu.Lock()
			circuit.lastActivityTime = time.Now().Add(-tt.lastActivity)
			circuit.mu.Unlock()

			config := &PaddingConfig{
				Strategy:    tt.strategy,
				MinInterval: time.Second,
				MaxInterval: 5 * time.Second,
				IdleTimeout: tt.idleTimeout,
			}
			pm, _ := NewPaddingMachine(circuit, config)

			if got := pm.shouldSendPadding(); got != tt.expected {
				t.Errorf("shouldSendPadding() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestPaddingMachineRandomDuration(t *testing.T) {
	circuit := NewCircuit(1)
	pm, _ := NewPaddingMachine(circuit, DefaultPaddingConfig())

	min := 100 * time.Millisecond
	max := 500 * time.Millisecond

	// Generate multiple random durations and verify they're in range
	for i := 0; i < 100; i++ {
		d := pm.randomDuration(min, max)
		if d < min {
			t.Errorf("randomDuration() = %v < min %v", d, min)
		}
		if d >= max {
			t.Errorf("randomDuration() = %v >= max %v", d, max)
		}
	}

	// Test when min >= max
	d := pm.randomDuration(max, min)
	if d != max {
		t.Errorf("randomDuration(max, min) = %v, want %v", d, max)
	}
}

func TestPaddingMachineCalculateNextDelay(t *testing.T) {
	circuit := NewCircuit(1)

	tests := []struct {
		name     string
		strategy PaddingStrategy
		minDelay time.Duration
		maxDelay time.Duration
	}{
		{
			name:     "none_strategy",
			strategy: PaddingStrategyNone,
			minDelay: time.Hour, // Returns long delay when disabled
		},
		{
			name:     "fixed_strategy",
			strategy: PaddingStrategyFixed,
			minDelay: 5 * time.Second,
			maxDelay: 5 * time.Second, // Fixed returns exact min
		},
		{
			name:     "random_strategy",
			strategy: PaddingStrategyRandom,
			minDelay: 3 * time.Second,
			maxDelay: 10 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &PaddingConfig{
				Strategy:    tt.strategy,
				MinInterval: tt.minDelay,
				MaxInterval: tt.maxDelay,
			}
			pm, _ := NewPaddingMachine(circuit, config)

			delay := pm.calculateNextDelay()

			if tt.strategy == PaddingStrategyNone {
				if delay < time.Hour {
					t.Errorf("None strategy delay = %v, want >= 1h", delay)
				}
			} else if tt.strategy == PaddingStrategyFixed {
				if delay != tt.minDelay {
					t.Errorf("Fixed strategy delay = %v, want %v", delay, tt.minDelay)
				}
			} else {
				if delay < tt.minDelay {
					t.Errorf("Delay = %v < min %v", delay, tt.minDelay)
				}
			}
		})
	}
}

func TestPaddingMachineConcurrency(t *testing.T) {
	circuit := NewCircuit(1)
	circuit.SetState(StateOpen)
	config := DefaultPaddingConfig()
	config.Strategy = PaddingStrategyNone // Disable actual padding
	pm, _ := NewPaddingMachine(circuit, config)

	var wg sync.WaitGroup
	done := make(chan struct{})

	// Multiple goroutines accessing config
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					pm.GetConfig()
					pm.RecordTrafficBurst()
					_ = pm.Stats()
				}
			}
		}()
	}

	// Multiple goroutines updating config
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					newConfig := DefaultPaddingConfig()
					_ = pm.UpdateConfig(newConfig)
				}
			}
		}()
	}

	// Let it run for a bit
	time.Sleep(100 * time.Millisecond)
	close(done)
	wg.Wait()
}

func TestPaddingMachineContextCancellation(t *testing.T) {
	circuit := NewCircuit(1)
	circuit.SetState(StateOpen)
	config := &PaddingConfig{
		Strategy:    PaddingStrategyFixed,
		MinInterval: time.Hour, // Long interval
	}
	pm, _ := NewPaddingMachine(circuit, config)

	ctx, cancel := context.WithCancel(context.Background())

	if err := pm.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if !pm.IsRunning() {
		t.Error("Machine should be running")
	}

	// Cancel context
	cancel()
	time.Sleep(50 * time.Millisecond) // Allow goroutine to process cancellation

	if pm.IsRunning() {
		t.Error("Machine should stop after context cancellation")
	}
}

func TestPaddingMachineAdaptiveStrategy(t *testing.T) {
	circuit := NewCircuit(1)
	config := &PaddingConfig{
		Strategy:    PaddingStrategyAdaptive,
		MinInterval: 100 * time.Millisecond,
		MaxInterval: 500 * time.Millisecond,
	}
	pm, _ := NewPaddingMachine(circuit, config)

	// Record traffic bursts
	for i := 0; i < 5; i++ {
		pm.RecordTrafficBurst()
	}

	// With bursts, delay should be longer (MaxInterval to MaxInterval*2)
	delay := pm.calculateNextDelay()
	if delay < config.MaxInterval {
		t.Errorf("Adaptive delay with bursts = %v, should be >= %v", delay, config.MaxInterval)
	}

	// Consume all bursts
	for pm.trafficBursts > 0 {
		_ = pm.calculateNextDelay()
	}

	// Without bursts, delay should be shorter (MinInterval to MinInterval*2)
	delay = pm.calculateNextDelay()
	if delay >= config.MaxInterval {
		t.Errorf("Adaptive delay without bursts = %v, should be < %v", delay, config.MaxInterval)
	}
}

func TestPaddingMachineAdaptiveStrategyCap(t *testing.T) {
	circuit := NewCircuit(1)
	// Use very large MaxInterval to test the 5 minute cap
	config := &PaddingConfig{
		Strategy:    PaddingStrategyAdaptive,
		MinInterval: time.Minute,
		MaxInterval: 10 * time.Minute, // Very large to exceed 5 minute cap
	}
	pm, _ := NewPaddingMachine(circuit, config)

	// Record traffic bursts
	pm.RecordTrafficBurst()

	// With bursts and large MaxInterval, delay should be capped at 5 minutes
	delay := pm.calculateNextDelay()
	maxCap := 5 * time.Minute
	if delay > maxCap {
		t.Errorf("Adaptive delay with large MaxInterval = %v, should be <= %v", delay, maxCap)
	}
}
