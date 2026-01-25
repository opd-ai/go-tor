// Package relay implements Tor relay (bridge/non-exit) functionality.
// This provides server-side OR protocol support for educational and research purposes.
package relay

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1" // #nosec G505 - SHA1 required by Tor protocol for fingerprints
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"github.com/opd-ai/go-tor/pkg/security"
)

// RelayKeys holds the cryptographic keys for a relay
type RelayKeys struct {
	// Ed25519 identity key (32 bytes public + 64 bytes private)
	Ed25519Public  ed25519.PublicKey
	Ed25519Private ed25519.PrivateKey

	// RSA identity key (1024 bits as per Tor spec)
	RSAPrivate *rsa.PrivateKey

	// TLS certificate for OR connections
	TLSCert []byte // DER-encoded X.509 certificate
}

// Fingerprint returns the relay's RSA identity fingerprint (SHA-1 of RSA public key)
// This is the 40-character hex string used to identify relays in Tor.
// #nosec G401 - SHA1 required by Tor specification for fingerprints
func (k *RelayKeys) Fingerprint() string {
	if k.RSAPrivate == nil {
		return ""
	}

	// Encode RSA public key to PKCS#1 DER format
	pubDER := x509.MarshalPKCS1PublicKey(&k.RSAPrivate.PublicKey)

	// Compute SHA-1 hash (Tor fingerprint format)
	h := sha1.Sum(pubDER) // #nosec G401
	return hex.EncodeToString(h[:])
}

// Ed25519Fingerprint returns the base64-encoded Ed25519 identity key
func (k *RelayKeys) Ed25519Fingerprint() string {
	if k.Ed25519Public == nil {
		return ""
	}
	return hex.EncodeToString(k.Ed25519Public)
}

// GenerateRelayKeys generates a new set of relay keys
// Returns Ed25519 identity key, RSA identity key (1024-bit), and TLS certificate
func GenerateRelayKeys() (*RelayKeys, error) {
	keys := &RelayKeys{}

	// Generate Ed25519 identity key
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate Ed25519 key: %w", err)
	}
	keys.Ed25519Public = pub
	keys.Ed25519Private = priv

	// Generate RSA identity key (1024 bits per Tor spec)
	rsaKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		return nil, fmt.Errorf("failed to generate RSA key: %w", err)
	}
	keys.RSAPrivate = rsaKey

	// Generate self-signed TLS certificate
	cert, err := generateTLSCertificate(rsaKey)
	if err != nil {
		return nil, fmt.Errorf("failed to generate TLS certificate: %w", err)
	}
	keys.TLSCert = cert

	return keys, nil
}

// generateTLSCertificate generates a self-signed X.509 certificate for TLS
// Per tor-spec.txt §1.1, relays use self-signed certificates with specific properties
func generateTLSCertificate(rsaKey *rsa.PrivateKey) ([]byte, error) {
	// Certificate template per Tor specification
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Tor"},
			CommonName:   "www.torproject.org",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour), // 1 year validity
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// Create self-signed certificate
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &rsaKey.PublicKey, rsaKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	return certDER, nil
}

// SaveKeys saves relay keys to disk with secure permissions
func (k *RelayKeys) SaveKeys(dataDir string) error {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Save Ed25519 identity key (private key is 64 bytes in Go's representation)
	ed25519Path := filepath.Join(dataDir, "ed25519_identity_secret_key")
	if err := saveSecureFile(ed25519Path, k.Ed25519Private, 0o600); err != nil {
		return fmt.Errorf("failed to save Ed25519 key: %w", err)
	}

	// Save RSA identity key in PEM format
	rsaPath := filepath.Join(dataDir, "rsa_identity_secret_key")
	rsaDER := x509.MarshalPKCS1PrivateKey(k.RSAPrivate)
	rsaPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: rsaDER,
	})
	if err := saveSecureFile(rsaPath, rsaPEM, 0o600); err != nil {
		return fmt.Errorf("failed to save RSA key: %w", err)
	}

	// Save TLS certificate in PEM format
	certPath := filepath.Join(dataDir, "tls_certificate.pem")
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: k.TLSCert,
	})
	if err := saveSecureFile(certPath, certPEM, 0o644); err != nil {
		return fmt.Errorf("failed to save TLS certificate: %w", err)
	}

	return nil
}

// LoadKeys loads relay keys from disk
func LoadKeys(dataDir string) (*RelayKeys, error) {
	keys := &RelayKeys{}

	// Load Ed25519 identity key
	ed25519Path := filepath.Join(dataDir, "ed25519_identity_secret_key")
	ed25519Data, err := os.ReadFile(ed25519Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read Ed25519 key: %w", err)
	}
	if len(ed25519Data) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid Ed25519 key size: got %d, want %d", len(ed25519Data), ed25519.PrivateKeySize)
	}
	keys.Ed25519Private = ed25519.PrivateKey(ed25519Data)
	keys.Ed25519Public = keys.Ed25519Private.Public().(ed25519.PublicKey)

	// Load RSA identity key
	rsaPath := filepath.Join(dataDir, "rsa_identity_secret_key")
	rsaPEMData, err := os.ReadFile(rsaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read RSA key: %w", err)
	}
	block, _ := pem.Decode(rsaPEMData)
	if block == nil || block.Type != "RSA PRIVATE KEY" {
		return nil, fmt.Errorf("invalid RSA key PEM format")
	}
	rsaKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse RSA key: %w", err)
	}
	keys.RSAPrivate = rsaKey

	// Load TLS certificate
	certPath := filepath.Join(dataDir, "tls_certificate.pem")
	certPEMData, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read TLS certificate: %w", err)
	}
	block, _ = pem.Decode(certPEMData)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("invalid certificate PEM format")
	}
	keys.TLSCert = block.Bytes

	return keys, nil
}

// saveSecureFile writes data to a file with specified permissions atomically
func saveSecureFile(path string, data []byte, perm os.FileMode) error {
	// Write to temporary file first
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, perm); err != nil {
		return err
	}

	// Atomic rename
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath) // Clean up on failure
		return err
	}

	return nil
}

// Destroy securely zeroes and frees the relay keys
func (k *RelayKeys) Destroy() {
	if k.Ed25519Private != nil {
		security.SecureZeroMemory(k.Ed25519Private)
		k.Ed25519Private = nil
	}
	if k.Ed25519Public != nil {
		security.SecureZeroMemory(k.Ed25519Public)
		k.Ed25519Public = nil
	}
	// RSA key zeroing (best effort - Go doesn't expose internals)
	k.RSAPrivate = nil

	if k.TLSCert != nil {
		security.SecureZeroMemory(k.TLSCert)
		k.TLSCert = nil
	}
}
