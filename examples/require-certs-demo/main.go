package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/opd-ai/go-tor/pkg/connection"
	"github.com/opd-ai/go-tor/pkg/logger"
	"github.com/opd-ai/go-tor/pkg/protocol"
)

func main() {
	fmt.Println("=== CERTS Cell Strict Enforcement Demo ===")

	// Example 1: Non-enforcing mode (default)
	fmt.Println("1. Non-enforcing mode (default):")
	fmt.Println("   - CERTS validation failures are logged as warnings")
	fmt.Println("   - Connection continues even with validation errors")
	demoNonEnforcing()

	fmt.Println()

	// Example 2: Strict enforcing mode
	fmt.Println("2. Strict enforcing mode:")
	fmt.Println("   - CERTS validation failures terminate the handshake")
	fmt.Println("   - Requires valid certificates, signatures, and identity")
	demoStrictEnforcing()
}

func demoNonEnforcing() {
	cfg := connection.DefaultConfig("127.0.0.1:9001")
	// RequireCERTS is false by default
	fmt.Printf("   RequireCERTS: %v (default)\n", cfg.RequireCERTS)

	// Optional: Set expected identity for validation
	// Even with expected identity, validation failures only log warnings
	expectedIdentity, _ := hex.DecodeString("1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	cfg.ExpectedIdentity = expectedIdentity

	fmt.Println("   Status: Validation failures will log warnings but connection continues")
}

func demoStrictEnforcing() {
	// Example relay identity from directory consensus
	// In production, fetch this from the consensus for the target relay
	expectedIdentity, _ := hex.DecodeString("a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2")
	expectedFingerprint := "1234567890ABCDEF1234567890ABCDEF12345678"

	cfg := &connection.Config{
		Address:             "127.0.0.1:9001",
		Timeout:             30 * time.Second,
		LinkProtocolV4:      true,
		ExpectedIdentity:    expectedIdentity,
		ExpectedFingerprint: expectedFingerprint,
		RequireCERTS:        true, // Enable strict enforcement
	}

	fmt.Printf("   RequireCERTS: %v\n", cfg.RequireCERTS)
	fmt.Printf("   ExpectedIdentity: %x (first 16 bytes)\n", expectedIdentity[:16])
	fmt.Printf("   ExpectedFingerprint: %s\n", expectedFingerprint)

	// Create connection with strict enforcement
	log := logger.NewDefault()
	conn := connection.New(cfg, log)

	fmt.Println("   Status: CERTS validation failures will terminate handshake")

	// Demonstrate handshake attempt (would fail if relay identity doesn't match)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// In real usage, you would connect and perform handshake
	handshake := protocol.NewHandshake(conn, log)
	_ = handshake // Would call PerformHandshake(ctx) here

	fmt.Println("   Note: In production, handshake would validate CERTS cell")
	fmt.Println("         - Expired certificates → error")
	fmt.Println("         - Invalid signatures → error")
	fmt.Println("         - Identity mismatch → error")

	_ = ctx // suppress unused warning
}

// Example production configuration
func productionConfig(relayAddress string, identityFromConsensus []byte, fingerprintFromConsensus string) *connection.Config {
	return &connection.Config{
		Address:             relayAddress,
		Timeout:             30 * time.Second,
		LinkProtocolV4:      true,
		ExpectedIdentity:    identityFromConsensus,    // From directory consensus
		ExpectedFingerprint: fingerprintFromConsensus, // From directory consensus
		RequireCERTS:        true,                     // Enforce CERTS validation
	}
}
