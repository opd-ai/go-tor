// Example: INTRODUCE2 Cell Parsing
//
// This example demonstrates how to parse INTRODUCE2 cells from clients
// connecting to an onion service.
//
// Usage:
//
//	go run main.go
package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"log"

	"github.com/opd-ai/go-tor/pkg/crypto"
	"github.com/opd-ai/go-tor/pkg/onion"
	"golang.org/x/crypto/hkdf"
)

func main() {
	fmt.Println("=== INTRODUCE2 Cell Parsing Example ===")
	fmt.Println()

	// Step 1: Generate introduction point keys
	// In a real service, these would be established during ESTABLISH_INTRO
	fmt.Println("1. Generating introduction point keys...")
	introAuthKey := make([]byte, 32)
	introEncKey := make([]byte, 32)
	if _, err := rand.Read(introAuthKey); err != nil {
		log.Fatalf("Failed to generate auth key: %v", err)
	}
	if _, err := rand.Read(introEncKey); err != nil {
		log.Fatalf("Failed to generate enc key: %v", err)
	}
	fmt.Printf("   Auth Key: %x...\n", introAuthKey[:8])
	fmt.Printf("   Enc Key:  %x...\n", introEncKey[:8])

	// Step 2: Create a mock INTRODUCE2 cell (simulating client side)
	fmt.Println("\n2. Creating mock INTRODUCE2 cell...")
	introduce2Cell := createMockIntroduce2(introEncKey)
	fmt.Printf("   Cell size: %d bytes\n", len(introduce2Cell))

	// Step 3: Parse the INTRODUCE2 cell
	fmt.Println("\n3. Parsing INTRODUCE2 cell...")
	request, err := onion.ParseIntroduce2(introduce2Cell, introAuthKey, introEncKey)
	if err != nil {
		log.Fatalf("Failed to parse INTRODUCE2: %v", err)
	}
	fmt.Println("   ✓ Successfully parsed and decrypted")

	// Step 4: Display parsed information
	fmt.Println("\n4. Parsed Information:")
	fmt.Printf("   Rendezvous Cookie: %x\n", request.RendezvousCookie)
	fmt.Printf("   Client Onion Key:  %x...\n", request.ClientOnionKey[:16])
	fmt.Printf("   Client Auth Key:   %x...\n", request.ClientAuthKey[:16])
	fmt.Printf("   Link Specifiers:   %d\n", len(request.LinkSpecifiers))

	// Step 5: Extract rendezvous point address
	fmt.Println("\n5. Extracting Rendezvous Point:")
	address, err := onion.LinkSpecifierToAddress(request.LinkSpecifiers)
	if err != nil {
		log.Printf("   Warning: Could not extract address: %v", err)
	} else {
		fmt.Printf("   Rendezvous Address: %s\n", address)
	}

	fmt.Println("\n=== Next Steps ===")
	fmt.Println("In a complete implementation, the service would now:")
	fmt.Println("  1. Build a circuit to the rendezvous point")
	fmt.Println("  2. Perform ntor handshake with the client")
	fmt.Println("  3. Send RENDEZVOUS1 cell with handshake response")
	fmt.Println("  4. Establish end-to-end encrypted connection")
}

// createMockIntroduce2 creates a properly formatted INTRODUCE2 cell for testing
func createMockIntroduce2(introEncKey []byte) []byte {
	// Generate client keys
	clientAuthKey := make([]byte, 32)
	clientOnionKey := make([]byte, 32)
	rendezvousCookie := make([]byte, 20)
	rand.Read(clientAuthKey)
	rand.Read(clientOnionKey)
	rand.Read(rendezvousCookie)

	// Build inner plaintext
	innerPlaintext := make([]byte, 0)
	innerPlaintext = append(innerPlaintext, rendezvousCookie...) // 20 bytes
	innerPlaintext = append(innerPlaintext, 0x01)                // NSPEC: 1 link specifier

	// Add IPv4 link specifier (192.0.2.1:9001)
	innerPlaintext = append(innerPlaintext, 0x00)         // Type: IPv4
	innerPlaintext = append(innerPlaintext, 0x06)         // Length: 6
	innerPlaintext = append(innerPlaintext, 192, 0, 2, 1) // IP
	innerPlaintext = append(innerPlaintext, 0x23, 0x29)   // Port: 9001

	// Add onion key
	innerPlaintext = append(innerPlaintext, 0x00)       // Type: ntor
	innerPlaintext = append(innerPlaintext, 0x00, 0x20) // Length: 32
	innerPlaintext = append(innerPlaintext, clientOnionKey...)
	innerPlaintext = append(innerPlaintext, 0x00) // No extensions

	// Derive encryption and MAC keys
	kdfInfo := []byte("tor-hs-ntor-curve25519-sha3-256-1:hs_key_extract")
	kdf := hkdf.New(sha256.New, introEncKey, nil, kdfInfo)
	keys := make([]byte, 64)
	io.ReadFull(kdf, keys)

	encKey := keys[0:32]
	macKey := keys[32:64]

	// Encrypt inner plaintext
	ciphertext, _ := crypto.EncryptAES256CTR(innerPlaintext, encKey, make([]byte, 16))

	// Compute MAC
	mac := hmac.New(sha256.New, macKey)
	mac.Write(ciphertext)
	macValue := mac.Sum(nil)

	// Build encrypted data
	encryptedData := append(ciphertext, macValue...)

	// Build outer layer
	outerCell := make([]byte, 0)
	outerCell = append(outerCell, 0x02)       // Auth key type: ED25519-SHA3-256
	outerCell = append(outerCell, 0x00, 0x20) // Auth key len: 32
	outerCell = append(outerCell, clientAuthKey...)
	outerCell = append(outerCell, 0x00) // No outer extensions
	outerCell = append(outerCell, encryptedData...)

	return outerCell
}
