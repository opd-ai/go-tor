# Distributed Tracing Guide

This guide explains how to use the distributed tracing capabilities in go-tor for production debugging and observability.

## Overview

Distributed tracing provides end-to-end visibility into operations across the Tor client, enabling:
- **Performance analysis**: Identify slow operations and bottlenecks
- **Error debugging**: Track errors through complex operation chains
- **Operation flow**: Visualize how operations relate to each other
- **Production insights**: Understand system behavior in real-world conditions

## Quick Start

### Basic Usage

```go
import (
    "context"
    "github.com/opd-ai/go-tor/pkg/trace"
)

// Create a tracer
exporter := trace.NewStdoutExporter(true)
sampler := trace.AlwaysSample()
tracer := trace.NewTracer("go-tor", exporter, sampler)

// Start a span
ctx := context.Background()
ctx, span := tracer.StartSpan(ctx, "build-circuit", trace.SpanKindInternal)
defer func() {
    span.End()
    exporter.Export(span)
}()

// Add metadata
span.SetAttribute("circuit.id", 12345)

// Do work...
circuit, err := buildCircuit()
if err != nil {
    span.RecordError(err)
}
```

### With Automatic Cleanup

```go
err := trace.WithSpan(ctx, tracer, "operation", trace.SpanKindInternal, 
    func(ctx context.Context, span *trace.Span) error {
        // Your code here
        span.SetAttribute("key", "value")
        return doWork()
    })
```

## Core Concepts

### Span

A span represents a single operation with timing, status, and metadata:

```go
type Span struct {
    TraceID    string                 // Groups related operations
    SpanID     string                 // Unique operation identifier
    ParentID   string                 // Parent operation (if any)
    Name       string                 // Operation name
    Kind       SpanKind              // Client, server, or internal
    StartTime  time.Time             // When operation started
    EndTime    time.Time             // When operation ended
    Duration   time.Duration         // How long it took
    Status     SpanStatus            // ok, error, or cancelled
    Attributes map[string]interface{} // Metadata
    Events     []Event               // Timestamped events
}
```

### Span Hierarchy

Spans can have parent-child relationships to represent operation flows:

```go
// Parent span
ctx, parentSpan := tracer.StartSpan(ctx, "build-circuit", trace.SpanKindInternal)
defer parentSpan.End()

// Child spans inherit trace ID
ctx, childSpan1 := tracer.StartSpan(ctx, "select-guard", trace.SpanKindInternal)
childSpan1.End()

ctx, childSpan2 := tracer.StartSpan(ctx, "extend-circuit", trace.SpanKindInternal)
childSpan2.End()
```

All child spans share the same TraceID and reference their parent via ParentID.

### Span Kinds

Three types of spans:

- **SpanKindClient**: Outbound operations (e.g., connecting to relay)
- **SpanKindServer**: Inbound operations (e.g., handling SOCKS connection)
- **SpanKindInternal**: Internal operations (e.g., selecting path)

```go
// Client operation
ctx, span := tracer.StartSpan(ctx, "connect-relay", trace.SpanKindClient)

// Server operation
ctx, span := tracer.StartSpan(ctx, "handle-socks", trace.SpanKindServer)

// Internal operation
ctx, span := tracer.StartSpan(ctx, "select-path", trace.SpanKindInternal)
```

## Adding Metadata

### Attributes

Key-value pairs attached to spans:

```go
// Single attribute
span.SetAttribute("circuit.id", 12345)
span.SetAttribute("relay.fingerprint", "AAAA1111")

// Multiple attributes
span.SetAttributes(map[string]interface{}{
    "circuit.hops": 3,
    "circuit.type": "general",
    "bandwidth":    "10 Mbps",
})
```

### Events

Timestamped events within a span:

```go
span.AddEvent("guard-selected", map[string]interface{}{
    "relay.id":       "guard-1",
    "selection.time": "5ms",
})

span.AddEvent("circuit-extended", map[string]interface{}{
    "hop": 2,
})
```

### Error Recording

Record errors with automatic status update:

```go
circuit, err := buildCircuit()
if err != nil {
    span.RecordError(err)
    // Automatically:
    // - Sets status to StatusError
    // - Adds error event with type and message
}
```

## Exporters

Exporters determine where traces are sent.

### Stdout Exporter

Print traces to stdout (development):

```go
exporter := trace.NewStdoutExporter(true) // true = pretty print
tracer := trace.NewTracer("go-tor", exporter, sampler)
```

### File Exporter

Write traces to a file (production):

```go
exporter, err := trace.NewFileExporter("/var/log/tor/traces.json", false)
if err != nil {
    log.Fatal(err)
}
defer exporter.Close()

tracer := trace.NewTracer("go-tor", exporter, sampler)
```

### Writer Exporter

Write traces to any io.Writer:

```go
var buf bytes.Buffer
exporter := trace.NewWriterExporter(&buf, false)
tracer := trace.NewTracer("go-tor", exporter, sampler)
```

### Multi Exporter

Send traces to multiple destinations:

```go
stdout := trace.NewStdoutExporter(false)
file, _ := trace.NewFileExporter("/var/log/tor/traces.json", false)

multi := trace.NewMultiExporter(stdout, file)
tracer := trace.NewTracer("go-tor", multi, sampler)
```

### Noop Exporter

Discard all traces (testing):

```go
exporter := trace.NewNoopExporter()
tracer := trace.NewTracer("go-tor", exporter, sampler)
```

## Sampling

Samplers control which operations are traced to manage overhead.

### Always Sample

Trace everything (development/debugging):

```go
sampler := trace.AlwaysSample()
```

### Never Sample

Trace nothing (default production):

```go
sampler := trace.NeverSample()
```

### Probability Sample

Trace N% of operations:

```go
sampler := trace.ProbabilitySample(0.01) // 1% of operations
```

Use for:
- Production with low overhead
- Statistical sampling of operations
- Load-proportional tracing

### Rate Limit Sample

Trace up to N operations per second:

```go
sampler := trace.RateLimitSample(100) // Max 100 traces/second
```

Use for:
- Bounded overhead
- High-volume operations
- Fixed trace budget

## Integration Examples

### Circuit Building

```go
ctx, span := tracer.StartSpan(ctx, "build-circuit", trace.SpanKindInternal)
defer func() {
    span.End()
    exporter.Export(span)
}()

span.SetAttribute("circuit.purpose", "general")

// Select guard
ctx, guardSpan := tracer.StartSpan(ctx, "select-guard", trace.SpanKindInternal)
guard, err := selectGuard(ctx)
if err != nil {
    guardSpan.RecordError(err)
    guardSpan.End()
    return err
}
guardSpan.SetAttribute("relay.fingerprint", guard.Fingerprint)
guardSpan.End()

// Extend circuit
ctx, extendSpan := tracer.StartSpan(ctx, "extend-circuit", trace.SpanKindInternal)
err = extendCircuit(ctx, guard)
if err != nil {
    extendSpan.RecordError(err)
}
extendSpan.End()

span.SetAttribute("circuit.id", circuit.ID)
```

### Stream Operations

```go
ctx, span := tracer.StartSpan(ctx, "create-stream", trace.SpanKindInternal)
defer func() {
    span.End()
    exporter.Export(span)
}()

span.SetAttribute("stream.circuit_id", circuitID)
span.SetAttribute("stream.target", "example.com:443")

stream, err := createStream(ctx, circuitID)
if err != nil {
    span.RecordError(err)
    return err
}

span.SetAttribute("stream.id", stream.ID)
span.AddEvent("stream-connected", map[string]interface{}{
    "time_to_connect": "150ms",
})
```

### Connection Handling

```go
ctx, span := tracer.StartSpan(ctx, "connect-relay", trace.SpanKindClient)
defer func() {
    span.End()
    exporter.Export(span)
}()

span.SetAttribute("relay.address", "1.2.3.4:9001")
span.SetAttribute("relay.fingerprint", fingerprint)

conn, err := net.DialTimeout("tcp", address, timeout)
if err != nil {
    span.RecordError(err)
    return err
}

span.AddEvent("tcp-connected", nil)

// TLS handshake
span.AddEvent("tls-handshake-start", nil)
tlsConn := tls.Client(conn, config)
if err := tlsConn.Handshake(); err != nil {
    span.RecordError(err)
    return err
}
span.AddEvent("tls-handshake-complete", map[string]interface{}{
    "tls.version": tlsConn.ConnectionState().Version,
})
```

## Production Configuration

### Recommended Setup

For production environments:

```go
// File export with rotation support
exporter, err := trace.NewFileExporter("/var/log/tor/traces.json", false)
if err != nil {
    log.Fatal(err)
}

// Low sampling rate (1%)
sampler := trace.ProbabilitySample(0.01)

tracer := trace.NewTracer("go-tor", exporter, sampler)
```

### Performance Considerations

- **Sampling overhead**: ~1µs per operation when not sampled
- **Span overhead**: ~1KB memory per span with typical attributes
- **Export overhead**: Asynchronous, minimal impact on operations
- **File I/O**: Buffered writes for efficiency

### Best Practices

1. **Always defer span.End()**: Ensures proper cleanup
   ```go
   ctx, span := tracer.StartSpan(ctx, "operation", trace.SpanKindInternal)
   defer span.End()
   ```

2. **Use context propagation**: Pass context through all operations
   ```go
   func doWork(ctx context.Context) error {
       ctx, span := tracer.StartSpan(ctx, "work", trace.SpanKindInternal)
       defer span.End()
       // ...
   }
   ```

3. **Add meaningful attributes**: Help with debugging
   ```go
   span.SetAttribute("circuit.id", id)
   span.SetAttribute("relay.fingerprint", fp)
   ```

4. **Record all errors**: Automatic error tracking
   ```go
   if err != nil {
       span.RecordError(err)
       return err
   }
   ```

5. **Use appropriate sampling**: Balance visibility vs overhead
   ```go
   // Development: trace everything
   sampler := trace.AlwaysSample()
   
   // Production: sample 1%
   sampler := trace.ProbabilitySample(0.01)
   ```

## Troubleshooting

### No traces appearing

Check:
1. Sampler is not NeverSample
2. Exporter is configured correctly
3. span.End() is being called
4. Exporter.Export() is being called

### High overhead

Solutions:
1. Reduce sampling rate
2. Use rate-limited sampler
3. Reduce attribute count
4. Use simpler exporter (file vs multi)

### Missing spans

Ensure:
1. Context is propagated through all operations
2. Parent span exists in context before creating child
3. Spans are ended in correct order (child before parent)

## Related Documentation

- [API Documentation](API.md) - Complete API reference
- [Metrics](METRICS.md) - Complementary metrics system
- [Health Monitoring](../pkg/health/README.md) - Health check integration
- [Examples](../examples/trace-demo/) - Complete working examples

## Future Enhancements

Potential future improvements:
- OpenTelemetry protocol support
- Distributed tracing across processes
- Trace visualization tools
- Integration with APM systems
- Automatic context propagation
