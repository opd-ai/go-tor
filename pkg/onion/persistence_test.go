// Package onion - Service Persistence Tests
package onion

import (
	"crypto/ed25519"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/crypto"
)

func TestServicePersistence_SaveLoadIdentityKey(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()

	sp, err := NewServicePersistence(tempDir, nil)
	if err != nil {
		t.Fatalf("Failed to create persistence: %v", err)
	}

	// Generate test key
	_, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	// Save key
	if err := sp.SaveIdentityKey(privateKey); err != nil {
		t.Fatalf("Failed to save identity key: %v", err)
	}

	// Verify file exists with correct permissions
	keyPath := filepath.Join(tempDir, identityKeyFile)
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("Key file not found: %v", err)
	}

	if info.Mode().Perm() != keyFilePerms {
		t.Errorf("Incorrect key file permissions: got %o, want %o",
			info.Mode().Perm(), keyFilePerms)
	}

	// Load key
	loadedKey, err := sp.LoadIdentityKey()
	if err != nil {
		t.Fatalf("Failed to load identity key: %v", err)
	}

	// Verify key matches
	if !privateKey.Equal(loadedKey) {
		t.Error("Loaded key does not match saved key")
	}
}

func TestServicePersistence_SaveLoadNtorKey(t *testing.T) {
	tempDir := t.TempDir()

	sp, err := NewServicePersistence(tempDir, nil)
	if err != nil {
		t.Fatalf("Failed to create persistence: %v", err)
	}

	// Generate test ntor key (32 bytes)
	ntorKey, err := crypto.GenerateRandomBytes(32)
	if err != nil {
		t.Fatalf("Failed to generate ntor key: %v", err)
	}

	// Save key
	if err := sp.SaveNtorKey(ntorKey); err != nil {
		t.Fatalf("Failed to save ntor key: %v", err)
	}

	// Load key
	loadedKey, err := sp.LoadNtorKey()
	if err != nil {
		t.Fatalf("Failed to load ntor key: %v", err)
	}

	// Verify key matches
	if len(loadedKey) != 32 {
		t.Errorf("Invalid ntor key size: %d", len(loadedKey))
	}

	for i := 0; i < 32; i++ {
		if ntorKey[i] != loadedKey[i] {
			t.Error("Loaded ntor key does not match saved key")
			break
		}
	}
}

func TestServicePersistence_SaveLoadState(t *testing.T) {
	tempDir := t.TempDir()

	sp, err := NewServicePersistence(tempDir, nil)
	if err != nil {
		t.Fatalf("Failed to create persistence: %v", err)
	}

	// Create test state
	now := time.Now().Truncate(time.Second) // Truncate for JSON roundtrip
	state := &ServiceState{
		OnionAddress:          "test3huzglnk2s4fzqszqrmxs6rkk7gkp2gqcejv42kjcemwvbdlrohqd.onion",
		CreatedAt:             now,
		LastStarted:           now,
		LastDescriptorPublish: now,
		DescriptorRevision:    42,
		IntroPointCache: []IntroPointState{
			{
				Fingerprint: "ABC123",
				AuthKeyHex:  "deadbeef",
				EncKeyHex:   "cafebabe",
				CreatedAt:   now,
			},
		},
	}

	// Save state
	if err := sp.SaveState(state); err != nil {
		t.Fatalf("Failed to save state: %v", err)
	}

	// Load state
	loadedState, err := sp.LoadState()
	if err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}

	// Verify state matches
	if loadedState.OnionAddress != state.OnionAddress {
		t.Errorf("OnionAddress mismatch: got %s, want %s",
			loadedState.OnionAddress, state.OnionAddress)
	}

	if !loadedState.CreatedAt.Equal(state.CreatedAt) {
		t.Errorf("CreatedAt mismatch: got %v, want %v",
			loadedState.CreatedAt, state.CreatedAt)
	}

	if loadedState.DescriptorRevision != state.DescriptorRevision {
		t.Errorf("DescriptorRevision mismatch: got %d, want %d",
			loadedState.DescriptorRevision, state.DescriptorRevision)
	}

	if len(loadedState.IntroPointCache) != 1 {
		t.Fatalf("IntroPointCache length mismatch: got %d, want 1",
			len(loadedState.IntroPointCache))
	}

	if loadedState.IntroPointCache[0].Fingerprint != "ABC123" {
		t.Errorf("IntroPoint fingerprint mismatch: got %s, want ABC123",
			loadedState.IntroPointCache[0].Fingerprint)
	}
}

func TestServicePersistence_KeysExist(t *testing.T) {
	tempDir := t.TempDir()

	sp, err := NewServicePersistence(tempDir, nil)
	if err != nil {
		t.Fatalf("Failed to create persistence: %v", err)
	}

	// Initially, keys should not exist
	if sp.KeysExist() {
		t.Error("KeysExist() returned true for empty directory")
	}

	// Save identity key
	_, privateKey, _ := ed25519.GenerateKey(nil)
	sp.SaveIdentityKey(privateKey)

	// Only identity key exists
	if sp.KeysExist() {
		t.Error("KeysExist() returned true with only identity key")
	}

	// Save ntor key
	ntorKey, err := crypto.GenerateRandomBytes(32)
	if err != nil {
		t.Fatalf("Failed to generate ntor key: %v", err)
	}
	sp.SaveNtorKey(ntorKey)

	// Both keys exist
	if !sp.KeysExist() {
		t.Error("KeysExist() returned false with both keys present")
	}
}

func TestServicePersistence_StateExists(t *testing.T) {
	tempDir := t.TempDir()

	sp, err := NewServicePersistence(tempDir, nil)
	if err != nil {
		t.Fatalf("Failed to create persistence: %v", err)
	}

	// Initially, state should not exist
	if sp.StateExists() {
		t.Error("StateExists() returned true for empty directory")
	}

	// Save state
	state := &ServiceState{
		OnionAddress: "test.onion",
		CreatedAt:    time.Now(),
	}
	sp.SaveState(state)

	// State exists
	if !sp.StateExists() {
		t.Error("StateExists() returned false after saving state")
	}
}

func TestServicePersistence_ExportImportKeys(t *testing.T) {
	tempDir := t.TempDir()

	sp, err := NewServicePersistence(tempDir, nil)
	if err != nil {
		t.Fatalf("Failed to create persistence: %v", err)
	}

	// Generate and save keys
	_, privateKey, _ := ed25519.GenerateKey(nil)
	ntorKey, err := crypto.GenerateRandomBytes(32)
	if err != nil {
		t.Fatalf("Failed to generate ntor key: %v", err)
	}

	sp.SaveIdentityKey(privateKey)
	sp.SaveNtorKey(ntorKey)

	// Export keys
	exportedIdentity, exportedNtor, err := sp.ExportKeys()
	if err != nil {
		t.Fatalf("Failed to export keys: %v", err)
	}

	// Verify exported keys match
	if !privateKey.Equal(exportedIdentity) {
		t.Error("Exported identity key does not match original")
	}

	for i := 0; i < 32; i++ {
		if ntorKey[i] != exportedNtor[i] {
			t.Error("Exported ntor key does not match original")
			break
		}
	}

	// Import to new directory
	tempDir2 := t.TempDir()
	sp2, _ := NewServicePersistence(tempDir2, nil)

	if err := sp2.ImportKeys(exportedIdentity, exportedNtor); err != nil {
		t.Fatalf("Failed to import keys: %v", err)
	}

	// Verify imported keys
	importedIdentity, err := sp2.LoadIdentityKey()
	if err != nil {
		t.Fatalf("Failed to load imported identity key: %v", err)
	}

	importedNtor, err := sp2.LoadNtorKey()
	if err != nil {
		t.Fatalf("Failed to load imported ntor key: %v", err)
	}

	if !privateKey.Equal(importedIdentity) {
		t.Error("Imported identity key does not match original")
	}

	for i := 0; i < 32; i++ {
		if ntorKey[i] != importedNtor[i] {
			t.Error("Imported ntor key does not match original")
			break
		}
	}
}

func TestServicePersistence_SecureDelete(t *testing.T) {
	tempDir := t.TempDir()

	sp, err := NewServicePersistence(tempDir, nil)
	if err != nil {
		t.Fatalf("Failed to create persistence: %v", err)
	}

	// Create keys and state
	_, privateKey, _ := ed25519.GenerateKey(nil)
	ntorKey, err := crypto.GenerateRandomBytes(32)
	if err != nil {
		t.Fatalf("Failed to generate ntor key: %v", err)
	}
	state := &ServiceState{
		OnionAddress: "test.onion",
		CreatedAt:    time.Now(),
	}

	sp.SaveIdentityKey(privateKey)
	sp.SaveNtorKey(ntorKey)
	sp.SaveState(state)

	// Verify files exist
	if !sp.KeysExist() {
		t.Fatal("Keys should exist before secure delete")
	}
	if !sp.StateExists() {
		t.Fatal("State should exist before secure delete")
	}

	// Secure delete
	if err := sp.SecureDelete(); err != nil {
		t.Fatalf("Failed to secure delete: %v", err)
	}

	// Verify files are gone
	if sp.KeysExist() {
		t.Error("Keys still exist after secure delete")
	}
	if sp.StateExists() {
		t.Error("State still exists after secure delete")
	}
}

func TestServicePersistence_InvalidKeySize(t *testing.T) {
	tempDir := t.TempDir()

	sp, err := NewServicePersistence(tempDir, nil)
	if err != nil {
		t.Fatalf("Failed to create persistence: %v", err)
	}

	// Try to save invalid identity key
	invalidKey := make([]byte, 32) // Wrong size (should be 64)
	if err := sp.SaveIdentityKey(invalidKey); err == nil {
		t.Error("SaveIdentityKey should reject invalid key size")
	}

	// Try to save invalid ntor key
	invalidNtor := make([]byte, 16) // Wrong size (should be 32)
	if err := sp.SaveNtorKey(invalidNtor); err == nil {
		t.Error("SaveNtorKey should reject invalid key size")
	}
}

func TestServicePersistence_LoadNonexistentKeys(t *testing.T) {
	tempDir := t.TempDir()

	sp, err := NewServicePersistence(tempDir, nil)
	if err != nil {
		t.Fatalf("Failed to create persistence: %v", err)
	}

	// Try to load nonexistent identity key
	_, err = sp.LoadIdentityKey()
	if err == nil {
		t.Error("LoadIdentityKey should fail for nonexistent key")
	}

	// Try to load nonexistent ntor key
	_, err = sp.LoadNtorKey()
	if err == nil {
		t.Error("LoadNtorKey should fail for nonexistent key")
	}

	// Try to load nonexistent state
	_, err = sp.LoadState()
	if err == nil {
		t.Error("LoadState should fail for nonexistent state")
	}
}

func TestServicePersistence_EmptyDataDirectory(t *testing.T) {
	// Try to create persistence with empty directory
	_, err := NewServicePersistence("", nil)
	if err == nil {
		t.Error("NewServicePersistence should fail with empty data directory")
	}
}

func TestServicePersistence_SaveNilState(t *testing.T) {
	tempDir := t.TempDir()

	sp, err := NewServicePersistence(tempDir, nil)
	if err != nil {
		t.Fatalf("Failed to create persistence: %v", err)
	}

	// Try to save nil state
	if err := sp.SaveState(nil); err == nil {
		t.Error("SaveState should fail with nil state")
	}
}

func TestServicePersistence_AtomicStateWrite(t *testing.T) {
	tempDir := t.TempDir()

	sp, err := NewServicePersistence(tempDir, nil)
	if err != nil {
		t.Fatalf("Failed to create persistence: %v", err)
	}

	// Save initial state
	state1 := &ServiceState{
		OnionAddress:       "test1.onion",
		CreatedAt:          time.Now(),
		DescriptorRevision: 1,
	}
	sp.SaveState(state1)

	// Update state (should be atomic)
	state2 := &ServiceState{
		OnionAddress:       "test2.onion",
		CreatedAt:          time.Now(),
		DescriptorRevision: 2,
	}
	sp.SaveState(state2)

	// Load and verify we got the latest state
	loaded, err := sp.LoadState()
	if err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}

	if loaded.OnionAddress != "test2.onion" {
		t.Errorf("Expected test2.onion, got %s", loaded.OnionAddress)
	}

	if loaded.DescriptorRevision != 2 {
		t.Errorf("Expected revision 2, got %d", loaded.DescriptorRevision)
	}

	// Verify no temp file left behind
	tempFile := filepath.Join(tempDir, stateFile+".tmp")
	if _, err := os.Stat(tempFile); err == nil {
		t.Error("Temp file should not exist after atomic write")
	}
}
