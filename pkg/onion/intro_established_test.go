package onion

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/circuit"
	"github.com/opd-ai/go-tor/pkg/logger"
)

// TestWaitForIntroEstablished_Timeout tests timeout handling
func TestWaitForIntroEstablished_Timeout(t *testing.T) {
	config := &ServiceConfig{}
	testLogger := logger.New(slog.LevelWarn, os.Stderr)
	service, err := NewService(config, testLogger)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	circ := circuit.NewCircuit(1)

	// Call with very short timeout and no response
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err = service.waitForIntroEstablished(ctx, circ)
	if err == nil {
		t.Error("waitForIntroEstablished() expected timeout error but got nil")
	} else if !stringContains(err.Error(), "deadline exceeded") && !stringContains(err.Error(), "context") {
		t.Errorf("waitForIntroEstablished() error = %q, want timeout error", err.Error())
	}
}

// TestWaitForIntroEstablished_ContextCancellation tests context cancellation
func TestWaitForIntroEstablished_ContextCancellation(t *testing.T) {
	config := &ServiceConfig{}
	testLogger := logger.New(slog.LevelWarn, os.Stderr)
	service, err := NewService(config, testLogger)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	circ := circuit.NewCircuit(1)

	// Create context and cancel it immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = service.waitForIntroEstablished(ctx, circ)
	if err == nil {
		t.Error("waitForIntroEstablished() expected cancellation error but got nil")
	} else if !stringContains(err.Error(), "canceled") && !stringContains(err.Error(), "context") {
		t.Errorf("waitForIntroEstablished() error = %q, want cancellation error", err.Error())
	}
}

// stringContains is a helper function to check if a string contains a substring
func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
