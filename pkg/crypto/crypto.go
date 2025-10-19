// Package crypto provides cryptographic primitives for the Tor protocol.
// This package wraps Go's standard crypto libraries for Tor-specific operations.
//
// Security considerations:
// - All random number generation uses crypto/rand (CSPRNG)
// - Sensitive data should be zeroed after use (see security.SecureZeroMemory)
// - Key comparisons should use constant-time operations (see security.ConstantTimeCompare)
// - Memory containing keys should be zeroed before being freed
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1" // #nosec G505 - SHA1 required by Tor protocol specification (tor-spec.txt)
	"crypto/sha256"
	"fmt"
	"io"

	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
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
// #nosec G401 - SHA1 required by Tor specification (tor-spec.txt section 0.3)
// SHA1 is mandated by the Tor protocol for specific operations and cannot be replaced
// without breaking protocol compatibility. It is not used for collision-resistant purposes.
func SHA1Hash(data []byte) []byte {
	h := sha1.Sum(data) // #nosec G401
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
// #nosec G401 - SHA1 with RSA-OAEP required by Tor specification (tor-spec.txt section 0.3)
// The Tor protocol mandates RSA-1024-OAEP-SHA1 for hybrid encryption.
func (k *RSAPublicKey) Encrypt(plaintext []byte) ([]byte, error) {
	ciphertext, err := rsa.EncryptOAEP(sha1.New(), rand.Reader, k.key, plaintext, nil) // #nosec G401
	if err != nil {
		return nil, fmt.Errorf("RSA encryption failed: %w", err)
	}
	return ciphertext, nil
}

// Decrypt decrypts data using RSA OAEP with SHA-1
// #nosec G401 - SHA1 with RSA-OAEP required by Tor specification (tor-spec.txt section 0.3)
// The Tor protocol mandates RSA-1024-OAEP-SHA1 for hybrid encryption.
func (k *RSAPrivateKey) Decrypt(ciphertext []byte) ([]byte, error) {
	plaintext, err := rsa.DecryptOAEP(sha1.New(), rand.Reader, k.key, ciphertext, nil) // #nosec G401
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
// #nosec G401 - SHA1 required by Tor specification (tor-spec.txt)
// SHA1 is mandated by the Tor protocol for computing digests in various protocol operations.
func NewSHA1DigestWriter() *DigestWriter {
	return &DigestWriter{hash: sha1.New()} // #nosec G401
}

// Write writes data to the digest
func (d *DigestWriter) Write(p []byte) (n int, err error) {
	return d.hash.Write(p)
}

// DeriveKey derives key material using KDF-TOR
// KDF-TOR uses iterative SHA-1 hashing to expand a shared secret
//
// Security note: The caller is responsible for zeroing the returned key material
// when it's no longer needed using security.SecureZeroMemory()
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

// NtorKeyPair represents a Curve25519 key pair for ntor handshake
type NtorKeyPair struct {
	Private [32]byte
	Public  [32]byte
}

// GenerateNtorKeyPair generates a new Curve25519 key pair for ntor handshake
// This implements tor-spec.txt section 5.1.4 (ntor handshake)
func GenerateNtorKeyPair() (*NtorKeyPair, error) {
	kp := &NtorKeyPair{}
	
	// Generate random private key
	if _, err := rand.Read(kp.Private[:]); err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}
	
	// Compute public key: X = x*G
	curve25519.ScalarBaseMult(&kp.Public, &kp.Private)
	
	return kp, nil
}

// NtorClientHandshake performs the client side of the ntor handshake
// Returns the handshake data to send to the relay and the shared secret
// 
// Parameters:
//   - identityKey: The relay's Ed25519 identity key (32 bytes)
//   - ntorOnionKey: The relay's ntor onion key (32 bytes)
//
// Returns:
//   - handshakeData: The data to send in CREATE2/EXTEND2 cell
//   - sharedSecret: The derived shared secret for KDF
//
// Implements tor-spec.txt section 5.1.4
func NtorClientHandshake(identityKey, ntorOnionKey []byte) (handshakeData []byte, sharedSecret []byte, err error) {
	if len(identityKey) != 32 {
		return nil, nil, fmt.Errorf("invalid identity key length: %d", len(identityKey))
	}
	if len(ntorOnionKey) != 32 {
		return nil, nil, fmt.Errorf("invalid ntor onion key length: %d", len(ntorOnionKey))
	}
	
	// Generate ephemeral key pair (x, X)
	ephemeral, err := GenerateNtorKeyPair()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate ephemeral key: %w", err)
	}
	
	// Handshake data is: NODEID || KEYID || CLIENT_PK
	// NODEID (20 bytes): relay identity fingerprint (we use first 20 bytes of Ed25519 key)
	// KEYID (32 bytes): relay's ntor onion key  
	// CLIENT_PK (32 bytes): client's ephemeral public key X
	handshakeData = make([]byte, 20+32+32)
	copy(handshakeData[0:20], identityKey[0:20])     // NODEID
	copy(handshakeData[20:52], ntorOnionKey)         // KEYID
	copy(handshakeData[52:84], ephemeral.Public[:])  // CLIENT_PK
	
	// Note: The complete ntor handshake requires processing the server's response
	// to compute the actual shared secret. For now, we return a placeholder.
	// A full implementation would:
	// 1. Receive server's Y and auth from CREATED2/EXTENDED2
	// 2. Compute shared secrets: EXP(Y,x) and EXP(B,x)
	// 3. Derive key material using HKDF-SHA256
	// 4. Verify auth MAC
	
	// Placeholder shared secret (will be replaced when processing server response)
	sharedSecret = make([]byte, 32)
	copy(sharedSecret, ephemeral.Private[:])
	
	return handshakeData, sharedSecret, nil
}

// NtorProcessResponse processes the server's response to complete the ntor handshake
// 
// Parameters:
//   - response: The server's response from CREATED2/EXTENDED2 (HLEN bytes of handshake data)
//   - clientPrivate: The client's ephemeral private key from the initial handshake
//   - serverNtorKey: The relay's ntor onion key (32 bytes)
//   - serverIdentity: The relay's identity key (32 bytes)
//
// Returns:
//   - sharedSecret: The verified shared secret for key derivation
//
// Implements tor-spec.txt section 5.1.4
func NtorProcessResponse(response []byte, clientPrivate, serverNtorKey, serverIdentity []byte) ([]byte, error) {
	// Expected response: Y (32 bytes) || AUTH (32 bytes)
	if len(response) != 64 {
		return nil, fmt.Errorf("invalid response length: %d, expected 64", len(response))
	}
	
	var serverY, auth [32]byte
	copy(serverY[:], response[0:32])
	copy(auth[:], response[32:64])
	
	// Convert client private key
	var clientX [32]byte
	copy(clientX[:], clientPrivate)
	
	// Compute shared secrets
	// secret_input = EXP(Y,x) | EXP(B,x) | ID | B | X | Y | PROTOID
	// where PROTOID = "ntor-curve25519-sha256-1"
	
	var sharedXY, sharedXB [32]byte
	
	// EXP(Y,x) - Diffie-Hellman with server's ephemeral key
	curve25519.ScalarMult(&sharedXY, &clientX, &serverY)
	
	// EXP(B,x) - Diffie-Hellman with server's ntor onion key
	var serverB [32]byte
	copy(serverB[:], serverNtorKey)
	curve25519.ScalarMult(&sharedXB, &clientX, &serverB)
	
	// Build secret_input
	protoid := []byte("ntor-curve25519-sha256-1")
	secretInput := make([]byte, 0, 32+32+32+32+32+32+len(protoid))
	secretInput = append(secretInput, sharedXY[:]...)
	secretInput = append(secretInput, sharedXB[:]...)
	secretInput = append(secretInput, serverIdentity[0:32]...)
	secretInput = append(secretInput, serverNtorKey...)
	
	var clientPub [32]byte
	curve25519.ScalarBaseMult(&clientPub, &clientX)
	secretInput = append(secretInput, clientPub[:]...)
	secretInput = append(secretInput, serverY[:]...)
	secretInput = append(secretInput, protoid...)
	
	// Derive keys using HKDF-SHA256 per tor-spec.txt section 5.1.4
	// The handshake uses two derivation steps:
	// 1. verify = HKDF(secret_input, t_verify, M_EXPAND) for auth verification
	// 2. key_material = HKDF(secret_input, t_key, M_EXPAND) for circuit keys
	
	// First derive the verification key to check the AUTH value
	verify := []byte("ntor-curve25519-sha256-1:verify")
	hkdfVerify := hkdf.New(sha256.New, secretInput, nil, verify)
	expectedAuth := make([]byte, 32)
	if _, err := io.ReadFull(hkdfVerify, expectedAuth); err != nil {
		return nil, fmt.Errorf("HKDF verify derivation failed: %w", err)
	}
	
	// Verify the AUTH value matches our computation (constant-time comparison)
	// This ensures the server has the correct private keys
	if !constantTimeCompare(auth[:], expectedAuth) {
		return nil, fmt.Errorf("auth MAC verification failed: server authentication invalid")
	}
	
	// Now derive the actual key material for circuit use
	keyInfo := []byte("ntor-curve25519-sha256-1:key_extract")
	hkdfKey := hkdf.New(sha256.New, secretInput, nil, keyInfo)
	keyMaterial := make([]byte, 72) // Tor uses 72 bytes of key material
	if _, err := io.ReadFull(hkdfKey, keyMaterial); err != nil {
		return nil, fmt.Errorf("HKDF key derivation failed: %w", err)
	}
	
	return keyMaterial, nil
}

// constantTimeCompare performs constant-time comparison of two byte slices
// This prevents timing attacks when comparing cryptographic values
func constantTimeCompare(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	
	var result byte = 0
	for i := 0; i < len(a); i++ {
		result |= a[i] ^ b[i]
	}
	return result == 0
}

// Ed25519Verify verifies an Ed25519 signature
// This is used for onion service descriptor signature verification
// Implements rend-spec-v3.txt section 2.1
func Ed25519Verify(publicKey, message, signature []byte) bool {
	if len(publicKey) != ed25519.PublicKeySize {
		return false
	}
	if len(signature) != ed25519.SignatureSize {
		return false
	}
	
	return ed25519.Verify(ed25519.PublicKey(publicKey), message, signature)
}

// Ed25519Sign signs a message with an Ed25519 private key
func Ed25519Sign(privateKey, message []byte) ([]byte, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid private key length: %d", len(privateKey))
	}
	
	signature := ed25519.Sign(ed25519.PrivateKey(privateKey), message)
	return signature, nil
}

// GenerateEd25519KeyPair generates a new Ed25519 key pair
func GenerateEd25519KeyPair() (publicKey, privateKey []byte, err error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate Ed25519 key: %w", err)
	}
	return pub, priv, nil
}
