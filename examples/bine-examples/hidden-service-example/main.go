// Package main demonstrates creating a v3 onion service using the bine wrapper.
//
// This example shows how to:
// 1. Create a hidden service with one function call using the bine wrapper
// 2. Serve HTTP content over the onion service
// 3. Handle the complete lifecycle with automatic cleanup
//
// The wrapper simplifies hidden service creation by:
// - Automatically starting go-tor for connectivity
// - Enabling bine for hidden service management
// - Managing lifecycle and cleanup
//
// IMPORTANT: This example requires the Tor binary to be installed on your system.
// - Ubuntu/Debian: sudo apt-get install tor
// - macOS: brew install tor
// - Windows: Download from torproject.org
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/opd-ai/go-tor/pkg/bine"
)

func main() {
	fmt.Println("=== Bine Wrapper Hidden Service Example ===")
	fmt.Println()
	fmt.Println("This example demonstrates creating a v3 onion service with zero configuration.")
	fmt.Println("The wrapper handles all the setup automatically.")
	fmt.Println()

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Step 1: Connect with bine enabled
	fmt.Println("Step 1: Connecting with bine enabled (this may take up to 90 seconds)...")
	client, err := bine.ConnectWithOptions(&bine.Options{
		EnableBine:     true, // Required for hidden services
		StartupTimeout: 120 * time.Second,
	})
	if err != nil {
		log.Fatalf("Failed to connect: %v\n\nThis requires Tor binary to be installed:\n  Ubuntu/Debian: sudo apt-get install tor\n  macOS: brew install tor\n  Windows: Download from https://www.torproject.org/download/", err)
	}
	defer func() {
		fmt.Println("\nShutting down...")
		client.Close()
		fmt.Println("✓ Shut down successfully")
	}()

	fmt.Println("✓ Connected successfully")
	fmt.Println()

	// Step 2: Create hidden service with one function call
	fmt.Println("Step 2: Creating v3 onion service...")
	fmt.Println("  This may take 2-3 minutes as the service is published to the network...")
	service, err := client.CreateHiddenService(ctx, 80)
	if err != nil {
		log.Fatalf("Failed to create hidden service: %v", err)
	}
	defer service.Close()

	// Display onion address
	onionAddr := service.OnionAddress()
	fmt.Println()
	fmt.Println("✓ Onion service created successfully!")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("  Onion Address: http://%s\n", onionAddr)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	// Step 3: Start HTTP server on the onion service
	fmt.Println("Step 3: Starting HTTP server on the onion service...")
	srv := createHTTPServer(onionAddr)

	// Start server in background
	errChan := make(chan error, 1)
	go func() {
		if err := srv.Serve(service); err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	fmt.Println("✓ HTTP server started")
	fmt.Println()
	displayServiceInfo(onionAddr)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Monitor for errors or shutdown signal
	select {
	case err := <-errChan:
		log.Fatalf("Server error: %v", err)
	case <-sigChan:
		fmt.Println("\n\nReceived shutdown signal...")
		fmt.Println("Shutting down HTTP server...")

		// Graceful shutdown with timeout
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("Error during server shutdown: %v", err)
		}
		fmt.Println("✓ HTTP server shut down")
	}
}

// createHTTPServer creates an HTTP server for the hidden service
func createHTTPServer(onionAddr string) *http.Server {
	// Create HTTP mux
	mux := http.NewServeMux()

	// Root handler - displays welcome page
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Bine Wrapper Hidden Service</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            max-width: 800px;
            margin: 50px auto;
            padding: 20px;
            background-color: #f5f5f5;
        }
        .container {
            background-color: white;
            padding: 30px;
            border-radius: 10px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
        }
        h1 { color: #7d4698; }
        .info { 
            background-color: #f0f0f0;
            padding: 15px;
            border-radius: 5px;
            margin: 20px 0;
        }
        .onion-addr {
            font-family: monospace;
            background-color: #e8e8e8;
            padding: 5px 10px;
            border-radius: 3px;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>🧅 Welcome to the Bine Wrapper Hidden Service!</h1>
        
        <p>This is a v3 onion service created with the <code>pkg/bine</code> wrapper.</p>
        
        <div class="info">
            <h3>Service Information:</h3>
            <p><strong>Onion Address:</strong> <span class="onion-addr">%s</span></p>
            <p><strong>Status:</strong> ✓ Online</p>
            <p><strong>Created with:</strong> <code>client.CreateHiddenService(ctx, 80)</code></p>
            <p><strong>Protocol:</strong> Tor v3 Onion Services</p>
        </div>
        
        <h3>Features:</h3>
        <ul>
            <li>Zero configuration required</li>
            <li>One function call to create</li>
            <li>Automatic lifecycle management</li>
            <li>End-to-end encrypted connection</li>
            <li>Hidden location and IP address</li>
            <li>NAT traversal (no port forwarding needed)</li>
        </ul>
        
        <h3>Available Endpoints:</h3>
        <ul>
            <li><a href="/">/</a> - This page</li>
            <li><a href="/api">/api</a> - API endpoint</li>
            <li><a href="/health">/health</a> - Health check</li>
        </ul>
        
        <hr>
        <p><em>Created with pkg/bine wrapper</em></p>
    </div>
</body>
</html>`, onionAddr)

		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(html))
	})

	// API endpoint - returns JSON
	mux.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		response := fmt.Sprintf(`{
  "service": "Bine Wrapper Hidden Service",
  "onion_address": "%s",
  "status": "online",
  "timestamp": "%s",
  "wrapper": "pkg/bine",
  "features": ["zero_config", "auto_lifecycle", "v3_onion"]
}`, onionAddr, time.Now().Format(time.RFC3339))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	})

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","service":"online","wrapper":"pkg/bine"}`))
	})

	return &http.Server{
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

// displayServiceInfo shows information about accessing the service
func displayServiceInfo(onionAddr string) {
	fmt.Println("SERVICE INFORMATION:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Println("Your onion service is now online and accessible!")
	fmt.Println()
	fmt.Println("🌐 Access the service:")
	fmt.Printf("   http://%s\n", onionAddr)
	fmt.Println()
	fmt.Println("📝 Available endpoints:")
	fmt.Printf("   http://%s/          - Home page\n", onionAddr)
	fmt.Printf("   http://%s/api       - JSON API\n", onionAddr)
	fmt.Printf("   http://%s/health    - Health check\n", onionAddr)
	fmt.Println()
	fmt.Println("🔐 Security features:")
	fmt.Println("   ✓ End-to-end encryption")
	fmt.Println("   ✓ Hidden server location")
	fmt.Println("   ✓ Self-authenticating address")
	fmt.Println("   ✓ NAT traversal (no port forwarding)")
	fmt.Println()
	fmt.Println("📱 How to access:")
	fmt.Println("   1. Use Tor Browser: https://www.torproject.org/download/")
	fmt.Println("   2. Or any application configured to use Tor SOCKS proxy")
	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Println("Press Ctrl+C to stop the service...")
	fmt.Println()
}
