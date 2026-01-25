// Package connection provides connection-level padding tests.
package connection

import (
	"context"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/cell"
	"github.com/opd-ai/go-tor/pkg/logger"
)

// TestConnectionPaddingConfigValidation tests padding configuration validation.
func TestConnectionPaddingConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *ConnectionPaddingConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: &ConnectionPaddingConfig{
				Strategy:    ConnectionPaddingRandom,
				MinInterval: 3 * time.Second,
				MaxInterval: 10 * time.Second,
				IdleTimeout: time.Second,
			},
			wantErr: false,
		},
		{
			name: "negative MinInterval",
			config: &ConnectionPaddingConfig{
				Strategy:    ConnectionPaddingRandom,
				MinInterval: -1 * time.Second,
				MaxInterval: 10 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "negative MaxInterval",
			config: &ConnectionPaddingConfig{
				Strategy:    ConnectionPaddingRandom,
				MinInterval: 3 * time.Second,
				MaxInterval: -1 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "MaxInterval < MinInterval",
			config: &ConnectionPaddingConfig{
				Strategy:    ConnectionPaddingRandom,
				MinInterval: 10 * time.Second,
				MaxInterval: 3 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "negative IdleTimeout",
			config: &ConnectionPaddingConfig{
				Strategy:    ConnectionPaddingRandom,
				MinInterval: 3 * time.Second,
				MaxInterval: 10 * time.Second,
				IdleTimeout: -1 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "invalid strategy",
			config: &ConnectionPaddingConfig{
				Strategy:    ConnectionPaddingStrategy(99),
				MinInterval: 3 * time.Second,
				MaxInterval: 10 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "zero intervals (disabled)",
			config: &ConnectionPaddingConfig{
				Strategy:    ConnectionPaddingNone,
				MinInterval: 0,
				MaxInterval: 0,
			},
			wantErr: false,
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

// TestConnectionPaddingConfigClone tests configuration cloning.
func TestConnectionPaddingConfigClone(t *testing.T) {
	original := &ConnectionPaddingConfig{
		Strategy:          ConnectionPaddingRandom,
		MinInterval:       3 * time.Second,
		MaxInterval:       10 * time.Second,
		IdleTimeout:       time.Second,
		UseVariableLength: true,
	}

	cloned := original.Clone()

	if cloned == original {
		t.Error("Clone() returned same pointer")
	}

	if cloned.Strategy != original.Strategy {
		t.Error("Strategy not cloned correctly")
	}
	if cloned.MinInterval != original.MinInterval {
		t.Error("MinInterval not cloned correctly")
	}
	if cloned.MaxInterval != original.MaxInterval {
		t.Error("MaxInterval not cloned correctly")
	}
	if cloned.IdleTimeout != original.IdleTimeout {
		t.Error("IdleTimeout not cloned correctly")
	}
	if cloned.UseVariableLength != original.UseVariableLength {
		t.Error("UseVariableLength not cloned correctly")
	}

	// Modify clone and verify original is unchanged
	cloned.MinInterval = 100 * time.Second
	if original.MinInterval == 100*time.Second {
		t.Error("Modifying clone affected original")
	}
}

// TestConnectionPaddingStrategyString tests strategy string representation.
func TestConnectionPaddingStrategyString(t *testing.T) {
	tests := []struct {
		strategy ConnectionPaddingStrategy
		want     string
	}{
		{ConnectionPaddingNone, "none"},
		{ConnectionPaddingFixed, "fixed"},
		{ConnectionPaddingRandom, "random"},
		{ConnectionPaddingAdaptive, "adaptive"},
		{ConnectionPaddingStrategy(99), "unknown(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.strategy.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestNewConnectionPaddingMachine tests padding machine creation.
func TestNewConnectionPaddingMachine(t *testing.T) {
	t.Run("nil connection", func(t *testing.T) {
		_, err := NewConnectionPaddingMachine(nil, nil)
		if err == nil {
			t.Error("Expected error for nil connection")
		}
	})

	t.Run("valid creation with nil config", func(t *testing.T) {
		conn := &Connection{
			address: "127.0.0.1:9001",
			state:   StateOpen,
			logger:  logger.NewDefault(),
		}

		pm, err := NewConnectionPaddingMachine(conn, nil)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if pm == nil {
			t.Fatal("Expected non-nil padding machine")
		}
		if pm.conn != conn {
			t.Error("Connection not set correctly")
		}
		// Should use default config
		if pm.config == nil {
			t.Error("Config should be set to default")
		}
	})

	t.Run("valid creation with custom config", func(t *testing.T) {
		conn := &Connection{
			address: "127.0.0.1:9001",
			state:   StateOpen,
			logger:  logger.NewDefault(),
		}
		config := &ConnectionPaddingConfig{
			Strategy:    ConnectionPaddingFixed,
			MinInterval: 5 * time.Second,
			MaxInterval: 5 * time.Second,
		}

		pm, err := NewConnectionPaddingMachine(conn, config)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if pm.config.Strategy != ConnectionPaddingFixed {
			t.Error("Config not set correctly")
		}
	})

	t.Run("invalid config", func(t *testing.T) {
		conn := &Connection{
			address: "127.0.0.1:9001",
			state:   StateOpen,
			logger:  logger.NewDefault(),
		}
		config := &ConnectionPaddingConfig{
			MinInterval: -1 * time.Second,
		}

		_, err := NewConnectionPaddingMachine(conn, config)
		if err == nil {
			t.Error("Expected error for invalid config")
		}
	})
}

// TestConnectionPaddingMachineStartStop tests starting and stopping the machine.
func TestConnectionPaddingMachineStartStop(t *testing.T) {
	conn := &Connection{
		address: "127.0.0.1:9001",
		state:   StateOpen,
		logger:  logger.NewDefault(),
	}

	pm, err := NewConnectionPaddingMachine(conn, DefaultConnectionPaddingConfig())
	if err != nil {
		t.Fatalf("Failed to create padding machine: %v", err)
	}

	if pm.IsRunning() {
		t.Error("Machine should not be running initially")
	}

	ctx := context.Background()
	if err := pm.Start(ctx); err != nil {
		t.Fatalf("Failed to start padding machine: %v", err)
	}

	if !pm.IsRunning() {
		t.Error("Machine should be running after Start()")
	}

	// Try to start again - should fail
	if err := pm.Start(ctx); err == nil {
		t.Error("Expected error when starting already-running machine")
	}

	pm.Stop()

	// Give it a moment to actually stop
	time.Sleep(50 * time.Millisecond)

	if pm.IsRunning() {
		t.Error("Machine should not be running after Stop()")
	}

	// Stopping again should be safe
	pm.Stop()
}

// TestConnectionPaddingMachineUpdateConfig tests configuration updates.
func TestConnectionPaddingMachineUpdateConfig(t *testing.T) {
	conn := &Connection{
		address: "127.0.0.1:9001",
		state:   StateOpen,
		logger:  logger.NewDefault(),
	}

	pm, _ := NewConnectionPaddingMachine(conn, DefaultConnectionPaddingConfig())

	t.Run("valid update", func(t *testing.T) {
		newConfig := &ConnectionPaddingConfig{
			Strategy:    ConnectionPaddingFixed,
			MinInterval: 10 * time.Second,
			MaxInterval: 10 * time.Second,
		}

		if err := pm.UpdateConfig(newConfig); err != nil {
			t.Errorf("UpdateConfig() error = %v", err)
		}

		config := pm.GetConfig()
		if config.Strategy != ConnectionPaddingFixed {
			t.Error("Config not updated")
		}
	})

	t.Run("nil config", func(t *testing.T) {
		if err := pm.UpdateConfig(nil); err == nil {
			t.Error("Expected error for nil config")
		}
	})

	t.Run("invalid config", func(t *testing.T) {
		badConfig := &ConnectionPaddingConfig{
			MinInterval: -1 * time.Second,
		}
		if err := pm.UpdateConfig(badConfig); err == nil {
			t.Error("Expected error for invalid config")
		}
	})
}

// TestConnectionPaddingMachineRecordActivity tests activity recording.
func TestConnectionPaddingMachineRecordActivity(t *testing.T) {
	conn := &Connection{
		address: "127.0.0.1:9001",
		state:   StateOpen,
		logger:  logger.NewDefault(),
	}

	pm, _ := NewConnectionPaddingMachine(conn, DefaultConnectionPaddingConfig())

	before := time.Now()
	time.Sleep(10 * time.Millisecond)
	pm.RecordActivity()

	pm.mu.RLock()
	lastActivity := pm.lastActivityTime
	bursts := pm.activityBursts
	pm.mu.RUnlock()

	if lastActivity.Before(before) {
		t.Error("Activity time not recorded")
	}

	if bursts != 1 {
		t.Errorf("Expected 1 burst, got %d", bursts)
	}

	// Record multiple times and verify cap
	for i := 0; i < 15; i++ {
		pm.RecordActivity()
	}

	pm.mu.RLock()
	bursts = pm.activityBursts
	pm.mu.RUnlock()

	if bursts > 10 {
		t.Errorf("Bursts should be capped at 10, got %d", bursts)
	}
}

// TestConnectionPaddingMachineStats tests statistics tracking.
func TestConnectionPaddingMachineStats(t *testing.T) {
	conn := &Connection{
		address: "127.0.0.1:9001",
		state:   StateOpen,
		logger:  logger.NewDefault(),
	}

	pm, _ := NewConnectionPaddingMachine(conn, DefaultConnectionPaddingConfig())

	stats := pm.Stats()
	if stats.PaddingsSent != 0 {
		t.Error("Initial PaddingsSent should be 0")
	}
	if stats.VPaddingsSent != 0 {
		t.Error("Initial VPaddingsSent should be 0")
	}
	if stats.FailedPaddings != 0 {
		t.Error("Initial FailedPaddings should be 0")
	}

	// Simulate some padding sends
	pm.paddingsSent.Add(5)
	pm.vpaddingsSent.Add(3)
	pm.failedPaddings.Add(1)

	stats = pm.Stats()
	if stats.PaddingsSent != 5 {
		t.Errorf("Expected PaddingsSent=5, got %d", stats.PaddingsSent)
	}
	if stats.VPaddingsSent != 3 {
		t.Errorf("Expected VPaddingsSent=3, got %d", stats.VPaddingsSent)
	}
	if stats.FailedPaddings != 1 {
		t.Errorf("Expected FailedPaddings=1, got %d", stats.FailedPaddings)
	}
}

// TestConnectionPaddingMachineCalculateNextDelay tests delay calculation.
func TestConnectionPaddingMachineCalculateNextDelay(t *testing.T) {
	conn := &Connection{
		address: "127.0.0.1:9001",
		state:   StateOpen,
		logger:  logger.NewDefault(),
	}

	t.Run("none strategy", func(t *testing.T) {
		config := &ConnectionPaddingConfig{
			Strategy: ConnectionPaddingNone,
		}
		pm, _ := NewConnectionPaddingMachine(conn, config)
		delay := pm.calculateNextDelay()
		if delay != time.Hour {
			t.Errorf("Expected 1 hour delay for None strategy, got %v", delay)
		}
	})

	t.Run("fixed strategy", func(t *testing.T) {
		config := &ConnectionPaddingConfig{
			Strategy:    ConnectionPaddingFixed,
			MinInterval: 5 * time.Second,
		}
		pm, _ := NewConnectionPaddingMachine(conn, config)
		delay := pm.calculateNextDelay()
		if delay != 5*time.Second {
			t.Errorf("Expected 5s delay for Fixed strategy, got %v", delay)
		}
	})

	t.Run("random strategy", func(t *testing.T) {
		config := &ConnectionPaddingConfig{
			Strategy:    ConnectionPaddingRandom,
			MinInterval: 3 * time.Second,
			MaxInterval: 10 * time.Second,
		}
		pm, _ := NewConnectionPaddingMachine(conn, config)

		// Test multiple times to ensure randomness within range
		for i := 0; i < 10; i++ {
			delay := pm.calculateNextDelay()
			if delay < 3*time.Second || delay > 10*time.Second {
				t.Errorf("Delay %v outside expected range [3s, 10s]", delay)
			}
		}
	})

	t.Run("adaptive strategy", func(t *testing.T) {
		config := &ConnectionPaddingConfig{
			Strategy:    ConnectionPaddingAdaptive,
			MinInterval: 3 * time.Second,
			MaxInterval: 10 * time.Second,
		}
		pm, _ := NewConnectionPaddingMachine(conn, config)

		// No bursts - should use shorter delays
		delay := pm.calculateNextDelay()
		if delay > 6*time.Second {
			t.Errorf("Expected shorter delay for quiet period, got %v", delay)
		}

		// Add activity bursts
		pm.activityBursts = 5
		delay = pm.calculateNextDelay()
		if delay < 10*time.Second {
			t.Errorf("Expected longer delay for active period, got %v", delay)
		}
	})
}

// TestConnectionPaddingMachineShouldSendPadding tests padding send decision.
func TestConnectionPaddingMachineShouldSendPadding(t *testing.T) {
	conn := &Connection{
		address: "127.0.0.1:9001",
		logger:  logger.NewDefault(),
	}

	t.Run("none strategy", func(t *testing.T) {
		config := &ConnectionPaddingConfig{
			Strategy: ConnectionPaddingNone,
		}
		pm, _ := NewConnectionPaddingMachine(conn, config)
		conn.setState(StateOpen)

		if pm.shouldSendPadding() {
			t.Error("Should not send padding with None strategy")
		}
	})

	t.Run("connection not open", func(t *testing.T) {
		config := DefaultConnectionPaddingConfig()
		pm, _ := NewConnectionPaddingMachine(conn, config)
		conn.setState(StateClosed)

		if pm.shouldSendPadding() {
			t.Error("Should not send padding when connection not open")
		}
	})

	t.Run("within idle timeout", func(t *testing.T) {
		config := &ConnectionPaddingConfig{
			Strategy:    ConnectionPaddingRandom,
			MinInterval: 3 * time.Second,
			MaxInterval: 10 * time.Second,
			IdleTimeout: 5 * time.Second,
		}
		pm, _ := NewConnectionPaddingMachine(conn, config)
		conn.setState(StateOpen)
		pm.RecordActivity() // Just had activity

		if pm.shouldSendPadding() {
			t.Error("Should not send padding within idle timeout")
		}
	})

	t.Run("idle long enough", func(t *testing.T) {
		config := &ConnectionPaddingConfig{
			Strategy:    ConnectionPaddingRandom,
			MinInterval: 3 * time.Second,
			MaxInterval: 10 * time.Second,
			IdleTimeout: 10 * time.Millisecond,
		}
		pm, _ := NewConnectionPaddingMachine(conn, config)
		conn.setState(StateOpen)
		pm.RecordActivity()

		time.Sleep(50 * time.Millisecond)

		if !pm.shouldSendPadding() {
			t.Error("Should send padding after idle timeout")
		}
	})
}

// TestConnectionPaddingMachineRandomDuration tests random duration generation.
func TestConnectionPaddingMachineRandomDuration(t *testing.T) {
	conn := &Connection{
		address: "127.0.0.1:9001",
		state:   StateOpen,
		logger:  logger.NewDefault(),
	}
	pm, _ := NewConnectionPaddingMachine(conn, DefaultConnectionPaddingConfig())

	t.Run("equal min and max", func(t *testing.T) {
		duration := pm.randomDuration(5*time.Second, 5*time.Second)
		if duration != 5*time.Second {
			t.Errorf("Expected 5s when min==max, got %v", duration)
		}
	})

	t.Run("min > max", func(t *testing.T) {
		duration := pm.randomDuration(10*time.Second, 5*time.Second)
		if duration != 10*time.Second {
			t.Errorf("Expected min when min>max, got %v", duration)
		}
	})

	t.Run("valid range", func(t *testing.T) {
		min := 3 * time.Second
		max := 10 * time.Second

		// Test multiple times to verify range
		for i := 0; i < 100; i++ {
			duration := pm.randomDuration(min, max)
			if duration < min || duration > max {
				t.Errorf("Duration %v outside range [%v, %v]", duration, min, max)
			}
		}
	})
}

// TestConnectionPaddingMachineRandomRange tests random range generation.
func TestConnectionPaddingMachineRandomRange(t *testing.T) {
	conn := &Connection{
		address: "127.0.0.1:9001",
		state:   StateOpen,
		logger:  logger.NewDefault(),
	}
	pm, _ := NewConnectionPaddingMachine(conn, DefaultConnectionPaddingConfig())

	t.Run("equal min and max", func(t *testing.T) {
		val := pm.randomRange(5, 5)
		if val != 5 {
			t.Errorf("Expected 5 when min==max, got %d", val)
		}
	})

	t.Run("min > max", func(t *testing.T) {
		val := pm.randomRange(10, 5)
		if val != 10 {
			t.Errorf("Expected min when min>max, got %d", val)
		}
	})

	t.Run("valid range", func(t *testing.T) {
		min := 100
		max := 500

		// Test multiple times to verify range
		for i := 0; i < 100; i++ {
			val := pm.randomRange(min, max)
			if val < min || val > max {
				t.Errorf("Value %d outside range [%d, %d]", val, min, max)
			}
		}
	})
}

// TestHandleConnectionPaddingCell tests padding cell handling.
func TestHandleConnectionPaddingCell(t *testing.T) {
	// PADDING cells should be silently ignored - just verify no panic
	paddingCell := &cell.Cell{
		CircID:  0,
		Command: cell.CmdPadding,
		Payload: make([]byte, cell.PayloadLen),
	}

	HandleConnectionPaddingCell(paddingCell)

	vpaddingCell := &cell.Cell{
		CircID:  0,
		Command: cell.CmdVPadding,
		Payload: make([]byte, 200),
	}

	HandleConnectionPaddingCell(vpaddingCell)
}

// TestDefaultConnectionPaddingConfig tests default configuration.
func TestDefaultConnectionPaddingConfig(t *testing.T) {
	config := DefaultConnectionPaddingConfig()

	if config == nil {
		t.Fatal("Default config should not be nil")
	}

	if config.Strategy != ConnectionPaddingRandom {
		t.Errorf("Expected Random strategy, got %v", config.Strategy)
	}

	if config.MinInterval <= 0 {
		t.Error("MinInterval should be positive")
	}

	if config.MaxInterval <= config.MinInterval {
		t.Error("MaxInterval should be > MinInterval")
	}

	if config.IdleTimeout <= 0 {
		t.Error("IdleTimeout should be positive")
	}

	if err := config.Validate(); err != nil {
		t.Errorf("Default config should be valid: %v", err)
	}
}
