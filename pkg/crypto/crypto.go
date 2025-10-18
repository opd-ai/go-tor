// Package crypto provides cryptographic primitives for the Tor protocol.
// This package wraps Go's standard crypto libraries for Tor-specific operations.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"fmt"
	"io"
)

// Key sizes
const (
	// AES128KeySize is the size of AES-128 keys
	AES128KeySize = 16
	// AES256KeySize is the size of AES-256 keys
	AES256KeySize = 32
	// SHA1Size is the size of SHA-1 digests
	SHA1Size = 20
	// SHA256Size is the size of SHA-256 digests
	SHA256Size = 32
)

// GenerateRandomBytes generates n random bytes using crypto/rand
func GenerateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return b, nil
}

// SHA1Hash computes the SHA-1 hash of the input
func SHA1Hash(data []byte) []byte {
	h := sha1.Sum(data)
	return h[:]
}

// SHA256Hash computes the SHA-256 hash of the input
func SHA256Hash(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

// AESCTRCipher represents an AES-CTR cipher for encryption/decryption
type AESCTRCipher struct {
	stream cipher.Stream
}

// NewAESCTRCipher creates a new AES-CTR cipher with the given key and IV
func NewAESCTRCipher(key, iv []byte) (*AESCTRCipher, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	stream := cipher.NewCTR(block, iv)
	return &AESCTRCipher{stream: stream}, nil
}

// Encrypt encrypts the plaintext in-place using AES-CTR
func (c *AESCTRCipher) Encrypt(plaintext []byte) {
	c.stream.XORKeyStream(plaintext, plaintext)
}

// Decrypt decrypts the ciphertext in-place using AES-CTR
func (c *AESCTRCipher) Decrypt(ciphertext []byte) {
	// In CTR mode, encryption and decryption are the same operation
	c.stream.XORKeyStream(ciphertext, ciphertext)
}

// RSAPublicKey wraps an RSA public key
type RSAPublicKey struct {
	key *rsa.PublicKey
}

// RSAPrivateKey wraps an RSA private key
type RSAPrivateKey struct {
	key *rsa.PrivateKey
}

// GenerateRSAKey generates a new RSA key pair with the given bit size
func GenerateRSAKey(bits int) (*RSAPrivateKey, error) {
	key, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, fmt.Errorf("failed to generate RSA key: %w", err)
	}
	return &RSAPrivateKey{key: key}, nil
}

// PublicKey returns the public key corresponding to the private key
func (k *RSAPrivateKey) PublicKey() *RSAPublicKey {
	return &RSAPublicKey{key: &k.key.PublicKey}
}

// Encrypt encrypts data using RSA OAEP with SHA-1
func (k *RSAPublicKey) Encrypt(plaintext []byte) ([]byte, error) {
	ciphertext, err := rsa.EncryptOAEP(sha1.New(), rand.Reader, k.key, plaintext, nil)
	if err != nil {
		return nil, fmt.Errorf("RSA encryption failed: %w", err)
	}
	return ciphertext, nil
}

// Decrypt decrypts data using RSA OAEP with SHA-1
func (k *RSAPrivateKey) Decrypt(ciphertext []byte) ([]byte, error) {
	plaintext, err := rsa.DecryptOAEP(sha1.New(), rand.Reader, k.key, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("RSA decryption failed: %w", err)
	}
	return plaintext, nil
}

// DigestWriter wraps a hash writer for computing running digests
type DigestWriter struct {
	hash io.Writer
}

// NewSHA1DigestWriter creates a new SHA-1 digest writer
func NewSHA1DigestWriter() *DigestWriter {
	return &DigestWriter{hash: sha1.New()}
}

// Write writes data to the digest
func (d *DigestWriter) Write(p []byte) (n int, err error) {
	return d.hash.Write(p)
}

// DeriveKey derives key material using KDF-TOR
// KDF-TOR uses iterative SHA-1 hashing to expand a shared secret
func DeriveKey(secret []byte, keyLen int) ([]byte, error) {
	if keyLen <= 0 {
		return nil, fmt.Errorf("invalid key length: %d", keyLen)
	}

	// KDF-TOR: K = K_0 | K_1 | K_2 | ...
	// Where K_i = H(K_0 | [i])
	// And K_0 = H(secret)

	k0 := SHA1Hash(secret)
	result := make([]byte, 0, keyLen)

	// Append K_0
	result = append(result, k0...)

	// Generate additional blocks if needed
	i := byte(1)
	for len(result) < keyLen {
		// K_i = H(K_0 | [i])
		data := append(k0, i)
		ki := SHA1Hash(data)
		result = append(result, ki...)
		i++
	}

	// Return exactly keyLen bytes
	return result[:keyLen], nil
}
