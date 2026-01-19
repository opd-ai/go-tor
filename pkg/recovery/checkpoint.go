// Package recovery provides crash recovery state checkpointing for the Tor client.
// This package implements periodic state snapshots that allow faster recovery after crashes
// by preserving runtime state like bootstrap progress, circuit statistics, and bandwidth counters.
package recovery

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/gofrs/flock"
	"github.com/opd-ai/go-tor/pkg/logger"
	"github.com/opd-ai/go-tor/pkg/metrics"
)

// CheckpointState represents the recoverable runtime state of the Tor client.
// This state is periodically saved to disk and can be used to speed up recovery after a crash.
type CheckpointState struct {
	// Version is the schema version for forward compatibility.
	Version int `json:"version"`

	// Bootstrap tracks client bootstrap progress.
	Bootstrap BootstrapState `json:"bootstrap"`

	// Bandwidth tracks cumulative bandwidth usage.
	Bandwidth BandwidthState `json:"bandwidth"`

	// Circuits tracks circuit build statistics for recovery optimization.
	Circuits CircuitState `json:"circuits"`

	// UpdatedAt is when this checkpoint was created.
	UpdatedAt time.Time `json:"updated_at"`

	// Checksum is the SHA-256 checksum for integrity verification.
	Checksum string `json:"checksum"`
}

// BootstrapState tracks client bootstrap progress for faster recovery.
type BootstrapState struct {
	// Phase indicates the current bootstrap phase (0-100 percent).
	Phase int `json:"phase"`
	// Status is a human-readable description of the current phase.
	Status string `json:"status"`
	// StartedAt is when bootstrap began.
	StartedAt time.Time `json:"started_at"`
	// CompletedAt is when bootstrap completed (zero if not complete).
	CompletedAt time.Time `json:"completed_at,omitempty"`
	// LastConsensusUpdate is when the consensus was last updated.
	LastConsensusUpdate time.Time `json:"last_consensus_update,omitempty"`
}

// BandwidthState tracks cumulative bandwidth for continuity across restarts.
type BandwidthState struct {
	// TotalBytesRead is the cumulative bytes read since first start.
	TotalBytesRead uint64 `json:"total_bytes_read"`
	// TotalBytesWritten is the cumulative bytes written since first start.
	TotalBytesWritten uint64 `json:"total_bytes_written"`
	// SessionBytesRead is bytes read in the current session.
	SessionBytesRead uint64 `json:"session_bytes_read"`
	// SessionBytesWritten is bytes written in the current session.
	SessionBytesWritten uint64 `json:"session_bytes_written"`
}

// CircuitState tracks circuit statistics for recovery optimization.
type CircuitState struct {
	// TotalBuilds is the cumulative circuit builds since first start.
	TotalBuilds int64 `json:"total_builds"`
	// TotalSuccesses is the cumulative successful circuit builds.
	TotalSuccesses int64 `json:"total_successes"`
	// TotalFailures is the cumulative failed circuit builds.
	TotalFailures int64 `json:"total_failures"`
	// LastBuildTime is when the last circuit was successfully built.
	LastBuildTime time.Time `json:"last_build_time,omitempty"`
	// AverageBuildTimeMs is the average circuit build time in milliseconds.
	AverageBuildTimeMs int64 `json:"average_build_time_ms"`
}

// currentSchemaVersion is the current checkpoint schema version.
const currentSchemaVersion = 1

// Default values for checkpoint operations.
const (
	// defaultLockRetryInterval is the interval between lock acquisition retries.
	defaultLockRetryInterval = 100 * time.Millisecond
	// defaultFinalCheckpointTimeout is the timeout for the final checkpoint save during shutdown.
	defaultFinalCheckpointTimeout = 5 * time.Second
	// emaAlpha is the smoothing factor for exponential moving average (0.1 = 10% weight to new values).
	emaAlpha = 0.1
)

// CheckpointConfig holds configuration for state checkpointing.
type CheckpointConfig struct {
	// FilePath is the path to the checkpoint file.
	FilePath string
	// CheckpointInterval is the interval between automatic checkpoints.
	// Set to 0 to disable automatic checkpointing.
	CheckpointInterval time.Duration
	// LockTimeout is the timeout for acquiring the file lock.
	LockTimeout time.Duration
	// BackupCount is the number of backup files to retain.
	BackupCount int
}

// DefaultCheckpointConfig returns sensible defaults for checkpointing.
func DefaultCheckpointConfig(filePath string) *CheckpointConfig {
	return &CheckpointConfig{
		FilePath:           filePath,
		CheckpointInterval: 1 * time.Minute,
		LockTimeout:        10 * time.Second,
		BackupCount:        2,
	}
}

// StateCheckpointer manages periodic state checkpointing with atomic writes.
type StateCheckpointer struct {
	config  *CheckpointConfig
	logger  *logger.Logger
	metrics *metrics.Metrics
	flock   *flock.Flock
	mu      sync.Mutex

	// Current state protected by stateMu
	stateMu sync.RWMutex
	state   *CheckpointState

	// Checkpoint loop management
	loopMu      sync.Mutex
	loopCancel  context.CancelFunc
	loopRunning bool
	loopWg      sync.WaitGroup
}

// NewStateCheckpointer creates a new state checkpointer.
func NewStateCheckpointer(config *CheckpointConfig, log *logger.Logger) *StateCheckpointer {
	return NewStateCheckpointerWithMetrics(config, log, nil)
}

// NewStateCheckpointerWithMetrics creates a new state checkpointer with metrics support.
func NewStateCheckpointerWithMetrics(config *CheckpointConfig, log *logger.Logger, m *metrics.Metrics) *StateCheckpointer {
	if log == nil {
		log = logger.NewDefault()
	}
	if config == nil {
		config = DefaultCheckpointConfig("checkpoint.json")
	}

	return &StateCheckpointer{
		config:  config,
		logger:  log.Component("checkpoint"),
		metrics: m,
		flock:   flock.New(config.FilePath + ".lock"),
		state: &CheckpointState{
			Version: currentSchemaVersion,
			Bootstrap: BootstrapState{
				Phase:  0,
				Status: "not_started",
			},
		},
	}
}

// calculateChecksum returns the hex-encoded SHA-256 checksum of the given data.
func calculateChecksum(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// verifyChecksum validates the checkpoint's integrity.
func verifyChecksum(state *CheckpointState) bool {
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
func (sc *StateCheckpointer) acquireLock(ctx context.Context) error {
	lockCtx, cancel := context.WithTimeout(ctx, sc.config.LockTimeout)
	defer cancel()

	locked, err := sc.flock.TryLockContext(lockCtx, defaultLockRetryInterval)
	if err != nil {
		return fmt.Errorf("failed to acquire checkpoint file lock: %w", err)
	}
	if !locked {
		return fmt.Errorf("timeout waiting for checkpoint file lock after %v", sc.config.LockTimeout)
	}
	return nil
}

// releaseLock releases the file lock.
func (sc *StateCheckpointer) releaseLock() error {
	return sc.flock.Unlock()
}

// copyFile copies src to dst with proper resource cleanup.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read source: %w", err)
	}

	if err := os.WriteFile(dst, data, 0o600); err != nil {
		return fmt.Errorf("write destination: %w", err)
	}

	return nil
}

// rotateBackups creates a backup of the current checkpoint file.
// It also cleans up any old backups beyond the configured count.
func (sc *StateCheckpointer) rotateBackups() error {
	path := sc.config.FilePath
	keepN := sc.config.BackupCount

	if keepN <= 0 {
		return nil // Backups disabled
	}

	// Check if source file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil // Nothing to back up
	}

	// Clean up any backups beyond the configured count (from previous runs with higher BackupCount)
	for i := keepN + 1; ; i++ {
		oldBackup := fmt.Sprintf("%s.backup.%d", path, i)
		if _, err := os.Stat(oldBackup); os.IsNotExist(err) {
			break // No more old backups
		}
		if err := os.Remove(oldBackup); err != nil {
			sc.logger.Warn("Failed to remove old backup", "path", oldBackup, "error", err)
		} else {
			sc.logger.Debug("Removed old backup beyond configured count", "path", oldBackup)
		}
	}

	// Rotate from oldest to newest: .backup.N -> .backup.N+1
	for i := keepN - 1; i >= 1; i-- {
		oldPath := fmt.Sprintf("%s.backup.%d", path, i)
		newPath := fmt.Sprintf("%s.backup.%d", path, i+1)
		if err := os.Rename(oldPath, newPath); err != nil {
			if !os.IsNotExist(err) {
				return fmt.Errorf("failed to rotate backup %s to %s: %w", oldPath, newPath, err)
			}
		}
	}

	// Create new backup as .backup.1
	if err := copyFile(path, path+".backup.1"); err != nil {
		return fmt.Errorf("failed to create backup for %s: %w", path, err)
	}

	sc.logger.Debug("Created checkpoint backup", "path", path+".backup.1")
	return nil
}

// Save atomically saves the current checkpoint state to disk.
func (sc *StateCheckpointer) Save(ctx context.Context) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	// Acquire file lock
	if err := sc.acquireLock(ctx); err != nil {
		sc.recordMetric(func(m *metrics.Metrics) { m.RecordCheckpointFailed() })
		return err
	}
	defer func() {
		if err := sc.releaseLock(); err != nil {
			sc.logger.Warn("Failed to release checkpoint file lock", "error", err)
		}
	}()

	// Rotate backups before writing new state
	if err := sc.rotateBackups(); err != nil {
		sc.logger.Warn("Failed to rotate checkpoint backups", "error", err)
		// Continue with save even if backup rotation fails
	}

	// Get current state with read lock
	sc.stateMu.RLock()
	stateCopy := *sc.state
	sc.stateMu.RUnlock()

	// Update timestamp
	stateCopy.UpdatedAt = time.Now()
	stateCopy.Version = currentSchemaVersion

	// Marshal without checksum first
	stateCopy.Checksum = ""
	dataWithoutChecksum, err := json.Marshal(stateCopy)
	if err != nil {
		sc.recordMetric(func(m *metrics.Metrics) { m.RecordCheckpointFailed() })
		return fmt.Errorf("failed to marshal checkpoint state: %w", err)
	}

	// Calculate and set checksum
	stateCopy.Checksum = calculateChecksum(dataWithoutChecksum)

	// Marshal with checksum
	data, err := json.MarshalIndent(stateCopy, "", "  ")
	if err != nil {
		sc.recordMetric(func(m *metrics.Metrics) { m.RecordCheckpointFailed() })
		return fmt.Errorf("failed to marshal checkpoint state with checksum: %w", err)
	}

	// Write to temporary file first, then rename for atomic update
	tmpFile := sc.config.FilePath + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0o600); err != nil {
		sc.recordMetric(func(m *metrics.Metrics) { m.RecordCheckpointFailed() })
		return fmt.Errorf("failed to write checkpoint state: %w", err)
	}

	if err := os.Rename(tmpFile, sc.config.FilePath); err != nil {
		os.Remove(tmpFile) // Clean up temp file on rename failure
		sc.recordMetric(func(m *metrics.Metrics) { m.RecordCheckpointFailed() })
		return fmt.Errorf("failed to rename checkpoint state file: %w", err)
	}

	sc.recordMetric(func(m *metrics.Metrics) { m.RecordCheckpointSaved() })
	sc.logger.Debug("Saved checkpoint state",
		"bootstrap_phase", stateCopy.Bootstrap.Phase,
		"circuit_builds", stateCopy.Circuits.TotalBuilds)
	return nil
}

// recordMetric safely calls a metric recording function if metrics are available.
func (sc *StateCheckpointer) recordMetric(fn func(*metrics.Metrics)) {
	if sc.metrics != nil {
		fn(sc.metrics)
	}
}

// Load loads the checkpoint state from disk.
// Note: The file lock is held during backup recovery to prevent concurrent
// writes to the checkpoint file while we're reading backup files.
func (sc *StateCheckpointer) Load(ctx context.Context) (*CheckpointState, error) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	// Acquire file lock for reading
	if err := sc.acquireLock(ctx); err != nil {
		return nil, err
	}
	defer func() {
		if err := sc.releaseLock(); err != nil {
			sc.logger.Warn("Failed to release checkpoint file lock", "error", err)
		}
	}()

	data, err := os.ReadFile(sc.config.FilePath)
	if err != nil {
		return nil, err
	}

	var state CheckpointState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse checkpoint state: %w", err)
	}

	// Verify checksum
	if !verifyChecksum(&state) {
		sc.logger.Warn("Checkpoint checksum mismatch, attempting backup recovery")
		return sc.loadFromBackup()
	}

	// Update internal state
	sc.stateMu.Lock()
	sc.state = &state
	sc.stateMu.Unlock()

	sc.recordMetric(func(m *metrics.Metrics) { m.RecordCheckpointLoaded() })
	sc.logger.Info("Loaded checkpoint state",
		"version", state.Version,
		"bootstrap_phase", state.Bootstrap.Phase,
		"updated_at", state.UpdatedAt)

	return &state, nil
}

// loadFromBackup attempts to load checkpoint state from backup files.
// This is called while holding the checkpoint file lock to prevent concurrent writes.
func (sc *StateCheckpointer) loadFromBackup() (*CheckpointState, error) {
	for i := 1; i <= sc.config.BackupCount; i++ {
		backupPath := fmt.Sprintf("%s.backup.%d", sc.config.FilePath, i)

		data, err := os.ReadFile(backupPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			sc.logger.Warn("Failed to read checkpoint backup", "path", backupPath, "error", err)
			continue
		}

		var state CheckpointState
		if err := json.Unmarshal(data, &state); err != nil {
			sc.logger.Warn("Failed to parse checkpoint backup", "path", backupPath, "error", err)
			continue
		}

		if !verifyChecksum(&state) {
			sc.logger.Warn("Checkpoint backup checksum mismatch", "path", backupPath)
			continue
		}

		// Update internal state
		sc.stateMu.Lock()
		sc.state = &state
		sc.stateMu.Unlock()

		sc.recordMetric(func(m *metrics.Metrics) { m.RecordCheckpointRecovery() })
		sc.logger.Info("Recovered checkpoint state from backup",
			"path", backupPath,
			"bootstrap_phase", state.Bootstrap.Phase)
		return &state, nil
	}

	return nil, fmt.Errorf("no valid checkpoint backup found")
}

// GetState returns a copy of the current checkpoint state.
func (sc *StateCheckpointer) GetState() CheckpointState {
	sc.stateMu.RLock()
	defer sc.stateMu.RUnlock()
	return *sc.state
}

// UpdateBootstrap updates the bootstrap state.
func (sc *StateCheckpointer) UpdateBootstrap(phase int, status string) {
	sc.stateMu.Lock()
	defer sc.stateMu.Unlock()

	sc.state.Bootstrap.Phase = phase
	sc.state.Bootstrap.Status = status

	if sc.state.Bootstrap.StartedAt.IsZero() {
		sc.state.Bootstrap.StartedAt = time.Now()
	}

	if phase >= 100 && sc.state.Bootstrap.CompletedAt.IsZero() {
		sc.state.Bootstrap.CompletedAt = time.Now()
	}
}

// UpdateConsensusTime records when the consensus was last updated.
func (sc *StateCheckpointer) UpdateConsensusTime(t time.Time) {
	sc.stateMu.Lock()
	defer sc.stateMu.Unlock()
	sc.state.Bootstrap.LastConsensusUpdate = t
}

// RecordBandwidth records bandwidth usage.
func (sc *StateCheckpointer) RecordBandwidth(bytesRead, bytesWritten uint64) {
	sc.stateMu.Lock()
	defer sc.stateMu.Unlock()

	sc.state.Bandwidth.SessionBytesRead += bytesRead
	sc.state.Bandwidth.SessionBytesWritten += bytesWritten
	sc.state.Bandwidth.TotalBytesRead += bytesRead
	sc.state.Bandwidth.TotalBytesWritten += bytesWritten
}

// RecordCircuitBuild records a circuit build attempt.
func (sc *StateCheckpointer) RecordCircuitBuild(success bool, buildTimeMs int64) {
	sc.stateMu.Lock()
	defer sc.stateMu.Unlock()

	sc.state.Circuits.TotalBuilds++
	if success {
		sc.state.Circuits.TotalSuccesses++
		sc.state.Circuits.LastBuildTime = time.Now()

		// Update running average of build time using exponential moving average
		// EMA formula: new_avg = old_avg * (1-alpha) + new_value * alpha
		if sc.state.Circuits.TotalSuccesses == 1 {
			sc.state.Circuits.AverageBuildTimeMs = buildTimeMs
		} else {
			oldAvg := float64(sc.state.Circuits.AverageBuildTimeMs)
			newVal := float64(buildTimeMs)
			newAvg := (1-emaAlpha)*oldAvg + emaAlpha*newVal
			sc.state.Circuits.AverageBuildTimeMs = int64(newAvg)
		}
	} else {
		sc.state.Circuits.TotalFailures++
	}
}

// RestoreFromCheckpoint restores state from a loaded checkpoint.
// This is called during startup to apply recovered state.
func (sc *StateCheckpointer) RestoreFromCheckpoint(state *CheckpointState) {
	if state == nil {
		return
	}

	sc.stateMu.Lock()
	defer sc.stateMu.Unlock()

	// Restore bandwidth totals (session counters reset)
	sc.state.Bandwidth.TotalBytesRead = state.Bandwidth.TotalBytesRead
	sc.state.Bandwidth.TotalBytesWritten = state.Bandwidth.TotalBytesWritten
	sc.state.Bandwidth.SessionBytesRead = 0
	sc.state.Bandwidth.SessionBytesWritten = 0

	// Restore circuit statistics
	sc.state.Circuits = state.Circuits

	// Don't restore bootstrap state - client should re-bootstrap
	// but log the previous state for debugging
	sc.logger.Info("Restored checkpoint state",
		"previous_bootstrap_phase", state.Bootstrap.Phase,
		"total_bytes_read", state.Bandwidth.TotalBytesRead,
		"total_circuit_builds", state.Circuits.TotalBuilds)
}

// StartCheckpointLoop starts automatic periodic checkpointing.
// This method is safe to call concurrently; duplicate calls are ignored.
func (sc *StateCheckpointer) StartCheckpointLoop(ctx context.Context) {
	if sc.config.CheckpointInterval <= 0 {
		return
	}

	sc.loopMu.Lock()
	defer sc.loopMu.Unlock()

	if sc.loopRunning {
		sc.logger.Debug("Checkpoint loop already running, ignoring duplicate start")
		return
	}

	loopCtx, cancel := context.WithCancel(ctx)
	sc.loopCancel = cancel
	sc.loopRunning = true

	sc.loopWg.Add(1)
	go func() {
		defer sc.loopWg.Done()
		sc.checkpointLoop(loopCtx)
	}()

	sc.logger.Info("Started checkpoint loop", "interval", sc.config.CheckpointInterval)
}

// StopCheckpointLoop stops the automatic checkpointing loop.
// This method is safe to call concurrently and multiple times.
func (sc *StateCheckpointer) StopCheckpointLoop() {
	sc.loopMu.Lock()
	if !sc.loopRunning {
		sc.loopMu.Unlock()
		return
	}
	cancel := sc.loopCancel
	sc.loopRunning = false
	sc.loopMu.Unlock()

	if cancel != nil {
		cancel()
		sc.loopWg.Wait()
		sc.logger.Info("Stopped checkpoint loop")
	}
}

// checkpointLoop periodically saves checkpoint state.
func (sc *StateCheckpointer) checkpointLoop(ctx context.Context) {
	ticker := time.NewTicker(sc.config.CheckpointInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Final checkpoint before exit
			saveCtx, cancel := context.WithTimeout(context.Background(), defaultFinalCheckpointTimeout)
			if err := sc.Save(saveCtx); err != nil {
				sc.logger.Warn("Failed to save final checkpoint", "error", err)
			}
			cancel()
			return
		case <-ticker.C:
			if err := sc.Save(ctx); err != nil {
				if ctx.Err() != nil {
					return
				}
				sc.logger.Error("Failed to save checkpoint", "error", err)
			}
		}
	}
}

// FileExists returns true if the checkpoint file exists.
func (sc *StateCheckpointer) FileExists() bool {
	_, err := os.Stat(sc.config.FilePath)
	return err == nil
}

// GetBackupPaths returns the paths to all backup files that exist.
func (sc *StateCheckpointer) GetBackupPaths() []string {
	var paths []string
	for i := 1; i <= sc.config.BackupCount; i++ {
		backupPath := fmt.Sprintf("%s.backup.%d", sc.config.FilePath, i)
		if _, err := os.Stat(backupPath); err == nil {
			paths = append(paths, backupPath)
		}
	}
	return paths
}

// IsLoopRunning returns true if the checkpoint loop is currently running.
func (sc *StateCheckpointer) IsLoopRunning() bool {
	sc.loopMu.Lock()
	defer sc.loopMu.Unlock()
	return sc.loopRunning
}
