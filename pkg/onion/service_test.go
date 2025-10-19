package onion

import (
	"context"
	"crypto/ed25519"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/logger"
)

func TestNewService(t *testing.T) {
	tests := []struct {
		name        string
		config      *ServiceConfig
		expectError bool
	}{
		{
			name:        "nil config",
			config:      nil,
			expectError: true,
		},
		{
			name: "new identity generation",
			config: &ServiceConfig{
				Ports: map[int]string{
					80: "localhost:8080",
				},
			},
			expectError: false,
		},
		{
			name: "with invalid private key size",
			config: &ServiceConfig{
				PrivateKey: make([]byte, 32), // Wrong size, should be 64
				Ports: map[int]string{
					80: "localhost:8080",
				},
			},
			expectError: true,
		},
		{
			name: "custom intro points",
			config: &ServiceConfig{
				NumIntroPoints: 5,
				Ports: map[int]string{
					80: "localhost:8080",
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := logger.NewDefault()
			service, err := NewService(tt.config, log)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if service == nil {
				t.Fatal("service is nil")
			}

			// Check address is valid
			addr := service.GetAddress()
			if !IsOnionAddress(addr) {
				t.Errorf("invalid onion address: %s", addr)
			}

			// Check defaults
			if tt.config.NumIntroPoints == 0 && service.config.NumIntroPoints != 3 {
				t.Errorf("expected default 3 intro points, got %d", service.config.NumIntroPoints)
			}

			if tt.config.DescriptorLifetime == 0 && service.config.DescriptorLifetime != 3*time.Hour {
				t.Errorf("expected default 3h lifetime, got %v", service.config.DescriptorLifetime)
			}
		})
	}
}

func TestServiceWithValidKey(t *testing.T) {
	// Generate a valid Ed25519 key pair
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	config := &ServiceConfig{
		PrivateKey: privateKey,
		Ports: map[int]string{
			80: "localhost:8080",
		},
	}

	log := logger.NewDefault()
	service, err := NewService(config, log)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	// Verify the service uses the provided key
	if string(service.publicKey) != string(publicKey) {
		t.Error("service public key doesn't match provided key")
	}

	// Verify address is derived correctly
	addr := service.GetAddress()
	if !IsOnionAddress(addr) {
		t.Errorf("invalid onion address: %s", addr)
	}

	// Parse the address and verify public key
	parsedAddr, err := ParseAddress(addr)
	if err != nil {
		t.Fatalf("failed to parse generated address: %v", err)
	}

	if string(parsedAddr.Pubkey) != string(publicKey) {
		t.Error("address public key doesn't match service key")
	}
}

func TestAddressFromPublicKey(t *testing.T) {
	// Generate a test key
	publicKey, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	addr, err := addressFromPublicKey(publicKey)
	if err != nil {
		t.Fatalf("failed to derive address: %v", err)
	}

	if addr == nil {
		t.Fatal("address is nil")
	}

	// Verify address format
	if !IsOnionAddress(addr.String()) {
		t.Errorf("invalid address format: %s", addr.String())
	}

	// Verify we can parse it back
	parsedAddr, err := ParseAddress(addr.String())
	if err != nil {
		t.Fatalf("failed to parse derived address: %v", err)
	}

	if string(parsedAddr.Pubkey) != string(publicKey) {
		t.Error("parsed public key doesn't match original")
	}
}

func TestServiceStartStop(t *testing.T) {
	config := &ServiceConfig{
		NumIntroPoints: 2,
		Ports: map[int]string{
			80: "localhost:8080",
		},
	}

	log := logger.NewDefault()
	service, err := NewService(config, log)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	// Create mock HSDirs
	hsdirs := []*HSDirectory{
		{Fingerprint: "hsdir1", Address: "127.0.0.1", ORPort: 9001, DirPort: 9030, HSDir: true},
		{Fingerprint: "hsdir2", Address: "127.0.0.1", ORPort: 9002, DirPort: 9031, HSDir: true},
		{Fingerprint: "hsdir3", Address: "127.0.0.1", ORPort: 9003, DirPort: 9032, HSDir: true},
	}

	ctx := context.Background()

	// Start service
	if err := service.Start(ctx, hsdirs); err != nil {
		t.Fatalf("failed to start service: %v", err)
	}

	// Verify service is running
	stats := service.GetStats()
	if !stats.Running {
		t.Error("service should be running")
	}

	if stats.IntroPoints != 2 {
		t.Errorf("expected 2 intro points, got %d", stats.IntroPoints)
	}

	// Try to start again (should fail)
	if err := service.Start(ctx, hsdirs); err == nil {
		t.Error("expected error starting already running service")
	}

	// Stop service
	if err := service.Stop(); err != nil {
		t.Fatalf("failed to stop service: %v", err)
	}

	// Verify service is stopped
	stats = service.GetStats()
	if stats.Running {
		t.Error("service should not be running")
	}

	// Stop again (should be idempotent)
	if err := service.Stop(); err != nil {
		t.Errorf("stop should be idempotent: %v", err)
	}
}

func TestEstablishIntroductionPoints(t *testing.T) {
	config := &ServiceConfig{
		NumIntroPoints: 3,
		Ports: map[int]string{
			80: "localhost:8080",
		},
	}

	log := logger.NewDefault()
	service, err := NewService(config, log)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	hsdirs := []*HSDirectory{
		{Fingerprint: "relay1", Address: "127.0.0.1", ORPort: 9001, DirPort: 9030, HSDir: true},
		{Fingerprint: "relay2", Address: "127.0.0.1", ORPort: 9002, DirPort: 9031, HSDir: true},
		{Fingerprint: "relay3", Address: "127.0.0.1", ORPort: 9003, DirPort: 9032, HSDir: true},
		{Fingerprint: "relay4", Address: "127.0.0.1", ORPort: 9004, DirPort: 9033, HSDir: true},
	}

	ctx := context.Background()
	if err := service.establishIntroductionPoints(ctx, hsdirs); err != nil {
		t.Fatalf("failed to establish intro points: %v", err)
	}

	if len(service.introPoints) != 3 {
		t.Errorf("expected 3 intro points, got %d", len(service.introPoints))
	}

	// Verify each intro point has required fields
	for i, intro := range service.introPoints {
		if intro.Relay == nil {
			t.Errorf("intro point %d has nil relay", i)
		}
		if intro.CircuitID == 0 {
			t.Errorf("intro point %d has zero circuit ID", i)
		}
		if len(intro.AuthKey) != 32 {
			t.Errorf("intro point %d has invalid auth key length: %d", i, len(intro.AuthKey))
		}
		if len(intro.EncKey) != 32 {
			t.Errorf("intro point %d has invalid enc key length: %d", i, len(intro.EncKey))
		}
		if !intro.Established {
			t.Errorf("intro point %d not marked as established", i)
		}
	}
}

func TestEstablishIntroductionPointsInsufficientRelays(t *testing.T) {
	config := &ServiceConfig{
		NumIntroPoints: 5,
		Ports: map[int]string{
			80: "localhost:8080",
		},
	}

	log := logger.NewDefault()
	service, err := NewService(config, log)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	// Only 2 relays, but need 5
	hsdirs := []*HSDirectory{
		{Fingerprint: "relay1", Address: "127.0.0.1", ORPort: 9001, DirPort: 9030, HSDir: true},
		{Fingerprint: "relay2", Address: "127.0.0.1", ORPort: 9002, DirPort: 9031, HSDir: true},
	}

	ctx := context.Background()
	if err := service.establishIntroductionPoints(ctx, hsdirs); err == nil {
		t.Error("expected error with insufficient relays")
	}
}

func TestCreateDescriptor(t *testing.T) {
	config := &ServiceConfig{
		NumIntroPoints: 2,
		Ports: map[int]string{
			80: "localhost:8080",
		},
	}

	log := logger.NewDefault()
	service, err := NewService(config, log)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	// Setup some intro points first
	service.introPoints = []*ServiceIntroPoint{
		{
			Relay:       &HSDirectory{Fingerprint: "relay1"},
			CircuitID:   3001,
			AuthKey:     make([]byte, 32),
			EncKey:      make([]byte, 32),
			Established: true,
		},
		{
			Relay:       &HSDirectory{Fingerprint: "relay2"},
			CircuitID:   3002,
			AuthKey:     make([]byte, 32),
			EncKey:      make([]byte, 32),
			Established: true,
		},
	}

	// Create descriptor
	if err := service.createDescriptor(); err != nil {
		t.Fatalf("failed to create descriptor: %v", err)
	}

	// Verify descriptor
	service.mu.RLock()
	desc := service.descriptor
	service.mu.RUnlock()

	if desc == nil {
		t.Fatal("descriptor is nil")
	}

	if desc.Version != 3 {
		t.Errorf("expected version 3, got %d", desc.Version)
	}

	if len(desc.IntroPoints) != 2 {
		t.Errorf("expected 2 intro points in descriptor, got %d", len(desc.IntroPoints))
	}

	if len(desc.Signature) == 0 {
		t.Error("descriptor has no signature")
	}

	if len(desc.DescriptorID) != 32 {
		t.Errorf("invalid descriptor ID length: %d", len(desc.DescriptorID))
	}

	if desc.Address.String() != service.address.String() {
		t.Error("descriptor address doesn't match service address")
	}
}

func TestSignDescriptor(t *testing.T) {
	config := &ServiceConfig{
		Ports: map[int]string{
			80: "localhost:8080",
		},
	}

	log := logger.NewDefault()
	service, err := NewService(config, log)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	// Create a test descriptor
	desc := &Descriptor{
		Version:     3,
		Address:     service.address,
		IntroPoints: []IntroductionPoint{},
		CreatedAt:   time.Now(),
		Lifetime:    3 * time.Hour,
	}

	// Sign it
	if err := service.signDescriptor(desc); err != nil {
		t.Fatalf("failed to sign descriptor: %v", err)
	}

	// Verify signature exists
	if len(desc.Signature) != ed25519.SignatureSize {
		t.Errorf("invalid signature size: %d, expected %d", len(desc.Signature), ed25519.SignatureSize)
	}

	if len(desc.RawDescriptor) == 0 {
		t.Error("raw descriptor not set")
	}

	// Verify signature is valid
	if err := VerifyDescriptorSignature(desc, service.address); err != nil {
		t.Errorf("signature verification failed: %v", err)
	}
}

func TestPublishDescriptor(t *testing.T) {
	config := &ServiceConfig{
		NumIntroPoints: 2,
		Ports: map[int]string{
			80: "localhost:8080",
		},
	}

	log := logger.NewDefault()
	service, err := NewService(config, log)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	// Setup intro points and descriptor
	service.introPoints = []*ServiceIntroPoint{
		{
			Relay:       &HSDirectory{Fingerprint: "relay1"},
			CircuitID:   3001,
			AuthKey:     make([]byte, 32),
			EncKey:      make([]byte, 32),
			Established: true,
		},
	}

	if err := service.createDescriptor(); err != nil {
		t.Fatalf("failed to create descriptor: %v", err)
	}

	// Mock HSDirs
	hsdirs := []*HSDirectory{
		{Fingerprint: "hsdir1", Address: "127.0.0.1", ORPort: 9001, DirPort: 9030, HSDir: true},
		{Fingerprint: "hsdir2", Address: "127.0.0.1", ORPort: 9002, DirPort: 9031, HSDir: true},
		{Fingerprint: "hsdir3", Address: "127.0.0.1", ORPort: 9003, DirPort: 9032, HSDir: true},
		{Fingerprint: "hsdir4", Address: "127.0.0.1", ORPort: 9004, DirPort: 9033, HSDir: true},
	}

	ctx := context.Background()
	if err := service.publishDescriptor(ctx, hsdirs); err != nil {
		t.Fatalf("failed to publish descriptor: %v", err)
	}

	// Verify last publish time was set
	service.mu.RLock()
	lastPublish := service.lastPublish
	service.mu.RUnlock()

	if lastPublish.IsZero() {
		t.Error("last publish time not set")
	}
}

func TestHandleIntroduce2(t *testing.T) {
	config := &ServiceConfig{
		Ports: map[int]string{
			80: "localhost:8080",
		},
	}

	log := logger.NewDefault()
	service, err := NewService(config, log)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	// Create mock INTRODUCE2 data
	introduce2Data := make([]byte, 52)
	// Rendezvous cookie (20 bytes)
	copy(introduce2Data[0:20], []byte("test-cookie-12345678"))
	// Client onion key (32 bytes)
	copy(introduce2Data[20:52], make([]byte, 32))

	if err := service.HandleIntroduce2(3001, introduce2Data); err != nil {
		t.Fatalf("failed to handle INTRODUCE2: %v", err)
	}

	// Verify pending intro was stored
	service.mu.RLock()
	pending := len(service.pendingIntros)
	service.mu.RUnlock()

	if pending != 1 {
		t.Errorf("expected 1 pending intro, got %d", pending)
	}
}

func TestHandleIntroduce2InvalidData(t *testing.T) {
	config := &ServiceConfig{
		Ports: map[int]string{
			80: "localhost:8080",
		},
	}

	log := logger.NewDefault()
	service, err := NewService(config, log)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	// Too short data
	shortData := make([]byte, 10)
	if err := service.HandleIntroduce2(3001, shortData); err == nil {
		t.Error("expected error with short INTRODUCE2 data")
	}
}

func TestServiceGetStats(t *testing.T) {
	config := &ServiceConfig{
		NumIntroPoints: 2,
		Ports: map[int]string{
			80: "localhost:8080",
		},
	}

	log := logger.NewDefault()
	service, err := NewService(config, log)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	stats := service.GetStats()

	if stats.Address == "" {
		t.Error("stats address is empty")
	}

	if stats.Running {
		t.Error("service should not be running yet")
	}

	if stats.IntroPoints != 0 {
		t.Errorf("expected 0 intro points, got %d", stats.IntroPoints)
	}
}

func TestServiceIntroPointLimits(t *testing.T) {
	tests := []struct {
		name     string
		numIntro int
		expected int
	}{
		{"zero defaults to 3", 0, 3},
		{"negative defaults to 1", -5, 1},
		{"valid value", 5, 5},
		{"max capped at 10", 15, 10},
		{"min capped at 1", 0, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &ServiceConfig{
				NumIntroPoints: tt.numIntro,
				Ports: map[int]string{
					80: "localhost:8080",
				},
			}

			log := logger.NewDefault()
			service, err := NewService(config, log)
			if err != nil {
				t.Fatalf("failed to create service: %v", err)
			}

			if service.config.NumIntroPoints != tt.expected {
				t.Errorf("expected %d intro points, got %d", tt.expected, service.config.NumIntroPoints)
			}
		})
	}
}
