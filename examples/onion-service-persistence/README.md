# Onion Service Key Persistence Example

This example demonstrates how to create an onion service that persists its cryptographic keys across restarts, ensuring the same `.onion` address is maintained.

## Overview

When hosting an onion service, it's critical that the service maintains the same address across restarts. This is achieved by persisting the Ed25519 identity key and Curve25519 ntor key to disk.

## Features Demonstrated

- **Automatic Key Generation**: On first run, generates new cryptographic keys
- **Key Persistence**: Saves keys to disk with secure permissions (0600)
- **Key Loading**: On subsequent runs, loads existing keys from disk
- **Stable Address**: The same `.onion` address is used across all runs

## Usage

```bash
# Run the example
go run examples/onion-service-persistence/main.go
```

On first run, you'll see output like:
```
Data directory: /home/user/.go-tor-example
Generating new service keys
Identity key saved
Ntor key saved
========================================
Onion Service Address: abc123...xyz.onion
========================================
This address will remain the same across restarts!
```

On subsequent runs:
```
Data directory: /home/user/.go-tor-example
Loading existing service keys from storage
Identity key loaded
Ntor key loaded
========================================
Onion Service Address: abc123...xyz.onion  # Same address!
========================================
```

## Key Files

The following files are created in the data directory (`~/.go-tor-example`):

- `hs_ed25519_secret_key` - Ed25519 identity private key (64 bytes, hex-encoded)
- `hs_ntor_secret_key` - Curve25519 ntor private key (32 bytes, hex-encoded)

Both files have strict permissions (0600 - owner read/write only) for security.

## Security Considerations

1. **File Permissions**: Keys are stored with mode 0600 (owner-only access)
2. **Directory Permissions**: Data directory is created with mode 0700
3. **Backup**: Keys can be exported for backup using the persistence API
4. **Secure Delete**: Use `SecureDelete()` to overwrite keys before deletion

## API Usage

```go
// Create service with persistence
config := &onion.ServiceConfig{
    DataDirectory: "/path/to/data",  // Keys saved/loaded here
    Ports: map[int]string{
        80: "localhost:8080",
    },
}

service, err := onion.NewService(config, logger)
if err != nil {
    log.Fatal(err)
}

// Service automatically:
// - Generates and saves keys on first run
// - Loads existing keys on subsequent runs

address := service.GetAddress()
fmt.Println("Onion address:", address)
```

## Advanced: Manual Key Management

```go
// Create persistence handler
persistence, err := onion.NewServicePersistence(dataDir, logger)

// Export keys for backup
identityKey, ntorKey, err := persistence.ExportKeys()

// Import keys (e.g., from backup)
err = persistence.ImportKeys(identityKey, ntorKey)

// Secure delete (3-pass random overwrite)
err = persistence.SecureDelete()
```

## Implementation Details

The persistence implementation follows these principles:

- **Atomic Writes**: State files use temp file + rename for atomicity
- **Version Format**: Keys stored with version prefix for future compatibility
- **Error Recovery**: Robust error handling with detailed error messages
- **Testing**: >85% test coverage with comprehensive edge case testing

See `pkg/onion/persistence.go` for full implementation details.
