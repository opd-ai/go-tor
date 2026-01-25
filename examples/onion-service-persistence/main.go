// Example: Onion Service with Key Persistence
//
// This example demonstrates how to create an onion service that persists
// its keys across restarts, maintaining the same .onion address.
//
// Usage:
//   go run examples/onion-service-persistence/main.go

package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/opd-ai/go-tor/pkg/logger"
	"github.com/opd-ai/go-tor/pkg/onion"
)

func main() {
	// Create data directory for persistent storage
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Failed to get home directory: %v", err)
	}

	dataDir := filepath.Join(homeDir, ".go-tor-example")
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	fmt.Printf("Data directory: %s\n", dataDir)

	// Start a simple HTTP server on localhost:8080
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello from onion service!\n")
		fmt.Fprintf(w, "Time: %s\n", time.Now().Format(time.RFC3339))
	})

	go func() {
		fmt.Println("Starting HTTP server on localhost:8080...")
		if err := http.ListenAndServe("localhost:8080", nil); err != nil {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// Create logger
	lgr := logger.New(slog.LevelInfo, os.Stdout)

	// Configure onion service with persistence
	config := &onion.ServiceConfig{
		DataDirectory: dataDir, // Keys will be saved/loaded from here
		Ports: map[int]string{
			80: "localhost:8080", // Map onion service port 80 to local HTTP server
		},
		NumIntroPoints:     3,
		DescriptorLifetime: 3 * time.Hour,
	}

	// Create onion service
	// On first run: generates new keys and saves them
	// On subsequent runs: loads existing keys from dataDir
	service, err := onion.NewService(config, lgr)
	if err != nil {
		log.Fatalf("Failed to create onion service: %v", err)
	}

	// Get the onion address
	address := service.GetAddress()
	fmt.Printf("\n========================================\n")
	fmt.Printf("Onion Service Address: %s\n", address)
	fmt.Printf("========================================\n\n")
	fmt.Printf("This address will remain the same across restarts!\n")
	fmt.Printf("Keys are stored in: %s\n\n", dataDir)

	// Note: In a real implementation, you would also need to:
	// 1. Start the onion service with service.Start()
	// 2. Establish introduction points
	// 3. Publish descriptors to hidden service directories
	//
	// This example focuses on demonstrating key persistence.

	fmt.Println("Service created. Press Ctrl+C to exit.")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nShutting down...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Stop the service
	if err := service.Stop(); err != nil {
		log.Printf("Error stopping service: %v", err)
	}

	<-ctx.Done()
	fmt.Println("Service stopped.")

	// Show key file locations
	fmt.Println("\nPersistent key files:")
	fmt.Printf("  Identity key: %s/hs_ed25519_secret_key\n", dataDir)
	fmt.Printf("  Ntor key:     %s/hs_ntor_secret_key\n", dataDir)
	fmt.Println("\nRun this program again to see the same onion address!")
}
