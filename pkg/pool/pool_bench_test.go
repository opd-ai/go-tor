package pool

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/circuit"
	"github.com/opd-ai/go-tor/pkg/logger"
)

// Benchmark buffer pool operations
func BenchmarkCellBufferPoolGet(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := CellBufferPool.Get()
		_ = buf
	}
}

func BenchmarkCellBufferPoolGetPut(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := CellBufferPool.Get()
		CellBufferPool.Put(buf)
	}
}

func BenchmarkCellBufferPoolGetPutParallel(b *testing.B) {
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := CellBufferPool.Get()
			CellBufferPool.Put(buf)
		}
	})
}

// Benchmark without pooling for comparison
func BenchmarkCellBufferNoPool(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := make([]byte, 514)
		_ = buf
	}
}

func BenchmarkPayloadBufferPoolGetPut(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := PayloadBufferPool.Get()
		PayloadBufferPool.Put(buf)
	}
}

func BenchmarkCryptoBufferPoolGetPut(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := CryptoBufferPool.Get()
		CryptoBufferPool.Put(buf)
	}
}

func BenchmarkLargeCryptoBufferPoolGetPut(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := LargeCryptoBufferPool.Get()
		LargeCryptoBufferPool.Put(buf)
	}
}

// Benchmark circuit pool operations
func BenchmarkCircuitPoolGet(b *testing.B) {
	log := logger.NewDefault()
	builder := func(ctx context.Context) (*circuit.Circuit, error) {
		circ := &circuit.Circuit{ID: 1}
		circ.SetState(circuit.StateOpen)
		return circ, nil
	}

	cfg := &CircuitPoolConfig{
		MinCircuits:     10,
		MaxCircuits:     100,
		PrebuildEnabled: false,
	}

	pool := NewCircuitPool(cfg, builder, log)
	defer pool.Close()

	// Pre-fill the pool
	ctx := context.Background()
	circuits := make([]*circuit.Circuit, 10)
	for i := 0; i < 10; i++ {
		circ, _ := pool.Get(ctx)
		circuits[i] = circ
	}
	for _, circ := range circuits {
		pool.Put(circ)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		circ, _ := pool.Get(ctx)
		pool.Put(circ)
	}
}

func BenchmarkCircuitPoolGetParallel(b *testing.B) {
	log := logger.NewDefault()
	builder := func(ctx context.Context) (*circuit.Circuit, error) {
		circ := &circuit.Circuit{ID: uint32(time.Now().UnixNano())}
		circ.SetState(circuit.StateOpen)
		return circ, nil
	}

	cfg := &CircuitPoolConfig{
		MinCircuits:     20,
		MaxCircuits:     100,
		PrebuildEnabled: false,
	}

	pool := NewCircuitPool(cfg, builder, log)
	defer pool.Close()

	// Pre-fill the pool
	ctx := context.Background()
	circuits := make([]*circuit.Circuit, 20)
	for i := 0; i < 20; i++ {
		circ, _ := pool.Get(ctx)
		circuits[i] = circ
	}
	for _, circ := range circuits {
		pool.Put(circ)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			circ, _ := pool.Get(ctx)
			pool.Put(circ)
		}
	})
}

// Benchmark circuit creation without pooling
func BenchmarkCircuitCreateNoPool(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		circ := &circuit.Circuit{ID: uint32(i)}
		circ.SetState(circuit.StateOpen)
		_ = circ
	}
}

// Benchmark connection pool operations
func BenchmarkConnectionPoolStats(b *testing.B) {
	log := logger.NewDefault()
	pool := NewConnectionPool(nil, log)
	defer pool.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pool.Stats()
	}
}

func BenchmarkConnectionPoolCleanupIdle(b *testing.B) {
	log := logger.NewDefault()
	pool := NewConnectionPool(nil, log)
	defer pool.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pool.CleanupIdle(1 * time.Minute)
	}
}

func BenchmarkConnectionPoolCleanupExpired(b *testing.B) {
	log := logger.NewDefault()
	pool := NewConnectionPool(nil, log)
	defer pool.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pool.CleanupExpired()
	}
}

// Benchmark different buffer sizes
func BenchmarkBufferPoolSizes(b *testing.B) {
	sizes := []int{64, 128, 256, 512, 1024, 2048, 4096, 8192}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("size-%d", size), func(b *testing.B) {
			pool := NewBufferPool(size)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				buf := pool.Get()
				pool.Put(buf)
			}
		})
	}
}

// Benchmark memory allocation patterns
func BenchmarkMemoryAllocationWithPool(b *testing.B) {
	pool := NewBufferPool(1024)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		buf := pool.Get()
		// Simulate some work
		buf[0] = byte(i)
		pool.Put(buf)
	}
}

func BenchmarkMemoryAllocationWithoutPool(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		buf := make([]byte, 1024)
		// Simulate some work
		buf[0] = byte(i)
		_ = buf
	}
}
