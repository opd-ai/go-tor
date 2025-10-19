// Package main demonstrates descriptor management functionality
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/opd-ai/go-tor/pkg/logger"
	"github.com/opd-ai/go-tor/pkg/onion"
	"github.com/opd-ai/go-tor/pkg/security"
)

func main() {
fmt.Println("=== Onion Service Descriptor Management Demo ===")
fmt.Println()

// Initialize logger
lgr := logger.NewDefault()

// Create onion client
client := onion.NewClient(lgr)
fmt.Println("✓ Created onion service client")

// Example v3 onion address (DuckDuckGo)
exampleAddr := "vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd.onion"

// Parse the address
addr, err := onion.ParseAddress(exampleAddr)
if err != nil {
log.Fatalf("Failed to parse address: %v", err)
}
fmt.Printf("✓ Parsed onion address: %s\n", addr.String())
fmt.Printf("  Version: v%d\n", addr.Version)
fmt.Printf("  Public key (hex): %x...\n", addr.Pubkey[:8])
fmt.Println()

// Demonstrate time period calculation
now := time.Now()
timePeriod := onion.GetTimePeriod(now)
fmt.Printf("Current time period: %d\n", timePeriod)
fmt.Printf("  (Changes every 24 hours)\n")
fmt.Println()

// Demonstrate descriptor fetching
ctx := context.Background()
fmt.Println("Fetching descriptor for onion service...")
desc, err := client.GetDescriptor(ctx, addr)
if err != nil {
log.Fatalf("Failed to get descriptor: %v", err)
}
fmt.Println("✓ Descriptor retrieved")
fmt.Printf("  Version: %d\n", desc.Version)
fmt.Printf("  Revision counter: %d\n", desc.RevisionCounter)
fmt.Printf("  Lifetime: %v\n", desc.Lifetime)
fmt.Printf("  Created at: %s\n", desc.CreatedAt.Format(time.RFC3339))
fmt.Printf("  Introduction points: %d\n", len(desc.IntroPoints))
fmt.Println()

// Demonstrate cache hit
fmt.Println("Fetching descriptor again (should hit cache)...")
desc2, err := client.GetDescriptor(ctx, addr)
if err != nil {
log.Fatalf("Failed to get cached descriptor: %v", err)
}
if desc2 == desc {
fmt.Println("✓ Descriptor retrieved from cache (same instance)")
}
fmt.Println()

// Demonstrate descriptor cache functionality
fmt.Println("Descriptor Cache Statistics:")
cache := onion.NewDescriptorCache(lgr)

// Add multiple descriptors
addresses := []string{
"vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd.onion", // DuckDuckGo
"thehiddenwiki646uekl52zqznz3qqq6u5emjktqovfiibxyafzqnbkad.onion",   // Example
"danielas3rtn54uwmofdo3x2bsdifr47huasnmbgqzfrec5ubupvtpid.onion",   // Example
}

for i, addrStr := range addresses {
a, err := onion.ParseAddress(addrStr)
if err != nil {
continue
}

// Safely convert index to uint64
revCounter, err := security.SafeIntToUint64(i + 1)
if err != nil {
revCounter = 1 // fallback to 1
}

d := &onion.Descriptor{
Version:         3,
Address:         a,
RevisionCounter: revCounter,
CreatedAt:       time.Now(),
Lifetime:        3 * time.Hour,
}
cache.Put(a, d)
}

fmt.Printf("  Cache size: %d descriptors\n", cache.Size())
fmt.Println()

// Demonstrate descriptor encoding
fmt.Println("Encoding descriptor to wire format...")
encoded, err := onion.EncodeDescriptor(desc)
if err != nil {
log.Fatalf("Failed to encode descriptor: %v", err)
}
fmt.Printf("✓ Descriptor encoded (%d bytes)\n", len(encoded))
fmt.Println("  Preview:")
preview := string(encoded)
if len(preview) > 200 {
preview = preview[:200] + "..."
}
fmt.Printf("  %s\n", preview)
fmt.Println()

// Demonstrate descriptor parsing
fmt.Println("Parsing descriptor from wire format...")
parsed, err := onion.ParseDescriptor(encoded)
if err != nil {
log.Fatalf("Failed to parse descriptor: %v", err)
}
fmt.Printf("✓ Descriptor parsed\n")
fmt.Printf("  Version: %d\n", parsed.Version)
fmt.Println()

fmt.Println("=== Demo Complete ===")
fmt.Println()
fmt.Println("Note: This demonstrates the descriptor management foundation.")
fmt.Println("Full HSDir fetching protocol to be implemented in future phases.")
}
