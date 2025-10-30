package config

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewReloadableConfig(t *testing.T) {
	cfg := DefaultConfig()
	rc := NewReloadableConfig(cfg, "", nil)

	if rc == nil {
		t.Fatal("NewReloadableConfig returned nil")
	}

	if rc.config != cfg {
		t.Error("Config not properly stored")
	}

	if rc.logger == nil {
		t.Error("Logger should default to slog.Default()")
	}
}

func TestReloadableConfig_Get(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LogLevel = "debug"
	rc := NewReloadableConfig(cfg, "", nil)

	retrieved := rc.Get()
	if retrieved == nil {
		t.Fatal("Get() returned nil")
	}

	if retrieved.LogLevel != "debug" {
		t.Errorf("Expected LogLevel 'debug', got '%s'", retrieved.LogLevel)
	}

	// Verify it's a copy - modifying retrieved shouldn't affect original
	retrieved.LogLevel = "error"
	if rc.config.LogLevel == "error" {
		t.Error("Get() should return a copy, not the original")
	}
}

func TestReloadableConfig_OnReload(t *testing.T) {
	cfg := DefaultConfig()
	rc := NewReloadableConfig(cfg, "", nil)

	callCount := 0
	callback := func(old, new *Config) error {
		callCount++
		return nil
	}

	rc.OnReload(callback)
	if len(rc.reloadCallbacks) != 1 {
		t.Errorf("Expected 1 callback, got %d", len(rc.reloadCallbacks))
	}
}

func TestReloadableConfig_MergeReloadableFields(t *testing.T) {
	oldConfig := DefaultConfig()
	oldConfig.LogLevel = "info"
	oldConfig.MaxCircuitDirtiness = 10 * time.Minute
	oldConfig.SocksPort = 9050 // Non-reloadable field

	newConfig := DefaultConfig()
	newConfig.LogLevel = "debug"
	newConfig.MaxCircuitDirtiness = 15 * time.Minute
	newConfig.SocksPort = 9999 // Should NOT be changed

	rc := NewReloadableConfig(oldConfig, "", nil)
	merged := rc.mergeReloadableFields(oldConfig, newConfig)

	// Check reloadable fields were updated
	if merged.LogLevel != "debug" {
		t.Errorf("Expected LogLevel 'debug', got '%s'", merged.LogLevel)
	}
	if merged.MaxCircuitDirtiness != 15*time.Minute {
		t.Errorf("Expected MaxCircuitDirtiness 15m, got %v", merged.MaxCircuitDirtiness)
	}

	// Check non-reloadable field was preserved
	if merged.SocksPort != 9050 {
		t.Errorf("Expected SocksPort 9050 (preserved), got %d", merged.SocksPort)
	}
}

func TestReloadableConfig_ApplyConfig(t *testing.T) {
	oldConfig := DefaultConfig()
	oldConfig.LogLevel = "info"

	rc := NewReloadableConfig(oldConfig, "", nil)

	// Add a callback to track execution
	callbackExecuted := false
	var oldConfigInCallback, newConfigInCallback *Config
	rc.OnReload(func(old, new *Config) error {
		callbackExecuted = true
		oldConfigInCallback = old
		newConfigInCallback = new
		return nil
	})

	newConfig := DefaultConfig()
	newConfig.LogLevel = "debug"

	err := rc.applyConfig(newConfig)
	if err != nil {
		t.Fatalf("applyConfig failed: %v", err)
	}

	if !callbackExecuted {
		t.Error("Reload callback was not executed")
	}

	if oldConfigInCallback.LogLevel != "info" {
		t.Error("Callback received wrong old config")
	}

	if newConfigInCallback.LogLevel != "debug" {
		t.Error("Callback received wrong new config")
	}

	// Verify config was actually updated
	if rc.config.LogLevel != "debug" {
		t.Errorf("Config not updated, expected 'debug', got '%s'", rc.config.LogLevel)
	}
}

func TestReloadableConfig_ApplyConfig_CallbackError(t *testing.T) {
	oldConfig := DefaultConfig()
	oldConfig.LogLevel = "info"

	rc := NewReloadableConfig(oldConfig, "", nil)

	// Add a callback that returns an error
	rc.OnReload(func(old, new *Config) error {
		return fmt.Errorf("validation failed")
	})

	newConfig := DefaultConfig()
	newConfig.LogLevel = "debug"

	err := rc.applyConfig(newConfig)
	if err == nil {
		t.Fatal("Expected error from callback, got nil")
	}

	// Verify config was NOT updated (rollback)
	if rc.config.LogLevel != "info" {
		t.Errorf("Config should not have been updated, expected 'info', got '%s'", rc.config.LogLevel)
	}
}

func TestReloadableConfig_ReloadFromFile(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "torrc")

	// Write initial config
	initialConfig := `# Test configuration
LogLevel info
CircuitBuildTimeout 60
MaxCircuitDirtiness 600
`
	if err := os.WriteFile(configPath, []byte(initialConfig), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Load config and create reloadable wrapper
	cfg := DefaultConfig()
	if err := LoadFromFile(configPath, cfg); err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	rc := NewReloadableConfig(cfg, configPath, nil)

	// Verify initial state
	if rc.Get().LogLevel != "info" {
		t.Errorf("Initial LogLevel should be 'info', got '%s'", rc.Get().LogLevel)
	}

	// Modify the config file
	time.Sleep(10 * time.Millisecond) // Ensure different mod time
	updatedConfig := `# Test configuration
LogLevel debug
CircuitBuildTimeout 90
MaxCircuitDirtiness 900
`
	if err := os.WriteFile(configPath, []byte(updatedConfig), 0644); err != nil {
		t.Fatalf("Failed to update config file: %v", err)
	}

	// Reload configuration
	if err := rc.Reload(); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	// Verify config was updated
	if rc.Get().LogLevel != "debug" {
		t.Errorf("Expected LogLevel 'debug' after reload, got '%s'", rc.Get().LogLevel)
	}
	if rc.Get().CircuitBuildTimeout != 90*time.Second {
		t.Errorf("Expected CircuitBuildTimeout 90s after reload, got %v", rc.Get().CircuitBuildTimeout)
	}
}

func TestReloadableConfig_CheckAndReload(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "torrc")

	// Write initial config
	initialConfig := `LogLevel info`
	if err := os.WriteFile(configPath, []byte(initialConfig), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Load config
	cfg := DefaultConfig()
	if err := LoadFromFile(configPath, cfg); err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	rc := NewReloadableConfig(cfg, configPath, nil)

	// First check - file hasn't changed, should return nil without reloading
	if err := rc.checkAndReload(); err != nil {
		t.Errorf("checkAndReload should return nil when file unchanged: %v", err)
	}

	// Modify file
	time.Sleep(10 * time.Millisecond) // Ensure different mod time
	updatedConfig := `LogLevel debug`
	if err := os.WriteFile(configPath, []byte(updatedConfig), 0644); err != nil {
		t.Fatalf("Failed to update config file: %v", err)
	}

	// Second check - file changed, should reload
	if err := rc.checkAndReload(); err != nil {
		t.Fatalf("checkAndReload failed: %v", err)
	}

	if rc.Get().LogLevel != "debug" {
		t.Errorf("Expected LogLevel 'debug', got '%s'", rc.Get().LogLevel)
	}
}

func TestReloadableConfig_StartWatcher(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "torrc")

	// Write initial config
	initialConfig := `LogLevel info`
	if err := os.WriteFile(configPath, []byte(initialConfig), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Load config
	cfg := DefaultConfig()
	if err := LoadFromFile(configPath, cfg); err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError, // Reduce noise in test output
	}))
	rc := NewReloadableConfig(cfg, configPath, logger)

	// Start watcher with short interval
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go rc.StartWatcher(ctx, 50*time.Millisecond)

	// Give watcher time to start
	time.Sleep(20 * time.Millisecond)

	// Modify config file
	updatedConfig := `LogLevel debug`
	if err := os.WriteFile(configPath, []byte(updatedConfig), 0644); err != nil {
		t.Fatalf("Failed to update config file: %v", err)
	}

	// Wait for watcher to detect change (should happen within 2-3 check intervals)
	timeout := time.After(200 * time.Millisecond)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	reloaded := false
	for !reloaded {
		select {
		case <-timeout:
			t.Fatal("Watcher did not detect config change within timeout")
		case <-ticker.C:
			if rc.Get().LogLevel == "debug" {
				reloaded = true
			}
		}
	}

	// Stop watcher
	rc.Stop()
}

func TestReloadableConfig_StartWatcher_NoConfigPath(t *testing.T) {
	cfg := DefaultConfig()
	rc := NewReloadableConfig(cfg, "", nil) // Empty config path

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Should return immediately and not block
	done := make(chan struct{})
	go func() {
		rc.StartWatcher(ctx, 50*time.Millisecond)
		close(done)
	}()

	select {
	case <-done:
		// Success - watcher returned immediately
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Watcher should return immediately when no config path specified")
	}
}

func TestReloadableConfig_InvalidConfigReload(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "torrc")

	// Write valid initial config
	initialConfig := `LogLevel info`
	if err := os.WriteFile(configPath, []byte(initialConfig), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg := DefaultConfig()
	if err := LoadFromFile(configPath, cfg); err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	rc := NewReloadableConfig(cfg, configPath, nil)

	// Write invalid config
	time.Sleep(10 * time.Millisecond)
	invalidConfig := `LogLevel invalid_level` // Invalid log level
	if err := os.WriteFile(configPath, []byte(invalidConfig), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Reload should fail
	if err := rc.Reload(); err == nil {
		t.Fatal("Expected error when reloading invalid config, got nil")
	}

	// Original config should be preserved
	if rc.Get().LogLevel != "info" {
		t.Errorf("Original config should be preserved, expected 'info', got '%s'", rc.Get().LogLevel)
	}
}

func TestReloadableFields(t *testing.T) {
	// Verify expected reloadable fields are present
	expectedReloadable := []string{
		"LogLevel",
		"MaxCircuitDirtiness",
		"NewCircuitPeriod",
		"CircuitBuildTimeout",
		"CircuitPoolMinSize",
		"CircuitPoolMaxSize",
		"EnableCircuitPrebuilding",
		"ConnectionPoolMaxIdle",
		"ConnectionPoolMaxLife",
		"EnableConnectionPooling",
		"EnableBufferPooling",
	}

	for _, field := range expectedReloadable {
		if !ReloadableFields[field] {
			t.Errorf("Field '%s' should be reloadable but is not in ReloadableFields map", field)
		}
	}

	// Verify critical non-reloadable fields are NOT present
	nonReloadable := []string{
		"SocksPort",
		"ControlPort",
		"DataDirectory",
		"MetricsPort",
	}

	for _, field := range nonReloadable {
		if ReloadableFields[field] {
			t.Errorf("Field '%s' should NOT be reloadable but is in ReloadableFields map", field)
		}
	}
}
