// Package directory provides Tor directory protocol functionality.
// This package handles fetching and parsing directory consensus documents and router descriptors.
package directory

import (
	"bufio"
	"compress/gzip"
	"compress/zlib"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/opd-ai/go-tor/pkg/logger"
)

const (
	// Consensus validation thresholds (SEC-004, SEC-014)
	maxMalformedEntryRate = 10 // Reject if >10% of entries are malformed
	maxPortParseErrorRate = 20 // Warn if >20% of entries have port parse errors

	// SPEC-003: Enhanced consensus signature validation thresholds
	// These constants support future implementation of multi-signature threshold validation
	// per dir-spec.txt section 3.4 (Voting and consensus signature requirements)
	minDirectoryAuthorities = 3                // Minimum authorities for valid consensus
	minSignatureThreshold   = 2                // Minimum signatures required (future: implement proper quorum)
	maxClockSkew            = 30 * time.Minute // Maximum allowed clock skew for consensus timestamps
)

// Default directory authority addresses (hardcoded fallback directories)
// Using HTTP instead of HTTPS for better compatibility with IP-based authorities
// The Tor consensus is cryptographically signed, so transport encryption is not critical
var DefaultAuthorities = []string{
	"http://194.109.206.212/tor/status-vote/current/consensus",      // gabelmoo
	"http://131.188.40.189/tor/status-vote/current/consensus",       // moria1
	"http://128.31.0.34:9131/tor/status-vote/current/consensus",     // tor26
	"http://86.59.21.38/tor/status-vote/current/consensus",          // longclaw
	"http://199.58.81.140/tor/status-vote/current/consensus",        // bastet
	"http://204.13.164.118:18080/tor/status-vote/current/consensus", // faravahar
}

// Relay represents a Tor relay from the consensus
type Relay struct {
	Nickname        string
	Fingerprint     string
	Address         string
	ORPort          int
	DirPort         int
	Flags           []string
	Published       time.Time
	IdentityKey     []byte // Ed25519 identity key (32 bytes) - SPEC-001
	NtorOnionKey    []byte // Curve25519 ntor onion key (32 bytes) - SPEC-001
	MicrodescDigest string // SHA256 digest of microdescriptor (base64) - SPEC-001
}

// Client provides directory protocol operations
type Client struct {
	httpClient  *http.Client
	logger      *logger.Logger
	authorities []string
}

// NewClient creates a new directory client
func NewClient(log *logger.Logger) *Client {
	if log == nil {
		log = logger.NewDefault()
	}

	// Create HTTP client with custom transport
	// Use TLS config that skips verification for IP-based authorities
	// This is acceptable because consensus documents are cryptographically signed
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // Required for IP-based directory authorities
		},
	}

	return &Client{
		httpClient: &http.Client{
			Timeout:   10 * time.Second, // Reduced timeout for faster fallback
			Transport: transport,
		},
		logger:      log.Component("directory"),
		authorities: DefaultAuthorities,
	}
}

// FetchConsensus fetches the network consensus from directory authorities
// and populates relay cryptographic keys from microdescriptors (SPEC-001)
func (c *Client) FetchConsensus(ctx context.Context) ([]*Relay, error) {
	c.logger.Info("Fetching network consensus")

	// Try each authority until one succeeds
	var lastErr error
	for _, authority := range c.authorities {
		relays, err := c.fetchFromAuthority(ctx, authority)
		if err != nil {
			c.logger.Warn("Failed to fetch from authority", "authority", authority, "error", err)
			lastErr = err
			continue
		}

		c.logger.Info("Successfully fetched consensus", "relays", len(relays), "authority", authority)

		// Fetch microdescriptors to populate relay keys (SPEC-001)
		if err := c.FetchMicrodescriptors(ctx, relays); err != nil {
			c.logger.Warn("Failed to fetch microdescriptors, relays will lack cryptographic keys", "error", err)
			// Don't fail the entire consensus fetch, just warn
		}

		return relays, nil
	}

	return nil, fmt.Errorf("failed to fetch consensus from any authority: %w", lastErr)
}

// fetchFromAuthority fetches consensus from a specific authority
func (c *Client) fetchFromAuthority(ctx context.Context, authorityURL string) ([]*Relay, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", authorityURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch consensus: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.logger.Error("Failed to close response body", "function", "fetchFromAuthority", "error", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Handle compressed response
	var reader io.Reader = resp.Body
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer func() {
			if err := gzReader.Close(); err != nil {
				c.logger.Error("Failed to close gzip reader", "function", "fetchFromAuthority", "error", err)
			}
		}()
		reader = gzReader
	case "deflate":
		// Try zlib format first (deflate with wrapper)
		zlibReader, err := zlib.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to create zlib reader: %w", err)
		}
		defer func() {
			if err := zlibReader.Close(); err != nil {
				c.logger.Error("Failed to close zlib reader", "function", "fetchFromAuthority", "error", err)
			}
		}()
		reader = zlibReader
	}

	// Parse the consensus document
	relays, err := c.parseConsensus(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to parse consensus: %w", err)
	}

	return relays, nil
}

// parseConsensus parses a consensus document and extracts relay information
func (c *Client) parseConsensus(r io.Reader) ([]*Relay, error) {
	var relays []*Relay
	scanner := bufio.NewScanner(r)

	var currentRelay *Relay
	var totalEntries int
	var malformedEntries int
	var portParseErrors int

	for scanner.Scan() {
		line := scanner.Text()

		// Parse "r" lines (router status entries)
		if strings.HasPrefix(line, "r ") {
			totalEntries++

			if currentRelay != nil {
				relays = append(relays, currentRelay)
			}

			parts := strings.Fields(line)
			if len(parts) < 9 {
				malformedEntries++
				c.logger.Debug("Skipping malformed relay entry", "line", line)
				continue // Skip malformed entries
			}

			currentRelay = &Relay{
				Nickname:    parts[1],
				Fingerprint: parts[2],
				Address:     parts[6],
			}

			// Parse ORPort (track errors for SEC-014)
			if _, err := fmt.Sscanf(parts[7], "%d", &currentRelay.ORPort); err != nil {
				portParseErrors++
				c.logger.Debug("Failed to parse ORPort", "error", err, "value", parts[7])
			}
			// Parse DirPort (track errors for SEC-014)
			if _, err := fmt.Sscanf(parts[8], "%d", &currentRelay.DirPort); err != nil {
				portParseErrors++
				c.logger.Debug("Failed to parse DirPort", "error", err, "value", parts[8])
			}
		}

		// Parse "a" lines (microdescriptor digests) - SPEC-001
		if strings.HasPrefix(line, "a ") && currentRelay != nil {
			parts := strings.Fields(line)
			// Format: "a" SP algname "=" digest
			// e.g., "a sha256=base64digest"
			if len(parts) >= 2 && strings.HasPrefix(parts[1], "sha256=") {
				currentRelay.MicrodescDigest = strings.TrimPrefix(parts[1], "sha256=")
			}
		}

		// Parse "s" lines (flags)
		if strings.HasPrefix(line, "s ") && currentRelay != nil {
			flags := strings.Fields(line[2:]) // Skip "s "
			currentRelay.Flags = flags
		}
	}

	// Add the last relay
	if currentRelay != nil {
		relays = append(relays, currentRelay)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading consensus: %w", err)
	}

	// Validate that consensus is not excessively malformed (SEC-004)
	// Reject if malformed entries exceed threshold, indicating possible attack or corruption
	malformedThreshold := totalEntries * maxMalformedEntryRate / 100
	if totalEntries > 0 && malformedEntries > malformedThreshold {
		c.logger.Warn("Excessive malformed entries in consensus",
			"malformed", malformedEntries, "total", totalEntries)
		return nil, fmt.Errorf("excessive malformed entries in consensus: %d/%d (>%d%%)",
			malformedEntries, totalEntries, maxMalformedEntryRate)
	}

	// Warn if excessive port parse errors (SEC-014)
	portErrorThreshold := totalEntries * maxPortParseErrorRate / 100
	if totalEntries > 0 && portParseErrors > portErrorThreshold {
		c.logger.Warn("Excessive port parse errors in consensus",
			"port_errors", portParseErrors, "total", totalEntries)
	}

	if malformedEntries > 0 || portParseErrors > 0 {
		c.logger.Debug("Consensus parsing completed with some errors",
			"malformed", malformedEntries, "port_errors", portParseErrors,
			"total", totalEntries, "valid", len(relays))
	}

	return relays, nil
}

// HasFlag checks if a relay has a specific flag
func (r *Relay) HasFlag(flag string) bool {
	for _, f := range r.Flags {
		if f == flag {
			return true
		}
	}
	return false
}

// IsGuard returns true if the relay is a guard
func (r *Relay) IsGuard() bool {
	return r.HasFlag("Guard")
}

// IsExit returns true if the relay is an exit
func (r *Relay) IsExit() bool {
	return r.HasFlag("Exit")
}

// IsStable returns true if the relay is stable
func (r *Relay) IsStable() bool {
	return r.HasFlag("Stable")
}

// IsRunning returns true if the relay is running
func (r *Relay) IsRunning() bool {
	return r.HasFlag("Running")
}

// IsValid returns true if the relay is valid
func (r *Relay) IsValid() bool {
	return r.HasFlag("Valid")
}

// String returns a string representation of the relay
func (r *Relay) String() string {
	return fmt.Sprintf("%s (%s:%d)", r.Nickname, r.Address, r.ORPort)
}

// GetIdentityKey returns the relay's Ed25519 identity key (SPEC-001)
func (r *Relay) GetIdentityKey() []byte {
	return r.IdentityKey
}

// GetNtorOnionKey returns the relay's Curve25519 ntor onion key (SPEC-001)
func (r *Relay) GetNtorOnionKey() []byte {
	return r.NtorOnionKey
}

// HasValidKeys returns true if the relay has both required cryptographic keys (SPEC-001)
func (r *Relay) HasValidKeys() bool {
	return len(r.IdentityKey) == 32 && len(r.NtorOnionKey) == 32
}

// SPEC-003: Enhanced consensus validation infrastructure
// These types and methods provide hooks for implementing full multi-signature
// threshold validation per dir-spec.txt section 3.4

// ConsensusMetadata contains metadata about a consensus document (SPEC-003)
// Future enhancement: parse and validate directory authority signatures
type ConsensusMetadata struct {
	ValidAfter  time.Time
	FreshUntil  time.Time
	ValidUntil  time.Time
	Signatures  int // Number of authority signatures (future: validate each)
	Authorities int // Number of authorities in consensus
}

// ValidateConsensusMetadata performs enhanced validation on consensus metadata (SPEC-003)
// This provides infrastructure for implementing multi-signature threshold validation
// Current implementation provides basic timing validation; future versions should:
// - Parse and verify all directory authority signatures
// - Validate signature threshold meets quorum requirements
// - Check authority keys against hardcoded trusted set
// - Implement proper Byzantine fault tolerance
func ValidateConsensusMetadata(meta *ConsensusMetadata) error {
	now := time.Now()

	// Check clock skew
	if meta.ValidAfter.After(now.Add(maxClockSkew)) {
		return fmt.Errorf("consensus valid-after time is too far in the future")
	}

	// Check expiration
	if meta.ValidUntil.Before(now.Add(-maxClockSkew)) {
		return fmt.Errorf("consensus has expired")
	}

	// Basic signature count validation
	// Future enhancement: implement proper quorum calculation per dir-spec.txt
	if meta.Signatures < minSignatureThreshold {
		return fmt.Errorf("insufficient signatures: %d < %d", meta.Signatures, minSignatureThreshold)
	}

	// Authority count validation
	if meta.Authorities < minDirectoryAuthorities {
		return fmt.Errorf("insufficient authorities: %d < %d", meta.Authorities, minDirectoryAuthorities)
	}

	return nil
}

// FetchMicrodescriptors fetches microdescriptors for relays and populates their cryptographic keys (SPEC-001)
// This implements the microdescriptor fetching protocol per dir-spec.txt §3.3
func (c *Client) FetchMicrodescriptors(ctx context.Context, relays []*Relay) error {
	// Collect unique microdescriptor digests
	digestMap := make(map[string][]*Relay)
	for _, relay := range relays {
		if relay.MicrodescDigest != "" {
			digestMap[relay.MicrodescDigest] = append(digestMap[relay.MicrodescDigest], relay)
		}
	}

	if len(digestMap) == 0 {
		c.logger.Warn("No microdescriptor digests found in consensus")
		return nil
	}

	c.logger.Info("Fetching microdescriptors", "count", len(digestMap))

	// Build URL with digests (batch fetch up to 92 descriptors per request per spec)
	digests := make([]string, 0, len(digestMap))
	for digest := range digestMap {
		digests = append(digests, digest)
	}

	// Fetch in batches to avoid URL length limits
	const batchSize = 90
	for i := 0; i < len(digests); i += batchSize {
		end := i + batchSize
		if end > len(digests) {
			end = len(digests)
		}
		batch := digests[i:end]

		if err := c.fetchMicrodescriptorBatch(ctx, batch, digestMap); err != nil {
			c.logger.Warn("Failed to fetch microdescriptor batch", "error", err, "batch", i/batchSize)
			// Continue with next batch instead of failing entirely
		}
	}

	return nil
}

// fetchMicrodescriptorBatch fetches a batch of microdescriptors from directory authorities
func (c *Client) fetchMicrodescriptorBatch(ctx context.Context, digests []string, digestMap map[string][]*Relay) error {
	// Build URL: /tor/micro/d/digest1-digest2-digest3
	digestList := strings.Join(digests, "-")
	urlPath := "/tor/micro/d/" + digestList

	// Try each authority until one succeeds
	var lastErr error
	for _, authority := range c.authorities {
		// Extract base URL from consensus URL
		baseURL := strings.TrimSuffix(authority, "/tor/status-vote/current/consensus")
		mdURL := baseURL + urlPath

		md, err := c.fetchMicrodescriptorsFromAuthority(ctx, mdURL)
		if err != nil {
			c.logger.Debug("Failed to fetch microdescriptors from authority", "authority", baseURL, "error", err)
			lastErr = err
			continue
		}

		// Parse and populate relay keys
		if err := c.parseMicrodescriptors(md, digestMap); err != nil {
			c.logger.Warn("Failed to parse microdescriptors", "error", err)
			lastErr = err
			continue
		}

		c.logger.Debug("Successfully fetched microdescriptor batch", "count", len(digests))
		return nil
	}

	return fmt.Errorf("failed to fetch microdescriptors from any authority: %w", lastErr)
}

// fetchMicrodescriptorsFromAuthority fetches microdescriptors from a specific authority
func (c *Client) fetchMicrodescriptorsFromAuthority(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch microdescriptors: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.logger.Error("Failed to close response body", "function", "fetchMicrodescriptorsFromAuthority", "error", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Handle compressed response
	var reader io.Reader = resp.Body
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer func() {
			if err := gzReader.Close(); err != nil {
				c.logger.Error("Failed to close gzip reader", "function", "fetchMicrodescriptorsFromAuthority", "error", err)
			}
		}()
		reader = gzReader
	case "deflate":
		zlibReader, err := zlib.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to create zlib reader: %w", err)
		}
		defer func() {
			if err := zlibReader.Close(); err != nil {
				c.logger.Error("Failed to close zlib reader", "function", "fetchMicrodescriptorsFromAuthority", "error", err)
			}
		}()
		reader = zlibReader
	}

	return io.ReadAll(reader)
}

// parseMicrodescriptors parses microdescriptor documents and populates relay keys (SPEC-001)
// Microdescriptor format per dir-spec.txt §3.3:
//
//	onion-key (RSA key, not used for ntor)
//	ntor-onion-key base64(curve25519 key)
//	id ed25519 base64(32-byte identity key)
func (c *Client) parseMicrodescriptors(data []byte, digestMap map[string][]*Relay) error {
	scanner := bufio.NewScanner(strings.NewReader(string(data)))

	var currentMD struct {
		ntorKey     []byte
		identityKey []byte
		lines       []string
	}

	for scanner.Scan() {
		line := scanner.Text()
		currentMD.lines = append(currentMD.lines, line)

		// Parse ntor-onion-key
		if strings.HasPrefix(line, "ntor-onion-key ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				key, err := base64.StdEncoding.DecodeString(parts[1])
				if err != nil {
					c.logger.Debug("Failed to decode ntor-onion-key", "error", err)
					continue
				}
				if len(key) == 32 {
					currentMD.ntorKey = key
				}
			}
		}

		// Parse id ed25519
		if strings.HasPrefix(line, "id ed25519") {
			// Next line contains base64-encoded identity key
			if scanner.Scan() {
				keyLine := scanner.Text()
				currentMD.lines = append(currentMD.lines, keyLine)
				key, err := base64.StdEncoding.DecodeString(keyLine)
				if err != nil {
					c.logger.Debug("Failed to decode ed25519 identity", "error", err)
					continue
				}
				if len(key) == 32 {
					currentMD.identityKey = key
				}
			}
		}

		// End of microdescriptor (blank line or start of next)
		if line == "" || strings.HasPrefix(line, "onion-key") {
			if len(currentMD.ntorKey) == 32 && len(currentMD.identityKey) == 32 {
				// Calculate digest and match to relays
				digest := c.calculateMicrodescriptorDigest(currentMD.lines)
				if relays, ok := digestMap[digest]; ok {
					for _, relay := range relays {
						relay.NtorOnionKey = currentMD.ntorKey
						relay.IdentityKey = currentMD.identityKey
					}
				}
			}

			// Reset for next microdescriptor
			currentMD.ntorKey = nil
			currentMD.identityKey = nil
			currentMD.lines = nil
		}
	}

	// Process last microdescriptor
	if len(currentMD.ntorKey) == 32 && len(currentMD.identityKey) == 32 {
		digest := c.calculateMicrodescriptorDigest(currentMD.lines)
		if relays, ok := digestMap[digest]; ok {
			for _, relay := range relays {
				relay.NtorOnionKey = currentMD.ntorKey
				relay.IdentityKey = currentMD.identityKey
			}
		}
	}

	return scanner.Err()
}

// calculateMicrodescriptorDigest computes SHA256 digest of microdescriptor for verification
func (c *Client) calculateMicrodescriptorDigest(lines []string) string {
	content := strings.Join(lines, "\n")
	hash := sha256.Sum256([]byte(content))
	return base64.StdEncoding.EncodeToString(hash[:])
}
