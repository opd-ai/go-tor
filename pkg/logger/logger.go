// Package logger provides structured logging for the Tor client.
// It uses Go's standard log/slog package for structured logging with context support.
package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
)

// Logger wraps slog.Logger to provide application-specific logging functionality
type Logger struct {
	*slog.Logger
}

// contextKey is the type for context keys used by this package
type contextKey string

const loggerKey contextKey = "logger"

// New creates a new Logger with the specified level and output writer
func New(level slog.Level, w io.Writer) *Logger {
	opts := &slog.HandlerOptions{
		Level: level,
	}
	handler := slog.NewTextHandler(w, opts)
	return &Logger{
		Logger: slog.New(handler),
	}
}

// NewDefault creates a logger with default settings (Info level, stdout)
func NewDefault() *Logger {
	return New(slog.LevelInfo, os.Stdout)
}

// ParseLevel parses a string log level into slog.Level
func ParseLevel(level string) (slog.Level, error) {
	switch level {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, nil
	}
}

// WithContext returns a new context with the logger attached
func WithContext(ctx context.Context, logger *Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// FromContext retrieves the logger from the context, or returns a default logger
func FromContext(ctx context.Context) *Logger {
	if logger, ok := ctx.Value(loggerKey).(*Logger); ok {
		return logger
	}
	return NewDefault()
}

// With returns a new Logger with additional attributes
func (l *Logger) With(args ...any) *Logger {
	return &Logger{
		Logger: l.Logger.With(args...),
	}
}

// WithGroup returns a new Logger with a group name
func (l *Logger) WithGroup(name string) *Logger {
	return &Logger{
		Logger: l.Logger.WithGroup(name),
	}
}

// Component returns a new Logger with a "component" attribute
func (l *Logger) Component(name string) *Logger {
	return l.With("component", name)
}

// Circuit returns a new Logger with circuit information
func (l *Logger) Circuit(id uint32) *Logger {
	return l.With("circuit_id", id)
}

// Stream returns a new Logger with stream information
func (l *Logger) Stream(id uint16) *Logger {
	return l.With("stream_id", id)
}
