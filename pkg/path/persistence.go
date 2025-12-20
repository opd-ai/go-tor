// Package path provides guard node persistence for Tor circuits.
package path

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/gofrs/flock"
	"github.com/opd-ai/go-tor/pkg/logger"
)

// PersistenceConfig holds configuration for guard state persistence.
type PersistenceConfig struct {
	// FilePath is the path to the guard state file.
	FilePath string
	// BackupCount is the number of backup files to retain.
	BackupCount int
	// SnapshotInterval is the interval between automatic state snapshots.
	// Set to 0 to disable automatic snapshots.
	SnapshotInterval time.Duration
	// LockTimeout is the timeout for acquiring the file lock.
	LockTimeout time.Duration
}

// DefaultPersistenceConfig returns sensible defaults for persistence.
func DefaultPersistenceConfig(filePath string) *PersistenceConfig {
	return &PersistenceConfig{
		FilePath:         filePath,
		BackupCount:      3,
		SnapshotInterval: 5 * time.Minute,
		LockTimeout:      10 * time.Second,
	}
}

// GuardStateV2 represents the versioned guard state with integrity checks.
type GuardStateV2 struct {
	Version   int          `json:"version"`
	Guards    []GuardEntry `json:"guards"`
	UpdatedAt time.Time    `json:"updated_at"`
	Checksum  string       `json:"checksum"`
}

// currentSchemaVersion is the current schema version for guard state.
const currentSchemaVersion = 2

// Persistence manages atomic file writes, locking, and backup rotation
// for guard state persistence.
type Persistence struct {
	config *PersistenceConfig
	logger *logger.Logger
	flock  *flock.Flock
	mu     sync.Mutex

	// snapshotCancel cancels the snapshot loop.
	snapshotCancel context.CancelFunc
	snapshotWg     sync.WaitGroup
}

// NewPersistence creates a new Persistence manager.
func NewPersistence(config *PersistenceConfig, log *logger.Logger) *Persistence {
	if log == nil {
		log = logger.NewDefault()
	}
	if config == nil {
		config = DefaultPersistenceConfig("guard_state.json")
	}

	return &Persistence{
		config: config,
		logger: log.Component("persistence"),
		flock:  flock.New(config.FilePath + ".lock"),
	}
}

// calculateChecksum returns the hex-encoded SHA-256 checksum of the given data.
func calculateChecksum(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// verifyChecksum recomputes the checksum for the given state (excluding the
// Checksum field itself) and compares it with the stored Checksum value.
// Returns true only if the checksum matches.
func verifyChecksum(state *GuardStateV2) bool {
	if state == nil || state.Checksum == "" {
		return false
	}

	// Make a copy and clear Checksum so it's not included in calculation
	copyState := *state
	copyState.Checksum = ""

	data, err := json.Marshal(copyState)
	if err != nil {
		return false
	}

	expected := calculateChecksum(data)
	return expected == state.Checksum
}

// acquireLock acquires an exclusive file lock with timeout.
func (p *Persistence) acquireLock(ctx context.Context) error {
	lockCtx, cancel := context.WithTimeout(ctx, p.config.LockTimeout)
	defer cancel()

	locked, err := p.flock.TryLockContext(lockCtx, 100*time.Millisecond)
	if err != nil {
		return fmt.Errorf("failed to acquire file lock: %w", err)
	}
	if !locked {
		return fmt.Errorf("timeout waiting for file lock after %v", p.config.LockTimeout)
	}
	return nil
}

// releaseLock releases the file lock.
func (p *Persistence) releaseLock() error {
	return p.flock.Unlock()
}

// copyFile copies src to dst with proper resource cleanup.
// Preserves restrictive permissions (0600) for security.
func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer source.Close()

	// Get source file info for size verification
	info, err := source.Stat()
	if err != nil {
		return fmt.Errorf("stat source: %w", err)
	}

	// Use restrictive permissions for security
	dest, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("create destination: %w", err)
	}

	n, err := io.Copy(dest, source)
	if closeErr := dest.Close(); closeErr != nil && err == nil {
		err = fmt.Errorf("close destination: %w", closeErr)
	}
	if err != nil {
		os.Remove(dst) // Clean up partial copy on error
		return fmt.Errorf("copy failed: %w", err)
	}

	// Verify copy was complete
	if n != info.Size() {
		os.Remove(dst)
		return fmt.Errorf("incomplete copy: wrote %d of %d bytes", n, info.Size())
	}

	return nil
}

// rotateBackups creates a backup of the current state file and rotates old backups.
// Keeps the last BackupCount backup files.
func (p *Persistence) rotateBackups() error {
	path := p.config.FilePath
	keepN := p.config.BackupCount

	if keepN <= 0 {
		return nil // Backups disabled
	}

	// Check if source file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil // Nothing to back up
	}

	// Rotate from oldest to newest: .backup.N -> .backup.N+1
	for i := keepN - 1; i >= 1; i-- {
		oldPath := fmt.Sprintf("%s.backup.%d", path, i)
		newPath := fmt.Sprintf("%s.backup.%d", path, i+1)
		if err := os.Rename(oldPath, newPath); err != nil {
			// Ignore missing older backups, but fail on other errors
			if !os.IsNotExist(err) {
				return fmt.Errorf("failed to rotate backup %s to %s: %w", oldPath, newPath, err)
			}
		}
	}

	// Create new backup as .backup.1
	if err := copyFile(path, path+".backup.1"); err != nil {
		return fmt.Errorf("failed to create backup for %s: %w", path, err)
	}

	p.logger.Debug("Created backup", "path", path+".backup.1")
	return nil
}

// Save atomically saves the guard state to disk with locking, checksums, and backups.
func (p *Persistence) Save(ctx context.Context, guards []GuardEntry) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Acquire file lock
	if err := p.acquireLock(ctx); err != nil {
		return err
	}
	defer p.releaseLock()

	// Rotate backups before writing new state
	if err := p.rotateBackups(); err != nil {
		p.logger.Warn("Failed to rotate backups", "error", err)
		// Continue with save even if backup rotation fails
	}

	// Create versioned state
	state := &GuardStateV2{
		Version:   currentSchemaVersion,
		Guards:    guards,
		UpdatedAt: time.Now(),
	}

	// Marshal without checksum first
	dataWithoutChecksum, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal guard state: %w", err)
	}

	// Calculate and set checksum
	state.Checksum = calculateChecksum(dataWithoutChecksum)

	// Marshal with checksum
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal guard state with checksum: %w", err)
	}

	// Write to temporary file first, then rename for atomic update
	tmpFile := p.config.FilePath + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0o600); err != nil {
		return fmt.Errorf("failed to write guard state: %w", err)
	}

	if err := os.Rename(tmpFile, p.config.FilePath); err != nil {
		os.Remove(tmpFile) // Clean up temp file on rename failure
		return fmt.Errorf("failed to rename guard state file: %w", err)
	}

	p.logger.Debug("Saved guard state", "guards", len(guards), "checksum", state.Checksum[:16]+"...")
	return nil
}

// Load loads the guard state from disk with integrity verification.
func (p *Persistence) Load(ctx context.Context) ([]GuardEntry, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Acquire file lock for reading
	if err := p.acquireLock(ctx); err != nil {
		return nil, err
	}
	defer p.releaseLock()

	data, err := os.ReadFile(p.config.FilePath)
	if err != nil {
		return nil, err
	}

	// Try to parse as V2 first
	var stateV2 GuardStateV2
	if err := json.Unmarshal(data, &stateV2); err != nil {
		return nil, fmt.Errorf("failed to parse guard state: %w", err)
	}

	// Handle schema migration from V1 (no version field means V1)
	if stateV2.Version == 0 {
		// V1 format: try to parse as old GuardState
		var oldState GuardState
		if err := json.Unmarshal(data, &oldState); err != nil {
			return nil, fmt.Errorf("failed to parse legacy guard state: %w", err)
		}
		p.logger.Info("Migrating guard state from V1 to V2",
			"guards", len(oldState.Guards))
		return oldState.Guards, nil
	}

	// Verify checksum for V2
	if !verifyChecksum(&stateV2) {
		p.logger.Warn("Guard state checksum mismatch, attempting backup recovery")
		return p.loadFromBackup(ctx)
	}

	p.logger.Info("Loaded guard state",
		"version", stateV2.Version,
		"guards", len(stateV2.Guards),
		"updated_at", stateV2.UpdatedAt)

	return stateV2.Guards, nil
}

// loadFromBackup attempts to load guard state from backup files.
func (p *Persistence) loadFromBackup(ctx context.Context) ([]GuardEntry, error) {
	for i := 1; i <= p.config.BackupCount; i++ {
		backupPath := fmt.Sprintf("%s.backup.%d", p.config.FilePath, i)

		data, err := os.ReadFile(backupPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			p.logger.Warn("Failed to read backup", "path", backupPath, "error", err)
			continue
		}

		var stateV2 GuardStateV2
		if err := json.Unmarshal(data, &stateV2); err != nil {
			p.logger.Warn("Failed to parse backup", "path", backupPath, "error", err)
			continue
		}

		// For V2, verify checksum
		if stateV2.Version >= currentSchemaVersion && !verifyChecksum(&stateV2) {
			p.logger.Warn("Backup checksum mismatch", "path", backupPath)
			continue
		}

		p.logger.Info("Recovered guard state from backup",
			"path", backupPath,
			"guards", len(stateV2.Guards))
		return stateV2.Guards, nil
	}

	return nil, fmt.Errorf("no valid backup found")
}

// StartSnapshotLoop starts a goroutine that periodically saves guard state.
// The provided getGuards function is called to retrieve the current guard list.
// Returns immediately if SnapshotInterval is 0.
func (p *Persistence) StartSnapshotLoop(getGuards func() []GuardEntry) {
	if p.config.SnapshotInterval <= 0 {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	p.snapshotCancel = cancel

	p.snapshotWg.Add(1)
	go func() {
		defer p.snapshotWg.Done()
		p.snapshotLoop(ctx, getGuards)
	}()

	p.logger.Info("Started snapshot loop", "interval", p.config.SnapshotInterval)
}

// StopSnapshotLoop stops the snapshot loop and waits for it to finish.
func (p *Persistence) StopSnapshotLoop() {
	if p.snapshotCancel != nil {
		p.snapshotCancel()
		p.snapshotWg.Wait()
		p.logger.Info("Stopped snapshot loop")
	}
}

// snapshotLoop periodically saves guard state.
func (p *Persistence) snapshotLoop(ctx context.Context, getGuards func() []GuardEntry) {
	ticker := time.NewTicker(p.config.SnapshotInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.logger.Debug("Stopping snapshot loop", "reason", ctx.Err())
			return
		case <-ticker.C:
			guards := getGuards()
			if err := p.Save(ctx, guards); err != nil {
				if ctx.Err() != nil {
					// Context was cancelled, exit silently
					return
				}
				p.logger.Error("Failed to save guard state snapshot", "error", err)
			}
		}
	}
}

// migrateV1ToV2 is a helper for explicit migration if needed.
func migrateV1ToV2(oldState *GuardState) *GuardStateV2 {
	state := &GuardStateV2{
		Version:   currentSchemaVersion,
		Guards:    oldState.Guards,
		UpdatedAt: oldState.LastUpdated,
	}

	// Calculate checksum - Marshal should not fail for a valid struct,
	// but if it does, we'll have an empty checksum which is detectable
	data, err := json.Marshal(&GuardStateV2{
		Version:   state.Version,
		Guards:    state.Guards,
		UpdatedAt: state.UpdatedAt,
	})
	if err != nil {
		// Fall back to empty checksum - callers should verify checksum validity
		state.Checksum = ""
		return state
	}
	state.Checksum = calculateChecksum(data)

	return state
}

// FileExists returns true if the guard state file exists.
func (p *Persistence) FileExists() bool {
	_, err := os.Stat(p.config.FilePath)
	return err == nil
}

// GetBackupPaths returns the paths to all backup files that exist.
func (p *Persistence) GetBackupPaths() []string {
	var paths []string
	for i := 1; i <= p.config.BackupCount; i++ {
		backupPath := fmt.Sprintf("%s.backup.%d", p.config.FilePath, i)
		if _, err := os.Stat(backupPath); err == nil {
			paths = append(paths, backupPath)
		}
	}
	return paths
}
