// Circuit Isolation Example
//
// This example demonstrates how to use circuit isolation in the go-tor client
// to prevent different applications or users from sharing Tor circuits.
//
// Circuit isolation helps protect against correlation attacks by ensuring that
// different activities use separate circuits through the Tor network.

package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/opd-ai/go-tor/pkg/circuit"
	"github.com/opd-ai/go-tor/pkg/logger"
	"github.com/opd-ai/go-tor/pkg/pool"
)

func main() {
	fmt.Println("=== Circuit Isolation Example ===")

	// Create a logger
	torLogger := logger.NewDefault()

	// Create a mock circuit builder for demonstration
	circuitCounter := uint32(1)
	buildCircuit := func(ctx context.Context) (*circuit.Circuit, error) {
		circ := circuit.NewCircuit(circuitCounter)
		circuitCounter++
		circ.SetState(circuit.StateOpen)
		
		// In a real implementation, this would:
		// 1. Select path (guard, middle, exit)
		// 2. Establish TLS connection to guard
		// 3. Extend circuit through middle to exit
		
		log.Printf("Built circuit %d\n", circ.ID)
		return circ, nil
	}

	// Configure circuit pool
	poolConfig := &pool.CircuitPoolConfig{
		MinCircuits:     2,
		MaxCircuits:     10,
		PrebuildEnabled: false, // Disable for demo clarity
		RebuildInterval: 30 * time.Second,
	}

	// Create circuit pool
	circuitPool := pool.NewCircuitPool(poolConfig, buildCircuit, torLogger)
	defer circuitPool.Close()

	ctx := context.Background()

	// Example 1: No Isolation (Default Behavior)
	fmt.Println("Example 1: No Isolation (Backward Compatible)")
	fmt.Println("-----------------------------------------------")
	demonstrateNoIsolation(ctx, circuitPool)
	fmt.Println()

	// Example 2: Destination-Based Isolation
	fmt.Println("Example 2: Destination-Based Isolation")
	fmt.Println("---------------------------------------")
	demonstrateDestinationIsolation(ctx, circuitPool)
	fmt.Println()

	// Example 3: Credential-Based Isolation
	fmt.Println("Example 3: Credential-Based Isolation (SOCKS5 Username)")
	fmt.Println("--------------------------------------------------------")
	demonstrateCredentialIsolation(ctx, circuitPool)
	fmt.Println()

	// Example 4: Port-Based Isolation
	fmt.Println("Example 4: Port-Based Isolation")
	fmt.Println("--------------------------------")
	demonstratePortIsolation(ctx, circuitPool)
	fmt.Println()

	// Example 5: Session-Based Isolation
	fmt.Println("Example 5: Session-Based Isolation (Custom Tokens)")
	fmt.Println("--------------------------------------------------")
	demonstrateSessionIsolation(ctx, circuitPool)
	fmt.Println()

	// Show pool statistics
	fmt.Println("Final Pool Statistics:")
	fmt.Println("---------------------")
	stats := circuitPool.Stats()
	fmt.Printf("Total circuits: %d\n", stats.Total)
	fmt.Printf("Open circuits: %d\n", stats.Open)
	fmt.Printf("Isolated pools: %d\n", stats.IsolatedPools)
	fmt.Printf("Isolated circuits: %d\n", stats.IsolatedCircuits)
}

func demonstrateNoIsolation(ctx context.Context, pool *pool.CircuitPool) {
	// Get first circuit without isolation key
	circ1, err := pool.Get(ctx)
	if err != nil {
		log.Fatalf("Failed to get circuit: %v", err)
	}
	fmt.Printf("Got circuit %d for first request (no isolation)\n", circ1.ID)

	// Return to pool
	pool.Put(circ1)

	// Get second circuit - should reuse the same one
	circ2, err := pool.Get(ctx)
	if err != nil {
		log.Fatalf("Failed to get circuit: %v", err)
	}
	fmt.Printf("Got circuit %d for second request (no isolation)\n", circ2.ID)

	if circ1.ID == circ2.ID {
		fmt.Println("✓ Circuits are shared (same circuit ID)")
	} else {
		fmt.Println("✗ Unexpected: circuits should be shared")
	}
}

func demonstrateDestinationIsolation(ctx context.Context, pool *pool.CircuitPool) {
	// Create isolation keys for different destinations
	keyGoogle := circuit.NewIsolationKey(circuit.IsolationDestination).
		WithDestination("www.google.com:443")
	
	keyWikipedia := circuit.NewIsolationKey(circuit.IsolationDestination).
		WithDestination("en.wikipedia.org:443")

	// Get circuits for different destinations
	circGoogle, err := pool.GetWithIsolation(ctx, keyGoogle)
	if err != nil {
		log.Fatalf("Failed to get circuit for Google: %v", err)
	}
	fmt.Printf("Got circuit %d for www.google.com:443\n", circGoogle.ID)

	circWikipedia, err := pool.GetWithIsolation(ctx, keyWikipedia)
	if err != nil {
		log.Fatalf("Failed to get circuit for Wikipedia: %v", err)
	}
	fmt.Printf("Got circuit %d for en.wikipedia.org:443\n", circWikipedia.ID)

	if circGoogle.ID != circWikipedia.ID {
		fmt.Println("✓ Different destinations use different circuits")
	} else {
		fmt.Println("✗ Unexpected: different destinations should use different circuits")
	}

	// Return to pool
	pool.Put(circGoogle)
	pool.Put(circWikipedia)

	// Get again for same destination - should reuse
	circGoogleAgain, err := pool.GetWithIsolation(ctx, keyGoogle)
	if err != nil {
		log.Fatalf("Failed to get circuit for Google again: %v", err)
	}
	fmt.Printf("Got circuit %d for www.google.com:443 (second request)\n", circGoogleAgain.ID)

	if circGoogle.ID == circGoogleAgain.ID {
		fmt.Println("✓ Same destination reuses circuit from isolated pool")
	}
}

func demonstrateCredentialIsolation(ctx context.Context, pool *pool.CircuitPool) {
	// Simulate SOCKS5 authentication with different usernames
	keyAlice := circuit.NewIsolationKey(circuit.IsolationCredential).
		WithCredentials("alice")
	
	keyBob := circuit.NewIsolationKey(circuit.IsolationCredential).
		WithCredentials("bob")

	// Get circuits for different users
	circAlice, err := pool.GetWithIsolation(ctx, keyAlice)
	if err != nil {
		log.Fatalf("Failed to get circuit for Alice: %v", err)
	}
	fmt.Printf("Got circuit %d for user 'alice'\n", circAlice.ID)

	circBob, err := pool.GetWithIsolation(ctx, keyBob)
	if err != nil {
		log.Fatalf("Failed to get circuit for Bob: %v", err)
	}
	fmt.Printf("Got circuit %d for user 'bob'\n", circBob.ID)

	if circAlice.ID != circBob.ID {
		fmt.Println("✓ Different users (SOCKS5 usernames) use different circuits")
		fmt.Println("  This prevents correlation between different applications/users")
	}

	// Note: Credentials are hashed in the isolation key for privacy
	fmt.Printf("  Alice's isolation key: %s\n", circAlice.GetIsolationKey().String())
	fmt.Printf("  Bob's isolation key: %s\n", circBob.GetIsolationKey().String())
}

func demonstratePortIsolation(ctx context.Context, pool *pool.CircuitPool) {
	// Simulate connections from different client ports
	keyPort1 := circuit.NewIsolationKey(circuit.IsolationPort).
		WithSourcePort(12345)
	
	keyPort2 := circuit.NewIsolationKey(circuit.IsolationPort).
		WithSourcePort(54321)

	// Get circuits for different source ports
	circ1, err := pool.GetWithIsolation(ctx, keyPort1)
	if err != nil {
		log.Fatalf("Failed to get circuit for port 12345: %v", err)
	}
	fmt.Printf("Got circuit %d for client port 12345\n", circ1.ID)

	circ2, err := pool.GetWithIsolation(ctx, keyPort2)
	if err != nil {
		log.Fatalf("Failed to get circuit for port 54321: %v", err)
	}
	fmt.Printf("Got circuit %d for client port 54321\n", circ2.ID)

	if circ1.ID != circ2.ID {
		fmt.Println("✓ Different client ports use different circuits")
		fmt.Println("  This is useful when multiple applications connect from different ports")
	}
}

func demonstrateSessionIsolation(ctx context.Context, pool *pool.CircuitPool) {
	// Create isolation keys with custom session tokens
	keySession1 := circuit.NewIsolationKey(circuit.IsolationSession).
		WithSessionToken("shopping-session-abc123")
	
	keySession2 := circuit.NewIsolationKey(circuit.IsolationSession).
		WithSessionToken("browsing-session-xyz789")

	// Get circuits for different sessions
	circ1, err := pool.GetWithIsolation(ctx, keySession1)
	if err != nil {
		log.Fatalf("Failed to get circuit for session 1: %v", err)
	}
	fmt.Printf("Got circuit %d for shopping session\n", circ1.ID)

	circ2, err := pool.GetWithIsolation(ctx, keySession2)
	if err != nil {
		log.Fatalf("Failed to get circuit for session 2: %v", err)
	}
	fmt.Printf("Got circuit %d for browsing session\n", circ2.ID)

	if circ1.ID != circ2.ID {
		fmt.Println("✓ Different session tokens use different circuits")
		fmt.Println("  This allows application-level control over circuit isolation")
	}

	// Note: Session tokens are hashed for privacy
	fmt.Printf("  Shopping session key: %s\n", circ1.GetIsolationKey().String())
	fmt.Printf("  Browsing session key: %s\n", circ2.GetIsolationKey().String())
}
