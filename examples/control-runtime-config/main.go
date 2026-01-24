// Package main demonstrates runtime configuration updates via SETCONF
package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

func main() {
	// Connect to control port (default 9051)
	conn, err := net.Dial("tcp", "127.0.0.1:9051")
	if err != nil {
		fmt.Printf("Error connecting to control port: %v\n", err)
		fmt.Println("\nMake sure go-tor is running with control port enabled.")
		os.Exit(1)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)

	fmt.Println("=== Control Protocol: Runtime Configuration Updates ===")

	// Authenticate (assumes no password)
	if err := sendCommand(conn, reader, "AUTHENTICATE"); err != nil {
		fmt.Printf("Authentication failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ Authenticated")

	// Demonstrate runtime-updateable configuration options
	examples := []struct {
		name        string
		key         string
		value       string
		description string
	}{
		{
			name:        "Circuit Dirtiness",
			key:         "MaxCircuitDirtiness",
			value:       "15m",
			description: "How long circuits can be reused before rebuilding",
		},
		{
			name:        "New Circuit Period",
			key:         "NewCircuitPeriod",
			value:       "45s",
			description: "How often to build new circuits preemptively",
		},
		{
			name:        "Circuit Build Timeout",
			key:         "CircuitBuildTimeout",
			value:       "90s",
			description: "Maximum time allowed for circuit construction",
		},
		{
			name:        "Log Level",
			key:         "LogLevel",
			value:       "debug",
			description: "Logging verbosity (requires restart to affect logger)",
		},
	}

	for _, ex := range examples {
		fmt.Printf("Setting %s to %s\n", ex.name, ex.value)
		fmt.Printf("  Description: %s\n", ex.description)

		// Get current value
		currentValue, err := getConfig(conn, reader, ex.key)
		if err != nil {
			fmt.Printf("  ✗ Failed to get current value: %v\n\n", err)
			continue
		}
		fmt.Printf("  Current value: %s\n", currentValue)

		// Set new value
		if err := setConfig(conn, reader, ex.key, ex.value); err != nil {
			fmt.Printf("  ✗ Failed to set: %v\n\n", err)
			continue
		}

		// Verify new value
		newValue, err := getConfig(conn, reader, ex.key)
		if err != nil {
			fmt.Printf("  ✗ Failed to verify: %v\n\n", err)
			continue
		}
		fmt.Printf("  New value: %s\n", newValue)
		fmt.Println("  ✓ Updated successfully")

		time.Sleep(100 * time.Millisecond)
	}

	// Demonstrate read-only options
	fmt.Println("=== Attempting to Update Read-Only Options ===")
	readOnlyExamples := []string{"SocksPort", "ControlPort", "DataDirectory"}
	for _, key := range readOnlyExamples {
		fmt.Printf("Attempting to set %s...\n", key)
		if err := setConfig(conn, reader, key, "test-value"); err != nil {
			fmt.Printf("  Expected error: %v\n\n", err)
		} else {
			fmt.Println("  ✗ Unexpected success (should require restart)")
		}
	}

	// Demonstrate validation
	fmt.Println("=== Testing Configuration Validation ===")
	validationTests := []struct {
		key   string
		value string
		error string
	}{
		{"MaxCircuitDirtiness", "10s", "too short (minimum 30s)"},
		{"NewCircuitPeriod", "5s", "too short (minimum 10s)"},
		{"CircuitBuildTimeout", "10m", "too long (maximum 5m)"},
		{"LogLevel", "trace", "invalid log level"},
	}

	for _, test := range validationTests {
		fmt.Printf("Setting %s=%s (expecting: %s)\n", test.key, test.value, test.error)
		if err := setConfig(conn, reader, test.key, test.value); err != nil {
			fmt.Printf("  Expected error: %v\n\n", err)
		} else {
			fmt.Println("  ✗ Unexpected success (should be rejected)")
		}
	}

	fmt.Println("=== Demo Complete ===")
	fmt.Println("\nRuntime-updateable options:")
	fmt.Println("  • MaxCircuitDirtiness (≥30s)")
	fmt.Println("  • NewCircuitPeriod (≥10s)")
	fmt.Println("  • CircuitBuildTimeout (10s-5m)")
	fmt.Println("  • LogLevel (debug/info/warn/error)")
	fmt.Println("\nThese options can be updated without restarting the client.")
}

func sendCommand(conn net.Conn, reader *bufio.Reader, command string) error {
	_, err := fmt.Fprintf(conn, "%s\r\n", command)
	if err != nil {
		return err
	}

	response, err := reader.ReadString('\n')
	if err != nil {
		return err
	}

	// Check response code
	if !strings.HasPrefix(response, "250") {
		return fmt.Errorf("command failed: %s", strings.TrimSpace(response))
	}

	return nil
}

func getConfig(conn net.Conn, reader *bufio.Reader, key string) (string, error) {
	_, err := fmt.Fprintf(conn, "GETCONF %s\r\n", key)
	if err != nil {
		return "", err
	}

	response, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	// Parse response: "250-<key>=<value>" or "250 OK"
	response = strings.TrimSpace(response)
	if strings.HasPrefix(response, "250-") || strings.HasPrefix(response, "250 ") {
		parts := strings.SplitN(response, "=", 2)
		if len(parts) == 2 {
			return parts[1], nil
		}
		return "", nil
	}

	return "", fmt.Errorf("unexpected response: %s", response)
}

func setConfig(conn net.Conn, reader *bufio.Reader, key, value string) error {
	_, err := fmt.Fprintf(conn, "SETCONF %s=%s\r\n", key, value)
	if err != nil {
		return err
	}

	response, err := reader.ReadString('\n')
	if err != nil {
		return err
	}

	// Check response code
	if !strings.HasPrefix(response, "250") {
		return fmt.Errorf("%s", strings.TrimSpace(response))
	}

	return nil
}
