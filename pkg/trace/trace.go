// Package trace provides distributed tracing and observability for the Tor client.
// This package enables end-to-end tracking of operations across circuit building,
// stream management, and connection handling for production debugging and analysis.
package trace

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// SpanKind represents the type of span
type SpanKind string

const (
	// SpanKindClient represents a client-side operation
	SpanKindClient SpanKind = "client"
	// SpanKindServer represents a server-side operation
	SpanKindServer SpanKind = "server"
	// SpanKindInternal represents an internal operation
	SpanKindInternal SpanKind = "internal"
)

// SpanStatus represents the completion status of a span
type SpanStatus string

const (
	// StatusOK indicates successful completion
	StatusOK SpanStatus = "ok"
	// StatusError indicates an error occurred
	StatusError SpanStatus = "error"
	// StatusCancelled indicates the operation was cancelled
	StatusCancelled SpanStatus = "cancelled"
)

type contextKey int

const (
	spanContextKey contextKey = iota
)

// Span represents a single operation in a trace
type Span struct {
	TraceID    string
	SpanID     string
	ParentID   string
	Name       string
	Kind       SpanKind
	StartTime  time.Time
	EndTime    time.Time
	Duration   time.Duration
	Status     SpanStatus
	Attributes map[string]interface{}
	Events     []Event
	mu         sync.RWMutex
}

// Event represents a timestamped event within a span
type Event struct {
	Timestamp  time.Time
	Name       string
	Attributes map[string]interface{}
}

// Tracer provides tracing functionality
type Tracer struct {
	serviceName string
	exporter    Exporter
	sampler     Sampler
	mu          sync.RWMutex
}

// Exporter defines the interface for exporting spans
type Exporter interface {
	Export(span *Span) error
	Close() error
}

// Sampler determines whether to create a trace
type Sampler interface {
	ShouldSample(name string) bool
}

// NewTracer creates a new tracer instance
func NewTracer(serviceName string, exporter Exporter, sampler Sampler) *Tracer {
	if sampler == nil {
		sampler = AlwaysSample()
	}
	return &Tracer{
		serviceName: serviceName,
		exporter:    exporter,
		sampler:     sampler,
	}
}

// StartSpan creates a new span and adds it to the context
func (t *Tracer) StartSpan(ctx context.Context, name string, kind SpanKind) (context.Context, *Span) {
	// Check if we should sample this trace
	if !t.sampler.ShouldSample(name) {
		return ctx, nil
	}

	span := &Span{
		TraceID:    generateID(),
		SpanID:     generateID(),
		Name:       name,
		Kind:       kind,
		StartTime:  time.Now(),
		Status:     StatusOK,
		Attributes: make(map[string]interface{}),
		Events:     make([]Event, 0),
	}

	// Check if there's a parent span in the context
	if parentSpan := FromContext(ctx); parentSpan != nil {
		span.TraceID = parentSpan.TraceID
		span.ParentID = parentSpan.SpanID
	}

	// Add service name as attribute
	span.SetAttribute("service.name", t.serviceName)

	// Add span to context
	return context.WithValue(ctx, spanContextKey, span), span
}

// End completes the span and exports it
func (s *Span) End() {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.EndTime = time.Now()
	s.Duration = s.EndTime.Sub(s.StartTime)
}

// SetStatus sets the span status
func (s *Span) SetStatus(status SpanStatus, description string) {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.Status = status
	if description != "" {
		s.Attributes["status.description"] = description
	}
}

// SetAttribute sets a span attribute
func (s *Span) SetAttribute(key string, value interface{}) {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.Attributes[key] = value
}

// SetAttributes sets multiple span attributes at once
func (s *Span) SetAttributes(attrs map[string]interface{}) {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for k, v := range attrs {
		s.Attributes[k] = v
	}
}

// AddEvent adds a timestamped event to the span
func (s *Span) AddEvent(name string, attributes map[string]interface{}) {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	event := Event{
		Timestamp:  time.Now(),
		Name:       name,
		Attributes: attributes,
	}
	s.Events = append(s.Events, event)
}

// RecordError records an error in the span
func (s *Span) RecordError(err error) {
	if s == nil || err == nil {
		return
	}

	s.SetStatus(StatusError, err.Error())
	s.AddEvent("error", map[string]interface{}{
		"error.type":    fmt.Sprintf("%T", err),
		"error.message": err.Error(),
	})
}

// FromContext retrieves the span from context
func FromContext(ctx context.Context) *Span {
	if ctx == nil {
		return nil
	}
	span, _ := ctx.Value(spanContextKey).(*Span)
	return span
}

// generateID generates a unique identifier for traces and spans
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// Helper function to end span with error handling
func EndSpan(span *Span, err error, exporter Exporter) {
	if span == nil {
		return
	}

	if err != nil {
		span.RecordError(err)
	}

	span.End()

	if exporter != nil {
		_ = exporter.Export(span)
	}
}

// Helper function to add span to context with automatic cleanup
func WithSpan(ctx context.Context, tracer *Tracer, name string, kind SpanKind, fn func(context.Context, *Span) error) error {
	ctx, span := tracer.StartSpan(ctx, name, kind)
	defer func() {
		span.End()
		if tracer.exporter != nil {
			_ = tracer.exporter.Export(span)
		}
	}()

	err := fn(ctx, span)
	if err != nil {
		span.RecordError(err)
	}

	return err
}
