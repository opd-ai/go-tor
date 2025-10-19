// Package main demonstrates zero-configuration usage with custom options.
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/opd-ai/go-tor/pkg/client"
)

func main() {
	fmt.Println("=== Zero-Configuration with Custom Options Demo ===")
	fmt.Println()

	// You can still customize settings while keeping the zero-config convenience
	opts := &client.Options{
		SocksPort:   19050, // Use custom port
		ControlPort: 19051,
		LogLevel:    "debug", // More verbose logging
		// DataDirectory is auto-detected if not specified
	}

	fmt.Println("Connecting to Tor network with custom options...")
	fmt.Printf("  SOCKS Port: %d\n", opts.SocksPort)
	fmt.Printf("  Control Port: %d\n", opts.ControlPort)
	fmt.Printf("  Log Level: %s\n", opts.LogLevel)
	fmt.Println()

	torClient, err := client.ConnectWithOptions(opts)
	if err != nil {
		log.Fatalf("Failed to connect to Tor: %v", err)
	}
	defer torClient.Close()

	// Wait for ready state
	fmt.Println("Waiting for circuits to be established...")
	if err := torClient.WaitUntilReady(60 * time.Second); err != nil {
		log.Fatalf("Timeout waiting for Tor to be ready: %v", err)
	}

	fmt.Println()
	fmt.Println("✓ Connected to Tor network!")
	fmt.Printf("✓ SOCKS5 proxy: %s\n", torClient.ProxyAddr())
	fmt.Println()

	// Display detailed statistics
	stats := torClient.Stats()
	fmt.Println("Detailed Statistics:")
	fmt.Printf("  Active Circuits: %d\n", stats.ActiveCircuits)
	fmt.Printf("  Circuit Builds: %d\n", stats.CircuitBuilds)
	fmt.Printf("  Circuit Build Success: %d\n", stats.CircuitBuildSuccess)
	fmt.Printf("  Circuit Build Failures: %d\n", stats.CircuitBuildFailure)
	fmt.Printf("  Avg Build Time: %s\n", stats.CircuitBuildTimeAvg)
	fmt.Printf("  P95 Build Time: %s\n", stats.CircuitBuildTimeP95)
	fmt.Printf("  Guard Nodes Active: %d\n", stats.GuardsActive)
	fmt.Printf("  Guard Nodes Confirmed: %d\n", stats.GuardsConfirmed)
	fmt.Printf("  Connection Attempts: %d\n", stats.ConnectionAttempts)
	fmt.Printf("  Connection Retries: %d\n", stats.ConnectionRetries)
	fmt.Printf("  Uptime: %ds\n", stats.UptimeSeconds)
	fmt.Println()

	fmt.Println("Client is running. Press Ctrl+C to exit")
	select {}
}
