// Package client provides the high-level Tor client orchestration.
package client

import (
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/config"
)

func TestClientConfigProvider_GetConfigValue(t *testing.T) {
	cfg := &config.Config{
		SocksPort:              9050,
		ControlPort:            9051,
		DataDirectory:          "/tmp/tor",
		CircuitBuildTimeout:    60 * time.Second,
		MaxCircuitDirtiness:    10 * time.Minute,
		NewCircuitPeriod:       30 * time.Second,
		NumEntryGuards:         3,
		UseEntryGuards:         true,
		UseBridges:             false,
		LogLevel:               "info",
		MetricsPort:            9052,
		EnableMetrics:          true,
		ConnLimit:              1000,
		DormantTimeout:         24 * time.Hour,
		ExcludeNodes:           []string{"node1", "node2"},
		ExcludeExitNodes:       []string{"exit1"},
		EnableCircuitPadding:   true,
		PaddingStrategy:        "random",
		PaddingMinInterval:     3 * time.Second,
		PaddingMaxInterval:     10 * time.Second,
		PaddingIdleTimeout:     1 * time.Second,
		PaddingBurstSize:       5,
		EnableRateLimiting:     true,
		SOCKSConnectionsPerSecond: 100.5,
		SOCKSConnectionsBurst:  50,
		MaxConcurrentConnections: 1000,
		EnablePerClientRateLimiting: false,
		PerClientConnectionsPerSecond: 10.0,
		PerClientConnectionsBurst: 5,
		RateLimitCleanupInterval: 300,
		GuardStateBackupCount: 3,
		GuardStateSnapshotInterval: 300,
		GuardStateLockTimeout: 10,
		EnableTracing:         true,
		TracingEndpoint:       "localhost:4317",
		TracingSampleRate:     0.5,
		TracingExporter:       "otlp",
		TracingInsecure:       false,
		TracingTimeout:        10 * time.Second,
		EnableMemoryMonitoring: true,
		MemoryHighWaterMark:   100 * 1024 * 1024,
		MemoryCriticalMark:    200 * 1024 * 1024,
		MemoryMaxGoroutines:   10000,
		MemoryCheckInterval:   30,
		MemoryTriggerGCOnCritical: true,
		EnableCrashRecovery:   true,
		CrashRecoveryCheckpointPath: "/tmp/tor/checkpoint.json",
		CrashRecoveryInterval: 60,
		CrashRecoveryBackupCount: 2,
		EnableProfiling:       true,
		ProfilingPort:         6060,
		ProfilingPath:         "/debug/pprof",
		EnableCPUProfiling:    true,
		EnableHeapProfiling:   true,
		EnableMutexProfile:    false,
		EnableBlockProfile:    false,
		EnableConnectionPooling: true,
		ConnectionPoolMaxIdle: 5,
		ConnectionPoolMaxLife: 10 * time.Minute,
		EnableCircuitPrebuilding: true,
		CircuitPoolMinSize:    2,
		CircuitPoolMaxSize:    10,
		EnableBufferPooling:   true,
		IsolationLevel:        "destination",
		IsolateDestinations:   true,
		IsolateSOCKSAuth:      false,
		IsolateClientPort:     false,
		IsolateClientProtocol: false,
		PaddingDummyTraffic:   false,
	}

	client := &Client{config: cfg}
	provider := &clientConfigProvider{client: client}

	tests := []struct {
		name     string
		key      string
		expected string
		exists   bool
	}{
		{"SocksPort", "SocksPort", "9050", true},
		{"ControlPort", "ControlPort", "9051", true},
		{"DataDirectory", "DataDirectory", "/tmp/tor", true},
		{"CircuitBuildTimeout", "CircuitBuildTimeout", "1m0s", true},
		{"MaxCircuitDirtiness", "MaxCircuitDirtiness", "10m0s", true},
		{"NewCircuitPeriod", "NewCircuitPeriod", "30s", true},
		{"NumEntryGuards", "NumEntryGuards", "3", true},
		{"UseEntryGuards", "UseEntryGuards", "1", true},
		{"UseBridges", "UseBridges", "0", true},
		{"LogLevel", "LogLevel", "info", true},
		{"MetricsPort", "MetricsPort", "9052", true},
		{"EnableMetrics", "EnableMetrics", "1", true},
		{"ConnLimit", "ConnLimit", "1000", true},
		{"DormantTimeout", "DormantTimeout", "24h0m0s", true},
		{"ExcludeNodes", "ExcludeNodes", "node1,node2", true},
		{"ExcludeExitNodes", "ExcludeExitNodes", "exit1", true},
		{"EnableCircuitPadding", "EnableCircuitPadding", "1", true},
		{"PaddingStrategy", "PaddingStrategy", "random", true},
		{"PaddingMinInterval", "PaddingMinInterval", "3s", true},
		{"PaddingMaxInterval", "PaddingMaxInterval", "10s", true},
		{"PaddingIdleTimeout", "PaddingIdleTimeout", "1s", true},
		{"PaddingBurstSize", "PaddingBurstSize", "5", true},
		{"EnableRateLimiting", "EnableRateLimiting", "1", true},
		{"SOCKSConnectionsPerSecond", "SOCKSConnectionsPerSecond", "100.50", true},
		{"SOCKSConnectionsBurst", "SOCKSConnectionsBurst", "50", true},
		{"MaxConcurrentConnections", "MaxConcurrentConnections", "1000", true},
		{"EnablePerClientRateLimiting", "EnablePerClientRateLimiting", "0", true},
		{"PerClientConnectionsPerSecond", "PerClientConnectionsPerSecond", "10.00", true},
		{"PerClientConnectionsBurst", "PerClientConnectionsBurst", "5", true},
		{"RateLimitCleanupInterval", "RateLimitCleanupInterval", "300", true},
		{"GuardStateBackupCount", "GuardStateBackupCount", "3", true},
		{"GuardStateSnapshotInterval", "GuardStateSnapshotInterval", "300", true},
		{"GuardStateLockTimeout", "GuardStateLockTimeout", "10", true},
		{"EnableTracing", "EnableTracing", "1", true},
		{"TracingEndpoint", "TracingEndpoint", "localhost:4317", true},
		{"TracingSampleRate", "TracingSampleRate", "0.50", true},
		{"TracingExporter", "TracingExporter", "otlp", true},
		{"TracingInsecure", "TracingInsecure", "0", true},
		{"TracingTimeout", "TracingTimeout", "10s", true},
		{"EnableMemoryMonitoring", "EnableMemoryMonitoring", "1", true},
		{"MemoryHighWaterMark", "MemoryHighWaterMark", "104857600", true},
		{"MemoryCriticalMark", "MemoryCriticalMark", "209715200", true},
		{"MemoryMaxGoroutines", "MemoryMaxGoroutines", "10000", true},
		{"MemoryCheckInterval", "MemoryCheckInterval", "30", true},
		{"MemoryTriggerGCOnCritical", "MemoryTriggerGCOnCritical", "1", true},
		{"EnableCrashRecovery", "EnableCrashRecovery", "1", true},
		{"CrashRecoveryCheckpointPath", "CrashRecoveryCheckpointPath", "/tmp/tor/checkpoint.json", true},
		{"CrashRecoveryInterval", "CrashRecoveryInterval", "60", true},
		{"CrashRecoveryBackupCount", "CrashRecoveryBackupCount", "2", true},
		{"EnableProfiling", "EnableProfiling", "1", true},
		{"ProfilingPort", "ProfilingPort", "6060", true},
		{"ProfilingPath", "ProfilingPath", "/debug/pprof", true},
		{"EnableCPUProfiling", "EnableCPUProfiling", "1", true},
		{"EnableHeapProfiling", "EnableHeapProfiling", "1", true},
		{"EnableMutexProfile", "EnableMutexProfile", "0", true},
		{"EnableBlockProfile", "EnableBlockProfile", "0", true},
		{"EnableConnectionPooling", "EnableConnectionPooling", "1", true},
		{"ConnectionPoolMaxIdle", "ConnectionPoolMaxIdle", "5", true},
		{"ConnectionPoolMaxLife", "ConnectionPoolMaxLife", "10m0s", true},
		{"EnableCircuitPrebuilding", "EnableCircuitPrebuilding", "1", true},
		{"CircuitPoolMinSize", "CircuitPoolMinSize", "2", true},
		{"CircuitPoolMaxSize", "CircuitPoolMaxSize", "10", true},
		{"EnableBufferPooling", "EnableBufferPooling", "1", true},
		{"IsolationLevel", "IsolationLevel", "destination", true},
		{"IsolateDestinations", "IsolateDestinations", "1", true},
		{"IsolateSOCKSAuth", "IsolateSOCKSAuth", "0", true},
		{"IsolateClientPort", "IsolateClientPort", "0", true},
		{"IsolateClientProtocol", "IsolateClientProtocol", "0", true},
		{"PaddingDummyTraffic", "PaddingDummyTraffic", "0", true},
		{"UnknownKey", "UnknownKey", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, exists := provider.GetConfigValue(tt.key)
			if exists != tt.exists {
				t.Errorf("GetConfigValue(%q) exists = %v, want %v", tt.key, exists, tt.exists)
			}
			if exists && value != tt.expected {
				t.Errorf("GetConfigValue(%q) = %q, want %q", tt.key, value, tt.expected)
			}
		})
	}
}

func TestClientConfigProvider_GetConfigValue_NilConfig(t *testing.T) {
	client := &Client{config: nil}
	provider := &clientConfigProvider{client: client}

	value, exists := provider.GetConfigValue("SocksPort")
	if exists {
		t.Errorf("GetConfigValue with nil config should return exists=false, got %v", exists)
	}
	if value != "" {
		t.Errorf("GetConfigValue with nil config should return empty string, got %q", value)
	}
}

func TestClientConfigProvider_SetConfigValue(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		value     string
		wantErr   bool
		validate  func(*testing.T, *config.Config)
	}{
		{
			name:    "LogLevel valid",
			key:     "LogLevel",
			value:   "debug",
			wantErr: false,
			validate: func(t *testing.T, cfg *config.Config) {
				if cfg.LogLevel != "debug" {
					t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "debug")
				}
			},
		},
		{
			name:    "LogLevel invalid",
			key:     "LogLevel",
			value:   "invalid",
			wantErr: true,
		},
		{
			name:    "MaxCircuitDirtiness valid",
			key:     "MaxCircuitDirtiness",
			value:   "5m",
			wantErr: false,
			validate: func(t *testing.T, cfg *config.Config) {
				if cfg.MaxCircuitDirtiness != 5*time.Minute {
					t.Errorf("MaxCircuitDirtiness = %v, want %v", cfg.MaxCircuitDirtiness, 5*time.Minute)
				}
			},
		},
		{
			name:    "MaxCircuitDirtiness too small",
			key:     "MaxCircuitDirtiness",
			value:   "10s",
			wantErr: true,
		},
		{
			name:    "NewCircuitPeriod valid",
			key:     "NewCircuitPeriod",
			value:   "1m",
			wantErr: false,
			validate: func(t *testing.T, cfg *config.Config) {
				if cfg.NewCircuitPeriod != 1*time.Minute {
					t.Errorf("NewCircuitPeriod = %v, want %v", cfg.NewCircuitPeriod, 1*time.Minute)
				}
			},
		},
		{
			name:    "NewCircuitPeriod too small",
			key:     "NewCircuitPeriod",
			value:   "5s",
			wantErr: true,
		},
		{
			name:    "CircuitBuildTimeout valid",
			key:     "CircuitBuildTimeout",
			value:   "2m",
			wantErr: false,
			validate: func(t *testing.T, cfg *config.Config) {
				if cfg.CircuitBuildTimeout != 2*time.Minute {
					t.Errorf("CircuitBuildTimeout = %v, want %v", cfg.CircuitBuildTimeout, 2*time.Minute)
				}
			},
		},
		{
			name:    "CircuitBuildTimeout too small",
			key:     "CircuitBuildTimeout",
			value:   "5s",
			wantErr: true,
		},
		{
			name:    "CircuitBuildTimeout too large",
			key:     "CircuitBuildTimeout",
			value:   "10m",
			wantErr: true,
		},
		{
			name:    "DormantTimeout valid",
			key:     "DormantTimeout",
			value:   "1h",
			wantErr: false,
			validate: func(t *testing.T, cfg *config.Config) {
				if cfg.DormantTimeout != 1*time.Hour {
					t.Errorf("DormantTimeout = %v, want %v", cfg.DormantTimeout, 1*time.Hour)
				}
			},
		},
		{
			name:    "DormantTimeout too small",
			key:     "DormantTimeout",
			value:   "30s",
			wantErr: true,
		},
		{
			name:    "ExcludeNodes",
			key:     "ExcludeNodes",
			value:   "node1,node2,node3",
			wantErr: false,
			validate: func(t *testing.T, cfg *config.Config) {
				expected := []string{"node1", "node2", "node3"}
				if len(cfg.ExcludeNodes) != len(expected) {
					t.Errorf("ExcludeNodes length = %d, want %d", len(cfg.ExcludeNodes), len(expected))
				}
				for i, node := range expected {
					if cfg.ExcludeNodes[i] != node {
						t.Errorf("ExcludeNodes[%d] = %q, want %q", i, cfg.ExcludeNodes[i], node)
					}
				}
			},
		},
		{
			name:    "ExcludeNodes empty",
			key:     "ExcludeNodes",
			value:   "",
			wantErr: false,
			validate: func(t *testing.T, cfg *config.Config) {
				if cfg.ExcludeNodes != nil {
					t.Errorf("ExcludeNodes = %v, want nil", cfg.ExcludeNodes)
				}
			},
		},
		{
			name:    "EnableCircuitPadding true",
			key:     "EnableCircuitPadding",
			value:   "1",
			wantErr: false,
			validate: func(t *testing.T, cfg *config.Config) {
				if !cfg.EnableCircuitPadding {
					t.Errorf("EnableCircuitPadding = false, want true")
				}
			},
		},
		{
			name:    "EnableCircuitPadding false",
			key:     "EnableCircuitPadding",
			value:   "0",
			wantErr: false,
			validate: func(t *testing.T, cfg *config.Config) {
				if cfg.EnableCircuitPadding {
					t.Errorf("EnableCircuitPadding = true, want false")
				}
			},
		},
		{
			name:    "PaddingStrategy valid",
			key:     "PaddingStrategy",
			value:   "adaptive",
			wantErr: false,
			validate: func(t *testing.T, cfg *config.Config) {
				if cfg.PaddingStrategy != "adaptive" {
					t.Errorf("PaddingStrategy = %q, want %q", cfg.PaddingStrategy, "adaptive")
				}
			},
		},
		{
			name:    "PaddingStrategy invalid",
			key:     "PaddingStrategy",
			value:   "invalid",
			wantErr: true,
		},
		{
			name:    "PaddingBurstSize valid",
			key:     "PaddingBurstSize",
			value:   "10",
			wantErr: false,
			validate: func(t *testing.T, cfg *config.Config) {
				if cfg.PaddingBurstSize != 10 {
					t.Errorf("PaddingBurstSize = %d, want %d", cfg.PaddingBurstSize, 10)
				}
			},
		},
		{
			name:    "PaddingBurstSize too large",
			key:     "PaddingBurstSize",
			value:   "200",
			wantErr: true,
		},
		{
			name:    "SOCKSConnectionsPerSecond valid",
			key:     "SOCKSConnectionsPerSecond",
			value:   "50.5",
			wantErr: false,
			validate: func(t *testing.T, cfg *config.Config) {
				if cfg.SOCKSConnectionsPerSecond != 50.5 {
					t.Errorf("SOCKSConnectionsPerSecond = %f, want %f", cfg.SOCKSConnectionsPerSecond, 50.5)
				}
			},
		},
		{
			name:    "SOCKSConnectionsPerSecond negative",
			key:     "SOCKSConnectionsPerSecond",
			value:   "-10",
			wantErr: true,
		},
		{
			name:    "TracingSampleRate valid",
			key:     "TracingSampleRate",
			value:   "0.75",
			wantErr: false,
			validate: func(t *testing.T, cfg *config.Config) {
				if cfg.TracingSampleRate != 0.75 {
					t.Errorf("TracingSampleRate = %f, want %f", cfg.TracingSampleRate, 0.75)
				}
			},
		},
		{
			name:    "TracingSampleRate out of range",
			key:     "TracingSampleRate",
			value:   "1.5",
			wantErr: true,
		},
		{
			name:    "SocksPort requires restart",
			key:     "SocksPort",
			value:   "9999",
			wantErr: true,
		},
		{
			name:    "Unknown key",
			key:     "UnknownKey",
			value:   "value",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{}
			client := &Client{config: cfg}
			provider := &clientConfigProvider{client: client}

			err := provider.SetConfigValue(tt.key, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetConfigValue(%q, %q) error = %v, wantErr %v", tt.key, tt.value, err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, cfg)
			}
		})
	}
}

func TestClientConfigProvider_SetConfigValue_NilConfig(t *testing.T) {
	client := &Client{config: nil}
	provider := &clientConfigProvider{client: client}

	err := provider.SetConfigValue("LogLevel", "debug")
	if err == nil {
		t.Errorf("SetConfigValue with nil config should return error")
	}
}

func TestClientConfigProvider_BooleanParsing(t *testing.T) {
	tests := []struct {
		value   string
		wantErr bool
		expect  bool
	}{
		{"1", false, true},
		{"true", false, true},
		{"True", false, true},
		{"TRUE", false, true},
		{"yes", false, true},
		{"Yes", false, true},
		{"YES", false, true},
		{"0", false, false},
		{"false", false, false},
		{"False", false, false},
		{"FALSE", false, false},
		{"no", false, false},
		{"No", false, false},
		{"NO", false, false},
		{"invalid", true, false},
		{"maybe", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			cfg := &config.Config{}
			client := &Client{config: cfg}
			provider := &clientConfigProvider{client: client}

			err := provider.SetConfigValue("EnableCircuitPadding", tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetConfigValue(EnableCircuitPadding, %q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
				return
			}

			if !tt.wantErr && cfg.EnableCircuitPadding != tt.expect {
				t.Errorf("EnableCircuitPadding = %v, want %v", cfg.EnableCircuitPadding, tt.expect)
			}
		})
	}
}
