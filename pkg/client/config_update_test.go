package client

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/config"
	"github.com/opd-ai/go-tor/pkg/logger"
)

// TestSetConfigValue_LiveUpdatableOptions tests runtime-updateable configuration options
func TestSetConfigValue_LiveUpdatableOptions(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		value     string
		wantErr   bool
		errMsg    string
		validator func(*testing.T, *config.Config)
	}{
		{
			name:    "Update MaxCircuitDirtiness - valid duration",
			key:     "MaxCircuitDirtiness",
			value:   "15m",
			wantErr: false,
			validator: func(t *testing.T, cfg *config.Config) {
				if cfg.MaxCircuitDirtiness != 15*time.Minute {
					t.Errorf("MaxCircuitDirtiness = %v, want %v", cfg.MaxCircuitDirtiness, 15*time.Minute)
				}
			},
		},
		{
			name:    "Update MaxCircuitDirtiness - invalid duration",
			key:     "MaxCircuitDirtiness",
			value:   "invalid",
			wantErr: true,
			errMsg:  "invalid duration for MaxCircuitDirtiness",
		},
		{
			name:    "Update MaxCircuitDirtiness - too short",
			key:     "MaxCircuitDirtiness",
			value:   "10s",
			wantErr: true,
			errMsg:  "MaxCircuitDirtiness must be at least 30 seconds",
		},
		{
			name:    "Update NewCircuitPeriod - valid duration",
			key:     "NewCircuitPeriod",
			value:   "45s",
			wantErr: false,
			validator: func(t *testing.T, cfg *config.Config) {
				if cfg.NewCircuitPeriod != 45*time.Second {
					t.Errorf("NewCircuitPeriod = %v, want %v", cfg.NewCircuitPeriod, 45*time.Second)
				}
			},
		},
		{
			name:    "Update NewCircuitPeriod - invalid duration",
			key:     "NewCircuitPeriod",
			value:   "not-a-duration",
			wantErr: true,
			errMsg:  "invalid duration for NewCircuitPeriod",
		},
		{
			name:    "Update NewCircuitPeriod - too short",
			key:     "NewCircuitPeriod",
			value:   "5s",
			wantErr: true,
			errMsg:  "NewCircuitPeriod must be at least 10 seconds",
		},
		{
			name:    "Update CircuitBuildTimeout - valid duration",
			key:     "CircuitBuildTimeout",
			value:   "90s",
			wantErr: false,
			validator: func(t *testing.T, cfg *config.Config) {
				if cfg.CircuitBuildTimeout != 90*time.Second {
					t.Errorf("CircuitBuildTimeout = %v, want %v", cfg.CircuitBuildTimeout, 90*time.Second)
				}
			},
		},
		{
			name:    "Update CircuitBuildTimeout - invalid duration",
			key:     "CircuitBuildTimeout",
			value:   "xyz",
			wantErr: true,
			errMsg:  "invalid duration for CircuitBuildTimeout",
		},
		{
			name:    "Update CircuitBuildTimeout - too short",
			key:     "CircuitBuildTimeout",
			value:   "5s",
			wantErr: true,
			errMsg:  "CircuitBuildTimeout must be at least 10 seconds",
		},
		{
			name:    "Update CircuitBuildTimeout - too long",
			key:     "CircuitBuildTimeout",
			value:   "10m",
			wantErr: true,
			errMsg:  "CircuitBuildTimeout must not exceed 5 minutes",
		},
		{
			name:    "Update LogLevel - valid level",
			key:     "LogLevel",
			value:   "debug",
			wantErr: false,
			validator: func(t *testing.T, cfg *config.Config) {
				if cfg.LogLevel != "debug" {
					t.Errorf("LogLevel = %v, want debug", cfg.LogLevel)
				}
			},
		},
		{
			name:    "Update LogLevel - invalid level",
			key:     "LogLevel",
			value:   "trace",
			wantErr: true,
			errMsg:  "invalid log level",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test client with default config
			cfg := config.DefaultConfig()
			log := logger.New(slog.LevelInfo, os.Stdout)
			client := &Client{
				config: cfg,
				logger: log,
			}
			provider := &clientConfigProvider{client: client}

			// Attempt to set the configuration value
			err := provider.SetConfigValue(tt.key, tt.value)

			// Check error expectations
			if tt.wantErr {
				if err == nil {
					t.Errorf("SetConfigValue() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if tt.errMsg != "" && err.Error() != tt.errMsg {
					// Check if error message contains expected substring
					if len(tt.errMsg) > 0 && len(err.Error()) >= len(tt.errMsg) {
						// Allow partial match for error messages
						if err.Error()[:len(tt.errMsg)] != tt.errMsg {
							t.Errorf("SetConfigValue() error = %v, want error containing %v", err, tt.errMsg)
						}
					} else {
						t.Errorf("SetConfigValue() error = %v, want error containing %v", err, tt.errMsg)
					}
				}
				return
			}

			// No error expected
			if err != nil {
				t.Errorf("SetConfigValue() unexpected error = %v", err)
				return
			}

			// Validate the configuration was updated correctly
			if tt.validator != nil {
				tt.validator(t, cfg)
			}
		})
	}
}

// TestSetConfigValue_ReadOnlyOptions tests that read-only options cannot be updated at runtime
func TestSetConfigValue_ReadOnlyOptions(t *testing.T) {
	readOnlyKeys := []string{
		"SocksPort",
		"ControlPort",
		"DataDirectory",
		"NumEntryGuards",
		"UseEntryGuards",
		"UseBridges",
		"MetricsPort",
		"EnableMetrics",
	}

	cfg := config.DefaultConfig()
	log := logger.New(slog.LevelInfo, os.Stdout)
	client := &Client{
		config: cfg,
		logger: log,
	}
	provider := &clientConfigProvider{client: client}

	for _, key := range readOnlyKeys {
		t.Run(key, func(t *testing.T) {
			err := provider.SetConfigValue(key, "test-value")
			if err == nil {
				t.Errorf("SetConfigValue(%s) error = nil, want error about restart required", key)
				return
			}
			expectedMsg := "requires restart"
			if len(err.Error()) < len(expectedMsg) || err.Error()[len(err.Error())-len(expectedMsg):] != expectedMsg {
				t.Errorf("SetConfigValue(%s) error = %v, want error containing 'requires restart'", key, err)
			}
		})
	}
}

// TestSetConfigValue_UnknownOption tests handling of unknown configuration options
func TestSetConfigValue_UnknownOption(t *testing.T) {
	cfg := config.DefaultConfig()
	log := logger.New(slog.LevelInfo, os.Stdout)
	client := &Client{
		config: cfg,
		logger: log,
	}
	provider := &clientConfigProvider{client: client}

	err := provider.SetConfigValue("UnknownOption", "value")
	if err == nil {
		t.Error("SetConfigValue(UnknownOption) error = nil, want error about unknown option")
		return
	}
	expectedMsg := "unknown configuration option"
	if len(err.Error()) < len(expectedMsg) || err.Error()[:len(expectedMsg)] != expectedMsg {
		t.Errorf("SetConfigValue(UnknownOption) error = %v, want error containing 'unknown configuration option'", err)
	}
}

// TestSetConfigValue_NilConfig tests handling when configuration is not available
func TestSetConfigValue_NilConfig(t *testing.T) {
	client := &Client{
		config: nil, // No configuration
	}
	provider := &clientConfigProvider{client: client}

	err := provider.SetConfigValue("LogLevel", "debug")
	if err == nil {
		t.Error("SetConfigValue() with nil config error = nil, want error")
		return
	}
	expectedMsg := "configuration not available"
	if err.Error() != expectedMsg {
		t.Errorf("SetConfigValue() error = %v, want %v", err, expectedMsg)
	}
}

// TestSetConfigValue_DurationFormats tests various duration format inputs
func TestSetConfigValue_DurationFormats(t *testing.T) {
	tests := []struct {
		key   string
		value string
		want  time.Duration
	}{
		{"MaxCircuitDirtiness", "1h", time.Hour},
		{"MaxCircuitDirtiness", "30m", 30 * time.Minute},
		{"MaxCircuitDirtiness", "1h30m", 90 * time.Minute},
		{"NewCircuitPeriod", "1m", time.Minute},
		{"NewCircuitPeriod", "30s", 30 * time.Second},
		{"CircuitBuildTimeout", "2m", 2 * time.Minute},
		{"CircuitBuildTimeout", "120s", 120 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.key+"="+tt.value, func(t *testing.T) {
			cfg := config.DefaultConfig()
			log := logger.New(slog.LevelInfo, os.Stdout)
			client := &Client{
				config: cfg,
				logger: log,
			}
			provider := &clientConfigProvider{client: client}

			err := provider.SetConfigValue(tt.key, tt.value)
			if err != nil {
				t.Errorf("SetConfigValue() unexpected error = %v", err)
				return
			}

			// Verify the duration was set correctly
			var actual time.Duration
			switch tt.key {
			case "MaxCircuitDirtiness":
				actual = cfg.MaxCircuitDirtiness
			case "NewCircuitPeriod":
				actual = cfg.NewCircuitPeriod
			case "CircuitBuildTimeout":
				actual = cfg.CircuitBuildTimeout
			}

			if actual != tt.want {
				t.Errorf("%s = %v, want %v", tt.key, actual, tt.want)
			}
		})
	}
}

// TestSetConfigValue_LogLevelUpdate tests that LogLevel configuration is updated
func TestSetConfigValue_LogLevelUpdate(t *testing.T) {
	cfg := config.DefaultConfig()
	log := logger.New(slog.LevelInfo, os.Stdout)
	client := &Client{
		config: cfg,
		logger: log,
	}
	provider := &clientConfigProvider{client: client}

	// Update log level to debug
	err := provider.SetConfigValue("LogLevel", "debug")
	if err != nil {
		t.Errorf("SetConfigValue(LogLevel) unexpected error = %v", err)
		return
	}

	// Verify config was updated
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %v, want debug", cfg.LogLevel)
	}

	// Note: Logger level changes require restart since slog.Handler is immutable.
	// This test verifies the configuration is updated correctly.
}
