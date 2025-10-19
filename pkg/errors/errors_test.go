package errors

import (
	"errors"
	"fmt"
	"testing"
)

func TestNew(t *testing.T) {
	err := New(CategoryConnection, SeverityMedium, "test error")
	if err == nil {
		t.Fatal("New returned nil")
	}
	if err.Category != CategoryConnection {
		t.Errorf("Expected category %s, got %s", CategoryConnection, err.Category)
	}
	if err.Severity != SeverityMedium {
		t.Errorf("Expected severity %s, got %s", SeverityMedium, err.Severity)
	}
	if err.Message != "test error" {
		t.Errorf("Expected message 'test error', got '%s'", err.Message)
	}
	if err.Retryable {
		t.Error("Expected non-retryable error")
	}
}

func TestWrap(t *testing.T) {
	underlying := fmt.Errorf("underlying error")
	err := Wrap(CategoryCircuit, SeverityHigh, "wrapped error", underlying)

	if err.Underlying == nil {
		t.Error("Expected underlying error to be set")
	}
	if !errors.Is(err, underlying) {
		t.Error("Wrapped error should unwrap to underlying error")
	}
}

func TestNewRetryable(t *testing.T) {
	err := NewRetryable(CategoryTimeout, SeverityMedium, "timeout error")
	if !err.Retryable {
		t.Error("Expected retryable error")
	}
}

func TestError(t *testing.T) {
	tests := []struct {
		name     string
		err      *TorError
		contains string
	}{
		{
			name:     "simple error",
			err:      New(CategoryConnection, SeverityLow, "connection failed"),
			contains: "[connection:low] connection failed",
		},
		{
			name:     "wrapped error",
			err:      Wrap(CategoryCircuit, SeverityHigh, "circuit error", fmt.Errorf("underlying")),
			contains: "[circuit:high] circuit error: underlying",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errStr := tt.err.Error()
			if errStr != tt.contains {
				t.Errorf("Expected error string to contain '%s', got '%s'", tt.contains, errStr)
			}
		})
	}
}

func TestWithContext(t *testing.T) {
	err := New(CategoryConnection, SeverityMedium, "test")
	err.WithContext("address", "127.0.0.1:9050")
	err.WithContext("attempt", 3)

	if err.Context == nil {
		t.Fatal("Context not initialized")
	}
	if err.Context["address"] != "127.0.0.1:9050" {
		t.Error("Context 'address' not set correctly")
	}
	if err.Context["attempt"] != 3 {
		t.Error("Context 'attempt' not set correctly")
	}
}

func TestIs(t *testing.T) {
	err1 := New(CategoryConnection, SeverityMedium, "error1")
	err2 := New(CategoryConnection, SeverityHigh, "error2")
	err3 := New(CategoryCircuit, SeverityMedium, "error3")

	if !errors.Is(err1, err2) {
		t.Error("Errors with same category should match with Is")
	}
	if errors.Is(err1, err3) {
		t.Error("Errors with different categories should not match")
	}
}

func TestConnectionError(t *testing.T) {
	underlying := fmt.Errorf("network error")
	err := ConnectionError("failed to connect", underlying)

	if err.Category != CategoryConnection {
		t.Errorf("Expected category %s, got %s", CategoryConnection, err.Category)
	}
	if !err.Retryable {
		t.Error("Connection errors should be retryable")
	}
}

func TestCircuitError(t *testing.T) {
	err := CircuitError("circuit build failed", nil)
	if err.Category != CategoryCircuit {
		t.Errorf("Expected category %s, got %s", CategoryCircuit, err.Category)
	}
	if !err.Retryable {
		t.Error("Circuit errors should be retryable")
	}
}

func TestProtocolError(t *testing.T) {
	err := ProtocolError("invalid cell", nil)
	if err.Category != CategoryProtocol {
		t.Errorf("Expected category %s, got %s", CategoryProtocol, err.Category)
	}
	if err.Retryable {
		t.Error("Protocol errors should not be retryable")
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "retryable error",
			err:      NewRetryable(CategoryTimeout, SeverityMedium, "timeout"),
			expected: true,
		},
		{
			name:     "non-retryable error",
			err:      New(CategoryProtocol, SeverityHigh, "protocol error"),
			expected: false,
		},
		{
			name:     "standard error",
			err:      fmt.Errorf("standard error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetryable(tt.err)
			if result != tt.expected {
				t.Errorf("Expected IsRetryable to return %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGetCategory(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected ErrorCategory
	}{
		{
			name:     "tor error",
			err:      New(CategoryCircuit, SeverityMedium, "test"),
			expected: CategoryCircuit,
		},
		{
			name:     "standard error",
			err:      fmt.Errorf("standard error"),
			expected: CategoryInternal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetCategory(tt.err)
			if result != tt.expected {
				t.Errorf("Expected category %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestGetSeverity(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected Severity
	}{
		{
			name:     "tor error",
			err:      New(CategoryCircuit, SeverityCritical, "test"),
			expected: SeverityCritical,
		},
		{
			name:     "standard error",
			err:      fmt.Errorf("standard error"),
			expected: SeverityMedium,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetSeverity(tt.err)
			if result != tt.expected {
				t.Errorf("Expected severity %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestIsCategory(t *testing.T) {
	err := New(CategoryConnection, SeverityMedium, "test")

	if !IsCategory(err, CategoryConnection) {
		t.Error("Expected IsCategory to return true for matching category")
	}
	if IsCategory(err, CategoryCircuit) {
		t.Error("Expected IsCategory to return false for non-matching category")
	}

	stdErr := fmt.Errorf("standard error")
	if IsCategory(stdErr, CategoryConnection) {
		t.Error("Expected IsCategory to return false for standard error")
	}
}

func TestAllErrorConstructors(t *testing.T) {
	tests := []struct {
		name         string
		constructor  func() *TorError
		category     ErrorCategory
		shouldRetry  bool
	}{
		{
			name:        "ConnectionError",
			constructor: func() *TorError { return ConnectionError("test", nil) },
			category:    CategoryConnection,
			shouldRetry: true,
		},
		{
			name:        "CircuitError",
			constructor: func() *TorError { return CircuitError("test", nil) },
			category:    CategoryCircuit,
			shouldRetry: true,
		},
		{
			name:        "DirectoryError",
			constructor: func() *TorError { return DirectoryError("test", nil) },
			category:    CategoryDirectory,
			shouldRetry: true,
		},
		{
			name:        "ProtocolError",
			constructor: func() *TorError { return ProtocolError("test", nil) },
			category:    CategoryProtocol,
			shouldRetry: false,
		},
		{
			name:        "CryptoError",
			constructor: func() *TorError { return CryptoError("test", nil) },
			category:    CategoryCrypto,
			shouldRetry: false,
		},
		{
			name:        "ConfigurationError",
			constructor: func() *TorError { return ConfigurationError("test", nil) },
			category:    CategoryConfiguration,
			shouldRetry: false,
		},
		{
			name:        "TimeoutError",
			constructor: func() *TorError { return TimeoutError("test", nil) },
			category:    CategoryTimeout,
			shouldRetry: true,
		},
		{
			name:        "NetworkError",
			constructor: func() *TorError { return NetworkError("test", nil) },
			category:    CategoryNetwork,
			shouldRetry: true,
		},
		{
			name:        "InternalError",
			constructor: func() *TorError { return InternalError("test", nil) },
			category:    CategoryInternal,
			shouldRetry: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.constructor()
			if err.Category != tt.category {
				t.Errorf("Expected category %s, got %s", tt.category, err.Category)
			}
			if err.Retryable != tt.shouldRetry {
				t.Errorf("Expected retryable %v, got %v", tt.shouldRetry, err.Retryable)
			}
		})
	}
}
