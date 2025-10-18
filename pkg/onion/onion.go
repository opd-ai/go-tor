// Package onion provides Onion Service (hidden service) functionality.
// This package implements both client and server functionality for .onion addresses.
// Supports v3 onion services (ed25519-based, 56-character addresses).
package onion

import (
	"crypto/sha3"
	"encoding/base32"
	"fmt"
	"strings"
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

// Descriptor represents an onion service descriptor
type Descriptor struct {
	Version         int
	Address         *Address
	IntroPoints     []IntroductionPoint
	DescriptorID    []byte
	BlindedPubkey   []byte
	RevisionCounter int64
	Signature       []byte
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

// Client provides onion service client functionality
type Client struct {
	// TODO: Add fields for descriptor fetching
	// directoryClient *directory.Client
	// hsDir cache
}

// NewClient creates a new onion service client
func NewClient() *Client {
	return &Client{}
}

// TODO: Implement descriptor fetching from HSDirs
// TODO: Implement introduction point protocol
// TODO: Implement rendezvous protocol
