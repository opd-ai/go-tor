package circuit_test

import (
	"context"
	"testing"

	"github.com/opd-ai/go-tor/pkg/circuit"
	"github.com/opd-ai/go-tor/pkg/logger"
	"github.com/opd-ai/go-tor/pkg/pool"
)

// BenchmarkCircuitPool_NoIsolation benchmarks circuit pool without isolation
func BenchmarkCircuitPool_NoIsolation(b *testing.B) {
	log := logger.NewDefault()
	circuitID := uint32(1)
	builder := func(ctx context.Context) (*circuit.Circuit, error) {
		circ := circuit.NewCircuit(circuitID)
		circuitID++
		circ.SetState(circuit.StateOpen)
		return circ, nil
	}

	cfg := pool.DefaultCircuitPoolConfig()
	cfg.PrebuildEnabled = false
	circuitPool := pool.NewCircuitPool(cfg, builder, log)
	defer circuitPool.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		circ, err := circuitPool.Get(ctx)
		if err != nil {
			b.Fatal(err)
		}
		circuitPool.Put(circ)
	}
}

// BenchmarkCircuitPool_DestinationIsolation benchmarks destination-based isolation
func BenchmarkCircuitPool_DestinationIsolation(b *testing.B) {
	log := logger.NewDefault()
	circuitID := uint32(1)
	builder := func(ctx context.Context) (*circuit.Circuit, error) {
		circ := circuit.NewCircuit(circuitID)
		circuitID++
		circ.SetState(circuit.StateOpen)
		return circ, nil
	}

	cfg := pool.DefaultCircuitPoolConfig()
	cfg.PrebuildEnabled = false
	circuitPool := pool.NewCircuitPool(cfg, builder, log)
	defer circuitPool.Close()

	ctx := context.Background()
	key := circuit.NewIsolationKey(circuit.IsolationDestination).
		WithDestination("example.com:443")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		circ, err := circuitPool.GetWithIsolation(ctx, key)
		if err != nil {
			b.Fatal(err)
		}
		circuitPool.Put(circ)
	}
}

// BenchmarkCircuitPool_CredentialIsolation benchmarks credential-based isolation
func BenchmarkCircuitPool_CredentialIsolation(b *testing.B) {
	log := logger.NewDefault()
	circuitID := uint32(1)
	builder := func(ctx context.Context) (*circuit.Circuit, error) {
		circ := circuit.NewCircuit(circuitID)
		circuitID++
		circ.SetState(circuit.StateOpen)
		return circ, nil
	}

	cfg := pool.DefaultCircuitPoolConfig()
	cfg.PrebuildEnabled = false
	circuitPool := pool.NewCircuitPool(cfg, builder, log)
	defer circuitPool.Close()

	ctx := context.Background()
	key := circuit.NewIsolationKey(circuit.IsolationCredential).
		WithCredentials("alice")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		circ, err := circuitPool.GetWithIsolation(ctx, key)
		if err != nil {
			b.Fatal(err)
		}
		circuitPool.Put(circ)
	}
}

// BenchmarkIsolationKey_Creation benchmarks isolation key creation and hashing
func BenchmarkIsolationKey_Creation(b *testing.B) {
	b.Run("Destination", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = circuit.NewIsolationKey(circuit.IsolationDestination).
				WithDestination("example.com:443")
		}
	})

	b.Run("Credential", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = circuit.NewIsolationKey(circuit.IsolationCredential).
				WithCredentials("alice")
		}
	})

	b.Run("Port", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = circuit.NewIsolationKey(circuit.IsolationPort).
				WithSourcePort(12345)
		}
	})

	b.Run("Session", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = circuit.NewIsolationKey(circuit.IsolationSession).
				WithSessionToken("session-abc")
		}
	})
}

// BenchmarkIsolationKey_Validation benchmarks isolation key validation
func BenchmarkIsolationKey_Validation(b *testing.B) {
	key := circuit.NewIsolationKey(circuit.IsolationDestination).
		WithDestination("example.com:443")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = key.Validate()
	}
}

// BenchmarkIsolationKey_Equals benchmarks isolation key comparison
func BenchmarkIsolationKey_Equals(b *testing.B) {
	key1 := circuit.NewIsolationKey(circuit.IsolationDestination).
		WithDestination("example.com:443")
	key2 := circuit.NewIsolationKey(circuit.IsolationDestination).
		WithDestination("example.com:443")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = key1.Equals(key2)
	}
}

// BenchmarkIsolationKey_String benchmarks isolation key string representation
func BenchmarkIsolationKey_String(b *testing.B) {
	key := circuit.NewIsolationKey(circuit.IsolationDestination).
		WithDestination("example.com:443")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = key.String()
	}
}

// BenchmarkCircuitPool_ManyIsolationKeys benchmarks pool with many isolation keys
func BenchmarkCircuitPool_ManyIsolationKeys(b *testing.B) {
	log := logger.NewDefault()
	circuitID := uint32(1)
	builder := func(ctx context.Context) (*circuit.Circuit, error) {
		circ := circuit.NewCircuit(circuitID)
		circuitID++
		circ.SetState(circuit.StateOpen)
		return circ, nil
	}

	cfg := pool.DefaultCircuitPoolConfig()
	cfg.PrebuildEnabled = false
	cfg.MaxCircuits = 100
	circuitPool := pool.NewCircuitPool(cfg, builder, log)
	defer circuitPool.Close()

	ctx := context.Background()

	// Create many different isolation keys
	keys := make([]*circuit.IsolationKey, 20)
	for i := 0; i < 20; i++ {
		keys[i] = circuit.NewIsolationKey(circuit.IsolationDestination).
			WithDestination("example.com:" + string(rune(443+i)))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := keys[i%len(keys)]
		circ, err := circuitPool.GetWithIsolation(ctx, key)
		if err != nil {
			b.Fatal(err)
		}
		circuitPool.Put(circ)
	}
}
