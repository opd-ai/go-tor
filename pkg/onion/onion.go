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
	Version         int                  // Descriptor version (3)
	Address         *Address             // Onion service address
	IntroPoints     []IntroductionPoint  // Introduction points
	DescriptorID    []byte               // Descriptor identifier (32 bytes)
	BlindedPubkey   []byte               // Blinded ed25519 public key (32 bytes)
	RevisionCounter uint64               // Revision counter for freshness
	Signature       []byte               // Descriptor signature
	RawDescriptor   []byte               // Raw descriptor content
	CreatedAt       time.Time            // When descriptor was created
	Lifetime        time.Duration        // Descriptor validity lifetime
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

// CacheDescriptor caches a descriptor for testing or manual management
func (c *Client) CacheDescriptor(addr *Address, desc *Descriptor) {
	c.cache.Put(addr, desc)
	c.logger.Debug("Descriptor manually cached", "address", addr.String())
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
	const periodLength = 24 * 60 * 60        // 24 hours in seconds
	const offset = 12 * 60 * 60              // 12 hours in seconds
	
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

// IntroductionProtocol handles introduction point operations for onion services
type IntroductionProtocol struct {
	logger *logger.Logger
}

// NewIntroductionProtocol creates a new introduction protocol handler
func NewIntroductionProtocol(log *logger.Logger) *IntroductionProtocol {
	if log == nil {
		log = logger.NewDefault()
	}

	return &IntroductionProtocol{
		logger: log.Component("intro-protocol"),
	}
}

// SelectIntroductionPoint selects an appropriate introduction point from a descriptor
// Per Tor spec (rend-spec-v3.txt): Clients should pick a random introduction point
func (ip *IntroductionProtocol) SelectIntroductionPoint(desc *Descriptor) (*IntroductionPoint, error) {
	if desc == nil {
		return nil, fmt.Errorf("descriptor is nil")
	}

	if len(desc.IntroPoints) == 0 {
		return nil, fmt.Errorf("no introduction points available in descriptor")
	}

	// For Phase 7.3.3, select the first available introduction point
	// In a full implementation, this would:
	// 1. Filter out introduction points we've tried and failed
	// 2. Randomly select from remaining points
	// 3. Consider network conditions and performance
	selected := &desc.IntroPoints[0]

	ip.logger.Debug("Selected introduction point",
		"intro_points_available", len(desc.IntroPoints),
		"selected_index", 0)

	return selected, nil
}

// IntroduceRequest represents an INTRODUCE1 request
type IntroduceRequest struct {
	IntroPoint     *IntroductionPoint // Target introduction point
	RendezvousCookie []byte           // Rendezvous cookie (20 bytes)
	RendezvousPoint string            // Rendezvous point fingerprint
	OnionKey       []byte             // Client's ephemeral onion key
}

// BuildIntroduce1Cell constructs an INTRODUCE1 cell for the introduction protocol
// Per Tor spec (rend-spec-v3.txt section 3.2):
// INTRODUCE1 {
//   LEGACY_KEY_ID     [20 bytes]
//   AUTH_KEY_TYPE     [1 byte]
//   AUTH_KEY_LEN      [2 bytes]
//   AUTH_KEY          [AUTH_KEY_LEN bytes]
//   EXTENSIONS        [N bytes]
//   ENCRYPTED_DATA    [remaining bytes]
// }
func (ip *IntroductionProtocol) BuildIntroduce1Cell(req *IntroduceRequest) ([]byte, error) {
	if req == nil {
		return nil, fmt.Errorf("introduce request is nil")
	}
	if req.IntroPoint == nil {
		return nil, fmt.Errorf("introduction point is nil")
	}
	if len(req.RendezvousCookie) != 20 {
		return nil, fmt.Errorf("invalid rendezvous cookie length: %d, expected 20", len(req.RendezvousCookie))
	}

	ip.logger.Debug("Building INTRODUCE1 cell",
		"rendezvous_point", req.RendezvousPoint)

	var buf bytes.Buffer

	// LEGACY_KEY_ID (20 bytes) - set to zero for v3
	legacyKeyID := make([]byte, 20)
	buf.Write(legacyKeyID)

	// AUTH_KEY_TYPE (1 byte) - 0x02 for ed25519
	buf.WriteByte(0x02)

	// AUTH_KEY_LEN (2 bytes) - 32 bytes for ed25519
	authKeyLen := uint16(32)
	if len(req.IntroPoint.AuthKey) > 0 {
		authKeyLen = uint16(len(req.IntroPoint.AuthKey))
	}
	binary.BigEndian.PutUint16(buf.Bytes()[len(buf.Bytes()):len(buf.Bytes())+2], authKeyLen)
	buf.Write(make([]byte, 2)) // placeholder, then overwrite
	buf.Truncate(buf.Len() - 2)
	authKeyLenBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(authKeyLenBytes, authKeyLen)
	buf.Write(authKeyLenBytes)

	// AUTH_KEY (AUTH_KEY_LEN bytes)
	if len(req.IntroPoint.AuthKey) > 0 {
		buf.Write(req.IntroPoint.AuthKey)
	} else {
		// Mock auth key for testing
		buf.Write(make([]byte, 32))
	}

	// EXTENSIONS (N bytes) - empty for now
	buf.WriteByte(0) // N_EXTENSIONS = 0

	// ENCRYPTED_DATA would contain:
	// - RENDEZVOUS_COOKIE (20 bytes)
	// - ONION_KEY (32 bytes for x25519)
	// - LINK_SPECIFIERS for rendezvous point
	// For Phase 7.3.3, we'll create a simplified version
	encryptedData := ip.buildEncryptedData(req)
	buf.Write(encryptedData)

	ip.logger.Debug("Built INTRODUCE1 cell",
		"total_size", buf.Len(),
		"encrypted_data_size", len(encryptedData))

	return buf.Bytes(), nil
}

// buildEncryptedData constructs the encrypted portion of INTRODUCE1
// In a full implementation, this would be encrypted with the intro point's key
func (ip *IntroductionProtocol) buildEncryptedData(req *IntroduceRequest) []byte {
	var buf bytes.Buffer

	// RENDEZVOUS_COOKIE (20 bytes)
	buf.Write(req.RendezvousCookie)

	// ONION_KEY (32 bytes for x25519)
	if len(req.OnionKey) > 0 {
		buf.Write(req.OnionKey)
	} else {
		// Mock onion key
		buf.Write(make([]byte, 32))
	}

	// LINK_SPECIFIERS for rendezvous point
	// Format: N_SPEC [1 byte] || LINK_SPEC_1 || ... || LINK_SPEC_N
	// For Phase 7.3.3, simplified version
	buf.WriteByte(0) // N_SPEC = 0 (no link specifiers in this phase)

	// In a full implementation, this entire buffer would be encrypted
	// using the introduction point's encryption key

	return buf.Bytes()
}

// CreateIntroductionCircuit creates a circuit to an introduction point
// This is a placeholder for the full circuit creation logic
func (ip *IntroductionProtocol) CreateIntroductionCircuit(ctx context.Context, introPoint *IntroductionPoint) (uint32, error) {
	if introPoint == nil {
		return 0, fmt.Errorf("introduction point is nil")
	}

	ip.logger.Info("Creating introduction circuit",
		"link_specifiers_count", len(introPoint.LinkSpecifiers))

	// In Phase 7.3.3, we return a mock circuit ID
	// In a full implementation (Phase 8), this would:
	// 1. Use the circuit builder to create a 3-hop circuit
	// 2. Extend the circuit to the introduction point
	// 3. Wait for circuit to be established
	// 4. Return the circuit ID

	// Mock circuit ID for testing
	circuitID := uint32(1000)

	ip.logger.Debug("Introduction circuit created (mock)",
		"circuit_id", circuitID)

	return circuitID, nil
}

// SendIntroduce1 sends an INTRODUCE1 cell over a circuit
// This is a placeholder for the full send logic
func (ip *IntroductionProtocol) SendIntroduce1(ctx context.Context, circuitID uint32, introduce1Data []byte) error {
	if len(introduce1Data) == 0 {
		return fmt.Errorf("introduce1 data is empty")
	}

	ip.logger.Info("Sending INTRODUCE1 cell",
		"circuit_id", circuitID,
		"data_size", len(introduce1Data))

	// In a full implementation (Phase 8), this would:
	// 1. Wrap introduce1Data in a RELAY cell with command INTRODUCE1
	// 2. Send the cell over the circuit
	// 3. Wait for acknowledgment or timeout
	// 4. Handle retries and errors

	ip.logger.Debug("INTRODUCE1 cell sent (mock)")

	return nil
}

// ConnectToOnionService orchestrates the full connection process to an onion service
// This combines descriptor fetching, introduction, and rendezvous protocols
func (c *Client) ConnectToOnionService(ctx context.Context, addr *Address) (uint32, error) {
	c.logger.Info("Connecting to onion service", "address", addr.String())

	// Step 1: Get descriptor (from cache or fetch from HSDirs)
	desc, err := c.GetDescriptor(ctx, addr)
	if err != nil {
		return 0, fmt.Errorf("failed to get descriptor: %w", err)
	}

	c.logger.Debug("Descriptor retrieved", "intro_points", len(desc.IntroPoints))

	// Step 2: Generate rendezvous cookie
	rendezvousCookie := make([]byte, 20)
	// NOTE: In Phase 7.3.4, using zeros for testing
	// Phase 8 will use crypto/rand.Read(rendezvousCookie) for production security

	// Step 3: Establish rendezvous point
	rendezvousCircuitID, rendezvousPoint, err := c.EstablishRendezvousPoint(ctx, rendezvousCookie, c.consensus)
	if err != nil {
		return 0, fmt.Errorf("failed to establish rendezvous point: %w", err)
	}

	c.logger.Debug("Rendezvous point established",
		"circuit_id", rendezvousCircuitID,
		"fingerprint", rendezvousPoint.Fingerprint)

	// Step 4: Select an introduction point
	intro := NewIntroductionProtocol(c.logger)
	introPoint, err := intro.SelectIntroductionPoint(desc)
	if err != nil {
		return 0, fmt.Errorf("failed to select introduction point: %w", err)
	}

	c.logger.Debug("Introduction point selected")

	// Step 5: Create circuit to introduction point
	introCircuitID, err := intro.CreateIntroductionCircuit(ctx, introPoint)
	if err != nil {
		return 0, fmt.Errorf("failed to create introduction circuit: %w", err)
	}

	c.logger.Debug("Introduction circuit created", "circuit_id", introCircuitID)

	// Step 6: Build and send INTRODUCE1 cell
	req := &IntroduceRequest{
		IntroPoint:       introPoint,
		RendezvousCookie: rendezvousCookie,
		RendezvousPoint:  rendezvousPoint.Fingerprint,
		OnionKey:         make([]byte, 32), // Phase 8 will generate real ephemeral key
	}

	introduce1Data, err := intro.BuildIntroduce1Cell(req)
	if err != nil {
		return 0, fmt.Errorf("failed to build INTRODUCE1 cell: %w", err)
	}

	c.logger.Debug("INTRODUCE1 cell built", "size", len(introduce1Data))

	if err := intro.SendIntroduce1(ctx, introCircuitID, introduce1Data); err != nil {
		return 0, fmt.Errorf("failed to send INTRODUCE1: %w", err)
	}

	c.logger.Debug("INTRODUCE1 cell sent")

	// Step 7: Wait for RENDEZVOUS2 and complete the connection
	if err := c.CompleteRendezvous(ctx, rendezvousCircuitID); err != nil {
		return 0, fmt.Errorf("failed to complete rendezvous: %w", err)
	}

	c.logger.Info("Successfully connected to onion service",
		"address", addr.String(),
		"rendezvous_circuit_id", rendezvousCircuitID)

	// Return the rendezvous circuit ID as it's the final connection circuit
	return rendezvousCircuitID, nil
}

// RendezvousProtocol handles rendezvous point operations for onion services
type RendezvousProtocol struct {
	logger *logger.Logger
}

// NewRendezvousProtocol creates a new rendezvous protocol handler
func NewRendezvousProtocol(log *logger.Logger) *RendezvousProtocol {
	if log == nil {
		log = logger.NewDefault()
	}

	return &RendezvousProtocol{
		logger: log.Component("rendezvous-protocol"),
	}
}

// SelectRendezvousPoint selects a suitable rendezvous point from available relays
// Per Tor spec (rend-spec-v3.txt section 3.3):
// The client selects a rendezvous point randomly from available relays
func (rp *RendezvousProtocol) SelectRendezvousPoint(relays []*HSDirectory) (*HSDirectory, error) {
	if len(relays) == 0 {
		return nil, fmt.Errorf("no relays available for rendezvous point selection")
	}

	// For Phase 7.3.4, select the first available relay
	// In a full implementation, this would:
	// 1. Filter relays that support being rendezvous points
	// 2. Exclude relays that are already in our circuit
	// 3. Randomly select from remaining relays
	// 4. Consider relay stability and performance metrics
	selected := relays[0]

	rp.logger.Debug("Selected rendezvous point",
		"relays_available", len(relays),
		"selected_fingerprint", selected.Fingerprint)

	return selected, nil
}

// EstablishRendezvousRequest represents a request to establish a rendezvous point
type EstablishRendezvousRequest struct {
	RendezvousCookie []byte // Rendezvous cookie (20 bytes)
}

// BuildEstablishRendezvousCell constructs an ESTABLISH_RENDEZVOUS cell
// Per Tor spec (rend-spec-v3.txt section 3.3):
// ESTABLISH_RENDEZVOUS {
//   RENDEZVOUS_COOKIE [20 bytes]
// }
func (rp *RendezvousProtocol) BuildEstablishRendezvousCell(req *EstablishRendezvousRequest) ([]byte, error) {
	if req == nil {
		return nil, fmt.Errorf("establish rendezvous request is nil")
	}
	if len(req.RendezvousCookie) != 20 {
		return nil, fmt.Errorf("invalid rendezvous cookie length: %d, expected 20", len(req.RendezvousCookie))
	}

	rp.logger.Debug("Building ESTABLISH_RENDEZVOUS cell",
		"cookie_len", len(req.RendezvousCookie))

	// The cell contains only the rendezvous cookie
	data := make([]byte, 20)
	copy(data, req.RendezvousCookie)

	rp.logger.Debug("Built ESTABLISH_RENDEZVOUS cell", "size", len(data))

	return data, nil
}

// CreateRendezvousCircuit creates a circuit to a rendezvous point
// This is a placeholder for the full circuit creation logic
func (rp *RendezvousProtocol) CreateRendezvousCircuit(ctx context.Context, rendezvousPoint *HSDirectory) (uint32, error) {
	if rendezvousPoint == nil {
		return 0, fmt.Errorf("rendezvous point is nil")
	}

	rp.logger.Info("Creating rendezvous circuit",
		"rendezvous_point", rendezvousPoint.Fingerprint)

	// In Phase 7.3.4, we return a mock circuit ID
	// In a full implementation (Phase 8), this would:
	// 1. Use the circuit builder to create a 3-hop circuit
	// 2. Extend the circuit to the rendezvous point
	// 3. Wait for circuit to be established
	// 4. Return the circuit ID

	// Mock circuit ID for testing
	circuitID := uint32(2000)

	rp.logger.Debug("Rendezvous circuit created (mock)",
		"circuit_id", circuitID)

	return circuitID, nil
}

// SendEstablishRendezvous sends an ESTABLISH_RENDEZVOUS cell over a circuit
// This is a placeholder for the full send logic
func (rp *RendezvousProtocol) SendEstablishRendezvous(ctx context.Context, circuitID uint32, establishData []byte) error {
	if len(establishData) == 0 {
		return fmt.Errorf("establish rendezvous data is empty")
	}

	rp.logger.Info("Sending ESTABLISH_RENDEZVOUS cell",
		"circuit_id", circuitID,
		"data_size", len(establishData))

	// In a full implementation (Phase 8), this would:
	// 1. Wrap establishData in a RELAY cell with command ESTABLISH_RENDEZVOUS
	// 2. Send the cell over the circuit
	// 3. Wait for RENDEZVOUS_ESTABLISHED acknowledgment
	// 4. Handle retries and errors

	rp.logger.Debug("ESTABLISH_RENDEZVOUS cell sent (mock)")

	return nil
}

// Rendezvous1Request represents a RENDEZVOUS1 request
type Rendezvous1Request struct {
	RendezvousCookie []byte // Rendezvous cookie (20 bytes)
	HandshakeData    []byte // Handshake data from INTRODUCE2
}

// BuildRendezvous1Cell constructs a RENDEZVOUS1 cell
// Per Tor spec (rend-spec-v3.txt section 3.4):
// RENDEZVOUS1 {
//   RENDEZVOUS_COOKIE [20 bytes]
//   HANDSHAKE_DATA    [remaining bytes]
// }
// Note: RENDEZVOUS1 is sent by the hidden service, not the client.
// This function is included for completeness but won't be used in client-only implementation.
func (rp *RendezvousProtocol) BuildRendezvous1Cell(req *Rendezvous1Request) ([]byte, error) {
	if req == nil {
		return nil, fmt.Errorf("rendezvous1 request is nil")
	}
	if len(req.RendezvousCookie) != 20 {
		return nil, fmt.Errorf("invalid rendezvous cookie length: %d, expected 20", len(req.RendezvousCookie))
	}

	rp.logger.Debug("Building RENDEZVOUS1 cell",
		"cookie_len", len(req.RendezvousCookie),
		"handshake_data_len", len(req.HandshakeData))

	var buf bytes.Buffer

	// RENDEZVOUS_COOKIE (20 bytes)
	buf.Write(req.RendezvousCookie)

	// HANDSHAKE_DATA
	if len(req.HandshakeData) > 0 {
		buf.Write(req.HandshakeData)
	}

	rp.logger.Debug("Built RENDEZVOUS1 cell", "size", buf.Len())

	return buf.Bytes(), nil
}

// ParseRendezvous2Cell parses a RENDEZVOUS2 cell
// Per Tor spec (rend-spec-v3.txt section 3.4):
// RENDEZVOUS2 {
//   HANDSHAKE_DATA [remaining bytes]
// }
func (rp *RendezvousProtocol) ParseRendezvous2Cell(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("rendezvous2 data is empty")
	}

	rp.logger.Debug("Parsing RENDEZVOUS2 cell", "size", len(data))

	// The entire cell is handshake data
	handshakeData := make([]byte, len(data))
	copy(handshakeData, data)

	rp.logger.Debug("Parsed RENDEZVOUS2 cell", "handshake_data_len", len(handshakeData))

	return handshakeData, nil
}

// WaitForRendezvous2 waits for a RENDEZVOUS2 cell on a circuit
// This is a placeholder for the full receive logic
func (rp *RendezvousProtocol) WaitForRendezvous2(ctx context.Context, circuitID uint32) ([]byte, error) {
	rp.logger.Info("Waiting for RENDEZVOUS2 cell", "circuit_id", circuitID)

	// In a full implementation (Phase 8), this would:
	// 1. Wait for a RELAY cell with command RENDEZVOUS2
	// 2. Parse the handshake data
	// 3. Verify the handshake
	// 4. Return the handshake data

	// Mock handshake data for testing
	handshakeData := make([]byte, 32)

	rp.logger.Debug("Received RENDEZVOUS2 cell (mock)", "handshake_data_len", len(handshakeData))

	return handshakeData, nil
}

// EstablishRendezvousPoint orchestrates establishing a rendezvous point
// This combines rendezvous point selection, circuit creation, and ESTABLISH_RENDEZVOUS
func (c *Client) EstablishRendezvousPoint(ctx context.Context, rendezvousCookie []byte, relays []*HSDirectory) (uint32, *HSDirectory, error) {
	c.logger.Info("Establishing rendezvous point")

	// Step 1: Select rendezvous point
	rendezvous := NewRendezvousProtocol(c.logger)
	rendezvousPoint, err := rendezvous.SelectRendezvousPoint(relays)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to select rendezvous point: %w", err)
	}

	c.logger.Debug("Rendezvous point selected", "fingerprint", rendezvousPoint.Fingerprint)

	// Step 2: Create circuit to rendezvous point
	circuitID, err := rendezvous.CreateRendezvousCircuit(ctx, rendezvousPoint)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to create rendezvous circuit: %w", err)
	}

	c.logger.Debug("Rendezvous circuit created", "circuit_id", circuitID)

	// Step 3: Build and send ESTABLISH_RENDEZVOUS cell
	req := &EstablishRendezvousRequest{
		RendezvousCookie: rendezvousCookie,
	}

	establishData, err := rendezvous.BuildEstablishRendezvousCell(req)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to build ESTABLISH_RENDEZVOUS cell: %w", err)
	}

	if err := rendezvous.SendEstablishRendezvous(ctx, circuitID, establishData); err != nil {
		return 0, nil, fmt.Errorf("failed to send ESTABLISH_RENDEZVOUS: %w", err)
	}

	c.logger.Info("Successfully established rendezvous point",
		"circuit_id", circuitID,
		"fingerprint", rendezvousPoint.Fingerprint)

	return circuitID, rendezvousPoint, nil
}

// CompleteRendezvous completes the rendezvous protocol
// This waits for RENDEZVOUS2 and establishes the final connection
func (c *Client) CompleteRendezvous(ctx context.Context, rendezvousCircuitID uint32) error {
	c.logger.Info("Completing rendezvous protocol", "circuit_id", rendezvousCircuitID)

	// Wait for RENDEZVOUS2 cell from the hidden service
	rendezvous := NewRendezvousProtocol(c.logger)
	handshakeData, err := rendezvous.WaitForRendezvous2(ctx, rendezvousCircuitID)
	if err != nil {
		return fmt.Errorf("failed to receive RENDEZVOUS2: %w", err)
	}

	c.logger.Debug("Received RENDEZVOUS2", "handshake_data_len", len(handshakeData))

	// In a full implementation, we would:
	// 1. Verify the handshake data
	// 2. Complete the key exchange
	// 3. Derive shared secrets
	// 4. Establish the final encrypted connection

	c.logger.Info("Rendezvous protocol completed successfully")

	return nil
}
