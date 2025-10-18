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
	socksPort := flag.Int("socks-port", 9050, "SOCKS5 proxy port")
	controlPort := flag.Int("control-port", 9051, "Control protocol port")
	dataDir := flag.String("data-dir", "/var/lib/tor", "Data directory for persistent state")
	logLevel := flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	if *showVersion {
		fmt.Printf("go-tor version %s (built %s)\n", version, buildTime)
		fmt.Println("Pure Go Tor client implementation")
		os.Exit(0)
	}

	// Load or create configuration
	cfg := config.DefaultConfig()

	// Apply command-line overrides
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

	// TODO: Load from config file if specified
	if *configFile != "" {
		log.Warn("Configuration file support not yet implemented", "path", *configFile)
	}

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
	// Initialize Tor client
	torClient, err := client.New(cfg, log)
	if err != nil {
		return fmt.Errorf("failed to create Tor client: %w", err)
	}

	// Start the client
	if err := torClient.Start(ctx); err != nil {
		return fmt.Errorf("failed to start Tor client: %w", err)
	}

	// Display status
	stats := torClient.GetStats()
	log.Info("Tor client running",
		"active_circuits", stats.ActiveCircuits,
		"socks_port", stats.SocksPort)
	log.Info("SOCKS5 proxy available at", "address", fmt.Sprintf("127.0.0.1:%d", stats.SocksPort))
	log.Info("Configure your application to use SOCKS5 proxy for anonymous connections")

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
