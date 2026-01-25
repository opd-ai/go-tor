// Package main demonstrates circuit rate limiting to protect against DoS.
//
// This example shows how to configure and use circuit rate limiting
// to control the rate at which circuits are created.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/opd-ai/go-tor/pkg/config"
	"github.com/opd-ai/go-tor/pkg/logger"
)

func main() {
	// Create configuration with circuit rate limiting enabled
	cfg := config.DefaultConfig()

	// Enable circuit rate limiting
	// Allow 10 circuit creations per second with a burst of 5
	cfg.CircuitCreationsPerSecond = 10.0
	cfg.CircuitCreationsBurst = 5

	// Set other required config
	cfg.DataDirectory = "/tmp/go-tor-ratelimit-demo"
	cfg.SocksPort = 9050
	cfg.ControlPort = 9051

	// Create logger
	logger := logger.NewDefault()

	fmt.Println("Circuit Rate Limiting Demo")
	fmt.Println("===========================")
	fmt.Printf("Rate: %.1f circuits/second\n", cfg.CircuitCreationsPerSecond)
	fmt.Printf("Burst: %d circuits\n", cfg.CircuitCreationsBurst)
	fmt.Println()

	// Note: In a real application, you would create a client with this config:
	//
	// client, err := client.New(cfg, logger)
	// if err != nil {
	//     log.Fatalf("Failed to create client: %v", err)
	// }
	// defer client.Close()
	//
	// err = client.Start(context.Background())
	// if err != nil {
	//     log.Fatalf("Failed to start client: %v", err)
	// }

	// Demonstrate the rate limiting behavior
	fmt.Println("Rate Limiting Behavior:")
	fmt.Println("-----------------------")
	fmt.Println()
	fmt.Println("Burst Phase:")
	fmt.Println("- First 5 circuit requests: Immediate (burst)")
	fmt.Println()
	fmt.Println("Rate Limited Phase:")
	fmt.Println("- Subsequent requests: Rate limited to 10/second")
	fmt.Println("- Each request waits ~100ms")
	fmt.Println()
	fmt.Println("Protection Against DoS:")
	fmt.Println("- Prevents resource exhaustion from excessive circuit creation")
	fmt.Println("- Ensures fair resource allocation")
	fmt.Println("- Metrics recorded for monitoring:")
	fmt.Println("  * RateLimitedCircuits: Count of rate-limited requests")
	fmt.Println("  * RateLimitWaitTime: Average wait time due to rate limiting")
	fmt.Println()

	// Simulate rate limiting with a simple demonstration
	fmt.Println("Simulation:")
	fmt.Println("-----------")

	rate := cfg.CircuitCreationsPerSecond
	burst := cfg.CircuitCreationsBurst

	// Calculate delays
	burstDuration := time.Duration(0) // Burst happens immediately
	intervalDuration := time.Second / time.Duration(rate)

	fmt.Printf("Burst of %d circuits: %v\n", burst, burstDuration)
	fmt.Printf("Rate-limited interval: %v per circuit\n", intervalDuration)
	fmt.Println()

	// Example timeline
	fmt.Println("Example Timeline:")
	currentTime := time.Duration(0)
	for i := 1; i <= 10; i++ {
		if i <= burst {
			fmt.Printf("Circuit %2d: %6s (burst)\n", i, currentTime)
		} else {
			currentTime += intervalDuration
			fmt.Printf("Circuit %2d: %6s (rate limited)\n", i, currentTime)
		}
	}

	fmt.Println()
	fmt.Println("Configuration Options:")
	fmt.Println("---------------------")
	fmt.Println("CircuitCreationsPerSecond: Maximum sustained rate (e.g., 10.0)")
	fmt.Println("CircuitCreationsBurst: Burst capacity (e.g., 5)")
	fmt.Println()
	fmt.Println("To disable rate limiting, set both values to 0 in config.")

	_ = logger
	_ = context.Background()
	_ = log.Print
}
