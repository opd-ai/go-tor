// Package main demonstrates the complete integration of cretz/bine with go-tor.
//
// This all-in-one example shows:
// 1. Starting go-tor client for network connectivity
// 2. Using bine to create a hidden service
// 3. Accessing the hidden service through go-tor's SOCKS proxy
// 4. Complete lifecycle management
//
// This demonstrates the full power of combining both libraries:
// - go-tor provides pure-Go Tor connectivity (no external binary for client)
// - bine provides convenient hidden service management
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
	fmt.Println("=== All-in-One: Bine + go-tor Integration ===")
	fmt.Println()
	fmt.Println("This example demonstrates the complete integration:")
	fmt.Println("  1. go-tor for client connectivity (pure Go)")
	fmt.Println("  2. bine for hidden service management")
	fmt.Println("  3. Accessing the service through go-tor")
	fmt.Println()

	// Create context for managing lifecycle
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Step 1: Start go-tor client for network connectivity
	fmt.Println("Step 1: Starting go-tor client...")
	torClient, err := startGoTorClient()
	if err != nil {
		log.Fatalf("Failed to start go-tor: %v", err)
	}
	defer func() {
		fmt.Println("\nShutting down go-tor client...")
		torClient.Close()
	}()

	proxyAddr := torClient.ProxyAddr()
	fmt.Printf("✓ go-tor client ready on %s\n", proxyAddr)
	fmt.Println()

	// Step 2: Start bine and create hidden service
	fmt.Println("Step 2: Creating hidden service with bine...")
	fmt.Println("  (This requires Tor binary to be installed)")
	
	onionAddr, cleanup, err := startHiddenService(ctx)
	if err != nil {
		fmt.Printf("❌ Failed to start hidden service: %v\n", err)
		fmt.Println()
		fmt.Println("This is expected if Tor binary is not installed.")
		fmt.Println("Install Tor with:")
		fmt.Println("  Ubuntu/Debian: sudo apt-get install tor")
		fmt.Println("  macOS: brew install tor")
		fmt.Println()
		fmt.Println("Continuing with client-only demonstration...")
		demonstrateClientOnly(proxyAddr)
	} else {
		defer cleanup()
		
		fmt.Printf("✓ Hidden service created: http://%s\n", onionAddr)
		fmt.Println()

		// Step 3: Access the hidden service through go-tor
		fmt.Println("Step 3: Accessing hidden service through go-tor...")
		time.Sleep(5 * time.Second) // Give service time to be fully published
		
		if err := accessHiddenService(proxyAddr, onionAddr); err != nil {
			fmt.Printf("Note: Could not access service yet: %v\n", err)
			fmt.Println("(Hidden services can take 2-3 minutes to be fully accessible)")
		}
		fmt.Println()

		// Display summary
		displaySummary(onionAddr, proxyAddr)
	}

	// Keep running until interrupted
	fmt.Println("Press Ctrl+C to exit...")
	fmt.Println()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nReceived shutdown signal, cleaning up...")
}

// startGoTorClient initializes and starts a go-tor client
func startGoTorClient() (*client.SimpleClient, error) {
	torClient, err := client.Connect()
	if err != nil {
		return nil, fmt.Errorf("failed to create go-tor client: %w", err)
	}

	fmt.Println("  Waiting for Tor circuits (30-90 seconds)...")
	if err := torClient.WaitUntilReady(90 * time.Second); err != nil {
		torClient.Close()
		return nil, fmt.Errorf("timeout waiting for Tor: %w", err)
	}

	return torClient, nil
}

// startHiddenService creates a hidden service using bine
func startHiddenService(ctx context.Context) (string, func(), error) {
	// Start Tor with bine
	fmt.Println("  Starting bine Tor instance...")
	startCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	t, err := tor.Start(startCtx, nil)
	if err != nil {
		return "", nil, fmt.Errorf("failed to start bine Tor: %w", err)
	}

	// Create hidden service
	fmt.Println("  Creating onion service (2-3 minutes)...")
	listenCtx, listenCancel := context.WithTimeout(ctx, 5*time.Minute)
	defer listenCancel()

	conf := &tor.ListenConf{
		RemotePorts: []int{80},
		Version3:    true,
	}

	onion, err := t.Listen(listenCtx, conf)
	if err != nil {
		t.Close()
		return "", nil, fmt.Errorf("failed to create onion service: %w", err)
	}

	// Start HTTP server on the onion service
	srv := &http.Server{
		Handler: createHTTPHandler(onion.ID),
	}

	go func() {
		if err := srv.Serve(onion); err != http.ErrServerClosed {
			log.Printf("Server error: %v", err)
		}
	}()

	// Cleanup function
	cleanup := func() {
		fmt.Println("Shutting down hidden service...")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		srv.Shutdown(shutdownCtx)
		onion.Close()
		t.Close()
		fmt.Println("✓ Hidden service shut down")
	}

	return fmt.Sprintf("%v.onion", onion.ID), cleanup, nil
}

// createHTTPHandler creates an HTTP handler for the hidden service
func createHTTPHandler(onionID string) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>All-in-One Example</title>
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
        <h1>🚀 All-in-One Integration Example</h1>
        <p>Bine + go-tor working together!</p>
    </div>
    <div class="content">
        <h2>Welcome!</h2>
        <p>You're connected to a hidden service created with <code>cretz/bine</code>, accessible through <code>go-tor</code>'s SOCKS proxy.</p>
        
        <div class="info">
            <h3>This Demonstrates:</h3>
            <ul>
                <li>✓ go-tor providing pure-Go Tor connectivity</li>
                <li>✓ bine managing the hidden service</li>
                <li>✓ Complete integration between both libraries</li>
                <li>✓ End-to-end encrypted connection</li>
            </ul>
        </div>

        <div class="info">
            <h3>Service Details:</h3>
            <p><strong>Onion Address:</strong> <code>%s.onion</code></p>
            <p><strong>Time:</strong> %s</p>
        </div>

        <h3>Try These Endpoints:</h3>
        <ul>
            <li><a href="/">/</a> - This page</li>
            <li><a href="/api">/api</a> - JSON API</li>
            <li><a href="/health">/health</a> - Health check</li>
        </ul>
    </div>
</body>
</html>`, onionID, time.Now().Format(time.RFC3339))

		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	})

	mux.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		response := fmt.Sprintf(`{
  "service": "All-in-One Integration Example",
  "onion_id": "%s",
  "timestamp": "%s",
  "powered_by": ["go-tor", "cretz/bine"],
  "features": {
    "client": "go-tor (pure Go)",
    "hidden_service": "bine",
    "integration": "seamless"
  }
}`, onionID, time.Now().Format(time.RFC3339))

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(response))
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"healthy","integration":"active"}`))
	})

	return mux
}

// accessHiddenService attempts to access the hidden service through go-tor's SOCKS proxy
func accessHiddenService(proxyAddr, onionAddr string) error {
	// Create SOCKS5 dialer
	dialer, err := proxy.SOCKS5("tcp", proxyAddr, nil, proxy.Direct)
	if err != nil {
		return fmt.Errorf("failed to create SOCKS5 dialer: %w", err)
	}

	// Create HTTP client
	httpClient := &http.Client{
		Transport: &http.Transport{
			Dial: dialer.Dial,
		},
		Timeout: 30 * time.Second,
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

// demonstrateClientOnly shows client-only functionality when hidden service isn't available
func demonstrateClientOnly(proxyAddr string) {
	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("CLIENT-ONLY DEMONSTRATION")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Printf("go-tor SOCKS Proxy: %s\n", proxyAddr)
	fmt.Println()
	fmt.Println("You can use this proxy with any application:")
	fmt.Println()
	fmt.Println("Example with curl:")
	fmt.Printf("  curl --socks5 %s https://check.torproject.org\n", proxyAddr)
	fmt.Println()
	fmt.Println("Example with Go:")
	fmt.Println("  dialer, _ := proxy.SOCKS5(\"tcp\", proxyAddr, nil, proxy.Direct)")
	fmt.Println("  httpClient := &http.Client{Transport: &http.Transport{Dial: dialer.Dial}}")
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
	fmt.Println("✓ go-tor Client (Pure Go)")
	fmt.Printf("  SOCKS Proxy: %s\n", proxyAddr)
	fmt.Println()
	fmt.Println("✓ Bine Hidden Service")
	fmt.Printf("  Onion Address: http://%s\n", onionAddr)
	fmt.Println()
	fmt.Println("🌐 Access the service:")
	fmt.Println("  1. Via Tor Browser:")
	fmt.Printf("     http://%s\n", onionAddr)
	fmt.Println()
	fmt.Println("  2. Via curl (using go-tor's proxy):")
	fmt.Printf("     curl --socks5 %s http://%s\n", proxyAddr, onionAddr)
	fmt.Println()
	fmt.Println("  3. Via any app configured to use the SOCKS proxy")
	fmt.Println()
	fmt.Println("📋 API Endpoints:")
	fmt.Printf("  http://%s/         - Home page\n", onionAddr)
	fmt.Printf("  http://%s/api      - JSON API\n", onionAddr)
	fmt.Printf("  http://%s/health   - Health check\n", onionAddr)
	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
}
