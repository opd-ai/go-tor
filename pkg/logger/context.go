// Package logger provides structured logging for the Tor client.
// This file implements context utilities for correlation ID tracking across
// distributed operations.
package logger

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// CorrelationIDKey is the context key for correlation ID storage.
// Using a typed key prevents collisions with other context values.
const CorrelationIDKey contextKey = "correlation_id"

// ConnectionIDKey is the context key for connection ID storage.
const ConnectionIDKey contextKey = "connection_id"

// GenerateRequestID creates a cryptographically random 16-character request ID.
// Returns the ID and an error if random generation fails.
// The generated ID is suitable for use as a correlation ID in distributed tracing.
func GenerateRequestID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate request ID: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// WithCorrelationID returns a new context with the correlation ID attached.
// The correlation ID is used to trace requests across multiple operations
// and log entries for debugging and observability.
func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, CorrelationIDKey, id)
}

// GetCorrelationID retrieves the correlation ID from the context.
// Returns an empty string if no correlation ID is set.
func GetCorrelationID(ctx context.Context) string {
	if id, ok := ctx.Value(CorrelationIDKey).(string); ok {
		return id
	}
	return ""
}

// WithConnectionID returns a new context with the connection ID attached.
// The connection ID identifies a specific network connection for debugging.
func WithConnectionID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ConnectionIDKey, id)
}

// GetConnectionID retrieves the connection ID from the context.
// Returns an empty string if no connection ID is set.
func GetConnectionID(ctx context.Context) string {
	if id, ok := ctx.Value(ConnectionIDKey).(string); ok {
		return id
	}
	return ""
}

// WithCorrelationContext returns a new Logger with the correlation ID from
// the context added as an attribute. If no correlation ID is in the context,
// returns the logger unchanged.
func (l *Logger) WithCorrelationContext(ctx context.Context) *Logger {
	result := l
	if corrID := GetCorrelationID(ctx); corrID != "" {
		result = result.With("correlation_id", corrID)
	}
	if connID := GetConnectionID(ctx); connID != "" {
		result = result.With("connection_id", connID)
	}
	return result
}

// Connection returns a new Logger with a connection_id attribute.
// This is useful for tracking operations related to a specific connection.
func (l *Logger) Connection(id string) *Logger {
	return l.With("connection_id", id)
}

// CorrelationID returns a new Logger with a correlation_id attribute.
// This is useful for tracing requests across multiple operations.
func (l *Logger) CorrelationID(id string) *Logger {
	return l.With("correlation_id", id)
}

// NewContextWithRequestID creates a new context with a generated correlation ID.
// Returns the context and the generated ID, or an error if ID generation fails.
// This is a convenience function for initializing request contexts.
func NewContextWithRequestID(ctx context.Context) (context.Context, string, error) {
	id, err := GenerateRequestID()
	if err != nil {
		return ctx, "", err
	}
	return WithCorrelationID(ctx, id), id, nil
}
