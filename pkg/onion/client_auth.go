// Package onion - Client Authorization Implementation
// This file implements client authorization for v3 onion services per rend-spec-v3.txt §2.5
package onion

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"

	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"

	"github.com/opd-ai/go-tor/pkg/security"
)

// ClientAuthCredential represents a client authorization credential for accessing private onion services
// Per rend-spec-v3.txt §2.5, clients need x25519 key pairs to decrypt authorized descriptors
type ClientAuthCredential struct {
	OnionAddress string   // The .onion address this credential is for
	PrivateKey   [32]byte // x25519 private key for client authorization
	PublicKey    [32]byte // x25519 public key (derived from private key)
}

// ClientAuthStore manages client authorization credentials
type ClientAuthStore struct {
	credentials map[string]*ClientAuthCredential // key: onion address
}

// NewClientAuthStore creates a new client authorization store
func NewClientAuthStore() *ClientAuthStore {
	return &ClientAuthStore{
		credentials: make(map[string]*ClientAuthCredential),
	}
}

// AddCredential adds a client authorization credential for an onion service
// The privateKey should be a 32-byte x25519 private key
func (s *ClientAuthStore) AddCredential(onionAddress string, privateKey [32]byte) error {
	if len(onionAddress) == 0 {
		return fmt.Errorf("onion address cannot be empty")
	}

	// Derive public key from private key
	var publicKey [32]byte
	curve25519.ScalarBaseMult(&publicKey, &privateKey)

	credential := &ClientAuthCredential{
		OnionAddress: onionAddress,
		PrivateKey:   privateKey,
		PublicKey:    publicKey,
	}

	s.credentials[onionAddress] = credential
	return nil
}

// GetCredential retrieves a client authorization credential for an onion service
func (s *ClientAuthStore) GetCredential(onionAddress string) (*ClientAuthCredential, bool) {
	cred, exists := s.credentials[onionAddress]
	return cred, exists
}

// RemoveCredential removes a client authorization credential
func (s *ClientAuthStore) RemoveCredential(onionAddress string) {
	if cred, exists := s.credentials[onionAddress]; exists {
		// Securely zero the private key before removal
		security.SecureZeroMemory(cred.PrivateKey[:])
		delete(s.credentials, onionAddress)
	}
}

// Clear removes all credentials
func (s *ClientAuthStore) Clear() {
	for _, cred := range s.credentials {
		security.SecureZeroMemory(cred.PrivateKey[:])
	}
	s.credentials = make(map[string]*ClientAuthCredential)
}

// DecryptAuthDescriptor decrypts the encrypted layer of an authorized descriptor
// Per rend-spec-v3.txt §2.5, authorized descriptors have an additional encryption layer
// that requires the client's x25519 private key to decrypt
//
// Wire format of encrypted auth layer:
//
//	CLIENT_ID (8 bytes) || IV (16 bytes) || ENCRYPTED_DATA || MAC (16 bytes)
//
// Encryption uses AES-256-CTR with keys derived from:
//
//	shared_secret = X25519(client_private_key, service_public_key_for_auth)
func DecryptAuthDescriptor(encryptedData []byte, clientPrivateKey, servicePubKey [32]byte) ([]byte, error) {
	if len(encryptedData) < 40 { // CLIENT_ID(8) + IV(16) + MAC(16)
		return nil, fmt.Errorf("encrypted data too short: %d bytes", len(encryptedData))
	}

	// Extract components
	// CLIENT_ID is first 8 bytes (used to identify which client key to use)
	clientID := encryptedData[0:8]

	// IV is next 16 bytes
	iv := encryptedData[8:24]

	// Remaining data includes ciphertext and MAC
	ciphertextWithMAC := encryptedData[24:]

	if len(ciphertextWithMAC) < 16 {
		return nil, fmt.Errorf("insufficient data for MAC")
	}

	// MAC is last 16 bytes
	macOffset := len(ciphertextWithMAC) - 16
	ciphertext := ciphertextWithMAC[:macOffset]
	mac := ciphertextWithMAC[macOffset:]

	// Perform X25519 key exchange to get shared secret
	var sharedSecret [32]byte
	curve25519.ScalarMult(&sharedSecret, &clientPrivateKey, &servicePubKey)

	// Derive encryption and MAC keys using HKDF-SHA256
	// Per rend-spec-v3.txt §2.5: derive 64 bytes (32 for encryption, 32 for MAC)
	info := []byte("tor-hs-client-auth")
	keys, err := deriveAuthKeys(sharedSecret[:], clientID, info, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to derive keys: %w", err)
	}
	defer security.SecureZeroMemory(keys)

	encryptionKey := keys[0:32]
	macKey := keys[32:64]

	// Verify MAC before decryption
	// MAC covers: CLIENT_ID || IV || CIPHERTEXT
	macData := make([]byte, 0, len(clientID)+len(iv)+len(ciphertext))
	macData = append(macData, clientID...)
	macData = append(macData, iv...)
	macData = append(macData, ciphertext...)

	computedMAC := computeMAC(macKey, macData)
	if !security.ConstantTimeCompare(mac, computedMAC[:16]) {
		return nil, fmt.Errorf("MAC verification failed: descriptor authentication invalid")
	}

	// Decrypt using AES-256-CTR
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	plaintext := make([]byte, len(ciphertext))
	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(plaintext, ciphertext)

	return plaintext, nil
}

// deriveAuthKeys derives encryption and MAC keys for client authorization
// Uses HKDF-SHA256 with the shared secret, client ID as salt, and info string
func deriveAuthKeys(secret, salt, info []byte, length int) ([]byte, error) {
	kdf := hkdf.New(sha256.New, secret, salt, info)

	keys := make([]byte, length)
	if _, err := io.ReadFull(kdf, keys); err != nil {
		return nil, fmt.Errorf("HKDF derivation failed: %w", err)
	}

	return keys, nil
}

// computeMAC computes HMAC-SHA256 for MAC verification
func computeMAC(key, data []byte) []byte {
	h := sha256.New()
	h.Write(key)
	h.Write(data)
	return h.Sum(nil)
}

// ParseAuthClients parses auth-client entries from a descriptor
// Per rend-spec-v3.txt §2.5.1.1, authorized descriptors contain auth-client lines:
//
//	"auth-client" SP client-id SP iv SP encrypted-cookie
//
// Returns a map of client-id -> encrypted auth data
func ParseAuthClients(descriptorLines []string) (map[string][]byte, error) {
	authClients := make(map[string][]byte)

	for _, line := range descriptorLines {
		// Look for "auth-client" lines
		if len(line) < 12 || line[:11] != "auth-client" {
			continue
		}

		// Parse: auth-client <client-id> <iv> <encrypted-cookie>
		// Each field is base64-encoded
		fields := splitFields(line)
		if len(fields) != 4 {
			continue // Skip malformed lines
		}

		clientIDStr := fields[1]
		ivStr := fields[2]
		encCookieStr := fields[3]

		// Decode base64 fields
		clientID, err := base64.StdEncoding.DecodeString(clientIDStr)
		if err != nil {
			continue
		}

		iv, err := base64.StdEncoding.DecodeString(ivStr)
		if err != nil {
			continue
		}

		encCookie, err := base64.StdEncoding.DecodeString(encCookieStr)
		if err != nil {
			continue
		}

		// Combine into encrypted auth data: CLIENT_ID || IV || ENCRYPTED_COOKIE
		authData := make([]byte, 0, len(clientID)+len(iv)+len(encCookie))
		authData = append(authData, clientID...)
		authData = append(authData, iv...)
		authData = append(authData, encCookie...)

		authClients[clientIDStr] = authData
	}

	return authClients, nil
}

// splitFields splits a line into whitespace-separated fields
func splitFields(line string) []string {
	fields := make([]string, 0, 4)
	field := make([]byte, 0, 64)

	for i := 0; i < len(line); i++ {
		c := line[i]
		if c == ' ' || c == '\t' {
			if len(field) > 0 {
				fields = append(fields, string(field))
				field = field[:0]
			}
		} else {
			field = append(field, c)
		}
	}

	if len(field) > 0 {
		fields = append(fields, string(field))
	}

	return fields
}

// TryClientAuth attempts to decrypt an authorized descriptor using available credentials
// Returns the decrypted descriptor data if successful, error otherwise
func (c *Client) TryClientAuth(descriptor *Descriptor, address *Address) (*Descriptor, error) {
	if c.authStore == nil {
		// No auth store configured, descriptor is not authorized
		return descriptor, nil
	}

	// Check if we have a credential for this address
	cred, exists := c.authStore.GetCredential(address.String())
	if !exists {
		// No credential available, cannot decrypt authorized descriptor
		return nil, fmt.Errorf("descriptor requires client authorization but no credential available for %s", address.String())
	}

	c.logger.Debug("Attempting client authorization",
		"address", address.String())

	// Look for encrypted auth layer in descriptor
	// Per rend-spec-v3.txt §2.5, authorized descriptors have an "encrypted" section
	// that is separate from the superencrypted section

	// Parse the descriptor to find auth-client entries
	lines := splitDescriptorLines(descriptor.RawDescriptor)
	authClients, err := ParseAuthClients(lines)
	if err != nil {
		return nil, fmt.Errorf("failed to parse auth-client entries: %w", err)
	}

	if len(authClients) == 0 {
		// No auth-client entries found, descriptor may not be authorized
		c.logger.Debug("No auth-client entries found in descriptor")
		return descriptor, nil
	}

	c.logger.Debug("Found auth-client entries", "count", len(authClients))

	// Try to decrypt with our credential
	// We need to find our client-id in the auth entries
	var decryptedData []byte
	for clientIDStr, authData := range authClients {
		// Derive our client-id from our public key
		// Client-id is first 8 bytes of SHA256(client_public_key)
		h := sha256.New()
		h.Write(cred.PublicKey[:])
		derivedClientID := h.Sum(nil)[:8]
		derivedClientIDStr := base64.StdEncoding.EncodeToString(derivedClientID)

		if clientIDStr != derivedClientIDStr {
			continue // Not for us
		}

		c.logger.Debug("Found matching client-id, attempting decryption")

		// Extract service public key for auth from descriptor
		// This is typically the blinded public key
		var servicePubKey [32]byte
		if len(descriptor.BlindedPubkey) >= 32 {
			copy(servicePubKey[:], descriptor.BlindedPubkey[:32])
		} else {
			return nil, fmt.Errorf("invalid service public key in descriptor")
		}

		// Attempt decryption
		plaintext, err := DecryptAuthDescriptor(authData, cred.PrivateKey, servicePubKey)
		if err != nil {
			c.logger.Warn("Client auth decryption failed", "error", err)
			continue
		}

		decryptedData = plaintext
		c.logger.Info("Client authorization successful", "address", address.String())
		break
	}

	if decryptedData == nil {
		return nil, fmt.Errorf("client authorization failed: could not decrypt descriptor")
	}

	// Parse the decrypted data to update the descriptor
	// The decrypted data contains the descriptor content that was encrypted
	// This typically includes the introduction points
	decryptedDesc, err := parseDecryptedLayer(decryptedData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse decrypted auth layer: %w", err)
	}

	// Merge the decrypted introduction points into the descriptor
	descriptor.IntroPoints = decryptedDesc.IntroPoints

	return descriptor, nil
}

// splitDescriptorLines splits a descriptor into lines
func splitDescriptorLines(raw []byte) []string {
	lines := make([]string, 0, 50)
	line := make([]byte, 0, 256)

	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if c == '\n' {
			if len(line) > 0 {
				lines = append(lines, string(line))
				line = line[:0]
			}
		} else if c != '\r' {
			line = append(line, c)
		}
	}

	if len(line) > 0 {
		lines = append(lines, string(line))
	}

	return lines
}
