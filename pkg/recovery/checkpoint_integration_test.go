package recovery

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/logger"
)

// TestIntegrationFullSaveLoadRoundtrip tests a complete save/load roundtrip
// with all fields populated and verified.
func TestIntegrationFullSaveLoadRoundtrip(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "checkpoint.json")

	config := DefaultCheckpointConfig(filePath)
	sc := NewStateCheckpointer(config, logger.NewDefault())

	ctx := context.Background()

	// Populate all state fields
	sc.UpdateBootstrap(65, "loading_keys")
	sc.RecordBandwidth(2048, 1024)
	sc.RecordBandwidth(4096, 2048)
	sc.RecordCircuitBuild(true, 1200)
	sc.RecordCircuitBuild(true, 1800)
	sc.RecordCircuitBuild(false, 0)
	consensusTime := time.Now().Add(-5 * time.Minute).Truncate(time.Millisecond)
	sc.UpdateConsensusTime(consensusTime)

	originalState := sc.GetState()

	// Save
	if err := sc.Save(ctx); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Create a completely new checkpointer and load
	sc2 := NewStateCheckpointer(config, logger.NewDefault())
	loaded, err := sc2.Load(ctx)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Verify all fields match
	if loaded.Version != currentSchemaVersion {
		t.Errorf("Version = %d, want %d", loaded.Version, currentSchemaVersion)
	}
	if loaded.Bootstrap.Phase != originalState.Bootstrap.Phase {
		t.Errorf("Bootstrap.Phase = %d, want %d", loaded.Bootstrap.Phase, originalState.Bootstrap.Phase)
	}
	if loaded.Bootstrap.Status != originalState.Bootstrap.Status {
		t.Errorf("Bootstrap.Status = %q, want %q", loaded.Bootstrap.Status, originalState.Bootstrap.Status)
	}
	if loaded.Bandwidth.TotalBytesRead != originalState.Bandwidth.TotalBytesRead {
		t.Errorf("TotalBytesRead = %d, want %d", loaded.Bandwidth.TotalBytesRead, originalState.Bandwidth.TotalBytesRead)
	}
	if loaded.Bandwidth.TotalBytesWritten != originalState.Bandwidth.TotalBytesWritten {
		t.Errorf("TotalBytesWritten = %d, want %d", loaded.Bandwidth.TotalBytesWritten, originalState.Bandwidth.TotalBytesWritten)
	}
	if loaded.Bandwidth.SessionBytesRead != originalState.Bandwidth.SessionBytesRead {
		t.Errorf("SessionBytesRead = %d, want %d", loaded.Bandwidth.SessionBytesRead, originalState.Bandwidth.SessionBytesRead)
	}
	if loaded.Bandwidth.SessionBytesWritten != originalState.Bandwidth.SessionBytesWritten {
		t.Errorf("SessionBytesWritten = %d, want %d", loaded.Bandwidth.SessionBytesWritten, originalState.Bandwidth.SessionBytesWritten)
	}
	if loaded.Circuits.TotalBuilds != originalState.Circuits.TotalBuilds {
		t.Errorf("TotalBuilds = %d, want %d", loaded.Circuits.TotalBuilds, originalState.Circuits.TotalBuilds)
	}
	if loaded.Circuits.TotalSuccesses != originalState.Circuits.TotalSuccesses {
		t.Errorf("TotalSuccesses = %d, want %d", loaded.Circuits.TotalSuccesses, originalState.Circuits.TotalSuccesses)
	}
	if loaded.Circuits.TotalFailures != originalState.Circuits.TotalFailures {
		t.Errorf("TotalFailures = %d, want %d", loaded.Circuits.TotalFailures, originalState.Circuits.TotalFailures)
	}
	if loaded.Circuits.AverageBuildTimeMs != originalState.Circuits.AverageBuildTimeMs {
		t.Errorf("AverageBuildTimeMs = %d, want %d", loaded.Circuits.AverageBuildTimeMs, originalState.Circuits.AverageBuildTimeMs)
	}
	if !loaded.Bootstrap.LastConsensusUpdate.Equal(consensusTime) {
		t.Errorf("LastConsensusUpdate = %v, want %v", loaded.Bootstrap.LastConsensusUpdate, consensusTime)
	}
	if loaded.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set after save")
	}
	if loaded.Checksum == "" {
		t.Error("Checksum should be set after save")
	}
}

// TestIntegrationChecksumIntegrityVerification saves state, corrupts the file,
// and verifies load fails and attempts backup recovery.
func TestIntegrationChecksumIntegrityVerification(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "checkpoint.json")

	config := &CheckpointConfig{
		FilePath:           filePath,
		BackupCount:        0, // No backups, so recovery should fail
		LockTimeout:        10 * time.Second,
		CheckpointInterval: 0,
	}
	sc := NewStateCheckpointer(config, logger.NewDefault())
	ctx := context.Background()

	sc.UpdateBootstrap(50, "testing_integrity")
	if err := sc.Save(ctx); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Corrupt the file by modifying content
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile() failed: %v", err)
	}
	// Flip some bytes in the middle
	corrupted := make([]byte, len(data))
	copy(corrupted, data)
	corrupted[len(corrupted)/2] = corrupted[len(corrupted)/2] ^ 0xFF
	if err := os.WriteFile(filePath, corrupted, 0o600); err != nil {
		t.Fatalf("WriteFile() failed: %v", err)
	}

	// Load should fail because checksum won't match and no backups exist
	sc2 := NewStateCheckpointer(config, logger.NewDefault())
	_, err = sc2.Load(ctx)
	if err == nil {
		t.Fatal("Load() should fail with corrupted file and no backups")
	}
}

// TestIntegrationBackupRotation verifies backups are correctly rotated
// and old backups beyond the configured count are cleaned up.
func TestIntegrationBackupRotation(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "checkpoint.json")

	config := &CheckpointConfig{
		FilePath:           filePath,
		BackupCount:        3,
		LockTimeout:        10 * time.Second,
		CheckpointInterval: 0,
	}
	sc := NewStateCheckpointer(config, logger.NewDefault())
	ctx := context.Background()

	// Save 6 times with distinct states
	for i := 0; i < 6; i++ {
		sc.UpdateBootstrap(i*10, "rotation_test")
		if err := sc.Save(ctx); err != nil {
			t.Fatalf("Save() iteration %d failed: %v", i, err)
		}
	}

	// Verify exactly BackupCount backup files exist
	backups := sc.GetBackupPaths()
	if len(backups) != config.BackupCount {
		t.Errorf("Got %d backups, want %d", len(backups), config.BackupCount)
	}

	// Verify backup.1 through backup.3 exist
	for i := 1; i <= config.BackupCount; i++ {
		path := filePath + ".backup." + itoa(i)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Expected backup.%d to exist", i)
		}
	}

	// Verify no backup beyond count exists
	extraPath := filePath + ".backup." + itoa(config.BackupCount+1)
	if _, err := os.Stat(extraPath); !os.IsNotExist(err) {
		t.Errorf("Backup beyond count should not exist: %s", extraPath)
	}

	// Verify backup.1 is the most recent backup (contains phase from save N-1)
	data, err := os.ReadFile(filePath + ".backup.1")
	if err != nil {
		t.Fatalf("ReadFile(backup.1) failed: %v", err)
	}
	var state CheckpointState
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("Unmarshal(backup.1) failed: %v", err)
	}
	// backup.1 should have the state from the save before the last one (save index 4 = phase 40)
	if state.Bootstrap.Phase != 40 {
		t.Errorf("backup.1 Bootstrap.Phase = %d, want 40", state.Bootstrap.Phase)
	}
}

// TestIntegrationRecoveryFromBackup saves twice (creating a backup), corrupts
// the primary file, and verifies load recovers from the backup.
func TestIntegrationRecoveryFromBackup(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "checkpoint.json")

	config := &CheckpointConfig{
		FilePath:           filePath,
		BackupCount:        2,
		LockTimeout:        10 * time.Second,
		CheckpointInterval: 0,
	}
	sc := NewStateCheckpointer(config, logger.NewDefault())
	ctx := context.Background()

	// First save with known state
	sc.UpdateBootstrap(30, "first_save")
	sc.RecordBandwidth(500, 250)
	if err := sc.Save(ctx); err != nil {
		t.Fatalf("First Save() failed: %v", err)
	}

	// Second save with updated state (creates backup.1 of first save)
	sc.UpdateBootstrap(60, "second_save")
	sc.RecordBandwidth(500, 250)
	if err := sc.Save(ctx); err != nil {
		t.Fatalf("Second Save() failed: %v", err)
	}

	// Corrupt the primary file
	if err := os.WriteFile(filePath, []byte(`{"corrupted": true}`), 0o600); err != nil {
		t.Fatalf("WriteFile() failed: %v", err)
	}

	// Load should recover from backup.1
	sc2 := NewStateCheckpointer(config, logger.NewDefault())
	loaded, err := sc2.Load(ctx)
	if err != nil {
		t.Fatalf("Load() should recover from backup, got: %v", err)
	}

	// Backup.1 has the state from the first save
	if loaded.Bootstrap.Phase != 30 {
		t.Errorf("Recovered Bootstrap.Phase = %d, want 30", loaded.Bootstrap.Phase)
	}
	if loaded.Bandwidth.TotalBytesRead != 500 {
		t.Errorf("Recovered TotalBytesRead = %d, want 500", loaded.Bandwidth.TotalBytesRead)
	}
}

// TestIntegrationConcurrentStateUpdatesDuringSave tests that multiple goroutines
// updating bandwidth/circuits while save is in progress does not race.
func TestIntegrationConcurrentStateUpdatesDuringSave(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "checkpoint.json")

	config := DefaultCheckpointConfig(filePath)
	sc := NewStateCheckpointer(config, logger.NewDefault())
	ctx := context.Background()

	var wg sync.WaitGroup
	const goroutines = 20

	// Start concurrent writers
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				sc.RecordBandwidth(uint64(n+1), uint64(n+1))
				sc.RecordCircuitBuild(j%2 == 0, int64(100+n))
			}
		}(i)
	}

	// Concurrent saves
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = sc.Save(ctx)
		}()
	}

	wg.Wait()

	// Final save should succeed
	if err := sc.Save(ctx); err != nil {
		t.Fatalf("Final Save() failed: %v", err)
	}

	// Verify state is internally consistent
	state := sc.GetState()
	if state.Circuits.TotalBuilds != int64(goroutines*50) {
		t.Errorf("TotalBuilds = %d, want %d", state.Circuits.TotalBuilds, goroutines*50)
	}
	expectedSuccesses := int64(goroutines * 25)
	if state.Circuits.TotalSuccesses != expectedSuccesses {
		t.Errorf("TotalSuccesses = %d, want %d", state.Circuits.TotalSuccesses, expectedSuccesses)
	}
	expectedFailures := int64(goroutines * 25)
	if state.Circuits.TotalFailures != expectedFailures {
		t.Errorf("TotalFailures = %d, want %d", state.Circuits.TotalFailures, expectedFailures)
	}
}

// TestIntegrationCheckpointLoopStartStop starts the loop, waits for at least one
// checkpoint, stops the loop, and verifies the file exists.
func TestIntegrationCheckpointLoopStartStop(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "checkpoint.json")

	config := &CheckpointConfig{
		FilePath:           filePath,
		CheckpointInterval: 50 * time.Millisecond,
		LockTimeout:        10 * time.Second,
		BackupCount:        2,
	}
	sc := NewStateCheckpointer(config, logger.NewDefault())

	sc.UpdateBootstrap(42, "loop_test")

	ctx := context.Background()
	sc.StartCheckpointLoop(ctx)

	if !sc.IsLoopRunning() {
		t.Fatal("Loop should be running after StartCheckpointLoop")
	}

	// Wait for at least one checkpoint to fire
	time.Sleep(200 * time.Millisecond)

	sc.StopCheckpointLoop()

	if sc.IsLoopRunning() {
		t.Error("Loop should not be running after StopCheckpointLoop")
	}

	// Verify file was created
	if !sc.FileExists() {
		t.Error("Checkpoint file should exist after loop ran")
	}

	// Verify the content is valid
	sc2 := NewStateCheckpointer(config, logger.NewDefault())
	loaded, err := sc2.Load(ctx)
	if err != nil {
		t.Fatalf("Load() after loop: %v", err)
	}
	if loaded.Bootstrap.Phase != 42 {
		t.Errorf("Bootstrap.Phase = %d, want 42", loaded.Bootstrap.Phase)
	}
}

// TestIntegrationCheckpointLoopIdempotentStart calls StartCheckpointLoop twice
// and verifies no double-start occurs.
func TestIntegrationCheckpointLoopIdempotentStart(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "checkpoint.json")

	config := &CheckpointConfig{
		FilePath:           filePath,
		CheckpointInterval: 50 * time.Millisecond,
		LockTimeout:        10 * time.Second,
		BackupCount:        1,
	}
	sc := NewStateCheckpointer(config, logger.NewDefault())
	ctx := context.Background()

	// Start twice - should not create two goroutines
	sc.StartCheckpointLoop(ctx)
	sc.StartCheckpointLoop(ctx) // duplicate, should be ignored

	if !sc.IsLoopRunning() {
		t.Fatal("Loop should be running")
	}

	time.Sleep(150 * time.Millisecond)

	// Stop once should be sufficient
	sc.StopCheckpointLoop()

	if sc.IsLoopRunning() {
		t.Error("Loop should stop after single StopCheckpointLoop call")
	}
}

// TestIntegrationStopCheckpointLoopBeforeStart calls StopCheckpointLoop
// without a prior StartCheckpointLoop and verifies it does not panic.
func TestIntegrationStopCheckpointLoopBeforeStart(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "checkpoint.json")

	config := DefaultCheckpointConfig(filePath)
	sc := NewStateCheckpointer(config, logger.NewDefault())

	// Should not panic
	sc.StopCheckpointLoop()
	sc.StopCheckpointLoop() // Call again for good measure

	if sc.IsLoopRunning() {
		t.Error("Loop should not be running")
	}
}

// TestIntegrationRestoreFromCheckpointNil verifies RestoreFromCheckpoint
// with nil argument does not panic and preserves existing state.
func TestIntegrationRestoreFromCheckpointNil(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "checkpoint.json")

	config := DefaultCheckpointConfig(filePath)
	sc := NewStateCheckpointer(config, logger.NewDefault())

	// Set some state first
	sc.RecordBandwidth(100, 50)

	// Restore nil should not panic or change state
	sc.RestoreFromCheckpoint(nil)

	state := sc.GetState()
	if state.Bandwidth.TotalBytesRead != 100 {
		t.Errorf("TotalBytesRead = %d, want 100 (unchanged)", state.Bandwidth.TotalBytesRead)
	}
	if state.Bandwidth.SessionBytesRead != 100 {
		t.Errorf("SessionBytesRead = %d, want 100 (unchanged)", state.Bandwidth.SessionBytesRead)
	}
}

// TestIntegrationBootstrapPhaseTransitions tests progressive bootstrap
// updates from 0→25→50→75→100 and verifies CompletedAt is set at 100.
func TestIntegrationBootstrapPhaseTransitions(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "checkpoint.json")

	config := DefaultCheckpointConfig(filePath)
	sc := NewStateCheckpointer(config, logger.NewDefault())

	phases := []struct {
		phase  int
		status string
	}{
		{0, "idle"},
		{25, "connecting"},
		{50, "handshaking"},
		{75, "loading_descriptors"},
		{100, "done"},
	}

	for _, p := range phases {
		sc.UpdateBootstrap(p.phase, p.status)
		state := sc.GetState()

		if state.Bootstrap.Phase != p.phase {
			t.Errorf("After UpdateBootstrap(%d): Phase = %d", p.phase, state.Bootstrap.Phase)
		}
		if state.Bootstrap.Status != p.status {
			t.Errorf("After UpdateBootstrap(%d): Status = %q, want %q", p.phase, state.Bootstrap.Status, p.status)
		}

		if p.phase > 0 && state.Bootstrap.StartedAt.IsZero() {
			t.Errorf("After UpdateBootstrap(%d): StartedAt should be set", p.phase)
		}

		if p.phase < 100 && !state.Bootstrap.CompletedAt.IsZero() {
			t.Errorf("After UpdateBootstrap(%d): CompletedAt should be zero before 100", p.phase)
		}
	}

	// After phase 100
	finalState := sc.GetState()
	if finalState.Bootstrap.CompletedAt.IsZero() {
		t.Error("CompletedAt should be set after phase 100")
	}
	if finalState.Bootstrap.StartedAt.IsZero() {
		t.Error("StartedAt should be set")
	}
	if finalState.Bootstrap.CompletedAt.Before(finalState.Bootstrap.StartedAt) {
		t.Error("CompletedAt should not be before StartedAt")
	}
}

// TestIntegrationBandwidthAccumulationAcrossSaveRestore tests that bandwidth
// totals are cumulative across save/restore cycles.
func TestIntegrationBandwidthAccumulationAcrossSaveRestore(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "checkpoint.json")

	config := DefaultCheckpointConfig(filePath)
	ctx := context.Background()

	// First session: record bandwidth and save
	sc1 := NewStateCheckpointer(config, logger.NewDefault())
	sc1.RecordBandwidth(1000, 500)
	sc1.RecordBandwidth(2000, 1000)
	if err := sc1.Save(ctx); err != nil {
		t.Fatalf("First Save() failed: %v", err)
	}

	// Second session: load, restore, record more bandwidth
	sc2 := NewStateCheckpointer(config, logger.NewDefault())
	loaded, err := sc2.Load(ctx)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	sc2.RestoreFromCheckpoint(loaded)

	// After restore, session counters should be zero, totals preserved
	state := sc2.GetState()
	if state.Bandwidth.TotalBytesRead != 3000 {
		t.Errorf("After restore: TotalBytesRead = %d, want 3000", state.Bandwidth.TotalBytesRead)
	}
	if state.Bandwidth.SessionBytesRead != 0 {
		t.Errorf("After restore: SessionBytesRead = %d, want 0", state.Bandwidth.SessionBytesRead)
	}

	// Record more bandwidth in second session
	sc2.RecordBandwidth(500, 250)

	state = sc2.GetState()
	if state.Bandwidth.TotalBytesRead != 3500 {
		t.Errorf("After more bandwidth: TotalBytesRead = %d, want 3500", state.Bandwidth.TotalBytesRead)
	}
	if state.Bandwidth.SessionBytesRead != 500 {
		t.Errorf("After more bandwidth: SessionBytesRead = %d, want 500", state.Bandwidth.SessionBytesRead)
	}
	if state.Bandwidth.TotalBytesWritten != 1750 {
		t.Errorf("After more bandwidth: TotalBytesWritten = %d, want 1750", state.Bandwidth.TotalBytesWritten)
	}
	if state.Bandwidth.SessionBytesWritten != 250 {
		t.Errorf("After more bandwidth: SessionBytesWritten = %d, want 250", state.Bandwidth.SessionBytesWritten)
	}
}

// TestIntegrationCircuitBuildEMAAccuracy tests the exponential moving average
// calculation for circuit build times with known values.
func TestIntegrationCircuitBuildEMAAccuracy(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "checkpoint.json")

	config := DefaultCheckpointConfig(filePath)
	sc := NewStateCheckpointer(config, logger.NewDefault())

	// First successful build: EMA = 1000 (first value is used directly)
	sc.RecordCircuitBuild(true, 1000)
	state := sc.GetState()
	if state.Circuits.AverageBuildTimeMs != 1000 {
		t.Errorf("After 1st build: AverageBuildTimeMs = %d, want 1000", state.Circuits.AverageBuildTimeMs)
	}

	// Second successful build: EMA = 1000 * 0.9 + 2000 * 0.1 = 900 + 200 = 1100
	sc.RecordCircuitBuild(true, 2000)
	state = sc.GetState()
	expected := int64((1 - emaAlpha) * 1000 + emaAlpha*2000)
	if state.Circuits.AverageBuildTimeMs != expected {
		t.Errorf("After 2nd build: AverageBuildTimeMs = %d, want %d", state.Circuits.AverageBuildTimeMs, expected)
	}

	// Third build: EMA = 1100 * 0.9 + 1500 * 0.1 = 990 + 150 = 1140
	sc.RecordCircuitBuild(true, 1500)
	state = sc.GetState()
	expected3 := int64((1-emaAlpha)*float64(expected) + emaAlpha*1500)
	if state.Circuits.AverageBuildTimeMs != expected3 {
		t.Errorf("After 3rd build: AverageBuildTimeMs = %d, want %d", state.Circuits.AverageBuildTimeMs, expected3)
	}

	// Failed builds should not affect EMA
	prevEMA := state.Circuits.AverageBuildTimeMs
	sc.RecordCircuitBuild(false, 5000)
	state = sc.GetState()
	if state.Circuits.AverageBuildTimeMs != prevEMA {
		t.Errorf("After failed build: AverageBuildTimeMs = %d, want %d (unchanged)", state.Circuits.AverageBuildTimeMs, prevEMA)
	}

	// Verify counts
	if state.Circuits.TotalBuilds != 4 {
		t.Errorf("TotalBuilds = %d, want 4", state.Circuits.TotalBuilds)
	}
	if state.Circuits.TotalSuccesses != 3 {
		t.Errorf("TotalSuccesses = %d, want 3", state.Circuits.TotalSuccesses)
	}
	if state.Circuits.TotalFailures != 1 {
		t.Errorf("TotalFailures = %d, want 1", state.Circuits.TotalFailures)
	}

	// Test EMA convergence with many builds of same value
	for i := 0; i < 100; i++ {
		sc.RecordCircuitBuild(true, 500)
	}
	state = sc.GetState()
	// After 100 builds at 500ms, EMA should converge close to 500
	if math.Abs(float64(state.Circuits.AverageBuildTimeMs)-500) > 10 {
		t.Errorf("After convergence: AverageBuildTimeMs = %d, want ~500", state.Circuits.AverageBuildTimeMs)
	}
}

// TestIntegrationFileExistsAndGetBackupPaths verifies these utilities
// work correctly before and after saves.
func TestIntegrationFileExistsAndGetBackupPaths(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "checkpoint.json")

	config := &CheckpointConfig{
		FilePath:           filePath,
		BackupCount:        2,
		LockTimeout:        10 * time.Second,
		CheckpointInterval: 0,
	}
	sc := NewStateCheckpointer(config, logger.NewDefault())
	ctx := context.Background()

	// Before any save
	if sc.FileExists() {
		t.Error("FileExists() should be false before first save")
	}
	if paths := sc.GetBackupPaths(); len(paths) != 0 {
		t.Errorf("GetBackupPaths() = %v, want empty before any save", paths)
	}

	// After first save
	if err := sc.Save(ctx); err != nil {
		t.Fatalf("First Save() failed: %v", err)
	}
	if !sc.FileExists() {
		t.Error("FileExists() should be true after first save")
	}
	if paths := sc.GetBackupPaths(); len(paths) != 0 {
		t.Errorf("GetBackupPaths() should be empty after first save, got %v", paths)
	}

	// After second save (backup.1 should exist)
	sc.UpdateBootstrap(10, "second")
	if err := sc.Save(ctx); err != nil {
		t.Fatalf("Second Save() failed: %v", err)
	}
	if paths := sc.GetBackupPaths(); len(paths) != 1 {
		t.Errorf("GetBackupPaths() len = %d, want 1 after second save", len(paths))
	}

	// After third save (backup.1 and backup.2 should exist)
	sc.UpdateBootstrap(20, "third")
	if err := sc.Save(ctx); err != nil {
		t.Fatalf("Third Save() failed: %v", err)
	}
	if paths := sc.GetBackupPaths(); len(paths) != 2 {
		t.Errorf("GetBackupPaths() len = %d, want 2 after third save", len(paths))
	}

	// Fourth save should still have only 2 backups (due to rotation)
	sc.UpdateBootstrap(30, "fourth")
	if err := sc.Save(ctx); err != nil {
		t.Fatalf("Fourth Save() failed: %v", err)
	}
	if paths := sc.GetBackupPaths(); len(paths) != 2 {
		t.Errorf("GetBackupPaths() len = %d, want 2 after fourth save", len(paths))
	}
}

// TestIntegrationDefaultCheckpointConfigValues verifies that default
// configuration values are sensible.
func TestIntegrationDefaultCheckpointConfigValues(t *testing.T) {
	config := DefaultCheckpointConfig("test.json")

	if config.FilePath != "test.json" {
		t.Errorf("FilePath = %q, want %q", config.FilePath, "test.json")
	}
	if config.CheckpointInterval <= 0 {
		t.Errorf("CheckpointInterval = %v, should be positive", config.CheckpointInterval)
	}
	if config.CheckpointInterval > 10*time.Minute {
		t.Errorf("CheckpointInterval = %v, should not be excessively large", config.CheckpointInterval)
	}
	if config.LockTimeout <= 0 {
		t.Errorf("LockTimeout = %v, should be positive", config.LockTimeout)
	}
	if config.LockTimeout > 1*time.Minute {
		t.Errorf("LockTimeout = %v, should not be excessively large", config.LockTimeout)
	}
	if config.BackupCount < 1 {
		t.Errorf("BackupCount = %d, should be at least 1", config.BackupCount)
	}
	if config.BackupCount > 10 {
		t.Errorf("BackupCount = %d, should not be excessively large", config.BackupCount)
	}
}

// TestIntegrationSaveWithCancelledContext verifies that Save fails gracefully
// when the context is already cancelled.
func TestIntegrationSaveWithCancelledContext(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "checkpoint.json")

	config := &CheckpointConfig{
		FilePath:           filePath,
		BackupCount:        1,
		LockTimeout:        500 * time.Millisecond,
		CheckpointInterval: 0,
	}
	sc := NewStateCheckpointer(config, logger.NewDefault())

	// Create an already-cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := sc.Save(ctx)
	if err == nil {
		t.Error("Save() with cancelled context should fail")
	}

	// File should not exist (save should have failed before writing)
	if sc.FileExists() {
		t.Error("File should not exist after failed save")
	}
}

// itoa is a simple int to string helper for constructing paths.
func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}
