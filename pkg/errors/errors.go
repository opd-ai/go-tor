// Package errors provides structured error types for the Tor client.
// This package defines error categories and types for better error handling and diagnostics.
package errors

import (
	"errors"
	"fmt"
)

// ErrorCategory represents the category of an error
type ErrorCategory string

const (
	// CategoryConnection indicates a connection-related error
	CategoryConnection ErrorCategory = "connection"
	// CategoryCircuit indicates a circuit-related error
	CategoryCircuit ErrorCategory = "circuit"
	// CategoryDirectory indicates a directory-related error
	CategoryDirectory ErrorCategory = "directory"
	// CategoryProtocol indicates a protocol-related error
	CategoryProtocol ErrorCategory = "protocol"
	// CategoryCrypto indicates a cryptography-related error
	CategoryCrypto ErrorCategory = "crypto"
	// CategoryConfiguration indicates a configuration-related error
	CategoryConfiguration ErrorCategory = "configuration"
	// CategoryTimeout indicates a timeout error
	CategoryTimeout ErrorCategory = "timeout"
	// CategoryNetwork indicates a network-related error
	CategoryNetwork ErrorCategory = "network"
	// CategoryInternal indicates an internal error
	CategoryInternal ErrorCategory = "internal"
)

// Severity represents the severity level of an error
type Severity string

const (
	// SeverityLow indicates a low-severity error (recoverable)
	SeverityLow Severity = "low"
	// SeverityMedium indicates a medium-severity error (may degrade service)
	SeverityMedium Severity = "medium"
	// SeverityHigh indicates a high-severity error (service disruption likely)
	SeverityHigh Severity = "high"
	// SeverityCritical indicates a critical error (service unavailable)
	SeverityCritical Severity = "critical"
)

// TorError represents a structured error with additional context
type TorError struct {
	Category   ErrorCategory
	Severity   Severity
	Message    string
	Underlying error
	Retryable  bool
	Context    map[string]interface{}
}

// Error implements the error interface
func (e *TorError) Error() string {
	if e.Underlying != nil {
		return fmt.Sprintf("[%s:%s] %s: %v", e.Category, e.Severity, e.Message, e.Underlying)
	}
	return fmt.Sprintf("[%s:%s] %s", e.Category, e.Severity, e.Message)
}

// Unwrap returns the underlying error
func (e *TorError) Unwrap() error {
	return e.Underlying
}

// Is implements error comparison
func (e *TorError) Is(target error) bool {
	t, ok := target.(*TorError)
	if !ok {
		return false
	}
	return e.Category == t.Category
}

// WithContext adds context to the error
func (e *TorError) WithContext(key string, value interface{}) *TorError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// New creates a new TorError
func New(category ErrorCategory, severity Severity, message string) *TorError {
	return &TorError{
		Category:  category,
		Severity:  severity,
		Message:   message,
		Retryable: false,
	}
}

// Wrap wraps an existing error with TorError
func Wrap(category ErrorCategory, severity Severity, message string, err error) *TorError {
	return &TorError{
		Category:   category,
		Severity:   severity,
		Message:    message,
		Underlying: err,
		Retryable:  false,
	}
}

// NewRetryable creates a new retryable TorError
func NewRetryable(category ErrorCategory, severity Severity, message string) *TorError {
	return &TorError{
		Category:  category,
		Severity:  severity,
		Message:   message,
		Retryable: true,
	}
}

// WrapRetryable wraps an existing error with a retryable TorError
func WrapRetryable(category ErrorCategory, severity Severity, message string, err error) *TorError {
	return &TorError{
		Category:   category,
		Severity:   severity,
		Message:    message,
		Underlying: err,
		Retryable:  true,
	}
}

// Common error constructors

// ConnectionError creates a connection error
func ConnectionError(message string, err error) *TorError {
	return WrapRetryable(CategoryConnection, SeverityMedium, message, err)
}

// CircuitError creates a circuit error
func CircuitError(message string, err error) *TorError {
	return WrapRetryable(CategoryCircuit, SeverityMedium, message, err)
}

// DirectoryError creates a directory error
func DirectoryError(message string, err error) *TorError {
	return WrapRetryable(CategoryDirectory, SeverityMedium, message, err)
}

// ProtocolError creates a protocol error
func ProtocolError(message string, err error) *TorError {
	return Wrap(CategoryProtocol, SeverityHigh, message, err)
}

// CryptoError creates a cryptography error
func CryptoError(message string, err error) *TorError {
	return Wrap(CategoryCrypto, SeverityHigh, message, err)
}

// ConfigurationError creates a configuration error
func ConfigurationError(message string, err error) *TorError {
	return Wrap(CategoryConfiguration, SeverityCritical, message, err)
}

// TimeoutError creates a timeout error
func TimeoutError(message string, err error) *TorError {
	return WrapRetryable(CategoryTimeout, SeverityMedium, message, err)
}

// NetworkError creates a network error
func NetworkError(message string, err error) *TorError {
	return WrapRetryable(CategoryNetwork, SeverityMedium, message, err)
}

// InternalError creates an internal error
func InternalError(message string, err error) *TorError {
	return Wrap(CategoryInternal, SeverityHigh, message, err)
}

// IsRetryable checks if an error is retryable
func IsRetryable(err error) bool {
	var torErr *TorError
	if errors.As(err, &torErr) {
		return torErr.Retryable
	}
	return false
}

// GetCategory returns the error category
func GetCategory(err error) ErrorCategory {
	var torErr *TorError
	if errors.As(err, &torErr) {
		return torErr.Category
	}
	return CategoryInternal
}

// GetSeverity returns the error severity
func GetSeverity(err error) Severity {
	var torErr *TorError
	if errors.As(err, &torErr) {
		return torErr.Severity
	}
	return SeverityMedium
}

// IsCategory checks if an error belongs to a specific category
func IsCategory(err error, category ErrorCategory) bool {
	var torErr *TorError
	if errors.As(err, &torErr) {
		return torErr.Category == category
	}
	return false
}
