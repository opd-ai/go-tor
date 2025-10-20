// Package main demonstrates onion address parsing and validation.
// This example shows how to use the onion package to work with v3 .onion addresses.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"log"
	"strings"

	"github.com/opd-ai/go-tor/pkg/onion"
)

func main() {
	fmt.Println("=== Onion Address Demo ===")
	fmt.Println()

	// Example 1: Parse and validate a known onion address
	fmt.Println("Example 1: Parsing a v3 onion address")
	fmt.Println("---------------------------------------")
	exampleAddr := "vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd.onion"
	fmt.Printf("Address: %s\n", exampleAddr)

	addr, err := onion.ParseAddress(exampleAddr)
	if err != nil {
		fmt.Printf("Error parsing address: %v\n", err)
	} else {
		fmt.Printf("✓ Valid v3 address!\n")
		fmt.Printf("  Version: %d\n", addr.Version)
		fmt.Printf("  Public key length: %d bytes\n", len(addr.Pubkey))
		fmt.Printf("  Public key (hex): %x...\n", addr.Pubkey[:8])
	}
	fmt.Println()

	// Example 2: Check if strings are onion addresses
	fmt.Println("Example 2: Checking if strings are onion addresses")
	fmt.Println("---------------------------------------------------")
	testAddresses := []string{
		"example.com",
		"vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd.onion",
		"192.168.1.1",
		"invalid.onion",
	}

	for _, testAddr := range testAddresses {
		isOnion := onion.IsOnionAddress(testAddr)
		if isOnion {
			fmt.Printf("✓ %s IS an onion address\n", testAddr)
		} else {
			fmt.Printf("✗ %s is NOT an onion address\n", testAddr)
		}
	}
	fmt.Println()

	// Example 3: Generate and encode a new onion address
	fmt.Println("Example 3: Generating a new v3 onion address")
	fmt.Println("---------------------------------------------")
	newAddr := generateOnionAddress()
	fmt.Printf("Generated address: %s\n", newAddr)

	// Parse it to verify
	_, err = onion.ParseAddress(newAddr)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("✓ Successfully generated and validated new address\n")
		fmt.Printf("  Length: %d characters (excluding .onion)\n", len(strings.TrimSuffix(newAddr, ".onion")))
	}
	fmt.Println()

	// Example 4: Round-trip encoding
	fmt.Println("Example 4: Round-trip encoding test")
	fmt.Println("------------------------------------")
	original := "vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd.onion"
	fmt.Printf("Original: %s\n", original)

	// Parse
	addr1, err := onion.ParseAddress(original)
	if err != nil {
		log.Fatalf("Failed to parse address: %v", err)
	}

	// Encode back
	encoded := addr1.Encode()
	fmt.Printf("Encoded:  %s\n", encoded)

	// Verify they match (case-insensitive)
	if strings.EqualFold(original, encoded) {
		fmt.Printf("✓ Round-trip successful!\n")
	} else {
		fmt.Printf("✗ Round-trip failed\n")
	}
	fmt.Println()

	// Example 5: Error handling
	fmt.Println("Example 5: Error handling examples")
	fmt.Println("-----------------------------------")
	invalidAddresses := []string{
		"short.onion",                  // Too short
		"!!!invalid!!!base32!!!.onion", // Invalid base32
		"thisisatoolongaddressthatexceedsthemaximumlengthforanyonionaddress.onion", // Too long
	}

	for _, invalidAddr := range invalidAddresses {
		_, err := onion.ParseAddress(invalidAddr)
		if err != nil {
			fmt.Printf("✓ Correctly rejected: %s\n", invalidAddr)
			fmt.Printf("  Error: %v\n", err)
		} else {
			fmt.Printf("✗ Should have rejected: %s\n", invalidAddr)
		}
	}
	fmt.Println()

	// Example 6: Using the String() method
	fmt.Println("Example 6: Address string representation")
	fmt.Println("-----------------------------------------")
	addr6, err := onion.ParseAddress(exampleAddr)
	if err != nil {
		log.Fatalf("Failed to parse address: %v", err)
	}
	fmt.Printf("String() output: %s\n", addr6.String())
	fmt.Printf("Encode() output: %s\n", addr6.Encode())
	fmt.Println()

	fmt.Println("=== Demo Complete ===")
}

// generateOnionAddress creates a new valid v3 onion address
func generateOnionAddress() string {
	// Generate a random ed25519 public key
	pubkey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(fmt.Sprintf("Failed to generate key: %v", err))
	}

	// Compute checksum: SHA3-256(".onion checksum" || pubkey || 0x03)[:2]
	// We'll use the onion package's internal logic by creating an Address struct
	addr := &onion.Address{
		Version: onion.V3,
		Pubkey:  pubkey,
	}

	return addr.Encode()
}
