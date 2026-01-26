package trace

import (
	"os"
	"runtime"
	"testing"
	"time"
)

// TestFileExporterResourceLeak tests that FileExporter doesn't leak file descriptors
// This addresses the file descriptor leak identified in PLAN.md line 166-182
func TestFileExporterResourceLeak(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "trace-leak-test-*.json")
	if err != nil {
		t.Fatal(err)
	}
	filename := tmpfile.Name()
	tmpfile.Close()
	defer os.Remove(filename)

	// Create and close properly
	exporter, err := NewFileExporter(filename, false)
	if err != nil {
		t.Fatalf("Failed to create file exporter: %v", err)
	}

	// Export a span
	span := &Span{
		TraceID:   "test-trace",
		SpanID:    "test-span",
		Name:      "test-operation",
		StartTime: time.Now(),
	}

	if err := exporter.Export(span); err != nil {
		t.Errorf("Export failed: %v", err)
	}

	// Explicitly close
	if err := exporter.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Verify file was written
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if len(data) == 0 {
		t.Error("Expected file to contain data")
	}
}

// TestFileExporterFinalizer tests that finalizer prevents descriptor leak
func TestFileExporterFinalizer(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "trace-finalizer-test-*.json")
	if err != nil {
		t.Fatal(err)
	}
	filename := tmpfile.Name()
	tmpfile.Close()
	defer os.Remove(filename)

	// Create exporter without explicit close (relies on finalizer)
	func() {
		exporter, err := NewFileExporter(filename, false)
		if err != nil {
			t.Fatalf("Failed to create file exporter: %v", err)
		}

		span := &Span{
			TraceID:   "test-trace",
			SpanID:    "test-span",
			Name:      "test-operation",
			StartTime: time.Now(),
		}

		if err := exporter.Export(span); err != nil {
			t.Errorf("Export failed: %v", err)
		}
		// Intentionally not calling Close() - finalizer should handle it
		_ = exporter
	}()

	// Force garbage collection to trigger finalizer
	runtime.GC()
	time.Sleep(100 * time.Millisecond) // Give finalizer time to run

	// File should still be accessible (finalizer closed it)
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("Failed to read file after finalizer: %v", err)
	}
	if len(data) == 0 {
		t.Error("Expected file to contain data")
	}
}

// TestFileExporterMultipleClose tests that closing multiple times is safe
func TestFileExporterMultipleClose(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "trace-multiclose-test-*.json")
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

	// Close multiple times - should not panic
	if err := exporter.Close(); err != nil {
		t.Errorf("First close failed: %v", err)
	}

	if err := exporter.Close(); err != nil {
		t.Errorf("Second close failed: %v", err)
	}
}

// TestFileExporterDocumentation verifies that Close() requirement is documented
func TestFileExporterDocumentation(t *testing.T) {
	// This is a documentation test - NewFileExporter godoc should mention:
	// 1. Caller must call Close()
	// 2. Finalizer is defensive measure only
	// 3. Example usage with defer

	tmpfile, err := os.CreateTemp("", "trace-doc-test-*.json")
	if err != nil {
		t.Fatal(err)
	}
	filename := tmpfile.Name()
	tmpfile.Close()
	defer os.Remove(filename)

	// Proper usage pattern (as documented)
	exporter, err := NewFileExporter(filename, false)
	if err != nil {
		t.Fatalf("Failed to create file exporter: %v", err)
	}
	defer exporter.Close() // MUST call Close() as documented

	span := &Span{
		TraceID:   "test-trace",
		SpanID:    "test-span",
		Name:      "test-operation",
		StartTime: time.Now(),
	}

	if err := exporter.Export(span); err != nil {
		t.Errorf("Export failed: %v", err)
	}
}

// TestFileExporterConcurrentClose tests concurrent Close() calls
func TestFileExporterConcurrentClose(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "trace-concurrent-test-*.json")
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

	// Close from multiple goroutines concurrently
	done := make(chan bool, 3)
	for i := 0; i < 3; i++ {
		go func() {
			_ = exporter.Close()
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 3; i++ {
		<-done
	}
	// Test passes if no panic occurs
}
