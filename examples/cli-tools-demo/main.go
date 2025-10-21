// Package main demonstrates the CLI tools for go-tor
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/opd-ai/go-tor/pkg/client"
)

func main() {
	fmt.Println("=== go-tor CLI Tools Demo ===")
	fmt.Println()
	fmt.Println("This example demonstrates the CLI tools available for go-tor:")
	fmt.Println("1. torctl - Control utility for running Tor clients")
	fmt.Println("2. tor-config-validator - Configuration validation and generation")
	fmt.Println()

	// Demonstrate config validator
	fmt.Println("--- Configuration Validator Demo ---")
	fmt.Println()

	// Generate a sample config
	fmt.Println("Generating sample configuration...")
	cmd := exec.Command("../../bin/tor-config-validator", "-generate", "-output", "/tmp/sample-torrc")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Warning: tor-config-validator not found or failed: %v", err)
		log.Printf("Build it with: make build-config-validator")
	} else {
		fmt.Println(string(output))
	}

	// Validate the generated config
	if _, err := os.Stat("/tmp/sample-torrc"); err == nil {
		fmt.Println("Validating generated configuration...")
		cmd = exec.Command("../../bin/tor-config-validator", "-config", "/tmp/sample-torrc", "-verbose")
		output, err = cmd.CombinedOutput()
		if err != nil {
			log.Printf("Validation failed: %v", err)
		} else {
			fmt.Println(string(output))
		}
		fmt.Println()
	}

	// Demonstrate torctl with a running client
	fmt.Println("--- torctl Demo ---")
	fmt.Println()
	fmt.Println("Starting Tor client for demonstration...")

	// Start a Tor client
	torClient, err := client.Connect()
	if err != nil {
		log.Fatalf("Failed to start Tor client: %v", err)
	}
	defer torClient.Close()

	fmt.Println("Tor client started successfully")
	fmt.Println("Waiting for circuits to be established...")

	// Wait for client to be ready
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	ready := false
	for !ready {
		select {
		case <-ctx.Done():
			log.Println("Timeout waiting for circuits, continuing with demo...")
			ready = true
		case <-ticker.C:
			if torClient.IsReady() {
				fmt.Println("Circuits established!")
				ready = true
			} else {
				fmt.Println("Still building circuits...")
			}
		}
	}

	fmt.Println()
	fmt.Println("You can now use torctl to interact with the client:")
	fmt.Println()

	// Demonstrate torctl commands
	commands := []struct {
		name string
		args []string
	}{
		{"Status", []string{"../../bin/torctl", "status"}},
		{"Circuits", []string{"../../bin/torctl", "circuits"}},
		{"Info", []string{"../../bin/torctl", "info"}},
	}

	for _, cmdInfo := range commands {
		fmt.Printf("--- Running: torctl %s ---\n", cmdInfo.name)
		cmd = exec.Command(cmdInfo.args[0], cmdInfo.args[1:]...)
		output, err = cmd.CombinedOutput()
		if err != nil {
			log.Printf("Command failed: %v", err)
			log.Printf("Make sure torctl is built with: make build-torctl")
		} else {
			fmt.Println(string(output))
		}
		fmt.Println()
		time.Sleep(1 * time.Second)
	}

	fmt.Println("=== Demo Complete ===")
	fmt.Println()
	fmt.Println("Available CLI tools:")
	fmt.Println("  torctl               - Control running Tor clients")
	fmt.Println("  tor-config-validator - Validate and generate configurations")
	fmt.Println()
	fmt.Println("Build all tools with: make build-tools")
}
