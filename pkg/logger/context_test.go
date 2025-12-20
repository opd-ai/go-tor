package logger

import (
	"bytes"
	"context"
	"encoding/hex"
	"log/slog"
	"strings"
	"testing"
)

func TestGenerateRequestID(t *testing.T) {
	t.Run("generates valid ID", func(t *testing.T) {
		id, err := GenerateRequestID()
		if err != nil {
			t.Fatalf("GenerateRequestID() returned error: %v", err)
		}
		// ID should be 16 hex characters (8 bytes encoded)
		if len(id) != 16 {
			t.Errorf("GenerateRequestID() returned ID of length %d, want 16", len(id))
		}
		// Verify it's valid hex by attempting to decode it
		if _, err := hex.DecodeString(id); err != nil {
			t.Errorf("GenerateRequestID() returned invalid hex: %s, error: %v", id, err)
		}
	})

	t.Run("generates unique IDs", func(t *testing.T) {
		seen := make(map[string]bool)
		for i := 0; i < 100; i++ {
			id, err := GenerateRequestID()
			if err != nil {
				t.Fatalf("GenerateRequestID() returned error: %v", err)
			}
			if seen[id] {
				t.Errorf("GenerateRequestID() generated duplicate ID: %s", id)
			}
			seen[id] = true
		}
	})
}

func TestWithCorrelationID(t *testing.T) {
	ctx := context.Background()
	correlationID := "test-correlation-123"

	newCtx := WithCorrelationID(ctx, correlationID)

	if newCtx == ctx {
		t.Error("WithCorrelationID() should return a new context")
	}

	// Verify the original context is unchanged
	if GetCorrelationID(ctx) != "" {
		t.Error("Original context should not have correlation ID")
	}
}

func TestGetCorrelationID(t *testing.T) {
	t.Run("returns empty string for context without ID", func(t *testing.T) {
		ctx := context.Background()
		id := GetCorrelationID(ctx)
		if id != "" {
			t.Errorf("GetCorrelationID() = %q, want empty string", id)
		}
	})

	t.Run("returns ID when set", func(t *testing.T) {
		ctx := context.Background()
		expectedID := "my-correlation-id"
		ctx = WithCorrelationID(ctx, expectedID)

		id := GetCorrelationID(ctx)
		if id != expectedID {
			t.Errorf("GetCorrelationID() = %q, want %q", id, expectedID)
		}
	})

	t.Run("returns latest ID when overwritten", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithCorrelationID(ctx, "first-id")
		ctx = WithCorrelationID(ctx, "second-id")

		id := GetCorrelationID(ctx)
		if id != "second-id" {
			t.Errorf("GetCorrelationID() = %q, want %q", id, "second-id")
		}
	})
}

func TestWithConnectionID(t *testing.T) {
	ctx := context.Background()
	connectionID := "conn-456"

	newCtx := WithConnectionID(ctx, connectionID)

	if newCtx == ctx {
		t.Error("WithConnectionID() should return a new context")
	}

	// Verify the original context is unchanged
	if GetConnectionID(ctx) != "" {
		t.Error("Original context should not have connection ID")
	}
}

func TestGetConnectionID(t *testing.T) {
	t.Run("returns empty string for context without ID", func(t *testing.T) {
		ctx := context.Background()
		id := GetConnectionID(ctx)
		if id != "" {
			t.Errorf("GetConnectionID() = %q, want empty string", id)
		}
	})

	t.Run("returns ID when set", func(t *testing.T) {
		ctx := context.Background()
		expectedID := "my-connection-id"
		ctx = WithConnectionID(ctx, expectedID)

		id := GetConnectionID(ctx)
		if id != expectedID {
			t.Errorf("GetConnectionID() = %q, want %q", id, expectedID)
		}
	})
}

func TestLoggerWithCorrelationContext(t *testing.T) {
	t.Run("adds correlation ID from context", func(t *testing.T) {
		var buf bytes.Buffer
		logger := New(slog.LevelInfo, &buf)

		ctx := context.Background()
		ctx = WithCorrelationID(ctx, "corr-abc123")

		ctxLogger := logger.WithCorrelationContext(ctx)
		ctxLogger.Info("test message")

		output := buf.String()
		if !strings.Contains(output, "correlation_id=corr-abc123") {
			t.Errorf("Expected output to contain 'correlation_id=corr-abc123', got: %s", output)
		}
	})

	t.Run("adds connection ID from context", func(t *testing.T) {
		var buf bytes.Buffer
		logger := New(slog.LevelInfo, &buf)

		ctx := context.Background()
		ctx = WithConnectionID(ctx, "conn-xyz789")

		ctxLogger := logger.WithCorrelationContext(ctx)
		ctxLogger.Info("test message")

		output := buf.String()
		if !strings.Contains(output, "connection_id=conn-xyz789") {
			t.Errorf("Expected output to contain 'connection_id=conn-xyz789', got: %s", output)
		}
	})

	t.Run("adds both IDs from context", func(t *testing.T) {
		var buf bytes.Buffer
		logger := New(slog.LevelInfo, &buf)

		ctx := context.Background()
		ctx = WithCorrelationID(ctx, "corr-123")
		ctx = WithConnectionID(ctx, "conn-456")

		ctxLogger := logger.WithCorrelationContext(ctx)
		ctxLogger.Info("test message")

		output := buf.String()
		if !strings.Contains(output, "correlation_id=corr-123") {
			t.Errorf("Expected output to contain 'correlation_id=corr-123', got: %s", output)
		}
		if !strings.Contains(output, "connection_id=conn-456") {
			t.Errorf("Expected output to contain 'connection_id=conn-456', got: %s", output)
		}
	})

	t.Run("returns unchanged logger for empty context", func(t *testing.T) {
		var buf bytes.Buffer
		logger := New(slog.LevelInfo, &buf)

		ctx := context.Background()
		ctxLogger := logger.WithCorrelationContext(ctx)
		ctxLogger.Info("test message")

		output := buf.String()
		if strings.Contains(output, "correlation_id") {
			t.Errorf("Should not contain correlation_id for empty context, got: %s", output)
		}
		if strings.Contains(output, "connection_id") {
			t.Errorf("Should not contain connection_id for empty context, got: %s", output)
		}
	})
}

func TestLoggerConnection(t *testing.T) {
	var buf bytes.Buffer
	logger := New(slog.LevelInfo, &buf)

	connLogger := logger.Connection("conn-test-123")
	connLogger.Info("connection event")

	output := buf.String()
	if !strings.Contains(output, "connection_id=conn-test-123") {
		t.Errorf("Expected output to contain 'connection_id=conn-test-123', got: %s", output)
	}
}

func TestLoggerCorrelationID(t *testing.T) {
	var buf bytes.Buffer
	logger := New(slog.LevelInfo, &buf)

	corrLogger := logger.CorrelationID("corr-test-456")
	corrLogger.Info("correlated event")

	output := buf.String()
	if !strings.Contains(output, "correlation_id=corr-test-456") {
		t.Errorf("Expected output to contain 'correlation_id=corr-test-456', got: %s", output)
	}
}

func TestNewContextWithRequestID(t *testing.T) {
	t.Run("creates context with ID", func(t *testing.T) {
		ctx := context.Background()
		newCtx, id, err := NewContextWithRequestID(ctx)

		if err != nil {
			t.Fatalf("NewContextWithRequestID() returned error: %v", err)
		}
		if id == "" {
			t.Error("NewContextWithRequestID() returned empty ID")
		}
		if len(id) != 16 {
			t.Errorf("NewContextWithRequestID() returned ID of length %d, want 16", len(id))
		}

		// Verify the ID is in the context
		retrievedID := GetCorrelationID(newCtx)
		if retrievedID != id {
			t.Errorf("GetCorrelationID() = %q, want %q", retrievedID, id)
		}
	})

	t.Run("preserves parent context values", func(t *testing.T) {
		type testKey string
		ctx := context.WithValue(context.Background(), testKey("key"), "value")
		newCtx, _, err := NewContextWithRequestID(ctx)

		if err != nil {
			t.Fatalf("NewContextWithRequestID() returned error: %v", err)
		}

		// Original value should still be accessible
		if v := newCtx.Value(testKey("key")); v != "value" {
			t.Errorf("Parent context value lost, got %v", v)
		}
	})
}

func TestCombinedLogging(t *testing.T) {
	var buf bytes.Buffer
	logger := New(slog.LevelInfo, &buf)

	// Simulate a real-world logging scenario with multiple IDs
	ctx := context.Background()
	ctx = WithCorrelationID(ctx, "req-abc")
	ctx = WithConnectionID(ctx, "conn-123")

	// Create a logger with all context and additional attributes
	fullLogger := logger.
		WithCorrelationContext(ctx).
		Component("socks").
		Circuit(12345).
		Stream(42)

	fullLogger.Info("processing request",
		"address", "example.onion",
		"port", 80)

	output := buf.String()

	expectedFields := []string{
		"correlation_id=req-abc",
		"connection_id=conn-123",
		"component=socks",
		"circuit_id=12345",
		"stream_id=42",
		"address=example.onion",
		"port=80",
	}

	for _, field := range expectedFields {
		if !strings.Contains(output, field) {
			t.Errorf("Expected output to contain %q, got: %s", field, output)
		}
	}
}
