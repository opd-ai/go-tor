// Package protocol provides CERTS cell parsing and validation per tor-spec.txt §4.2
package protocol

import (
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/opd-ai/go-tor/pkg/cell"
)

// CertType represents the type of certificate in a CERTS cell
// Per tor-spec.txt §4.2, different cert types serve different purposes
type CertType byte

const (
	// CertTypeTLSLink is a TLS link certificate (type 1)
	CertTypeTLSLink CertType = 0x01
	// CertTypeRSAID is an RSA identity certificate (type 2)
	CertTypeRSAID CertType = 0x02
	// CertTypeRSAAuth is an RSA authentication certificate (type 3)
	CertTypeRSAAuth CertType = 0x03
	// CertTypeEd25519Signing is an Ed25519 signing key certificate (type 4)
	CertTypeEd25519Signing CertType = 0x04
	// CertTypeEd25519TLSLink is an Ed25519 TLS link certificate (type 5)
	CertTypeEd25519TLSLink CertType = 0x05
	// CertTypeEd25519Auth is an Ed25519 authentication certificate (type 6)
	CertTypeEd25519Auth CertType = 0x06
	// CertTypeEd25519Identity is an RSA cross-certification of Ed25519 identity (type 7)
	CertTypeEd25519Identity CertType = 0x07
)

// String returns a human-readable representation of the cert type
func (ct CertType) String() string {
	switch ct {
	case CertTypeTLSLink:
		return "TLS_LINK"
	case CertTypeRSAID:
		return "RSA_ID"
	case CertTypeRSAAuth:
		return "RSA_AUTH"
	case CertTypeEd25519Signing:
		return "ED25519_SIGNING"
	case CertTypeEd25519TLSLink:
		return "ED25519_TLS_LINK"
	case CertTypeEd25519Auth:
		return "ED25519_AUTH"
	case CertTypeEd25519Identity:
		return "ED25519_IDENTITY"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", ct)
	}
}

// Certificate represents a single certificate from a CERTS cell
type Certificate struct {
	CertType CertType
	CertBody []byte
	// Parsed X.509 certificate (for RSA/TLS certs)
	X509Cert *x509.Certificate
	// Parsed Ed25519 certificate (for Ed25519 certs)
	Ed25519Cert *Ed25519Certificate
}

// Ed25519Certificate represents a Tor Ed25519 certificate per cert-spec.txt
type Ed25519Certificate struct {
	Version    uint8
	CertType   uint8
	ExpiresAt  time.Time
	CertKeyType uint8
	CertifiedKey []byte
	Extensions []Ed25519Extension
	Signature  []byte
}

// Ed25519Extension represents an extension in an Ed25519 certificate
type Ed25519Extension struct {
	ExtType  uint8
	Flags    uint8
	ExtData  []byte
}

// CERTSCell represents a parsed CERTS cell
type CERTSCell struct {
	Certificates []*Certificate
}

// ParseCERTSCell parses a CERTS cell payload per tor-spec.txt §4.2
// Format:
//   N             [1 octet]   Number of certificates
//   N times:
//     CertType    [1 octet]   Certificate type
//     CLEN        [2 octets]  Certificate length (big-endian)
//     Certificate [CLEN bytes] Certificate body
func ParseCERTSCell(cellData *cell.Cell) (*CERTSCell, error) {
	if cellData.Command != cell.CmdCerts {
		return nil, fmt.Errorf("not a CERTS cell: got %s", cellData.Command)
	}

	payload := cellData.Payload
	if len(payload) < 1 {
		return nil, fmt.Errorf("CERTS cell payload too short: %d bytes", len(payload))
	}

	numCerts := int(payload[0])
	offset := 1

	certs := &CERTSCell{
		Certificates: make([]*Certificate, 0, numCerts),
	}

	for i := 0; i < numCerts; i++ {
		if offset+3 > len(payload) {
			return nil, fmt.Errorf("truncated certificate header at offset %d", offset)
		}

		certType := CertType(payload[offset])
		certLen := binary.BigEndian.Uint16(payload[offset+1 : offset+3])
		offset += 3

		if offset+int(certLen) > len(payload) {
			return nil, fmt.Errorf("truncated certificate body: expected %d bytes at offset %d", certLen, offset)
		}

		certBody := payload[offset : offset+int(certLen)]
		offset += int(certLen)

		cert := &Certificate{
			CertType: certType,
			CertBody: make([]byte, len(certBody)),
		}
		copy(cert.CertBody, certBody)

		// Parse certificate based on type
		if err := parseCertificateBody(cert); err != nil {
			// Log error but continue - some cert types may not be critical
			// We'll validate required certs separately
			cert.X509Cert = nil
			cert.Ed25519Cert = nil
		}

		certs.Certificates = append(certs.Certificates, cert)
	}

	return certs, nil
}

// parseCertificateBody parses the certificate body based on its type
func parseCertificateBody(cert *Certificate) error {
	switch cert.CertType {
	case CertTypeTLSLink, CertTypeRSAID, CertTypeRSAAuth:
		// These are X.509 certificates
		x509Cert, err := x509.ParseCertificate(cert.CertBody)
		if err != nil {
			return fmt.Errorf("failed to parse X.509 certificate type %s: %w", cert.CertType, err)
		}
		cert.X509Cert = x509Cert
		return nil

	case CertTypeEd25519Signing, CertTypeEd25519TLSLink, CertTypeEd25519Auth, CertTypeEd25519Identity:
		// These are Ed25519 certificates per cert-spec.txt
		ed25519Cert, err := parseEd25519Certificate(cert.CertBody)
		if err != nil {
			return fmt.Errorf("failed to parse Ed25519 certificate type %s: %w", cert.CertType, err)
		}
		cert.Ed25519Cert = ed25519Cert
		return nil

	default:
		// Unknown certificate type - store body but don't parse
		return fmt.Errorf("unknown certificate type: %d", cert.CertType)
	}
}

// parseEd25519Certificate parses an Ed25519 certificate per cert-spec.txt
// Format:
//   Version       [1 octet]   Must be 1
//   CertType      [1 octet]   Type of certificate
//   ExpirationDate [4 octets]  Hours since epoch
//   CertKeyType   [1 octet]   Type of certified key
//   CertifiedKey  [32 octets] The key being certified
//   N             [1 octet]   Number of extensions
//   N times:
//     ExtLength   [2 octets]  Extension length
//     ExtType     [1 octet]   Extension type
//     ExtFlags    [1 octet]   Extension flags
//     ExtData     [ExtLength-2 octets] Extension data
//   Signature     [64 octets] Ed25519 signature
func parseEd25519Certificate(data []byte) (*Ed25519Certificate, error) {
	if len(data) < 40 { // Minimum: 1+1+4+1+32+1 = 40 bytes (no extensions, no signature yet)
		return nil, fmt.Errorf("Ed25519 certificate too short: %d bytes", len(data))
	}

	cert := &Ed25519Certificate{}
	offset := 0

	// Version (1 byte)
	cert.Version = data[offset]
	offset++
	if cert.Version != 1 {
		return nil, fmt.Errorf("unsupported Ed25519 certificate version: %d", cert.Version)
	}

	// CertType (1 byte)
	cert.CertType = data[offset]
	offset++

	// ExpirationDate (4 bytes, hours since epoch)
	expirationHours := binary.BigEndian.Uint32(data[offset : offset+4])
	offset += 4
	cert.ExpiresAt = time.Unix(int64(expirationHours)*3600, 0)

	// CertKeyType (1 byte)
	cert.CertKeyType = data[offset]
	offset++

	// CertifiedKey (32 bytes)
	if offset+32 > len(data) {
		return nil, fmt.Errorf("truncated certified key at offset %d", offset)
	}
	cert.CertifiedKey = make([]byte, 32)
	copy(cert.CertifiedKey, data[offset:offset+32])
	offset += 32

	// Number of extensions (1 byte)
	if offset >= len(data) {
		return nil, fmt.Errorf("truncated extension count at offset %d", offset)
	}
	numExtensions := int(data[offset])
	offset++

	// Parse extensions
	cert.Extensions = make([]Ed25519Extension, 0, numExtensions)
	for i := 0; i < numExtensions; i++ {
		if offset+2 > len(data) {
			return nil, fmt.Errorf("truncated extension length at offset %d", offset)
		}
		extLen := binary.BigEndian.Uint16(data[offset : offset+2])
		offset += 2

		if extLen < 2 {
			return nil, fmt.Errorf("invalid extension length: %d", extLen)
		}

		if offset+int(extLen) > len(data) {
			return nil, fmt.Errorf("truncated extension data at offset %d", offset)
		}

		ext := Ed25519Extension{
			ExtType: data[offset],
			Flags:   data[offset+1],
			ExtData: make([]byte, extLen-2),
		}
		copy(ext.ExtData, data[offset+2:offset+int(extLen)])
		offset += int(extLen)

		cert.Extensions = append(cert.Extensions, ext)
	}

	// Signature (64 bytes)
	if offset+64 > len(data) {
		return nil, fmt.Errorf("truncated signature at offset %d", offset)
	}
	cert.Signature = make([]byte, 64)
	copy(cert.Signature, data[offset:offset+64])

	return cert, nil
}

// FindCertificate finds a certificate of the given type in the CERTS cell
func (c *CERTSCell) FindCertificate(certType CertType) *Certificate {
	for _, cert := range c.Certificates {
		if cert.CertType == certType {
			return cert
		}
	}
	return nil
}

// ValidateRelayIdentity validates the relay identity using CERTS cell
// This verifies that the relay's claimed identity matches the certificates
// Per tor-spec.txt §4.2, we need:
//   1. RSA identity key certificate (type 2)
//   2. Ed25519 identity key certificate (type 4 or 7)
func (c *CERTSCell) ValidateRelayIdentity(expectedRSAFingerprint string, expectedEd25519Identity []byte) error {
	// Check for RSA identity certificate if fingerprint provided
	if expectedRSAFingerprint != "" {
		rsaCert := c.FindCertificate(CertTypeRSAID)
		if rsaCert == nil || rsaCert.X509Cert == nil {
			return fmt.Errorf("missing RSA identity certificate")
		}

		// Verify RSA fingerprint
		rsaPubKey, ok := rsaCert.X509Cert.PublicKey.(*rsa.PublicKey)
		if !ok {
			return fmt.Errorf("RSA identity cert does not contain RSA public key")
		}

		// Calculate fingerprint (SHA-1 of DER-encoded RSA public key)
		derBytes, err := x509.MarshalPKIXPublicKey(rsaPubKey)
		if err != nil {
			return fmt.Errorf("failed to encode RSA public key: %w", err)
		}
		
		// For Tor, we use SHA-256 of the DER encoding
		fingerprint := sha256.Sum256(derBytes)
		fingerprintHex := fmt.Sprintf("%X", fingerprint[:20]) // Use first 20 bytes for compatibility
		
		if fingerprintHex != expectedRSAFingerprint {
			return fmt.Errorf("RSA identity mismatch: expected %s, got %s", expectedRSAFingerprint, fingerprintHex)
		}
	}

	// Check for Ed25519 identity certificate if identity provided
	if len(expectedEd25519Identity) > 0 {
		// Try type 4 (Ed25519 signing key) first
		ed25519Cert := c.FindCertificate(CertTypeEd25519Signing)
		if ed25519Cert == nil {
			// Try type 7 (cross-certification)
			ed25519Cert = c.FindCertificate(CertTypeEd25519Identity)
		}
		
		if ed25519Cert == nil || ed25519Cert.Ed25519Cert == nil {
			return fmt.Errorf("missing Ed25519 identity certificate")
		}

		// Verify Ed25519 identity matches
		certifiedKey := ed25519Cert.Ed25519Cert.CertifiedKey
		if len(certifiedKey) != 32 {
			return fmt.Errorf("invalid Ed25519 certified key length: %d", len(certifiedKey))
		}

		// Compare with expected identity
		if len(expectedEd25519Identity) != 32 {
			return fmt.Errorf("invalid expected Ed25519 identity length: %d", len(expectedEd25519Identity))
		}

		for i := 0; i < 32; i++ {
			if certifiedKey[i] != expectedEd25519Identity[i] {
				return fmt.Errorf("Ed25519 identity mismatch")
			}
		}
	}

	return nil
}

// ValidateExpiration checks if any certificates in the CERTS cell have expired
func (c *CERTSCell) ValidateExpiration() error {
	now := time.Now()
	
	for _, cert := range c.Certificates {
		if cert.X509Cert != nil {
			if now.After(cert.X509Cert.NotAfter) {
				return fmt.Errorf("X.509 certificate type %s expired at %v", cert.CertType, cert.X509Cert.NotAfter)
			}
			if now.Before(cert.X509Cert.NotBefore) {
				return fmt.Errorf("X.509 certificate type %s not yet valid (valid from %v)", cert.CertType, cert.X509Cert.NotBefore)
			}
		}
		
		if cert.Ed25519Cert != nil {
			if now.After(cert.Ed25519Cert.ExpiresAt) {
				return fmt.Errorf("Ed25519 certificate type %s expired at %v", cert.CertType, cert.Ed25519Cert.ExpiresAt)
			}
		}
	}
	
	return nil
}

// VerifySignature verifies the Ed25519 signature on a certificate.
// Per cert-spec.txt, the signature is over all bytes of the certificate
// before the signature field itself.
//
// Parameters:
//   - signingKey: The Ed25519 public key used to create the signature (32 bytes)
//     For self-signed certs, this is the certified key itself.
//     For cross-signed certs, this is the signing authority's key.
//
// Returns:
//   - error if verification fails, nil if signature is valid
func (e *Ed25519Certificate) VerifySignature(signingKey []byte) error {
	if len(signingKey) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid signing key length: %d, expected %d", len(signingKey), ed25519.PublicKeySize)
	}
	
	if len(e.Signature) != ed25519.SignatureSize {
		return fmt.Errorf("invalid signature length: %d, expected %d", len(e.Signature), ed25519.SignatureSize)
	}
	
	// Reconstruct the signed message (all fields before signature)
	// Per cert-spec.txt: Version || CertType || ExpirationDate || CertKeyType || 
	// CertifiedKey || NumExtensions || Extensions
	signedData := make([]byte, 0, 256)
	
	// Version (1 byte)
	signedData = append(signedData, e.Version)
	
	// CertType (1 byte)
	signedData = append(signedData, e.CertType)
	
	// ExpirationDate (4 bytes, hours since epoch)
	expirationHours := uint32(e.ExpiresAt.Unix() / 3600)
	expBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(expBytes, expirationHours)
	signedData = append(signedData, expBytes...)
	
	// CertKeyType (1 byte)
	signedData = append(signedData, e.CertKeyType)
	
	// CertifiedKey (32 bytes)
	signedData = append(signedData, e.CertifiedKey...)
	
	// Number of extensions (1 byte)
	signedData = append(signedData, byte(len(e.Extensions)))
	
	// Extensions
	for _, ext := range e.Extensions {
		// ExtLength (2 bytes) - length of (ExtType + ExtFlags + ExtData)
		extLen := uint16(2 + len(ext.ExtData))
		extLenBytes := make([]byte, 2)
		binary.BigEndian.PutUint16(extLenBytes, extLen)
		signedData = append(signedData, extLenBytes...)
		
		// ExtType (1 byte)
		signedData = append(signedData, ext.ExtType)
		
		// ExtFlags (1 byte)
		signedData = append(signedData, ext.Flags)
		
		// ExtData
		signedData = append(signedData, ext.ExtData...)
	}
	
	// Verify signature using Ed25519
	if !ed25519.Verify(ed25519.PublicKey(signingKey), signedData, e.Signature) {
		return fmt.Errorf("Ed25519 signature verification failed")
	}
	
	return nil
}

// ValidateSignatures verifies Ed25519 certificate signatures in the CERTS cell
// This implements cryptographic signature verification per cert-spec.txt
//
// For Type 4 (Ed25519 signing key), the certificate should be self-signed
// or signed by the identity key found in another certificate.
func (c *CERTSCell) ValidateSignatures() error {
	for _, cert := range c.Certificates {
		if cert.Ed25519Cert == nil {
			continue
		}
		
		switch cert.CertType {
		case CertTypeEd25519Signing:
			// Type 4: Ed25519 signing key certificate
			// This is typically signed by the identity key (certified key itself for self-signed)
			// For initial handshake, we verify it's self-signed
			if err := cert.Ed25519Cert.VerifySignature(cert.Ed25519Cert.CertifiedKey); err != nil {
				return fmt.Errorf("type 4 (Ed25519 signing key) signature verification failed: %w", err)
			}
			
		case CertTypeEd25519TLSLink:
			// Type 5: Ed25519 TLS link certificate
			// Signed by the Ed25519 signing key (type 4)
			signingKeyCert := c.FindCertificate(CertTypeEd25519Signing)
			if signingKeyCert == nil || signingKeyCert.Ed25519Cert == nil {
				return fmt.Errorf("type 5 (Ed25519 TLS link) requires type 4 signing key cert")
			}
			if err := cert.Ed25519Cert.VerifySignature(signingKeyCert.Ed25519Cert.CertifiedKey); err != nil {
				return fmt.Errorf("type 5 (Ed25519 TLS link) signature verification failed: %w", err)
			}
			
		case CertTypeEd25519Auth:
			// Type 6: Ed25519 authentication certificate
			// Signed by the Ed25519 signing key (type 4)
			signingKeyCert := c.FindCertificate(CertTypeEd25519Signing)
			if signingKeyCert == nil || signingKeyCert.Ed25519Cert == nil {
				return fmt.Errorf("type 6 (Ed25519 auth) requires type 4 signing key cert")
			}
			if err := cert.Ed25519Cert.VerifySignature(signingKeyCert.Ed25519Cert.CertifiedKey); err != nil {
				return fmt.Errorf("type 6 (Ed25519 auth) signature verification failed: %w", err)
			}
		}
	}
	
	return nil
}
