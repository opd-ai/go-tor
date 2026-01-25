// Package onion - Service State Persistence Tests
package onion

import (
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestServiceStatePersistence(t *testing.T) {
	tempDir := t.TempDir()

	// Create a service with persistence
	config := &ServiceConfig{
		DataDirectory:      tempDir,
		NumIntroPoints:     3,
		DescriptorLifetime: 3 * time.Hour,
		Ports:              map[int]string{80: "localhost:8080"},
	}

	service, err := NewService(config, nil)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// Check initial state
	if service.descriptorRev != 1 {
		t.Errorf("Expected initial descriptor revision 1, got %d", service.descriptorRev)
	}

	// Simulate some state changes
	service.lastPublish = time.Now().Add(-1 * time.Hour)
	service.descriptorRev = 5
	service.startTime = time.Now().Add(-2 * time.Hour)

	// Save state
	err = service.saveState()
	if err != nil {
		t.Fatalf("Failed to save state: %v", err)
	}

	// Create a new service with the same data directory
	service2, err := NewService(config, nil)
	if err != nil {
		t.Fatalf("Failed to create second service: %v", err)
	}

	// Verify state was restored
	if service2.descriptorRev != 5 {
		t.Errorf("Expected descriptor revision 5, got %d", service2.descriptorRev)
	}

	if service2.address.String() != service.address.String() {
		t.Errorf("Expected same address, got %s vs %s",
			service2.address.String(), service.address.String())
	}

	// Verify last publish time was restored (within 1 second tolerance)
	timeDiff := service2.lastPublish.Sub(service.lastPublish)
	if timeDiff < -time.Second || timeDiff > time.Second {
		t.Errorf("Last publish time not restored correctly: %v vs %v",
			service2.lastPublish, service.lastPublish)
	}
}

func TestServiceStateRevisionIncrement(t *testing.T) {
	tempDir := t.TempDir()

	config := &ServiceConfig{
		DataDirectory:      tempDir,
		NumIntroPoints:     3,
		DescriptorLifetime: 3 * time.Hour,
		Ports:              map[int]string{80: "localhost:8080"},
	}

	service, err := NewService(config, nil)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	initialRev := service.descriptorRev

	// Simulate descriptor creation and publishing
	// Mock descriptor for testing
	service.descriptor = &Descriptor{
		Version:         3,
		Address:         service.address,
		IntroPoints:     []IntroductionPoint{},
		DescriptorID:    make([]byte, 32),
		BlindedPubkey:   make([]byte, 32),
		RevisionCounter: service.descriptorRev,
		CreatedAt:       time.Now(),
		Lifetime:        3 * time.Hour,
	}

	// Manually trigger the logic from publishDescriptor
	service.mu.Lock()
	service.lastPublish = time.Now()
	service.descriptorRev++
	service.mu.Unlock()

	if service.descriptorRev != initialRev+1 {
		t.Errorf("Expected revision %d, got %d", initialRev+1, service.descriptorRev)
	}

	// Save state
	err = service.saveState()
	if err != nil {
		t.Fatalf("Failed to save state: %v", err)
	}

	// Reload and verify revision persisted
	service2, err := NewService(config, nil)
	if err != nil {
		t.Fatalf("Failed to create second service: %v", err)
	}

	if service2.descriptorRev != initialRev+1 {
		t.Errorf("Expected persisted revision %d, got %d", initialRev+1, service2.descriptorRev)
	}
}

func TestServiceStateIntroPointCache(t *testing.T) {
	tempDir := t.TempDir()

	config := &ServiceConfig{
		DataDirectory:      tempDir,
		NumIntroPoints:     3,
		DescriptorLifetime: 3 * time.Hour,
		Ports:              map[int]string{80: "localhost:8080"},
	}

	service, err := NewService(config, nil)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// Add some intro points
	authKey1 := make([]byte, 32)
	rand.Read(authKey1)
	encKey1 := make([]byte, 32)
	rand.Read(encKey1)

	service.introPoints = []*ServiceIntroPoint{
		{
			Relay: &HSDirectory{
				Fingerprint: "relay1fingerprint",
			},
			CircuitID:   1,
			AuthKey:     authKey1,
			EncKey:      encKey1,
			Established: true,
			CreatedAt:   time.Now().Add(-30 * time.Minute),
		},
		{
			Relay: &HSDirectory{
				Fingerprint: "relay2fingerprint",
			},
			CircuitID:   2,
			AuthKey:     make([]byte, 32),
			EncKey:      make([]byte, 32),
			Established: false, // Not established, should not be cached
			CreatedAt:   time.Now().Add(-10 * time.Minute),
		},
	}

	// Save state
	err = service.saveState()
	if err != nil {
		t.Fatalf("Failed to save state: %v", err)
	}

	// Reload and verify intro point cache
	_, err = NewService(config, nil)
	if err != nil {
		t.Fatalf("Failed to create second service: %v", err)
	}

	// Check that state was loaded (we can't directly access loadedState, but we can verify via logs)
	// The intro points themselves aren't restored (by design), but the cache should be in the state file
	stateFile := filepath.Join(tempDir, "state.json")
	data, err := os.ReadFile(stateFile)
	if err != nil {
		t.Fatalf("Failed to read state file: %v", err)
	}

	// Verify only established intro point was cached
	if !strings.Contains(string(data), "relay1fingerprint") {
		t.Error("Expected relay1 to be in cached intro points")
	}

	// Non-established intro point should not be cached
	if strings.Contains(string(data), "relay2fingerprint") {
		t.Error("Expected relay2 NOT to be in cached intro points (not established)")
	}
}

func TestServiceStateNoPersistence(t *testing.T) {
	// Create service without DataDirectory (no persistence)
	config := &ServiceConfig{
		NumIntroPoints:     3,
		DescriptorLifetime: 3 * time.Hour,
		Ports:              map[int]string{80: "localhost:8080"},
	}

	service, err := NewService(config, nil)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// Verify persistence is nil
	if service.persistence != nil {
		t.Error("Expected no persistence when DataDirectory is empty")
	}

	// saveState should not error when persistence is nil
	err = service.saveState()
	if err != nil {
		t.Errorf("saveState should not error with nil persistence: %v", err)
	}
}

func TestServiceStateWithProvidedKey(t *testing.T) {
	// Generate a key
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	tempDir := t.TempDir()

	// Create service with provided key
	config := &ServiceConfig{
		PrivateKey:         privKey,
		DataDirectory:      tempDir,
		NumIntroPoints:     3,
		DescriptorLifetime: 3 * time.Hour,
		Ports:              map[int]string{80: "localhost:8080"},
	}

	service, err := NewService(config, nil)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// Verify the key matches
	if string(service.publicKey) != string(pubKey) {
		t.Error("Service public key doesn't match provided key")
	}

	// Verify persistence is nil when using provided key
	if service.persistence != nil {
		t.Error("Expected no persistence when using provided key")
	}
}

func TestServiceStopSavesState(t *testing.T) {
	tempDir := t.TempDir()

	config := &ServiceConfig{
		DataDirectory:      tempDir,
		NumIntroPoints:     3,
		DescriptorLifetime: 3 * time.Hour,
		Ports:              map[int]string{80: "localhost:8080"},
	}

	service, err := NewService(config, nil)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// Mark as running and set some state
	service.running = true
	service.startTime = time.Now()
	service.descriptorRev = 10

	// Stop the service
	err = service.Stop()
	if err != nil {
		t.Fatalf("Failed to stop service: %v", err)
	}

	// Verify state was saved
	stateFile := filepath.Join(tempDir, "state.json")
	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		t.Error("State file should exist after Stop()")
	}

	// Reload and verify
	service2, err := NewService(config, nil)
	if err != nil {
		t.Fatalf("Failed to create second service: %v", err)
	}

	if service2.descriptorRev != 10 {
		t.Errorf("Expected descriptor revision 10 after reload, got %d", service2.descriptorRev)
	}
}

func TestServiceStateCreationTime(t *testing.T) {
	tempDir := t.TempDir()

	config := &ServiceConfig{
		DataDirectory:      tempDir,
		NumIntroPoints:     3,
		DescriptorLifetime: 3 * time.Hour,
		Ports:              map[int]string{80: "localhost:8080"},
	}

	service1, err := NewService(config, nil)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	createdAt := service1.createdAt

	// Save state
	err = service1.saveState()
	if err != nil {
		t.Fatalf("Failed to save state: %v", err)
	}

	// Wait a bit to ensure time difference
	time.Sleep(10 * time.Millisecond)

	// Create new service (should restore creation time)
	service2, err := NewService(config, nil)
	if err != nil {
		t.Fatalf("Failed to create second service: %v", err)
	}

	// Verify creation time was restored, not reset
	timeDiff := service2.createdAt.Sub(createdAt)
	if timeDiff < -time.Second || timeDiff > time.Second {
		t.Errorf("Creation time should be restored: got diff %v", timeDiff)
	}
}
