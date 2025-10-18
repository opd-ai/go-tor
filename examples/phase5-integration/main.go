// Package main demonstrates the integrated Tor client with all Phase 1-4 components.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/opd-ai/go-tor/pkg/client"
	"github.com/opd-ai/go-tor/pkg/config"
	"github.com/opd-ai/go-tor/pkg/logger"
)

func main() {
	fmt.Println("=== go-tor Phase 5 Integration Demo ===")
	fmt.Println("This demo shows the fully integrated Tor client with:")
	fmt.Println("  ✓ Directory client (fetch network consensus)")
	fmt.Println("  ✓ Path selection (guard, middle, exit)")
	fmt.Println("  ✓ Circuit building and management")
	fmt.Println("  ✓ SOCKS5 proxy server")
	fmt.Println("  ✓ Stream multiplexing")
	fmt.Println()

	// Create configuration
	cfg := config.DefaultConfig()
	cfg.SocksPort = 19050 // Use different port to avoid conflicts
	cfg.LogLevel = "info"

	// Initialize logger
	level, err := logger.ParseLevel(cfg.LogLevel)
	if err != nil {
		log.Fatalf("Invalid log level: %v", err)
	}
	logger := logger.New(level, os.Stdout)

	logger.Info("Starting integrated Tor client demo")

	// Create Tor client
	torClient, err := client.New(cfg, logger)
	if err != nil {
		log.Fatalf("Failed to create Tor client: %v", err)
	}

	// Create context with timeout for startup
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Start the client (this will take some time as it fetches consensus and builds circuits)
	logger.Info("Starting Tor client (this may take 30-60 seconds)...")
	if err := torClient.Start(ctx); err != nil {
		log.Fatalf("Failed to start Tor client: %v", err)
	}

	// Display stats
	stats := torClient.GetStats()
	fmt.Println()
	fmt.Println("=== Tor Client Started Successfully ===")
	fmt.Printf("Active Circuits: %d\n", stats.ActiveCircuits)
	fmt.Printf("SOCKS5 Proxy: 127.0.0.1:%d\n", stats.SocksPort)
	fmt.Println()
	fmt.Println("You can now configure applications to use the SOCKS5 proxy:")
	fmt.Printf("  curl --socks5 127.0.0.1:%d https://check.torproject.org\n", stats.SocksPort)
	fmt.Println()
	fmt.Println("Press Ctrl+C to stop the client...")
	fmt.Println()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println()
	logger.Info("Shutting down Tor client...")

	// Stop the client
	if err := torClient.Stop(); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}

	logger.Info("Tor client stopped successfully")
	fmt.Println()
	fmt.Println("=== Phase 5 Integration Complete ===")
}
