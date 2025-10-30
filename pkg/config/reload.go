// Package config provides configuration management for the Tor client.
package config

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ReloadableConfig wraps a Config with hot reload capabilities
type ReloadableConfig struct {
	mu              sync.RWMutex
	config          *Config
	configPath      string
	lastModTime     time.Time
	reloadCallbacks []ReloadCallback
	logger          *slog.Logger
	stopCh          chan struct{}
	doneCh          chan struct{}
}

// ReloadCallback is called when configuration is successfully reloaded
// It receives the old and new configuration for comparison
type ReloadCallback func(oldConfig, newConfig *Config) error

// ReloadableFields lists which configuration fields support hot reload
// Fields not in this list require a service restart to take effect
var ReloadableFields = map[string]bool{
	"LogLevel":                 true,
	"MaxCircuitDirtiness":      true,
	"NewCircuitPeriod":         true,
	"CircuitBuildTimeout":      true,
	"CircuitPoolMinSize":       true,
	"CircuitPoolMaxSize":       true,
	"EnableCircuitPrebuilding": true,
	"ConnectionPoolMaxIdle":    true,
	"ConnectionPoolMaxLife":    true,
	"EnableConnectionPooling":  true,
	"EnableBufferPooling":      true,
	"IsolateDestinations":      true,
	"IsolateSOCKSAuth":         true,
	"IsolateClientPort":        true,
	"IsolateClientProtocol":    true,
}

// NewReloadableConfig creates a new reloadable configuration
func NewReloadableConfig(config *Config, configPath string, logger *slog.Logger) *ReloadableConfig {
	if logger == nil {
		logger = slog.Default()
	}

	var modTime time.Time
	if configPath != "" {
		if info, err := os.Stat(configPath); err == nil {
			modTime = info.ModTime()
		}
	}

	return &ReloadableConfig{
		config:          config,
		configPath:      configPath,
		lastModTime:     modTime,
		reloadCallbacks: make([]ReloadCallback, 0),
		logger:          logger,
		stopCh:          make(chan struct{}),
		doneCh:          make(chan struct{}),
	}
}

// Get returns a copy of the current configuration (thread-safe)
func (rc *ReloadableConfig) Get() *Config {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	// Return a shallow copy to prevent external modifications
	cfg := *rc.config
	return &cfg
}

// OnReload registers a callback to be called when configuration is reloaded
func (rc *ReloadableConfig) OnReload(callback ReloadCallback) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.reloadCallbacks = append(rc.reloadCallbacks, callback)
}

// StartWatcher starts watching the configuration file for changes
// It checks for modifications every interval and reloads if the file changed
func (rc *ReloadableConfig) StartWatcher(ctx context.Context, interval time.Duration) {
	if rc.configPath == "" {
		rc.logger.Warn("Configuration hot reload disabled: no config file specified")
		close(rc.doneCh)
		return
	}

	rc.logger.Info("Starting configuration file watcher",
		"path", rc.configPath,
		"interval", interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	defer close(rc.doneCh)

	for {
		select {
		case <-ctx.Done():
			rc.logger.Info("Configuration watcher stopped: context cancelled")
			return
		case <-rc.stopCh:
			rc.logger.Info("Configuration watcher stopped")
			return
		case <-ticker.C:
			if err := rc.checkAndReload(); err != nil {
				rc.logger.Error("Failed to reload configuration",
					"error", err,
					"path", rc.configPath)
			}
		}
	}
}

// Stop stops the configuration watcher
func (rc *ReloadableConfig) Stop() {
	close(rc.stopCh)
	<-rc.doneCh
}

// checkAndReload checks if the config file has changed and reloads if necessary
func (rc *ReloadableConfig) checkAndReload() error {
	// Check if file has been modified
	info, err := os.Stat(rc.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			rc.logger.Warn("Configuration file disappeared", "path", rc.configPath)
			return nil
		}
		return fmt.Errorf("stat config file: %w", err)
	}

	modTime := info.ModTime()
	if !modTime.After(rc.lastModTime) {
		// File hasn't changed
		return nil
	}

	rc.logger.Info("Configuration file changed, reloading",
		"path", rc.configPath,
		"old_mod_time", rc.lastModTime,
		"new_mod_time", modTime)

	// Load and validate new configuration
	newConfig, err := rc.loadConfigFile()
	if err != nil {
		return fmt.Errorf("load config file: %w", err)
	}

	if err := newConfig.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// Apply the new configuration
	if err := rc.applyConfig(newConfig); err != nil {
		return fmt.Errorf("apply config: %w", err)
	}

	// Update last modified time on success
	rc.lastModTime = modTime

	rc.logger.Info("Configuration reloaded successfully", "path", rc.configPath)
	return nil
}

// Reload explicitly reloads configuration from the file
func (rc *ReloadableConfig) Reload() error {
	if rc.configPath == "" {
		return fmt.Errorf("no configuration file specified")
	}

	rc.logger.Info("Manually reloading configuration", "path", rc.configPath)

	// Load and validate new configuration
	newConfig, err := rc.loadConfigFile()
	if err != nil {
		return fmt.Errorf("load config file: %w", err)
	}

	if err := newConfig.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// Apply the new configuration
	if err := rc.applyConfig(newConfig); err != nil {
		return fmt.Errorf("apply config: %w", err)
	}

	// Update last modified time
	if info, err := os.Stat(rc.configPath); err == nil {
		rc.lastModTime = info.ModTime()
	}

	rc.logger.Info("Configuration reloaded successfully", "path", rc.configPath)
	return nil
}

// loadConfigFile loads configuration from the file
func (rc *ReloadableConfig) loadConfigFile() (*Config, error) {
	// Start with default configuration
	newConfig := DefaultConfig()

	ext := filepath.Ext(rc.configPath)
	switch ext {
	case ".conf", ".torrc", "":
		// Parse torrc-style configuration
		if err := LoadFromFile(rc.configPath, newConfig); err != nil {
			return nil, err
		}
		return newConfig, nil
	default:
		return nil, fmt.Errorf("unsupported config file extension: %s", ext)
	}
}

// applyConfig applies the new configuration, merging reloadable fields
func (rc *ReloadableConfig) applyConfig(newConfig *Config) error {
	rc.mu.Lock()
	oldConfig := rc.config

	// Create a merged config that preserves non-reloadable fields
	mergedConfig := rc.mergeReloadableFields(oldConfig, newConfig)

	// Temporarily store the merged config for validation
	tempConfig := mergedConfig
	rc.mu.Unlock()

	// Call reload callbacks with old and new config
	// Callbacks can validate the change before it's applied
	for _, callback := range rc.reloadCallbacks {
		if err := callback(oldConfig, tempConfig); err != nil {
			rc.logger.Error("Reload callback failed, rolling back",
				"error", err)
			return fmt.Errorf("reload callback failed: %w", err)
		}
	}

	// Callbacks succeeded, apply the config
	rc.mu.Lock()
	rc.config = tempConfig
	rc.mu.Unlock()

	rc.logReloadedFields(oldConfig, tempConfig)
	return nil
}

// mergeReloadableFields creates a new config with only reloadable fields updated
func (rc *ReloadableConfig) mergeReloadableFields(oldConfig, newConfig *Config) *Config {
	// Start with a copy of the old config
	merged := *oldConfig

	// Update only reloadable fields from new config
	if ReloadableFields["LogLevel"] {
		merged.LogLevel = newConfig.LogLevel
	}
	if ReloadableFields["MaxCircuitDirtiness"] {
		merged.MaxCircuitDirtiness = newConfig.MaxCircuitDirtiness
	}
	if ReloadableFields["NewCircuitPeriod"] {
		merged.NewCircuitPeriod = newConfig.NewCircuitPeriod
	}
	if ReloadableFields["CircuitBuildTimeout"] {
		merged.CircuitBuildTimeout = newConfig.CircuitBuildTimeout
	}
	if ReloadableFields["CircuitPoolMinSize"] {
		merged.CircuitPoolMinSize = newConfig.CircuitPoolMinSize
	}
	if ReloadableFields["CircuitPoolMaxSize"] {
		merged.CircuitPoolMaxSize = newConfig.CircuitPoolMaxSize
	}
	if ReloadableFields["EnableCircuitPrebuilding"] {
		merged.EnableCircuitPrebuilding = newConfig.EnableCircuitPrebuilding
	}
	if ReloadableFields["ConnectionPoolMaxIdle"] {
		merged.ConnectionPoolMaxIdle = newConfig.ConnectionPoolMaxIdle
	}
	if ReloadableFields["ConnectionPoolMaxLife"] {
		merged.ConnectionPoolMaxLife = newConfig.ConnectionPoolMaxLife
	}
	if ReloadableFields["EnableConnectionPooling"] {
		merged.EnableConnectionPooling = newConfig.EnableConnectionPooling
	}
	if ReloadableFields["EnableBufferPooling"] {
		merged.EnableBufferPooling = newConfig.EnableBufferPooling
	}
	if ReloadableFields["IsolateDestinations"] {
		merged.IsolateDestinations = newConfig.IsolateDestinations
	}
	if ReloadableFields["IsolateSOCKSAuth"] {
		merged.IsolateSOCKSAuth = newConfig.IsolateSOCKSAuth
	}
	if ReloadableFields["IsolateClientPort"] {
		merged.IsolateClientPort = newConfig.IsolateClientPort
	}
	if ReloadableFields["IsolateClientProtocol"] {
		merged.IsolateClientProtocol = newConfig.IsolateClientProtocol
	}

	return &merged
}

// logReloadedFields logs which fields were changed
func (rc *ReloadableConfig) logReloadedFields(oldConfig, newConfig *Config) {
	changes := make([]string, 0)

	if oldConfig.LogLevel != newConfig.LogLevel {
		changes = append(changes, fmt.Sprintf("LogLevel: %s -> %s", oldConfig.LogLevel, newConfig.LogLevel))
	}
	if oldConfig.MaxCircuitDirtiness != newConfig.MaxCircuitDirtiness {
		changes = append(changes, fmt.Sprintf("MaxCircuitDirtiness: %v -> %v", oldConfig.MaxCircuitDirtiness, newConfig.MaxCircuitDirtiness))
	}
	if oldConfig.NewCircuitPeriod != newConfig.NewCircuitPeriod {
		changes = append(changes, fmt.Sprintf("NewCircuitPeriod: %v -> %v", oldConfig.NewCircuitPeriod, newConfig.NewCircuitPeriod))
	}
	if oldConfig.CircuitBuildTimeout != newConfig.CircuitBuildTimeout {
		changes = append(changes, fmt.Sprintf("CircuitBuildTimeout: %v -> %v", oldConfig.CircuitBuildTimeout, newConfig.CircuitBuildTimeout))
	}
	if oldConfig.CircuitPoolMinSize != newConfig.CircuitPoolMinSize {
		changes = append(changes, fmt.Sprintf("CircuitPoolMinSize: %d -> %d", oldConfig.CircuitPoolMinSize, newConfig.CircuitPoolMinSize))
	}
	if oldConfig.CircuitPoolMaxSize != newConfig.CircuitPoolMaxSize {
		changes = append(changes, fmt.Sprintf("CircuitPoolMaxSize: %d -> %d", oldConfig.CircuitPoolMaxSize, newConfig.CircuitPoolMaxSize))
	}
	if oldConfig.EnableCircuitPrebuilding != newConfig.EnableCircuitPrebuilding {
		changes = append(changes, fmt.Sprintf("EnableCircuitPrebuilding: %v -> %v", oldConfig.EnableCircuitPrebuilding, newConfig.EnableCircuitPrebuilding))
	}

	if len(changes) > 0 {
		rc.logger.Info("Configuration fields updated",
			"changes", changes,
			"count", len(changes))
	}
}
