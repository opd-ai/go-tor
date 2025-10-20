// Package main demonstrates onion service hosting functionality
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/opd-ai/go-tor/pkg/logger"
	"github.com/opd-ai/go-tor/pkg/onion"
)

func main() {
	fmt.Println("=== Onion Service Hosting Demo ===")
	fmt.Println()

	// Create logger
	lg := logger.NewDefault()

	// Configure the onion service
	config := &onion.ServiceConfig{
		// If PrivateKey is nil, a new identity will be generated
		// To persist the same .onion address, save and reuse the key
		PrivateKey: nil,

		// Map virtual ports to local services
		// Format: virtual_port -> "host:port"
		Ports: map[int]string{
			80:  "localhost:8080", // HTTP
			443: "localhost:8443", // HTTPS
		},

		// Number of introduction points (default: 3, min: 1, max: 10)
		NumIntroPoints: 3,

		// How long descriptors remain valid (default: 3 hours)
		DescriptorLifetime: 3 * time.Hour,

		// Directory for persistent state (optional)
		DataDirectory: "/tmp/onion-service",
	}

	// Create the onion service
	fmt.Println("Creating onion service...")
	service, err := onion.NewService(config, lg)
	if err != nil {
		log.Fatalf("Failed to create service: %v", err)
	}

	// Get the onion address
	address := service.GetAddress()
	fmt.Printf("✓ Service created\n")
	fmt.Printf("✓ Onion address: %s\n", address)
	fmt.Println()

	// Create mock HSDirs (in production, these come from consensus)
	hsdirs := createMockHSDirs()
	fmt.Printf("Using %d HSDirs from consensus\n", len(hsdirs))
	fmt.Println()

	// Start the service
	ctx := context.Background()
	fmt.Println("Starting onion service...")
	fmt.Println("  1. Establishing introduction points...")
	fmt.Println("  2. Creating and signing descriptor...")
	fmt.Println("  3. Publishing descriptor to HSDirs...")

	if err := service.Start(ctx, hsdirs); err != nil {
		log.Fatalf("Failed to start service: %v", err)
	}

	fmt.Println()
	fmt.Println("✓ Onion service is now ONLINE")
	fmt.Println()

	// Display service statistics
	displayStats(service)

	// Display connection information
	fmt.Println("CONNECTION INFORMATION:")
	fmt.Println("─────────────────────────────────────────")
	fmt.Printf("Your service is accessible at:\n")
	fmt.Printf("  http://%s\n", address)
	fmt.Printf("  https://%s\n", address)
	fmt.Println()
	fmt.Println("Users can connect via Tor Browser or any Tor client")
	fmt.Println()

	// Display what's happening behind the scenes
	fmt.Println("BEHIND THE SCENES:")
	fmt.Println("─────────────────────────────────────────")
	fmt.Println("• Your service descriptor has been published to the Tor network")
	fmt.Println("• Introduction points are ready to relay connection requests")
	fmt.Println("• Descriptor will be automatically refreshed before expiration")
	fmt.Println("• All connections use end-to-end encryption")
	fmt.Println()

	// Handle graceful shutdown
	fmt.Println("Press Ctrl+C to stop the service...")
	fmt.Println()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Monitor service stats periodically
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-sigChan:
			fmt.Println()
			fmt.Println("Shutting down onion service...")

			if err := service.Stop(); err != nil {
				log.Printf("Error during shutdown: %v", err)
			}

			fmt.Println("✓ Service stopped")
			fmt.Println("Goodbye!")
			return

		case <-ticker.C:
			// Display updated stats
			fmt.Println()
			fmt.Println("SERVICE STATUS UPDATE:")
			displayStats(service)
		}
	}
}

func createMockHSDirs() []*onion.HSDirectory {
	// In a real implementation, these would come from the Tor consensus
	return []*onion.HSDirectory{
		{Fingerprint: "hsdir1", Address: "127.0.0.1", ORPort: 9001, DirPort: 9030, HSDir: true},
		{Fingerprint: "hsdir2", Address: "127.0.0.1", ORPort: 9002, DirPort: 9031, HSDir: true},
		{Fingerprint: "hsdir3", Address: "127.0.0.1", ORPort: 9003, DirPort: 9032, HSDir: true},
		{Fingerprint: "hsdir4", Address: "127.0.0.1", ORPort: 9004, DirPort: 9033, HSDir: true},
		{Fingerprint: "hsdir5", Address: "127.0.0.1", ORPort: 9005, DirPort: 9034, HSDir: true},
		{Fingerprint: "hsdir6", Address: "127.0.0.1", ORPort: 9006, DirPort: 9035, HSDir: true},
	}
}

func displayStats(service *onion.Service) {
	stats := service.GetStats()

	fmt.Println("─────────────────────────────────────────")
	fmt.Printf("Address:         %s\n", stats.Address)
	fmt.Printf("Status:          %s\n", formatStatus(stats.Running))
	fmt.Printf("Intro Points:    %d\n", stats.IntroPoints)
	fmt.Printf("Descriptor Age:  %s\n", formatDuration(stats.DescriptorAge))
	fmt.Printf("Pending Intros:  %d\n", stats.PendingIntros)
	fmt.Printf("HSDirs:          %d\n", stats.PublishedHSDirs)
	fmt.Println("─────────────────────────────────────────")
}

func formatStatus(running bool) string {
	if running {
		return "ONLINE ✓"
	}
	return "OFFLINE"
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh %dm", hours, minutes)
}
