package trace

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
)

// NoopExporter is an exporter that does nothing
type NoopExporter struct{}

// Export does nothing
func (e *NoopExporter) Export(span *Span) error {
	return nil
}

// Close does nothing
func (e *NoopExporter) Close() error {
	return nil
}

// NewNoopExporter creates a new noop exporter
func NewNoopExporter() *NoopExporter {
	return &NoopExporter{}
}

// StdoutExporter exports spans to stdout
type StdoutExporter struct {
	mu     sync.Mutex
	pretty bool
}

// Export writes the span to stdout
func (e *StdoutExporter) Export(span *Span) error {
	if span == nil {
		return nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	var data []byte
	var err error

	if e.pretty {
		data, err = json.MarshalIndent(span, "", "  ")
	} else {
		data, err = json.Marshal(span)
	}

	if err != nil {
		return fmt.Errorf("failed to marshal span: %w", err)
	}

	fmt.Println(string(data))
	return nil
}

// Close does nothing for stdout
func (e *StdoutExporter) Close() error {
	return nil
}

// NewStdoutExporter creates a new stdout exporter
func NewStdoutExporter(pretty bool) *StdoutExporter {
	return &StdoutExporter{
		pretty: pretty,
	}
}

// FileExporter exports spans to a file
type FileExporter struct {
	file   *os.File
	mu     sync.Mutex
	pretty bool
}

// Export writes the span to the file
func (e *FileExporter) Export(span *Span) error {
	if span == nil {
		return nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	var data []byte
	var err error

	if e.pretty {
		data, err = json.MarshalIndent(span, "", "  ")
	} else {
		data, err = json.Marshal(span)
	}

	if err != nil {
		return fmt.Errorf("failed to marshal span: %w", err)
	}

	_, err = e.file.Write(append(data, '\n'))
	if err != nil {
		return fmt.Errorf("failed to write span: %w", err)
	}

	return nil
}

// Close closes the file
func (e *FileExporter) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.file != nil {
		return e.file.Close()
	}
	return nil
}

// NewFileExporter creates a new file exporter
func NewFileExporter(filename string, pretty bool) (*FileExporter, error) {
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("failed to open trace file: %w", err)
	}

	return &FileExporter{
		file:   file,
		pretty: pretty,
	}, nil
}

// WriterExporter exports spans to an io.Writer
type WriterExporter struct {
	writer io.Writer
	mu     sync.Mutex
	pretty bool
}

// Export writes the span to the writer
func (e *WriterExporter) Export(span *Span) error {
	if span == nil {
		return nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	var data []byte
	var err error

	if e.pretty {
		data, err = json.MarshalIndent(span, "", "  ")
	} else {
		data, err = json.Marshal(span)
	}

	if err != nil {
		return fmt.Errorf("failed to marshal span: %w", err)
	}

	_, err = e.writer.Write(append(data, '\n'))
	if err != nil {
		return fmt.Errorf("failed to write span: %w", err)
	}

	return nil
}

// Close does nothing for writer
func (e *WriterExporter) Close() error {
	return nil
}

// NewWriterExporter creates a new writer exporter
func NewWriterExporter(writer io.Writer, pretty bool) *WriterExporter {
	return &WriterExporter{
		writer: writer,
		pretty: pretty,
	}
}

// MultiExporter exports to multiple exporters
type MultiExporter struct {
	exporters []Exporter
	mu        sync.RWMutex
}

// Export exports to all exporters
func (e *MultiExporter) Export(span *Span) error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var firstErr error
	for _, exp := range e.exporters {
		if err := exp.Export(span); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Close closes all exporters
func (e *MultiExporter) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	var firstErr error
	for _, exp := range e.exporters {
		if err := exp.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// NewMultiExporter creates a new multi-exporter
func NewMultiExporter(exporters ...Exporter) *MultiExporter {
	return &MultiExporter{
		exporters: exporters,
	}
}
