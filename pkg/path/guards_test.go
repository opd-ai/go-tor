package path

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/directory"
	"github.com/opd-ai/go-tor/pkg/logger"
)

func TestNewGuardManager(t *testing.T) {
	tmpDir := t.TempDir()

	gm, err := NewGuardManager(tmpDir, logger.NewDefault())
	if err != nil {
		t.Fatalf("NewGuardManager() failed: %v", err)
	}

	if gm == nil {
		t.Fatal("NewGuardManager() returned nil")
	}

	// Check that state file path is set correctly
	expectedPath := filepath.Join(tmpDir, "guard_state.json")
	if gm.stateFile != expectedPath {
		t.Errorf("stateFile = %s, want %s", gm.stateFile, expectedPath)
	}
}

func TestGuardManagerAddGuard(t *testing.T) {
	tmpDir := t.TempDir()
	gm, err := NewGuardManager(tmpDir, logger.NewDefault())
	if err != nil {
		t.Fatalf("NewGuardManager() failed: %v", err)
	}

	relay := &directory.Relay{
		Nickname:    "TestGuard",
		Fingerprint: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		Address:     "192.0.2.1:9001",
		Flags:       []string{"Guard", "Running", "Valid", "Stable"},
	}

	if err := gm.AddGuard(relay); err != nil {
		t.Fatalf("AddGuard() failed: %v", err)
	}

	guards := gm.GetGuards()
	if len(guards) != 1 {
		t.Errorf("GetGuards() returned %d guards, want 1", len(guards))
	}

	if guards[0].Fingerprint != relay.Fingerprint {
		t.Errorf("guard fingerprint = %s, want %s", guards[0].Fingerprint, relay.Fingerprint)
	}
}

func TestGuardManagerConfirmGuard(t *testing.T) {
	tmpDir := t.TempDir()
	gm, err := NewGuardManager(tmpDir, logger.NewDefault())
	if err != nil {
		t.Fatalf("NewGuardManager() failed: %v", err)
	}

	relay := &directory.Relay{
		Nickname:    "TestGuard",
		Fingerprint: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		Address:     "192.0.2.1:9001",
		Flags:       []string{"Guard", "Running", "Valid", "Stable"},
	}

	if err := gm.AddGuard(relay); err != nil {
		t.Fatalf("AddGuard() failed: %v", err)
	}

	// Initially should not be confirmed
	guards := gm.GetGuards()
	if guards[0].Confirmed {
		t.Error("guard should not be confirmed initially")
	}

	// Confirm the guard
	if err := gm.ConfirmGuard(relay.Fingerprint); err != nil {
		t.Fatalf("ConfirmGuard() failed: %v", err)
	}

	// Now should be confirmed
	guards = gm.GetGuards()
	if !guards[0].Confirmed {
		t.Error("guard should be confirmed after ConfirmGuard()")
	}
}

func TestGuardManagerSaveLoad(t *testing.T) {
	tmpDir := t.TempDir()

	// Create manager and add guards
	gm1, err := NewGuardManager(tmpDir, logger.NewDefault())
	if err != nil {
		t.Fatalf("NewGuardManager() failed: %v", err)
	}

	relay1 := &directory.Relay{
		Nickname:    "Guard1",
		Fingerprint: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		Address:     "192.0.2.1:9001",
		Flags:       []string{"Guard", "Running", "Valid", "Stable"},
	}
	relay2 := &directory.Relay{
		Nickname:    "Guard2",
		Fingerprint: "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB",
		Address:     "192.0.2.2:9001",
		Flags:       []string{"Guard", "Running", "Valid", "Stable"},
	}

	if err := gm1.AddGuard(relay1); err != nil {
		t.Fatalf("AddGuard() failed: %v", err)
	}
	if err := gm1.AddGuard(relay2); err != nil {
		t.Fatalf("AddGuard() failed: %v", err)
	}
	if err := gm1.ConfirmGuard(relay1.Fingerprint); err != nil {
		t.Fatalf("ConfirmGuard() failed: %v", err)
	}

	// Save state
	if err := gm1.Save(); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Create new manager and load state
	gm2, err := NewGuardManager(tmpDir, logger.NewDefault())
	if err != nil {
		t.Fatalf("NewGuardManager() failed: %v", err)
	}

	guards := gm2.GetGuards()
	if len(guards) != 2 {
		t.Errorf("GetGuards() returned %d guards, want 2", len(guards))
	}

	// Check that confirmation status was preserved
	foundConfirmed := false
	for _, guard := range guards {
		if guard.Fingerprint == relay1.Fingerprint && guard.Confirmed {
			foundConfirmed = true
		}
	}
	if !foundConfirmed {
		t.Error("confirmed guard status was not preserved after save/load")
	}
}

func TestGuardManagerMaxGuards(t *testing.T) {
	tmpDir := t.TempDir()
	gm, err := NewGuardManager(tmpDir, logger.NewDefault())
	if err != nil {
		t.Fatalf("NewGuardManager() failed: %v", err)
	}

	// Add more guards than the limit
	for i := 0; i < 5; i++ {
		relay := &directory.Relay{
			Nickname:    "Guard" + string(rune('A'+i)),
			Fingerprint: string(rune('A'+i)) + "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
			Address:     "192.0.2." + string(rune('1'+i)) + ":9001",
			Flags:       []string{"Guard", "Running", "Valid", "Stable"},
		}
		if err := gm.AddGuard(relay); err != nil {
			t.Fatalf("AddGuard() failed: %v", err)
		}
	}

	guards := gm.GetGuards()
	if len(guards) > gm.maxGuards {
		t.Errorf("GetGuards() returned %d guards, want <= %d", len(guards), gm.maxGuards)
	}
}

func TestGuardManagerRemoveGuard(t *testing.T) {
	tmpDir := t.TempDir()
	gm, err := NewGuardManager(tmpDir, logger.NewDefault())
	if err != nil {
		t.Fatalf("NewGuardManager() failed: %v", err)
	}

	relay := &directory.Relay{
		Nickname:    "TestGuard",
		Fingerprint: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		Address:     "192.0.2.1:9001",
		Flags:       []string{"Guard", "Running", "Valid", "Stable"},
	}

	if err := gm.AddGuard(relay); err != nil {
		t.Fatalf("AddGuard() failed: %v", err)
	}

	if err := gm.RemoveGuard(relay.Fingerprint); err != nil {
		t.Fatalf("RemoveGuard() failed: %v", err)
	}

	guards := gm.GetGuards()
	if len(guards) != 0 {
		t.Errorf("GetGuards() returned %d guards after removal, want 0", len(guards))
	}
}

func TestGuardManagerCleanupExpired(t *testing.T) {
	tmpDir := t.TempDir()
	gm, err := NewGuardManager(tmpDir, logger.NewDefault())
	if err != nil {
		t.Fatalf("NewGuardManager() failed: %v", err)
	}

	// Set shorter expiry for testing
	gm.guardExpiry = 1 * time.Second

	relay := &directory.Relay{
		Nickname:    "TestGuard",
		Fingerprint: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		Address:     "192.0.2.1:9001",
		Flags:       []string{"Guard", "Running", "Valid", "Stable"},
	}

	if err := gm.AddGuard(relay); err != nil {
		t.Fatalf("AddGuard() failed: %v", err)
	}

	// Wait for expiry
	time.Sleep(2 * time.Second)

	gm.CleanupExpired()

	guards := gm.GetGuards()
	if len(guards) != 0 {
		t.Errorf("GetGuards() returned %d guards after cleanup, want 0", len(guards))
	}
}

func TestGuardManagerGetStats(t *testing.T) {
	tmpDir := t.TempDir()
	gm, err := NewGuardManager(tmpDir, logger.NewDefault())
	if err != nil {
		t.Fatalf("NewGuardManager() failed: %v", err)
	}

	relay1 := &directory.Relay{
		Nickname:    "Guard1",
		Fingerprint: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		Address:     "192.0.2.1:9001",
		Flags:       []string{"Guard", "Running", "Valid", "Stable"},
	}
	relay2 := &directory.Relay{
		Nickname:    "Guard2",
		Fingerprint: "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB",
		Address:     "192.0.2.2:9001",
		Flags:       []string{"Guard", "Running", "Valid", "Stable"},
	}

	if err := gm.AddGuard(relay1); err != nil {
		t.Fatalf("AddGuard() failed: %v", err)
	}
	if err := gm.AddGuard(relay2); err != nil {
		t.Fatalf("AddGuard() failed: %v", err)
	}
	if err := gm.ConfirmGuard(relay1.Fingerprint); err != nil {
		t.Fatalf("ConfirmGuard() failed: %v", err)
	}

	stats := gm.GetStats()
	if stats.TotalGuards != 2 {
		t.Errorf("TotalGuards = %d, want 2", stats.TotalGuards)
	}
	if stats.ConfirmedGuards != 1 {
		t.Errorf("ConfirmedGuards = %d, want 1", stats.ConfirmedGuards)
	}
}

func TestGuardManagerNonExistentDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistentDir := filepath.Join(tmpDir, "nonexistent", "path")

	gm, err := NewGuardManager(nonExistentDir, logger.NewDefault())
	if err != nil {
		t.Fatalf("NewGuardManager() should create directory, got error: %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(nonExistentDir); os.IsNotExist(err) {
		t.Error("NewGuardManager() did not create data directory")
	}

	// Should be able to save
	if err := gm.Save(); err != nil {
		t.Errorf("Save() to new directory failed: %v", err)
	}
}

func TestNewGuardManagerWithConfig(t *testing.T) {
	tmpDir := t.TempDir()

	config := DefaultGuardManagerConfig(tmpDir)
	gm, err := NewGuardManagerWithConfig(config, logger.NewDefault())
	if err != nil {
		t.Fatalf("NewGuardManagerWithConfig() failed: %v", err)
	}

	if gm == nil {
		t.Fatal("NewGuardManagerWithConfig() returned nil")
	}

	// Verify enhanced persistence is enabled
	if !gm.HasEnhancedPersistence() {
		t.Error("Enhanced persistence should be enabled with default config")
	}
}

func TestGuardManagerWithConfigNilConfig(t *testing.T) {
	_, err := NewGuardManagerWithConfig(nil, logger.NewDefault())
	if err == nil {
		t.Error("NewGuardManagerWithConfig(nil) should fail")
	}
}

func TestGuardManagerEnhancedPersistenceSaveLoad(t *testing.T) {
	tmpDir := t.TempDir()

	config := DefaultGuardManagerConfig(tmpDir)
	gm1, err := NewGuardManagerWithConfig(config, logger.NewDefault())
	if err != nil {
		t.Fatalf("NewGuardManagerWithConfig() failed: %v", err)
	}

	relay := &directory.Relay{
		Nickname:    "EnhancedGuard",
		Fingerprint: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		Address:     "192.0.2.1:9001",
		Flags:       []string{"Guard", "Running", "Valid", "Stable"},
	}

	if err := gm1.AddGuard(relay); err != nil {
		t.Fatalf("AddGuard() failed: %v", err)
	}

	if err := gm1.Save(); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Create new manager and verify state was persisted
	gm2, err := NewGuardManagerWithConfig(config, logger.NewDefault())
	if err != nil {
		t.Fatalf("NewGuardManagerWithConfig() failed on reload: %v", err)
	}

	guards := gm2.GetGuards()
	if len(guards) != 1 {
		t.Errorf("GetGuards() returned %d guards, want 1", len(guards))
	}

	if guards[0].Nickname != "EnhancedGuard" {
		t.Errorf("Guard nickname = %s, want EnhancedGuard", guards[0].Nickname)
	}
}

func TestGuardManagerBackupCreation(t *testing.T) {
	tmpDir := t.TempDir()

	config := &GuardManagerConfig{
		DataDir:          tmpDir,
		MaxGuards:        3,
		GuardExpiry:      90 * 24 * time.Hour,
		BackupCount:      3,
		SnapshotInterval: 0, // Disable snapshots for this test
		LockTimeout:      10 * time.Second,
	}

	gm, err := NewGuardManagerWithConfig(config, logger.NewDefault())
	if err != nil {
		t.Fatalf("NewGuardManagerWithConfig() failed: %v", err)
	}

	relay := &directory.Relay{
		Nickname:    "BackupGuard",
		Fingerprint: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		Address:     "192.0.2.1:9001",
		Flags:       []string{"Guard", "Running", "Valid", "Stable"},
	}

	// Save multiple times to create backups
	for i := 0; i < 4; i++ {
		if err := gm.AddGuard(relay); err != nil {
			t.Fatalf("AddGuard() iteration %d failed: %v", i, err)
		}
		if err := gm.Save(); err != nil {
			t.Fatalf("Save() iteration %d failed: %v", i, err)
		}
	}

	// Check that backups exist
	backups := gm.GetBackupPaths()
	if len(backups) == 0 {
		t.Error("No backups created after multiple saves")
	}
	if len(backups) > config.BackupCount {
		t.Errorf("Too many backups: got %d, want <= %d", len(backups), config.BackupCount)
	}
}

func TestGuardManagerSnapshotLoop(t *testing.T) {
	tmpDir := t.TempDir()

	config := &GuardManagerConfig{
		DataDir:          tmpDir,
		MaxGuards:        3,
		GuardExpiry:      90 * 24 * time.Hour,
		BackupCount:      3,
		SnapshotInterval: 100 * time.Millisecond, // Fast for testing
		LockTimeout:      10 * time.Second,
	}

	gm, err := NewGuardManagerWithConfig(config, logger.NewDefault())
	if err != nil {
		t.Fatalf("NewGuardManagerWithConfig() failed: %v", err)
	}

	relay := &directory.Relay{
		Nickname:    "SnapshotGuard",
		Fingerprint: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		Address:     "192.0.2.1:9001",
		Flags:       []string{"Guard", "Running", "Valid", "Stable"},
	}

	if err := gm.AddGuard(relay); err != nil {
		t.Fatalf("AddGuard() failed: %v", err)
	}

	gm.StartSnapshotLoop()

	// Wait for snapshots to occur
	time.Sleep(350 * time.Millisecond)

	gm.StopSnapshotLoop()

	// Verify file was created by snapshot loop
	stateFile := filepath.Join(tmpDir, "guard_state.json")
	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		t.Error("State file should exist after snapshot loop")
	}
}

func TestGuardManagerLegacyCompatibility(t *testing.T) {
	tmpDir := t.TempDir()

	// Create using legacy constructor
	gm, err := NewGuardManager(tmpDir, logger.NewDefault())
	if err != nil {
		t.Fatalf("NewGuardManager() failed: %v", err)
	}

	// Legacy mode should not have enhanced persistence
	if gm.HasEnhancedPersistence() {
		t.Error("Legacy constructor should not enable enhanced persistence")
	}

	relay := &directory.Relay{
		Nickname:    "LegacyGuard",
		Fingerprint: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		Address:     "192.0.2.1:9001",
		Flags:       []string{"Guard", "Running", "Valid", "Stable"},
	}

	if err := gm.AddGuard(relay); err != nil {
		t.Fatalf("AddGuard() failed: %v", err)
	}

	if err := gm.Save(); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Verify file exists
	stateFile := filepath.Join(tmpDir, "guard_state.json")
	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		t.Error("State file should exist after Save()")
	}
}
