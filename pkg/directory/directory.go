// Package directory provides Tor directory protocol functionality.
// This package handles fetching and parsing directory consensus documents and router descriptors.
package directory

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/opd-ai/go-tor/pkg/logger"
	"github.com/opd-ai/go-tor/pkg/resources"
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
var DefaultAuthorities = []string{
	"https://194.109.206.212/tor/status-vote/current/consensus.z",  // gabelmoo
	"https://131.188.40.189/tor/status-vote/current/consensus.z",   // moria1
	"https://128.31.0.34:9131/tor/status-vote/current/consensus.z", // tor26
}

// Relay represents a Tor relay from the consensus
type Relay struct {
	Nickname     string
	Fingerprint  string
	Address      string
	ORPort       int
	DirPort      int
	Flags        []string
	Published    time.Time
	IdentityKey  []byte // Ed25519 identity key (32 bytes) - SPEC-001
	NtorOnionKey []byte // Curve25519 ntor onion key (32 bytes) - SPEC-001
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

	// Try to load embedded fallback authorities, fall back to hardcoded defaults
	authorities := DefaultAuthorities
	if embeddedAuth, err := resources.GetFallbackAuthorities(); err == nil && len(embeddedAuth) > 0 {
		authorities = embeddedAuth
		log.Component("directory").Debug("Loaded fallback authorities from embedded resources", "count", len(embeddedAuth))
	} else {
		log.Component("directory").Debug("Using hardcoded fallback authorities", "count", len(DefaultAuthorities))
	}

	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger:      log.Component("directory"),
		authorities: authorities,
	}
}

// FetchConsensus fetches the network consensus from directory authorities
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
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse the consensus document
	relays, err := c.parseConsensus(resp.Body)
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
