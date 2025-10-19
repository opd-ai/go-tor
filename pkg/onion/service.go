// Package onion - Onion Service Server Implementation
// This file implements the server/hosting side of onion services (Phase 7.4)
package onion

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/opd-ai/go-tor/pkg/logger"
	"github.com/opd-ai/go-tor/pkg/security"
)

// Service represents an onion service (hidden service) that can be hosted
type Service struct {
	mu sync.RWMutex

	// Identity
	identityKey ed25519.PrivateKey // 64-byte Ed25519 private key
	publicKey   ed25519.PublicKey  // 32-byte Ed25519 public key
	address     *Address           // Derived .onion address

	// Configuration
	config *ServiceConfig

	// State
	descriptor       *Descriptor
	introPoints      []*ServiceIntroPoint
	publishedHSDirs  []*HSDirectory
	lastPublish      time.Time
	running          bool
	ctx              context.Context
	cancel           context.CancelFunc
	logger           *logger.Logger

	// Connections
	pendingIntros map[string]*PendingIntro // cookie -> intro
}

// ServiceConfig contains configuration for hosting an onion service
type ServiceConfig struct {
	// Service identity (if nil, generates new identity)
	PrivateKey ed25519.PrivateKey

	// Service ports (map virtual port -> local target)
	// e.g., 80 -> "localhost:8080"
	Ports map[int]string

	// Number of introduction points (default: 3, min: 1, max: 10)
	NumIntroPoints int

	// Descriptor lifetime (default: 3 hours)
	DescriptorLifetime time.Duration

	// Directory to store persistent state
	DataDirectory string
}

// ServiceIntroPoint represents an introduction point for this service
type ServiceIntroPoint struct {
	Relay      *HSDirectory // The relay acting as intro point
	CircuitID  uint32       // Circuit to the intro point
	AuthKey    []byte       // Authentication key for this intro point
	EncKey     []byte       // Encryption key for this intro point
	Established bool        // Whether ESTABLISH_INTRO succeeded
	CreatedAt  time.Time
}

// PendingIntro represents a pending introduction request
type PendingIntro struct {
	Cookie           []byte    // Rendezvous cookie
	RendezvousPoint  string    // Rendezvous point fingerprint
	ClientOnionKey   []byte    // Client's onion key
	ReceivedAt       time.Time
}

// NewService creates a new onion service
func NewService(config *ServiceConfig, log *logger.Logger) (*Service, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}

	if log == nil {
		log = logger.NewDefault()
	}

	// Generate or load identity key
	var privateKey ed25519.PrivateKey
	var publicKey ed25519.PublicKey

	if len(config.PrivateKey) > 0 {
		// Use provided key
		if len(config.PrivateKey) != ed25519.PrivateKeySize {
			return nil, fmt.Errorf("invalid private key size: %d, expected %d", 
				len(config.PrivateKey), ed25519.PrivateKeySize)
		}
		privateKey = config.PrivateKey
		publicKey = privateKey.Public().(ed25519.PublicKey)
	} else {
		// Generate new identity
		var err error
		publicKey, privateKey, err = ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("failed to generate identity key: %w", err)
		}
	}

	// Derive onion address from public key
	addr, err := addressFromPublicKey(publicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to derive address: %w", err)
	}

	// Set defaults
	if config.NumIntroPoints == 0 {
		config.NumIntroPoints = 3
	}
	if config.NumIntroPoints < 1 {
		config.NumIntroPoints = 1
	}
	if config.NumIntroPoints > 10 {
		config.NumIntroPoints = 10
	}

	if config.DescriptorLifetime == 0 {
		config.DescriptorLifetime = 3 * time.Hour
	}

	if config.Ports == nil {
		config.Ports = make(map[int]string)
	}

	ctx, cancel := context.WithCancel(context.Background())

	service := &Service{
		identityKey:     privateKey,
		publicKey:       publicKey,
		address:         addr,
		config:          config,
		introPoints:     make([]*ServiceIntroPoint, 0, config.NumIntroPoints),
		publishedHSDirs: make([]*HSDirectory, 0),
		pendingIntros:   make(map[string]*PendingIntro),
		ctx:             ctx,
		cancel:          cancel,
		logger:          log.Component("onion-service"),
	}

	return service, nil
}

// addressFromPublicKey derives a v3 onion address from an Ed25519 public key
func addressFromPublicKey(pubkey ed25519.PublicKey) (*Address, error) {
	if len(pubkey) != 32 {
		return nil, fmt.Errorf("invalid public key length: %d", len(pubkey))
	}

	// Compute checksum
	checksum := computeV3Checksum(pubkey, V3Version)

	// Construct: pubkey || checksum || version
	data := make([]byte, 0, V3PubkeyLen+V3ChecksumLen+1)
	data = append(data, pubkey...)
	data = append(data, checksum...)
	data = append(data, V3Version)

	// Encode to base32
	encoder := base32.StdEncoding.WithPadding(base32.NoPadding)
	encoded := strings.ToLower(encoder.EncodeToString(data))

	return &Address{
		Version: V3,
		Pubkey:  pubkey,
		Raw:     encoded + V3Suffix,
	}, nil
}

// GetAddress returns the onion address of this service
func (s *Service) GetAddress() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.address.String()
}

// Start starts the onion service
func (s *Service) Start(ctx context.Context, hsdirs []*HSDirectory) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("service already running")
	}
	s.running = true
	s.mu.Unlock()

	s.logger.Info("Starting onion service",
		"address", s.address.String(),
		"intro_points", s.config.NumIntroPoints)

	// Step 1: Select and establish introduction points
	if err := s.establishIntroductionPoints(ctx, hsdirs); err != nil {
		s.running = false
		return fmt.Errorf("failed to establish introduction points: %w", err)
	}

	// Step 2: Create descriptor
	if err := s.createDescriptor(); err != nil {
		s.running = false
		return fmt.Errorf("failed to create descriptor: %w", err)
	}

	// Step 3: Publish descriptor to HSDirs
	if err := s.publishDescriptor(ctx, hsdirs); err != nil {
		s.running = false
		return fmt.Errorf("failed to publish descriptor: %w", err)
	}

	// Step 4: Start background tasks
	go s.maintenanceLoop(ctx, hsdirs)

	s.logger.Info("Onion service started successfully",
		"address", s.address.String())

	return nil
}

// Stop stops the onion service
func (s *Service) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.logger.Info("Stopping onion service", "address", s.address.String())

	// Cancel context to stop background tasks
	s.cancel()

	// Clean up introduction points
	for _, intro := range s.introPoints {
		// In a full implementation, we would:
		// 1. Send INTRO_ESTABLISHED teardown
		// 2. Close circuits
		_ = intro
	}

	s.running = false
	s.logger.Info("Onion service stopped", "address", s.address.String())

	return nil
}

// establishIntroductionPoints selects and establishes circuits to introduction points
func (s *Service) establishIntroductionPoints(ctx context.Context, hsdirs []*HSDirectory) error {
	s.logger.Info("Establishing introduction points", "count", s.config.NumIntroPoints)

	if len(hsdirs) < s.config.NumIntroPoints {
		return fmt.Errorf("not enough relays available: need %d, have %d",
			s.config.NumIntroPoints, len(hsdirs))
	}

	// Select introduction points (use first N relays for Phase 7.4)
	// In production, we would:
	// 1. Filter relays with appropriate flags
	// 2. Randomly select from filtered set
	// 3. Ensure geographical and network diversity
	selectedRelays := hsdirs[:s.config.NumIntroPoints]

	for i, relay := range selectedRelays {
		intro, err := s.establishIntroductionPoint(ctx, relay)
		if err != nil {
			s.logger.Warn("Failed to establish introduction point",
				"relay", relay.Fingerprint,
				"error", err)
			continue
		}

		s.introPoints = append(s.introPoints, intro)
		s.logger.Debug("Introduction point established",
			"index", i,
			"relay", relay.Fingerprint,
			"circuit", intro.CircuitID)
	}

	if len(s.introPoints) == 0 {
		return fmt.Errorf("failed to establish any introduction points")
	}

	s.logger.Info("Introduction points established", "count", len(s.introPoints))
	return nil
}

// establishIntroductionPoint establishes a single introduction point
func (s *Service) establishIntroductionPoint(ctx context.Context, relay *HSDirectory) (*ServiceIntroPoint, error) {
	s.logger.Debug("Establishing introduction point", "relay", relay.Fingerprint)

	// Generate keys for this introduction point
	authKey := make([]byte, 32)
	if _, err := rand.Read(authKey); err != nil {
		return nil, fmt.Errorf("failed to generate auth key: %w", err)
	}

	encKey := make([]byte, 32)
	if _, err := rand.Read(encKey); err != nil {
		return nil, fmt.Errorf("failed to generate enc key: %w", err)
	}

	// In Phase 7.4, use mock circuit ID
	// In production, this would:
	// 1. Build a 3-hop circuit to the relay
	// 2. Send ESTABLISH_INTRO cell
	// 3. Wait for INTRO_ESTABLISHED acknowledgment
	circuitID := uint32(3000 + len(s.introPoints))

	intro := &ServiceIntroPoint{
		Relay:       relay,
		CircuitID:   circuitID,
		AuthKey:     authKey,
		EncKey:      encKey,
		Established: true, // Mock for Phase 7.4
		CreatedAt:   time.Now(),
	}

	s.logger.Debug("Introduction point circuit created",
		"relay", relay.Fingerprint,
		"circuit", circuitID)

	return intro, nil
}

// createDescriptor creates the onion service descriptor
func (s *Service) createDescriptor() error {
	s.logger.Debug("Creating service descriptor")

	// Calculate blinded public key for current time period
	timePeriod := GetTimePeriod(time.Now())
	blindedPubkey := ComputeBlindedPubkey(s.publicKey, timePeriod)
	descriptorID := computeDescriptorID(blindedPubkey)

	// Build introduction points list
	introPoints := make([]IntroductionPoint, 0, len(s.introPoints))
	for _, serviceIntro := range s.introPoints {
		// Convert relay to link specifiers
		linkSpecs := make([]LinkSpecifier, 0)
		// In production, would add IPv4, IPv6, and fingerprint link specifiers
		// For Phase 7.4, simplified

		intro := IntroductionPoint{
			LinkSpecifiers: linkSpecs,
			OnionKey:       make([]byte, 32), // Would be relay's ntor key
			AuthKey:        serviceIntro.AuthKey,
			EncKey:         serviceIntro.EncKey,
			EncKeyCert:     nil, // Would be cross-certification
			LegacyKeyID:    make([]byte, 20),
		}
		introPoints = append(introPoints, intro)
	}

	// Safe conversion of timestamp to uint64
	now := time.Now()
	revisionCounter, err := security.SafeUnixToUint64(now)
	if err != nil {
		// In case of error, use 0 as revision counter
		revisionCounter = 0
	}

	desc := &Descriptor{
		Version:         3,
		Address:         s.address,
		IntroPoints:     introPoints,
		DescriptorID:    descriptorID,
		BlindedPubkey:   blindedPubkey,
		RevisionCounter: revisionCounter,
		CreatedAt:       now,
		Lifetime:        s.config.DescriptorLifetime,
	}

	// Sign the descriptor
	if err := s.signDescriptor(desc); err != nil {
		return fmt.Errorf("failed to sign descriptor: %w", err)
	}

	s.mu.Lock()
	s.descriptor = desc
	s.mu.Unlock()

	s.logger.Info("Descriptor created",
		"descriptor_id", fmt.Sprintf("%x", descriptorID[:8]),
		"intro_points", len(introPoints),
		"lifetime", s.config.DescriptorLifetime)

	return nil
}

// signDescriptor signs the descriptor with the service's identity key
func (s *Service) signDescriptor(desc *Descriptor) error {
	// In production, the signing process would be:
	// 1. Create a descriptor signing key (short-term key)
	// 2. Create a certificate signing the signing key with the identity key
	// 3. Sign the descriptor with the signing key
	// For Phase 7.4, we'll do simplified signing with identity key directly

	// First encode without signature to get the content to sign
	encoded, err := EncodeDescriptor(desc)
	if err != nil {
		return fmt.Errorf("failed to encode descriptor: %w", err)
	}

	// Sign the descriptor content (everything before the signature line)
	// We need to sign everything up to where "signature " would appear
	signature := ed25519.Sign(s.identityKey, encoded)
	desc.Signature = signature

	// Now encode again with the signature to get the complete descriptor
	encoded, err = EncodeDescriptor(desc)
	if err != nil {
		return fmt.Errorf("failed to encode descriptor with signature: %w", err)
	}

	// Store the complete raw descriptor
	desc.RawDescriptor = encoded

	s.logger.Debug("Descriptor signed", "signature_len", len(signature))

	return nil
}

// publishDescriptor publishes the descriptor to responsible HSDirs
func (s *Service) publishDescriptor(ctx context.Context, hsdirs []*HSDirectory) error {
	s.logger.Info("Publishing descriptor to HSDirs")

	s.mu.RLock()
	desc := s.descriptor
	s.mu.RUnlock()

	if desc == nil {
		return fmt.Errorf("no descriptor to publish")
	}

	// Select responsible HSDirs using HSDir protocol
	hsdir := NewHSDir(s.logger)
	
	// Publish to both replicas
	published := 0
	for replica := 0; replica < 2; replica++ {
		selectedHSDirs := hsdir.SelectHSDirs(desc.DescriptorID, hsdirs, replica)
		
		for _, targetHSDir := range selectedHSDirs {
			if err := s.uploadDescriptor(ctx, targetHSDir, desc, replica); err != nil {
				s.logger.Warn("Failed to publish to HSDir",
					"hsdir", targetHSDir.Fingerprint,
					"replica", replica,
					"error", err)
				continue
			}
			published++
			s.logger.Debug("Descriptor published",
				"hsdir", targetHSDir.Fingerprint,
				"replica", replica)
		}
	}

	if published == 0 {
		return fmt.Errorf("failed to publish descriptor to any HSDir")
	}

	s.mu.Lock()
	s.lastPublish = time.Now()
	s.mu.Unlock()

	s.logger.Info("Descriptor published successfully",
		"hsdirs", published)

	return nil
}

// uploadDescriptor uploads a descriptor to a specific HSDir
func (s *Service) uploadDescriptor(ctx context.Context, hsdir *HSDirectory, desc *Descriptor, replica int) error {
	// In production, this would:
	// 1. Build a circuit to the HSDir
	// 2. Send an HTTP POST to /tor/hs/3/publish
	// 3. Wait for 200 OK response
	// 4. Handle retries and errors
	
	// For Phase 7.4, we'll simulate successful upload
	s.logger.Debug("Uploading descriptor to HSDir",
		"hsdir", hsdir.Fingerprint,
		"replica", replica,
		"descriptor_size", len(desc.RawDescriptor))

	// Simulate network delay
	select {
	case <-time.After(10 * time.Millisecond):
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}

// maintenanceLoop handles periodic tasks
func (s *Service) maintenanceLoop(ctx context.Context, hsdirs []*HSDirectory) {
	// Refresh descriptor every hour or 2/3 of lifetime, whichever is shorter
	refreshInterval := s.config.DescriptorLifetime * 2 / 3
	if refreshInterval > time.Hour {
		refreshInterval = time.Hour
	}

	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.logger.Debug("Running maintenance tasks")
			
			// Re-publish descriptor
			if err := s.createDescriptor(); err != nil {
				s.logger.Error("Failed to refresh descriptor", "error", err)
			} else if err := s.publishDescriptor(ctx, hsdirs); err != nil {
				s.logger.Error("Failed to re-publish descriptor", "error", err)
			} else {
				s.logger.Info("Descriptor refreshed and re-published")
			}
		}
	}
}

// HandleIntroduce2 handles an INTRODUCE2 cell from an introduction point
func (s *Service) HandleIntroduce2(introCircuitID uint32, introduce2Data []byte) error {
	s.logger.Info("Received INTRODUCE2 cell",
		"circuit", introCircuitID,
		"size", len(introduce2Data))

	// Parse INTRODUCE2 cell
	// Format: RENDEZVOUS_COOKIE (20 bytes) || CLIENT_ONION_KEY (32 bytes) || LINK_SPECIFIERS || ...
	if len(introduce2Data) < 52 {
		return fmt.Errorf("INTRODUCE2 data too short: %d bytes", len(introduce2Data))
	}

	rendezvousCookie := introduce2Data[0:20]
	clientOnionKey := introduce2Data[20:52]
	// Link specifiers would follow, but we'll simplify for Phase 7.4

	cookieStr := fmt.Sprintf("%x", rendezvousCookie)

	// Store pending introduction
	s.mu.Lock()
	s.pendingIntros[cookieStr] = &PendingIntro{
		Cookie:         rendezvousCookie,
		ClientOnionKey: clientOnionKey,
		ReceivedAt:     time.Now(),
	}
	s.mu.Unlock()

	s.logger.Debug("INTRODUCE2 parsed and stored",
		"cookie", cookieStr[:16])

	// In production, we would now:
	// 1. Build a circuit to the rendezvous point
	// 2. Send RENDEZVOUS1 with our handshake response
	// 3. Complete the connection

	return nil
}

// GetStats returns statistics about the service
func (s *Service) GetStats() ServiceStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return ServiceStats{
		Address:          s.address.String(),
		Running:          s.running,
		IntroPoints:      len(s.introPoints),
		DescriptorAge:    time.Since(s.lastPublish),
		PendingIntros:    len(s.pendingIntros),
		PublishedHSDirs:  len(s.publishedHSDirs),
	}
}

// ServiceStats contains statistics about a running service
type ServiceStats struct {
	Address         string
	Running         bool
	IntroPoints     int
	DescriptorAge   time.Duration
	PendingIntros   int
	PublishedHSDirs int
}
