# Onion Service State Persistence

This document describes the state persistence implementation for go-tor onion services, which allows services to maintain state across restarts.

## Overview

The state persistence system provides:
- Automatic state saving on service shutdown and descriptor publication
- Automatic state loading on service initialization
- Descriptor revision tracking for monotonically increasing versions
- Introduction point caching for optimization
- Service creation timestamp tracking

## Architecture

### Components

1. **ServiceState** (`pkg/onion/persistence.go`):
   - Stores serializable service state
   - Includes onion address, timestamps, descriptor revision, and intro point cache
   - Persisted as JSON in the data directory

2. **ServicePersistence** (`pkg/onion/persistence.go`):
   - Handles saving/loading of keys and state
   - Provides secure file operations with atomic writes
   - Manages file permissions (0600 for keys, 0600 for state)

3. **Service Integration** (`pkg/onion/service.go`):
   - Loads state on initialization if available
   - Saves state on Stop() and after descriptor publish
   - Tracks descriptor revision counter
   - Maintains creation timestamp across restarts

### State Structure

```go
type ServiceState struct {
    OnionAddress          string              // .onion address
    CreatedAt             time.Time           // Service creation time
    LastStarted           time.Time           // Last service start time
    IntroPointCache       []IntroPointState   // Cached intro points
    LastDescriptorPublish time.Time           // Last publish timestamp
    DescriptorRevision    uint64              // Descriptor revision counter
}

type IntroPointState struct {
    Fingerprint string    // Relay fingerprint
    AuthKeyHex  string    // Hex-encoded auth key
    EncKeyHex   string    // Hex-encoded encryption key
    CreatedAt   time.Time // Creation timestamp
}
```

## Usage

### Basic Usage

State persistence is automatically enabled when a `DataDirectory` is configured:

```go
config := &onion.ServiceConfig{
    DataDirectory:      "/var/lib/tor-go/hidden-service",
    NumIntroPoints:     3,
    DescriptorLifetime: 3 * time.Hour,
    Ports:              map[int]string{80: "localhost:8080"},
}

// First run: generates and saves new keys and initial state
service, err := onion.NewService(config, logger)
if err != nil {
    log.Fatal(err)
}

// Service runs...
service.Start(ctx, hsdirs)

// State saved on stop
service.Stop()

// Second run: loads existing keys and state
service2, err := onion.NewService(config, logger)
// service2 has same identity and descriptor revision counter
```

### State Lifecycle

1. **Initialization**:
   - If `DataDirectory` is set:
     - Load identity and ntor keys if they exist
     - Load state file if it exists
     - Restore descriptor revision counter and timestamps
   - If no state exists, initialize with defaults

2. **Runtime**:
   - Descriptor revision increments on each publish
   - State updated in memory

3. **Descriptor Publish**:
   - After successful publish, state is saved to disk
   - Captures current descriptor revision and timestamp

4. **Shutdown**:
   - `Stop()` triggers state save
   - Includes current intro point cache
   - Persists descriptor revision and timestamps

### Files Created

In the `DataDirectory`:
- `hs_ed25519_secret_key` - Ed25519 identity key (0600)
- `hs_ntor_secret_key` - Curve25519 ntor key (0600)
- `state.json` - Service state (0600)

### State File Format

```json
{
  "onion_address": "abc123...xyz.onion",
  "created_at": "2026-01-25T10:00:00Z",
  "last_started": "2026-01-25T12:00:00Z",
  "intro_points": [
    {
      "fingerprint": "relay1fingerprint",
      "auth_key": "deadbeef...",
      "enc_key": "cafebabe...",
      "created_at": "2026-01-25T10:00:30Z"
    }
  ],
  "last_descriptor_publish": "2026-01-25T12:00:05Z",
  "descriptor_revision": 42
}
```

## Descriptor Revision Tracking

The descriptor revision counter ensures monotonically increasing revisions across restarts:

1. **Initial Service**: Revision starts at 1
2. **Each Publish**: Revision increments by 1
3. **After Restart**: Revision continues from saved value
4. **Descriptor Creation**: Uses current revision counter

This prevents issues where:
- Restarted services publish descriptors with lower revisions
- HSDirs reject "stale" descriptors
- Clients may receive outdated service information

## Introduction Point Cache

The state includes a cache of established introduction points:

- **Purpose**: Optimization for service restart
- **Storage**: Only established intro points are cached
- **Format**: Fingerprint, auth key, enc key, creation time
- **Usage**: Currently logged for monitoring; future versions may reuse intro points

## Security Considerations

1. **File Permissions**:
   - All sensitive files are created with 0600 (owner read/write only)
   - Keys and state protected from other users

2. **Atomic Writes**:
   - State saved using temp file + rename
   - Prevents corruption on crash during write

3. **Data Validation**:
   - State loading includes error handling
   - Invalid state triggers fresh initialization with warning

4. **Secure Deletion**:
   - `SecureDelete()` overwrites files with random data (3 passes)
   - Prevents recovery from disk

## Configuration

### With Persistence

```go
config := &onion.ServiceConfig{
    DataDirectory:      "/path/to/data", // Enables persistence
    NumIntroPoints:     3,
    DescriptorLifetime: 3 * time.Hour,
    Ports:              map[int]string{80: "localhost:8080"},
}
```

### Without Persistence

```go
config := &onion.ServiceConfig{
    DataDirectory:      "", // Empty = no persistence
    NumIntroPoints:     3,
    DescriptorLifetime: 3 * time.Hour,
    Ports:              map[int]string{80: "localhost:8080"},
}
```

### With Provided Key (No Persistence)

```go
config := &onion.ServiceConfig{
    PrivateKey:         myPrivateKey, // Provided key = no persistence
    NumIntroPoints:     3,
    DescriptorLifetime: 3 * time.Hour,
    Ports:              map[int]string{80: "localhost:8080"},
}
```

## Testing

The state persistence system includes comprehensive tests:

- `TestServiceStatePersistence`: Basic save/load cycle
- `TestServiceStateRevisionIncrement`: Revision tracking
- `TestServiceStateIntroPointCache`: Intro point caching
- `TestServiceStateNoPersistence`: No-persistence mode
- `TestServiceStateWithProvidedKey`: Provided key mode
- `TestServiceStopSavesState`: State saved on stop
- `TestServiceStateCreationTime`: Creation time persistence

Run tests:
```bash
go test -v -run TestServiceState ./pkg/onion/
```

## Error Handling

State persistence failures are handled gracefully:

1. **Load Failure**: Service initializes with fresh state and logs warning
2. **Save Failure**: Service continues running, logs error
3. **Invalid State**: Service uses defaults and logs warning

This ensures service availability is not compromised by state persistence issues.

## Future Enhancements

Potential improvements to state persistence:

1. **Intro Point Reuse**: Reestablish circuits to cached intro points on restart
2. **State Encryption**: Encrypt state file at rest
3. **State Versioning**: Support migration between state format versions
4. **Backup/Restore**: API for backing up and restoring complete service state
5. **State Compression**: Compress state file for services with large caches

## References

- Task 9.4.2 in PLAN.md
- `pkg/onion/persistence.go` - Persistence implementation
- `pkg/onion/service.go` - Service integration
- `pkg/onion/service_state_test.go` - State persistence tests
