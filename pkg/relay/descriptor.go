// Package relay implements Tor relay (bridge/non-exit) functionality.
package relay

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha1" // #nosec G505 - SHA1 required by Tor protocol
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/opd-ai/go-tor/pkg/crypto"
	"golang.org/x/crypto/curve25519"
)

// ServerDescriptor represents a Tor relay server descriptor
// Per dir-spec.txt §2.1, server descriptors advertise a relay's capabilities
type ServerDescriptor struct {
	// Metadata
	Nickname       string    // Relay nickname (1-19 alphanumeric characters)
	Address        string    // IPv4 address
	ORPort         uint16    // OR protocol port
	DirPort        uint16    // Directory port (0 for bridges)
	Platform       string    // Platform information (e.g., "Tor 0.4.8.0 on Linux")
	PublishedTime  time.Time // Descriptor publication time
	Uptime         int       // Relay uptime in seconds
	BandwidthAvg   uint64    // Average bandwidth (bytes/sec)
	BandwidthBurst uint64    // Burst bandwidth (bytes/sec)
	BandwidthObs   uint64    // Observed bandwidth (bytes/sec)
	Contact        string    // Contact information (optional)
	Family         []string  // Relay family members (optional)
	ExitPolicy     string    // Exit policy summary (default: "reject *:*")
	IPv6Addr       string    // IPv6 address (optional, e.g., "[2001:db8::1]:9001")

	// Cryptographic keys
	RSAIdentity     *rsa.PublicKey    // RSA-1024 identity public key
	Ed25519Identity ed25519.PublicKey // Ed25519 identity public key
	NtorOnionKey    []byte            // Curve25519 ntor onion key (32 bytes)

	// Internal fields
	rsaPrivate    *rsa.PrivateKey // RSA private key for signing
	Digest        []byte          // SHA-1 digest of descriptor (computed)
	Signature     []byte          // RSA signature of descriptor
	RawDescriptor []byte          // Complete descriptor text
}

// ExtraInfoDescriptor represents optional extra-info descriptor
// Per dir-spec.txt §2.2, extra-info contains additional statistics
type ExtraInfoDescriptor struct {
	Nickname      string
	Fingerprint   string
	PublishedTime time.Time
	Statistics    map[string]string
	Digest        []byte
	Signature     []byte
	RawDescriptor []byte // Complete extra-info descriptor text
}

// DescriptorConfig holds configuration for descriptor generation
type DescriptorConfig struct {
	Nickname       string   // Relay nickname (default: auto-generated)
	Address        string   // IPv4 address (required)
	ORPort         uint16   // OR port (required)
	DirPort        uint16   // Directory port (0 for bridges)
	Contact        string   // Contact info (optional)
	Family         []string // Family members (optional)
	BandwidthAvg   uint64   // Average bandwidth (default: 1MB/s)
	BandwidthBurst uint64   // Burst bandwidth (default: 2MB/s)
	IPv6Addr       string   // IPv6 address:port (optional)
	IsBridge       bool     // Whether this is a bridge relay
}

// GenerateServerDescriptor creates a signed server descriptor
// This implements dir-spec.txt §2.1 server descriptor format
func GenerateServerDescriptor(keys *RelayKeys, config *DescriptorConfig) (*ServerDescriptor, error) {
	if keys == nil {
		return nil, fmt.Errorf("relay keys cannot be nil")
	}
	if config == nil {
		return nil, fmt.Errorf("descriptor config cannot be nil")
	}
	if config.Address == "" {
		return nil, fmt.Errorf("relay address is required")
	}
	if config.ORPort == 0 {
		return nil, fmt.Errorf("OR port is required")
	}

	// Validate address
	if net.ParseIP(config.Address) == nil {
		return nil, fmt.Errorf("invalid IPv4 address: %s", config.Address)
	}

	// Set defaults
	nickname := config.Nickname
	if nickname == "" {
		nickname = generateNickname(keys)
	}

	bandwidthAvg := config.BandwidthAvg
	if bandwidthAvg == 0 {
		bandwidthAvg = 1024 * 1024 // 1 MB/s default
	}

	bandwidthBurst := config.BandwidthBurst
	if bandwidthBurst == 0 {
		bandwidthBurst = bandwidthAvg * 2 // 2x average
	}

	// Observed bandwidth starts at average (will be updated by bandwidth measurement)
	bandwidthObs := bandwidthAvg

	// Exit policy (always reject for non-exit/bridge relays)
	exitPolicy := "reject *:*"

	// Compute ntor onion public key from private key
	var ntorPublic [32]byte
	curve25519.ScalarBaseMult(&ntorPublic, (*[32]byte)(keys.NtorOnionKey))

	desc := &ServerDescriptor{
		Nickname:        nickname,
		Address:         config.Address,
		ORPort:          config.ORPort,
		DirPort:         config.DirPort,
		Platform:        "go-tor 0.1.0 on Go",
		PublishedTime:   time.Now().UTC(),
		Uptime:          0, // Will be updated by relay runtime
		BandwidthAvg:    bandwidthAvg,
		BandwidthBurst:  bandwidthBurst,
		BandwidthObs:    bandwidthObs,
		Contact:         config.Contact,
		Family:          config.Family,
		ExitPolicy:      exitPolicy,
		IPv6Addr:        config.IPv6Addr,
		RSAIdentity:     &keys.RSAPrivate.PublicKey,
		Ed25519Identity: keys.Ed25519Public,
		NtorOnionKey:    ntorPublic[:],
		rsaPrivate:      keys.RSAPrivate,
	}

	// Build and sign descriptor
	if err := desc.build(); err != nil {
		return nil, fmt.Errorf("failed to build descriptor: %w", err)
	}

	return desc, nil
}

// build constructs the descriptor text and computes digest/signature
func (d *ServerDescriptor) build() error {
	var buf bytes.Buffer

	// Per dir-spec.txt §2.1, descriptor format:
	// router <nickname> <address> <ORPort> <SOCKSPort> <DirPort>
	fmt.Fprintf(&buf, "router %s %s %d 0 %d\n",
		d.Nickname, d.Address, d.ORPort, d.DirPort)

	// IPv6 address (optional)
	if d.IPv6Addr != "" {
		fmt.Fprintf(&buf, "or-address %s\n", d.IPv6Addr)
	}

	// Platform
	fmt.Fprintf(&buf, "platform %s\n", d.Platform)

	// Protocols (link protocol versions we support: 3-5)
	fmt.Fprintf(&buf, "proto Link=3-5 Circuit=1-2\n")

	// Published time (UTC, format: "YYYY-MM-DD HH:MM:SS")
	fmt.Fprintf(&buf, "published %s\n",
		d.PublishedTime.Format("2006-01-02 15:04:05"))

	// RSA identity key (PKCS#1 format)
	rsaPEM, err := crypto.RSAPublicKeyToPEM(d.RSAIdentity)
	if err != nil {
		return fmt.Errorf("failed to encode RSA key: %w", err)
	}
	// Remove PEM headers and format as Tor expects
	rsaB64 := strings.TrimPrefix(string(rsaPEM), "-----BEGIN RSA PUBLIC KEY-----\n")
	rsaB64 = strings.TrimSuffix(rsaB64, "-----END RSA PUBLIC KEY-----\n")
	fmt.Fprintf(&buf, "identity-ed25519\n-----BEGIN ED25519 CERT-----\n")
	// Ed25519 identity is encoded as base64
	fmt.Fprintf(&buf, "%s\n", base64.StdEncoding.EncodeToString(d.Ed25519Identity))
	fmt.Fprintf(&buf, "-----END ED25519 CERT-----\n")

	// Master key ed25519 (32-byte public key)
	fmt.Fprintf(&buf, "master-key-ed25519 %s\n",
		base64.StdEncoding.EncodeToString(d.Ed25519Identity))

	// Bandwidth (avg, burst, observed in bytes/sec)
	fmt.Fprintf(&buf, "bandwidth %d %d %d\n",
		d.BandwidthAvg, d.BandwidthBurst, d.BandwidthObs)

	// Uptime
	fmt.Fprintf(&buf, "uptime %d\n", d.Uptime)

	// ntor onion key (base64-encoded Curve25519 public key)
	fmt.Fprintf(&buf, "ntor-onion-key %s\n",
		base64.StdEncoding.EncodeToString(d.NtorOnionKey))

	// Contact (optional)
	if d.Contact != "" {
		fmt.Fprintf(&buf, "contact %s\n", d.Contact)
	}

	// Family (optional)
	if len(d.Family) > 0 {
		fmt.Fprintf(&buf, "family %s\n", strings.Join(d.Family, " "))
	}

	// Exit policy
	fmt.Fprintf(&buf, "reject *:*\n")

	// Router signature follows
	// Compute digest before signature
	descriptorBody := buf.String()

	// Compute SHA-1 digest of descriptor (required per Tor spec)
	// #nosec G401 - SHA1 required by Tor protocol
	h := sha1.New() // #nosec G401
	h.Write([]byte(descriptorBody))
	d.Digest = h.Sum(nil)

	// Sign with RSA identity key
	// Per dir-spec.txt, signature is over SHA-1 digest
	signature, err := rsa.SignPKCS1v15(nil, d.rsaPrivate, 0, d.Digest)
	if err != nil {
		return fmt.Errorf("failed to sign descriptor: %w", err)
	}
	d.Signature = signature

	// Append signature to descriptor
	fmt.Fprintf(&buf, "router-signature\n-----BEGIN SIGNATURE-----\n")
	fmt.Fprintf(&buf, "%s\n", base64.StdEncoding.EncodeToString(signature))
	fmt.Fprintf(&buf, "-----END SIGNATURE-----\n")

	d.RawDescriptor = buf.Bytes()
	return nil
}

// generateNickname creates a default nickname from relay fingerprint
func generateNickname(keys *RelayKeys) string {
	// Use first 8 chars of hex-encoded Ed25519 public key
	if len(keys.Ed25519Public) >= 4 {
		return "Unnamed" + hex.EncodeToString(keys.Ed25519Public[:4])
	}
	return "UnnamedRelay"
}

// Fingerprint returns the relay's SHA-1 fingerprint (40 hex chars)
func (d *ServerDescriptor) Fingerprint() string {
	if d.RSAIdentity == nil {
		return ""
	}
	// Compute SHA-1 of RSA public key DER encoding
	// #nosec G401 - SHA1 required by Tor protocol
	h := sha1.New() // #nosec G401
	h.Write(d.RSAIdentity.N.Bytes())
	return strings.ToUpper(hex.EncodeToString(h.Sum(nil)))
}

// Ed25519Fingerprint returns base64-encoded Ed25519 identity
func (d *ServerDescriptor) Ed25519Fingerprint() string {
	return base64.StdEncoding.EncodeToString(d.Ed25519Identity)
}

// Validate checks descriptor integrity
func (d *ServerDescriptor) Validate() error {
	if d.Nickname == "" {
		return fmt.Errorf("nickname is required")
	}
	if len(d.Nickname) > 19 {
		return fmt.Errorf("nickname too long (max 19 chars): %s", d.Nickname)
	}
	if d.Address == "" {
		return fmt.Errorf("address is required")
	}
	if d.ORPort == 0 {
		return fmt.Errorf("OR port is required")
	}
	if d.RSAIdentity == nil {
		return fmt.Errorf("RSA identity key is required")
	}
	if len(d.Ed25519Identity) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid Ed25519 key size: %d", len(d.Ed25519Identity))
	}
	if len(d.NtorOnionKey) != 32 {
		return fmt.Errorf("invalid ntor key size: %d", len(d.NtorOnionKey))
	}
	if len(d.Signature) == 0 {
		return fmt.Errorf("descriptor signature is required")
	}
	if len(d.RawDescriptor) == 0 {
		return fmt.Errorf("raw descriptor is empty")
	}
	return nil
}

// GenerateExtraInfo creates an extra-info descriptor with statistics
// Per dir-spec.txt §2.2, extra-info provides bandwidth and usage statistics
func GenerateExtraInfo(keys *RelayKeys, desc *ServerDescriptor, stats map[string]string) (*ExtraInfoDescriptor, error) {
	if keys == nil {
		return nil, fmt.Errorf("relay keys cannot be nil")
	}
	if desc == nil {
		return nil, fmt.Errorf("server descriptor cannot be nil")
	}

	extraInfo := &ExtraInfoDescriptor{
		Nickname:      desc.Nickname,
		Fingerprint:   desc.Fingerprint(),
		PublishedTime: time.Now().UTC(),
		Statistics:    stats,
	}

	// Build extra-info document
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "extra-info %s %s\n", extraInfo.Nickname, extraInfo.Fingerprint)
	fmt.Fprintf(&buf, "published %s\n",
		extraInfo.PublishedTime.Format("2006-01-02 15:04:05"))

	// Add statistics
	for key, value := range stats {
		fmt.Fprintf(&buf, "%s %s\n", key, value)
	}

	// Compute digest and sign
	body := buf.String()
	h := sha256.New()
	h.Write([]byte(body))
	extraInfo.Digest = h.Sum(nil)

	// Sign with RSA key
	sig, err := rsa.SignPKCS1v15(nil, keys.RSAPrivate, 0, extraInfo.Digest)
	if err != nil {
		return nil, fmt.Errorf("failed to sign extra-info: %w", err)
	}
	extraInfo.Signature = sig

	// Build complete descriptor with signature
	var fullBuf bytes.Buffer
	fullBuf.WriteString(body)
	fmt.Fprintf(&fullBuf, "router-signature\n")

	// Encode signature in base64
	sigB64 := base64.StdEncoding.EncodeToString(sig)
	fmt.Fprintf(&fullBuf, "-----BEGIN SIGNATURE-----\n")
	// Split base64 into 64-char lines
	for i := 0; i < len(sigB64); i += 64 {
		end := i + 64
		if end > len(sigB64) {
			end = len(sigB64)
		}
		fmt.Fprintf(&fullBuf, "%s\n", sigB64[i:end])
	}
	fmt.Fprintf(&fullBuf, "-----END SIGNATURE-----\n")

	extraInfo.RawDescriptor = fullBuf.Bytes()

	return extraInfo, nil
}

// String returns human-readable descriptor summary
func (d *ServerDescriptor) String() string {
	return fmt.Sprintf("ServerDescriptor{nickname=%s, address=%s:%d, fingerprint=%s}",
		d.Nickname, d.Address, d.ORPort, d.Fingerprint()[:16]+"...")
}
