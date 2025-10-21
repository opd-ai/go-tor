// Package main demonstrates using the bine wrapper for client operations.
//
// This example shows how to:
// 1. Connect with zero configuration using the bine wrapper
// 2. Make HTTP requests through Tor using the integrated HTTPClient
// 3. Use the SOCKS proxy for custom applications
// 4. Handle graceful shutdown
//
// The bine wrapper simplifies integration by automatically:
// - Starting go-tor client (pure Go, no external binary)
// - Configuring SOCKS5 proxy
// - Managing lifecycle and cleanup
package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/opd-ai/go-tor/pkg/bine"
)

func main() {
	fmt.Println("=== Bine Wrapper Client Example ===")
	fmt.Println()

	// Step 1: Connect with zero configuration
	fmt.Println("Step 1: Connecting with bine wrapper...")
	client, err := bine.Connect()
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer func() {
		fmt.Println("\nShutting down...")
		if err := client.Close(); err != nil {
			log.Printf("Error during shutdown: %v", err)
		}
	}()

	proxyAddr := client.ProxyAddr()
	fmt.Printf("✓ Connected! SOCKS proxy: %s\n", proxyAddr)
	fmt.Println()

	// Step 2: Make HTTP request using integrated HTTPClient
	fmt.Println("Step 2: Making HTTP request through Tor...")
	if err := makeHTTPRequest(client); err != nil {
		log.Printf("Warning: HTTP request failed: %v", err)
	}
	fmt.Println()

	// Step 3: Show usage examples
	fmt.Println("Step 3: Usage examples")
	demonstrateUsage(client)
	fmt.Println()

	// All examples completed
	fmt.Println("All examples completed successfully!")
	fmt.Println("Press Ctrl+C to exit...")
	fmt.Println()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nReceived shutdown signal")
}

// makeHTTPRequest demonstrates making HTTP requests using the wrapper's HTTPClient
func makeHTTPRequest(client *bine.Client) error {
	// Get HTTP client from wrapper (automatically configured for Tor)
	httpClient, err := client.HTTPClient()
	if err != nil {
		return fmt.Errorf("failed to create HTTP client: %w", err)
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

// demonstrateUsage shows different ways to use the wrapper
func demonstrateUsage(client *bine.Client) {
	fmt.Println("  The bine wrapper provides multiple interfaces:")
	fmt.Println()
	
	fmt.Println("  1. Zero-Configuration HTTP Client:")
	fmt.Println("     httpClient, _ := client.HTTPClient()")
	fmt.Println("     resp, _ := httpClient.Get(\"https://example.com\")")
	fmt.Println()
	
	fmt.Println("  2. SOCKS Proxy Address:")
	fmt.Printf("     %s\n", client.ProxyAddr())
	fmt.Println("     Use with curl: curl --socks5", client.ProxyAddr(), "https://example.com")
	fmt.Println()
	
	fmt.Println("  3. Custom Dialer:")
	fmt.Println("     dialer := client.Dialer()")
	fmt.Println("     conn, _ := dialer.Dial(\"tcp\", \"example.com:80\")")
	fmt.Println()
	
	fmt.Println("  4. Check Readiness:")
	if client.IsReady() {
		fmt.Println("     ✓ Client is ready")
	}
}
