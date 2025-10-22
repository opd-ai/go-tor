package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadFromFile(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()

	tests := []struct {
		name      string
		content   string
		wantErr   bool
		checkFunc func(*testing.T, *Config)
	}{
		{
			name: "basic configuration",
			content: `# Test configuration
SocksPort 9150
ControlPort 9151
DataDirectory /tmp/tor-test
LogLevel debug`,
			wantErr: false,
			checkFunc: func(t *testing.T, cfg *Config) {
				if cfg.SocksPort != 9150 {
					t.Errorf("SocksPort = %d, want 9150", cfg.SocksPort)
				}
				if cfg.ControlPort != 9151 {
					t.Errorf("ControlPort = %d, want 9151", cfg.ControlPort)
				}
				if cfg.DataDirectory != "/tmp/tor-test" {
					t.Errorf("DataDirectory = %s, want /tmp/tor-test", cfg.DataDirectory)
				}
				if cfg.LogLevel != "debug" {
					t.Errorf("LogLevel = %s, want debug", cfg.LogLevel)
				}
			},
		},
		{
			name: "circuit settings",
			content: `CircuitBuildTimeout 90s
MaxCircuitDirtiness 15m
NewCircuitPeriod 45s
NumEntryGuards 5`,
			wantErr: false,
			checkFunc: func(t *testing.T, cfg *Config) {
				if cfg.CircuitBuildTimeout != 90*time.Second {
					t.Errorf("CircuitBuildTimeout = %v, want 90s", cfg.CircuitBuildTimeout)
				}
				if cfg.MaxCircuitDirtiness != 15*time.Minute {
					t.Errorf("MaxCircuitDirtiness = %v, want 15m", cfg.MaxCircuitDirtiness)
				}
				if cfg.NewCircuitPeriod != 45*time.Second {
					t.Errorf("NewCircuitPeriod = %v, want 45s", cfg.NewCircuitPeriod)
				}
				if cfg.NumEntryGuards != 5 {
					t.Errorf("NumEntryGuards = %d, want 5", cfg.NumEntryGuards)
				}
			},
		},
		{
			name: "boolean settings",
			content: `UseEntryGuards 0
UseBridges yes`,
			wantErr: false,
			checkFunc: func(t *testing.T, cfg *Config) {
				if cfg.UseEntryGuards != false {
					t.Errorf("UseEntryGuards = %v, want false", cfg.UseEntryGuards)
				}
				if cfg.UseBridges != true {
					t.Errorf("UseBridges = %v, want true", cfg.UseBridges)
				}
			},
		},
		{
			name: "list settings",
			content: `Bridge 192.168.1.1:9001
Bridge 192.168.1.2:9001
ExcludeNodes node1
ExcludeNodes node2
ExcludeExitNodes exit1`,
			wantErr: false,
			checkFunc: func(t *testing.T, cfg *Config) {
				if len(cfg.BridgeAddresses) != 2 {
					t.Errorf("len(BridgeAddresses) = %d, want 2", len(cfg.BridgeAddresses))
				}
				if len(cfg.ExcludeNodes) != 2 {
					t.Errorf("len(ExcludeNodes) = %d, want 2", len(cfg.ExcludeNodes))
				}
				if len(cfg.ExcludeExitNodes) != 1 {
					t.Errorf("len(ExcludeExitNodes) = %d, want 1", len(cfg.ExcludeExitNodes))
				}
			},
		},
		{
			name: "comments and empty lines",
			content: `# This is a comment
SocksPort 9050

# Another comment
ControlPort 9051
`,
			wantErr: false,
			checkFunc: func(t *testing.T, cfg *Config) {
				if cfg.SocksPort != 9050 {
					t.Errorf("SocksPort = %d, want 9050", cfg.SocksPort)
				}
				if cfg.ControlPort != 9051 {
					t.Errorf("ControlPort = %d, want 9051", cfg.ControlPort)
				}
			},
		},
		{
			name: "duration formats",
			content: `CircuitBuildTimeout 60s
MaxCircuitDirtiness 10m
NewCircuitPeriod 2h
DormantTimeout 1d`,
			wantErr: false,
			checkFunc: func(t *testing.T, cfg *Config) {
				if cfg.CircuitBuildTimeout != 60*time.Second {
					t.Errorf("CircuitBuildTimeout = %v, want 60s", cfg.CircuitBuildTimeout)
				}
				if cfg.MaxCircuitDirtiness != 10*time.Minute {
					t.Errorf("MaxCircuitDirtiness = %v, want 10m", cfg.MaxCircuitDirtiness)
				}
				if cfg.NewCircuitPeriod != 2*time.Hour {
					t.Errorf("NewCircuitPeriod = %v, want 2h", cfg.NewCircuitPeriod)
				}
				if cfg.DormantTimeout != 24*time.Hour {
					t.Errorf("DormantTimeout = %v, want 24h", cfg.DormantTimeout)
				}
			},
		},
		{
			name:      "invalid port",
			content:   `SocksPort invalid`,
			wantErr:   true,
			checkFunc: nil,
		},
		{
			name:      "invalid duration",
			content:   `CircuitBuildTimeout invalid`,
			wantErr:   true,
			checkFunc: nil,
		},
		{
			name:      "invalid validation - port too high",
			content:   `SocksPort 70000`,
			wantErr:   true,
			checkFunc: nil,
		},
		{
			name: "unknown options ignored",
			content: `SocksPort 9050
UnknownOption value
ControlPort 9051`,
			wantErr: false,
			checkFunc: func(t *testing.T, cfg *Config) {
				if cfg.SocksPort != 9050 {
					t.Errorf("SocksPort = %d, want 9050", cfg.SocksPort)
				}
				if cfg.ControlPort != 9051 {
					t.Errorf("ControlPort = %d, want 9051", cfg.ControlPort)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test file
			testFile := filepath.Join(tmpDir, tt.name+".conf")
			if err := os.WriteFile(testFile, []byte(tt.content), 0o644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			// Load configuration
			cfg := DefaultConfig()
			err := LoadFromFile(testFile, cfg)

			if (err != nil) != tt.wantErr {
				t.Errorf("LoadFromFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.checkFunc != nil {
				tt.checkFunc(t, cfg)
			}
		})
	}
}

func TestLoadFromFile_FileNotFound(t *testing.T) {
	cfg := DefaultConfig()
	err := LoadFromFile("/nonexistent/file.conf", cfg)
	if err == nil {
		t.Error("LoadFromFile() should return error for nonexistent file")
	}
}

func TestLoadFromFile_NilConfig(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.conf")
	if err := os.WriteFile(testFile, []byte("SocksPort 9050"), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	err := LoadFromFile(testFile, nil)
	if err == nil {
		t.Error("LoadFromFile() should return error for nil config")
	}
}

func TestSaveToFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "saved.conf")

	// Create a config with custom values
	cfg := DefaultConfig()
	cfg.SocksPort = 9150
	cfg.ControlPort = 9151
	cfg.DataDirectory = "/custom/path"
	cfg.LogLevel = "debug"
	cfg.NumEntryGuards = 5
	cfg.UseEntryGuards = false
	cfg.UseBridges = true
	cfg.BridgeAddresses = []string{"bridge1", "bridge2"}
	cfg.ExcludeNodes = []string{"node1"}
	cfg.CircuitBuildTimeout = 90 * time.Second

	// Save configuration
	if err := SaveToFile(testFile, cfg); err != nil {
		t.Fatalf("SaveToFile() error = %v", err)
	}

	// Load it back
	loadedCfg := DefaultConfig()
	if err := LoadFromFile(testFile, loadedCfg); err != nil {
		t.Fatalf("LoadFromFile() error = %v", err)
	}

	// Verify values match
	if loadedCfg.SocksPort != cfg.SocksPort {
		t.Errorf("SocksPort = %d, want %d", loadedCfg.SocksPort, cfg.SocksPort)
	}
	if loadedCfg.ControlPort != cfg.ControlPort {
		t.Errorf("ControlPort = %d, want %d", loadedCfg.ControlPort, cfg.ControlPort)
	}
	if loadedCfg.DataDirectory != cfg.DataDirectory {
		t.Errorf("DataDirectory = %s, want %s", loadedCfg.DataDirectory, cfg.DataDirectory)
	}
	if loadedCfg.LogLevel != cfg.LogLevel {
		t.Errorf("LogLevel = %s, want %s", loadedCfg.LogLevel, cfg.LogLevel)
	}
	if loadedCfg.NumEntryGuards != cfg.NumEntryGuards {
		t.Errorf("NumEntryGuards = %d, want %d", loadedCfg.NumEntryGuards, cfg.NumEntryGuards)
	}
	if loadedCfg.UseEntryGuards != cfg.UseEntryGuards {
		t.Errorf("UseEntryGuards = %v, want %v", loadedCfg.UseEntryGuards, cfg.UseEntryGuards)
	}
	if loadedCfg.UseBridges != cfg.UseBridges {
		t.Errorf("UseBridges = %v, want %v", loadedCfg.UseBridges, cfg.UseBridges)
	}
	if len(loadedCfg.BridgeAddresses) != len(cfg.BridgeAddresses) {
		t.Errorf("len(BridgeAddresses) = %d, want %d", len(loadedCfg.BridgeAddresses), len(cfg.BridgeAddresses))
	}
	if loadedCfg.CircuitBuildTimeout != cfg.CircuitBuildTimeout {
		t.Errorf("CircuitBuildTimeout = %v, want %v", loadedCfg.CircuitBuildTimeout, cfg.CircuitBuildTimeout)
	}
}

func TestSaveToFile_NilConfig(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.conf")

	err := SaveToFile(testFile, nil)
	if err == nil {
		t.Error("SaveToFile() should return error for nil config")
	}
}

func TestPathValidation(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "valid absolute path",
			path:    "/tmp/config.conf",
			wantErr: false,
		},
		{
			name:    "valid relative path",
			path:    "config.conf",
			wantErr: false,
		},
		{
			name:    "valid nested relative path",
			path:    "configs/tor/config.conf",
			wantErr: false,
		},
		{
			name:    "directory traversal attack with ..",
			path:    "../../../etc/passwd",
			wantErr: true,
		},
		{
			name:    "directory traversal in middle",
			path:    "configs/../../../etc/passwd",
			wantErr: true,
		},
		{
			name:    "double dot escape",
			path:    "configs/../../etc/passwd",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePath() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSaveToFile_PathValidation(t *testing.T) {
	cfg := DefaultConfig()

	// Try to save to a path with directory traversal
	err := SaveToFile("../../../etc/passwd", cfg)
	if err == nil {
		t.Error("SaveToFile() should reject path with directory traversal")
	}
	if !strings.Contains(err.Error(), "path validation failed") {
		t.Errorf("Expected path validation error, got: %v", err)
	}
}

func TestLoadFromFile_PathValidation(t *testing.T) {
	cfg := DefaultConfig()

	// Try to load from a path with directory traversal
	err := LoadFromFile("../../../etc/passwd", cfg)
	if err == nil {
		t.Error("LoadFromFile() should reject path with directory traversal")
	}
	if !strings.Contains(err.Error(), "path validation failed") {
		t.Errorf("Expected path validation error, got: %v", err)
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		{"seconds", "60s", 60 * time.Second, false},
		{"minutes", "5m", 5 * time.Minute, false},
		{"hours", "2h", 2 * time.Hour, false},
		{"days", "1d", 24 * time.Hour, false},
		{"uppercase seconds", "60S", 60 * time.Second, false},
		{"uppercase days", "2D", 48 * time.Hour, false},
		{"go duration", "1h30m", 90 * time.Minute, false},
		{"numeric only (seconds)", "300", 300 * time.Second, false},
		{"empty string", "", 0, true},
		{"invalid format", "abc", 0, true},
		{"invalid suffix", "10x", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDuration(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseDuration() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseDuration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseBool(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"1", "1", true},
		{"0", "0", false},
		{"true", "true", true},
		{"false", "false", false},
		{"yes", "yes", true},
		{"no", "no", false},
		{"on", "on", true},
		{"off", "off", false},
		{"uppercase TRUE", "TRUE", true},
		{"uppercase FALSE", "FALSE", false},
		{"mixed case Yes", "Yes", true},
		{"invalid", "invalid", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseBool(tt.input)
			if got != tt.want {
				t.Errorf("parseBool() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name  string
		input time.Duration
		want  string
	}{
		{"seconds", 45 * time.Second, "45s"},
		{"minutes", 5 * time.Minute, "5m"},
		{"hours", 2 * time.Hour, "2h"},
		{"days", 24 * time.Hour, "1d"},
		{"multiple days", 48 * time.Hour, "2d"},
		{"60 seconds as minutes", 60 * time.Second, "1m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.input)
			if got != tt.want {
				t.Errorf("formatDuration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatBool(t *testing.T) {
	tests := []struct {
		name  string
		input bool
		want  string
	}{
		{"true", true, "1"},
		{"false", false, "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatBool(tt.input)
			if got != tt.want {
				t.Errorf("formatBool() = %v, want %v", got, tt.want)
			}
		})
	}
}

func BenchmarkLoadFromFile(b *testing.B) {
	tmpDir := b.TempDir()
	testFile := filepath.Join(tmpDir, "bench.conf")

	content := `# Benchmark configuration
SocksPort 9050
ControlPort 9051
DataDirectory /tmp/tor
LogLevel info
CircuitBuildTimeout 60s
MaxCircuitDirtiness 10m
NumEntryGuards 3
UseEntryGuards 1
UseBridges 0
ConnLimit 1000`

	if err := os.WriteFile(testFile, []byte(content), 0o644); err != nil {
		b.Fatalf("Failed to create test file: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cfg := DefaultConfig()
		if err := LoadFromFile(testFile, cfg); err != nil {
			b.Fatalf("LoadFromFile() error = %v", err)
		}
	}
}

func BenchmarkSaveToFile(b *testing.B) {
	tmpDir := b.TempDir()
	cfg := DefaultConfig()
	cfg.BridgeAddresses = []string{"bridge1", "bridge2", "bridge3"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		testFile := filepath.Join(tmpDir, "bench"+string(rune(i))+".conf")
		if err := SaveToFile(testFile, cfg); err != nil {
			b.Fatalf("SaveToFile() error = %v", err)
		}
	}
}
