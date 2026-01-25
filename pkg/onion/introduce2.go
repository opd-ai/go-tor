// Package onion - INTRODUCE2 Cell Parsing
// This file implements INTRODUCE2 cell parsing for onion service hosting
// Following rend-spec-v3.txt §3.2-3.3
package onion

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"fmt"

	"golang.org/x/crypto/hkdf"
	"io"

	"github.com/opd-ai/go-tor/pkg/crypto"
)

// Introduce2Cell represents a parsed INTRODUCE2 cell from a client
type Introduce2Cell struct {
	// Encrypted data from client
	EncryptedData []byte

	// Decrypted contents
	AuthKey           []byte   // Client's authentication key (32 bytes)
	EncKey            []byte   // Client's encryption key (32 bytes)
	ClientPK          []byte   // Client's ephemeral public key for ntor (32 bytes)
	RendezvousCookie  []byte   // Rendezvous cookie (20 bytes)
	LinkSpecifiers    []byte   // Link specifiers for rendezvous point
	OnionKeyType      uint8    // Onion key type (0x00 = ntor)
	OnionKey          []byte   // Client's onion key (32 bytes for ntor)
	ExtensionFields   []byte   // Extension fields
}

// Introduce2Request contains parsed data needed to establish rendezvous
type Introduce2Request struct {
	RendezvousCookie []byte              // 20-byte cookie for rendezvous point
	LinkSpecifiers   []LinkSpecifier     // Rendezvous point address info
	ClientOnionKey   []byte              // Client's public key for ntor handshake
	ClientAuthKey    []byte              // Client's authentication key
	Extensions       map[uint8][]byte    // Extension data (type -> value)
}

// ParseIntroduce2 parses and decrypts an INTRODUCE2 cell
// Following rend-spec-v3.txt §3.2
//
// The INTRODUCE2 cell format:
//   AUTH_KEY_TYPE   [1 byte]
//   AUTH_KEY_LEN    [2 bytes]
//   AUTH_KEY        [AUTH_KEY_LEN bytes]
//   EXTENSIONS      [N bytes]
//   ENCRYPTED_DATA  [remaining bytes]
//
// The ENCRYPTED_DATA decrypts to:
//   RENDEZVOUS_COOKIE  [20 bytes]
//   NSPEC              [1 byte]
//   LINK_SPECIFIERS    [variable]
//   ONION_KEY_TYPE     [1 byte]
//   ONION_KEY_LEN      [2 bytes]
//   ONION_KEY          [ONION_KEY_LEN bytes]
//   EXTENSIONS         [variable]
func ParseIntroduce2(encryptedCell []byte, introAuthKey, introEncKey []byte) (*Introduce2Request, error) {
	if len(encryptedCell) < 100 {
		return nil, fmt.Errorf("INTRODUCE2 cell too short: %d bytes", len(encryptedCell))
	}

	offset := 0

	// Parse outer layer (before encryption)
	authKeyType := encryptedCell[offset]
	offset++
	if authKeyType != 0x02 { // ED25519-SHA3-256
		return nil, fmt.Errorf("unsupported auth key type: 0x%02x", authKeyType)
	}

	authKeyLen := binary.BigEndian.Uint16(encryptedCell[offset:offset+2])
	offset += 2
	if authKeyLen != 32 {
		return nil, fmt.Errorf("invalid auth key length: %d", authKeyLen)
	}

	clientAuthKey := make([]byte, authKeyLen)
	copy(clientAuthKey, encryptedCell[offset:offset+int(authKeyLen)])
	offset += int(authKeyLen)

	// Parse outer extensions
	if offset >= len(encryptedCell) {
		return nil, fmt.Errorf("truncated INTRODUCE2: no extension length")
	}
	nExtensions := encryptedCell[offset]
	offset++

	// Skip extensions (not used in basic implementation)
	for i := 0; i < int(nExtensions); i++ {
		if offset+3 > len(encryptedCell) {
			return nil, fmt.Errorf("truncated extension header")
		}
		// extType := encryptedCell[offset]
		offset++
		extLen := binary.BigEndian.Uint16(encryptedCell[offset:offset+2])
		offset += 2
		if offset+int(extLen) > len(encryptedCell) {
			return nil, fmt.Errorf("truncated extension data")
		}
		offset += int(extLen)
	}

	// Remaining data is encrypted
	encryptedData := encryptedCell[offset:]
	if len(encryptedData) < 32 { // Need at least MAC
		return nil, fmt.Errorf("encrypted data too short: %d bytes", len(encryptedData))
	}

	// Decrypt the inner layer
	// Format: ENCRYPTED_DATA || MAC (32 bytes)
	macSize := 32
	if len(encryptedData) < macSize {
		return nil, fmt.Errorf("no MAC in encrypted data")
	}

	ciphertext := encryptedData[:len(encryptedData)-macSize]
	mac := encryptedData[len(encryptedData)-macSize:]

	// Derive decryption keys from intro point keys
	// Using HKDF-SHA256 with intro enc key as input
	kdfInfo := []byte("tor-hs-ntor-curve25519-sha3-256-1:hs_key_extract")
	kdf := hkdf.New(sha256.New, introEncKey, nil, kdfInfo)
	keys := make([]byte, 64) // 32 for encryption, 32 for MAC
	if _, err := io.ReadFull(kdf, keys); err != nil {
		return nil, fmt.Errorf("key derivation failed: %w", err)
	}

	encKey := keys[0:32]
	macKey := keys[32:64]

	// Verify MAC
	computedMAC := hmac.New(sha256.New, macKey)
	computedMAC.Write(ciphertext)
	expectedMAC := computedMAC.Sum(nil)

	if !crypto.ConstantTimeCompare(mac, expectedMAC) {
		return nil, fmt.Errorf("MAC verification failed")
	}

	// Decrypt using AES-256-CTR (simplified - should use proper stream cipher)
	plaintext, err := crypto.DecryptAES256CTR(ciphertext, encKey, make([]byte, 16))
	if err != nil {
		return nil, fmt.Errorf("decryption failed: %w", err)
	}

	// Parse decrypted inner layer
	return parseIntroduce2Inner(plaintext, clientAuthKey)
}

// parseIntroduce2Inner parses the decrypted inner portion of INTRODUCE2
func parseIntroduce2Inner(plaintext []byte, clientAuthKey []byte) (*Introduce2Request, error) {
	if len(plaintext) < 20+1 {
		return nil, fmt.Errorf("decrypted data too short: %d bytes", len(plaintext))
	}

	offset := 0

	// Rendezvous cookie (20 bytes)
	rendezvousCookie := make([]byte, 20)
	copy(rendezvousCookie, plaintext[offset:offset+20])
	offset += 20

	// Link specifiers
	if offset >= len(plaintext) {
		return nil, fmt.Errorf("truncated: no NSPEC field")
	}
	nspec := plaintext[offset]
	offset++

	linkSpecifiers := make([]LinkSpecifier, 0, nspec)
	for i := 0; i < int(nspec); i++ {
		if offset+2 > len(plaintext) {
			return nil, fmt.Errorf("truncated link specifier %d", i)
		}
		lstype := plaintext[offset]
		offset++
		lslen := plaintext[offset]
		offset++

		if offset+int(lslen) > len(plaintext) {
			return nil, fmt.Errorf("truncated link specifier %d data", i)
		}

		lsdata := make([]byte, lslen)
		copy(lsdata, plaintext[offset:offset+int(lslen)])
		offset += int(lslen)

		linkSpecifiers = append(linkSpecifiers, LinkSpecifier{
			Type: lstype,
			Data: lsdata,
		})
	}

	// Onion key
	if offset+3 > len(plaintext) {
		return nil, fmt.Errorf("truncated: no onion key header")
	}
	onionKeyType := plaintext[offset]
	offset++
	if onionKeyType != 0x00 { // ntor
		return nil, fmt.Errorf("unsupported onion key type: 0x%02x", onionKeyType)
	}

	onionKeyLen := binary.BigEndian.Uint16(plaintext[offset:offset+2])
	offset += 2
	if onionKeyLen != 32 {
		return nil, fmt.Errorf("invalid ntor onion key length: %d", onionKeyLen)
	}

	if offset+int(onionKeyLen) > len(plaintext) {
		return nil, fmt.Errorf("truncated: onion key data")
	}
	clientOnionKey := make([]byte, onionKeyLen)
	copy(clientOnionKey, plaintext[offset:offset+int(onionKeyLen)])
	offset += int(onionKeyLen)

	// Parse inner extensions
	extensions := make(map[uint8][]byte)
	if offset < len(plaintext) {
		nExtensions := plaintext[offset]
		offset++

		for i := 0; i < int(nExtensions); i++ {
			if offset+3 > len(plaintext) {
				return nil, fmt.Errorf("truncated inner extension %d", i)
			}
			extType := plaintext[offset]
			offset++
			extLen := binary.BigEndian.Uint16(plaintext[offset:offset+2])
			offset += 2

			if offset+int(extLen) > len(plaintext) {
				return nil, fmt.Errorf("truncated inner extension %d data", i)
			}
			extData := make([]byte, extLen)
			copy(extData, plaintext[offset:offset+int(extLen)])
			offset += int(extLen)

			extensions[extType] = extData
		}
	}

	return &Introduce2Request{
		RendezvousCookie: rendezvousCookie,
		LinkSpecifiers:   linkSpecifiers,
		ClientOnionKey:   clientOnionKey,
		ClientAuthKey:    clientAuthKey,
		Extensions:       extensions,
	}, nil
}

// LinkSpecifierToAddress converts link specifiers to an address string
func LinkSpecifierToAddress(specs []LinkSpecifier) (string, error) {
	for _, spec := range specs {
		switch spec.Type {
		case 0x00: // TLS-over-TCP-IPv4
			if len(spec.Data) == 6 {
				ip := fmt.Sprintf("%d.%d.%d.%d", spec.Data[0], spec.Data[1], spec.Data[2], spec.Data[3])
				port := binary.BigEndian.Uint16(spec.Data[4:6])
				return fmt.Sprintf("%s:%d", ip, port), nil
			}
		case 0x01: // TLS-over-TCP-IPv6
			if len(spec.Data) == 18 {
				// IPv6 address format (simplified)
				port := binary.BigEndian.Uint16(spec.Data[16:18])
				return fmt.Sprintf("[%02x%02x:%02x%02x:%02x%02x:%02x%02x:%02x%02x:%02x%02x:%02x%02x:%02x%02x]:%d",
					spec.Data[0], spec.Data[1], spec.Data[2], spec.Data[3],
					spec.Data[4], spec.Data[5], spec.Data[6], spec.Data[7],
					spec.Data[8], spec.Data[9], spec.Data[10], spec.Data[11],
					spec.Data[12], spec.Data[13], spec.Data[14], spec.Data[15],
					port), nil
			}
		}
	}
	return "", fmt.Errorf("no supported link specifier found")
}
