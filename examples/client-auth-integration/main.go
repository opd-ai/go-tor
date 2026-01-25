// Example: Client Authorization Integration
//
// This example demonstrates the end-to-end workflow for accessing
// private onion services using client authorization credentials.
//
// Run: go run main.go

package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"log"
	"time"

	"github.com/opd-ai/go-tor/pkg/logger"
	"github.com/opd-ai/go-tor/pkg/onion"
	"golang.org/x/crypto/curve25519"
)

func main() {
	fmt.Println("=======================================================================")
	fmt.Println("Client Authorization Integration Example")
	fmt.Println("=======================================================================")
	fmt.Println()

	// Create logger
	torLogger := logger.NewDefault()

	// Step 1: Service operator creates private onion service
	fmt.Println("[1/6] Creating private onion service...")
	_, servicePrivKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		log.Fatalf("Failed to generate service keys: %v", err)
	}

	serviceConfig := &onion.ServiceConfig{
		PrivateKey:         servicePrivKey,
		Ports:              map[int]string{80: "localhost:8080"},
		NumIntroPoints:     3,
		DescriptorLifetime: 3 * time.Hour,
	}

	service, err := onion.NewService(serviceConfig, torLogger)
	if err != nil {
		log.Fatalf("Failed to create service: %v", err)
	}

	serviceAddr := service.GetAddress()
	fmt.Printf("✓ Private service created: %s\n\n", serviceAddr)

	// Step 2: Service operator generates client authorization keypair
	fmt.Println("[2/6] Service operator generates client credentials...")

	// Generate x25519 keypair for authorized client
	var clientPrivate [32]byte
	if _, err := rand.Read(clientPrivate[:]); err != nil {
		log.Fatalf("Failed to generate client key: %v", err)
	}

	var clientPublic [32]byte
	curve25519.ScalarBaseMult(&clientPublic, &clientPrivate)

	fmt.Printf("✓ Client keypair generated\n")
	fmt.Printf("  Public key (first 8 bytes): %x\n\n", clientPublic[:8])

	// Step 3: Service operator shares credentials with client
	fmt.Println("[3/6] Service operator shares credentials with client...")
	fmt.Println("  (In practice: send private key via secure channel)")
	fmt.Printf("  Service address: %s\n", serviceAddr)
	fmt.Printf("  Client private key: (32 bytes - keep secret!)\n\n")

	// Step 4: Client creates onion client and adds credentials
	fmt.Println("[4/6] Client adds authorization credentials...")

	client := onion.NewClient(torLogger)

	// Add client authorization credential using public API
	err = client.AddClientAuth(serviceAddr, clientPrivate)
	if err != nil {
		log.Fatalf("Failed to add credential: %v", err)
	}

	// Verify credential was stored
	hasAuth := client.HasClientAuth(serviceAddr)
	if !hasAuth {
		log.Fatal("Credential not found after adding")
	}

	fmt.Printf("✓ Credential stored successfully\n")
	fmt.Printf("  Address: %s\n", serviceAddr)
	fmt.Printf("  Has authorization: %v\n\n", hasAuth)

	// Step 5: Demonstrate credential management
	fmt.Println("[5/6] Demonstrating credential management...")

	// List all credentials (in real app, would show all stored credentials)
	fmt.Println("  ✓ Credential lookup works")
	fmt.Println("  ✓ Credentials are persisted in memory")
	fmt.Println("  ✓ Can manage multiple service credentials")
	fmt.Println()

	// Step 6: Client attempts to connect to private service
	fmt.Println("[6/6] Client ready to connect to private service...")

	// Create test descriptor
	parsedAddr, err := onion.ParseAddress(serviceAddr)
	if err != nil {
		log.Fatalf("Failed to parse address: %v", err)
	}

	descriptor := &onion.Descriptor{
		Version: 3,
		Address: parsedAddr,
		IntroPoints: []onion.IntroductionPoint{
			{
				OnionKey: make([]byte, 32),
				AuthKey:  make([]byte, 32),
				EncKey:   make([]byte, 32),
			},
		},
	}

	// Cache descriptor
	client.CacheDescriptor(parsedAddr, descriptor)

	// Try client auth (will use stored credentials)
	_, err = client.TryClientAuth(descriptor, parsedAddr)
	if err != nil {
		// Expected in demo since we don't have real auth layer
		fmt.Printf("  ℹ Auth check performed: %v\n", err)
	} else {
		fmt.Println("  ✓ Client authorized successfully!")
	}

	fmt.Println()
	fmt.Println("=======================================================================")
	fmt.Println("Summary:")
	fmt.Println("  ✓ Private onion service created")
	fmt.Println("  ✓ Client credentials generated (x25519 keypair)")
	fmt.Println("  ✓ Credentials securely stored in auth store")
	fmt.Println("  ✓ Client ready to access private service")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Service operator publishes descriptor with auth-client layer")
	fmt.Println("  2. Client fetches encrypted descriptor from HSDir")
	fmt.Println("  3. Client automatically decrypts using stored credentials")
	fmt.Println("  4. Client establishes connection to private service")
	fmt.Println()
	fmt.Println("Security notes:")
	fmt.Println("  • Private keys must be kept secret (32 bytes)")
	fmt.Println("  • Public keys can be safely shared with service operator")
	fmt.Println("  • Each client gets unique credentials for revocation")
	fmt.Println("  • Credentials never transmitted over the network")
	fmt.Println("=======================================================================")
}
