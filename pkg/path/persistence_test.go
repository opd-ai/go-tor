package path

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

func TestNewPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "guard_state.json")

	config := DefaultPersistenceConfig(filePath)
	p := NewPersistence(config, logger.NewDefault())

	if p == nil {
		t.Fatal("NewPersistence() returned nil")
	}
	if p.config.FilePath != filePath {
		t.Errorf("FilePath = %s, want %s", p.config.FilePath, filePath)
	}
}

func TestDefaultPersistenceConfig(t *testing.T) {
	config := DefaultPersistenceConfig("/tmp/test.json")

	if config.FilePath != "/tmp/test.json" {
		t.Errorf("FilePath = %s, want /tmp/test.json", config.FilePath)
	}
	if config.BackupCount != 3 {
		t.Errorf("BackupCount = %d, want 3", config.BackupCount)
	}
	if config.SnapshotInterval != 5*time.Minute {
		t.Errorf("SnapshotInterval = %v, want 5m", config.SnapshotInterval)
	}
	if config.LockTimeout != 10*time.Second {
		t.Errorf("LockTimeout = %v, want 10s", config.LockTimeout)
	}
}

func TestPersistenceSaveLoad(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "guard_state.json")

	config := DefaultPersistenceConfig(filePath)
	p := NewPersistence(config, logger.NewDefault())

	ctx := context.Background()

	guards := []GuardEntry{
		{
			Fingerprint: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
			Nickname:    "TestGuard1",
			Address:     "192.0.2.1:9001",
			FirstUsed:   time.Now().Add(-24 * time.Hour),
			LastUsed:    time.Now(),
			Confirmed:   true,
		},
		{
			Fingerprint: "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB",
			Nickname:    "TestGuard2",
			Address:     "192.0.2.2:9001",
			FirstUsed:   time.Now(),
			LastUsed:    time.Now(),
			Confirmed:   false,
		},
	}

	// Save guards
	if err := p.Save(ctx, guards); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Verify file exists
	if !p.FileExists() {
		t.Fatal("FileExists() returned false after Save()")
	}

	// Load guards
	loaded, err := p.Load(ctx)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if len(loaded) != len(guards) {
		t.Errorf("Loaded %d guards, want %d", len(loaded), len(guards))
	}

	// Verify guard data
	for i, guard := range loaded {
		if guard.Fingerprint != guards[i].Fingerprint {
			t.Errorf("Guard[%d].Fingerprint = %s, want %s", i, guard.Fingerprint, guards[i].Fingerprint)
		}
		if guard.Nickname != guards[i].Nickname {
			t.Errorf("Guard[%d].Nickname = %s, want %s", i, guard.Nickname, guards[i].Nickname)
		}
		if guard.Confirmed != guards[i].Confirmed {
			t.Errorf("Guard[%d].Confirmed = %v, want %v", i, guard.Confirmed, guards[i].Confirmed)
		}
	}
}

func TestPersistenceBackupRotation(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "guard_state.json")

	config := &PersistenceConfig{
		FilePath:    filePath,
		BackupCount: 3,
		LockTimeout: 10 * time.Second,
	}
	p := NewPersistence(config, logger.NewDefault())

	ctx := context.Background()

	// Save multiple times to create backups
	for i := 0; i < 5; i++ {
		guards := []GuardEntry{
			{
				Fingerprint: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
				Nickname:    "TestGuard",
				Address:     "192.0.2.1:9001",
				FirstUsed:   time.Now(),
				LastUsed:    time.Now(),
				Confirmed:   i%2 == 0, // Alternate confirmed status
			},
		}
		if err := p.Save(ctx, guards); err != nil {
			t.Fatalf("Save() iteration %d failed: %v", i, err)
		}
	}

	// Check backup files exist (should have at most BackupCount backups)
	backups := p.GetBackupPaths()
	if len(backups) > config.BackupCount {
		t.Errorf("Got %d backups, want <= %d", len(backups), config.BackupCount)
	}

	// At least backup.1 should exist after 5 saves
	if _, err := os.Stat(filePath + ".backup.1"); os.IsNotExist(err) {
		t.Error("Expected backup.1 to exist after multiple saves")
	}
}

func TestPersistenceChecksumVerification(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "guard_state.json")

	config := DefaultPersistenceConfig(filePath)
	p := NewPersistence(config, logger.NewDefault())

	ctx := context.Background()

	guards := []GuardEntry{
		{
			Fingerprint: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
			Nickname:    "TestGuard",
			Address:     "192.0.2.1:9001",
			FirstUsed:   time.Now(),
			LastUsed:    time.Now(),
			Confirmed:   true,
		},
	}

	// Save guards
	if err := p.Save(ctx, guards); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Read and verify file contains valid checksum
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile() failed: %v", err)
	}

	var state GuardStateV2
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("Unmarshal() failed: %v", err)
	}

	if state.Version != currentSchemaVersion {
		t.Errorf("Version = %d, want %d", state.Version, currentSchemaVersion)
	}

	if state.Checksum == "" {
		t.Error("Checksum is empty")
	}

	// Verify checksum is valid
	if !verifyChecksum(&state) {
		t.Error("verifyChecksum() returned false for valid state")
	}
}

func TestPersistenceCorruptedFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "guard_state.json")

	config := &PersistenceConfig{
		FilePath:    filePath,
		BackupCount: 3,
		LockTimeout: 10 * time.Second,
	}
	p := NewPersistence(config, logger.NewDefault())

	ctx := context.Background()

	guards := []GuardEntry{
		{
			Fingerprint: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
			Nickname:    "TestGuard",
			Address:     "192.0.2.1:9001",
			FirstUsed:   time.Now(),
			LastUsed:    time.Now(),
			Confirmed:   true,
		},
	}

	// Save guards (creates valid file)
	if err := p.Save(ctx, guards); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Create a backup manually
	if err := copyFile(filePath, filePath+".backup.1"); err != nil {
		t.Fatalf("copyFile() failed: %v", err)
	}

	// Corrupt the main file by changing the checksum
	// Errors are intentionally ignored here as we're setting up a corruption scenario
	// and the prior Save() ensures the file exists and is valid JSON
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile() failed: %v", err)
	}
	var state GuardStateV2
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("Unmarshal() failed: %v", err)
	}
	state.Checksum = "invalid_checksum"
	corrupted, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("Marshal() failed: %v", err)
	}
	if err := os.WriteFile(filePath, corrupted, 0o600); err != nil {
		t.Fatalf("WriteFile() failed: %v", err)
	}

	// Load should recover from backup
	loaded, err := p.Load(ctx)
	if err != nil {
		t.Fatalf("Load() should recover from backup, got error: %v", err)
	}

	if len(loaded) != 1 {
		t.Errorf("Loaded %d guards from backup, want 1", len(loaded))
	}
}

func TestPersistenceV1Migration(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "guard_state.json")

	// Create V1 format file (no version or checksum)
	v1State := GuardState{
		Guards: []GuardEntry{
			{
				Fingerprint: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
				Nickname:    "LegacyGuard",
				Address:     "192.0.2.1:9001",
				FirstUsed:   time.Now(),
				LastUsed:    time.Now(),
				Confirmed:   true,
			},
		},
		LastUpdated: time.Now(),
	}

	data, err := json.Marshal(v1State)
	if err != nil {
		t.Fatalf("Marshal() failed: %v", err)
	}
	if err := os.WriteFile(filePath, data, 0o600); err != nil {
		t.Fatalf("WriteFile() failed: %v", err)
	}

	config := DefaultPersistenceConfig(filePath)
	p := NewPersistence(config, logger.NewDefault())

	ctx := context.Background()

	// Load should migrate V1 to V2
	loaded, err := p.Load(ctx)
	if err != nil {
		t.Fatalf("Load() failed to migrate V1: %v", err)
	}

	if len(loaded) != 1 {
		t.Errorf("Loaded %d guards, want 1", len(loaded))
	}

	if loaded[0].Nickname != "LegacyGuard" {
		t.Errorf("Nickname = %s, want LegacyGuard", loaded[0].Nickname)
	}
}

func TestPersistenceConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "guard_state.json")

	config := DefaultPersistenceConfig(filePath)
	p := NewPersistence(config, logger.NewDefault())

	ctx := context.Background()

	// Initial save
	if err := p.Save(ctx, []GuardEntry{}); err != nil {
		t.Fatalf("Initial Save() failed: %v", err)
	}

	// Concurrent writes should not corrupt data
	var wg sync.WaitGroup
	errChan := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			guards := []GuardEntry{
				{
					Fingerprint: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
					Nickname:    "ConcurrentGuard",
					Address:     "192.0.2.1:9001",
					FirstUsed:   time.Now(),
					LastUsed:    time.Now(),
					Confirmed:   true,
				},
			}
			if err := p.Save(ctx, guards); err != nil {
				errChan <- err
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		t.Errorf("Concurrent Save() error: %v", err)
	}

	// Verify file is still valid
	loaded, err := p.Load(ctx)
	if err != nil {
		t.Fatalf("Load() after concurrent writes failed: %v", err)
	}

	if len(loaded) != 1 {
		t.Errorf("Loaded %d guards, want 1", len(loaded))
	}
}

func TestPersistenceSnapshotLoop(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "guard_state.json")

	config := &PersistenceConfig{
		FilePath:         filePath,
		BackupCount:      3,
		SnapshotInterval: 100 * time.Millisecond, // Fast for testing
		LockTimeout:      10 * time.Second,
	}
	p := NewPersistence(config, logger.NewDefault())

	callCount := 0
	var mu sync.Mutex

	getGuards := func() []GuardEntry {
		mu.Lock()
		callCount++
		mu.Unlock()
		return []GuardEntry{
			{
				Fingerprint: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
				Nickname:    "SnapshotGuard",
				Address:     "192.0.2.1:9001",
				FirstUsed:   time.Now(),
				LastUsed:    time.Now(),
				Confirmed:   true,
			},
		}
	}

	p.StartSnapshotLoop(getGuards)

	// Wait for a few snapshots
	time.Sleep(350 * time.Millisecond)

	p.StopSnapshotLoop()

	mu.Lock()
	calls := callCount
	mu.Unlock()

	if calls < 2 {
		t.Errorf("Snapshot loop called getGuards %d times, want >= 2", calls)
	}

	// Verify file was written
	if !p.FileExists() {
		t.Error("File should exist after snapshot loop")
	}
}

func TestPersistenceDisabledSnapshots(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "guard_state.json")

	config := &PersistenceConfig{
		FilePath:         filePath,
		BackupCount:      3,
		SnapshotInterval: 0, // Disabled
		LockTimeout:      10 * time.Second,
	}
	p := NewPersistence(config, logger.NewDefault())

	callCount := 0
	getGuards := func() []GuardEntry {
		callCount++
		return []GuardEntry{}
	}

	p.StartSnapshotLoop(getGuards) // Should return immediately

	time.Sleep(100 * time.Millisecond)

	p.StopSnapshotLoop()

	if callCount != 0 {
		t.Errorf("Snapshot loop should not run when disabled, got %d calls", callCount)
	}
}

func TestPersistenceDisabledBackups(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "guard_state.json")

	config := &PersistenceConfig{
		FilePath:    filePath,
		BackupCount: 0, // Disabled
		LockTimeout: 10 * time.Second,
	}
	p := NewPersistence(config, logger.NewDefault())

	ctx := context.Background()

	// Save multiple times
	for i := 0; i < 5; i++ {
		guards := []GuardEntry{
			{
				Fingerprint: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
				Nickname:    "TestGuard",
				Address:     "192.0.2.1:9001",
				FirstUsed:   time.Now(),
				LastUsed:    time.Now(),
				Confirmed:   true,
			},
		}
		if err := p.Save(ctx, guards); err != nil {
			t.Fatalf("Save() iteration %d failed: %v", i, err)
		}
	}

	// No backups should exist
	backups := p.GetBackupPaths()
	if len(backups) != 0 {
		t.Errorf("Got %d backups, want 0 (backups disabled)", len(backups))
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
	state := &GuardStateV2{
		Version: 2,
		Guards:  []GuardEntry{},
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
	dataWithoutChecksum, err := json.Marshal(&GuardStateV2{
		Version:   state.Version,
		Guards:    state.Guards,
		UpdatedAt: state.UpdatedAt,
	})
	if err != nil {
		t.Fatalf("Marshal() failed: %v", err)
	}
	state.Checksum = calculateChecksum(dataWithoutChecksum)
	if !verifyChecksum(state) {
		t.Error("verifyChecksum() with valid checksum should return true")
	}
}

func TestMigrateV1ToV2(t *testing.T) {
	oldState := &GuardState{
		Guards: []GuardEntry{
			{
				Fingerprint: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
				Nickname:    "OldGuard",
				Address:     "192.0.2.1:9001",
				FirstUsed:   time.Now(),
				LastUsed:    time.Now(),
				Confirmed:   true,
			},
		},
		LastUpdated: time.Now(),
	}

	newState := migrateV1ToV2(oldState)

	if newState.Version != currentSchemaVersion {
		t.Errorf("Version = %d, want %d", newState.Version, currentSchemaVersion)
	}

	if len(newState.Guards) != 1 {
		t.Errorf("Guards count = %d, want 1", len(newState.Guards))
	}

	if newState.Checksum == "" {
		t.Error("Checksum should be set after migration")
	}
}

func TestCopyFile(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "source.txt")
	dstPath := filepath.Join(tmpDir, "dest.txt")

	// Create source file
	content := []byte("test content for copy")
	if err := os.WriteFile(srcPath, content, 0o600); err != nil {
		t.Fatalf("WriteFile() failed: %v", err)
	}

	// Copy file
	if err := copyFile(srcPath, dstPath); err != nil {
		t.Fatalf("copyFile() failed: %v", err)
	}

	// Verify destination content
	destContent, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("ReadFile() failed: %v", err)
	}

	if string(destContent) != string(content) {
		t.Errorf("Destination content = %s, want %s", destContent, content)
	}
}

func TestCopyFileNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "nonexistent.txt")
	dstPath := filepath.Join(tmpDir, "dest.txt")

	err := copyFile(srcPath, dstPath)
	if err == nil {
		t.Error("copyFile() should fail for non-existent source")
	}
}

func TestPersistenceLoadNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "nonexistent.json")

	config := DefaultPersistenceConfig(filePath)
	p := NewPersistence(config, logger.NewDefault())

	ctx := context.Background()

	_, err := p.Load(ctx)
	if err == nil {
		t.Error("Load() should fail for non-existent file")
	}
}
