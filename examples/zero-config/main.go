// Package main demonstrates zero-configuration usage of go-tor.
//
// This example shows how easy it is to use go-tor as a library:
// just import and call Connect() - no configuration needed!
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/opd-ai/go-tor/pkg/client"
)

func main() {
	fmt.Println("=== Zero-Configuration Tor Client Demo ===")
	fmt.Println()

	// That's it! Just one function call to get a working Tor client.
	// This automatically:
	// - Detects the appropriate data directory for your OS
	// - Creates necessary directories with proper permissions
	// - Connects to the Tor network
	// - Builds initial circuits
	// - Starts the SOCKS5 proxy
	fmt.Println("Connecting to Tor network...")
	fmt.Println("(This may take 30-60 seconds on first run)")
	fmt.Println()

	torClient, err := client.Connect()
	if err != nil {
		log.Fatalf("Failed to connect to Tor: %v", err)
	}
	defer torClient.Close()

	// Wait for the client to be ready
	// Use 90s timeout for first run (consensus download + circuit build)
	// Subsequent runs can use shorter timeout (30-60s)
	fmt.Println("Waiting for circuits to be established...")
	if err := torClient.WaitUntilReady(90 * time.Second); err != nil {
		log.Fatalf("Timeout waiting for Tor to be ready: %v", err)
	}

	// Get the SOCKS5 proxy URL
	proxyURL := torClient.ProxyURL()
	proxyAddr := torClient.ProxyAddr()

	fmt.Println()
	fmt.Println("✓ Connected to Tor network!")
	fmt.Printf("✓ SOCKS5 proxy available at: %s\n", proxyAddr)
	fmt.Printf("✓ Proxy URL: %s\n", proxyURL)
	fmt.Println()

	// Display statistics
	stats := torClient.Stats()
	fmt.Println("Current Status:")
	fmt.Printf("  Active Circuits: %d\n", stats.ActiveCircuits)
	fmt.Printf("  SOCKS Port: %d\n", stats.SocksPort)
	fmt.Printf("  Control Port: %d\n", stats.ControlPort)
	fmt.Printf("  Uptime: %ds\n", stats.UptimeSeconds)
	fmt.Println()

	// Show how to use with HTTP clients
	fmt.Println("Usage with HTTP clients:")
	fmt.Println("  Go net/http:")
	fmt.Printf("    proxyURL, _ := url.Parse(\"%s\")\n", proxyURL)
	fmt.Println("    client := &http.Client{")
	fmt.Println("        Transport: &http.Transport{")
	fmt.Println("            Proxy: http.ProxyURL(proxyURL),")
	fmt.Println("        },")
	fmt.Println("    }")
	fmt.Println()
	fmt.Println("  curl:")
	fmt.Printf("    curl --socks5 %s https://check.torproject.org\n", proxyAddr)
	fmt.Println()

	// Keep running for demonstration
	fmt.Println("Press Ctrl+C to exit")
	select {}
}
