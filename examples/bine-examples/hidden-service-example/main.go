// Package main demonstrates creating a v3 onion service using cretz/bine.
//
// This example shows how to:
// 1. Create and manage a v3 onion service (hidden service) using bine
// 2. Serve HTTP content over the onion service
// 3. Handle the complete lifecycle of a hidden service
// 4. Optionally integrate with go-tor for additional functionality
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

	"github.com/cretz/bine/tor"
)

func main() {
	fmt.Println("=== Bine Hidden Service Example ===")
	fmt.Println()
	fmt.Println("This example demonstrates creating a v3 onion service using cretz/bine.")
	fmt.Println("The onion service will host a simple HTTP server.")
	fmt.Println()

	// Check for Tor binary
	fmt.Println("Checking for Tor binary...")
	if !checkTorBinary() {
		fmt.Println("❌ Tor binary not found!")
		fmt.Println()
		fmt.Println("Please install Tor:")
		fmt.Println("  Ubuntu/Debian: sudo apt-get install tor")
		fmt.Println("  macOS: brew install tor")
		fmt.Println("  Windows: Download from https://www.torproject.org/download/")
		fmt.Println()
		os.Exit(1)
	}
	fmt.Println("✓ Tor binary found")
	fmt.Println()

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Step 1: Start Tor
	fmt.Println("Step 1: Starting Tor (this may take 30-60 seconds)...")
	t, err := startTor(ctx)
	if err != nil {
		log.Fatalf("Failed to start Tor: %v", err)
	}
	defer func() {
		fmt.Println("\nShutting down Tor...")
		if err := t.Close(); err != nil {
			log.Printf("Error closing Tor: %v", err)
		}
		fmt.Println("✓ Tor shut down successfully")
	}()
	fmt.Println("✓ Tor started successfully")
	fmt.Println()

	// Step 2: Create onion service
	fmt.Println("Step 2: Creating v3 onion service...")
	fmt.Println("  This may take 2-3 minutes as the service is published to the network...")
	onion, err := createOnionService(ctx, t)
	if err != nil {
		log.Fatalf("Failed to create onion service: %v", err)
	}
	defer onion.Close()

	// Display onion address
	onionAddr := fmt.Sprintf("%v.onion", onion.ID)
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
		if err := srv.Serve(onion); err != http.ErrServerClosed {
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

// checkTorBinary checks if the Tor binary is available
func checkTorBinary() bool {
	// Try to start Tor briefly to check if it's available
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	t, err := tor.Start(ctx, nil)
	if err != nil {
		return false
	}
	t.Close()
	return true
}

// startTor initializes and starts a Tor instance
func startTor(ctx context.Context) (*tor.Tor, error) {
	// Start Tor with default configuration
	// This will start a Tor process and connect to the network
	startCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	t, err := tor.Start(startCtx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to start Tor: %w", err)
	}

	return t, nil
}

// createOnionService creates a v3 onion service
func createOnionService(ctx context.Context, t *tor.Tor) (*tor.OnionService, error) {
	// Create context with timeout for service creation
	// This can take 2-3 minutes on first run
	listenCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	// Create v3 onion service listening on port 80
	// The service will be accessible at http://<onion-id>.onion
	conf := &tor.ListenConf{
		RemotePorts: []int{80}, // Port that clients will connect to
		Version3:    true,      // Use v3 onion services (recommended)
		// LocalPort is set automatically to a random available port
	}

	onion, err := t.Listen(listenCtx, conf)
	if err != nil {
		return nil, fmt.Errorf("failed to create onion service: %w", err)
	}

	return onion, nil
}

// createHTTPServer creates an HTTP server for the onion service
func createHTTPServer(onionAddr string) *http.Server {
	// Create HTTP mux
	mux := http.NewServeMux()

	// Root handler - displays welcome page
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Bine Hidden Service</title>
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
        <h1>🧅 Welcome to the Bine Hidden Service!</h1>
        
        <p>This is a v3 onion service created using <code>cretz/bine</code>.</p>
        
        <div class="info">
            <h3>Service Information:</h3>
            <p><strong>Onion Address:</strong> <span class="onion-addr">%s</span></p>
            <p><strong>Status:</strong> ✓ Online</p>
            <p><strong>Protocol:</strong> Tor v3 Onion Services</p>
        </div>
        
        <h3>Features:</h3>
        <ul>
            <li>End-to-end encrypted connection</li>
            <li>Hidden location and IP address</li>
            <li>NAT traversal (no port forwarding needed)</li>
            <li>Self-authenticating address</li>
        </ul>
        
        <h3>Available Endpoints:</h3>
        <ul>
            <li><a href="/">/</a> - This page</li>
            <li><a href="/api">/api</a> - API endpoint</li>
            <li><a href="/health">/health</a> - Health check</li>
        </ul>
        
        <hr>
        <p><em>Created with cretz/bine and go-tor</em></p>
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
  "service": "Bine Hidden Service",
  "onion_address": "%s",
  "status": "online",
  "timestamp": "%s",
  "features": ["hidden", "encrypted", "authenticated"]
}`, onionAddr, time.Now().Format(time.RFC3339))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	})

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","service":"online"}`))
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
