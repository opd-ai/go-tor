// Performance optimization demonstration for go-tor Phase 8.3
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/opd-ai/go-tor/pkg/circuit"
	"github.com/opd-ai/go-tor/pkg/config"
	"github.com/opd-ai/go-tor/pkg/logger"
	"github.com/opd-ai/go-tor/pkg/pool"
)

func main() {
	fmt.Println("=== Performance Optimization Demo (Phase 8.3) ===")
	fmt.Println()

	// Demonstrate buffer pooling
	demonstrateBufferPooling()
	fmt.Println()

	// Demonstrate circuit pooling
	demonstrateCircuitPooling()
	fmt.Println()

	// Demonstrate configuration options
	demonstrateConfiguration()
	fmt.Println()

	fmt.Println("=== Demo Complete ===")
}

func demonstrateBufferPooling() {
	fmt.Println("1. Buffer Pooling Performance")
	fmt.Println("   Reuses memory buffers for cell encoding/decoding")
	fmt.Println()

	// Without pooling
	start := time.Now()
	for i := 0; i < 10000; i++ {
		buf := make([]byte, 514) // Allocate new buffer each time
		_ = buf
	}
	withoutPooling := time.Since(start)

	// With pooling
	start = time.Now()
	for i := 0; i < 10000; i++ {
		buf := pool.CellBufferPool.Get()
		pool.CellBufferPool.Put(buf)
	}
	withPooling := time.Since(start)

	fmt.Printf("   Without pooling: %v (10,000 allocations)\n", withoutPooling)
	fmt.Printf("   With pooling:    %v (reuses buffers)\n", withPooling)
	fmt.Printf("   Improvement:     %.1fx faster\n", float64(withoutPooling)/float64(withPooling))
	fmt.Println()

	// Show memory efficiency
	fmt.Println("   Memory Efficiency:")
	fmt.Println("   - Cell Buffer Pool:   514 bytes (reused)")
	fmt.Println("   - Payload Buffer Pool: 509 bytes (reused)")
	fmt.Println("   - Crypto Buffer Pool: 1KB (reused)")
	fmt.Println("   - Large Crypto Pool:  8KB (reused)")
}

func demonstrateCircuitPooling() {
	fmt.Println("2. Circuit Pool with Prebuilding")
	fmt.Println("   Maintains ready-to-use circuits for instant connections")
	fmt.Println()

	log := logger.NewDefault()

	// Mock circuit builder
	buildCount := 0
	builder := func(ctx context.Context) (*circuit.Circuit, error) {
		buildCount++
		// Simulate circuit build time
		time.Sleep(10 * time.Millisecond)
		circ := &circuit.Circuit{
			ID: uint32(buildCount),
		}
		circ.SetState(circuit.StateOpen)
		return circ, nil
	}

	// Create circuit pool with prebuilding disabled
	cfg := &pool.CircuitPoolConfig{
		MinCircuits:     3,
		MaxCircuits:     10,
		PrebuildEnabled: false,
		RebuildInterval: 100 * time.Millisecond,
	}

	circuitPool := pool.NewCircuitPool(cfg, builder, log)
	defer circuitPool.Close()

	// Without prebuilding - build on demand
	fmt.Println("   Without Prebuilding:")
	start := time.Now()
	ctx := context.Background()
	circ1, err := circuitPool.Get(ctx)
	if err != nil {
		log.Error("Failed to get circuit", "error", err)
		return
	}
	onDemandTime := time.Since(start)
	fmt.Printf("   - First circuit:  %v (builds on demand)\n", onDemandTime)
	circuitPool.Put(circ1)

	// With pooling - instant retrieval
	start = time.Now()
	circ2, err := circuitPool.Get(ctx)
	if err != nil {
		log.Error("Failed to get circuit", "error", err)
		return
	}
	pooledTime := time.Since(start)
	fmt.Printf("   - Pooled circuit: %v (instant retrieval)\n", pooledTime)
	fmt.Printf("   - Improvement:    %.1fx faster\n", float64(onDemandTime)/float64(pooledTime))
	circuitPool.Put(circ2)

	// Show pool stats
	stats := circuitPool.Stats()
	fmt.Println()
	fmt.Printf("   Pool Statistics:\n")
	fmt.Printf("   - Total circuits: %d\n", stats.Total)
	fmt.Printf("   - Open circuits:  %d\n", stats.Open)
	fmt.Printf("   - Min circuits:   %d\n", stats.MinCircuits)
	fmt.Printf("   - Max circuits:   %d\n", stats.MaxCircuits)
}

func demonstrateConfiguration() {
	fmt.Println("3. Performance Configuration Options")
	fmt.Println("   New tuning options in Phase 8.3")
	fmt.Println()

	cfg := config.DefaultConfig()

	fmt.Println("   Connection Pooling:")
	fmt.Printf("   - Enabled:          %v\n", cfg.EnableConnectionPooling)
	fmt.Printf("   - Max Idle/Host:    %d\n", cfg.ConnectionPoolMaxIdle)
	fmt.Printf("   - Max Lifetime:     %v\n", cfg.ConnectionPoolMaxLife)
	fmt.Println()

	fmt.Println("   Circuit Prebuilding:")
	fmt.Printf("   - Enabled:          %v\n", cfg.EnableCircuitPrebuilding)
	fmt.Printf("   - Min Pool Size:    %d circuits\n", cfg.CircuitPoolMinSize)
	fmt.Printf("   - Max Pool Size:    %d circuits\n", cfg.CircuitPoolMaxSize)
	fmt.Println()

	fmt.Println("   Buffer Pooling:")
	fmt.Printf("   - Enabled:          %v\n", cfg.EnableBufferPooling)
	fmt.Println()

	fmt.Println("   Benefits:")
	fmt.Println("   ✓ Reduces latency for new connections")
	fmt.Println("   ✓ Minimizes memory allocations")
	fmt.Println("   ✓ Improves throughput")
	fmt.Println("   ✓ Better resource utilization")
}
