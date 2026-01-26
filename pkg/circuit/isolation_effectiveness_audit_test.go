package circuit_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/circuit"
	"github.com/opd-ai/go-tor/pkg/logger"
	"github.com/opd-ai/go-tor/pkg/pool"
)

// TestIsolationEffectiveness_CorrelationResistance verifies isolation prevents correlation attacks
func TestIsolationEffectiveness_CorrelationResistance(t *testing.T) {
	t.Run("CrossDestinationCorrelation", func(t *testing.T) {
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

		// Adversary scenario: Try to correlate visits to example.com and evil.com
		keyExample := circuit.NewIsolationKey(circuit.IsolationDestination).
			WithDestination("example.com:443")
		keyEvil := circuit.NewIsolationKey(circuit.IsolationDestination).
			WithDestination("evil.com:443")

		circExample, err := circuitPool.GetWithIsolation(ctx, keyExample)
		if err != nil {
			t.Fatalf("Failed to get circuit for example.com: %v", err)
		}

		circEvil, err := circuitPool.GetWithIsolation(ctx, keyEvil)
		if err != nil {
			t.Fatalf("Failed to get circuit for evil.com: %v", err)
		}

		// Verify different circuits prevent correlation
		if circExample.ID == circEvil.ID {
			t.Error("VULNERABILITY: Same circuit used for different destinations - correlation attack possible")
		}

		// Verify isolation keys are properly set
		if circExample.GetIsolationKey() == nil || circExample.GetIsolationKey().Destination != "example.com:443" {
			t.Error("VULNERABILITY: Isolation key not properly set on example.com circuit")
		}
		if circEvil.GetIsolationKey() == nil || circEvil.GetIsolationKey().Destination != "evil.com:443" {
			t.Error("VULNERABILITY: Isolation key not properly set on evil.com circuit")
		}
	})

	t.Run("MultiUserCorrelation", func(t *testing.T) {
		log := logger.NewDefault()
		circuitID := uint32(100)
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

		// Multi-user proxy scenario: alice and bob should not share circuits
		keyAlice := circuit.NewIsolationKey(circuit.IsolationCredential).
			WithCredentials("alice")
		keyBob := circuit.NewIsolationKey(circuit.IsolationCredential).
			WithCredentials("bob")

		circAlice, _ := circuitPool.GetWithIsolation(ctx, keyAlice)
		circBob, _ := circuitPool.GetWithIsolation(ctx, keyBob)

		if circAlice.ID == circBob.ID {
			t.Error("VULNERABILITY: Different users sharing circuit - multi-user correlation possible")
		}

		// Verify credentials are hashed (not plaintext)
		if circAlice.GetIsolationKey().Credentials == "alice" {
			t.Error("SECURITY RISK: Credentials stored in plaintext")
		}
		if len(circAlice.GetIsolationKey().Credentials) != 64 {
			t.Errorf("SECURITY RISK: Credential hash wrong length: %d (expected 64)",
				len(circAlice.GetIsolationKey().Credentials))
		}
	})

	t.Run("CrossApplicationCorrelation", func(t *testing.T) {
		log := logger.NewDefault()
		circuitID := uint32(200)
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

		// Different applications (browser, email) use different ports
		keyBrowser := circuit.NewIsolationKey(circuit.IsolationPort).
			WithSourcePort(50000)
		keyEmail := circuit.NewIsolationKey(circuit.IsolationPort).
			WithSourcePort(50001)

		circBrowser, _ := circuitPool.GetWithIsolation(ctx, keyBrowser)
		circEmail, _ := circuitPool.GetWithIsolation(ctx, keyEmail)

		if circBrowser.ID == circEmail.ID {
			t.Error("VULNERABILITY: Different applications sharing circuit - cross-app correlation possible")
		}
	})

	t.Run("SessionLinkage", func(t *testing.T) {
		log := logger.NewDefault()
		circuitID := uint32(300)
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

		// Different sessions should not be linkable
		keySession1 := circuit.NewIsolationKey(circuit.IsolationSession).
			WithSessionToken("session-20260126-001")
		keySession2 := circuit.NewIsolationKey(circuit.IsolationSession).
			WithSessionToken("session-20260126-002")

		circ1, _ := circuitPool.GetWithIsolation(ctx, keySession1)
		circ2, _ := circuitPool.GetWithIsolation(ctx, keySession2)

		if circ1.ID == circ2.ID {
			t.Error("VULNERABILITY: Different sessions sharing circuit - session linkage possible")
		}

		// Verify tokens are hashed
		if circ1.GetIsolationKey().SessionToken == "session-20260126-001" {
			t.Error("SECURITY RISK: Session token stored in plaintext")
		}
	})
}

// TestIsolationEffectiveness_ValidationBypass attempts to bypass isolation via malformed keys
func TestIsolationEffectiveness_ValidationBypass(t *testing.T) {
	t.Run("EmptyDestinationBypass", func(t *testing.T) {
		key := circuit.NewIsolationKey(circuit.IsolationDestination)
		// Intentionally not calling WithDestination

		err := key.Validate()
		if err == nil {
			t.Error("VULNERABILITY: Empty destination accepted - isolation bypass possible")
		}
	})

	t.Run("InvalidDestinationFormat", func(t *testing.T) {
		key := circuit.NewIsolationKey(circuit.IsolationDestination).
			WithDestination("example.com") // Missing port

		err := key.Validate()
		if err == nil {
			t.Error("VULNERABILITY: Invalid destination format accepted - isolation bypass possible")
		}
	})

	t.Run("ZeroPortBypass", func(t *testing.T) {
		key := circuit.NewIsolationKey(circuit.IsolationPort)
		// SourcePort defaults to 0

		err := key.Validate()
		if err == nil {
			t.Error("VULNERABILITY: Zero port accepted - isolation bypass possible")
		}
	})

	t.Run("EmptyCredentialBypass", func(t *testing.T) {
		key := circuit.NewIsolationKey(circuit.IsolationCredential)
		// Intentionally not calling WithCredentials

		err := key.Validate()
		if err == nil {
			t.Error("VULNERABILITY: Empty credential accepted - isolation bypass possible")
		}
	})

	t.Run("EmptySessionTokenBypass", func(t *testing.T) {
		key := circuit.NewIsolationKey(circuit.IsolationSession)
		// Intentionally not calling WithSessionToken

		err := key.Validate()
		if err == nil {
			t.Error("VULNERABILITY: Empty session token accepted - isolation bypass possible")
		}
	})
}

// TestIsolationEffectiveness_HashSecurity verifies SHA-256 hash properties
func TestIsolationEffectiveness_HashSecurity(t *testing.T) {
	t.Run("PreimageResistance", func(t *testing.T) {
		// Create key with credential
		original := "sensitive-username-123"
		key := circuit.NewIsolationKey(circuit.IsolationCredential).
			WithCredentials(original)

		hash := key.Credentials

		// Verify hash doesn't reveal original
		if hash == original {
			t.Error("SECURITY RISK: Hash equals plaintext - no hashing performed")
		}

		// Verify hash is proper SHA-256 (64 hex chars)
		if len(hash) != 64 {
			t.Errorf("SECURITY RISK: Hash length %d, expected 64", len(hash))
		}

		// Verify hash is valid hex
		_, err := hex.DecodeString(hash)
		if err != nil {
			t.Errorf("SECURITY RISK: Hash not valid hex: %v", err)
		}

		// Verify we can't easily reverse the hash
		// (Manual verification: hash should be SHA-256 output)
		expectedHash := sha256.Sum256([]byte(original))
		expectedHex := hex.EncodeToString(expectedHash[:])
		if hash != expectedHex {
			t.Error("SECURITY RISK: Hash doesn't match expected SHA-256")
		}
	})

	t.Run("CollisionResistance", func(t *testing.T) {
		// Create 1000 different keys and verify no collisions
		hashes := make(map[string]string)

		for i := 0; i < 1000; i++ {
			username := fmt.Sprintf("user-%d", i)
			key := circuit.NewIsolationKey(circuit.IsolationCredential).
				WithCredentials(username)

			hash := key.Credentials

			// Check for collision
			if existingUser, exists := hashes[hash]; exists {
				t.Errorf("COLLISION: %s and %s produced same hash: %s",
					username, existingUser, hash)
			}

			hashes[hash] = username
		}

		// Verify all hashes unique
		if len(hashes) != 1000 {
			t.Errorf("Expected 1000 unique hashes, got %d", len(hashes))
		}
	})

	t.Run("Determinism", func(t *testing.T) {
		username := "alice"

		// Create same credential multiple times
		key1 := circuit.NewIsolationKey(circuit.IsolationCredential).
			WithCredentials(username)
		key2 := circuit.NewIsolationKey(circuit.IsolationCredential).
			WithCredentials(username)
		key3 := circuit.NewIsolationKey(circuit.IsolationCredential).
			WithCredentials(username)

		// All should produce identical hash
		if key1.Credentials != key2.Credentials {
			t.Error("SECURITY RISK: Non-deterministic hashing - pool lookups will fail")
		}
		if key2.Credentials != key3.Credentials {
			t.Error("SECURITY RISK: Non-deterministic hashing - pool lookups will fail")
		}
	})
}

// TestIsolationEffectiveness_PoolIntegrity verifies pool management
func TestIsolationEffectiveness_PoolIntegrity(t *testing.T) {
	t.Run("PoolSeparation", func(t *testing.T) {
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

		// Create circuits in different isolated pools
		key1 := circuit.NewIsolationKey(circuit.IsolationDestination).WithDestination("pool1.com:80")
		key2 := circuit.NewIsolationKey(circuit.IsolationDestination).WithDestination("pool2.com:80")
		keyNone := circuit.NewIsolationKey(circuit.IsolationNone)

		circ1, _ := circuitPool.GetWithIsolation(ctx, key1)
		circ2, _ := circuitPool.GetWithIsolation(ctx, key2)
		circNone, _ := circuitPool.GetWithIsolation(ctx, keyNone)

		// Return to pools
		circuitPool.Put(circ1)
		circuitPool.Put(circ2)
		circuitPool.Put(circNone)

		// Get again and verify correct pool assignment
		circ1Again, _ := circuitPool.GetWithIsolation(ctx, key1)
		circ2Again, _ := circuitPool.GetWithIsolation(ctx, key2)
		circNoneAgain, _ := circuitPool.GetWithIsolation(ctx, keyNone)

		// Each pool should return its own circuit
		if circ1.ID != circ1Again.ID {
			t.Error("VULNERABILITY: Pool1 returned wrong circuit - cross-pool contamination")
		}
		if circ2.ID != circ2Again.ID {
			t.Error("VULNERABILITY: Pool2 returned wrong circuit - cross-pool contamination")
		}
		if circNone.ID != circNoneAgain.ID {
			t.Error("VULNERABILITY: Default pool returned wrong circuit - cross-pool contamination")
		}

		// Different pools should have different circuits
		if circ1.ID == circ2.ID {
			t.Error("VULNERABILITY: Different isolated pools sharing circuit")
		}
		if circ1.ID == circNone.ID {
			t.Error("VULNERABILITY: Isolated pool sharing with default pool")
		}
	})

	t.Run("ClosedCircuitPrevention", func(t *testing.T) {
		log := logger.NewDefault()
		circuitID := uint32(100)
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
		key := circuit.NewIsolationKey(circuit.IsolationDestination).WithDestination("test.com:80")

		// Get circuit
		circ1, _ := circuitPool.GetWithIsolation(ctx, key)
		originalID := circ1.ID

		// Close circuit and return to pool
		circ1.SetState(circuit.StateClosed)
		circuitPool.Put(circ1) // Pool should reject this

		// Get new circuit - should NOT be the closed one
		circ2, _ := circuitPool.GetWithIsolation(ctx, key)

		if circ2.ID == originalID {
			t.Error("VULNERABILITY: Pool returned closed circuit - state leakage possible")
		}

		if circ2.GetState() != circuit.StateOpen {
			t.Errorf("VULNERABILITY: New circuit not in Open state: %s", circ2.GetState())
		}
	})

	t.Run("FailedCircuitPrevention", func(t *testing.T) {
		log := logger.NewDefault()
		circuitID := uint32(200)
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
		key := circuit.NewIsolationKey(circuit.IsolationDestination).WithDestination("fail.com:80")

		// Get circuit and mark as failed
		circ1, _ := circuitPool.GetWithIsolation(ctx, key)
		circ1.SetState(circuit.StateFailed)
		circuitPool.Put(circ1) // Pool should reject failed circuits

		// Get new circuit - should be fresh
		circ2, _ := circuitPool.GetWithIsolation(ctx, key)

		if circ2.ID == circ1.ID {
			t.Error("VULNERABILITY: Pool returned failed circuit - error propagation risk")
		}
	})

	t.Run("CapacityEnforcement", func(t *testing.T) {
		log := logger.NewDefault()
		circuitID := uint32(300)
		builder := func(ctx context.Context) (*circuit.Circuit, error) {
			circ := circuit.NewCircuit(circuitID)
			circuitID++
			circ.SetState(circuit.StateOpen)
			return circ, nil
		}

		cfg := &pool.CircuitPoolConfig{
			MinCircuits:     0,
			MaxCircuits:     3,
			PrebuildEnabled: false,
			RebuildInterval: 30 * time.Second,
		}
		circuitPool := pool.NewCircuitPool(cfg, builder, log)
		defer circuitPool.Close()

		ctx := context.Background()
		key := circuit.NewIsolationKey(circuit.IsolationDestination).WithDestination("capacity.com:80")

		// Get MaxCircuits + 2 circuits
		circuits := make([]*circuit.Circuit, 5)
		for i := 0; i < 5; i++ {
			circ, _ := circuitPool.GetWithIsolation(ctx, key)
			circuits[i] = circ
		}

		// Return all to pool
		for _, circ := range circuits {
			circuitPool.Put(circ)
		}

		// Verify capacity enforced
		stats := circuitPool.Stats()
		if stats.IsolatedCircuits > 3 {
			t.Errorf("VULNERABILITY: Pool exceeded capacity: %d circuits (max 3) - isolation bypass via exhaustion",
				stats.IsolatedCircuits)
		}
	})
}

// TestIsolationEffectiveness_Concurrency verifies thread-safety
func TestIsolationEffectiveness_Concurrency(t *testing.T) {
	log := logger.NewDefault()
	circuitID := uint32(1)
	var mu sync.Mutex
	builder := func(ctx context.Context) (*circuit.Circuit, error) {
		mu.Lock()
		circ := circuit.NewCircuit(circuitID)
		circuitID++
		mu.Unlock()
		circ.SetState(circuit.StateOpen)
		return circ, nil
	}

	cfg := pool.DefaultCircuitPoolConfig()
	cfg.PrebuildEnabled = false
	cfg.MaxCircuits = 100
	circuitPool := pool.NewCircuitPool(cfg, builder, log)
	defer circuitPool.Close()

	ctx := context.Background()

	// Concurrent access to multiple isolated pools
	const goroutines = 20
	const iterations = 50

	var wg sync.WaitGroup
	errors := make(chan error, goroutines*iterations)

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := circuit.NewIsolationKey(circuit.IsolationDestination).
				WithDestination(fmt.Sprintf("site%d.com:80", id%10)) // 10 different pools

			for i := 0; i < iterations; i++ {
				circ, err := circuitPool.GetWithIsolation(ctx, key)
				if err != nil {
					errors <- fmt.Errorf("goroutine %d: failed to get circuit: %v", id, err)
					continue
				}

				// Verify isolation key set correctly
				if circ.GetIsolationKey() == nil {
					errors <- fmt.Errorf("goroutine %d: isolation key not set", id)
					continue
				}

				// Return to pool
				circuitPool.Put(circ)
			}
		}(g)
	}

	wg.Wait()
	close(errors)

	// Check for any errors
	for err := range errors {
		t.Error(err)
	}

	// Verify pool integrity after concurrent access
	stats := circuitPool.Stats()
	if stats.IsolatedPools < 1 {
		t.Error("VULNERABILITY: Concurrent access corrupted isolated pools")
	}
}

// TestIsolationEffectiveness_KeyComparison verifies key equality logic
func TestIsolationEffectiveness_KeyComparison(t *testing.T) {
	t.Run("SameDestinationEquals", func(t *testing.T) {
		key1 := circuit.NewIsolationKey(circuit.IsolationDestination).
			WithDestination("example.com:443")
		key2 := circuit.NewIsolationKey(circuit.IsolationDestination).
			WithDestination("example.com:443")

		if !key1.Equals(key2) {
			t.Error("VULNERABILITY: Same destination keys not equal - pool lookup will fail")
		}

		// Verify pool keys identical
		if key1.Key() != key2.Key() {
			t.Error("VULNERABILITY: Same destination keys produce different pool keys")
		}
	})

	t.Run("DifferentDestinationNotEquals", func(t *testing.T) {
		key1 := circuit.NewIsolationKey(circuit.IsolationDestination).
			WithDestination("example.com:443")
		key2 := circuit.NewIsolationKey(circuit.IsolationDestination).
			WithDestination("example.com:80") // Different port

		if key1.Equals(key2) {
			t.Error("VULNERABILITY: Different destination keys equal - isolation bypass")
		}

		if key1.Key() == key2.Key() {
			t.Error("VULNERABILITY: Different destinations produce same pool key - collision")
		}
	})

	t.Run("PortCaseSensitivity", func(t *testing.T) {
		// Verify port is part of destination comparison
		key80 := circuit.NewIsolationKey(circuit.IsolationDestination).
			WithDestination("example.com:80")
		key443 := circuit.NewIsolationKey(circuit.IsolationDestination).
			WithDestination("example.com:443")

		if key80.Equals(key443) {
			t.Error("VULNERABILITY: HTTP and HTTPS ports not distinguished - protocol confusion attack")
		}
	})

	t.Run("CredentialHashEquality", func(t *testing.T) {
		// Same credential should hash to same value
		key1 := circuit.NewIsolationKey(circuit.IsolationCredential).
			WithCredentials("alice")
		key2 := circuit.NewIsolationKey(circuit.IsolationCredential).
			WithCredentials("alice")

		if !key1.Equals(key2) {
			t.Error("VULNERABILITY: Same credential hashes not equal - pool lookup will fail")
		}

		// Verify exact hash match
		if key1.Credentials != key2.Credentials {
			t.Error("VULNERABILITY: Same credential produces different hash - non-deterministic")
		}
	})

	t.Run("LevelMismatchNotEquals", func(t *testing.T) {
		keyDest := circuit.NewIsolationKey(circuit.IsolationDestination).
			WithDestination("example.com:80")
		keyPort := circuit.NewIsolationKey(circuit.IsolationPort).
			WithSourcePort(12345)

		if keyDest.Equals(keyPort) {
			t.Error("VULNERABILITY: Different isolation levels considered equal - isolation bypass")
		}
	})
}

// TestIsolationEffectiveness_PoolKeyUniqueness verifies pool key generation
func TestIsolationEffectiveness_PoolKeyUniqueness(t *testing.T) {
	poolKeys := make(map[string]*circuit.IsolationKey)

	// Generate various isolation keys
	testCases := []struct {
		name string
		key  *circuit.IsolationKey
	}{
		{"dest1", circuit.NewIsolationKey(circuit.IsolationDestination).WithDestination("site1.com:80")},
		{"dest2", circuit.NewIsolationKey(circuit.IsolationDestination).WithDestination("site2.com:80")},
		{"dest3", circuit.NewIsolationKey(circuit.IsolationDestination).WithDestination("site1.com:443")}, // Same host, different port
		{"cred1", circuit.NewIsolationKey(circuit.IsolationCredential).WithCredentials("alice")},
		{"cred2", circuit.NewIsolationKey(circuit.IsolationCredential).WithCredentials("bob")},
		{"port1", circuit.NewIsolationKey(circuit.IsolationPort).WithSourcePort(12345)},
		{"port2", circuit.NewIsolationKey(circuit.IsolationPort).WithSourcePort(54321)},
		{"session1", circuit.NewIsolationKey(circuit.IsolationSession).WithSessionToken("token-alpha")},
		{"session2", circuit.NewIsolationKey(circuit.IsolationSession).WithSessionToken("token-beta")},
	}

	for _, tc := range testCases {
		poolKey := tc.key.Key()

		// Check for collisions
		if existingKey, exists := poolKeys[poolKey]; exists {
			t.Errorf("COLLISION: %s and existing key produce same pool key: %s\n  Key1: %+v\n  Key2: %+v",
				tc.name, poolKey, tc.key, existingKey)
		}

		poolKeys[poolKey] = tc.key
	}

	// Verify all unique
	if len(poolKeys) != len(testCases) {
		t.Errorf("Expected %d unique pool keys, got %d", len(testCases), len(poolKeys))
	}

	// Verify no empty pool keys
	for name, key := range poolKeys {
		poolKey := key.Key()
		if poolKey == "" {
			t.Errorf("VULNERABILITY: %s produced empty pool key - will use default pool", name)
		}
	}
}

// TestIsolationEffectiveness_BackwardCompatibility verifies IsolationNone behavior
func TestIsolationEffectiveness_BackwardCompatibility(t *testing.T) {
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

	// Create circuit with IsolationNone (should use default pool)
	keyNone := circuit.NewIsolationKey(circuit.IsolationNone)
	circ1, _ := circuitPool.GetWithIsolation(ctx, keyNone)

	// Create circuit with nil key (backward compatible call)
	circ2, _ := circuitPool.GetWithIsolation(ctx, nil)

	// Both should use default pool (may reuse circuits)
	// We don't enforce same ID, but verify both work
	if circ1 == nil || circ2 == nil {
		t.Error("REGRESSION: IsolationNone or nil key failed - backward compatibility broken")
	}

	// Verify IsolationNone produces empty pool key
	if keyNone.Key() != "" {
		t.Error("VULNERABILITY: IsolationNone should not create isolated pool")
	}

	// Return and get again
	circuitPool.Put(circ1)
	circuitPool.Put(circ2)

	circ3, _ := circuitPool.GetWithIsolation(ctx, nil)
	if circ3 == nil {
		t.Error("REGRESSION: Failed to get circuit with nil key")
	}
}
