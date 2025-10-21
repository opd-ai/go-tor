// Package main demonstrates the complete integration using the bine wrapper.
//
// This all-in-one example shows:
// 1. Zero-configuration connection using pkg/bine wrapper
// 2. Creating a hidden service with one function call
// 3. Accessing the hidden service through the integrated SOCKS proxy
// 4. Complete lifecycle management
//
// The bine wrapper automatically handles:
// - go-tor client for pure-Go Tor connectivity
// - bine for hidden service management
// - SOCKS5 proxy configuration
// - Lifecycle management and graceful shutdown
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

	"github.com/opd-ai/go-tor/pkg/bine"
)

func main() {
	fmt.Println("=== All-in-One: Bine Wrapper Integration ===")
	fmt.Println()
	fmt.Println("This example demonstrates zero-configuration integration:")
	fmt.Println("  1. Automatic go-tor client setup (pure Go)")
	fmt.Println("  2. Optional bine for hidden service management")
	fmt.Println("  3. Seamless SOCKS proxy configuration")
	fmt.Println()

	// Create context for managing lifecycle
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Step 1: Connect with zero configuration
	fmt.Println("Step 1: Connecting with bine wrapper (zero configuration)...")
	client, err := bine.ConnectWithOptions(&bine.Options{
		EnableBine:     true, // Enable hidden service support
		StartupTimeout: 120 * time.Second,
	})
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer func() {
		fmt.Println("\nShutting down...")
		client.Close()
	}()

	fmt.Printf("✓ Connected! SOCKS proxy: %s\n", client.ProxyAddr())
	fmt.Println()

	// Step 2: Create hidden service with one function call
	fmt.Println("Step 2: Creating hidden service...")
	fmt.Println("  (This requires Tor binary to be installed)")
	
	service, err := createHiddenService(ctx, client)
	if err != nil {
		fmt.Printf("❌ Failed to create hidden service: %v\n", err)
		fmt.Println()
		fmt.Println("This is expected if Tor binary is not installed.")
		fmt.Println("Install Tor with:")
		fmt.Println("  Ubuntu/Debian: sudo apt-get install tor")
		fmt.Println("  macOS: brew install tor")
		fmt.Println()
		fmt.Println("Continuing with client-only demonstration...")
		demonstrateClientOnly(client)
	} else {
		defer service.Close()
		
		onionAddr := service.OnionAddress()
		fmt.Printf("✓ Hidden service created: http://%s\n", onionAddr)
		fmt.Println()

		// Step 3: Access the hidden service through integrated proxy
		fmt.Println("Step 3: Accessing hidden service through integrated SOCKS proxy...")
		time.Sleep(5 * time.Second) // Give service time to be fully published
		
		if err := accessHiddenService(client, onionAddr); err != nil {
			fmt.Printf("Note: Could not access service yet: %v\n", err)
			fmt.Println("(Hidden services can take 2-3 minutes to be fully accessible)")
		}
		fmt.Println()

		// Display summary
		displaySummary(onionAddr, client.ProxyAddr())
	}

	// Keep running until interrupted
	fmt.Println("Press Ctrl+C to exit...")
	fmt.Println()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nReceived shutdown signal, cleaning up...")
}

// createHiddenService creates a hidden service using the bine wrapper
func createHiddenService(ctx context.Context, client *bine.Client) (*bine.HiddenService, error) {
	// Create v3 onion service with one function call
	service, err := client.CreateHiddenService(ctx, 80)
	if err != nil {
		return nil, fmt.Errorf("failed to create service: %w", err)
	}

	// Start HTTP server on the onion service
	mux := createHTTPHandler(service.OnionAddress())
	srv := &http.Server{Handler: mux}

	go func() {
		if err := srv.Serve(service); err != http.ErrServerClosed {
			log.Printf("Server error: %v", err)
		}
	}()

	return service, nil
}

// createHTTPHandler creates an HTTP handler for the hidden service
func createHTTPHandler(onionAddr string) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Bine Wrapper Example</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 800px; margin: 50px auto; padding: 20px; }
        .header { background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%); color: white; padding: 20px; border-radius: 10px; }
        .content { background: white; padding: 20px; margin-top: 20px; border-radius: 10px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        .info { background: #f0f0f0; padding: 15px; border-radius: 5px; margin: 15px 0; }
        code { background: #e8e8e8; padding: 2px 6px; border-radius: 3px; }
    </style>
</head>
<body>
    <div class="header">
        <h1>🚀 Bine Wrapper Integration</h1>
        <p>Zero-configuration go-tor + bine</p>
    </div>
    <div class="content">
        <h2>Welcome!</h2>
        <p>You're connected to a hidden service created with the <code>pkg/bine</code> wrapper!</p>
        
        <div class="info">
            <h3>What Makes This Special:</h3>
            <ul>
                <li>✓ Zero configuration required</li>
                <li>✓ One function call to create service</li>
                <li>✓ Automatic lifecycle management</li>
                <li>✓ Integrated SOCKS proxy</li>
                <li>✓ Production-ready error handling</li>
            </ul>
        </div>

        <div class="info">
            <h3>Service Details:</h3>
            <p><strong>Onion Address:</strong> <code>%s</code></p>
            <p><strong>Time:</strong> %s</p>
            <p><strong>Created with:</strong> <code>client.CreateHiddenService(ctx, 80)</code></p>
        </div>

        <h3>Try These Endpoints:</h3>
        <ul>
            <li><a href="/">/</a> - This page</li>
            <li><a href="/api">/api</a> - JSON API</li>
            <li><a href="/health">/health</a> - Health check</li>
        </ul>
    </div>
</body>
</html>`, onionAddr, time.Now().Format(time.RFC3339))

		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	})

	mux.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		response := fmt.Sprintf(`{
  "service": "Bine Wrapper Integration",
  "onion_address": "%s",
  "timestamp": "%s",
  "wrapper": "pkg/bine",
  "features": {
    "zero_config": true,
    "auto_lifecycle": true,
    "integrated_proxy": true
  }
}`, onionAddr, time.Now().Format(time.RFC3339))

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(response))
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"healthy","wrapper":"pkg/bine"}`))
	})

	return mux
}

// accessHiddenService attempts to access the hidden service through the integrated proxy
func accessHiddenService(client *bine.Client, onionAddr string) error {
	// Get HTTP client from wrapper
	httpClient, err := client.HTTPClient()
	if err != nil {
		return fmt.Errorf("failed to create HTTP client: %w", err)
	}

	// Access the hidden service
	url := fmt.Sprintf("http://%s/health", onionAddr)
	resp, err := httpClient.Get(url)
	if err != nil {
		return fmt.Errorf("failed to access service: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("  ✓ Successfully accessed service!\n")
	fmt.Printf("  Response: %s\n", string(body))

	return nil
}

// demonstrateClientOnly shows client-only functionality
func demonstrateClientOnly(client *bine.Client) {
	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("CLIENT-ONLY DEMONSTRATION")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Printf("SOCKS Proxy: %s\n", client.ProxyAddr())
	fmt.Println()
	fmt.Println("You can use this with any application:")
	fmt.Println()
	fmt.Println("Example with curl:")
	fmt.Printf("  curl --socks5 %s https://check.torproject.org\n", client.ProxyAddr())
	fmt.Println()
	fmt.Println("Example with the wrapper:")
	fmt.Println("  httpClient, _ := client.HTTPClient()")
	fmt.Println("  resp, _ := httpClient.Get(\"https://example.com\")")
	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
}

// displaySummary shows a summary of the running services
func displaySummary(onionAddr, proxyAddr string) {
	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("ALL SERVICES RUNNING")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Println("✓ Bine Wrapper (Zero Configuration)")
	fmt.Printf("  SOCKS Proxy: %s\n", proxyAddr)
	fmt.Printf("  Onion Service: http://%s\n", onionAddr)
	fmt.Println()
	fmt.Println("🌐 Access the service:")
	fmt.Println("  1. Via Tor Browser:")
	fmt.Printf("     http://%s\n", onionAddr)
	fmt.Println()
	fmt.Println("  2. Via curl (using integrated proxy):")
	fmt.Printf("     curl --socks5 %s http://%s\n", proxyAddr, onionAddr)
	fmt.Println()
	fmt.Println("  3. Via the wrapper's HTTP client")
	fmt.Println()
	fmt.Println("📋 API Endpoints:")
	fmt.Printf("  http://%s/         - Home page\n", onionAddr)
	fmt.Printf("  http://%s/api      - JSON API\n", onionAddr)
	fmt.Printf("  http://%s/health   - Health check\n", onionAddr)
	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
}
