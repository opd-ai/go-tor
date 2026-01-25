// Example demonstrating pluggable transport client usage.
//
// This example shows how to use the PT client to connect through
// an external pluggable transport like obfs4proxy.
//
// Prerequisites:
// - Install obfs4proxy: apt install obfs4proxy (or download from Tor Project)
// - Obtain bridge credentials with obfs4 cert and iat-mode
//
// Usage:
//   go run main.go

package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/opd-ai/go-tor/pkg/pt"
)

func main() {
	// Configure PT client
	config := pt.TransportConfig{
		BinaryPath: "/usr/bin/obfs4proxy", // Adjust path for your system
		StateDir:   "/tmp/go-tor-pt-demo",
		Options:    map[string]string{
			// Example obfs4 options (get these from your bridge configuration)
			// "cert":     "...",
			// "iat-mode": "0",
		},
	}

	// Create managed PT client
	client, err := pt.NewManagedClient(config)
	if err != nil {
		log.Fatalf("Failed to create PT client: %v", err)
	}

	fmt.Println("Starting pluggable transport client...")
	fmt.Printf("Binary: %s\n", config.BinaryPath)
	fmt.Printf("State directory: %s\n", config.StateDir)

	// Start PT process and perform handshake
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := client.Start(ctx); err != nil {
		log.Fatalf("Failed to start PT: %v", err)
	}
	defer client.Close()

	fmt.Println("✓ PT process started successfully")

	// Check available transport methods
	methods := client.Methods()
	if len(methods) == 0 {
		log.Fatal("No transport methods available")
	}

	fmt.Printf("✓ Available transports: %v\n", methods)

	// Example: Connect through PT
	// In real usage, you would use this connection for Tor circuit building
	fmt.Println("\nPT client ready for connections.")
	fmt.Println("In a real application, use client.Dial(ctx, address) to connect through the PT.")

	// Keep PT running for demo
	time.Sleep(2 * time.Second)

	fmt.Println("\nShutting down PT client...")
}
