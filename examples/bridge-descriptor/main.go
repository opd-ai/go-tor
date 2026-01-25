// Package main demonstrates server descriptor generation for bridge relays.
// This example shows how to create and validate Tor relay server descriptors.
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/opd-ai/go-tor/pkg/relay"
)

func main() {
	fmt.Println("=== Bridge Relay Server Descriptor Example ===")
	fmt.Println()

	// Step 1: Generate relay cryptographic keys
	fmt.Println("Step 1: Generating relay keys (Ed25519 + RSA-1024 + ntor)...")
	keys, err := relay.GenerateRelayKeys()
	if err != nil {
		log.Fatalf("Failed to generate keys: %v", err)
	}
	fmt.Printf("✓ Keys generated successfully\n")
	fmt.Printf("  - RSA Fingerprint: %s\n", keys.Fingerprint()[:16]+"...")
	fmt.Printf("  - Ed25519 Fingerprint: %s\n", keys.Ed25519Fingerprint()[:16]+"...")
	fmt.Println()

	// Step 2: Create bridge descriptor configuration
	fmt.Println("Step 2: Configuring bridge descriptor...")
	config := &relay.DescriptorConfig{
		Nickname:       "MyBridge",
		Address:        "192.0.2.100", // Example IP (RFC 5737)
		ORPort:         443,           // Common bridge port (HTTPS)
		DirPort:        0,             // Bridges don't advertise directory port
		Contact:        "bridge@example.com",
		BandwidthAvg:   5 * 1024 * 1024,  // 5 MB/s average
		BandwidthBurst: 10 * 1024 * 1024, // 10 MB/s burst
		IsBridge:       true,
	}
	fmt.Printf("✓ Bridge configuration created\n")
	fmt.Printf("  - Nickname: %s\n", config.Nickname)
	fmt.Printf("  - Address: %s:%d\n", config.Address, config.ORPort)
	fmt.Printf("  - Bandwidth: %d KB/s avg, %d KB/s burst\n",
		config.BandwidthAvg/1024, config.BandwidthBurst/1024)
	fmt.Println()

	// Step 3: Generate signed server descriptor
	fmt.Println("Step 3: Generating signed server descriptor...")
	descriptor, err := relay.GenerateServerDescriptor(keys, config)
	if err != nil {
		log.Fatalf("Failed to generate descriptor: %v", err)
	}
	fmt.Printf("✓ Server descriptor generated\n")
	fmt.Printf("  - Published: %s\n", descriptor.PublishedTime.Format("2006-01-02 15:04:05"))
	fmt.Printf("  - Platform: %s\n", descriptor.Platform)
	fmt.Printf("  - Exit Policy: %s\n", descriptor.ExitPolicy)
	fmt.Println()

	// Step 4: Validate descriptor
	fmt.Println("Step 4: Validating descriptor...")
	if err := descriptor.Validate(); err != nil {
		log.Fatalf("Descriptor validation failed: %v", err)
	}
	fmt.Printf("✓ Descriptor is valid\n")
	fmt.Printf("  - Descriptor size: %d bytes\n", len(descriptor.RawDescriptor))
	fmt.Printf("  - Signature size: %d bytes\n", len(descriptor.Signature))
	fmt.Println()

	// Step 5: Display descriptor content
	fmt.Println("Step 5: Server descriptor content:")
	fmt.Println("---")
	fmt.Printf("%s", descriptor.RawDescriptor)
	fmt.Println("---")
	fmt.Println()

	// Step 6: Generate extra-info descriptor with statistics
	fmt.Println("Step 6: Generating extra-info descriptor...")
	stats := map[string]string{
		"read-history":  "2024-01-25 00:00:00 (900 s) 1024000,2048000,1536000",
		"write-history": "2024-01-25 00:00:00 (900 s) 512000,1024000,768000",
		"uptime":        "86400",
	}
	extraInfo, err := relay.GenerateExtraInfo(keys, descriptor, stats)
	if err != nil {
		log.Fatalf("Failed to generate extra-info: %v", err)
	}
	fmt.Printf("✓ Extra-info descriptor generated\n")
	fmt.Printf("  - Nickname: %s\n", extraInfo.Nickname)
	fmt.Printf("  - Fingerprint: %s\n", extraInfo.Fingerprint[:16]+"...")
	fmt.Printf("  - Statistics: %d entries\n", len(extraInfo.Statistics))
	fmt.Println()

	// Step 7: Save keys to disk (optional)
	fmt.Println("Step 7: Saving keys to disk...")
	dataDir := "/tmp/go-tor-bridge-keys"
	if err := keys.SaveKeys(dataDir); err != nil {
		log.Printf("Warning: Failed to save keys: %v", err)
	} else {
		fmt.Printf("✓ Keys saved to %s\n", dataDir)
		fmt.Printf("  - Ed25519 identity key\n")
		fmt.Printf("  - RSA identity key\n")
		fmt.Printf("  - TLS certificate\n")

		// Clean up example keys
		defer func() {
			if err := os.RemoveAll(dataDir); err != nil {
				log.Printf("Warning: Failed to cleanup keys: %v", err)
			}
		}()
	}
	fmt.Println()

	// Summary
	fmt.Println("=== Summary ===")
	fmt.Printf("Successfully generated bridge relay descriptor:\n")
	fmt.Printf("  - Nickname: %s\n", descriptor.Nickname)
	fmt.Printf("  - Address: %s:%d\n", descriptor.Address, descriptor.ORPort)
	fmt.Printf("  - Fingerprint: %s\n", descriptor.Fingerprint())
	fmt.Printf("  - Descriptor size: %d bytes\n", len(descriptor.RawDescriptor))
	fmt.Printf("  - Ready for publication to bridge authority\n")
	fmt.Println()
	fmt.Println("⚠️  WARNING: This is for educational/research purposes only")
	fmt.Println("    Do NOT use for production anonymity needs")
	fmt.Println("    Use official Tor software: https://www.torproject.org")
}
