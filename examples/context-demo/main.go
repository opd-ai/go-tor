// Package main demonstrates context-aware operations for improved timeout and cancellation control.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/opd-ai/go-tor/pkg/circuit"
	"github.com/opd-ai/go-tor/pkg/logger"
	"github.com/opd-ai/go-tor/pkg/stream"
)

func main() {
	fmt.Println("=== Context-Aware Operations Demo ===")
	fmt.Println()

	log := logger.NewDefault()

	// Demo 1: Circuit with timeout
	fmt.Println("--- Demo 1: Circuit State Management with Context ---")
	demonstrateCircuitContext(log)
	fmt.Println()

	// Demo 2: Stream operations with timeout
	fmt.Println("--- Demo 2: Stream Operations with Context ---")
	demonstrateStreamContext(log)
	fmt.Println()

	// Demo 3: Manager operations
	fmt.Println("--- Demo 3: Manager Operations with Context ---")
	demonstrateManagerContext()
	fmt.Println()

	// Demo 4: Cancellation handling
	fmt.Println("--- Demo 4: Graceful Cancellation ---")
	demonstrateCancellation(log)
	fmt.Println()

	fmt.Println("=== Demo Complete ===")
}

func demonstrateCircuitContext(log *logger.Logger) {
	// Create a circuit manager
	manager := circuit.NewManager()

	// Create a circuit with context
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	c, err := manager.CreateCircuitWithContext(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create circuit: %v\n", err)
		return
	}
	fmt.Printf("✓ Circuit created with ID: %d\n", c.ID)

	// Simulate circuit building in background
	go func() {
		time.Sleep(100 * time.Millisecond)
		c.SetState(circuit.StateOpen)
		fmt.Println("  Circuit state changed to OPEN")
	}()

	// Wait for circuit to become ready with timeout
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer waitCancel()

	fmt.Println("  Waiting for circuit to become ready...")
	if err := c.WaitUntilReady(waitCtx); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to wait for circuit: %v\n", err)
		return
	}
	fmt.Println("✓ Circuit is ready!")

	// Check circuit age
	if c.IsOlderThan(50 * time.Millisecond) {
		fmt.Printf("✓ Circuit is older than 50ms (age: %v)\n", c.Age())
	}

	// Close circuit with context
	closeCtx, closeCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer closeCancel()

	if err := manager.CloseCircuitWithContext(closeCtx, c.ID); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to close circuit: %v\n", err)
		return
	}
	fmt.Println("✓ Circuit closed gracefully")
}

func demonstrateStreamContext(log *logger.Logger) {
	// Create a stream
	s := stream.NewStream(1, 100, "example.com", 80, log)
	s.SetState(stream.StateConnecting)

	fmt.Printf("✓ Stream created (ID: %d, Circuit: %d)\n", s.ID, s.CircuitID)

	// Simulate connection in background
	go func() {
		time.Sleep(100 * time.Millisecond)
		s.SetState(stream.StateConnected)
		fmt.Println("  Stream state changed to CONNECTED")
	}()

	// Wait for stream to connect with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	fmt.Println("  Waiting for stream to connect...")
	if err := s.WaitForState(ctx, stream.StateConnected); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to wait for stream: %v\n", err)
		return
	}
	fmt.Println("✓ Stream connected!")

	// Send data with timeout
	data := []byte("Hello, Tor!")
	if err := s.SendWithTimeout(1*time.Second, data); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to send data: %v\n", err)
		return
	}
	fmt.Printf("✓ Sent %d bytes with timeout\n", len(data))

	// Check stream status
	if s.IsActive() {
		fmt.Println("✓ Stream is active")
	}

	// Close stream with context
	closeCtx, closeCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer closeCancel()

	if err := s.CloseWithContext(closeCtx); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to close stream: %v\n", err)
		return
	}
	fmt.Println("✓ Stream closed gracefully")

	if s.IsClosed() {
		fmt.Println("✓ Stream is closed")
	}
}

func demonstrateManagerContext() {
	manager := circuit.NewManager()

	// Create multiple circuits
	fmt.Println("Creating circuits...")
	for i := 0; i < 3; i++ {
		c, err := manager.CreateCircuit()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create circuit %d: %v\n", i, err)
			continue
		}
		// Simulate circuit becoming ready
		go func(c *circuit.Circuit) {
			time.Sleep(time.Duration(50+i*20) * time.Millisecond)
			c.SetState(circuit.StateOpen)
		}(c)
		fmt.Printf("  Created circuit %d\n", c.ID)
	}

	// Wait for specific number of circuits to be ready
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	fmt.Println("  Waiting for at least 2 circuits to be ready...")
	if err := manager.WaitForCircuitCount(ctx, circuit.StateOpen, 2); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to wait for circuits: %v\n", err)
		return
	}
	fmt.Println("✓ Required circuits are ready!")

	// Get circuits by state
	openCircuits := manager.GetCircuitsByState(circuit.StateOpen)
	fmt.Printf("✓ Found %d open circuits\n", len(openCircuits))

	// Count circuits by state
	count := manager.CountByState(circuit.StateOpen)
	fmt.Printf("✓ Circuit count by state (OPEN): %d\n", count)

	// Close manager with deadline
	if err := manager.CloseWithDeadline(1 * time.Second); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to close manager: %v\n", err)
		return
	}
	fmt.Println("✓ Manager closed gracefully")
}

func demonstrateCancellation(log *logger.Logger) {
	manager := circuit.NewManager()

	// Create a context that will be cancelled
	ctx, cancel := context.WithCancel(context.Background())

	// Start a long-running operation
	fmt.Println("Starting long-running operation...")
	go func() {
		time.Sleep(200 * time.Millisecond)
		fmt.Println("  Cancelling operation...")
		cancel()
	}()

	// Try to wait for circuits (will be cancelled)
	err := manager.WaitForCircuitCount(ctx, circuit.StateOpen, 5)
	if err != nil {
		if ctx.Err() == context.Canceled {
			fmt.Println("✓ Operation cancelled gracefully")
		} else {
			fmt.Fprintf(os.Stderr, "Unexpected error: %v\n", err)
		}
	}

	// Demonstrate timeout scenario
	timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer timeoutCancel()

	c, err := manager.CreateCircuit()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create circuit: %v\n", err)
		return
	}

	fmt.Println("  Waiting for circuit (will timeout)...")
	err = c.WaitUntilReady(timeoutCtx)
	if err != nil {
		if timeoutCtx.Err() == context.DeadlineExceeded {
			fmt.Println("✓ Timeout handled gracefully")
		} else {
			fmt.Fprintf(os.Stderr, "Unexpected error: %v\n", err)
		}
	}

	// Clean up
	_ = manager.CloseWithDeadline(1 * time.Second)
}
