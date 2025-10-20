// Package main demonstrates using go-tor with HTTP metrics endpoint.
// This example shows how to enable and access metrics for monitoring.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/opd-ai/go-tor/pkg/client"
	"github.com/opd-ai/go-tor/pkg/config"
	"github.com/opd-ai/go-tor/pkg/logger"
	"github.com/opd-ai/go-tor/pkg/metrics"
)

func main() {
	fmt.Println("go-tor HTTP Metrics Demo")
	fmt.Println("========================")
	fmt.Println()

	// Create configuration with metrics enabled
	cfg := config.DefaultConfig()
	cfg.MetricsPort = 9052       // HTTP metrics on port 9052
	cfg.EnableMetrics = true     // Enable metrics endpoint
	cfg.LogLevel = "info"

	// Initialize logger
	logLevel, _ := logger.ParseLevel(cfg.LogLevel)
	appLogger := logger.New(logLevel, os.Stdout)

	// Create Tor client
	torClient, err := client.New(cfg, appLogger)
	if err != nil {
		log.Fatalf("Failed to create Tor client: %v", err)
	}

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the client
	fmt.Println("Starting Tor client with metrics enabled...")
	if err := torClient.Start(ctx); err != nil {
		log.Fatalf("Failed to start Tor client: %v", err)
	}

	stats := torClient.GetStats()
	fmt.Println()
	fmt.Printf("✅ Tor client started successfully!\n")
	fmt.Printf("   SOCKS5 Proxy:  socks5://127.0.0.1:%d\n", stats.SocksPort)
	fmt.Printf("   Control Port:  127.0.0.1:%d\n", stats.ControlPort)
	fmt.Printf("   Metrics HTTP:  http://127.0.0.1:%d/\n", cfg.MetricsPort)
	fmt.Println()

	// Demonstrate metrics endpoints
	fmt.Println("Available metrics endpoints:")
	fmt.Printf("  - Dashboard:   http://127.0.0.1:%d/debug/metrics\n", cfg.MetricsPort)
	fmt.Printf("  - Prometheus:  http://127.0.0.1:%d/metrics\n", cfg.MetricsPort)
	fmt.Printf("  - JSON:        http://127.0.0.1:%d/metrics/json\n", cfg.MetricsPort)
	fmt.Printf("  - Health:      http://127.0.0.1:%d/health\n", cfg.MetricsPort)
	fmt.Println()

	// Give the client a moment to stabilize
	time.Sleep(2 * time.Second)

	// Fetch and display current metrics
	go displayMetricsPeriodically(cfg.MetricsPort)

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("Press Ctrl+C to exit")
	fmt.Println()

	// Wait for shutdown signal
	<-sigChan

	fmt.Println("\nShutting down...")
	if err := torClient.Stop(); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}

	fmt.Println("Shutdown complete")
}

// displayMetricsPeriodically fetches and displays metrics every 10 seconds
func displayMetricsPeriodically(port int) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		displayMetrics(port)
	}
}

// displayMetrics fetches and displays current metrics
func displayMetrics(port int) {
	url := fmt.Sprintf("http://127.0.0.1:%d/metrics/json", port)
	
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("Failed to fetch metrics: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Metrics endpoint returned status %d", resp.StatusCode)
		return
	}

	var snapshot metrics.Snapshot
	if err := json.NewDecoder(resp.Body).Decode(&snapshot); err != nil {
		log.Printf("Failed to decode metrics: %v", err)
		return
	}

	// Display key metrics
	fmt.Println("─────────────────────────────────────────────")
	fmt.Printf("Current Metrics (Uptime: %ds)\n", snapshot.UptimeSeconds)
	fmt.Println("─────────────────────────────────────────────")
	fmt.Printf("Circuits:     %d active, %d built (%d success, %d failed)\n",
		snapshot.ActiveCircuits,
		snapshot.CircuitBuilds,
		snapshot.CircuitBuildSuccess,
		snapshot.CircuitBuildFailure)
	fmt.Printf("Connections:  %d active, %d total (%d success, %d failed)\n",
		snapshot.ActiveConnections,
		snapshot.ConnectionAttempts,
		snapshot.ConnectionSuccess,
		snapshot.ConnectionFailures)
	fmt.Printf("Streams:      %d active, %d created, %d closed\n",
		snapshot.ActiveStreams,
		snapshot.StreamsCreated,
		snapshot.StreamsClosed)
	fmt.Printf("Guards:       %d active, %d confirmed\n",
		snapshot.GuardsActive,
		snapshot.GuardsConfirmed)
	fmt.Printf("SOCKS:        %d connections, %d requests\n",
		snapshot.SocksConnections,
		snapshot.SocksRequests)
	
	if snapshot.CircuitBuilds > 0 {
		fmt.Printf("Build Time:   avg %.2fs, p95 %.2fs\n",
			snapshot.CircuitBuildTimeAvg.Seconds(),
			snapshot.CircuitBuildTimeP95.Seconds())
	}
	
	fmt.Println("─────────────────────────────────────────────")
	fmt.Println()
}
