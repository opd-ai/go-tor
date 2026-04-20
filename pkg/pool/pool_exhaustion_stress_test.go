package pool

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/circuit"
)

// --- BufferPool Exhaustion Tests ---

func TestStressBufferPoolRapidGetWithoutPut(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}
	pool := NewBufferPool(512)
	for i := 0; i < 10000; i++ {
		buf := pool.Get()
		if len(buf) != 512 {
			t.Fatalf("unexpected buffer size: %d", len(buf))
		}
	}
}

func TestStressBufferPoolConcurrentHeavyLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}
	pool := NewBufferPool(256)
	var wg sync.WaitGroup
	const goroutines = 100
	const iterations = 1000

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				buf := pool.Get()
				buf[0] = byte(j)
				pool.Put(buf)
			}
		}()
	}
	wg.Wait()
}

func TestStressBufferPoolPutWrongSize(t *testing.T) {
	pool := NewBufferPool(512)

	tests := []struct {
		name string
		buf  []byte
	}{
		{"too_small", make([]byte, 64)},
		{"too_large", make([]byte, 4096)},
		{"empty", make([]byte, 0)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool.Put(tt.buf)
			buf := pool.Get()
			if len(buf) != 512 {
				t.Errorf("expected 512, got %d", len(buf))
			}
		})
	}
}

func TestStressMixedBufferPoolsConcurrent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}
	var wg sync.WaitGroup
	const goroutines = 25

	pools := []*BufferPool{CellBufferPool, PayloadBufferPool, CryptoBufferPool, LargeCryptoBufferPool}
	sizes := []int{514, 509, 1024, 8192}

	wg.Add(goroutines * len(pools))
	for pi, p := range pools {
		expectedSize := sizes[pi]
		for i := 0; i < goroutines; i++ {
			go func(bp *BufferPool, sz int) {
				defer wg.Done()
				for j := 0; j < 500; j++ {
					buf := bp.Get()
					if len(buf) != sz {
						return
					}
					bp.Put(buf)
				}
			}(p, expectedSize)
		}
	}
	wg.Wait()
}

// --- CircuitPool Exhaustion Tests ---

func failingCircuitBuilder(_ context.Context) (*circuit.Circuit, error) {
	return nil, errors.New("build failed")
}

func nilCircuitBuilder(_ context.Context) (*circuit.Circuit, error) {
	return nil, nil //nolint:nilnil // testing nil return
}

func stressCircuitBuilder(_ context.Context) (*circuit.Circuit, error) {
	circ := circuit.NewCircuit(uint32(time.Now().UnixNano() % 1000000))
	circ.SetState(circuit.StateOpen)
	return circ, nil
}

func TestStressCircuitPoolGetWithFailingBuilder(t *testing.T) {
	cfg := &CircuitPoolConfig{
		MinCircuits:     0,
		MaxCircuits:     5,
		PrebuildEnabled: false,
	}
	p := NewCircuitPool(cfg, failingCircuitBuilder, nil)
	defer p.Close()

	_, err := p.Get(context.Background())
	if err == nil {
		t.Fatal("expected error from failing builder")
	}
}

func TestStressCircuitPoolGetWithNilBuilder(t *testing.T) {
	cfg := &CircuitPoolConfig{
		MinCircuits:     0,
		MaxCircuits:     5,
		PrebuildEnabled: false,
	}
	p := NewCircuitPool(cfg, nilCircuitBuilder, nil)
	defer p.Close()

	circ, err := p.Get(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if circ != nil {
		t.Fatal("expected nil circuit from nil builder")
	}
}

func TestStressCircuitPoolGetCancelledContext(t *testing.T) {
	slowBuilder := func(ctx context.Context) (*circuit.Circuit, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(5 * time.Second):
			circ := circuit.NewCircuit(1)
			circ.SetState(circuit.StateOpen)
			return circ, nil
		}
	}

	cfg := &CircuitPoolConfig{
		MinCircuits:     0,
		MaxCircuits:     5,
		PrebuildEnabled: false,
	}
	p := NewCircuitPool(cfg, slowBuilder, nil)
	defer p.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := p.Get(ctx)
	if err == nil {
		t.Fatal("expected error with cancelled context")
	}
}

func TestStressCircuitPoolPutNilCircuit(t *testing.T) {
	cfg := &CircuitPoolConfig{
		MinCircuits:     0,
		MaxCircuits:     5,
		PrebuildEnabled: false,
	}
	p := NewCircuitPool(cfg, stressCircuitBuilder, nil)
	defer p.Close()

	// Should not panic
	p.Put(nil)

	stats := p.Stats()
	if stats.Total != 0 {
		t.Errorf("expected 0 circuits, got %d", stats.Total)
	}
}

func TestStressCircuitPoolPutClosedCircuit(t *testing.T) {
	cfg := &CircuitPoolConfig{
		MinCircuits:     0,
		MaxCircuits:     5,
		PrebuildEnabled: false,
	}
	p := NewCircuitPool(cfg, stressCircuitBuilder, nil)
	defer p.Close()

	circ := circuit.NewCircuit(42)
	circ.SetState(circuit.StateClosed)
	p.Put(circ)

	circ2 := circuit.NewCircuit(43)
	circ2.SetState(circuit.StateFailed)
	p.Put(circ2)

	stats := p.Stats()
	if stats.Total != 0 {
		t.Errorf("expected 0, got %d", stats.Total)
	}
}

func TestStressCircuitPoolPutAtMaxCapacity(t *testing.T) {
	cfg := &CircuitPoolConfig{
		MinCircuits:     0,
		MaxCircuits:     3,
		PrebuildEnabled: false,
	}
	p := NewCircuitPool(cfg, stressCircuitBuilder, nil)
	defer p.Close()

	// Fill pool to max
	for i := 0; i < 3; i++ {
		circ := circuit.NewCircuit(uint32(i + 1))
		circ.SetState(circuit.StateOpen)
		p.Put(circ)
	}

	stats := p.Stats()
	if stats.Total != 3 {
		t.Fatalf("expected 3, got %d", stats.Total)
	}

	// Try to add one more - should be rejected
	extra := circuit.NewCircuit(99)
	extra.SetState(circuit.StateOpen)
	p.Put(extra)

	stats = p.Stats()
	if stats.Total > 3 {
		t.Errorf("pool exceeded max: got %d", stats.Total)
	}
}

func TestStressCircuitPoolConcurrentGetPut(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	cfg := &CircuitPoolConfig{
		MinCircuits:     0,
		MaxCircuits:     20,
		PrebuildEnabled: false,
	}
	p := NewCircuitPool(cfg, stressCircuitBuilder, nil)
	defer p.Close()

	var wg sync.WaitGroup
	const goroutines = 50

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				circ, err := p.Get(context.Background())
				if err != nil {
					continue
				}
				if circ != nil {
					p.Put(circ)
				}
			}
		}()
	}
	wg.Wait()
}

func TestStressCircuitPoolIsolatedExhaustion(t *testing.T) {
	cfg := &CircuitPoolConfig{
		MinCircuits:     0,
		MaxCircuits:     3,
		PrebuildEnabled: false,
	}
	p := NewCircuitPool(cfg, stressCircuitBuilder, nil)
	defer p.Close()

	isoKey := circuit.NewIsolationKey(circuit.IsolationDestination)
	isoKey = isoKey.WithDestination("example.com:443")

	// Fill isolated pool to max
	for i := 0; i < 3; i++ {
		circ := circuit.NewCircuit(uint32(i + 100))
		circ.SetState(circuit.StateOpen)
		circ.SetIsolationKey(isoKey)
		p.Put(circ)
	}

	// Try to add one more to isolated pool
	extra := circuit.NewCircuit(200)
	extra.SetState(circuit.StateOpen)
	extra.SetIsolationKey(isoKey)
	p.Put(extra)

	stats := p.Stats()
	if stats.IsolatedCircuits > 3 {
		t.Errorf("isolated pool exceeded max: %d", stats.IsolatedCircuits)
	}
}

func TestStressCircuitPoolStatsAccuracyConcurrent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	cfg := &CircuitPoolConfig{
		MinCircuits:     0,
		MaxCircuits:     50,
		PrebuildEnabled: false,
	}
	p := NewCircuitPool(cfg, stressCircuitBuilder, nil)
	defer p.Close()

	var wg sync.WaitGroup
	var statsChecks atomic.Int64

	wg.Add(30)
	for i := 0; i < 20; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				circ, err := p.Get(context.Background())
				if err == nil && circ != nil {
					p.Put(circ)
				}
			}
		}()
	}
	for i := 0; i < 10; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				stats := p.Stats()
				if stats.Total >= 0 && stats.Open >= 0 {
					statsChecks.Add(1)
				}
			}
		}()
	}
	wg.Wait()

	if statsChecks.Load() == 0 {
		t.Error("no stats checks completed")
	}
}

func TestStressCircuitPoolCloseAndGet(t *testing.T) {
	cfg := &CircuitPoolConfig{
		MinCircuits:     0,
		MaxCircuits:     5,
		PrebuildEnabled: false,
	}
	p := NewCircuitPool(cfg, stressCircuitBuilder, nil)

	// Pre-fill the pool
	circ := circuit.NewCircuit(1)
	circ.SetState(circuit.StateOpen)
	p.Put(circ)

	// Close the pool
	if err := p.Close(); err != nil {
		t.Fatalf("close error: %v", err)
	}

	// After close, circuits should be gone
	stats := p.Stats()
	if stats.Open != 0 {
		t.Errorf("expected 0 open after close, got %d", stats.Open)
	}
}

// --- ConnectionPool Exhaustion Tests ---

func TestStressConnectionPoolCloseEmpty(t *testing.T) {
	p := NewConnectionPool(nil, nil)
	err := p.Close()
	if err != nil {
		t.Errorf("close empty pool error: %v", err)
	}
}

func TestStressConnectionPoolStatsEmpty(t *testing.T) {
	p := NewConnectionPool(nil, nil)
	defer p.Close()

	stats := p.Stats()
	if stats.Total != 0 || stats.InUse != 0 || stats.Idle != 0 {
		t.Errorf("unexpected stats on empty pool: %+v", stats)
	}
}

func TestStressConnectionPoolCleanupOnEmpty(t *testing.T) {
	p := NewConnectionPool(nil, nil)
	defer p.Close()

	// Should not panic on empty pool
	p.CleanupExpired()
	p.CleanupIdle(time.Second)
}

func TestStressConnectionPoolRemoveFromEmpty(t *testing.T) {
	p := NewConnectionPool(nil, nil)
	defer p.Close()

	// Should not panic
	p.Remove("nonexistent:1234")

	stats := p.Stats()
	if stats.Total != 0 {
		t.Errorf("expected 0, got %d", stats.Total)
	}
}

func TestStressConnectionPoolConcurrentOps(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	p := NewConnectionPool(nil, nil)
	defer p.Close()

	var wg sync.WaitGroup
	const goroutines = 50

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = p.Stats()
				p.CleanupExpired()
				p.CleanupIdle(time.Minute)
				p.Remove("addr:9999")
			}
		}(i)
	}
	wg.Wait()
}
