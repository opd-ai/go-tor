package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"log"
	"time"

	"github.com/opd-ai/go-tor/pkg/logger"
	"github.com/opd-ai/go-tor/pkg/onion"
)

func main() {
	fmt.Println("=== Rendezvous Protocol Demo ===")
	fmt.Println()

	// Create logger
	customLogger := logger.NewDefault()

	// Create onion service client
	client := onion.NewClient(customLogger)
	fmt.Println("✓ Created onion service client")

	// Generate a test onion address
	pubkey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		log.Fatalf("Failed to generate key: %v", err)
	}

	addr := &onion.Address{
		Version: onion.V3,
		Pubkey:  pubkey,
	}
	addr.Raw = addr.Encode()
	fmt.Printf("✓ Onion address: %s\n", addr.String())
	fmt.Println()

	// Create mock relays for demonstration
	mockRelays := []*onion.HSDirectory{
		{
			Fingerprint: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
			Address:     "1.1.1.1",
			ORPort:      9001,
			HSDir:       true,
		},
		{
			Fingerprint: "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB",
			Address:     "2.2.2.2",
			ORPort:      9002,
			HSDir:       true,
		},
		{
			Fingerprint: "CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC",
			Address:     "3.3.3.3",
			ORPort:      9003,
			HSDir:       true,
		},
	}

	// Update client with available relays
	client.UpdateHSDirs(mockRelays)
	fmt.Printf("✓ Updated client with %d available relays\n", len(mockRelays))
	fmt.Println()

	// Demo 1: Rendezvous Point Selection
	fmt.Println("--- Selecting Rendezvous Point ---")
	rendezvous := onion.NewRendezvousProtocol(customLogger)

	rendezvousPoint, err := rendezvous.SelectRendezvousPoint(mockRelays)
	if err != nil {
		log.Fatalf("Failed to select rendezvous point: %v", err)
	}

	fmt.Printf("✓ Rendezvous point selected:\n")
	fmt.Printf("  - Fingerprint: %s\n", rendezvousPoint.Fingerprint)
	fmt.Printf("  - Address: %s:%d\n", rendezvousPoint.Address, rendezvousPoint.ORPort)
	fmt.Println()

	// Demo 2: ESTABLISH_RENDEZVOUS Cell Construction
	fmt.Println("--- Building ESTABLISH_RENDEZVOUS Cell ---")

	rendezvousCookie := make([]byte, 20)
	for i := range rendezvousCookie {
		rendezvousCookie[i] = byte(i)
	}

	establishReq := &onion.EstablishRendezvousRequest{
		RendezvousCookie: rendezvousCookie,
	}

	establishData, err := rendezvous.BuildEstablishRendezvousCell(establishReq)
	if err != nil {
		log.Fatalf("Failed to build ESTABLISH_RENDEZVOUS cell: %v", err)
	}

	fmt.Printf("✓ ESTABLISH_RENDEZVOUS cell built (%d bytes)\n", len(establishData))
	fmt.Printf("  Cell structure:\n")
	fmt.Printf("    - RENDEZVOUS_COOKIE: 20 bytes\n")
	fmt.Println()

	// Demo 3: Creating Rendezvous Circuit
	fmt.Println("--- Creating Rendezvous Circuit ---")

	ctx := context.Background()
	circuitID, err := rendezvous.CreateRendezvousCircuit(ctx, rendezvousPoint)
	if err != nil {
		log.Fatalf("Failed to create rendezvous circuit: %v", err)
	}

	fmt.Printf("✓ Rendezvous circuit created\n")
	fmt.Printf("  - Circuit ID: %d\n", circuitID)
	fmt.Println()

	// Demo 4: Sending ESTABLISH_RENDEZVOUS
	fmt.Println("--- Sending ESTABLISH_RENDEZVOUS ---")

	err = rendezvous.SendEstablishRendezvous(ctx, circuitID, establishData)
	if err != nil {
		log.Fatalf("Failed to send ESTABLISH_RENDEZVOUS: %v", err)
	}

	fmt.Printf("✓ ESTABLISH_RENDEZVOUS sent successfully\n")
	fmt.Println()

	// Demo 5: RENDEZVOUS1 Cell Construction (for completeness)
	fmt.Println("--- Building RENDEZVOUS1 Cell (Service Side) ---")

	handshakeData := make([]byte, 32)
	for i := range handshakeData {
		handshakeData[i] = byte(i + 100)
	}

	rendezvous1Req := &onion.Rendezvous1Request{
		RendezvousCookie: rendezvousCookie,
		HandshakeData:    handshakeData,
	}

	rendezvous1Data, err := rendezvous.BuildRendezvous1Cell(rendezvous1Req)
	if err != nil {
		log.Fatalf("Failed to build RENDEZVOUS1 cell: %v", err)
	}

	fmt.Printf("✓ RENDEZVOUS1 cell built (%d bytes)\n", len(rendezvous1Data))
	fmt.Printf("  Cell structure:\n")
	fmt.Printf("    - RENDEZVOUS_COOKIE: 20 bytes\n")
	fmt.Printf("    - HANDSHAKE_DATA: %d bytes\n", len(handshakeData))
	fmt.Println()

	// Demo 6: RENDEZVOUS2 Cell Parsing
	fmt.Println("--- Parsing RENDEZVOUS2 Cell (Client Side) ---")

	// Simulate receiving RENDEZVOUS2 with handshake data
	rendezvous2Data := make([]byte, 32)
	for i := range rendezvous2Data {
		rendezvous2Data[i] = byte(i + 200)
	}

	parsedHandshake, err := rendezvous.ParseRendezvous2Cell(rendezvous2Data)
	if err != nil {
		log.Fatalf("Failed to parse RENDEZVOUS2 cell: %v", err)
	}

	fmt.Printf("✓ RENDEZVOUS2 cell parsed successfully\n")
	fmt.Printf("  - Handshake data length: %d bytes\n", len(parsedHandshake))
	fmt.Println()

	// Demo 7: Full Rendezvous Point Establishment
	fmt.Println("--- Full Rendezvous Point Establishment ---")

	newCookie := make([]byte, 20)
	for i := range newCookie {
		newCookie[i] = byte(i * 2)
	}

	rvCircuitID, rvPoint, err := client.EstablishRendezvousPoint(ctx, newCookie, mockRelays)
	if err != nil {
		log.Fatalf("Failed to establish rendezvous point: %v", err)
	}

	fmt.Printf("✓ Rendezvous point fully established\n")
	fmt.Printf("  - Circuit ID: %d\n", rvCircuitID)
	fmt.Printf("  - Rendezvous point: %s\n", rvPoint.Fingerprint)
	fmt.Println()

	// Demo 8: Complete Rendezvous Protocol
	fmt.Println("--- Completing Rendezvous Protocol ---")

	err = client.CompleteRendezvous(ctx, rvCircuitID)
	if err != nil {
		log.Fatalf("Failed to complete rendezvous: %v", err)
	}

	fmt.Printf("✓ Rendezvous protocol completed successfully\n")
	fmt.Println()

	// Demo 9: Full Onion Service Connection
	fmt.Println("--- Full Onion Service Connection Orchestration ---")

	// Create a mock descriptor with introduction points
	desc := &onion.Descriptor{
		Version:     3,
		Address:     addr,
		IntroPoints: []onion.IntroductionPoint{
			{
				OnionKey: make([]byte, 32),
				AuthKey:  make([]byte, 32),
				LinkSpecifiers: []onion.LinkSpecifier{
					{Type: 0, Data: []byte{127, 0, 0, 1}},
					{Type: 2, Data: []byte{0x23, 0x28}},
				},
			},
		},
		CreatedAt: time.Now(),
		Lifetime:  3 * time.Hour,
	}

	// Cache the descriptor
	client.CacheDescriptor(addr, desc)
	fmt.Printf("✓ Descriptor cached for %s\n", addr.String())

	// Connect to the onion service
	finalCircuitID, err := client.ConnectToOnionService(ctx, addr)
	if err != nil {
		log.Fatalf("Failed to connect to onion service: %v", err)
	}

	fmt.Printf("✓ Successfully orchestrated full connection to onion service\n")
	fmt.Printf("  - Circuit ID: %d\n", finalCircuitID)
	fmt.Printf("  - Address: %s\n", addr.String())
	fmt.Println()

	fmt.Println("=== Demo Complete ===")
	fmt.Println()
	fmt.Println("Summary:")
	fmt.Println("- Rendezvous point selection: ✓")
	fmt.Println("- ESTABLISH_RENDEZVOUS cell construction: ✓")
	fmt.Println("- Rendezvous circuit creation: ✓")
	fmt.Println("- RENDEZVOUS1 cell construction: ✓")
	fmt.Println("- RENDEZVOUS2 cell parsing: ✓")
	fmt.Println("- Full onion service connection: ✓")
	fmt.Println()
	fmt.Println("Phase 7.3.4 (Rendezvous Protocol) implementation complete!")
}
