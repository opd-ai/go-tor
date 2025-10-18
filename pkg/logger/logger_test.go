package logger

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	var buf bytes.Buffer
	logger := New(slog.LevelDebug, &buf)
	
	if logger == nil {
		t.Fatal("New() returned nil")
	}
	
	logger.Info("test message")
	output := buf.String()
	
	if !strings.Contains(output, "test message") {
		t.Errorf("Expected log output to contain 'test message', got: %s", output)
	}
}

func TestNewDefault(t *testing.T) {
	logger := NewDefault()
	if logger == nil {
		t.Fatal("NewDefault() returned nil")
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"error", slog.LevelError},
		{"unknown", slog.LevelInfo}, // defaults to info
	}
	
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			level, err := ParseLevel(tt.input)
			if err != nil {
				t.Errorf("ParseLevel(%q) returned error: %v", tt.input, err)
			}
			if level != tt.expected {
				t.Errorf("ParseLevel(%q) = %v, want %v", tt.input, level, tt.expected)
			}
		})
	}
}

func TestWithContext(t *testing.T) {
	logger := NewDefault()
	ctx := WithContext(context.Background(), logger)
	
	retrievedLogger := FromContext(ctx)
	if retrievedLogger != logger {
		t.Error("FromContext() did not return the same logger")
	}
}

func TestFromContextDefault(t *testing.T) {
	ctx := context.Background()
	logger := FromContext(ctx)
	
	if logger == nil {
		t.Fatal("FromContext() returned nil for context without logger")
	}
}

func TestWith(t *testing.T) {
	var buf bytes.Buffer
	logger := New(slog.LevelInfo, &buf)
	
	loggerWithAttrs := logger.With("key", "value")
	loggerWithAttrs.Info("test")
	
	output := buf.String()
	if !strings.Contains(output, "key=value") {
		t.Errorf("Expected output to contain 'key=value', got: %s", output)
	}
}

func TestComponent(t *testing.T) {
	var buf bytes.Buffer
	logger := New(slog.LevelInfo, &buf)
	
	componentLogger := logger.Component("circuit")
	componentLogger.Info("test message")
	
	output := buf.String()
	if !strings.Contains(output, "component=circuit") {
		t.Errorf("Expected output to contain 'component=circuit', got: %s", output)
	}
}

func TestCircuit(t *testing.T) {
	var buf bytes.Buffer
	logger := New(slog.LevelInfo, &buf)
	
	circuitLogger := logger.Circuit(12345)
	circuitLogger.Info("circuit event")
	
	output := buf.String()
	if !strings.Contains(output, "circuit_id=12345") {
		t.Errorf("Expected output to contain 'circuit_id=12345', got: %s", output)
	}
}

func TestStream(t *testing.T) {
	var buf bytes.Buffer
	logger := New(slog.LevelInfo, &buf)
	
	streamLogger := logger.Stream(42)
	streamLogger.Info("stream event")
	
	output := buf.String()
	if !strings.Contains(output, "stream_id=42") {
		t.Errorf("Expected output to contain 'stream_id=42', got: %s", output)
	}
}

func TestWithGroup(t *testing.T) {
	var buf bytes.Buffer
	logger := New(slog.LevelInfo, &buf)
	
	groupLogger := logger.WithGroup("network")
	groupLogger.Info("test", "bytes", 1024)
	
	output := buf.String()
	// WithGroup should nest attributes
	if !strings.Contains(output, "network.bytes=1024") {
		t.Errorf("Expected output to contain 'network.bytes=1024', got: %s", output)
	}
}

func TestLogLevels(t *testing.T) {
	tests := []struct {
		level   slog.Level
		logFunc func(*Logger, string)
		name    string
	}{
		{slog.LevelDebug, func(l *Logger, msg string) { l.Debug(msg) }, "Debug"},
		{slog.LevelInfo, func(l *Logger, msg string) { l.Info(msg) }, "Info"},
		{slog.LevelWarn, func(l *Logger, msg string) { l.Warn(msg) }, "Warn"},
		{slog.LevelError, func(l *Logger, msg string) { l.Error(msg) }, "Error"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := New(tt.level, &buf)
			tt.logFunc(logger, "test message")
			
			output := buf.String()
			if !strings.Contains(output, "test message") {
				t.Errorf("Expected output to contain 'test message', got: %s", output)
			}
		})
	}
}
