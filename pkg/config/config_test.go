package config

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg == nil {
		t.Fatal("DefaultConfig() returned nil")
	}

	// Verify some defaults
	if cfg.SocksPort != 9050 {
		t.Errorf("SocksPort = %v, want 9050", cfg.SocksPort)
	}
	if cfg.ControlPort != 9051 {
		t.Errorf("ControlPort = %v, want 9051", cfg.ControlPort)
	}
	if cfg.UseEntryGuards != true {
		t.Error("UseEntryGuards = false, want true")
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %v, want info", cfg.LogLevel)
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Config)
		wantErr bool
	}{
		{
			name:    "valid default config",
			modify:  func(c *Config) {},
			wantErr: false,
		},
		{
			name: "invalid SocksPort negative",
			modify: func(c *Config) {
				c.SocksPort = -1
			},
			wantErr: true,
		},
		{
			name: "invalid SocksPort too large",
			modify: func(c *Config) {
				c.SocksPort = 70000
			},
			wantErr: true,
		},
		{
			name: "invalid ControlPort",
			modify: func(c *Config) {
				c.ControlPort = -1
			},
			wantErr: true,
		},
		{
			name: "invalid CircuitBuildTimeout",
			modify: func(c *Config) {
				c.CircuitBuildTimeout = 0
			},
			wantErr: true,
		},
		{
			name: "invalid MaxCircuitDirtiness",
			modify: func(c *Config) {
				c.MaxCircuitDirtiness = -1 * time.Second
			},
			wantErr: true,
		},
		{
			name: "invalid NumEntryGuards",
			modify: func(c *Config) {
				c.NumEntryGuards = 0
			},
			wantErr: true,
		},
		{
			name: "invalid ConnLimit",
			modify: func(c *Config) {
				c.ConnLimit = 0
			},
			wantErr: true,
		},
		{
			name: "invalid LogLevel",
			modify: func(c *Config) {
				c.LogLevel = "invalid"
			},
			wantErr: true,
		},
		{
			name: "valid LogLevel debug",
			modify: func(c *Config) {
				c.LogLevel = "debug"
			},
			wantErr: false,
		},
		{
			name: "invalid onion service VirtualPort",
			modify: func(c *Config) {
				c.OnionServices = []OnionServiceConfig{
					{VirtualPort: 0, TargetAddr: "localhost:8080", ServiceDir: "/tmp/os"},
				}
			},
			wantErr: true,
		},
		{
			name: "invalid onion service missing TargetAddr",
			modify: func(c *Config) {
				c.OnionServices = []OnionServiceConfig{
					{VirtualPort: 80, TargetAddr: "", ServiceDir: "/tmp/os"},
				}
			},
			wantErr: true,
		},
		{
			name: "invalid onion service missing ServiceDir",
			modify: func(c *Config) {
				c.OnionServices = []OnionServiceConfig{
					{VirtualPort: 80, TargetAddr: "localhost:8080", ServiceDir: ""},
				}
			},
			wantErr: true,
		},
		{
			name: "valid onion service",
			modify: func(c *Config) {
				c.OnionServices = []OnionServiceConfig{
					{VirtualPort: 80, TargetAddr: "localhost:8080", ServiceDir: "/tmp/os"},
				}
			},
			wantErr: false,
		},
		{
			name: "invalid MetricsPort negative",
			modify: func(c *Config) {
				c.MetricsPort = -1
			},
			wantErr: true,
		},
		{
			name: "invalid MetricsPort too large",
			modify: func(c *Config) {
				c.MetricsPort = 65536
			},
			wantErr: true,
		},
		{
			name: "invalid MetricsPort way too large",
			modify: func(c *Config) {
				c.MetricsPort = 99999
			},
			wantErr: true,
		},
		{
			name: "valid MetricsPort zero (disabled)",
			modify: func(c *Config) {
				c.MetricsPort = 0
			},
			wantErr: false,
		},
		{
			name: "valid MetricsPort in range",
			modify: func(c *Config) {
				c.MetricsPort = 8080
			},
			wantErr: false,
		},
		{
			name: "port conflict SocksPort and ControlPort",
			modify: func(c *Config) {
				c.SocksPort = 9050
				c.ControlPort = 9050
			},
			wantErr: true,
		},
		{
			name: "port conflict SocksPort and MetricsPort",
			modify: func(c *Config) {
				c.SocksPort = 9050
				c.MetricsPort = 9050
			},
			wantErr: true,
		},
		{
			name: "port conflict ControlPort and MetricsPort",
			modify: func(c *Config) {
				c.ControlPort = 9051
				c.MetricsPort = 9051
			},
			wantErr: true,
		},
		{
			name: "port conflict all three ports same",
			modify: func(c *Config) {
				c.SocksPort = 9050
				c.ControlPort = 9050
				c.MetricsPort = 9050
			},
			wantErr: true,
		},
		{
			name: "no conflict with different ports",
			modify: func(c *Config) {
				c.SocksPort = 9050
				c.ControlPort = 9051
				c.MetricsPort = 8080
			},
			wantErr: false,
		},
		{
			name: "no conflict with zero ports",
			modify: func(c *Config) {
				c.SocksPort = 0
				c.ControlPort = 0
				c.MetricsPort = 0
			},
			wantErr: false,
		},
		// Tracing configuration validation tests
		{
			name: "invalid TracingExporter",
			modify: func(c *Config) {
				c.TracingExporter = "invalid"
			},
			wantErr: true,
		},
		{
			name: "invalid TracingExporter jaeger",
			modify: func(c *Config) {
				c.TracingExporter = "jaeger"
			},
			wantErr: true,
		},
		{
			name: "valid TracingExporter otlp",
			modify: func(c *Config) {
				c.TracingExporter = "otlp"
			},
			wantErr: false,
		},
		{
			name: "valid TracingExporter stdout",
			modify: func(c *Config) {
				c.TracingExporter = "stdout"
			},
			wantErr: false,
		},
		{
			name: "valid TracingExporter noop",
			modify: func(c *Config) {
				c.TracingExporter = "noop"
			},
			wantErr: false,
		},
		{
			name: "invalid TracingSampleRate negative",
			modify: func(c *Config) {
				c.TracingSampleRate = -0.1
			},
			wantErr: true,
		},
		{
			name: "invalid TracingSampleRate too large",
			modify: func(c *Config) {
				c.TracingSampleRate = 1.5
			},
			wantErr: true,
		},
		{
			name: "valid TracingSampleRate zero",
			modify: func(c *Config) {
				c.TracingSampleRate = 0.0
			},
			wantErr: false,
		},
		{
			name: "valid TracingSampleRate one",
			modify: func(c *Config) {
				c.TracingSampleRate = 1.0
			},
			wantErr: false,
		},
		{
			name: "valid TracingSampleRate half",
			modify: func(c *Config) {
				c.TracingSampleRate = 0.5
			},
			wantErr: false,
		},
		{
			name: "invalid TracingTimeout negative",
			modify: func(c *Config) {
				c.TracingTimeout = -1 * time.Second
			},
			wantErr: true,
		},
		{
			name: "valid TracingTimeout zero",
			modify: func(c *Config) {
				c.TracingTimeout = 0
			},
			wantErr: false,
		},
		{
			name: "valid TracingTimeout positive",
			modify: func(c *Config) {
				c.TracingTimeout = 30 * time.Second
			},
			wantErr: false,
		},
		// Memory monitoring configuration validation tests
		{
			name: "valid memory monitoring enabled with valid config",
			modify: func(c *Config) {
				c.EnableMemoryMonitoring = true
				c.MemoryHighWaterMark = 100 * 1024 * 1024  // 100 MB
				c.MemoryCriticalMark = 200 * 1024 * 1024   // 200 MB
				c.MemoryMaxGoroutines = 10000
				c.MemoryCheckInterval = 30
			},
			wantErr: false,
		},
		{
			name: "memory monitoring disabled with zero values is valid",
			modify: func(c *Config) {
				c.EnableMemoryMonitoring = false
				c.MemoryHighWaterMark = 0
				c.MemoryCriticalMark = 0
				c.MemoryMaxGoroutines = 0
				c.MemoryCheckInterval = 0
			},
			wantErr: false,
		},
		{
			name: "invalid memory monitoring HighWaterMark zero when enabled",
			modify: func(c *Config) {
				c.EnableMemoryMonitoring = true
				c.MemoryHighWaterMark = 0
				c.MemoryCriticalMark = 200 * 1024 * 1024
				c.MemoryMaxGoroutines = 10000
				c.MemoryCheckInterval = 30
			},
			wantErr: true,
		},
		{
			name: "invalid memory monitoring CriticalMark zero when enabled",
			modify: func(c *Config) {
				c.EnableMemoryMonitoring = true
				c.MemoryHighWaterMark = 100 * 1024 * 1024
				c.MemoryCriticalMark = 0
				c.MemoryMaxGoroutines = 10000
				c.MemoryCheckInterval = 30
			},
			wantErr: true,
		},
		{
			name: "invalid memory monitoring CriticalMark less than HighWaterMark",
			modify: func(c *Config) {
				c.EnableMemoryMonitoring = true
				c.MemoryHighWaterMark = 200 * 1024 * 1024
				c.MemoryCriticalMark = 100 * 1024 * 1024 // Less than high water mark
				c.MemoryMaxGoroutines = 10000
				c.MemoryCheckInterval = 30
			},
			wantErr: true,
		},
		{
			name: "invalid memory monitoring CriticalMark equals HighWaterMark",
			modify: func(c *Config) {
				c.EnableMemoryMonitoring = true
				c.MemoryHighWaterMark = 100 * 1024 * 1024
				c.MemoryCriticalMark = 100 * 1024 * 1024 // Equal to high water mark
				c.MemoryMaxGoroutines = 10000
				c.MemoryCheckInterval = 30
			},
			wantErr: true,
		},
		{
			name: "invalid memory monitoring MaxGoroutines zero when enabled",
			modify: func(c *Config) {
				c.EnableMemoryMonitoring = true
				c.MemoryHighWaterMark = 100 * 1024 * 1024
				c.MemoryCriticalMark = 200 * 1024 * 1024
				c.MemoryMaxGoroutines = 0
				c.MemoryCheckInterval = 30
			},
			wantErr: true,
		},
		{
			name: "invalid memory monitoring MaxGoroutines negative when enabled",
			modify: func(c *Config) {
				c.EnableMemoryMonitoring = true
				c.MemoryHighWaterMark = 100 * 1024 * 1024
				c.MemoryCriticalMark = 200 * 1024 * 1024
				c.MemoryMaxGoroutines = -1
				c.MemoryCheckInterval = 30
			},
			wantErr: true,
		},
		{
			name: "invalid memory monitoring CheckInterval zero when enabled",
			modify: func(c *Config) {
				c.EnableMemoryMonitoring = true
				c.MemoryHighWaterMark = 100 * 1024 * 1024
				c.MemoryCriticalMark = 200 * 1024 * 1024
				c.MemoryMaxGoroutines = 10000
				c.MemoryCheckInterval = 0
			},
			wantErr: true,
		},
		{
			name: "invalid memory monitoring CheckInterval negative when enabled",
			modify: func(c *Config) {
				c.EnableMemoryMonitoring = true
				c.MemoryHighWaterMark = 100 * 1024 * 1024
				c.MemoryCriticalMark = 200 * 1024 * 1024
				c.MemoryMaxGoroutines = 10000
				c.MemoryCheckInterval = -1
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tt.modify(cfg)
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfigClone(t *testing.T) {
	original := DefaultConfig()
	original.BridgeAddresses = []string{"bridge1", "bridge2"}
	original.ExcludeNodes = []string{"node1"}
	original.OnionServices = []OnionServiceConfig{
		{VirtualPort: 80, TargetAddr: "localhost:8080", ServiceDir: "/tmp/os1"},
	}

	clone := original.Clone()

	// Verify values match
	if clone.SocksPort != original.SocksPort {
		t.Errorf("SocksPort = %v, want %v", clone.SocksPort, original.SocksPort)
	}

	// Modify clone's slices - should not affect original
	clone.BridgeAddresses[0] = "modified"
	if original.BridgeAddresses[0] == "modified" {
		t.Error("Modifying clone's BridgeAddresses affected original")
	}

	clone.ExcludeNodes = append(clone.ExcludeNodes, "node2")
	if len(original.ExcludeNodes) != 1 {
		t.Error("Modifying clone's ExcludeNodes affected original")
	}

	clone.OnionServices[0].VirtualPort = 443
	if original.OnionServices[0].VirtualPort == 443 {
		t.Error("Modifying clone's OnionServices affected original")
	}
}

func TestOnionServiceConfig(t *testing.T) {
	cfg := OnionServiceConfig{
		ServiceDir:  "/tmp/service",
		VirtualPort: 80,
		TargetAddr:  "127.0.0.1:8080",
		MaxStreams:  10,
		ClientAuth:  map[string]string{"client1": "key1"},
	}

	if cfg.ServiceDir != "/tmp/service" {
		t.Errorf("ServiceDir = %v, want /tmp/service", cfg.ServiceDir)
	}
	if cfg.VirtualPort != 80 {
		t.Errorf("VirtualPort = %v, want 80", cfg.VirtualPort)
	}
	if cfg.TargetAddr != "127.0.0.1:8080" {
		t.Errorf("TargetAddr = %v, want 127.0.0.1:8080", cfg.TargetAddr)
	}
	if cfg.MaxStreams != 10 {
		t.Errorf("MaxStreams = %v, want 10", cfg.MaxStreams)
	}
}
