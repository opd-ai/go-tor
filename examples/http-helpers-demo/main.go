package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/opd-ai/go-tor/pkg/client"
	"github.com/opd-ai/go-tor/pkg/helpers"
)

func main() {
	fmt.Println("=== go-tor HTTP Client Helpers Demo ===")
	fmt.Println()

	// Step 1: Connect to Tor with zero configuration
	fmt.Println("1. Starting Tor client...")
	torClient, err := client.Connect()
	if err != nil {
		log.Fatalf("Failed to connect to Tor: %v", err)
	}
	defer torClient.Close()

	// Wait for Tor to be ready (first connection may take 30-60 seconds)
	fmt.Println("2. Waiting for Tor to bootstrap (this may take 30-60 seconds)...")
	if err := torClient.WaitUntilReady(90 * time.Second); err != nil {
		log.Fatalf("Tor failed to become ready: %v", err)
	}
	fmt.Println("   ✓ Tor is ready!")
	fmt.Println()

	// Step 2: Create HTTP client using helper
	fmt.Println("3. Creating HTTP client with Tor proxy...")
	httpClient, err := helpers.NewHTTPClient(torClient, nil)
	if err != nil {
		log.Fatalf("Failed to create HTTP client: %v", err)
	}
	fmt.Println("   ✓ HTTP client created")
	fmt.Println()

	// Step 3: Make a request through Tor
	fmt.Println("4. Making HTTP request through Tor...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := httpClient.Get("https://check.torproject.org/api/ip")
	if err != nil {
		log.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read response: %v", err)
	}

	fmt.Printf("   ✓ Response: %s\n", string(body))
	fmt.Println()

	// Step 4: Demonstrate custom configuration
	fmt.Println("5. Creating HTTP client with custom configuration...")
	customConfig := &helpers.HTTPClientConfig{
		Timeout:             60 * time.Second,
		MaxIdleConns:        20,
		DisableKeepAlives:   false,
		IdleConnTimeout:     120 * time.Second,
		TLSHandshakeTimeout: 15 * time.Second,
	}

	customHTTPClient, err := helpers.NewHTTPClient(torClient, customConfig)
	if err != nil {
		log.Fatalf("Failed to create custom HTTP client: %v", err)
	}
	fmt.Printf("   ✓ Custom HTTP client created (Timeout: %v)\n", customConfig.Timeout)
	fmt.Println()

	// Step 5: Demonstrate wrapping existing client
	fmt.Println("6. Wrapping an existing HTTP client...")
	existingClient := &http.Client{
		Timeout: 45 * time.Second,
	}

	if err := helpers.WrapHTTPClient(existingClient, torClient, nil); err != nil {
		log.Fatalf("Failed to wrap HTTP client: %v", err)
	}
	fmt.Println("   ✓ Existing HTTP client now routes through Tor")
	fmt.Println()

	// Step 6: Demonstrate DialContext
	fmt.Println("7. Creating custom dialer for advanced use cases...")
	dialFunc := helpers.DialContext(torClient)
	fmt.Println("   ✓ Custom dial function created")
	fmt.Println("   Note: Can be used with custom network applications")
	fmt.Println()

	fmt.Println("=== Demo Complete ===")
	fmt.Println()
	fmt.Println("Key Features Demonstrated:")
	fmt.Println("  • Zero-configuration Tor client startup")
	fmt.Println("  • Simple HTTP client creation with defaults")
	fmt.Println("  • Custom HTTP client configuration")
	fmt.Println("  • Wrapping existing HTTP clients")
	fmt.Println("  • Context-aware dialing for custom applications")
	fmt.Println()
	fmt.Println("This makes integrating Tor into your Go applications trivial!")

	// Unused variable fix
	_ = ctx
	_ = customHTTPClient
	_ = dialFunc
}
