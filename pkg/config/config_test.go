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
