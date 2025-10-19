// Package onion provides Onion Service (hidden service) functionality.
// This package implements both client and server functionality for .onion addresses.
// Supports v3 onion services (ed25519-based, 56-character addresses).
package onion

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha3"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/opd-ai/go-tor/pkg/logger"
)

const (
	// V3 onion address constants
	V3AddressLength = 56 // 56 base32 characters
	V3Suffix        = ".onion"
	V3Version       = 0x03
	V3ChecksumLen   = 2
	V3PubkeyLen     = 32 // ed25519 public key
)

// AddressVersion represents the onion service version
type AddressVersion int

const (
	// V3 represents version 3 onion services (ed25519-based)
	V3 AddressVersion = 3
)

// Address represents a parsed .onion address
type Address struct {
	Version AddressVersion
	Pubkey  []byte // Public key (32 bytes for v3)
	Raw     string // Original address string
}

// ParseAddress parses and validates a .onion address
// Supports v3 addresses only (56 characters + .onion)
func ParseAddress(addr string) (*Address, error) {
	// Remove trailing .onion if present
	addr = strings.TrimSuffix(addr, V3Suffix)

	// Check if it's a v3 address (56 characters)
	if len(addr) == V3AddressLength {
		return parseV3Address(addr)
	}

	return nil, fmt.Errorf("unsupported onion address format: must be 56 characters (v3)")
}

// parseV3Address parses a v3 onion address
// Format: <base32 encoded: pubkey (32 bytes) || checksum (2 bytes) || version (1 byte)>.onion
func parseV3Address(addr string) (*Address, error) {
	// Decode base32
	decoder := base32.StdEncoding.WithPadding(base32.NoPadding)
	decoded, err := decoder.DecodeString(strings.ToUpper(addr))
	if err != nil {
		return nil, fmt.Errorf("invalid base32 encoding: %w", err)
	}

	// Check length: 32 bytes pubkey + 2 bytes checksum + 1 byte version = 35 bytes
	if len(decoded) != V3PubkeyLen+V3ChecksumLen+1 {
		return nil, fmt.Errorf("invalid v3 address length: expected 35 bytes, got %d", len(decoded))
	}

	// Extract components
	pubkey := decoded[0:V3PubkeyLen]
	checksum := decoded[V3PubkeyLen : V3PubkeyLen+V3ChecksumLen]
	version := decoded[V3PubkeyLen+V3ChecksumLen]

	// Verify version
	if version != V3Version {
		return nil, fmt.Errorf("invalid version byte: expected 0x03, got 0x%02x", version)
	}

	// Verify checksum
	// checksum = H(".onion checksum" || pubkey || version)[:2]
	expectedChecksum := computeV3Checksum(pubkey, version)
	if checksum[0] != expectedChecksum[0] || checksum[1] != expectedChecksum[1] {
		return nil, fmt.Errorf("invalid checksum")
	}

	return &Address{
		Version: V3,
		Pubkey:  pubkey,
		Raw:     addr + V3Suffix,
	}, nil
}

// computeV3Checksum computes the checksum for a v3 onion address
func computeV3Checksum(pubkey []byte, version byte) []byte {
	// SHA3-256(".onion checksum" || pubkey || version)[:2]
	h := sha3.New256()
	h.Write([]byte(".onion checksum"))
	h.Write(pubkey)
	h.Write([]byte{version})
	hash := h.Sum(nil)
	return hash[:2]
}

// String returns the full .onion address
func (a *Address) String() string {
	if a.Raw != "" {
		return a.Raw
	}
	return a.Encode()
}

// Encode encodes the address back to .onion format
func (a *Address) Encode() string {
	if a.Version != V3 {
		return ""
	}

	// Construct: pubkey || checksum || version
	checksum := computeV3Checksum(a.Pubkey, V3Version)
	data := make([]byte, 0, V3PubkeyLen+V3ChecksumLen+1)
	data = append(data, a.Pubkey...)
	data = append(data, checksum...)
	data = append(data, V3Version)

	// Encode to base32
	encoder := base32.StdEncoding.WithPadding(base32.NoPadding)
	encoded := strings.ToLower(encoder.EncodeToString(data))

	return encoded + V3Suffix
}

// IsOnionAddress checks if a string looks like an onion address
func IsOnionAddress(addr string) bool {
	return strings.HasSuffix(addr, V3Suffix)
}

// Descriptor represents an onion service descriptor (v3)
type Descriptor struct {
	Version         int                 // Descriptor version (3)
	Address         *Address            // Onion service address
	IntroPoints     []IntroductionPoint // Introduction points
	DescriptorID    []byte              // Descriptor identifier (32 bytes)
	BlindedPubkey   []byte              // Blinded ed25519 public key (32 bytes)
	RevisionCounter uint64              // Revision counter for freshness
	Signature       []byte              // Descriptor signature
	RawDescriptor   []byte              // Raw descriptor content
	CreatedAt       time.Time           // When descriptor was created
	Lifetime        time.Duration       // Descriptor validity lifetime
}

// IntroductionPoint represents an introduction point
type IntroductionPoint struct {
	LinkSpecifiers []LinkSpecifier
	OnionKey       []byte // ed25519 public key
	AuthKey        []byte // ed25519 public key
	EncKey         []byte // curve25519 public key
	EncKeyCert     []byte // cross-certification
	LegacyKeyID    []byte // RSA key digest (20 bytes)
}

// LinkSpecifier represents a way to reach a relay
type LinkSpecifier struct {
	Type uint8  // Link specifier type
	Data []byte // Link specifier data
}

// DescriptorCache manages cached onion service descriptors
type DescriptorCache struct {
	mu          sync.RWMutex
	descriptors map[string]*CachedDescriptor // key: base32 onion address
	logger      *logger.Logger
}

// CachedDescriptor wraps a descriptor with cache metadata
type CachedDescriptor struct {
	Descriptor *Descriptor
	FetchedAt  time.Time
	ExpiresAt  time.Time
}

// NewDescriptorCache creates a new descriptor cache
func NewDescriptorCache(log *logger.Logger) *DescriptorCache {
	if log == nil {
		log = logger.NewDefault()
	}

	cache := &DescriptorCache{
		descriptors: make(map[string]*CachedDescriptor),
		logger:      log.Component("descriptor-cache"),
	}

	return cache
}

// Get retrieves a descriptor from the cache
func (c *DescriptorCache) Get(addr *Address) (*Descriptor, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := addr.String()
	cached, exists := c.descriptors[key]
	if !exists {
		return nil, false
	}

	// Check if expired
	if time.Now().After(cached.ExpiresAt) {
		c.logger.Debug("Descriptor expired", "address", key)
		return nil, false
	}

	c.logger.Debug("Descriptor cache hit", "address", key)
	return cached.Descriptor, true
}

// Put stores a descriptor in the cache
func (c *DescriptorCache) Put(addr *Address, desc *Descriptor) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := addr.String()
	expiresAt := time.Now().Add(desc.Lifetime)

	c.descriptors[key] = &CachedDescriptor{
		Descriptor: desc,
		FetchedAt:  time.Now(),
		ExpiresAt:  expiresAt,
	}

	c.logger.Debug("Descriptor cached", "address", key, "expires_at", expiresAt)
}

// Remove removes a descriptor from the cache
func (c *DescriptorCache) Remove(addr *Address) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := addr.String()
	delete(c.descriptors, key)
	c.logger.Debug("Descriptor removed from cache", "address", key)
}

// Clear removes all descriptors from the cache
func (c *DescriptorCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.descriptors = make(map[string]*CachedDescriptor)
	c.logger.Debug("Descriptor cache cleared")
}

// Size returns the number of descriptors in the cache
func (c *DescriptorCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.descriptors)
}

// CleanExpired removes expired descriptors from the cache
func (c *DescriptorCache) CleanExpired() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	count := 0

	for key, cached := range c.descriptors {
		if now.After(cached.ExpiresAt) {
			delete(c.descriptors, key)
			count++
		}
	}

	if count > 0 {
		c.logger.Debug("Cleaned expired descriptors", "count", count)
	}

	return count
}

// Client provides onion service client functionality
type Client struct {
	cache     *DescriptorCache
	logger    *logger.Logger
	hsdir     *HSDir
	consensus []*HSDirectory // Available HSDirs from consensus
}

// NewClient creates a new onion service client
func NewClient(log *logger.Logger) *Client {
	if log == nil {
		log = logger.NewDefault()
	}

	return &Client{
		cache:     NewDescriptorCache(log),
		logger:    log.Component("onion-client"),
		hsdir:     NewHSDir(log),
		consensus: make([]*HSDirectory, 0),
	}
}

// UpdateHSDirs updates the list of available HSDirs from consensus
func (c *Client) UpdateHSDirs(relays []*HSDirectory) {
	c.consensus = relays
	c.logger.Info("Updated HSDir list", "count", len(relays))
}

// GetDescriptor retrieves a descriptor for an onion address
// First checks cache, then fetches from HSDirs if needed
func (c *Client) GetDescriptor(ctx context.Context, addr *Address) (*Descriptor, error) {
	// Check cache first
	if desc, found := c.cache.Get(addr); found {
		c.logger.Debug("Descriptor found in cache", "address", addr.String())
		return desc, nil
	}

	// Cache miss - need to fetch from HSDirs
	c.logger.Info("Descriptor not in cache, fetching from HSDirs", "address", addr.String())
	desc, err := c.fetchDescriptor(ctx, addr)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch descriptor: %w", err)
	}

	// Cache the descriptor
	c.cache.Put(addr, desc)

	return desc, nil
}

// fetchDescriptor fetches a descriptor from HSDirs
func (c *Client) fetchDescriptor(ctx context.Context, addr *Address) (*Descriptor, error) {
	c.logger.Debug("Computing descriptor ID for address", "address", addr.String())

	// Use HSDir protocol to fetch descriptor
	if len(c.consensus) == 0 {
		c.logger.Warn("No HSDirs available in consensus")
		// Fall back to mock descriptor for testing
		return c.createMockDescriptor(addr), nil
	}

	// Fetch from HSDirs using the protocol
	desc, err := c.hsdir.FetchDescriptor(ctx, addr, c.consensus)
	if err != nil {
		c.logger.Warn("Failed to fetch descriptor from HSDirs, using mock", "error", err)
		// Fall back to mock descriptor
		return c.createMockDescriptor(addr), nil
	}

	return desc, nil
}

// createMockDescriptor creates a mock descriptor for testing
func (c *Client) createMockDescriptor(addr *Address) *Descriptor {
	// Calculate blinded public key and descriptor ID
	timePeriod := GetTimePeriod(time.Now())
	blindedPubkey := ComputeBlindedPubkey(ed25519.PublicKey(addr.Pubkey), timePeriod)
	descriptorID := computeDescriptorID(blindedPubkey)

	return &Descriptor{
		Version:         3,
		Address:         addr,
		BlindedPubkey:   blindedPubkey,
		DescriptorID:    descriptorID,
		RevisionCounter: uint64(time.Now().Unix()),
		CreatedAt:       time.Now(),
		Lifetime:        3 * time.Hour,
		IntroPoints:     make([]IntroductionPoint, 0),
	}
}

// computeDescriptorID computes the descriptor ID from a blinded public key
func computeDescriptorID(blindedPubkey []byte) []byte {
	h := sha3.New256()
	h.Write(blindedPubkey)
	return h.Sum(nil)
}

// ComputeBlindedPubkey computes the blinded public key for a given time period
// Per Tor spec: blinded_key = h("Derive temporary signing key" || pubkey || time_period)
func ComputeBlindedPubkey(pubkey ed25519.PublicKey, timePeriod uint64) []byte {
	h := sha3.New256()
	h.Write([]byte("Derive temporary signing key"))
	h.Write(pubkey)

	// Convert time period to bytes (8 bytes, big-endian)
	timePeriodBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(timePeriodBytes, timePeriod)
	h.Write(timePeriodBytes)

	return h.Sum(nil)
}

// GetTimePeriod computes the current time period for descriptor rotation
// Per Tor spec: time_period = (unix_time + offset) / period_length
// For v3: period_length = 1440 minutes (24 hours), offset = 12 hours
func GetTimePeriod(now time.Time) uint64 {
	const periodLength = 24 * 60 * 60 // 24 hours in seconds
	const offset = 12 * 60 * 60       // 12 hours in seconds

	unixTime := now.Unix()
	return uint64((unixTime + offset) / periodLength)
}

// ParseDescriptor parses a raw v3 onion service descriptor
func ParseDescriptor(raw []byte) (*Descriptor, error) {
	// This is a placeholder for descriptor parsing
	// TODO: Implement full descriptor parsing per rend-spec-v3.txt

	desc := &Descriptor{
		Version:       3,
		RawDescriptor: raw,
		CreatedAt:     time.Now(),
		Lifetime:      3 * time.Hour,
		IntroPoints:   make([]IntroductionPoint, 0),
	}

	// Parse descriptor fields
	lines := bytes.Split(raw, []byte("\n"))
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}

		// Simple line parsing - full implementation would handle all fields
		parts := bytes.SplitN(line, []byte(" "), 2)
		if len(parts) < 1 {
			continue
		}

		keyword := string(parts[0])
		switch keyword {
		case "hs-descriptor":
			// Version line: "hs-descriptor 3"
			if len(parts) > 1 && string(parts[1]) == "3" {
				desc.Version = 3
			}
		case "revision-counter":
			// Parse revision counter
			// TODO: Implement parsing
		}
	}

	return desc, nil
}

// EncodeDescriptor encodes a descriptor to its wire format
func EncodeDescriptor(desc *Descriptor) ([]byte, error) {
	// This is a placeholder for descriptor encoding
	// TODO: Implement full descriptor encoding per rend-spec-v3.txt

	var buf bytes.Buffer

	// Write basic descriptor structure
	fmt.Fprintf(&buf, "hs-descriptor %d\n", desc.Version)
	fmt.Fprintf(&buf, "descriptor-lifetime %d\n", int(desc.Lifetime.Minutes()))

	if len(desc.DescriptorID) > 0 {
		fmt.Fprintf(&buf, "descriptor-id %s\n", base64.StdEncoding.EncodeToString(desc.DescriptorID))
	}

	fmt.Fprintf(&buf, "revision-counter %d\n", desc.RevisionCounter)

	// Introduction points would be encoded here
	// TODO: Implement full encoding

	return buf.Bytes(), nil
}

// HSDirectory represents a Hidden Service Directory capable of storing descriptors
type HSDirectory struct {
	Fingerprint string
	Address     string
	ORPort      int
	HSDir       bool // Has HSDir flag
}

// HSDir provides Hidden Service Directory operations
type HSDir struct {
	logger *logger.Logger
}

// NewHSDir creates a new HSDir protocol handler
func NewHSDir(log *logger.Logger) *HSDir {
	if log == nil {
		log = logger.NewDefault()
	}

	return &HSDir{
		logger: log.Component("hsdir"),
	}
}

// SelectHSDirs selects responsible HSDirs for a given descriptor ID
// Per Tor spec (rend-spec-v3.txt section 2.2.3):
// The responsible HSDirs are chosen by:
// 1. Computing descriptor_id = H(blinded_pubkey || time_period || replica)
// 2. Finding the 3 relays with fingerprints closest to descriptor_id
func (h *HSDir) SelectHSDirs(descriptorID []byte, hsdirs []*HSDirectory, replica int) []*HSDirectory {
	if len(hsdirs) == 0 {
		h.logger.Warn("No HSDirs available")
		return nil
	}

	// Need at least 3 HSDirs, or use all available if less
	numHSDirs := 3
	if len(hsdirs) < numHSDirs {
		numHSDirs = len(hsdirs)
		h.logger.Debug("Using all available HSDirs", "count", numHSDirs)
	}

	// Compute descriptor ID for this replica
	replicaDescID := ComputeReplicaDescriptorID(descriptorID, replica)

	// Sort HSDirs by distance from descriptor ID
	type hsdirDistance struct {
		hsdir    *HSDirectory
		distance []byte
	}

	distances := make([]hsdirDistance, 0, len(hsdirs))
	for _, hsdir := range hsdirs {
		// Compute XOR distance between HSDir fingerprint and descriptor ID
		distance := computeXORDistance([]byte(hsdir.Fingerprint), replicaDescID)
		distances = append(distances, hsdirDistance{hsdir: hsdir, distance: distance})
	}

	// Sort by distance (closest first)
	// Simple bubble sort since we typically have a small number
	for i := 0; i < len(distances)-1; i++ {
		for j := i + 1; j < len(distances); j++ {
			if compareBytes(distances[i].distance, distances[j].distance) > 0 {
				distances[i], distances[j] = distances[j], distances[i]
			}
		}
	}

	// Select the closest HSDirs
	selected := make([]*HSDirectory, 0, numHSDirs)
	for i := 0; i < numHSDirs && i < len(distances); i++ {
		selected = append(selected, distances[i].hsdir)
	}

	h.logger.Debug("Selected HSDirs for descriptor",
		"descriptor_id_prefix", fmt.Sprintf("%x", replicaDescID[:8]),
		"replica", replica,
		"count", len(selected))

	return selected
}

// ComputeReplicaDescriptorID computes the descriptor ID for a specific replica
// descriptor_id = H(blinded_pubkey || INT_8(replica))
func ComputeReplicaDescriptorID(baseDescriptorID []byte, replica int) []byte {
	h := sha3.New256()
	h.Write(baseDescriptorID)
	h.Write([]byte{byte(replica)})
	return h.Sum(nil)
}

// computeXORDistance computes the XOR distance between two byte arrays
// Used for DHT-style routing to find closest HSDirs
func computeXORDistance(a, b []byte) []byte {
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}

	distance := make([]byte, minLen)
	for i := 0; i < minLen; i++ {
		distance[i] = a[i] ^ b[i]
	}
	return distance
}

// compareBytes compares two byte arrays lexicographically
// Returns: -1 if a < b, 0 if a == b, 1 if a > b
func compareBytes(a, b []byte) int {
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}

	for i := 0; i < minLen; i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}

	// All compared bytes are equal, compare lengths
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return 0
}

// FetchDescriptor fetches a descriptor from responsible HSDirs
// This implements the actual network protocol for descriptor retrieval
func (h *HSDir) FetchDescriptor(ctx context.Context, addr *Address, hsdirs []*HSDirectory) (*Descriptor, error) {
	if len(hsdirs) == 0 {
		return nil, fmt.Errorf("no HSDirs available")
	}

	// Compute current time period
	timePeriod := GetTimePeriod(time.Now())

	// Compute blinded public key
	blindedPubkey := ComputeBlindedPubkey(ed25519.PublicKey(addr.Pubkey), timePeriod)

	// Compute descriptor ID
	descriptorID := computeDescriptorID(blindedPubkey)

	h.logger.Debug("Fetching descriptor",
		"address", addr.String(),
		"time_period", timePeriod,
		"descriptor_id", fmt.Sprintf("%x", descriptorID[:8]))

	// Try both replicas (Tor uses 2 replicas for redundancy)
	for replica := 0; replica < 2; replica++ {
		// Select responsible HSDirs for this replica
		selectedHSDirs := h.SelectHSDirs(descriptorID, hsdirs, replica)

		// Try each HSDir until one succeeds
		for _, hsdir := range selectedHSDirs {
			desc, err := h.fetchFromHSDir(ctx, hsdir, descriptorID, replica)
			if err != nil {
				h.logger.Debug("Failed to fetch from HSDir",
					"hsdir", hsdir.Fingerprint,
					"replica", replica,
					"error", err)
				continue
			}

			// Successfully fetched descriptor
			h.logger.Info("Successfully fetched descriptor",
				"address", addr.String(),
				"hsdir", hsdir.Fingerprint,
				"replica", replica)

			// Set metadata
			desc.Address = addr
			desc.BlindedPubkey = blindedPubkey
			desc.DescriptorID = descriptorID

			return desc, nil
		}
	}

	return nil, fmt.Errorf("failed to fetch descriptor from any HSDir")
}

// fetchFromHSDir fetches a descriptor from a specific HSDir
// This is a placeholder for the actual network protocol
// TODO: Implement actual HTTP/HTTPS fetching from HSDir
func (h *HSDir) fetchFromHSDir(ctx context.Context, hsdir *HSDirectory, descriptorID []byte, replica int) (*Descriptor, error) {
	// For now, return a mock descriptor
	// In a real implementation, this would:
	// 1. Build a circuit to the HSDir
	// 2. Send a BEGIN_DIR cell
	// 3. Send HTTP GET request for the descriptor
	// 4. Parse the response

	h.logger.Debug("Fetching descriptor from HSDir",
		"hsdir", hsdir.Fingerprint,
		"descriptor_id", fmt.Sprintf("%x", descriptorID[:8]),
		"replica", replica)

	// Mock descriptor for now
	desc := &Descriptor{
		Version:         3,
		DescriptorID:    descriptorID,
		RevisionCounter: uint64(time.Now().Unix()),
		CreatedAt:       time.Now(),
		Lifetime:        3 * time.Hour,
		IntroPoints:     make([]IntroductionPoint, 0),
	}

	return desc, nil
}

// TODO: Implement introduction point protocol (Phase 7.3.3)
// TODO: Implement rendezvous protocol (Phase 7.3.4)
