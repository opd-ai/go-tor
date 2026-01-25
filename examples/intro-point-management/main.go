// Example: Introduction Point Management
//
// This example demonstrates the introduction point protocol implementation
// including circuit retry logic, health monitoring, and automatic rotation.

package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/opd-ai/go-tor/pkg/logger"
	"github.com/opd-ai/go-tor/pkg/onion"
)

func main() {
	// Create a logger
	log := logger.New(slog.LevelInfo, os.Stdout)

	fmt.Println("=== Introduction Point Management Example ===")
	fmt.Println()

	// Example 1: Create a service with introduction point management
	fmt.Println("1. Creating onion service with intro point management...")
	service, err := createServiceWithIntroManagement(log)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Printf("   Service address: %s\n", service.GetAddress())
	fmt.Println()

	// Example 2: Demonstrate introduction point health tracking
	fmt.Println("2. Introduction point health tracking...")
	demonstrateHealthTracking(service)
	fmt.Println()

	// Example 3: Show retry logic
	fmt.Println("3. Circuit retry with exponential backoff...")
	demonstrateRetryLogic()
	fmt.Println()

	// Example 4: Monitor service stats
	fmt.Println("4. Service statistics...")
	stats := service.GetStats()
	fmt.Printf("   Running: %v\n", stats.Running)
	fmt.Printf("   Introduction points: %d\n", stats.IntroPoints)
	fmt.Printf("   Descriptor age: %v\n", stats.DescriptorAge)
	fmt.Println()

	fmt.Println("=== Example Complete ===")
}

func createServiceWithIntroManagement(log *logger.Logger) (*onion.Service, error) {
	// Configure the service
	config := &onion.ServiceConfig{
		NumIntroPoints:     3,             // Number of introduction points
		DescriptorLifetime: 3 * time.Hour, // Descriptor validity period
		// Note: In production, you would set CircuitBuilder and PathSelector
		// CircuitBuilder:     circuitBuilder,
		// PathSelector:       pathSelector,
	}

	// Create the service (automatically initializes IntroPointManager)
	service, err := onion.NewService(config, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create service: %w", err)
	}

	return service, nil
}

func demonstrateHealthTracking(service *onion.Service) {
	fmt.Println("   Health tracking features:")
	fmt.Println("   - Continuous monitoring every 30 seconds")
	fmt.Println("   - Automatic failure detection (3 consecutive failures)")
	fmt.Println("   - Staleness detection (>24 hours)")
	fmt.Println("   - Automatic rotation of unhealthy intro points")
}

func demonstrateRetryLogic() {
	// Show the backoff delays
	fmt.Println("   Retry delays:")
	for attempt := 0; attempt <= 3; attempt++ {
		delay := onion.CalculateBackoffDelay(attempt)
		fmt.Printf("   Attempt %d: %v delay\n", attempt+1, delay)
	}
}

// Example: Starting a service with full intro point management
func startServiceExample() {
	log := logger.NewDefault()

	// Create service
	config := &onion.ServiceConfig{
		NumIntroPoints:     3,
		DescriptorLifetime: 3 * time.Hour,
		Ports: map[int]string{
			80: "localhost:8080", // Map virtual port 80 to local port 8080
		},
	}

	service, err := onion.NewService(config, log)
	if err != nil {
		log.Error("Failed to create service", "error", err)
		return
	}

	// In production, you would provide actual HSDirs from the consensus
	var hsdirs []*onion.HSDirectory

	// Start the service
	// This will:
	// 1. Establish introduction points (with retry)
	// 2. Create and sign descriptor
	// 3. Publish descriptor to HSDirs
	// 4. Start health monitoring
	// 5. Start maintenance loop (rotation)
	ctx := context.Background()
	if err := service.Start(ctx, hsdirs); err != nil {
		log.Error("Failed to start service", "error", err)
		return
	}

	log.Info("Service started successfully",
		"address", service.GetAddress(),
		"intro_points", 3)

	// Service will automatically:
	// - Monitor intro point health every 30s
	// - Rotate unhealthy/stale intro points hourly
	// - Re-publish descriptor every hour

	// Wait for shutdown signal
	// service.Stop()
}
