// Package main demonstrates RENDEZVOUS1 cell construction for onion services
package main

import (
"bytes"
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
sentCells []*cell.RelayCell
}

func (m *MockCircuit) SendRelayCell(c *cell.RelayCell) error {
m.sentCells = append(m.sentCells, c)
return nil
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
clientHandshake, clientPrivate, err := crypto.NtorClientHandshake(serverIdentity, serverPublic[:])
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
mockCircuit := &MockCircuit{}

keyMaterial, err := onion.SendRendezvous1(
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

sentCell := mockCircuit.sentCells[0]
fmt.Printf("  RENDEZVOUS1 cell command: %d\n", sentCell.Command)
fmt.Printf("  RENDEZVOUS1 payload length: %d bytes\n", len(sentCell.Data))
fmt.Printf("  Server key material length: %d bytes\n", len(keyMaterial))
fmt.Println()

// Step 5: Client processes RENDEZVOUS1 response
fmt.Println("Step 5: Client processing RENDEZVOUS1 response...")

// Verify cookie matches
cookieInCell := sentCell.Data[0:20]
if !bytes.Equal(cookieInCell, rendezvousCookie) {
log.Fatal("Cookie mismatch!")
}
fmt.Println("  ✓ Cookie verified")

// Extract handshake response (Y || AUTH)
handshakeResponse := sentCell.Data[20:84]
fmt.Printf("  Handshake response length: %d bytes\n", len(handshakeResponse))

// Client processes response and derives key material
clientKeyMaterial, err := crypto.NtorProcessResponse(
handshakeResponse,
clientPrivate,
serverPublic[:],
serverIdentity,
)
if err != nil {
log.Fatalf("Client failed to process response: %v", err)
}
fmt.Printf("  Client key material length: %d bytes\n", len(clientKeyMaterial))
fmt.Println()

// Step 6: Verify both parties derived same key material
fmt.Println("Step 6: Verifying key material agreement...")
if !bytes.Equal(clientKeyMaterial, keyMaterial) {
log.Fatal("❌ Key material mismatch!")
}
fmt.Println("  ✓ Key material matches!")
fmt.Println()

// Display key material components
fmt.Println("Derived Key Material Components:")
fmt.Println("--------------------------------")
Df := keyMaterial[0:20]
Db := keyMaterial[20:40]
Kf := keyMaterial[40:56]
Kb := keyMaterial[56:72]

fmt.Printf("  Df (forward digest):  %x...\n", Df[:8])
fmt.Printf("  Db (backward digest): %x...\n", Db[:8])
fmt.Printf("  Kf (forward cipher):  %x...\n", Kf[:8])
fmt.Printf("  Kb (backward cipher): %x...\n", Kb[:8])
fmt.Println()

fmt.Println("✅ RENDEZVOUS1 handshake completed successfully!")
fmt.Println()
fmt.Println("Summary:")
fmt.Println("--------")
fmt.Printf("  Protocol: ntor (Curve25519 + HKDF-SHA256)\n")
fmt.Printf("  Security: Forward secrecy, mutual authentication\n")
fmt.Printf("  Client handshake: %d bytes\n", len(clientHandshake))
fmt.Printf("  Server response: %d bytes\n", len(handshakeResponse))
fmt.Printf("  Key material: %d bytes (4 keys derived)\n", len(keyMaterial))
fmt.Println()
fmt.Println("⚠️  Educational demo only - not for production use!")
}
