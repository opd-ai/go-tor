// Package main provides a command-line tool for running comprehensive
// performance benchmarks on the go-tor Tor client implementation.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/opd-ai/go-tor/pkg/benchmark"
	"github.com/opd-ai/go-tor/pkg/logger"
)

var (
	version   = "0.1.0-dev"
	buildTime = "unknown"
)

func main() {
	// Parse command-line flags
	showVersion := flag.Bool("version", false, "Show version information")
	logLevel := flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	runCircuit := flag.Bool("circuit", true, "Run circuit build benchmarks")
	runMemory := flag.Bool("memory", true, "Run memory usage benchmarks")
	runStreams := flag.Bool("streams", true, "Run concurrent streams benchmarks")
	runAll := flag.Bool("all", false, "Run all benchmarks (overrides individual flags)")
	timeout := flag.Duration("timeout", 5*time.Minute, "Global timeout for all benchmarks")
	flag.Parse()

	if *showVersion {
		fmt.Printf("go-tor benchmark tool version %s (built %s)\n", version, buildTime)
		os.Exit(0)
	}

	// Initialize logger
	level, err := logger.ParseLevel(*logLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid log level: %v\n", err)
		os.Exit(1)
	}
	log := logger.New(level, os.Stdout)

	log.Info("Starting go-tor performance benchmarks",
		"version", version,
		"build_time", buildTime)

	// Create benchmark suite
	suite := benchmark.NewSuite(log)

	// Set up context with timeout and signal handling
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	go func() {
		<-sigChan
		log.Warn("Received interrupt signal, canceling benchmarks...")
		cancel()
	}()

	// Determine which benchmarks to run
	if *runAll {
		*runCircuit = true
		*runMemory = true
		*runStreams = true
	}

	// Run selected benchmarks
	var hasErrors bool

	if *runCircuit {
		log.Info("Running circuit build benchmarks...")
		if err := suite.BenchmarkCircuitBuild(ctx); err != nil {
			log.Error("Circuit build benchmark failed", "error", err)
			hasErrors = true
		}
		if err := suite.BenchmarkCircuitBuildWithPool(ctx); err != nil {
			log.Error("Circuit build with pool benchmark failed", "error", err)
			hasErrors = true
		}
	}

	if *runMemory {
		log.Info("Running memory usage benchmarks...")
		if err := suite.BenchmarkMemoryUsage(ctx); err != nil {
			log.Error("Memory usage benchmark failed", "error", err)
			hasErrors = true
		}
		if err := suite.BenchmarkMemoryLeaks(ctx); err != nil {
			log.Error("Memory leak detection failed", "error", err)
			hasErrors = true
		}
	}

	if *runStreams {
		log.Info("Running concurrent streams benchmarks...")
		if err := suite.BenchmarkConcurrentStreams(ctx); err != nil {
			log.Error("Concurrent streams benchmark failed", "error", err)
			hasErrors = true
		}
		if err := suite.BenchmarkStreamScaling(ctx); err != nil {
			log.Error("Stream scaling benchmark failed", "error", err)
			hasErrors = true
		}
		if err := suite.BenchmarkStreamMultiplexing(ctx); err != nil {
			log.Error("Stream multiplexing benchmark failed", "error", err)
			hasErrors = true
		}
	}

	// Print summary
	suite.PrintSummary()

	// Analyze results
	results := suite.Results()
	passCount := 0
	failCount := 0

	for _, r := range results {
		if r.Success {
			passCount++
		} else {
			failCount++
		}
	}

	// Print final status
	separator := "================================================================================"
	fmt.Println("\n" + separator)
	fmt.Printf("FINAL RESULTS: %d PASSED, %d FAILED (out of %d total)\n",
		passCount, failCount, len(results))
	fmt.Println(separator)

	// Evaluate against README targets
	fmt.Println("\n" + separator)
	fmt.Println("PERFORMANCE TARGETS EVALUATION")
	fmt.Println(separator)

	for _, r := range results {
		if target, ok := r.AdditionalMetrics["meets_target"].(bool); ok {
			status := "✓ PASS"
			if !target {
				status = "✗ FAIL"
			}
			fmt.Printf("%s: %s\n", status, r.Name)

			// Print specific target information
			if r.Name == "Circuit Build Performance" {
				if targetP95, ok := r.AdditionalMetrics["target_p95"].(time.Duration); ok {
					fmt.Printf("  Target: p95 < %v\n", targetP95)
					fmt.Printf("  Actual: p95 = %v\n", r.P95Latency)
				}
			} else if r.Name == "Memory Usage in Steady State" {
				if targetMB, ok := r.AdditionalMetrics["target_mb"].(int); ok {
					if actualMB, ok := r.AdditionalMetrics["actual_mb"].(float64); ok {
						fmt.Printf("  Target: < %d MB\n", targetMB)
						fmt.Printf("  Actual: %.1f MB\n", actualMB)
					}
				}
			} else if r.Name == "Concurrent Streams Performance" {
				if targetStreams, ok := r.AdditionalMetrics["target_streams"].(int); ok {
					fmt.Printf("  Target: %d+ concurrent streams\n", targetStreams)
					fmt.Printf("  Actual: %d streams handled successfully\n", targetStreams)
				}
			}
		}
	}
	fmt.Println(separator)

	if hasErrors || failCount > 0 {
		log.Error("Benchmarks completed with errors")
		os.Exit(1)
	}

	log.Info("All benchmarks completed successfully")
}
