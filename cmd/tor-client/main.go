// Package main provides the Tor client executable.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/opd-ai/go-tor/pkg/client"
	"github.com/opd-ai/go-tor/pkg/config"
	"github.com/opd-ai/go-tor/pkg/logger"
)

var (
	version   = "0.1.0-dev"
	buildTime = "unknown"
)

func main() {
	// Parse command-line flags
	configFile := flag.String("config", "", "Path to configuration file (torrc format)")
	socksPort := flag.Int("socks-port", 0, "SOCKS5 proxy port (default: auto-detect or 9050)")
	controlPort := flag.Int("control-port", 0, "Control protocol port (default: 9051)")
	dataDir := flag.String("data-dir", "", "Data directory for persistent state (default: auto-detect)")
	logLevel := flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	if *showVersion {
		fmt.Printf("go-tor version %s (built %s)\n", version, buildTime)
		fmt.Println("Pure Go Tor client implementation")
		os.Exit(0)
	}

	// Load or create configuration
	var cfg *config.Config
	if *configFile != "" {
		// Load from config file
		cfg = config.DefaultConfig()
		if err := config.LoadFromFile(*configFile, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to load config file: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Use zero-configuration defaults
		cfg = config.DefaultConfig()
		fmt.Printf("[INFO] Using zero-configuration mode\n")
		fmt.Printf("[INFO] Data directory: %s\n", cfg.DataDirectory)
	}

	// Apply command-line overrides (command-line flags take precedence)
	if *socksPort != 0 {
		cfg.SocksPort = *socksPort
	}
	if *controlPort != 0 {
		cfg.ControlPort = *controlPort
	}
	if *dataDir != "" {
		cfg.DataDirectory = *dataDir
	}
	if *logLevel != "" {
		cfg.LogLevel = *logLevel
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize structured logger
	level, err := logger.ParseLevel(cfg.LogLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid log level: %v\n", err)
		os.Exit(1)
	}
	log := logger.New(level, os.Stdout)

	log.Info("Starting go-tor",
		"version", version,
		"build_time", buildTime)
	log.Info("Configuration loaded",
		"socks_port", cfg.SocksPort,
		"control_port", cfg.ControlPort,
		"data_directory", cfg.DataDirectory,
		"log_level", cfg.LogLevel)

	// Create root context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Attach logger to context
	ctx = logger.WithContext(ctx, log)

	// Run the application
	if err := run(ctx, cfg, log); err != nil {
		log.Error("Application error", "error", err)
		os.Exit(1)
	}

	log.Info("Shutdown complete")
}

// run contains the main application logic
func run(ctx context.Context, cfg *config.Config, log *logger.Logger) error {
	// Display initialization message
	log.Info("Initializing Tor client...")
	
	// Initialize Tor client
	torClient, err := client.New(cfg, log)
	if err != nil {
		return fmt.Errorf("failed to create Tor client: %w", err)
	}

	// Display bootstrapping message
	log.Info("Bootstrapping Tor network connection...")
	log.Info("This may take 30-60 seconds on first run")
	
	// Start the client
	startTime := time.Now()
	if err := torClient.Start(ctx); err != nil {
		return fmt.Errorf("failed to start Tor client: %w", err)
	}
	bootstrapDuration := time.Since(startTime)

	// Display success status
	stats := torClient.GetStats()
	log.Info("✓ Connected to Tor network",
		"bootstrap_time", bootstrapDuration.Round(time.Second),
		"active_circuits", stats.ActiveCircuits)
	log.Info("✓ SOCKS proxy available",
		"address", fmt.Sprintf("127.0.0.1:%d", stats.SocksPort),
		"url", fmt.Sprintf("socks5://127.0.0.1:%d", stats.SocksPort))
	log.Info("Configure your application to use the SOCKS5 proxy for anonymous connections")
	
	// Example usage instructions
	fmt.Println()
	fmt.Println("Example: Test with curl")
	fmt.Printf("  curl --socks5 127.0.0.1:%d https://check.torproject.org\n", stats.SocksPort)
	fmt.Println()

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	log.Info("Press Ctrl+C to exit")

	// Wait for shutdown signal or context cancellation
	select {
	case sig := <-sigChan:
		log.Info("Received shutdown signal", "signal", sig.String())
	case <-ctx.Done():
		log.Info("Context cancelled", "reason", ctx.Err())
	}

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	log.Info("Initiating graceful shutdown...")

	// Stop the client
	if err := torClient.Stop(); err != nil {
		log.Warn("Error during shutdown", "error", err)
	}

	select {
	case <-shutdownCtx.Done():
		log.Warn("Shutdown timeout exceeded, forcing exit")
		return shutdownCtx.Err()
	default:
		// Shutdown completed successfully
	}

	return nil
}
