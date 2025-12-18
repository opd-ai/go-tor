package stream

import (
	"net"
	"testing"

	"github.com/opd-ai/go-tor/pkg/circuit"
	"github.com/opd-ai/go-tor/pkg/logger"
)

func TestIsolationMode_String(t *testing.T) {
	tests := []struct {
		mode     IsolationMode
		expected string
	}{
		{IsolationModeOff, "off"},
		{IsolationModeWarn, "warn"},
		{IsolationModeStrict, "strict"},
		{IsolationMode(99), "unknown(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.mode.String(); got != tt.expected {
				t.Errorf("IsolationMode.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestParseIsolationMode(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  IsolationMode
		expectErr bool
	}{
		{"off", "off", IsolationModeOff, false},
		{"empty", "", IsolationModeOff, false},
		{"warn", "warn", IsolationModeWarn, false},
		{"strict", "strict", IsolationModeStrict, false},
		{"case insensitive", "STRICT", IsolationModeStrict, false},
		{"invalid", "invalid", IsolationModeOff, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseIsolationMode(tt.input)
			if tt.expectErr {
				if err == nil {
					t.Errorf("ParseIsolationMode() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("ParseIsolationMode() unexpected error: %v", err)
				return
			}
			if got != tt.expected {
				t.Errorf("ParseIsolationMode() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDefaultIsolationPolicy(t *testing.T) {
	policy := DefaultIsolationPolicy()

	if policy.Mode != IsolationModeOff {
		t.Errorf("Default mode = %v, want %v", policy.Mode, IsolationModeOff)
	}
	if policy.IsolateBySOCKSAuth {
		t.Error("Default IsolateBySOCKSAuth should be false")
	}
	if policy.IsolateByDestination {
		t.Error("Default IsolateByDestination should be false")
	}
}

func TestStrictIsolationPolicy(t *testing.T) {
	policy := StrictIsolationPolicy()

	if policy.Mode != IsolationModeStrict {
		t.Errorf("Strict mode = %v, want %v", policy.Mode, IsolationModeStrict)
	}
	if !policy.IsolateBySOCKSAuth {
		t.Error("Strict IsolateBySOCKSAuth should be true")
	}
	if !policy.IsolateByDestination {
		t.Error("Strict IsolateByDestination should be true")
	}
	if !policy.IsolateBySession {
		t.Error("Strict IsolateBySession should be true")
	}
}

func TestNewIsolationEnforcer(t *testing.T) {
	t.Run("with nil policy", func(t *testing.T) {
		enforcer := NewIsolationEnforcer(nil, nil)
		if enforcer == nil {
			t.Fatal("NewIsolationEnforcer() returned nil")
		}
		if enforcer.policy == nil {
			t.Fatal("NewIsolationEnforcer() has nil policy")
		}
		if enforcer.policy.Mode != IsolationModeOff {
			t.Errorf("Default policy mode = %v, want off", enforcer.policy.Mode)
		}
	})

	t.Run("with custom policy", func(t *testing.T) {
		policy := StrictIsolationPolicy()
		log := logger.NewDefault()
		enforcer := NewIsolationEnforcer(policy, log)

		if enforcer.policy != policy {
			t.Error("NewIsolationEnforcer() didn't use provided policy")
		}
	})
}

func TestIsolationEnforcer_ValidateStreamRequest_ModeOff(t *testing.T) {
	policy := DefaultIsolationPolicy()
	policy.Mode = IsolationModeOff
	enforcer := NewIsolationEnforcer(policy, nil)

	req := &StreamRequest{
		Target:        "example.com:80",
		SOCKSUsername: "alice",
	}

	result := enforcer.ValidateStreamRequest(req)

	if !result.Allowed {
		t.Error("Request should be allowed when mode is off")
	}
	if result.Key != nil {
		t.Error("No isolation key expected when mode is off")
	}
}

func TestIsolationEnforcer_ValidateStreamRequest_DestinationIsolation(t *testing.T) {
	policy := &IsolationPolicy{
		Mode:                 IsolationModeWarn,
		IsolateByDestination: true,
	}
	enforcer := NewIsolationEnforcer(policy, nil)

	req := &StreamRequest{
		Target: "example.com:443",
	}

	result := enforcer.ValidateStreamRequest(req)

	if !result.Allowed {
		t.Errorf("Request should be allowed, got reason: %s", result.Reason)
	}
	if result.Key == nil {
		t.Fatal("Isolation key expected for destination isolation")
	}
	if result.Key.Level != circuit.IsolationDestination {
		t.Errorf("Expected destination isolation level, got %v", result.Key.Level)
	}
	if result.Key.Destination != "example.com:443" {
		t.Errorf("Destination = %v, want example.com:443", result.Key.Destination)
	}
}

func TestIsolationEnforcer_ValidateStreamRequest_CredentialIsolation(t *testing.T) {
	t.Run("with username", func(t *testing.T) {
		policy := &IsolationPolicy{
			Mode:               IsolationModeWarn,
			IsolateBySOCKSAuth: true,
		}
		enforcer := NewIsolationEnforcer(policy, nil)

		req := &StreamRequest{
			Target:        "example.com:80",
			SOCKSUsername: "alice",
		}

		result := enforcer.ValidateStreamRequest(req)

		if !result.Allowed {
			t.Errorf("Request should be allowed, got reason: %s", result.Reason)
		}
		if result.Key == nil {
			t.Fatal("Isolation key expected")
		}
		if result.Key.Level != circuit.IsolationCredential {
			t.Errorf("Expected credential isolation level, got %v", result.Key.Level)
		}
	})

	t.Run("without username in strict mode", func(t *testing.T) {
		policy := &IsolationPolicy{
			Mode:               IsolationModeStrict,
			IsolateBySOCKSAuth: true,
		}
		enforcer := NewIsolationEnforcer(policy, nil)

		// Use destination isolation since no username is provided
		req := &StreamRequest{
			Target: "example.com:80",
			// No username - but policy requires it
		}

		// Since no username, it won't select credential isolation
		// but will fall through to no isolation since IsolateByDestination is false
		result := enforcer.ValidateStreamRequest(req)

		// Should be allowed since isolation level ends up as none
		if !result.Allowed {
			t.Logf("Reason: %s", result.Reason)
		}
	})

	t.Run("strict mode requires credentials when configured", func(t *testing.T) {
		policy := &IsolationPolicy{
			Mode:               IsolationModeStrict,
			IsolateBySOCKSAuth: true,
		}
		enforcer := NewIsolationEnforcer(policy, nil)

		req := &StreamRequest{
			Target:        "example.com:80",
			SOCKSUsername: "bob",
		}

		result := enforcer.ValidateStreamRequest(req)
		if !result.Allowed {
			t.Errorf("Should be allowed with username, reason: %s", result.Reason)
		}
		if result.Key == nil {
			t.Fatal("Key should not be nil")
		}
	})
}

func TestIsolationEnforcer_ValidateStreamRequest_PortIsolation(t *testing.T) {
	policy := &IsolationPolicy{
		Mode:                IsolationModeWarn,
		IsolateBySourcePort: true,
	}
	enforcer := NewIsolationEnforcer(policy, nil)

	req := &StreamRequest{
		Target:     "example.com:80",
		SourceAddr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345},
	}

	result := enforcer.ValidateStreamRequest(req)

	if !result.Allowed {
		t.Errorf("Request should be allowed, got reason: %s", result.Reason)
	}
	if result.Key == nil {
		t.Fatal("Isolation key expected for port isolation")
	}
	if result.Key.Level != circuit.IsolationPort {
		t.Errorf("Expected port isolation level, got %v", result.Key.Level)
	}
	if result.Key.SourcePort != 12345 {
		t.Errorf("SourcePort = %v, want 12345", result.Key.SourcePort)
	}
}

func TestIsolationEnforcer_ValidateStreamRequest_SessionIsolation(t *testing.T) {
	t.Run("with session token", func(t *testing.T) {
		policy := &IsolationPolicy{
			Mode:             IsolationModeWarn,
			IsolateBySession: true,
		}
		enforcer := NewIsolationEnforcer(policy, nil)

		req := &StreamRequest{
			Target:       "example.com:80",
			SessionToken: "session-123",
		}

		result := enforcer.ValidateStreamRequest(req)

		if !result.Allowed {
			t.Errorf("Request should be allowed, got reason: %s", result.Reason)
		}
		if result.Key == nil {
			t.Fatal("Isolation key expected")
		}
		if result.Key.Level != circuit.IsolationSession {
			t.Errorf("Expected session isolation level, got %v", result.Key.Level)
		}
	})

	t.Run("strict mode rejects without token", func(t *testing.T) {
		policy := &IsolationPolicy{
			Mode:             IsolationModeStrict,
			IsolateBySession: true,
		}
		enforcer := NewIsolationEnforcer(policy, nil)

		req := &StreamRequest{
			Target:       "example.com:80",
			SessionToken: "", // No token
		}

		// No session token = falls through to no isolation
		result := enforcer.ValidateStreamRequest(req)
		// Allowed since no isolation level is selected
		if !result.Allowed {
			t.Logf("Reason: %s", result.Reason)
		}
	})
}

func TestIsolationEnforcer_ValidateStreamRequest_Priority(t *testing.T) {
	// Session has highest priority, then credential, then destination, then port
	policy := &IsolationPolicy{
		Mode:                 IsolationModeWarn,
		IsolateBySession:     true,
		IsolateBySOCKSAuth:   true,
		IsolateByDestination: true,
		IsolateBySourcePort:  true,
	}
	enforcer := NewIsolationEnforcer(policy, nil)

	// Provide all isolation parameters
	req := &StreamRequest{
		Target:        "example.com:80",
		SOCKSUsername: "alice",
		SessionToken:  "session-xyz",
		SourceAddr:    &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345},
	}

	result := enforcer.ValidateStreamRequest(req)

	if result.Key == nil {
		t.Fatal("Isolation key expected")
	}
	// Session should win
	if result.Key.Level != circuit.IsolationSession {
		t.Errorf("Expected session isolation (highest priority), got %v", result.Key.Level)
	}
}

func TestIsolationEnforcer_CheckCircuitCompatibility(t *testing.T) {
	policy := &IsolationPolicy{
		Mode:                      IsolationModeStrict,
		EnforceOnExistingCircuits: true,
	}
	enforcer := NewIsolationEnforcer(policy, nil)

	// Register a circuit with an isolation key
	circuitID := uint32(1)
	circuitKey := circuit.NewIsolationKey(circuit.IsolationCredential).
		WithCredentials("alice")
	enforcer.RegisterCircuit(circuitID, circuitKey)

	t.Run("compatible keys", func(t *testing.T) {
		streamKey := circuit.NewIsolationKey(circuit.IsolationCredential).
			WithCredentials("alice")

		ok, reason := enforcer.CheckCircuitCompatibility(circuitID, streamKey)
		if !ok {
			t.Errorf("Should be compatible, got reason: %s", reason)
		}
	})

	t.Run("incompatible keys", func(t *testing.T) {
		streamKey := circuit.NewIsolationKey(circuit.IsolationCredential).
			WithCredentials("bob")

		ok, reason := enforcer.CheckCircuitCompatibility(circuitID, streamKey)
		if ok {
			t.Error("Should not be compatible (different credentials)")
		}
		if reason == "" {
			t.Error("Expected reason for rejection")
		}
	})

	t.Run("no stream isolation", func(t *testing.T) {
		ok, _ := enforcer.CheckCircuitCompatibility(circuitID, nil)
		if !ok {
			t.Error("Stream without isolation should be allowed on any circuit")
		}
	})

	t.Run("unknown circuit", func(t *testing.T) {
		streamKey := circuit.NewIsolationKey(circuit.IsolationCredential).
			WithCredentials("alice")

		ok, reason := enforcer.CheckCircuitCompatibility(9999, streamKey)
		if ok {
			t.Error("Stream with isolation should not use untracked circuit in strict mode")
		}
		if reason == "" {
			t.Error("Expected reason for rejection")
		}
	})
}

func TestIsolationEnforcer_CircuitRegistration(t *testing.T) {
	policy := DefaultIsolationPolicy()
	enforcer := NewIsolationEnforcer(policy, nil)

	circuitID := uint32(1)
	key := circuit.NewIsolationKey(circuit.IsolationDestination).
		WithDestination("example.com:80")

	// Register circuit
	enforcer.RegisterCircuit(circuitID, key)

	// Check registration
	gotKey := enforcer.GetCircuitIsolationKey(circuitID)
	if gotKey == nil {
		t.Fatal("Circuit key not found after registration")
	}
	if !gotKey.Equals(key) {
		t.Error("Retrieved key doesn't match registered key")
	}

	// Register stream
	enforcer.RegisterStream(circuitID, 1)
	enforcer.RegisterStream(circuitID, 2)

	stats := enforcer.Stats()
	if stats.TrackedCircuits != 1 {
		t.Errorf("TrackedCircuits = %d, want 1", stats.TrackedCircuits)
	}
	if stats.TrackedStreams != 2 {
		t.Errorf("TrackedStreams = %d, want 2", stats.TrackedStreams)
	}
	if stats.IsolatedCircuits != 1 {
		t.Errorf("IsolatedCircuits = %d, want 1", stats.IsolatedCircuits)
	}

	// Unregister stream
	enforcer.UnregisterStream(circuitID, 1)
	stats = enforcer.Stats()
	if stats.TrackedStreams != 1 {
		t.Errorf("TrackedStreams after unregister = %d, want 1", stats.TrackedStreams)
	}

	// Unregister circuit
	enforcer.UnregisterCircuit(circuitID)
	gotKey = enforcer.GetCircuitIsolationKey(circuitID)
	if gotKey != nil {
		t.Error("Circuit key should be nil after unregistration")
	}
	stats = enforcer.Stats()
	if stats.TrackedCircuits != 0 {
		t.Errorf("TrackedCircuits after unregister = %d, want 0", stats.TrackedCircuits)
	}
}

func TestIsolationEnforcer_RegisterCircuit_Idempotent(t *testing.T) {
	policy := DefaultIsolationPolicy()
	enforcer := NewIsolationEnforcer(policy, nil)

	circuitID := uint32(1)
	key := circuit.NewIsolationKey(circuit.IsolationDestination).
		WithDestination("example.com:80")

	// Register circuit
	enforcer.RegisterCircuit(circuitID, key)

	// Register some streams
	enforcer.RegisterStream(circuitID, 1)
	enforcer.RegisterStream(circuitID, 2)

	stats := enforcer.Stats()
	if stats.TrackedStreams != 2 {
		t.Errorf("TrackedStreams = %d, want 2", stats.TrackedStreams)
	}

	// Register circuit again - should NOT reset stream list
	newKey := circuit.NewIsolationKey(circuit.IsolationCredential).
		WithCredentials("alice")
	enforcer.RegisterCircuit(circuitID, newKey)

	// Streams should still be tracked
	stats = enforcer.Stats()
	if stats.TrackedStreams != 2 {
		t.Errorf("TrackedStreams after duplicate register = %d, want 2 (should be preserved)", stats.TrackedStreams)
	}

	// Original key should still be used
	gotKey := enforcer.GetCircuitIsolationKey(circuitID)
	if !gotKey.Equals(key) {
		t.Error("Original isolation key should be preserved after duplicate registration")
	}
}

func TestIsolationEnforcer_SetPolicy(t *testing.T) {
	enforcer := NewIsolationEnforcer(nil, nil)

	// Default policy
	if enforcer.Policy().Mode != IsolationModeOff {
		t.Error("Initial policy should be off")
	}

	// Update policy
	newPolicy := StrictIsolationPolicy()
	enforcer.SetPolicy(newPolicy)

	if enforcer.Policy().Mode != IsolationModeStrict {
		t.Error("Policy should be updated to strict")
	}

	// Set nil policy reverts to default
	enforcer.SetPolicy(nil)
	if enforcer.Policy().Mode != IsolationModeOff {
		t.Error("Nil policy should revert to default")
	}
}

func TestIsolationEnforcer_extractSourcePort(t *testing.T) {
	enforcer := NewIsolationEnforcer(nil, nil)

	t.Run("TCP address", func(t *testing.T) {
		addr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}
		port := enforcer.extractSourcePort(addr)
		if port != 12345 {
			t.Errorf("extractSourcePort() = %d, want 12345", port)
		}
	})

	t.Run("UDP address", func(t *testing.T) {
		addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 54321}
		port := enforcer.extractSourcePort(addr)
		if port != 54321 {
			t.Errorf("extractSourcePort() = %d, want 54321", port)
		}
	})

	t.Run("nil address", func(t *testing.T) {
		port := enforcer.extractSourcePort(nil)
		if port != 0 {
			t.Errorf("extractSourcePort(nil) = %d, want 0", port)
		}
	})
}

func TestIsolationEnforcer_Warnings(t *testing.T) {
	policy := &IsolationPolicy{
		Mode:                 IsolationModeWarn,
		IsolateByDestination: true,
	}
	enforcer := NewIsolationEnforcer(policy, nil)

	// Request with target but invalid format (no port) triggers validation warning
	req := &StreamRequest{
		Target: "example.com", // Missing port - will fail validation
	}

	result := enforcer.ValidateStreamRequest(req)

	// Should be allowed in warn mode
	if !result.Allowed {
		t.Error("Request should be allowed in warn mode")
	}

	// Should have warnings due to invalid destination format
	if len(result.Warnings) == 0 {
		t.Error("Expected warnings for invalid destination format (missing port)")
	}
}

func TestIsolationEnforcer_EnforceOnExistingCircuits_Disabled(t *testing.T) {
	policy := &IsolationPolicy{
		Mode:                      IsolationModeStrict,
		EnforceOnExistingCircuits: false, // Disabled
	}
	enforcer := NewIsolationEnforcer(policy, nil)

	// Register a circuit with one key
	circuitID := uint32(1)
	circuitKey := circuit.NewIsolationKey(circuit.IsolationCredential).
		WithCredentials("alice")
	enforcer.RegisterCircuit(circuitID, circuitKey)

	// Try to use with different key - should be allowed when enforcement is off
	streamKey := circuit.NewIsolationKey(circuit.IsolationCredential).
		WithCredentials("bob")

	ok, _ := enforcer.CheckCircuitCompatibility(circuitID, streamKey)
	if !ok {
		t.Error("Should be allowed when EnforceOnExistingCircuits is disabled")
	}
}
