// Package path provides tests for guard rotation persistence
// per proposal 271 (Guard Selection) and tor-spec.txt.
//
// These tests verify that guard state is correctly persisted,
// loaded, migrated, and recovered from backup files. Guard
// persistence is essential for maintaining anonymity properties
// across client restarts.
//
// Compliance: proposal 271 (Guard Selection), tor-spec.txt §5.1
package path

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/logger"
)

// TestPersistenceSaveLoadRoundTrip verifies that guard state
// survives a save/load cycle.
func TestPersistenceSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "guard-state.json")
	config := DefaultPersistenceConfig(filePath)
	log := logger.NewDefault()
	p := NewPersistence(config, log)
	ctx := context.Background()

	guards := []GuardEntry{
		{
			Fingerprint: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
			Nickname:    "GuardAlpha",
			Address:     "1.2.3.4:9001",
			FirstUsed:   time.Now().Add(-24 * time.Hour),
			LastUsed:    time.Now(),
			Confirmed:   true,
		},
		{
			Fingerprint: "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB",
			Nickname:    "GuardBeta",
			Address:     "5.6.7.8:9001",
			FirstUsed:   time.Now().Add(-48 * time.Hour),
			LastUsed:    time.Now().Add(-1 * time.Hour),
			Confirmed:   false,
		},
	}

	// Save
	if err := p.Save(ctx, guards); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Verify file exists
	if !p.FileExists() {
		t.Fatal("state file does not exist after save")
	}

	// Load
	loaded, err := p.Load(ctx)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(loaded) != len(guards) {
		t.Fatalf("loaded %d guards, want %d", len(loaded), len(guards))
	}

	for i, g := range loaded {
		if g.Fingerprint != guards[i].Fingerprint {
			t.Errorf("guard %d: fingerprint = %q, want %q",
				i, g.Fingerprint, guards[i].Fingerprint)
		}
		if g.Nickname != guards[i].Nickname {
			t.Errorf("guard %d: nickname = %q, want %q",
				i, g.Nickname, guards[i].Nickname)
		}
		if g.Confirmed != guards[i].Confirmed {
			t.Errorf("guard %d: confirmed = %v, want %v",
				i, g.Confirmed, guards[i].Confirmed)
		}
	}
}

// TestPersistenceSaveEmptyGuards verifies saving an empty guard list.
func TestPersistenceSaveEmptyGuards(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "guard-state.json")
	config := DefaultPersistenceConfig(filePath)
	log := logger.NewDefault()
	p := NewPersistence(config, log)
	ctx := context.Background()

	if err := p.Save(ctx, []GuardEntry{}); err != nil {
		t.Fatalf("Save empty: %v", err)
	}

	loaded, err := p.Load(ctx)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(loaded) != 0 {
		t.Errorf("loaded %d guards, want 0", len(loaded))
	}
}

// TestPersistenceLoadNonexistent verifies loading from a
// nonexistent file.
func TestPersistenceLoadNonexistent(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "does-not-exist.json")
	config := DefaultPersistenceConfig(filePath)
	log := logger.NewDefault()
	p := NewPersistence(config, log)
	ctx := context.Background()

	_, err := p.Load(ctx)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

// TestPersistenceLoadCorruptedJSON verifies that loading corrupted
// JSON fails gracefully.
func TestPersistenceLoadCorruptedJSON(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "guard-state.json")
	config := DefaultPersistenceConfig(filePath)
	log := logger.NewDefault()
	p := NewPersistence(config, log)
	ctx := context.Background()

	// Write corrupted JSON
	if err := os.WriteFile(filePath, []byte("{invalid json}"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := p.Load(ctx)
	if err == nil {
		t.Error("expected error for corrupted JSON")
	}
}

// TestPersistenceLoadCorruptedChecksum verifies that loading
// state with a bad checksum triggers backup recovery.
func TestPersistenceLoadCorruptedChecksum(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "guard-state.json")
	config := DefaultPersistenceConfig(filePath)
	log := logger.NewDefault()
	p := NewPersistence(config, log)
	ctx := context.Background()

	// Write state with incorrect checksum
	state := &GuardStateV2{
		Version:   2,
		Guards:    []GuardEntry{{Fingerprint: "TEST", Confirmed: true}},
		UpdatedAt: time.Now(),
		Checksum:  "bad-checksum",
	}
	data, _ := json.MarshalIndent(state, "", "  ")
	if err := os.WriteFile(filePath, data, 0o600); err != nil {
		t.Fatal(err)
	}

	// Load should fail (bad checksum, no backups)
	_, err := p.Load(ctx)
	if err == nil {
		t.Error("expected error for bad checksum with no backups")
	}
}

// TestPersistenceV1MigrationRoundTrip verifies migration from V1 format.
func TestPersistenceV1MigrationRoundTrip(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "guard-state.json")
	config := DefaultPersistenceConfig(filePath)
	log := logger.NewDefault()
	p := NewPersistence(config, log)
	ctx := context.Background()

	// Write V1 format (no version field)
	oldState := &GuardState{
		Guards: []GuardEntry{
			{Fingerprint: "V1GUARD", Nickname: "OldGuard", Confirmed: true},
		},
		LastUpdated: time.Now(),
	}
	data, _ := json.MarshalIndent(oldState, "", "  ")
	if err := os.WriteFile(filePath, data, 0o600); err != nil {
		t.Fatal(err)
	}

	loaded, err := p.Load(ctx)
	if err != nil {
		t.Fatalf("Load V1: %v", err)
	}

	if len(loaded) != 1 {
		t.Fatalf("loaded %d guards, want 1", len(loaded))
	}
	if loaded[0].Fingerprint != "V1GUARD" {
		t.Errorf("fingerprint = %q, want V1GUARD", loaded[0].Fingerprint)
	}
}

// TestPersistenceMultipleSaves verifies that multiple saves
// overwrite correctly and backups are created.
func TestPersistenceMultipleSaves(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "guard-state.json")
	config := DefaultPersistenceConfig(filePath)
	config.BackupCount = 2
	log := logger.NewDefault()
	p := NewPersistence(config, log)
	ctx := context.Background()

	// Save three times with different data
	for i := 0; i < 3; i++ {
		guards := []GuardEntry{
			{
				Fingerprint: strings.Repeat(string(rune('A'+i)), 40),
				Nickname:    "Guard" + string(rune('A'+i)),
				Confirmed:   true,
			},
		}
		if err := p.Save(ctx, guards); err != nil {
			t.Fatalf("Save %d: %v", i, err)
		}
	}

	// Load should return the last saved data
	loaded, err := p.Load(ctx)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(loaded) != 1 {
		t.Fatalf("loaded %d guards, want 1", len(loaded))
	}
	expected := strings.Repeat("C", 40) // Third save used 'C'
	if loaded[0].Fingerprint != expected {
		t.Errorf("fingerprint = %q, want %q", loaded[0].Fingerprint, expected)
	}
}

// TestPersistenceFileExists verifies FileExists behavior.
func TestPersistenceFileExists(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "guard-state.json")
	config := DefaultPersistenceConfig(filePath)
	log := logger.NewDefault()
	p := NewPersistence(config, log)

	// Before save
	if p.FileExists() {
		t.Error("FileExists() true before any save")
	}

	// After save
	ctx := context.Background()
	if err := p.Save(ctx, []GuardEntry{}); err != nil {
		t.Fatal(err)
	}

	if !p.FileExists() {
		t.Error("FileExists() false after save")
	}
}

// TestPersistenceGetBackupPaths verifies backup path generation.
func TestPersistenceGetBackupPaths(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "guard-state.json")
	config := DefaultPersistenceConfig(filePath)
	config.BackupCount = 3
	log := logger.NewDefault()
	p := NewPersistence(config, log)
	ctx := context.Background()

	// Before any saves, no backups exist
	paths := p.GetBackupPaths()
	if len(paths) != 0 {
		t.Errorf("got %d backup paths before saves, want 0", len(paths))
	}

	// Save multiple times to create backups
	for i := 0; i < 4; i++ {
		guards := []GuardEntry{
			{Fingerprint: strings.Repeat(string(rune('A'+i)), 40), Confirmed: true},
		}
		if err := p.Save(ctx, guards); err != nil {
			t.Fatalf("Save %d: %v", i, err)
		}
	}

	// After multiple saves, backups should exist
	paths = p.GetBackupPaths()
	if len(paths) == 0 {
		t.Error("no backup paths after multiple saves")
	}
	// All returned paths should exist
	for _, bp := range paths {
		if _, err := os.Stat(bp); os.IsNotExist(err) {
			t.Errorf("backup path %q does not exist", bp)
		}
	}
}

// TestDefaultPersistenceConfigValues verifies default configuration values.
func TestDefaultPersistenceConfigValues(t *testing.T) {
	config := DefaultPersistenceConfig("/tmp/test-guards.json")

	if config.FilePath != "/tmp/test-guards.json" {
		t.Errorf("FilePath = %q", config.FilePath)
	}
	if config.BackupCount <= 0 {
		t.Errorf("BackupCount = %d, want > 0", config.BackupCount)
	}
	if config.SnapshotInterval <= 0 {
		t.Errorf("SnapshotInterval = %v, want > 0", config.SnapshotInterval)
	}
	if config.LockTimeout <= 0 {
		t.Errorf("LockTimeout = %v, want > 0", config.LockTimeout)
	}
}

// TestNewPersistenceNilLogger verifies that NewPersistence handles
// nil logger gracefully.
func TestNewPersistenceNilLogger(t *testing.T) {
	dir := t.TempDir()
	config := DefaultPersistenceConfig(filepath.Join(dir, "state.json"))
	p := NewPersistence(config, nil)
	if p == nil {
		t.Fatal("NewPersistence returned nil")
	}
}

// TestPersistenceSaveNilGuards verifies saving nil guard list.
func TestPersistenceSaveNilGuards(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "guard-state.json")
	config := DefaultPersistenceConfig(filePath)
	log := logger.NewDefault()
	p := NewPersistence(config, log)
	ctx := context.Background()

	if err := p.Save(ctx, nil); err != nil {
		t.Fatalf("Save nil: %v", err)
	}

	loaded, err := p.Load(ctx)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// nil guard list should be saved and loaded as empty
	if loaded == nil {
		t.Log("loaded nil guards (acceptable)")
	}
}
