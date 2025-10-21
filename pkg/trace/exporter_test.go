package trace

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestNoopExporter(t *testing.T) {
	exporter := NewNoopExporter()

	span := &Span{
		TraceID:   "trace-123",
		SpanID:    "span-123",
		Name:      "test-operation",
		StartTime: time.Now(),
	}

	err := exporter.Export(span)
	if err != nil {
		t.Errorf("Expected no error from noop exporter, got %v", err)
	}

	err = exporter.Close()
	if err != nil {
		t.Errorf("Expected no error from noop close, got %v", err)
	}
}

func TestStdoutExporter(t *testing.T) {
	exporter := NewStdoutExporter(false)

	span := &Span{
		TraceID:    "trace-123",
		SpanID:     "span-123",
		Name:       "test-operation",
		StartTime:  time.Now(),
		Attributes: make(map[string]interface{}),
		Events:     make([]Event, 0),
	}

	// Note: This will print to stdout during test, but we can't easily capture it
	// without redirecting stdout, which is complex in tests
	err := exporter.Export(span)
	if err != nil {
		t.Errorf("Expected no error from stdout exporter, got %v", err)
	}

	err = exporter.Close()
	if err != nil {
		t.Errorf("Expected no error from stdout close, got %v", err)
	}
}

func TestFileExporter(t *testing.T) {
	// Create temp file
	tmpfile, err := os.CreateTemp("", "trace-test-*.json")
	if err != nil {
		t.Fatal(err)
	}
	filename := tmpfile.Name()
	tmpfile.Close()
	defer os.Remove(filename)

	exporter, err := NewFileExporter(filename, false)
	if err != nil {
		t.Fatalf("Failed to create file exporter: %v", err)
	}
	defer exporter.Close()

	span := &Span{
		TraceID:    "trace-123",
		SpanID:     "span-123",
		Name:       "test-operation",
		StartTime:  time.Now(),
		Attributes: make(map[string]interface{}),
		Events:     make([]Event, 0),
	}

	err = exporter.Export(span)
	if err != nil {
		t.Errorf("Expected no error from file exporter, got %v", err)
	}

	err = exporter.Close()
	if err != nil {
		t.Errorf("Expected no error from file close, got %v", err)
	}

	// Verify file contains span data
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("Failed to read trace file: %v", err)
	}

	if len(data) == 0 {
		t.Error("Expected file to contain span data")
	}

	// Verify JSON is valid
	var parsedSpan Span
	if err := json.Unmarshal(bytes.TrimSpace(data), &parsedSpan); err != nil {
		t.Errorf("Failed to parse span JSON: %v", err)
	}

	if parsedSpan.TraceID != "trace-123" {
		t.Errorf("Expected trace ID 'trace-123', got '%s'", parsedSpan.TraceID)
	}
}

func TestFileExporterPretty(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "trace-test-pretty-*.json")
	if err != nil {
		t.Fatal(err)
	}
	filename := tmpfile.Name()
	tmpfile.Close()
	defer os.Remove(filename)

	exporter, err := NewFileExporter(filename, true)
	if err != nil {
		t.Fatalf("Failed to create file exporter: %v", err)
	}
	defer exporter.Close()

	span := &Span{
		TraceID:    "trace-123",
		SpanID:     "span-123",
		Name:       "test-operation",
		StartTime:  time.Now(),
		Attributes: make(map[string]interface{}),
		Events:     make([]Event, 0),
	}

	err = exporter.Export(span)
	if err != nil {
		t.Errorf("Expected no error from file exporter, got %v", err)
	}

	exporter.Close()

	// Verify file contains pretty-printed JSON
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("Failed to read trace file: %v", err)
	}

	// Pretty printed JSON should contain newlines and spaces
	if !bytes.Contains(data, []byte("\n")) {
		t.Error("Expected pretty-printed JSON to contain newlines")
	}
}

func TestWriterExporter(t *testing.T) {
	buf := &bytes.Buffer{}
	exporter := NewWriterExporter(buf, false)

	span := &Span{
		TraceID:    "trace-123",
		SpanID:     "span-123",
		Name:       "test-operation",
		StartTime:  time.Now(),
		Attributes: make(map[string]interface{}),
		Events:     make([]Event, 0),
	}

	err := exporter.Export(span)
	if err != nil {
		t.Errorf("Expected no error from writer exporter, got %v", err)
	}

	err = exporter.Close()
	if err != nil {
		t.Errorf("Expected no error from writer close, got %v", err)
	}

	// Verify buffer contains data
	if buf.Len() == 0 {
		t.Error("Expected buffer to contain span data")
	}

	// Verify JSON is valid
	var parsedSpan Span
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &parsedSpan); err != nil {
		t.Errorf("Failed to parse span JSON: %v", err)
	}

	if parsedSpan.TraceID != "trace-123" {
		t.Errorf("Expected trace ID 'trace-123', got '%s'", parsedSpan.TraceID)
	}
}

func TestMultiExporter(t *testing.T) {
	buf1 := &bytes.Buffer{}
	buf2 := &bytes.Buffer{}

	exp1 := NewWriterExporter(buf1, false)
	exp2 := NewWriterExporter(buf2, false)

	multiExporter := NewMultiExporter(exp1, exp2)

	span := &Span{
		TraceID:    "trace-123",
		SpanID:     "span-123",
		Name:       "test-operation",
		StartTime:  time.Now(),
		Attributes: make(map[string]interface{}),
		Events:     make([]Event, 0),
	}

	err := multiExporter.Export(span)
	if err != nil {
		t.Errorf("Expected no error from multi exporter, got %v", err)
	}

	err = multiExporter.Close()
	if err != nil {
		t.Errorf("Expected no error from multi close, got %v", err)
	}

	// Verify both buffers contain data
	if buf1.Len() == 0 {
		t.Error("Expected first buffer to contain span data")
	}

	if buf2.Len() == 0 {
		t.Error("Expected second buffer to contain span data")
	}

	// Verify both contain the same span
	var span1, span2 Span
	if err := json.Unmarshal(bytes.TrimSpace(buf1.Bytes()), &span1); err != nil {
		t.Errorf("Failed to parse span from buffer 1: %v", err)
	}
	if err := json.Unmarshal(bytes.TrimSpace(buf2.Bytes()), &span2); err != nil {
		t.Errorf("Failed to parse span from buffer 2: %v", err)
	}

	if span1.TraceID != span2.TraceID {
		t.Error("Expected both buffers to contain same trace ID")
	}
}

func TestExportNilSpan(t *testing.T) {
	exporters := []Exporter{
		NewNoopExporter(),
		NewStdoutExporter(false),
		NewWriterExporter(&bytes.Buffer{}, false),
	}

	for _, exp := range exporters {
		err := exp.Export(nil)
		if err != nil {
			t.Errorf("Expected no error exporting nil span, got %v", err)
		}
	}
}
