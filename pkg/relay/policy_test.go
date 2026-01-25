// Package relay - Exit policy tests
package relay

import (
	"testing"

	"github.com/opd-ai/go-tor/pkg/cell"
	"github.com/opd-ai/go-tor/pkg/logger"
)

func TestNewExitPolicy(t *testing.T) {
	log := logger.NewDefault()
	policy := NewExitPolicy(log)

	if policy == nil {
		t.Fatal("NewExitPolicy returned nil")
	}

	if policy.AllowExit {
		t.Error("Expected AllowExit to be false for non-exit relay")
	}

	if policy.GetRejectedCount() != 0 {
		t.Errorf("Expected 0 rejected connections initially, got %d", policy.GetRejectedCount())
	}
}

func TestCheckExitAllowed(t *testing.T) {
	log := logger.NewDefault()
	policy := NewExitPolicy(log)

	tests := []struct {
		name            string
		address         string
		port            uint16
		expectedAllowed bool
		expectedReason  byte
	}{
		{
			name:            "HTTP to example.com",
			address:         "example.com",
			port:            80,
			expectedAllowed: false,
			expectedReason:  cell.EndReasonExitPolicy,
		},
		{
			name:            "HTTPS to example.com",
			address:         "example.com",
			port:            443,
			expectedAllowed: false,
			expectedReason:  cell.EndReasonExitPolicy,
		},
		{
			name:            "SSH to localhost",
			address:         "127.0.0.1",
			port:            22,
			expectedAllowed: false,
			expectedReason:  cell.EndReasonExitPolicy,
		},
		{
			name:            "Any port",
			address:         "192.168.1.1",
			port:            12345,
			expectedAllowed: false,
			expectedReason:  cell.EndReasonExitPolicy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, reason := policy.CheckExitAllowed(tt.address, tt.port)

			if allowed != tt.expectedAllowed {
				t.Errorf("CheckExitAllowed(%s, %d) = %v, expected %v",
					tt.address, tt.port, allowed, tt.expectedAllowed)
			}

			if reason != tt.expectedReason {
				t.Errorf("CheckExitAllowed(%s, %d) reason = %d, expected %d",
					tt.address, tt.port, reason, tt.expectedReason)
			}
		})
	}

	// Verify that rejected count increased
	expectedRejected := uint64(len(tests))
	if policy.GetRejectedCount() != expectedRejected {
		t.Errorf("Expected %d rejected connections, got %d",
			expectedRejected, policy.GetRejectedCount())
	}
}

func TestGetExitPolicyString(t *testing.T) {
	log := logger.NewDefault()
	policy := NewExitPolicy(log)

	expected := "reject *:*"
	if policy.GetExitPolicyString() != expected {
		t.Errorf("GetExitPolicyString() = %s, expected %s",
			policy.GetExitPolicyString(), expected)
	}

	if policy.String() != expected {
		t.Errorf("String() = %s, expected %s",
			policy.String(), expected)
	}
}

func TestValidateExitAttempt(t *testing.T) {
	log := logger.NewDefault()
	policy := NewExitPolicy(log)

	tests := []struct {
		name        string
		command     byte
		address     string
		port        uint16
		expectError bool
	}{
		{
			name:        "RELAY_BEGIN should be rejected",
			command:     cell.RelayBegin,
			address:     "example.com",
			port:        80,
			expectError: true,
		},
		{
			name:        "RELAY_BEGIN_DIR should be rejected",
			command:     cell.RelayBeginDir,
			address:     "",
			port:        0,
			expectError: true,
		},
		{
			name:        "RELAY_DATA should not be checked",
			command:     cell.RelayData,
			address:     "example.com",
			port:        80,
			expectError: false,
		},
		{
			name:        "RELAY_END should not be checked",
			command:     cell.RelayEnd,
			address:     "example.com",
			port:        80,
			expectError: false,
		},
		{
			name:        "RELAY_EXTEND2 should not be checked",
			command:     cell.RelayExtend2,
			address:     "127.0.0.1",
			port:        9001,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := policy.ValidateExitAttempt(tt.command, tt.address, tt.port)

			if tt.expectError && err == nil {
				t.Error("Expected exit policy violation error, got nil")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if tt.expectError && err != nil {
				if !IsExitPolicyError(err) {
					t.Errorf("Expected ExitPolicyViolation error, got: %T", err)
				}

				violation, ok := err.(*ExitPolicyViolation)
				if !ok {
					t.Fatal("Failed to cast to ExitPolicyViolation")
				}

				if violation.Address != tt.address {
					t.Errorf("Violation address = %s, expected %s",
						violation.Address, tt.address)
				}

				if violation.Port != tt.port {
					t.Errorf("Violation port = %d, expected %d",
						violation.Port, tt.port)
				}

				if violation.GetReason() != cell.EndReasonExitPolicy {
					t.Errorf("Violation reason = %d, expected %d",
						violation.GetReason(), cell.EndReasonExitPolicy)
				}
			}
		})
	}
}

func TestExitPolicyViolation_Error(t *testing.T) {
	violation := &ExitPolicyViolation{
		Address: "example.com",
		Port:    80,
		Reason:  cell.EndReasonExitPolicy,
	}

	errMsg := violation.Error()
	if errMsg == "" {
		t.Error("Error() returned empty string")
	}

	expectedSubstring := "example.com:80"
	if !contains(errMsg, expectedSubstring) {
		t.Errorf("Error message should contain %s, got: %s",
			expectedSubstring, errMsg)
	}

	expectedReason := "EXITPOLICY"
	if !contains(errMsg, expectedReason) {
		t.Errorf("Error message should contain %s, got: %s",
			expectedReason, errMsg)
	}
}

func TestEndReasonString(t *testing.T) {
	tests := []struct {
		reason   byte
		expected string
	}{
		{cell.EndReasonMisc, "MISC"},
		{cell.EndReasonResolveFailed, "RESOLVEFAILED"},
		{cell.EndReasonConnRefused, "CONNECTREFUSED"},
		{cell.EndReasonExitPolicy, "EXITPOLICY"},
		{cell.EndReasonDestroy, "DESTROY"},
		{cell.EndReasonDone, "DONE"},
		{cell.EndReasonTimeout, "TIMEOUT"},
		{cell.EndReasonNoRoute, "NOROUTE"},
		{cell.EndReasonHibernating, "HIBERNATING"},
		{cell.EndReasonInternal, "INTERNAL"},
		{cell.EndReasonResourceLimit, "RESOURCELIMIT"},
		{cell.EndReasonConnReset, "CONNRESET"},
		{cell.EndReasonProtocol, "TORPROTOCOL"},
		{cell.EndReasonNotDirectory, "NOTDIRECTORY"},
		{255, "UNKNOWN(255)"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := endReasonString(tt.reason)
			if result != tt.expected {
				t.Errorf("endReasonString(%d) = %s, expected %s",
					tt.reason, result, tt.expected)
			}
		})
	}
}

func TestIsExitPolicyError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name: "ExitPolicyViolation",
			err: &ExitPolicyViolation{
				Address: "example.com",
				Port:    80,
				Reason:  cell.EndReasonExitPolicy,
			},
			expected: true,
		},
		{
			name:     "Generic error",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsExitPolicyError(tt.err)
			if result != tt.expected {
				t.Errorf("IsExitPolicyError() = %v, expected %v",
					result, tt.expected)
			}
		})
	}
}

func TestGetRejectedCount_Concurrent(t *testing.T) {
	log := logger.NewDefault()
	policy := NewExitPolicy(log)

	// Simulate concurrent exit attempts
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				policy.CheckExitAllowed("example.com", uint16(80+id))
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	expected := uint64(1000)
	if policy.GetRejectedCount() != expected {
		t.Errorf("Expected %d rejected connections, got %d",
			expected, policy.GetRejectedCount())
	}
}

func TestExitPolicy_AllowExitWarning(t *testing.T) {
	log := logger.NewDefault()
	policy := NewExitPolicy(log)

	// Manually set AllowExit to true (should never happen in production)
	policy.AllowExit = true

	allowed, _ := policy.CheckExitAllowed("example.com", 80)
	if !allowed {
		t.Error("Expected exit to be allowed when AllowExit is true")
	}

	// Note: In real usage, AllowExit should always be false for non-exit relays
	// This test just verifies the warning path
}
