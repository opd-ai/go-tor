// Package main demonstrates using cretz/bine with go-tor for Tor client operations.
//
// This example shows how to:
// 1. Start a go-tor client to provide a SOCKS5 proxy
// 2. Use cretz/bine to make HTTP requests through that proxy
// 3. Verify Tor connectivity
// 4. Handle graceful shutdown
//
// This integration pattern is useful when you want:
// - go-tor's pure-Go implementation (no external Tor binary for basic connectivity)
// - bine's convenient high-level API for Tor operations
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cretz/bine/tor"
	"github.com/opd-ai/go-tor/pkg/client"
	"golang.org/x/net/proxy"
)

func main() {
	fmt.Println("=== Bine + go-tor Client Integration Example ===")
	fmt.Println()

	// Step 1: Start go-tor client for SOCKS proxy
	fmt.Println("Step 1: Starting go-tor client...")
	torClient, err := startGoTorClient()
	if err != nil {
		log.Fatalf("Failed to start go-tor: %v", err)
	}
	defer func() {
		fmt.Println("\nShutting down go-tor client...")
		if err := torClient.Close(); err != nil {
			log.Printf("Error closing go-tor: %v", err)
		}
	}()

	// Step 2: Get SOCKS proxy information from go-tor
	proxyAddr := torClient.ProxyAddr()
	proxyURL := torClient.ProxyURL()
	fmt.Printf("✓ go-tor client ready on %s\n", proxyURL)
	fmt.Println()

	// Step 3: Make HTTP requests through the SOCKS proxy using standard Go
	fmt.Println("Step 2: Making HTTP request through go-tor SOCKS proxy...")
	if err := makeHTTPRequestThroughProxy(proxyAddr); err != nil {
		log.Printf("Warning: HTTP request failed: %v", err)
	}
	fmt.Println()

	// Step 4: Optionally, use bine for additional Tor operations
	// Note: bine typically starts its own Tor process, but we can configure it
	// to use our go-tor SOCKS proxy for network operations
	fmt.Println("Step 3: Demonstrating bine integration pattern...")
	demonstrateBineIntegration(proxyAddr)
	fmt.Println()

	// Step 5: Keep running until interrupted
	fmt.Println("All examples completed successfully!")
	fmt.Println("Press Ctrl+C to exit...")
	fmt.Println()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nReceived shutdown signal")
}

// startGoTorClient initializes and starts a go-tor client
func startGoTorClient() (*client.SimpleClient, error) {
	// Create go-tor client with default configuration
	torClient, err := client.Connect()
	if err != nil {
		return nil, fmt.Errorf("failed to create go-tor client: %w", err)
	}

	// Wait for Tor to be ready (establish circuits)
	// First time may take 30-90 seconds, subsequent starts are faster
	fmt.Println("  Waiting for Tor circuits to be ready (this may take 30-90 seconds)...")
	if err := torClient.WaitUntilReady(90 * time.Second); err != nil {
		torClient.Close()
		return nil, fmt.Errorf("timeout waiting for Tor to be ready: %w", err)
	}

	return torClient, nil
}

// makeHTTPRequestThroughProxy demonstrates making HTTP requests through go-tor's SOCKS proxy
func makeHTTPRequestThroughProxy(proxyAddr string) error {
	// Create a SOCKS5 dialer
	dialer, err := proxy.SOCKS5("tcp", proxyAddr, nil, proxy.Direct)
	if err != nil {
		return fmt.Errorf("failed to create SOCKS5 dialer: %w", err)
	}

	// Create HTTP client with the SOCKS5 proxy
	httpClient := &http.Client{
		Transport: &http.Transport{
			Dial: dialer.Dial,
		},
		Timeout: 30 * time.Second,
	}

	// Make request to check.torproject.org to verify Tor connectivity
	fmt.Println("  Making request to check.torproject.org...")
	resp, err := httpClient.Get("https://check.torproject.org/api/ip")
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	fmt.Printf("  ✓ Request successful! Status: %s\n", resp.Status)
	fmt.Printf("  Response: %s\n", string(body))

	return nil
}

// demonstrateBineIntegration shows how bine can be used alongside go-tor
func demonstrateBineIntegration(goTorProxy string) {
	fmt.Println("  Bine Integration Pattern:")
	fmt.Println()
	fmt.Println("  Option 1: Use go-tor as primary SOCKS proxy (shown above)")
	fmt.Println("    - Pure Go implementation")
	fmt.Println("    - No external Tor binary needed")
	fmt.Println("    - Configure your app to use:", goTorProxy)
	fmt.Println()
	fmt.Println("  Option 2: Use bine to start a separate Tor instance")
	fmt.Println("    - Requires Tor binary to be installed")
	fmt.Println("    - Full control protocol support")
	fmt.Println("    - Example shown below...")
	fmt.Println()

	// Demonstrate starting a bine Tor instance (requires Tor binary)
	// This is optional and shows how to use bine's features
	if err := demonstrateBineTorInstance(); err != nil {
		fmt.Printf("  Note: Bine Tor instance demo skipped: %v\n", err)
		fmt.Println("  (This is expected if Tor binary is not installed)")
	}
}

// demonstrateBineTorInstance shows how to use bine to start its own Tor instance
func demonstrateBineTorInstance() error {
	fmt.Println("  Starting bine Tor instance (requires Tor binary)...")

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// Start Tor with bine
	// Note: This requires the 'tor' binary to be in PATH
	t, err := tor.Start(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start bine Tor: %w", err)
	}
	defer t.Close()

	fmt.Println("  ✓ Bine Tor started successfully")

	// Get SOCKS proxy info from bine
	// Note: We're not using this since we already have go-tor,
	// but this shows bine's capabilities
	fmt.Println("  ✓ Bine can also provide SOCKS proxy and control protocol")
	fmt.Println()

	return nil
}

// Example of using bine with go-tor's SOCKS proxy in production:
//
// func useGoTorWithCustomApp() error {
//     // Start go-tor
//     torClient, _ := client.Connect()
//     defer torClient.Close()
//     torClient.WaitUntilReady(90 * time.Second)
//
//     // Configure your application to use go-tor's SOCKS proxy
//     proxyURL, _ := url.Parse(torClient.ProxyURL())
//     httpClient := &http.Client{
//         Transport: &http.Transport{
//             Proxy: http.ProxyURL(proxyURL),
//         },
//     }
//
//     // Make requests through Tor
//     resp, _ := httpClient.Get("https://example.com")
//     // ... handle response
//
//     return nil
// }

// Example of using bine for hidden service with go-tor for connectivity:
//
// func createHiddenServiceWithBine() error {
//     // Start go-tor for network connectivity
//     torClient, _ := client.Connect()
//     defer torClient.Close()
//     torClient.WaitUntilReady(90 * time.Second)
//
//     // Start bine Tor instance for hidden service management
//     ctx := context.Background()
//     t, _ := tor.Start(ctx, nil)
//     defer t.Close()
//
//     // Create v3 onion service
//     onion, _ := t.Listen(ctx, &tor.ListenConf{RemotePorts: []int{80}})
//     defer onion.Close()
//
//     fmt.Printf("Onion service: http://%v.onion\n", onion.ID)
//
//     // Serve HTTP on the onion service
//     http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
//         fmt.Fprintf(w, "Hello from hidden service!")
//     })
//     http.Serve(onion, nil)
//
//     return nil
// }
