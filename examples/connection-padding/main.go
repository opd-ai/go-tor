// Package main demonstrates connection-level padding usage.
// This example shows how to enable padding on a Tor relay connection
// to resist traffic analysis attacks at the link level.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/opd-ai/go-tor/pkg/connection"
	"github.com/opd-ai/go-tor/pkg/logger"
)

func main() {
	// Create a logger
	logger := logger.NewDefault()

	// Configure the connection
	cfg := connection.DefaultConfig("127.0.0.1:9001")
	cfg.Timeout = 30 * time.Second

	// Create and connect to relay
	conn := connection.New(cfg, logger)
	ctx := context.Background()

	if err := conn.Connect(ctx, cfg); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	fmt.Println("Connected to relay")

	// Example 1: Use default padding configuration
	paddingMachine1, err := connection.NewConnectionPaddingMachine(conn, nil)
	if err != nil {
		log.Fatalf("Failed to create padding machine: %v", err)
	}

	if err := paddingMachine1.Start(ctx); err != nil {
		log.Fatalf("Failed to start padding: %v", err)
	}
	defer paddingMachine1.Stop()

	fmt.Println("Started connection-level padding with default config")

	// Example 2: Custom padding configuration for high security
	highSecurityConfig := &connection.ConnectionPaddingConfig{
		Strategy:          connection.ConnectionPaddingRandom,
		MinInterval:       2 * time.Second, // More frequent padding
		MaxInterval:       5 * time.Second, // Shorter max interval
		IdleTimeout:       500 * time.Millisecond,
		UseVariableLength: true, // Use VPADDING for better resistance
	}

	paddingMachine2, err := connection.NewConnectionPaddingMachine(conn, highSecurityConfig)
	if err != nil {
		log.Fatalf("Failed to create high-security padding machine: %v", err)
	}
	_ = paddingMachine2 // Could be used for a separate connection

	fmt.Println("Created high-security padding configuration")

	// Example 3: Adaptive padding that adjusts based on activity
	adaptiveConfig := &connection.ConnectionPaddingConfig{
		Strategy:          connection.ConnectionPaddingAdaptive,
		MinInterval:       3 * time.Second,
		MaxInterval:       10 * time.Second,
		IdleTimeout:       1 * time.Second,
		UseVariableLength: false,
	}

	adaptivePadding, err := connection.NewConnectionPaddingMachine(conn, adaptiveConfig)
	if err != nil {
		log.Fatalf("Failed to create adaptive padding machine: %v", err)
	}

	if err := adaptivePadding.Start(ctx); err != nil {
		log.Fatalf("Failed to start adaptive padding: %v", err)
	}
	defer adaptivePadding.Stop()

	fmt.Println("Started adaptive padding")

	// Simulate connection activity
	fmt.Println("\nSimulating connection activity...")
	for i := 0; i < 5; i++ {
		time.Sleep(2 * time.Second)

		// Record activity when sending/receiving cells
		adaptivePadding.RecordActivity()
		fmt.Printf("Activity recorded at %s\n", time.Now().Format("15:04:05"))
	}

	// Check padding statistics
	stats := adaptivePadding.Stats()
	fmt.Printf("\nPadding Statistics:\n")
	fmt.Printf("  PADDING cells sent: %d\n", stats.PaddingsSent)
	fmt.Printf("  VPADDING cells sent: %d\n", stats.VPaddingsSent)
	fmt.Printf("  Failed attempts: %d\n", stats.FailedPaddings)

	// Example 4: Update configuration dynamically
	fmt.Println("\nUpdating to fixed-interval padding...")
	newConfig := &connection.ConnectionPaddingConfig{
		Strategy:    connection.ConnectionPaddingFixed,
		MinInterval: 3 * time.Second,
		IdleTimeout: 1 * time.Second,
	}

	if err := adaptivePadding.UpdateConfig(newConfig); err != nil {
		log.Fatalf("Failed to update config: %v", err)
	}

	fmt.Println("Configuration updated successfully")

	// Let padding run for a bit
	time.Sleep(10 * time.Second)

	// Final statistics
	finalStats := adaptivePadding.Stats()
	fmt.Printf("\nFinal Statistics:\n")
	fmt.Printf("  PADDING cells sent: %d\n", finalStats.PaddingsSent)
	fmt.Printf("  VPADDING cells sent: %d\n", finalStats.VPaddingsSent)
	fmt.Printf("  Failed attempts: %d\n", finalStats.FailedPaddings)
}
