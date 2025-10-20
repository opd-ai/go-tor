# Development Guide

## Getting Started

### Prerequisites

- Go 1.24 or later
- Git
- (Optional) Docker for testing

### Building

```bash
# Clone the repository
git clone https://github.com/opd-ai/go-tor.git
cd go-tor

# Build the client
go build -o bin/tor-client ./cmd/tor-client

# Run tests
go test ./...

# Run with coverage
go test -cover ./...
```

### Running

```bash
# Run with default settings
./bin/tor-client

# Run with custom ports
./bin/tor-client -socks-port 9150 -control-port 9151

# Run with custom data directory
./bin/tor-client -data-dir /tmp/tor-data

# Show version
./bin/tor-client -version
```

## Project Structure

```
go-tor/
├── cmd/
│   └── tor-client/          # Main executable
│       └── main.go
├── pkg/                      # Public packages
│   ├── cell/                 # Cell encoding/decoding
│   ├── circuit/              # Circuit management
│   ├── config/               # Configuration
│   ├── control/              # Control protocol
│   ├── crypto/               # Cryptographic primitives
│   ├── directory/            # Directory protocol
│   ├── onion/                # Onion services
│   ├── path/                 # Path selection
│   ├── protocol/             # Core protocol
│   └── socks/                # SOCKS5 proxy
├── internal/                 # Internal packages
├── examples/                 # Example applications
│   ├── socks-proxy/
│   └── onion-service/
├── docs/                     # Documentation
└── go.mod
```

## Coding Standards

### Go Style
- Follow [Effective Go](https://golang.org/doc/effective_go.html)
- Use `gofmt` for formatting
- Use `golint` for linting
- Use `go vet` for static analysis

### Naming Conventions
- Packages: lowercase, single word
- Types: PascalCase
- Functions/methods: PascalCase (exported) or camelCase (unexported)
- Constants: PascalCase or SCREAMING_SNAKE_CASE for exported constants

### Comments
- All exported types, functions, and constants must have comments
- Comments should be complete sentences
- Package comments go in `doc.go` or the main package file

### Error Handling
- Always check errors
- Wrap errors with context using `fmt.Errorf("context: %w", err)`
- Use `errors.Is()` and `errors.As()` for error inspection

## Testing

### Unit Tests
```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests with race detector
go test -race ./...

# Run specific package tests
go test ./pkg/cell/...
```

### Writing Tests
- Test files should be named `*_test.go`
- Use table-driven tests where appropriate
- Test both success and error cases
- Aim for >80% code coverage

Example:
```go
func TestSomething(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        {"valid", "input", "output", false},
        {"invalid", "", "", true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := SomeFunction(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
            }
            if got != tt.want {
                t.Errorf("got %v, want %v", got, tt.want)
            }
        })
    }
}
```

## Documentation

### GoDoc
All public APIs should be documented with GoDoc comments:

```go
// Cell represents a Tor protocol cell.
// A cell consists of a circuit ID, command, and payload.
type Cell struct {
    CircID  uint32  // Circuit ID
    Command Command // Cell command
    Payload []byte  // Cell payload
}

// NewCell creates a new cell with the given circuit ID and command.
// The payload is initialized to an empty slice.
func NewCell(circID uint32, cmd Command) *Cell {
    // ...
}
```

### Markdown Documentation
- Architecture overview: `docs/ARCHITECTURE.md`
- Development guide: `docs/DEVELOPMENT.md` (this file)
- User guide: `docs/USER_GUIDE.md`
- Security considerations: `docs/SECURITY.md`

## Contributing

### Workflow
1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Add tests for your changes
5. Ensure all tests pass (`go test ./...`)
6. Commit your changes (`git commit -m 'Add amazing feature'`)
7. Push to the branch (`git push origin feature/amazing-feature`)
8. Open a Pull Request

### Pull Request Guidelines
- Include tests for new functionality
- Update documentation as needed
- Ensure all tests pass
- Follow existing code style
- Keep commits focused and atomic

## Debugging

### Logging
```go
import "log"

log.Printf("Circuit %d: state changed to %s", circID, state)
```

### Debugging with Delve
```bash
# Install delve
go install github.com/go-delve/delve/cmd/dlv@latest

# Run with debugger
dlv debug ./cmd/tor-client
```

## Performance Profiling

### CPU Profiling
```bash
go test -cpuprofile=cpu.prof ./pkg/...
go tool pprof cpu.prof
```

### Memory Profiling
```bash
go test -memprofile=mem.prof ./pkg/...
go tool pprof mem.prof
```

### Benchmarking
```go
func BenchmarkCellEncode(b *testing.B) {
    cell := NewCell(12345, CmdCreate)
    var buf bytes.Buffer
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        buf.Reset()
        cell.Encode(&buf)
    }
}
```

Run benchmarks:
```bash
go test -bench=. ./pkg/cell/...
```

## Tools

### Useful Go Tools
- `gofmt`: Format code
- `golint`: Lint code
- `go vet`: Static analysis
- `staticcheck`: Advanced static analysis
- `goreleaser`: Build releases

### Install Tools
```bash
go install golang.org/x/lint/golint@latest
go install honnef.co/go/tools/cmd/staticcheck@latest
```

## References

### Tor Protocol
- [Tor Specifications](https://spec.torproject.org/)
- [Tor Project Git](https://github.com/torproject/tor)

### Go Resources
- [Effective Go](https://golang.org/doc/effective_go.html)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Go Proverbs](https://go-proverbs.github.io/)
