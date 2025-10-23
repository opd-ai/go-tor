# CLI Tools Demo

This example demonstrates the command-line tools available for managing and interacting with go-tor clients.

## Available Tools

### 1. torctl - Control Utility

A command-line tool for interacting with running go-tor clients via the control protocol.

**Features:**
- View client status and statistics
- List active circuits and streams
- Send control signals (SHUTDOWN, RELOAD, etc.)
- Query configuration values
- Real-time monitoring

**Usage:**
```bash
# Show status
torctl status

# List active circuits
torctl circuits

# List active streams
torctl streams

# Show detailed information
torctl info

# Send signal to client
torctl signal SHUTDOWN

# Use custom control port
torctl -control 127.0.0.1:9051 status

# Note: Configuration querying (config command) is currently limited
# Use 'info' command to see commonly used configuration values
```

### 2. tor-config-validator - Configuration Tool

A tool for validating existing configurations and generating sample configuration files.

**Features:**
- Validate torrc-compatible configuration files
- Generate sample configurations with defaults
- Verbose validation with detailed feedback
- Cross-platform path handling

**Usage:**
```bash
# Validate a configuration file
tor-config-validator -config /etc/tor/torrc

# Validate with verbose output
tor-config-validator -config myconfig.conf -verbose

# Generate sample configuration to stdout
tor-config-validator -generate

# Generate sample configuration to file
tor-config-validator -generate -output /tmp/sample-torrc
```

## Building the Tools

Build all CLI tools:
```bash
make build-tools
```

Or build individually:
```bash
make build-torctl
make build-config-validator
```

## Running the Demo

1. Build the tools:
   ```bash
   cd ../..
   make build-tools
   ```

2. Run the demo:
   ```bash
   cd examples/cli-tools-demo
   go run main.go
   ```

The demo will:
1. Generate and validate a sample configuration
2. Start a Tor client
3. Demonstrate torctl commands for monitoring the client

## Integration in Your Applications

### Using torctl in Scripts

```bash
#!/bin/bash
# Monitor Tor client health

while true; do
    if torctl status > /dev/null 2>&1; then
        echo "Tor client is running"
    else
        echo "Tor client is not responding"
        # Restart logic here
    fi
    sleep 30
done
```

### Configuration Management

```bash
#!/bin/bash
# Validate configuration before deployment

if tor-config-validator -config /etc/tor/torrc -verbose; then
    echo "Configuration is valid, restarting Tor..."
    torctl signal RELOAD
else
    echo "Configuration validation failed!"
    exit 1
fi
```

## Command Reference

### torctl Commands

| Command | Description | Example |
|---------|-------------|---------|
| status | Show current status | `torctl status` |
| circuits | List active circuits | `torctl circuits` |
| streams | List active streams | `torctl streams` |
| info | Show detailed information | `torctl info` |
| signal | Send control signal | `torctl signal SHUTDOWN` |
| version | Show client version | `torctl version` |

**Note**: Configuration querying (GETCONF) is currently limited. Use `info` command to see commonly used configuration values.

### tor-config-validator Options

| Option | Description | Example |
|--------|-------------|---------|
| -config | Validate config file | `-config /etc/tor/torrc` |
| -generate | Generate sample config | `-generate` |
| -output | Output file for generation | `-output sample.conf` |
| -verbose | Verbose output | `-verbose` |
| -version | Show version | `-version` |

## Requirements

- Go 1.24 or later
- Running go-tor client (for torctl)
- Appropriate file permissions for config files

## Notes

- torctl requires a running go-tor client with control protocol enabled
- Default control port is 9051
- Configuration validator supports torrc-compatible format
- All tools support -version flag for version information

## See Also

- [Control Protocol Documentation](../../docs/CONTROL_PROTOCOL.md)
- [Configuration Guide](../../docs/PRODUCTION.md)
- [API Reference](../../docs/API.md)
