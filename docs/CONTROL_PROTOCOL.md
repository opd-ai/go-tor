# Control Protocol Documentation

## Overview

The go-tor client implements a subset of the Tor control protocol for monitoring and management. The control protocol allows external applications to query status, configure settings, and monitor events.

## Protocol Specification

The control protocol follows the [Tor Control Protocol Specification](https://spec.torproject.org/control-spec) and uses a text-based, line-oriented protocol over TCP.

## Connection

Default address: `127.0.0.1:9051`

Configure with the `-control-port` command-line option:
```bash
./bin/tor-client -control-port 9051
```

## Authentication

Currently, the implementation accepts any authentication (including no password) for development purposes. In production, authentication should be implemented using one of these methods:

- **NULL**: No authentication (current implementation)
- **HASHEDPASSWORD**: Password-based authentication (planned)
- **COOKIE**: Cookie file authentication (planned)

## Supported Commands

### PROTOCOLINFO

Get information about the control protocol version and authentication methods.

**Syntax:**
```
PROTOCOLINFO [version]
```

**Example:**
```
> PROTOCOLINFO
< 250-PROTOCOLINFO 1
< 250-AUTH METHODS=NULL
< 250-VERSION Tor="go-tor-0.1.0"
< 250 OK
```

### AUTHENTICATE

Authenticate to the control port. Currently accepts any authentication.

**Syntax:**
```
AUTHENTICATE [token]
```

**Example:**
```
> AUTHENTICATE
< 250 OK
```

### GETINFO

Query various information about the Tor client state.

**Syntax:**
```
GETINFO key [key ...]
```

**Supported Keys:**

| Key | Description | Example Value |
|-----|-------------|---------------|
| `version` | Client version | `go-tor 0.1.0` |
| `status/circuit-established` | Whether circuits are available | `0` or `1` |
| `status/enough-dir-info` | Whether directory info is available | `0` or `1` |
| `traffic/read` | Bytes read (placeholder) | `0` |
| `traffic/written` | Bytes written (placeholder) | `0` |

**Example:**
```
> GETINFO version status/circuit-established
< 250-version=go-tor 0.1.0
< 250 status/circuit-established=1
```

### GETCONF

Query configuration values.

**Syntax:**
```
GETCONF key [key ...]
```

**Example:**
```
> GETCONF SocksPort
< 250 SocksPort=
```

**Note:** Currently returns placeholder values. Full configuration introspection will be added in future updates.

### SETCONF

Set configuration values.

**Syntax:**
```
SETCONF key=value [key=value ...]
```

**Example:**
```
> SETCONF SocksPort=9150
< 250 OK
```

**Note:** Currently acknowledges but does not apply changes. Full configuration management will be added in future updates.

### SETEVENTS

Subscribe to asynchronous event notifications.

**Syntax:**
```
SETEVENTS [event ...]
```

**Supported Events (planned):**
- `CIRC` - Circuit status changes
- `STREAM` - Stream status changes
- `ORCONN` - OR connection status
- `BW` - Bandwidth usage
- `NEWDESC` - New relay descriptors
- `GUARD` - Guard node changes

**Example:**
```
> SETEVENTS CIRC STREAM
< 250 OK
```

**Note:** Event subscriptions are accepted but events are not yet published. Event generation will be added in future updates.

### QUIT

Close the control connection.

**Syntax:**
```
QUIT
```

**Example:**
```
> QUIT
< 250 closing connection
```

## Response Codes

The control protocol uses numeric response codes similar to SMTP:

| Code | Meaning |
|------|---------|
| `250` | OK - Command successful |
| `500` | Syntax error |
| `510` | Unrecognized command |
| `514` | Authentication required |
| `552` | Unrecognized key or invalid argument |

## Multi-line Responses

Responses can span multiple lines. All lines except the last use `code-` format, and the last line uses `code ` (space) format:

```
250-version=go-tor 0.1.0
250-status/circuit-established=1
250 status/enough-dir-info=1
```

## Example Session

Here's a complete example session:

```bash
$ telnet localhost 9051
Connected to localhost.
< 250 OK

> PROTOCOLINFO
< 250-PROTOCOLINFO 1
< 250-AUTH METHODS=NULL
< 250-VERSION Tor="go-tor-0.1.0"
< 250 OK

> AUTHENTICATE
< 250 OK

> GETINFO version status/circuit-established
< 250-version=go-tor 0.1.0
< 250 status/circuit-established=1

> SETEVENTS CIRC
< 250 OK

> QUIT
< 250 closing connection
Connection closed.
```

## Using with Tools

### netcat

```bash
(echo "AUTHENTICATE"; echo "GETINFO version"; echo "QUIT") | nc localhost 9051
```

### Python

```python
import socket

sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
sock.connect(('localhost', 9051))

# Read greeting
print(sock.recv(1024).decode())

# Authenticate
sock.send(b'AUTHENTICATE\r\n')
print(sock.recv(1024).decode())

# Get info
sock.send(b'GETINFO version\r\n')
print(sock.recv(1024).decode())

# Quit
sock.send(b'QUIT\r\n')
print(sock.recv(1024).decode())

sock.close()
```

### stem (Python Tor Control Library)

```python
from stem.control import Controller

with Controller.from_port(port=9051) as controller:
    controller.authenticate()
    
    version = controller.get_info("version")
    print(f"Version: {version}")
    
    circuits = controller.get_info("status/circuit-established")
    print(f"Circuits established: {circuits}")
```

## Implementation Status

| Feature | Status |
|---------|--------|
| Basic protocol server | ✅ Complete |
| PROTOCOLINFO command | ✅ Complete |
| AUTHENTICATE command | ✅ Complete (NULL auth only) |
| GETINFO command | ✅ Partial (core keys implemented) |
| GETCONF command | ✅ Placeholder |
| SETCONF command | ✅ Placeholder |
| SETEVENTS command | ✅ Subscription only (no events yet) |
| QUIT command | ✅ Complete |
| Password authentication | ⏳ Planned |
| Cookie authentication | ⏳ Planned |
| Event notifications | ⏳ Planned |
| Circuit management commands | ⏳ Planned |
| Configuration management | ⏳ Planned |

## Security Considerations

**Development Mode:** The current implementation accepts connections without authentication. This is suitable for:
- Development and testing
- Single-user systems
- Containerized deployments with network isolation

**Production Requirements:**
- Implement proper authentication (password or cookie)
- Use firewall rules to restrict access to `127.0.0.1`
- Consider TLS for remote connections
- Implement rate limiting

## Future Enhancements

### Phase 7.1 (Near-term)
- Event notification system (CIRC, STREAM, ORCONN)
- Extended GETINFO keys (circuit details, stream info)
- Full configuration management
- Password/cookie authentication

### Phase 7.2 (Medium-term)
- Circuit management commands (EXTENDCIRCUIT, CLOSECIRCUIT)
- Stream management commands
- Signal handling (NEWNYM, RELOAD, SHUTDOWN)
- Hidden service management (ADD_ONION, DEL_ONION)

### Phase 7.3 (Long-term)
- Control port over Unix domain socket
- TLS support for remote connections
- Rate limiting and DoS protection
- Audit logging

## References

- [Tor Control Protocol Specification](https://spec.torproject.org/control-spec)
- [Tor Control Port Usage](https://2019.www.torproject.org/docs/tor-manual.html.en#ControlPort)
- [stem - Python controller library](https://stem.torproject.org/)

## Support

For issues or questions about the control protocol:
- GitHub Issues: https://github.com/opd-ai/go-tor/issues
- Tag with `control-protocol` label
