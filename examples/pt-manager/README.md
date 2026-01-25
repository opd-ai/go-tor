# PT Manager Example

This example demonstrates the pluggable transport manager with automatic PT discovery and restart functionality.

## Features Demonstrated

1. **PT Discovery**: Automatically finds installed PT binaries
2. **Multi-PT Management**: Manages multiple PTs simultaneously
3. **Auto-Restart**: Monitors PT processes and restarts on failure
4. **Graceful Shutdown**: Properly stops all PTs on exit

## Running the Example

```bash
# Build and run
go run examples/pt-manager/main.go

# Or build first
go build -o pt-manager examples/pt-manager/main.go
./pt-manager
```

## Requirements

For full functionality, install PT binaries:

```bash
# Ubuntu/Debian
apt-get install obfs4proxy

# macOS (with Homebrew)
brew install tor  # Includes PTs

# Or download from:
# https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/lyrebird
```

## What to Expect

The example will:

1. Search for common PT binaries (obfs4proxy, snowflake-client, etc.)
2. Create a PT manager with auto-restart enabled
3. Register discovered PTs
4. Start all PTs with health monitoring
5. Report PT status every 3 seconds
6. Gracefully shutdown on Ctrl+C

## Sample Output

```
=== Pluggable Transport Manager Demo ===

1. Discovering available PTs...
   Found 1 PTs:
   - obfs4proxy: /usr/bin/obfs4proxy

2. Creating PT manager...
   ✓ Manager created with auto-restart enabled

3. Registering PTs...
   ✓ Registered obfs4proxy client
   Total registered: 1 PTs

4. Starting PTs (monitoring enabled)...
   ✓ All PTs started successfully

5. Active PTs:
   ✓ obfs4 (methods: [obfs4])

6. PT usage example:
   To connect through obfs4:
   conn, err := client.Dial(ctx, "bridge.example.com:443")

7. Monitoring PTs (press Ctrl+C to exit)...
   - Manager will restart crashed PTs automatically
   - Max restarts: 3 per PT

   Status: 1/1 PTs running
   Status: 1/1 PTs running
   ^C
8. Shutting down...
   Stopping all PTs...
   ✓ All PTs stopped

=== Demo Complete ===
```

## Implementation Details

The example showcases:

- **DiscoverCommonPTs()**: Finds obfs4proxy, snowflake-client, etc.
- **Manager.AddClient()**: Registers PTs with configuration
- **Manager.StartAll()**: Launches all PTs with IPC handshake
- **Manager.GetClient()**: Retrieves specific PT for use
- **Auto-restart**: Manager monitors processes every 2s
- **Restart limits**: MaxRestarts=3 prevents infinite loops
- **Graceful shutdown**: Manager.Close() stops monitoring and terminates PTs

## Testing Without PT Binaries

If no PTs are installed, the demo will:
- Show empty discovery results
- Skip PT registration
- Display informative messages
- Exit gracefully

This demonstrates robust error handling.

## See Also

- [PLUGGABLE_TRANSPORTS.md](../../docs/PLUGGABLE_TRANSPORTS.md) - Full PT documentation
- [pt package](../../pkg/pt/) - Implementation source code
- [pt-spec.txt](https://spec.torproject.org/pt-spec) - Official specification
