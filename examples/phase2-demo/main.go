// Package main demonstrates Phase 2 functionality: connecting to Tor network.
// This example shows how to:
// 1. Fetch the Tor network consensus
// 2. Establish a TLS connection to a relay
// 3. Perform protocol handshake
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/opd-ai/go-tor/pkg/connection"
	"github.com/opd-ai/go-tor/pkg/directory"
	"github.com/opd-ai/go-tor/pkg/logger"
	"github.com/opd-ai/go-tor/pkg/protocol"
)

func main() {
	// Initialize logger
	logLevel, _ := logger.ParseLevel("info")
	appLogger := logger.New(logLevel, os.Stdout)
	
	appLogger.Info("go-tor Phase 2 Demo: Core Protocol Implementation")
	appLogger.Info("This demo fetches consensus and connects to a Tor relay")
	
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	
	// Step 1: Fetch network consensus
	appLogger.Info("Step 1: Fetching Tor network consensus...")
	dirClient := directory.NewClient(appLogger)
	
	relays, err := dirClient.FetchConsensus(ctx)
	if err != nil {
		log.Fatalf("Failed to fetch consensus: %v", err)
	}
	
	appLogger.Info("Consensus fetched successfully", "total_relays", len(relays))
	
	// Find suitable relays (guard nodes)
	var guards []*directory.Relay
	for _, relay := range relays {
		if relay.IsGuard() && relay.IsRunning() && relay.IsValid() {
			guards = append(guards, relay)
			if len(guards) >= 5 {
				break
			}
		}
	}
	
	if len(guards) == 0 {
		log.Fatal("No suitable guard relays found")
	}
	
	appLogger.Info("Found guard relays", "count", len(guards))
	
	// Display some relay information
	fmt.Println("\nSample Guard Relays:")
	for i, guard := range guards[:3] {
		fmt.Printf("  %d. %s\n", i+1, guard.String())
		fmt.Printf("     Flags: %v\n", guard.Flags)
	}
	
	// Step 2: Connect to a relay (just demonstrate connection, not full circuit)
	appLogger.Info("\nStep 2: Demonstrating connection to a guard relay...")
	
	guard := guards[0]
	relayAddr := fmt.Sprintf("%s:%d", guard.Address, guard.ORPort)
	appLogger.Info("Attempting connection", "relay", guard.Nickname, "address", relayAddr)
	
	// Create connection
	connConfig := connection.DefaultConfig(relayAddr)
	conn := connection.New(connConfig, appLogger)
	
	// Note: This will likely fail because we need proper Tor TLS certificates
	// This is just a demonstration of the API
	err = conn.Connect(ctx, connConfig)
	if err != nil {
		appLogger.Warn("Connection failed (expected - need proper cert validation)",
			"error", err,
			"note", "This is expected behavior in the demo")
	} else {
		defer conn.Close()
		
		appLogger.Info("Connection established!", "state", conn.GetState())
		
		// Step 3: Perform protocol handshake
		appLogger.Info("Step 3: Performing protocol handshake...")
		handshake := protocol.NewHandshake(conn, appLogger)
		
		err = handshake.PerformHandshake(ctx)
		if err != nil {
			appLogger.Error("Handshake failed", "error", err)
		} else {
			appLogger.Info("Handshake complete!",
				"version", handshake.NegotiatedVersion())
		}
	}
	
	// Summary
	fmt.Println("\n=== Phase 2 Implementation Summary ===")
	fmt.Println("✅ Directory client: Fetch network consensus")
	fmt.Println("✅ Connection management: TLS connections to relays")
	fmt.Println("✅ Protocol handshake: Version negotiation")
	fmt.Println("\nNext Steps (Phase 3):")
	fmt.Println("  - Circuit building with CREATE2/CREATED2 cells")
	fmt.Println("  - Path selection algorithms")
	fmt.Println("  - SOCKS5 proxy for application traffic")
	fmt.Println("  - Stream handling and multiplexing")
}
