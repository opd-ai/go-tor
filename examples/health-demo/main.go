// Package main demonstrates the health monitoring system.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/opd-ai/go-tor/pkg/health"
)

func main() {
	fmt.Println("=== Health Monitoring System Demo ===")
	fmt.Println()

	// Create a health monitor
	monitor := health.NewMonitor()

	// Register circuit health checker
	circuitChecker := health.NewCircuitHealthChecker(func() health.CircuitStats {
		return health.CircuitStats{
			ActiveCircuits: 3,
			MinRequired:    2,
			FailedBuilds:   1,
			AverageAge:     5 * time.Minute,
			MaxAge:         8 * time.Minute,
		}
	})
	monitor.RegisterChecker(circuitChecker)

	// Register connection health checker
	connChecker := health.NewConnectionHealthChecker(func() health.ConnectionStats {
		return health.ConnectionStats{
			TotalConnections:   10,
			OpenConnections:    8,
			FailedConnections:  2,
			AverageLatency:     100 * time.Millisecond,
			ConnectionAttempts: 10,
		}
	})
	monitor.RegisterChecker(connChecker)

	// Register directory health checker
	dirChecker := health.NewDirectoryHealthChecker(func() health.DirectoryStats {
		return health.DirectoryStats{
			LastConsensusUpdate: time.Now().Add(-30 * time.Minute),
			ConsensusAge:        30 * time.Minute,
			RelayCount:          1000,
			GuardCount:          100,
			ExitCount:           200,
		}
	})
	monitor.RegisterChecker(dirChecker)

	// Perform health check
	ctx := context.Background()
	result := monitor.Check(ctx)

	// Display results
	fmt.Printf("Overall Health Status: %s\n", result.Status)
	fmt.Printf("Timestamp: %s\n", result.Timestamp.Format(time.RFC3339))
	fmt.Printf("Uptime: %s\n\n", result.Uptime)

	fmt.Println("Component Health:")
	for name, component := range result.Components {
		fmt.Printf("  %s:\n", name)
		fmt.Printf("    Status: %s\n", component.Status)
		fmt.Printf("    Message: %s\n", component.Message)
		fmt.Printf("    Response Time: %dms\n", component.ResponseTimeMs)
		if len(component.Details) > 0 {
			fmt.Printf("    Details:\n")
			for key, value := range component.Details {
				fmt.Printf("      %s: %v\n", key, value)
			}
		}
		fmt.Println()
	}

	// Demonstrate JSON serialization for APIs
	fmt.Println("\n=== JSON Output (for API endpoints) ===")
	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(jsonData))

	// Demonstrate degraded state
	fmt.Println("\n=== Degraded State Example ===")
	degradedCircuitChecker := health.NewCircuitHealthChecker(func() health.CircuitStats {
		return health.CircuitStats{
			ActiveCircuits: 1, // Below minimum
			MinRequired:    2,
			FailedBuilds:   3,
			AverageAge:     5 * time.Minute,
			MaxAge:         8 * time.Minute,
		}
	})

	monitor2 := health.NewMonitor()
	monitor2.RegisterChecker(degradedCircuitChecker)
	result2 := monitor2.Check(ctx)

	fmt.Printf("Overall Status: %s\n", result2.Status)
	fmt.Printf("Circuits Status: %s\n", result2.Components["circuits"].Status)
	fmt.Printf("Message: %s\n", result2.Components["circuits"].Message)

	// Demonstrate unhealthy state
	fmt.Println("\n=== Unhealthy State Example ===")
	unhealthyConnChecker := health.NewConnectionHealthChecker(func() health.ConnectionStats {
		return health.ConnectionStats{
			TotalConnections:   5,
			OpenConnections:    0, // No connections
			FailedConnections:  5,
			AverageLatency:     0,
			ConnectionAttempts: 5,
		}
	})

	monitor3 := health.NewMonitor()
	monitor3.RegisterChecker(unhealthyConnChecker)
	result3 := monitor3.Check(ctx)

	fmt.Printf("Overall Status: %s\n", result3.Status)
	fmt.Printf("Connections Status: %s\n", result3.Components["connections"].Status)
	fmt.Printf("Message: %s\n", result3.Components["connections"].Message)
}
