package recovery

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/logger"
)

func TestNewStateCheckpointer(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "checkpoint.json")

	config := DefaultCheckpointConfig(filePath)
	sc := NewStateCheckpointer(config, logger.NewDefault())

	if sc == nil {
		t.Fatal("NewStateCheckpointer() returned nil")
	}
	if sc.config.FilePath != filePath {
		t.Errorf("FilePath = %s, want %s", sc.config.FilePath, filePath)
	}
}

func TestDefaultCheckpointConfig(t *testing.T) {
	tmpDir := t.TempDir()
	testPath := filepath.Join(tmpDir, "test.json")
	config := DefaultCheckpointConfig(testPath)

	if config.FilePath != testPath {
		t.Errorf("FilePath = %s, want %s", config.FilePath, testPath)
	}
	if config.CheckpointInterval != 1*time.Minute {
		t.Errorf("CheckpointInterval = %v, want 1m", config.CheckpointInterval)
	}
	if config.LockTimeout != 10*time.Second {
		t.Errorf("LockTimeout = %v, want 10s", config.LockTimeout)
	}
	if config.BackupCount != 2 {
		t.Errorf("BackupCount = %d, want 2", config.BackupCount)
	}
}

func TestStateCheckpointerSaveLoad(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "checkpoint.json")

	config := DefaultCheckpointConfig(filePath)
	sc := NewStateCheckpointer(config, logger.NewDefault())

	ctx := context.Background()

	// Update state
	sc.UpdateBootstrap(50, "loading_descriptors")
	sc.RecordBandwidth(1000, 500)
	sc.RecordCircuitBuild(true, 1500)
	sc.RecordCircuitBuild(true, 2000)
	sc.RecordCircuitBuild(false, 0)

	// Save state
	if err := sc.Save(ctx); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Verify file exists
	if !sc.FileExists() {
		t.Fatal("FileExists() returned false after Save()")
	}

	// Create new checkpointer and load
	sc2 := NewStateCheckpointer(config, logger.NewDefault())
	state, err := sc2.Load(ctx)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Verify loaded state
	if state.Bootstrap.Phase != 50 {
		t.Errorf("Bootstrap.Phase = %d, want 50", state.Bootstrap.Phase)
	}
	if state.Bootstrap.Status != "loading_descriptors" {
		t.Errorf("Bootstrap.Status = %s, want loading_descriptors", state.Bootstrap.Status)
	}
	if state.Bandwidth.TotalBytesRead != 1000 {
		t.Errorf("Bandwidth.TotalBytesRead = %d, want 1000", state.Bandwidth.TotalBytesRead)
	}
	if state.Bandwidth.TotalBytesWritten != 500 {
		t.Errorf("Bandwidth.TotalBytesWritten = %d, want 500", state.Bandwidth.TotalBytesWritten)
	}
	if state.Circuits.TotalBuilds != 3 {
		t.Errorf("Circuits.TotalBuilds = %d, want 3", state.Circuits.TotalBuilds)
	}
	if state.Circuits.TotalSuccesses != 2 {
		t.Errorf("Circuits.TotalSuccesses = %d, want 2", state.Circuits.TotalSuccesses)
	}
	if state.Circuits.TotalFailures != 1 {
		t.Errorf("Circuits.TotalFailures = %d, want 1", state.Circuits.TotalFailures)
	}
}

func TestStateCheckpointerBootstrapProgress(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "checkpoint.json")

	config := DefaultCheckpointConfig(filePath)
	sc := NewStateCheckpointer(config, logger.NewDefault())

	// Initially not started
	state := sc.GetState()
	if state.Bootstrap.Phase != 0 {
		t.Errorf("Initial Bootstrap.Phase = %d, want 0", state.Bootstrap.Phase)
	}
	if state.Bootstrap.Status != "not_started" {
		t.Errorf("Initial Bootstrap.Status = %s, want not_started", state.Bootstrap.Status)
	}

	// Update bootstrap progress
	sc.UpdateBootstrap(25, "connecting")
	state = sc.GetState()
	if state.Bootstrap.Phase != 25 {
		t.Errorf("Bootstrap.Phase = %d, want 25", state.Bootstrap.Phase)
	}
	if state.Bootstrap.StartedAt.IsZero() {
		t.Error("StartedAt should be set after first update")
	}
	if !state.Bootstrap.CompletedAt.IsZero() {
		t.Error("CompletedAt should be zero before 100%")
	}

	// Complete bootstrap
	sc.UpdateBootstrap(100, "done")
	state = sc.GetState()
	if state.Bootstrap.Phase != 100 {
		t.Errorf("Bootstrap.Phase = %d, want 100", state.Bootstrap.Phase)
	}
	if state.Bootstrap.CompletedAt.IsZero() {
		t.Error("CompletedAt should be set at 100%")
	}
}

func TestStateCheckpointerConsensusTime(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "checkpoint.json")

	config := DefaultCheckpointConfig(filePath)
	sc := NewStateCheckpointer(config, logger.NewDefault())

	now := time.Now()
	sc.UpdateConsensusTime(now)

	state := sc.GetState()
	if state.Bootstrap.LastConsensusUpdate.IsZero() {
		t.Error("LastConsensusUpdate should be set")
	}
	if !state.Bootstrap.LastConsensusUpdate.Equal(now) {
		t.Errorf("LastConsensusUpdate = %v, want %v", state.Bootstrap.LastConsensusUpdate, now)
	}
}

func TestStateCheckpointerBandwidth(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "checkpoint.json")

	config := DefaultCheckpointConfig(filePath)
	sc := NewStateCheckpointer(config, logger.NewDefault())

	// Record bandwidth
	sc.RecordBandwidth(100, 50)
	sc.RecordBandwidth(200, 100)

	state := sc.GetState()
	if state.Bandwidth.SessionBytesRead != 300 {
		t.Errorf("SessionBytesRead = %d, want 300", state.Bandwidth.SessionBytesRead)
	}
	if state.Bandwidth.SessionBytesWritten != 150 {
		t.Errorf("SessionBytesWritten = %d, want 150", state.Bandwidth.SessionBytesWritten)
	}
	if state.Bandwidth.TotalBytesRead != 300 {
		t.Errorf("TotalBytesRead = %d, want 300", state.Bandwidth.TotalBytesRead)
	}
	if state.Bandwidth.TotalBytesWritten != 150 {
		t.Errorf("TotalBytesWritten = %d, want 150", state.Bandwidth.TotalBytesWritten)
	}
}

func TestStateCheckpointerCircuitBuild(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "checkpoint.json")

	config := DefaultCheckpointConfig(filePath)
	sc := NewStateCheckpointer(config, logger.NewDefault())

	// Record successful builds
	sc.RecordCircuitBuild(true, 1000)
	sc.RecordCircuitBuild(true, 2000)

	state := sc.GetState()
	if state.Circuits.TotalBuilds != 2 {
		t.Errorf("TotalBuilds = %d, want 2", state.Circuits.TotalBuilds)
	}
	if state.Circuits.TotalSuccesses != 2 {
		t.Errorf("TotalSuccesses = %d, want 2", state.Circuits.TotalSuccesses)
	}
	if state.Circuits.TotalFailures != 0 {
		t.Errorf("TotalFailures = %d, want 0", state.Circuits.TotalFailures)
	}
	if state.Circuits.LastBuildTime.IsZero() {
		t.Error("LastBuildTime should be set after successful build")
	}

	// Average should be calculated
	if state.Circuits.AverageBuildTimeMs == 0 {
		t.Error("AverageBuildTimeMs should be non-zero")
	}

	// Record failed build
	sc.RecordCircuitBuild(false, 0)
	state = sc.GetState()
	if state.Circuits.TotalFailures != 1 {
		t.Errorf("TotalFailures = %d, want 1", state.Circuits.TotalFailures)
	}
}

func TestStateCheckpointerBackupRotation(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "checkpoint.json")

	config := &CheckpointConfig{
		FilePath:    filePath,
		BackupCount: 2,
		LockTimeout: 10 * time.Second,
	}
	sc := NewStateCheckpointer(config, logger.NewDefault())

	ctx := context.Background()

	// Save multiple times to create backups
	for i := 0; i < 5; i++ {
		sc.UpdateBootstrap(i*20, "phase")
		if err := sc.Save(ctx); err != nil {
			t.Fatalf("Save() iteration %d failed: %v", i, err)
		}
	}

	// Check backup files exist
	backups := sc.GetBackupPaths()
	if len(backups) > config.BackupCount {
		t.Errorf("Got %d backups, want <= %d", len(backups), config.BackupCount)
	}

	// At least backup.1 should exist
	if _, err := os.Stat(filePath + ".backup.1"); os.IsNotExist(err) {
		t.Error("Expected backup.1 to exist after multiple saves")
	}
}

func TestStateCheckpointerChecksumVerification(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "checkpoint.json")

	config := DefaultCheckpointConfig(filePath)
	sc := NewStateCheckpointer(config, logger.NewDefault())

	ctx := context.Background()

	sc.UpdateBootstrap(50, "testing")

	if err := sc.Save(ctx); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Read and verify file contains valid checksum
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile() failed: %v", err)
	}

	var state CheckpointState
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("Unmarshal() failed: %v", err)
	}

	if state.Version != currentSchemaVersion {
		t.Errorf("Version = %d, want %d", state.Version, currentSchemaVersion)
	}

	if state.Checksum == "" {
		t.Error("Checksum is empty")
	}

	if !verifyChecksum(&state) {
		t.Error("verifyChecksum() returned false for valid state")
	}
}

func TestStateCheckpointerCorruptedFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "checkpoint.json")

	config := &CheckpointConfig{
		FilePath:    filePath,
		BackupCount: 2,
		LockTimeout: 10 * time.Second,
	}
	sc := NewStateCheckpointer(config, logger.NewDefault())

	ctx := context.Background()

	sc.UpdateBootstrap(75, "testing")

	if err := sc.Save(ctx); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Create a backup manually
	if err := copyFile(filePath, filePath+".backup.1"); err != nil {
		t.Fatalf("copyFile() failed: %v", err)
	}

	// Corrupt the main file by changing the checksum
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile() failed: %v", err)
	}
	var state CheckpointState
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("Unmarshal() failed: %v", err)
	}
	state.Checksum = "corrupted_checksum"
	corrupted, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("Marshal() failed: %v", err)
	}
	if err := os.WriteFile(filePath, corrupted, 0o600); err != nil {
		t.Fatalf("WriteFile() failed: %v", err)
	}

	// Load should recover from backup
	sc2 := NewStateCheckpointer(config, logger.NewDefault())
	loaded, err := sc2.Load(ctx)
	if err != nil {
		t.Fatalf("Load() should recover from backup, got error: %v", err)
	}

	if loaded.Bootstrap.Phase != 75 {
		t.Errorf("Bootstrap.Phase = %d, want 75", loaded.Bootstrap.Phase)
	}
}

func TestStateCheckpointerRestoreFromCheckpoint(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "checkpoint.json")

	config := DefaultCheckpointConfig(filePath)
	sc := NewStateCheckpointer(config, logger.NewDefault())

	// Create a checkpoint state to restore
	oldState := &CheckpointState{
		Version: 1,
		Bandwidth: BandwidthState{
			TotalBytesRead:      10000,
			TotalBytesWritten:   5000,
			SessionBytesRead:    1000,
			SessionBytesWritten: 500,
		},
		Circuits: CircuitState{
			TotalBuilds:        100,
			TotalSuccesses:     90,
			TotalFailures:      10,
			AverageBuildTimeMs: 1500,
		},
	}

	sc.RestoreFromCheckpoint(oldState)

	state := sc.GetState()

	// Bandwidth totals should be restored
	if state.Bandwidth.TotalBytesRead != 10000 {
		t.Errorf("TotalBytesRead = %d, want 10000", state.Bandwidth.TotalBytesRead)
	}
	if state.Bandwidth.TotalBytesWritten != 5000 {
		t.Errorf("TotalBytesWritten = %d, want 5000", state.Bandwidth.TotalBytesWritten)
	}

	// Session counters should be reset
	if state.Bandwidth.SessionBytesRead != 0 {
		t.Errorf("SessionBytesRead = %d, want 0 (reset)", state.Bandwidth.SessionBytesRead)
	}
	if state.Bandwidth.SessionBytesWritten != 0 {
		t.Errorf("SessionBytesWritten = %d, want 0 (reset)", state.Bandwidth.SessionBytesWritten)
	}

	// Circuit stats should be restored
	if state.Circuits.TotalBuilds != 100 {
		t.Errorf("TotalBuilds = %d, want 100", state.Circuits.TotalBuilds)
	}
}

func TestStateCheckpointerConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "checkpoint.json")

	config := DefaultCheckpointConfig(filePath)
	sc := NewStateCheckpointer(config, logger.NewDefault())

	ctx := context.Background()

	// Initial save
	if err := sc.Save(ctx); err != nil {
		t.Fatalf("Initial Save() failed: %v", err)
	}

	// Concurrent updates should not cause data races
	var wg sync.WaitGroup
	errChan := make(chan error, 30)

	// Concurrent bandwidth recordings
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sc.RecordBandwidth(100, 50)
		}()
	}

	// Concurrent circuit recordings
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sc.RecordCircuitBuild(i%2 == 0, int64(i*100))
		}(i)
	}

	// Concurrent saves
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := sc.Save(ctx); err != nil {
				errChan <- err
			}
		}()
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		t.Errorf("Concurrent operation error: %v", err)
	}

	// Verify final state is consistent
	state := sc.GetState()
	if state.Bandwidth.TotalBytesRead != 1000 {
		t.Errorf("TotalBytesRead = %d, want 1000", state.Bandwidth.TotalBytesRead)
	}
	if state.Circuits.TotalBuilds != 10 {
		t.Errorf("TotalBuilds = %d, want 10", state.Circuits.TotalBuilds)
	}
}

func TestStateCheckpointerLoop(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "checkpoint.json")

	config := &CheckpointConfig{
		FilePath:           filePath,
		CheckpointInterval: 100 * time.Millisecond, // Fast for testing
		LockTimeout:        10 * time.Second,
		BackupCount:        2,
	}
	sc := NewStateCheckpointer(config, logger.NewDefault())

	ctx := context.Background()

	sc.UpdateBootstrap(25, "testing")

	sc.StartCheckpointLoop(ctx)

	if !sc.IsLoopRunning() {
		t.Error("IsLoopRunning() should return true after start")
	}

	// Wait for a few checkpoints
	time.Sleep(350 * time.Millisecond)

	sc.StopCheckpointLoop()

	if sc.IsLoopRunning() {
		t.Error("IsLoopRunning() should return false after stop")
	}

	// Verify file was written
	if !sc.FileExists() {
		t.Error("File should exist after checkpoint loop")
	}
}

func TestStateCheckpointerLoopDisabled(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "checkpoint.json")

	config := &CheckpointConfig{
		FilePath:           filePath,
		CheckpointInterval: 0, // Disabled
		LockTimeout:        10 * time.Second,
	}
	sc := NewStateCheckpointer(config, logger.NewDefault())

	ctx := context.Background()

	sc.StartCheckpointLoop(ctx) // Should return immediately

	if sc.IsLoopRunning() {
		t.Error("Loop should not be running when interval is 0")
	}

	time.Sleep(100 * time.Millisecond)

	sc.StopCheckpointLoop() // Should be safe to call

	if sc.FileExists() {
		t.Error("File should not exist when checkpointing is disabled")
	}
}

func TestStateCheckpointerDuplicateStart(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "checkpoint.json")

	config := &CheckpointConfig{
		FilePath:           filePath,
		CheckpointInterval: 100 * time.Millisecond,
		LockTimeout:        10 * time.Second,
	}
	sc := NewStateCheckpointer(config, logger.NewDefault())

	ctx := context.Background()

	// Start twice - should not panic or create issues
	sc.StartCheckpointLoop(ctx)
	sc.StartCheckpointLoop(ctx)

	if !sc.IsLoopRunning() {
		t.Error("Loop should be running after start")
	}

	time.Sleep(150 * time.Millisecond)

	sc.StopCheckpointLoop()
	sc.StopCheckpointLoop() // Should be safe to call multiple times
}

func TestStateCheckpointerLoadNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "nonexistent.json")

	config := DefaultCheckpointConfig(filePath)
	sc := NewStateCheckpointer(config, logger.NewDefault())

	ctx := context.Background()

	_, err := sc.Load(ctx)
	if err == nil {
		t.Error("Load() should fail for non-existent file")
	}
}

func TestCalculateChecksum(t *testing.T) {
	data := []byte(`{"test": "data"}`)
	checksum := calculateChecksum(data)

	if len(checksum) != 64 { // SHA-256 hex is 64 chars
		t.Errorf("Checksum length = %d, want 64", len(checksum))
	}

	// Same data should produce same checksum
	checksum2 := calculateChecksum(data)
	if checksum != checksum2 {
		t.Error("Checksum should be deterministic")
	}

	// Different data should produce different checksum
	checksum3 := calculateChecksum([]byte(`{"test": "other"}`))
	if checksum == checksum3 {
		t.Error("Different data should produce different checksum")
	}
}

func TestVerifyChecksum(t *testing.T) {
	// Test nil state
	if verifyChecksum(nil) {
		t.Error("verifyChecksum(nil) should return false")
	}

	// Test empty checksum
	state := &CheckpointState{
		Version: 1,
	}
	if verifyChecksum(state) {
		t.Error("verifyChecksum() with empty checksum should return false")
	}

	// Test invalid checksum
	state.Checksum = "invalid"
	if verifyChecksum(state) {
		t.Error("verifyChecksum() with invalid checksum should return false")
	}

	// Test valid checksum
	stateWithoutChecksum := &CheckpointState{
		Version: state.Version,
	}
	dataWithoutChecksum, err := json.Marshal(stateWithoutChecksum)
	if err != nil {
		t.Fatalf("Marshal() failed: %v", err)
	}
	state.Checksum = calculateChecksum(dataWithoutChecksum)
	if !verifyChecksum(state) {
		t.Error("verifyChecksum() with valid checksum should return true")
	}
}

func TestStateCheckpointerRestoreNil(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "checkpoint.json")

	config := DefaultCheckpointConfig(filePath)
	sc := NewStateCheckpointer(config, logger.NewDefault())

	// Should not panic
	sc.RestoreFromCheckpoint(nil)

	state := sc.GetState()
	if state.Bandwidth.TotalBytesRead != 0 {
		t.Errorf("TotalBytesRead = %d, want 0", state.Bandwidth.TotalBytesRead)
	}
}

func TestStateCheckpointerDisabledBackups(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "checkpoint.json")

	config := &CheckpointConfig{
		FilePath:    filePath,
		BackupCount: 0, // Disabled
		LockTimeout: 10 * time.Second,
	}
	sc := NewStateCheckpointer(config, logger.NewDefault())

	ctx := context.Background()

	// Save multiple times
	for i := 0; i < 5; i++ {
		sc.UpdateBootstrap(i*20, "testing")
		if err := sc.Save(ctx); err != nil {
			t.Fatalf("Save() iteration %d failed: %v", i, err)
		}
	}

	// No backups should exist
	backups := sc.GetBackupPaths()
	if len(backups) != 0 {
		t.Errorf("Got %d backups, want 0 (backups disabled)", len(backups))
	}
}
