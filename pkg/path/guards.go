// Package path provides guard node persistence for Tor circuits.
package path

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/opd-ai/go-tor/pkg/directory"
	"github.com/opd-ai/go-tor/pkg/logger"
)

// GuardState represents the persistent state of guard nodes
type GuardState struct {
	Guards      []GuardEntry `json:"guards"`
	LastUpdated time.Time    `json:"last_updated"`
}

// GuardEntry represents a persisted guard node
type GuardEntry struct {
	Fingerprint string    `json:"fingerprint"`
	Nickname    string    `json:"nickname"`
	Address     string    `json:"address"`
	FirstUsed   time.Time `json:"first_used"`
	LastUsed    time.Time `json:"last_used"`
	Confirmed   bool      `json:"confirmed"`
}

// GuardManagerConfig holds configuration for the guard manager.
type GuardManagerConfig struct {
	// DataDir is the directory for persistent state.
	DataDir string
	// MaxGuards is the maximum number of guard nodes to maintain.
	MaxGuards int
	// GuardExpiry is how long before a guard expires without use.
	GuardExpiry time.Duration
	// BackupCount is the number of backup files to retain.
	BackupCount int
	// SnapshotInterval is the interval between automatic state snapshots.
	// Set to 0 to disable automatic snapshots.
	SnapshotInterval time.Duration
	// LockTimeout is the timeout for acquiring the file lock.
	LockTimeout time.Duration
}

// DefaultGuardManagerConfig returns sensible defaults for guard management.
func DefaultGuardManagerConfig(dataDir string) *GuardManagerConfig {
	return &GuardManagerConfig{
		DataDir:          dataDir,
		MaxGuards:        3,                   // Tor typically uses 3 guard nodes
		GuardExpiry:      90 * 24 * time.Hour, // 90 days per Tor spec
		BackupCount:      3,
		SnapshotInterval: 5 * time.Minute,
		LockTimeout:      10 * time.Second,
	}
}

// GuardManager manages persistent guard nodes
type GuardManager struct {
	logger      *logger.Logger
	stateFile   string
	state       GuardState
	mu          sync.RWMutex
	maxGuards   int
	guardExpiry time.Duration

	// persistence is the enhanced persistence layer (optional)
	persistence *Persistence
}

// NewGuardManager creates a new guard manager.
// This is the backward-compatible constructor that uses legacy persistence.
// For enhanced persistence with file locking, backups, and snapshots,
// use NewGuardManagerWithConfig instead.
func NewGuardManager(dataDir string, log *logger.Logger) (*GuardManager, error) {
	config := DefaultGuardManagerConfig(dataDir)
	// Disable enhanced persistence for backward compatibility
	config.BackupCount = 0
	config.SnapshotInterval = 0
	return NewGuardManagerWithConfig(config, log)
}

// NewGuardManagerWithConfig creates a new guard manager with enhanced configuration.
// This constructor enables all persistence features including file locking,
// backup rotation, and automatic snapshots.
func NewGuardManagerWithConfig(config *GuardManagerConfig, log *logger.Logger) (*GuardManager, error) {
	if log == nil {
		log = logger.NewDefault()
	}

	if config == nil {
		return nil, fmt.Errorf("config is required")
	}

	// Ensure data directory exists
	if err := os.MkdirAll(config.DataDir, 0o700); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	stateFile := filepath.Join(config.DataDir, "guard_state.json")

	gm := &GuardManager{
		logger:      log.Component("guards"),
		stateFile:   stateFile,
		maxGuards:   config.MaxGuards,
		guardExpiry: config.GuardExpiry,
	}

	// Set up enhanced persistence if backups or snapshots are enabled
	if config.BackupCount > 0 || config.SnapshotInterval > 0 {
		persistConfig := &PersistenceConfig{
			FilePath:         stateFile,
			BackupCount:      config.BackupCount,
			SnapshotInterval: config.SnapshotInterval,
			LockTimeout:      config.LockTimeout,
		}
		gm.persistence = NewPersistence(persistConfig, log)
	}

	// Load existing state if available
	if err := gm.load(); err != nil {
		// If file doesn't exist, that's okay - we'll create it later
		if !os.IsNotExist(err) {
			log.Warn("Failed to load guard state", "error", err)
		}
	}

	return gm, nil
}

// load loads guard state from disk
func (gm *GuardManager) load() error {
	// Use enhanced persistence if available
	if gm.persistence != nil {
		ctx := context.Background()
		guards, err := gm.persistence.Load(ctx)
		if err != nil {
			return err
		}
		gm.mu.Lock()
		gm.state.Guards = guards
		gm.state.LastUpdated = time.Now()
		gm.mu.Unlock()
		return nil
	}

	// Legacy load
	gm.mu.Lock()
	defer gm.mu.Unlock()

	data, err := os.ReadFile(gm.stateFile)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(data, &gm.state); err != nil {
		return fmt.Errorf("failed to parse guard state: %w", err)
	}

	gm.logger.Info("Loaded guard state",
		"guards", len(gm.state.Guards),
		"last_updated", gm.state.LastUpdated)

	return nil
}

// Save saves guard state to disk
func (gm *GuardManager) Save() error {
	// Use enhanced persistence if available
	if gm.persistence != nil {
		gm.mu.RLock()
		guards := make([]GuardEntry, len(gm.state.Guards))
		copy(guards, gm.state.Guards)
		gm.mu.RUnlock()

		ctx := context.Background()
		return gm.persistence.Save(ctx, guards)
	}

	// Legacy save
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	gm.state.LastUpdated = time.Now()

	data, err := json.MarshalIndent(gm.state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal guard state: %w", err)
	}

	// Write to temporary file first, then rename for atomic update
	tmpFile := gm.stateFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0o600); err != nil {
		return fmt.Errorf("failed to write guard state: %w", err)
	}

	if err := os.Rename(tmpFile, gm.stateFile); err != nil {
		return fmt.Errorf("failed to rename guard state file: %w", err)
	}

	gm.logger.Debug("Saved guard state", "guards", len(gm.state.Guards))
	return nil
}

// GetGuards returns the list of persisted guards that are still valid
func (gm *GuardManager) GetGuards() []GuardEntry {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	now := time.Now()
	validGuards := make([]GuardEntry, 0)

	for _, guard := range gm.state.Guards {
		// Check if guard hasn't expired
		if now.Sub(guard.LastUsed) < gm.guardExpiry {
			validGuards = append(validGuards, guard)
		}
	}

	return validGuards
}

// AddGuard adds or updates a guard node
func (gm *GuardManager) AddGuard(relay *directory.Relay) error {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	now := time.Now()

	// Check if guard already exists
	for i, guard := range gm.state.Guards {
		if guard.Fingerprint == relay.Fingerprint {
			// Update existing guard
			gm.state.Guards[i].LastUsed = now
			gm.state.Guards[i].Confirmed = true
			gm.logger.Debug("Updated existing guard", "nickname", relay.Nickname)
			return nil
		}
	}

	// Add new guard if we haven't reached the limit
	if len(gm.state.Guards) >= gm.maxGuards {
		// Remove oldest non-confirmed guard if possible
		removed := false
		for i, guard := range gm.state.Guards {
			if !guard.Confirmed {
				gm.state.Guards = append(gm.state.Guards[:i], gm.state.Guards[i+1:]...)
				removed = true
				gm.logger.Info("Removed non-confirmed guard to make room", "nickname", guard.Nickname)
				break
			}
		}

		// If all guards are confirmed, don't add new one
		if !removed {
			gm.logger.Debug("Guard limit reached, not adding new guard")
			return nil
		}
	}

	// Add new guard
	entry := GuardEntry{
		Fingerprint: relay.Fingerprint,
		Nickname:    relay.Nickname,
		Address:     relay.Address,
		FirstUsed:   now,
		LastUsed:    now,
		Confirmed:   false,
	}

	gm.state.Guards = append(gm.state.Guards, entry)
	gm.logger.Info("Added new guard", "nickname", relay.Nickname, "fingerprint", relay.Fingerprint)

	return nil
}

// ConfirmGuard marks a guard as confirmed (successfully used)
func (gm *GuardManager) ConfirmGuard(fingerprint string) error {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	for i, guard := range gm.state.Guards {
		if guard.Fingerprint == fingerprint {
			gm.state.Guards[i].Confirmed = true
			gm.state.Guards[i].LastUsed = time.Now()
			gm.logger.Info("Confirmed guard", "nickname", guard.Nickname)
			return nil
		}
	}

	return fmt.Errorf("guard not found: %s", fingerprint)
}

// RemoveGuard removes a guard from persistence
func (gm *GuardManager) RemoveGuard(fingerprint string) error {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	for i, guard := range gm.state.Guards {
		if guard.Fingerprint == fingerprint {
			gm.state.Guards = append(gm.state.Guards[:i], gm.state.Guards[i+1:]...)
			gm.logger.Info("Removed guard", "nickname", guard.Nickname)
			return nil
		}
	}

	return fmt.Errorf("guard not found: %s", fingerprint)
}

// CleanupExpired removes expired guards
func (gm *GuardManager) CleanupExpired() {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	now := time.Now()
	validGuards := make([]GuardEntry, 0)

	for _, guard := range gm.state.Guards {
		if now.Sub(guard.LastUsed) < gm.guardExpiry {
			validGuards = append(validGuards, guard)
		} else {
			gm.logger.Info("Removing expired guard",
				"nickname", guard.Nickname,
				"last_used", guard.LastUsed)
		}
	}

	if len(validGuards) != len(gm.state.Guards) {
		gm.state.Guards = validGuards
		gm.logger.Info("Cleaned up expired guards",
			"removed", len(gm.state.Guards)-len(validGuards),
			"remaining", len(validGuards))
	}
}

// GetStats returns guard statistics
func (gm *GuardManager) GetStats() GuardStats {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	confirmed := 0
	for _, guard := range gm.state.Guards {
		if guard.Confirmed {
			confirmed++
		}
	}

	return GuardStats{
		TotalGuards:     len(gm.state.Guards),
		ConfirmedGuards: confirmed,
		LastUpdated:     gm.state.LastUpdated,
	}
}

// GuardStats represents guard node statistics
type GuardStats struct {
	TotalGuards     int
	ConfirmedGuards int
	LastUpdated     time.Time
}

// StartSnapshotLoop starts automatic periodic saving of guard state.
// This only works when enhanced persistence is enabled via NewGuardManagerWithConfig.
func (gm *GuardManager) StartSnapshotLoop() {
	if gm.persistence == nil {
		gm.logger.Debug("Snapshot loop not started: enhanced persistence not enabled")
		return
	}

	gm.persistence.StartSnapshotLoop(gm.GetGuards)
}

// StopSnapshotLoop stops the automatic periodic saving of guard state.
// This is safe to call even if the snapshot loop was never started.
func (gm *GuardManager) StopSnapshotLoop() {
	if gm.persistence != nil {
		gm.persistence.StopSnapshotLoop()
	}
}

// HasEnhancedPersistence returns true if enhanced persistence features are enabled.
func (gm *GuardManager) HasEnhancedPersistence() bool {
	return gm.persistence != nil
}

// GetBackupPaths returns the paths to all backup files that exist.
// Returns nil if enhanced persistence is not enabled.
func (gm *GuardManager) GetBackupPaths() []string {
	if gm.persistence == nil {
		return nil
	}
	return gm.persistence.GetBackupPaths()
}
