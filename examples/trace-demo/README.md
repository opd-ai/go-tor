# Distributed Tracing Demo

This example demonstrates the distributed tracing capabilities in go-tor, showing how to track operations end-to-end for debugging and analysis in production environments.

## Features Demonstrated

1. **Basic Tracing**: Creating spans and tracking operation duration
2. **Span Hierarchy**: Parent-child relationships between operations
3. **Error Recording**: Capturing and recording errors in spans
4. **Events and Attributes**: Adding metadata to spans
5. **Sampling Strategies**: Different approaches to controlling trace volume
6. **File Export**: Exporting traces to files for analysis
7. **Circuit Building**: Realistic example of tracing Tor circuit operations

## Running the Demo

```bash
cd examples/trace-demo
go run main.go
```

## Output

The demo will show:
- Span creation with trace and span IDs
- Parent-child span relationships with timing
- Error recording in spans
- Event and attribute logging
- Different sampling strategies in action
- File export with trace data
- Simulated circuit building with nested operations

## Key Concepts

### Span

A span represents a single operation with:
- **TraceID**: Groups related operations together
- **SpanID**: Unique identifier for this operation
- **ParentID**: Links to parent operation (if any)
- **Attributes**: Key-value metadata
- **Events**: Timestamped events within the operation
- **Status**: Success, error, or cancelled

### Tracer

The tracer creates and manages spans:
```go
tracer := trace.NewTracer("service-name", exporter, sampler)
ctx, span := tracer.StartSpan(ctx, "operation-name", trace.SpanKindInternal)
defer span.End()
```

### Exporter

Exporters send spans to different destinations:
- **NoopExporter**: Discards all spans (for testing)
- **StdoutExporter**: Prints spans to stdout
- **FileExporter**: Writes spans to a file
- **WriterExporter**: Writes spans to any io.Writer
- **MultiExporter**: Sends spans to multiple exporters

### Sampler

Samplers control which operations are traced:
- **AlwaysSample**: Traces everything (dev/debug)
- **NeverSample**: Traces nothing (production default)
- **ProbabilitySample**: Traces N% of operations
- **RateLimitSample**: Limits traces per second

## Integration with go-tor

The tracing package integrates with existing go-tor operations:

```go
// Circuit building
ctx, span := tracer.StartSpan(ctx, "build-circuit", trace.SpanKindInternal)
defer span.End()

circuit, err := circuitManager.CreateCircuit(ctx)
if err != nil {
    span.RecordError(err)
}
span.SetAttribute("circuit.id", circuit.ID)
```

## Best Practices

1. **Always defer span.End()**: Ensures spans are completed even on error
2. **Use context propagation**: Pass context through function calls
3. **Add meaningful attributes**: Help with debugging and analysis
4. **Record errors**: Use `span.RecordError(err)` for error tracking
5. **Use appropriate sampling**: Balance observability vs overhead
6. **Export to appropriate destination**: stdout for dev, file/service for prod

## Production Usage

For production environments:

```go
// Use file exporter with sampling
exporter, _ := trace.NewFileExporter("/var/log/tor/traces.json", false)
sampler := trace.ProbabilitySample(0.01) // 1% sampling
tracer := trace.NewTracer("go-tor", exporter, sampler)

// Or use rate limiting
sampler := trace.RateLimitSample(100) // Max 100 traces/sec
```

## Performance Impact

- **Overhead**: Minimal (<1µs per span when not sampled)
- **Memory**: ~1KB per span with typical attributes
- **Sampling**: Reduces overhead proportionally
- **Export**: Asynchronous, doesn't block operations

## Related Documentation

- [Phase 9.10 Report](../../docs/PHASE_9_10_REPORT.md) - Context propagation
- [Phase 9.1 Report](../../docs/METRICS.md) - Metrics and observability
- [Architecture](../../docs/ARCHITECTURE.md) - System architecture
