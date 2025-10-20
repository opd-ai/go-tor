// Package main demonstrates the Introduction Point Protocol for onion services.
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"time"

	"github.com/opd-ai/go-tor/pkg/logger"
	"github.com/opd-ai/go-tor/pkg/onion"
	"github.com/opd-ai/go-tor/pkg/security"
)

func main() {
	fmt.Println("=== Introduction Point Protocol Demo ===")
	fmt.Println()

	// Create logger
	log := logger.NewDefault()

	// Create onion service client
	client := onion.NewClient(log)
	fmt.Println("✓ Created onion service client")

	// Parse a v3 onion address (or generate one for demo)
	addr, err := onion.ParseAddress("thgtoa7imksbg7rit4grgijl2ef6kc7b56bp56pmtta4g354lydlzkqd.onion")
	if err != nil {
		fmt.Printf("✗ Failed to parse address: %v\n", err)
		fmt.Println("\nGenerating test v3 onion address...")
		addr = generateTestAddress()
	}

	fmt.Printf("✓ Onion address: %s\n", addr.String())
	fmt.Println()

	// === Part 1: Descriptor with Introduction Points ===
	fmt.Println("--- Creating Descriptor with Introduction Points ---")

	// Create a mock descriptor with introduction points
	desc := createMockDescriptor(addr)
	fmt.Printf("✓ Descriptor created with %d introduction points\n", len(desc.IntroPoints))

	// Cache the descriptor
	client.CacheDescriptor(addr, desc)
	fmt.Println("✓ Descriptor cached")
	fmt.Println()

	// === Part 2: Introduction Point Selection ===
	fmt.Println("--- Selecting Introduction Point ---")

	intro := onion.NewIntroductionProtocol(log)
	introPoint, err := intro.SelectIntroductionPoint(desc)
	if err != nil {
		fmt.Printf("✗ Failed to select introduction point: %v\n", err)
		return
	}

	fmt.Println("✓ Introduction point selected:")
	fmt.Printf("  - OnionKey: %x...\n", introPoint.OnionKey[:8])
	fmt.Printf("  - AuthKey: %x...\n", introPoint.AuthKey[:8])
	fmt.Printf("  - Link Specifiers: %d\n", len(introPoint.LinkSpecifiers))
	fmt.Println()

	// === Part 3: Building INTRODUCE1 Cell ===
	fmt.Println("--- Building INTRODUCE1 Cell ---")

	// Create rendezvous cookie
	rendezvousCookie := make([]byte, 20)
	rand.Read(rendezvousCookie)

	// Create onion key
	onionKey := make([]byte, 32)
	rand.Read(onionKey)

	// Build introduce request
	req := &onion.IntroduceRequest{
		IntroPoint:       introPoint,
		RendezvousCookie: rendezvousCookie,
		RendezvousPoint:  "mock-rendezvous-point-fingerprint",
		OnionKey:         onionKey,
	}

	introduce1Data, err := intro.BuildIntroduce1Cell(req)
	if err != nil {
		fmt.Printf("✗ Failed to build INTRODUCE1 cell: %v\n", err)
		return
	}

	fmt.Printf("✓ INTRODUCE1 cell built (%d bytes)\n", len(introduce1Data))
	fmt.Println("  Cell structure:")
	fmt.Println("    - LEGACY_KEY_ID: 20 bytes (zeros for v3)")
	fmt.Println("    - AUTH_KEY_TYPE: 1 byte (0x02 for ed25519)")
	fmt.Println("    - AUTH_KEY_LEN: 2 bytes")
	fmt.Println("    - AUTH_KEY: 32 bytes")
	fmt.Println("    - EXTENSIONS: N bytes")
	fmt.Println("    - ENCRYPTED_DATA: remaining bytes")
	fmt.Println()

	// === Part 4: Creating Introduction Circuit ===
	fmt.Println("--- Creating Introduction Circuit ---")

	ctx := context.Background()
	// Pass nil for circuit builder to use mock implementation
	circuitID, err := intro.CreateIntroductionCircuit(ctx, introPoint, nil)
	if err != nil {
		fmt.Printf("✗ Failed to create introduction circuit: %v\n", err)
		return
	}

	fmt.Printf("✓ Introduction circuit created (ID: %d)\n", circuitID)
	fmt.Println("  In a full implementation, this circuit would:")
	fmt.Println("    1. Use the circuit builder to create a 3-hop circuit")
	fmt.Println("    2. Extend to the introduction point")
	fmt.Println("    3. Wait for circuit establishment")
	fmt.Println()

	// === Part 5: Sending INTRODUCE1 Cell ===
	fmt.Println("--- Sending INTRODUCE1 Cell ---")

	// Pass nil for cell sender to use mock implementation
	err = intro.SendIntroduce1(ctx, circuitID, introduce1Data, nil)
	if err != nil {
		fmt.Printf("✗ Failed to send INTRODUCE1: %v\n", err)
		return
	}

	fmt.Println("✓ INTRODUCE1 cell sent")
	fmt.Println("  In a full implementation, this would:")
	fmt.Println("    1. Wrap data in RELAY cell with INTRODUCE1 command")
	fmt.Println("    2. Send over the circuit")
	fmt.Println("    3. Wait for acknowledgment")
	fmt.Println()

	// === Part 6: Full Connection Orchestration ===
	fmt.Println("--- Full Connection Orchestration ---")

	// Create a fresh address for full demo
	freshAddr := generateTestAddress()
	freshDesc := createMockDescriptor(freshAddr)
	client.CacheDescriptor(freshAddr, freshDesc)

	fullCircuitID, err := client.ConnectToOnionService(ctx, freshAddr)
	if err != nil {
		fmt.Printf("✗ Failed to connect to onion service: %v\n", err)
		return
	}

	fmt.Printf("✓ Successfully orchestrated connection to onion service\n")
	fmt.Printf("  Circuit ID: %d\n", fullCircuitID)
	fmt.Printf("  Address: %s\n", freshAddr.String())
	fmt.Println()

	// === Summary ===
	fmt.Println("=== Demo Complete ===")
	fmt.Println()
	fmt.Println("Phase 7.3.3 Implementation Status:")
	fmt.Println("  ✓ Introduction point selection")
	fmt.Println("  ✓ INTRODUCE1 cell construction")
	fmt.Println("  ✓ Introduction circuit creation (mock)")
	fmt.Println("  ✓ Cell sending protocol (mock)")
	fmt.Println("  ✓ Full connection orchestration")
	fmt.Println()
	fmt.Println("Next Steps (Phase 7.3.4):")
	fmt.Println("  - Rendezvous point selection")
	fmt.Println("  - RENDEZVOUS1/2 protocol")
	fmt.Println("  - End-to-end circuit completion")
	fmt.Println()
	fmt.Println("Production Hardening (Phase 8):")
	fmt.Println("  - Real circuit-based communication")
	fmt.Println("  - Encryption of INTRODUCE1 data")
	fmt.Println("  - INTRODUCE_ACK handling")
	fmt.Println("  - Retry and timeout logic")
}

// generateTestAddress generates a test v3 onion address
func generateTestAddress() *onion.Address {
	pubkey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(fmt.Sprintf("Failed to generate key: %v", err))
	}
	addr := &onion.Address{
		Version: onion.V3,
		Pubkey:  pubkey,
	}
	addr.Raw = addr.Encode()
	return addr
}

// createMockDescriptor creates a mock descriptor with introduction points
func createMockDescriptor(addr *onion.Address) *onion.Descriptor {
	// Create mock introduction points
	introPoints := make([]onion.IntroductionPoint, 3)
	for i := 0; i < 3; i++ {
		onionKey := make([]byte, 32)
		authKey := make([]byte, 32)
		rand.Read(onionKey)
		rand.Read(authKey)

		introPoints[i] = onion.IntroductionPoint{
			OnionKey: onionKey,
			AuthKey:  authKey,
			LinkSpecifiers: []onion.LinkSpecifier{
				{
					Type: 0, // IPv4
					Data: []byte{192, 168, 1, byte(100 + i)},
				},
				{
					Type: 2, // Legacy ID
					Data: make([]byte, 20),
				},
			},
		}
	}

	// Compute time period and blinded key
	timePeriod := onion.GetTimePeriod(time.Now())
	blindedPubkey := onion.ComputeBlindedPubkey(ed25519.PublicKey(addr.Pubkey), timePeriod)

	// Safely convert timestamp to uint64
	now := time.Now()
	revisionCounter, err := security.SafeUnixToUint64(now)
	if err != nil {
		revisionCounter = 0
	}

	return &onion.Descriptor{
		Version:         3,
		Address:         addr,
		IntroPoints:     introPoints,
		BlindedPubkey:   blindedPubkey,
		DescriptorID:    computeDescriptorID(blindedPubkey),
		RevisionCounter: revisionCounter,
		CreatedAt:       now,
		Lifetime:        3 * time.Hour,
	}
}

// computeDescriptorID computes a descriptor ID (simplified)
func computeDescriptorID(blindedPubkey []byte) []byte {
	// In reality, this would be SHA3-256(blinded_pubkey)
	// For demo, just return first 32 bytes
	id := make([]byte, 32)
	copy(id, blindedPubkey)
	return id
}
