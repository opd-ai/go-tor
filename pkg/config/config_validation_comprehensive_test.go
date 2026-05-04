package config

import (
	"testing"
	"time"
)

func TestValidateRateLimitingEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Config)
		wantErr bool
	}{
		{
			name: "rate limiting enabled zero SOCKSConnectionsPerSecond",
			modify: func(c *Config) {
				c.EnableRateLimiting = true
				c.SOCKSConnectionsPerSecond = 0
			},
			wantErr: true,
		},
		{
			name: "rate limiting enabled zero SOCKSConnectionsBurst",
			modify: func(c *Config) {
				c.EnableRateLimiting = true
				c.SOCKSConnectionsBurst = 0
			},
			wantErr: true,
		},
		{
			name: "rate limiting enabled zero CircuitCreationsPerSecond",
			modify: func(c *Config) {
				c.EnableRateLimiting = true
				c.CircuitCreationsPerSecond = 0
			},
			wantErr: true,
		},
		{
			name: "rate limiting enabled zero CircuitCreationsBurst",
			modify: func(c *Config) {
				c.EnableRateLimiting = true
				c.CircuitCreationsBurst = 0
			},
			wantErr: true,
		},
		{
			name: "rate limiting disabled zero values pass",
			modify: func(c *Config) {
				c.EnableRateLimiting = false
				c.SOCKSConnectionsPerSecond = 0
				c.SOCKSConnectionsBurst = 0
				c.CircuitCreationsPerSecond = 0
				c.CircuitCreationsBurst = 0
			},
			wantErr: false,
		},
		{
			name: "per-client rate limiting enabled zero PerClientConnectionsPerSecond",
			modify: func(c *Config) {
				c.EnablePerClientRateLimiting = true
				c.PerClientConnectionsPerSecond = 0
			},
			wantErr: true,
		},
		{
			name: "per-client rate limiting enabled zero PerClientConnectionsBurst",
			modify: func(c *Config) {
				c.EnablePerClientRateLimiting = true
				c.PerClientConnectionsBurst = 0
			},
			wantErr: true,
		},
		{
			name: "negative MaxConcurrentConnections",
			modify: func(c *Config) {
				c.MaxConcurrentConnections = -1
			},
			wantErr: true,
		},
		{
			name: "negative RateLimitCleanupInterval",
			modify: func(c *Config) {
				c.RateLimitCleanupInterval = -1
			},
			wantErr: true,
		},
		{
			name: "per-client rate limiting disabled zero values pass",
			modify: func(c *Config) {
				c.EnablePerClientRateLimiting = false
				c.PerClientConnectionsPerSecond = 0
				c.PerClientConnectionsBurst = 0
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

func TestValidatePaddingConfiguration(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Config)
		wantErr bool
	}{
		{
			name: "invalid PaddingStrategy",
			modify: func(c *Config) {
				c.PaddingStrategy = "invalid"
			},
			wantErr: true,
		},
		{
			name: "valid PaddingStrategy none",
			modify: func(c *Config) {
				c.PaddingStrategy = "none"
			},
			wantErr: false,
		},
		{
			name: "valid PaddingStrategy fixed",
			modify: func(c *Config) {
				c.PaddingStrategy = "fixed"
			},
			wantErr: false,
		},
		{
			name: "valid PaddingStrategy random",
			modify: func(c *Config) {
				c.PaddingStrategy = "random"
			},
			wantErr: false,
		},
		{
			name: "valid PaddingStrategy adaptive",
			modify: func(c *Config) {
				c.PaddingStrategy = "adaptive"
			},
			wantErr: false,
		},
		{
			name: "negative PaddingMinInterval",
			modify: func(c *Config) {
				c.PaddingMinInterval = -1 * time.Second
			},
			wantErr: true,
		},
		{
			name: "negative PaddingMaxInterval",
			modify: func(c *Config) {
				c.PaddingMaxInterval = -1 * time.Second
			},
			wantErr: true,
		},
		{
			name: "PaddingMaxInterval less than PaddingMinInterval",
			modify: func(c *Config) {
				c.PaddingMinInterval = 10 * time.Second
				c.PaddingMaxInterval = 5 * time.Second
			},
			wantErr: true,
		},
		{
			name: "PaddingMinInterval positive with PaddingMaxInterval zero",
			modify: func(c *Config) {
				c.PaddingMinInterval = 5 * time.Second
				c.PaddingMaxInterval = 0
			},
			wantErr: true,
		},
		{
			name: "both padding intervals zero pass",
			modify: func(c *Config) {
				c.PaddingMinInterval = 0
				c.PaddingMaxInterval = 0
			},
			wantErr: false,
		},
		{
			name: "negative PaddingIdleTimeout",
			modify: func(c *Config) {
				c.PaddingIdleTimeout = -1 * time.Second
			},
			wantErr: true,
		},
		{
			name: "negative PaddingBurstSize",
			modify: func(c *Config) {
				c.PaddingBurstSize = -1
			},
			wantErr: true,
		},
		{
			name: "PaddingMaxInterval equals PaddingMinInterval passes",
			modify: func(c *Config) {
				c.PaddingMinInterval = 5 * time.Second
				c.PaddingMaxInterval = 5 * time.Second
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

func TestValidateStreamBufferWatermarks(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Config)
		wantErr bool
	}{
		{
			name: "negative StreamBufferHighWaterMark",
			modify: func(c *Config) {
				c.StreamBufferHighWaterMark = -1
			},
			wantErr: true,
		},
		{
			name: "negative StreamBufferLowWaterMark",
			modify: func(c *Config) {
				c.StreamBufferLowWaterMark = -1
			},
			wantErr: true,
		},
		{
			name: "LowWaterMark greater than HighWaterMark",
			modify: func(c *Config) {
				c.StreamBufferHighWaterMark = 100
				c.StreamBufferLowWaterMark = 200
			},
			wantErr: true,
		},
		{
			name: "both watermarks zero passes",
			modify: func(c *Config) {
				c.StreamBufferHighWaterMark = 0
				c.StreamBufferLowWaterMark = 0
			},
			wantErr: false,
		},
		{
			name: "LowWaterMark equals HighWaterMark passes",
			modify: func(c *Config) {
				c.StreamBufferHighWaterMark = 1024
				c.StreamBufferLowWaterMark = 1024
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

func TestValidateConnectionPoolSettings(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Config)
		wantErr bool
	}{
		{
			name: "negative ConnectionPoolMaxIdle",
			modify: func(c *Config) {
				c.ConnectionPoolMaxIdle = -1
			},
			wantErr: true,
		},
		{
			name: "negative ConnectionPoolMaxLife",
			modify: func(c *Config) {
				c.ConnectionPoolMaxLife = -1 * time.Second
			},
			wantErr: true,
		},
		{
			name: "negative CircuitPoolMinSize",
			modify: func(c *Config) {
				c.CircuitPoolMinSize = -1
			},
			wantErr: true,
		},
		{
			name: "CircuitPoolMaxSize less than CircuitPoolMinSize",
			modify: func(c *Config) {
				c.CircuitPoolMinSize = 10
				c.CircuitPoolMaxSize = 5
			},
			wantErr: true,
		},
		{
			name: "CircuitPoolMaxSize equals CircuitPoolMinSize passes",
			modify: func(c *Config) {
				c.CircuitPoolMinSize = 5
				c.CircuitPoolMaxSize = 5
			},
			wantErr: false,
		},
		{
			name: "zero ConnectionPoolMaxIdle passes",
			modify: func(c *Config) {
				c.ConnectionPoolMaxIdle = 0
			},
			wantErr: false,
		},
		{
			name: "zero ConnectionPoolMaxLife passes",
			modify: func(c *Config) {
				c.ConnectionPoolMaxLife = 0
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

func TestValidateIsolationLevel(t *testing.T) {
	tests := []struct {
		name    string
		level   string
		wantErr bool
	}{
		{"valid none", "none", false},
		{"valid destination", "destination", false},
		{"valid credential", "credential", false},
		{"valid port", "port", false},
		{"valid session", "session", false},
		{"invalid level", "invalid", true},
		{"empty string invalid", "", true},
		{"uppercase invalid", "None", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.IsolationLevel = tt.level
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateGuardPersistence(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Config)
		wantErr bool
	}{
		{
			name: "negative GuardStateBackupCount",
			modify: func(c *Config) {
				c.GuardStateBackupCount = -1
			},
			wantErr: true,
		},
		{
			name: "negative GuardStateSnapshotInterval",
			modify: func(c *Config) {
				c.GuardStateSnapshotInterval = -1
			},
			wantErr: true,
		},
		{
			name: "negative GuardStateLockTimeout",
			modify: func(c *Config) {
				c.GuardStateLockTimeout = -1
			},
			wantErr: true,
		},
		{
			name: "zero GuardStateBackupCount passes",
			modify: func(c *Config) {
				c.GuardStateBackupCount = 0
			},
			wantErr: false,
		},
		{
			name: "zero GuardStateSnapshotInterval passes",
			modify: func(c *Config) {
				c.GuardStateSnapshotInterval = 0
			},
			wantErr: false,
		},
		{
			name: "zero GuardStateLockTimeout passes",
			modify: func(c *Config) {
				c.GuardStateLockTimeout = 0
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

func TestValidateCrashRecovery(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Config)
		wantErr bool
	}{
		{
			name: "crash recovery enabled zero interval",
			modify: func(c *Config) {
				c.EnableCrashRecovery = true
				c.CrashRecoveryInterval = 0
			},
			wantErr: true,
		},
		{
			name: "crash recovery enabled negative interval",
			modify: func(c *Config) {
				c.EnableCrashRecovery = true
				c.CrashRecoveryInterval = -1
			},
			wantErr: true,
		},
		{
			name: "crash recovery enabled negative backup count",
			modify: func(c *Config) {
				c.EnableCrashRecovery = true
				c.CrashRecoveryInterval = 60
				c.CrashRecoveryBackupCount = -1
			},
			wantErr: true,
		},
		{
			name: "crash recovery disabled zero values pass",
			modify: func(c *Config) {
				c.EnableCrashRecovery = false
				c.CrashRecoveryInterval = 0
				c.CrashRecoveryBackupCount = 0
			},
			wantErr: false,
		},
		{
			name: "crash recovery disabled negative values pass",
			modify: func(c *Config) {
				c.EnableCrashRecovery = false
				c.CrashRecoveryInterval = -5
				c.CrashRecoveryBackupCount = -3
			},
			wantErr: false,
		},
		{
			name: "crash recovery enabled zero backup count passes",
			modify: func(c *Config) {
				c.EnableCrashRecovery = true
				c.CrashRecoveryInterval = 60
				c.CrashRecoveryBackupCount = 0
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

func TestCloneComprehensive(t *testing.T) {
	t.Run("preserves all core fields", func(t *testing.T) {
		orig := DefaultConfig()
		orig.SocksPort = 1234
		orig.ControlPort = 5678
		orig.ControlPassword = "secret"
		orig.DataDirectory = "/data/tor"
		orig.CircuitBuildTimeout = 90 * time.Second
		orig.MaxCircuitDirtiness = 15 * time.Minute
		orig.NumEntryGuards = 5
		orig.ConnLimit = 500
		orig.LogLevel = "debug"
		orig.IsolationLevel = "session"
		orig.PaddingStrategy = "adaptive"
		orig.EnableRateLimiting = false
		orig.GuardStateBackupCount = 7
		orig.EnableCrashRecovery = false

		clone := orig.Clone()

		if clone.SocksPort != 1234 {
			t.Errorf("SocksPort = %d, want 1234", clone.SocksPort)
		}
		if clone.ControlPort != 5678 {
			t.Errorf("ControlPort = %d, want 5678", clone.ControlPort)
		}
		if clone.ControlPassword != "secret" {
			t.Errorf("ControlPassword = %q, want %q", clone.ControlPassword, "secret")
		}
		if clone.DataDirectory != "/data/tor" {
			t.Errorf("DataDirectory = %q, want %q", clone.DataDirectory, "/data/tor")
		}
		if clone.CircuitBuildTimeout != 90*time.Second {
			t.Errorf("CircuitBuildTimeout = %v, want 90s", clone.CircuitBuildTimeout)
		}
		if clone.MaxCircuitDirtiness != 15*time.Minute {
			t.Errorf("MaxCircuitDirtiness = %v, want 15m", clone.MaxCircuitDirtiness)
		}
		if clone.NumEntryGuards != 5 {
			t.Errorf("NumEntryGuards = %d, want 5", clone.NumEntryGuards)
		}
		if clone.ConnLimit != 500 {
			t.Errorf("ConnLimit = %d, want 500", clone.ConnLimit)
		}
		if clone.LogLevel != "debug" {
			t.Errorf("LogLevel = %q, want %q", clone.LogLevel, "debug")
		}
		if clone.IsolationLevel != "session" {
			t.Errorf("IsolationLevel = %q, want %q", clone.IsolationLevel, "session")
		}
		if clone.PaddingStrategy != "adaptive" {
			t.Errorf("PaddingStrategy = %q, want %q", clone.PaddingStrategy, "adaptive")
		}
		if clone.EnableRateLimiting != false {
			t.Error("EnableRateLimiting should be false")
		}
		if clone.GuardStateBackupCount != 7 {
			t.Errorf("GuardStateBackupCount = %d, want 7", clone.GuardStateBackupCount)
		}
		if clone.EnableCrashRecovery != false {
			t.Error("EnableCrashRecovery should be false")
		}
	})

	t.Run("deep copy BridgeAddresses", func(t *testing.T) {
		orig := DefaultConfig()
		orig.BridgeAddresses = []string{"bridge1", "bridge2", "bridge3"}

		clone := orig.Clone()
		clone.BridgeAddresses[1] = "modified"

		if orig.BridgeAddresses[1] == "modified" {
			t.Error("modifying clone BridgeAddresses affected original")
		}
	})

	t.Run("deep copy Bridges with parameters", func(t *testing.T) {
		orig := DefaultConfig()
		orig.Bridges = []*BridgeInfo{
			{
				Transport:   "obfs4",
				Address:     "192.0.2.1",
				Port:        443,
				Fingerprint: "AAAA",
				Parameters:  map[string]string{"cert": "abc", "iat-mode": "0"},
			},
		}

		clone := orig.Clone()
		clone.Bridges[0].Parameters["cert"] = "xyz"
		clone.Bridges[0].Address = "10.0.0.1"

		if orig.Bridges[0].Parameters["cert"] != "abc" {
			t.Error("modifying clone Bridge parameters affected original")
		}
		if orig.Bridges[0].Address != "192.0.2.1" {
			t.Error("modifying clone Bridge address affected original")
		}
	})

	t.Run("deep copy Bridges with nil entry", func(t *testing.T) {
		orig := DefaultConfig()
		orig.Bridges = []*BridgeInfo{
			{Transport: "obfs4", Address: "192.0.2.1", Port: 443, Parameters: map[string]string{}},
			nil,
			{Transport: "meek", Address: "192.0.2.2", Port: 80, Parameters: map[string]string{}},
		}

		clone := orig.Clone()

		if len(clone.Bridges) != 3 {
			t.Fatalf("clone Bridges length = %d, want 3", len(clone.Bridges))
		}
		if clone.Bridges[1] != nil {
			t.Error("nil bridge entry should remain nil in clone")
		}
		if clone.Bridges[0].Transport != "obfs4" {
			t.Errorf("Bridges[0].Transport = %q, want %q", clone.Bridges[0].Transport, "obfs4")
		}
		if clone.Bridges[2].Transport != "meek" {
			t.Errorf("Bridges[2].Transport = %q, want %q", clone.Bridges[2].Transport, "meek")
		}
	})

	t.Run("deep copy OnionServices", func(t *testing.T) {
		orig := DefaultConfig()
		orig.OnionServices = []OnionServiceConfig{
			{VirtualPort: 80, TargetAddr: "localhost:8080", ServiceDir: "/srv/os1"},
			{VirtualPort: 443, TargetAddr: "localhost:8443", ServiceDir: "/srv/os2"},
		}

		clone := orig.Clone()
		clone.OnionServices[0].VirtualPort = 9999

		if orig.OnionServices[0].VirtualPort == 9999 {
			t.Error("modifying clone OnionServices affected original")
		}
	})

	t.Run("deep copy ExcludeNodes and ExcludeExitNodes", func(t *testing.T) {
		orig := DefaultConfig()
		orig.ExcludeNodes = []string{"node1", "node2"}
		orig.ExcludeExitNodes = []string{"exit1"}

		clone := orig.Clone()
		clone.ExcludeNodes[0] = "modified"
		clone.ExcludeExitNodes = append(clone.ExcludeExitNodes, "exit2")

		if orig.ExcludeNodes[0] == "modified" {
			t.Error("modifying clone ExcludeNodes affected original")
		}
		if len(orig.ExcludeExitNodes) != 1 {
			t.Error("appending to clone ExcludeExitNodes affected original")
		}
	})
}

func TestGetCheckpointPathVariations(t *testing.T) {
	tests := []struct {
		name     string
		dataDir  string
		explicit string
		want     string
	}{
		{
			name:     "default path from DataDirectory",
			dataDir:  "/home/user/.tor",
			explicit: "",
			want:     "/home/user/.tor/checkpoint.json",
		},
		{
			name:     "explicit path overrides DataDirectory",
			dataDir:  "/home/user/.tor",
			explicit: "/custom/path/state.json",
			want:     "/custom/path/state.json",
		},
		{
			name:     "relative DataDirectory",
			dataDir:  "./data",
			explicit: "",
			want:     "data/checkpoint.json",
		},
		{
			name:     "empty DataDirectory",
			dataDir:  "",
			explicit: "",
			want:     "checkpoint.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.DataDirectory = tt.dataDir
			cfg.CrashRecoveryCheckpointPath = tt.explicit
			got := cfg.GetCheckpointPath()
			if got != tt.want {
				t.Errorf("GetCheckpointPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDefaultConfigComprehensiveVerification(t *testing.T) {
	cfg := DefaultConfig()

	// Circuit settings
	assertDuration(t, "CircuitBuildTimeout", cfg.CircuitBuildTimeout, 60*time.Second)
	assertDuration(t, "MaxCircuitDirtiness", cfg.MaxCircuitDirtiness, 10*time.Minute)
	assertDuration(t, "NewCircuitPeriod", cfg.NewCircuitPeriod, 30*time.Second)
	assertInt(t, "NumEntryGuards", cfg.NumEntryGuards, 3)

	// Path selection
	assertBool(t, "UseEntryGuards", cfg.UseEntryGuards, true)
	assertBool(t, "UseBridges", cfg.UseBridges, false)
	assertInt(t, "len(BridgeAddresses)", len(cfg.BridgeAddresses), 0)
	assertInt(t, "len(ExcludeNodes)", len(cfg.ExcludeNodes), 0)
	assertInt(t, "len(ExcludeExitNodes)", len(cfg.ExcludeExitNodes), 0)

	// Network behavior
	assertInt(t, "ConnLimit", cfg.ConnLimit, 1000)
	assertDuration(t, "DormantTimeout", cfg.DormantTimeout, 24*time.Hour)

	// Logging
	assertString(t, "LogLevel", cfg.LogLevel, "info")

	// Monitoring
	assertInt(t, "MetricsPort", cfg.MetricsPort, 0)
	assertBool(t, "EnableMetrics", cfg.EnableMetrics, false)

	// Performance tuning
	assertBool(t, "EnableConnectionPooling", cfg.EnableConnectionPooling, true)
	assertInt(t, "ConnectionPoolMaxIdle", cfg.ConnectionPoolMaxIdle, 5)
	assertDuration(t, "ConnectionPoolMaxLife", cfg.ConnectionPoolMaxLife, 10*time.Minute)
	assertBool(t, "EnableCircuitPrebuilding", cfg.EnableCircuitPrebuilding, true)
	assertInt(t, "CircuitPoolMinSize", cfg.CircuitPoolMinSize, 2)
	assertInt(t, "CircuitPoolMaxSize", cfg.CircuitPoolMaxSize, 10)
	assertBool(t, "EnableBufferPooling", cfg.EnableBufferPooling, true)

	// Isolation
	assertString(t, "IsolationLevel", cfg.IsolationLevel, "none")
	assertBool(t, "IsolateDestinations", cfg.IsolateDestinations, false)
	assertBool(t, "IsolateSOCKSAuth", cfg.IsolateSOCKSAuth, false)
	assertBool(t, "IsolateClientPort", cfg.IsolateClientPort, false)
	assertBool(t, "IsolateClientProtocol", cfg.IsolateClientProtocol, false)

	// Padding
	assertBool(t, "EnableCircuitPadding", cfg.EnableCircuitPadding, true)
	assertString(t, "PaddingStrategy", cfg.PaddingStrategy, "random")
	assertDuration(t, "PaddingMinInterval", cfg.PaddingMinInterval, 3*time.Second)
	assertDuration(t, "PaddingMaxInterval", cfg.PaddingMaxInterval, 10*time.Second)
	assertDuration(t, "PaddingIdleTimeout", cfg.PaddingIdleTimeout, time.Second)
	assertBool(t, "PaddingDummyTraffic", cfg.PaddingDummyTraffic, false)
	assertInt(t, "PaddingBurstSize", cfg.PaddingBurstSize, 1)

	// Rate limiting
	assertBool(t, "EnableRateLimiting", cfg.EnableRateLimiting, true)
	assertFloat(t, "SOCKSConnectionsPerSecond", cfg.SOCKSConnectionsPerSecond, 100.0)
	assertInt(t, "SOCKSConnectionsBurst", cfg.SOCKSConnectionsBurst, 50)
	assertFloat(t, "CircuitCreationsPerSecond", cfg.CircuitCreationsPerSecond, 10.0)
	assertInt(t, "CircuitCreationsBurst", cfg.CircuitCreationsBurst, 5)
	assertInt(t, "MaxConcurrentConnections", cfg.MaxConcurrentConnections, 1000)
	assertInt(t, "StreamBufferHighWaterMark", cfg.StreamBufferHighWaterMark, 65536)
	assertInt(t, "StreamBufferLowWaterMark", cfg.StreamBufferLowWaterMark, 16384)
	assertBool(t, "EnablePerClientRateLimiting", cfg.EnablePerClientRateLimiting, false)
	assertFloat(t, "PerClientConnectionsPerSecond", cfg.PerClientConnectionsPerSecond, 10.0)
	assertInt(t, "PerClientConnectionsBurst", cfg.PerClientConnectionsBurst, 5)
	assertInt(t, "RateLimitCleanupInterval", cfg.RateLimitCleanupInterval, 300)

	// Guard persistence
	assertInt(t, "GuardStateBackupCount", cfg.GuardStateBackupCount, 3)
	assertInt(t, "GuardStateSnapshotInterval", cfg.GuardStateSnapshotInterval, 300)
	assertInt(t, "GuardStateLockTimeout", cfg.GuardStateLockTimeout, 10)

	// Tracing
	assertBool(t, "EnableTracing", cfg.EnableTracing, false)
	assertString(t, "TracingEndpoint", cfg.TracingEndpoint, "localhost:4317")
	assertFloat(t, "TracingSampleRate", cfg.TracingSampleRate, 1.0)
	assertString(t, "TracingExporter", cfg.TracingExporter, "noop")
	assertBool(t, "TracingInsecure", cfg.TracingInsecure, false)
	assertDuration(t, "TracingTimeout", cfg.TracingTimeout, 10*time.Second)

	// Memory monitoring
	assertBool(t, "EnableMemoryMonitoring", cfg.EnableMemoryMonitoring, false)
	assertUint64(t, "MemoryHighWaterMark", cfg.MemoryHighWaterMark, 100*1024*1024)
	assertUint64(t, "MemoryCriticalMark", cfg.MemoryCriticalMark, 200*1024*1024)
	assertInt(t, "MemoryMaxGoroutines", cfg.MemoryMaxGoroutines, 10000)
	assertInt(t, "MemoryCheckInterval", cfg.MemoryCheckInterval, 30)
	assertBool(t, "MemoryTriggerGCOnCritical", cfg.MemoryTriggerGCOnCritical, true)

	// Crash recovery
	assertBool(t, "EnableCrashRecovery", cfg.EnableCrashRecovery, true)
	assertString(t, "CrashRecoveryCheckpointPath", cfg.CrashRecoveryCheckpointPath, "")
	assertInt(t, "CrashRecoveryInterval", cfg.CrashRecoveryInterval, 60)
	assertInt(t, "CrashRecoveryBackupCount", cfg.CrashRecoveryBackupCount, 2)

	// Profiling
	assertBool(t, "EnableProfiling", cfg.EnableProfiling, false)
	assertInt(t, "ProfilingPort", cfg.ProfilingPort, 0)
	assertString(t, "ProfilingPath", cfg.ProfilingPath, "/debug/pprof")
	assertBool(t, "EnableCPUProfiling", cfg.EnableCPUProfiling, true)
	assertBool(t, "EnableHeapProfiling", cfg.EnableHeapProfiling, true)
	assertBool(t, "EnableMutexProfile", cfg.EnableMutexProfile, false)
	assertBool(t, "EnableBlockProfile", cfg.EnableBlockProfile, false)
	assertInt(t, "MutexProfileRate", cfg.MutexProfileRate, 0)
	assertInt(t, "BlockProfileRate", cfg.BlockProfileRate, 0)
}

// Helper functions for assertions.

func assertInt(t *testing.T, field string, got, want int) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %d, want %d", field, got, want)
	}
}

func assertUint64(t *testing.T, field string, got, want uint64) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %d, want %d", field, got, want)
	}
}

func assertFloat(t *testing.T, field string, got, want float64) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %f, want %f", field, got, want)
	}
}

func assertString(t *testing.T, field, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %q, want %q", field, got, want)
	}
}

func assertBool(t *testing.T, field string, got, want bool) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %v, want %v", field, got, want)
	}
}

func assertDuration(t *testing.T, field string, got, want time.Duration) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %v, want %v", field, got, want)
	}
}
