// Package main demonstrates structured error handling.
package main

import (
	"fmt"
	"net"
	"time"

	"github.com/opd-ai/go-tor/pkg/errors"
)

func main() {
	fmt.Println("=== Structured Error Handling Demo ===")
	fmt.Println()

	// Demonstrate error creation
	fmt.Println("1. Creating different error types:")

	// Connection error (retryable)
	connErr := errors.ConnectionError("failed to connect to relay", net.ErrClosed)
	fmt.Printf("Connection Error: %v\n", connErr)
	fmt.Printf("  Category: %s\n", connErr.Category)
	fmt.Printf("  Severity: %s\n", connErr.Severity)
	fmt.Printf("  Retryable: %v\n\n", connErr.Retryable)

	// Circuit error (retryable)
	circErr := errors.CircuitError("circuit build timeout", nil)
	fmt.Printf("Circuit Error: %v\n", circErr)
	fmt.Printf("  Category: %s\n", circErr.Category)
	fmt.Printf("  Severity: %s\n", circErr.Severity)
	fmt.Printf("  Retryable: %v\n\n", circErr.Retryable)

	// Protocol error (not retryable)
	protoErr := errors.ProtocolError("invalid cell format", nil)
	fmt.Printf("Protocol Error: %v\n", protoErr)
	fmt.Printf("  Category: %s\n", protoErr.Category)
	fmt.Printf("  Severity: %s\n", protoErr.Severity)
	fmt.Printf("  Retryable: %v\n\n", protoErr.Retryable)

	// Demonstrate error with context
	fmt.Println("2. Error with context:")
	timeoutErr := errors.TimeoutError("connection timeout", nil).
		WithContext("address", "127.0.0.1:9050").
		WithContext("attempt", 3).
		WithContext("timeout", 30*time.Second)
	fmt.Printf("Error: %v\n", timeoutErr)
	fmt.Printf("  Context:\n")
	for key, value := range timeoutErr.Context {
		fmt.Printf("    %s: %v\n", key, value)
	}
	fmt.Println()

	// Demonstrate error checking and handling
	fmt.Println("3. Error classification and handling:")

	testErrors := []error{
		errors.ConnectionError("connection failed", nil),
		errors.CircuitError("circuit failed", nil),
		errors.ProtocolError("protocol error", nil),
		errors.ConfigurationError("invalid config", nil),
		errors.TimeoutError("timeout", nil),
	}

	for _, err := range testErrors {
		fmt.Printf("Error: %v\n", err)
		fmt.Printf("  Retryable: %v\n", errors.IsRetryable(err))
		fmt.Printf("  Category: %s\n", errors.GetCategory(err))
		fmt.Printf("  Severity: %s\n", errors.GetSeverity(err))

		// Decision logic based on error properties
		if errors.IsRetryable(err) {
			fmt.Println("  -> Action: Retry operation with exponential backoff")
		} else {
			fmt.Println("  -> Action: Fail fast, report error to user")
		}

		if errors.GetSeverity(err) == errors.SeverityCritical {
			fmt.Println("  -> Alert: Critical error - immediate attention required")
		}
		fmt.Println()
	}

	// Demonstrate error comparison
	fmt.Println("4. Error comparison with errors.Is:")
	err1 := errors.ConnectionError("test1", nil)
	err3 := errors.CircuitError("test3", nil)

	fmt.Printf("ConnectionError vs ConnectionError: %v\n", errors.IsCategory(err1, errors.CategoryConnection))
	fmt.Printf("ConnectionError vs CircuitError: %v\n", errors.IsCategory(err1, errors.CategoryCircuit))
	fmt.Printf("CircuitError vs CircuitError: %v\n", errors.IsCategory(err3, errors.CategoryCircuit))
	fmt.Println()

	// Demonstrate retry logic
	fmt.Println("5. Simulated retry logic:")
	simulateRetryLogic()
}

func simulateRetryLogic() {
	attempts := []error{
		errors.ConnectionError("attempt 1 failed", nil),
		errors.TimeoutError("attempt 2 timeout", nil),
		nil, // Success on attempt 3
	}

	for i, err := range attempts {
		if err == nil {
			fmt.Printf("Attempt %d: Success!\n", i+1)
			break
		}

		fmt.Printf("Attempt %d: %v\n", i+1, err)
		if errors.IsRetryable(err) {
			// Cap maximum backoff to prevent integer overflow
			// Max power of 10 gives us 1024 seconds (~17 minutes)
			const maxBackoffPower = 10
			backoffPower := uint(i)
			if backoffPower > maxBackoffPower {
				backoffPower = maxBackoffPower
			}
			backoff := time.Duration(1<<backoffPower) * time.Second
			fmt.Printf("  -> Retrying after %v backoff\n", backoff)
		} else {
			fmt.Printf("  -> Error not retryable, aborting\n")
			break
		}
	}
}
