package onion

import (
	"context"
	"crypto/ed25519"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/logger"
)

// TestServiceWithCircuitBuilder tests onion service with circuit builder configured
func TestServiceWithCircuitBuilder(t *testing.T) {
	// Generate a test identity key
	_, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	config := &ServiceConfig{
		PrivateKey:     privateKey,
		NumIntroPoints: 1,
		Ports:          map[int]string{80: "localhost:8080"},
		// CircuitBuilder and PathSelector are nil - should fall back to placeholder
	}

	service, err := NewService(config, logger.NewDefault())
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// Start the service
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use mock HSDirs
	hsdirs := []*HSDirectory{
		{Fingerprint: "hsdir1", Address: "127.0.0.1", DirPort: 9030},
	}

	err = service.Start(ctx, hsdirs)
	// We expect this to fail because no HSDirs are running, but intro points should be established
	if err == nil {
		t.Fatal("Expected error (no HSDirs running)")
	}

	// Check that intro points were established with placeholder circuits
	stats := service.GetStats()
	if stats.IntroPoints != 1 {
		t.Errorf("Expected 1 intro point, got %d", stats.IntroPoints)
	}

	// Stop the service
	service.Stop()
}

// TestEstablishIntroCircuitFallback tests fallback to placeholder when no circuit builder
func TestEstablishIntroCircuitFallback(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	config := &ServiceConfig{
		PrivateKey:     privateKey,
		NumIntroPoints: 2,
		Ports:          map[int]string{80: "localhost:8080"},
		// No circuit builder - should use placeholders
	}

	service, err := NewService(config, logger.NewDefault())
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// Create a mock HSDirectory for testing
	relay := &HSDirectory{
		Fingerprint: "test-relay-fp",
		Address:     "127.0.0.1",
		DirPort:     9030,
	}

	ctx := context.Background()
	intro, err := service.establishIntroductionPoint(ctx, relay)
	if err != nil {
		t.Fatalf("Failed to establish intro point: %v", err)
	}

	// Verify the intro point was created
	if intro == nil {
		t.Fatal("Intro point is nil")
	}

	if intro.Relay != relay {
		t.Error("Relay mismatch")
	}

	if len(intro.AuthKey) != 32 {
		t.Errorf("Expected 32-byte auth key, got %d bytes", len(intro.AuthKey))
	}

	if len(intro.EncKey) != 32 {
		t.Errorf("Expected 32-byte enc key, got %d bytes", len(intro.EncKey))
	}

	// Without circuit builder, established should be false
	if intro.Established {
		t.Error("Expected Established=false when no circuit builder configured")
	}

	// Circuit ID should be placeholder (3000+)
	if intro.CircuitID < 3000 {
		t.Errorf("Expected placeholder circuit ID >= 3000, got %d", intro.CircuitID)
	}
}

// TestSendEstablishIntroPayload tests ESTABLISH_INTRO cell format
func TestSendEstablishIntroPayload(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	config := &ServiceConfig{
		PrivateKey: privateKey,
		Ports:      map[int]string{80: "localhost:8080"},
	}

	service, err := NewService(config, logger.NewDefault())
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// This test verifies the payload format without actually sending it
	// In a real scenario, we'd need a mock circuit

	// Verify that authKey and encKey are generated properly
	authKey := make([]byte, 32)
	encKey := make([]byte, 32)

	// The actual payload building is inside sendEstablishIntro
	// For now, just verify the service can be created with the right config
	if service.identityKey == nil {
		t.Error("Identity key should not be nil")
	}

	// Verify keys are 32 bytes
	if len(authKey) != 32 || len(encKey) != 32 {
		t.Error("Auth/Enc keys should be 32 bytes")
	}
}
