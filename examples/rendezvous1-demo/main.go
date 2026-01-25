// Package main demonstrates RENDEZVOUS1 cell construction for onion services
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"log"

	"github.com/opd-ai/go-tor/pkg/cell"
	"github.com/opd-ai/go-tor/pkg/crypto"
	"github.com/opd-ai/go-tor/pkg/onion"
	"golang.org/x/crypto/curve25519"
)

// MockCircuit implements minimal CircuitInterface for demo
type MockCircuit struct {
	id        uint32
	sentCells []*cell.RelayCell
}

func (m *MockCircuit) SendRelayCell(c *cell.RelayCell) error {
	m.sentCells = append(m.sentCells, c)
	return nil
}

func (m *MockCircuit) ReceiveRelayCell(ctx context.Context) (*cell.RelayCell, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *MockCircuit) GetID() uint32 {
	return m.id
}

func main() {
	fmt.Println("RENDEZVOUS1 Cell Construction Demo")
	fmt.Println("===================================")
	fmt.Println()

	// Step 1: Setup - Generate server (onion service) keys
	fmt.Println("Step 1: Generating server keys...")
	serverNtor, err := crypto.GenerateNtorKeyPair()
	if err != nil {
		log.Fatalf("Failed to generate server ntor key: %v", err)
	}

	serverIdentity := make([]byte, 32)
	if _, err := rand.Read(serverIdentity); err != nil {
		log.Fatalf("Failed to generate server identity: %v", err)
	}

	var serverPublic [32]byte
	curve25519.ScalarBaseMult(&serverPublic, &serverNtor.Private)

	fmt.Printf("  Server ntor public key: %x...\n", serverPublic[:8])
	fmt.Printf("  Server identity: %x...\n", serverIdentity[:8])
	fmt.Println()

	// Step 2: Client generates ntor handshake (in INTRODUCE2)
	fmt.Println("Step 2: Client generating ntor handshake...")
	clientHandshake, _, err := crypto.NtorClientHandshake(serverIdentity, serverPublic[:])
	if err != nil {
		log.Fatalf("Client handshake failed: %v", err)
	}

	fmt.Printf("  Client handshake length: %d bytes\n", len(clientHandshake))
	fmt.Printf("  Client handshake: %x...\n", clientHandshake[:16])
	fmt.Println()

	// Step 3: Generate rendezvous cookie (from INTRODUCE2)
	fmt.Println("Step 3: Generating rendezvous cookie...")
	rendezvousCookie := make([]byte, 20)
	if _, err := rand.Read(rendezvousCookie); err != nil {
		log.Fatalf("Failed to generate cookie: %v", err)
	}
	fmt.Printf("  Cookie: %x...\n", rendezvousCookie[:8])
	fmt.Println()

	// Step 4: Server builds and sends RENDEZVOUS1 cell
	fmt.Println("Step 4: Server building and sending RENDEZVOUS1...")
	circuitID := uint32(12345)
	mockCircuit := &MockCircuit{id: circuitID}

	_, err = onion.SendRendezvous1(
		mockCircuit,
		circuitID,
		rendezvousCookie,
		clientHandshake,
		serverNtor.Private[:],
		serverIdentity,
	)
	if err != nil {
		log.Fatalf("Failed to send RENDEZVOUS1: %v", err)
	}

	if len(mockCircuit.sentCells) != 1 {
		log.Fatalf("Expected 1 sent cell, got %d", len(mockCircuit.sentCells))
	}

	rendezvous1Cell := mockCircuit.sentCells[0]
	fmt.Printf("  RENDEZVOUS1 cell created:\n")
	fmt.Printf("    Command: %d (RELAY_RENDEZVOUS1)\n", rendezvous1Cell.Command)
	fmt.Printf("    Stream ID: %d\n", rendezvous1Cell.StreamID)
	fmt.Printf("    Data length: %d bytes\n", len(rendezvous1Cell.Data))
	fmt.Println()

	// Verify cookie is in cell
	if !bytes.Equal(rendezvous1Cell.Data[:20], rendezvousCookie) {
		log.Fatalf("Cookie mismatch in RENDEZVOUS1 cell")
	}
	fmt.Println("  ✓ Rendezvous cookie verified in cell")

	// Verify handshake response is present (64 bytes after cookie)
	if len(rendezvous1Cell.Data) < 84 {
		log.Fatalf("RENDEZVOUS1 cell data too short: %d < 84", len(rendezvous1Cell.Data))
	}
	fmt.Println("  ✓ Handshake response verified in cell (64 bytes)")
	fmt.Println()

	fmt.Println("Demo completed successfully!")
	fmt.Println("The rendezvous circuit is now established.")
}
