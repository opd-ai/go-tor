// Package circuit provides circuit isolation functionality.
package circuit

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// IsolationLevel defines the level of circuit isolation
type IsolationLevel int

const (
	// IsolationNone disables isolation (legacy mode, default for backward compatibility)
	IsolationNone IsolationLevel = iota
	// IsolationDestination isolates circuits by destination (host:port)
	IsolationDestination
	// IsolationCredential isolates circuits by SOCKS5 username
	IsolationCredential
	// IsolationPort isolates circuits by client source port
	IsolationPort
	// IsolationSession isolates circuits by explicit session token
	IsolationSession
)

// String returns a string representation of the isolation level
func (l IsolationLevel) String() string {
	switch l {
	case IsolationNone:
		return "none"
	case IsolationDestination:
		return "destination"
	case IsolationCredential:
		return "credential"
	case IsolationPort:
		return "port"
	case IsolationSession:
		return "session"
	default:
		return fmt.Sprintf("unknown(%d)", l)
	}
}

// ParseIsolationLevel parses a string into an IsolationLevel
func ParseIsolationLevel(s string) (IsolationLevel, error) {
	switch strings.ToLower(s) {
	case "none":
		return IsolationNone, nil
	case "destination":
		return IsolationDestination, nil
	case "credential", "credentials":
		return IsolationCredential, nil
	case "port":
		return IsolationPort, nil
	case "session":
		return IsolationSession, nil
	default:
		return IsolationNone, fmt.Errorf("invalid isolation level: %s", s)
	}
}

// IsolationKey represents the key used to isolate circuits
// Different streams with different isolation keys will not share circuits
type IsolationKey struct {
	Level        IsolationLevel // The isolation level being used
	Destination  string         // host:port for destination isolation
	Credentials  string         // SOCKS5 username for credential isolation (hashed)
	SourcePort   uint16         // client port for port isolation
	SessionToken string         // explicit isolation token (hashed)
}

// NewIsolationKey creates a new isolation key with the specified level
func NewIsolationKey(level IsolationLevel) *IsolationKey {
	return &IsolationKey{
		Level: level,
	}
}

// WithDestination sets the destination for the isolation key
func (k *IsolationKey) WithDestination(dest string) *IsolationKey {
	k.Destination = dest
	return k
}

// WithCredentials sets the credentials for the isolation key (hashed for privacy)
func (k *IsolationKey) WithCredentials(username string) *IsolationKey {
	// Hash the username to avoid storing PII in memory
	if username != "" {
		hash := sha256.Sum256([]byte(username))
		k.Credentials = hex.EncodeToString(hash[:])
	}
	return k
}

// WithSourcePort sets the source port for the isolation key
func (k *IsolationKey) WithSourcePort(port uint16) *IsolationKey {
	k.SourcePort = port
	return k
}

// WithSessionToken sets the session token for the isolation key (hashed for privacy)
func (k *IsolationKey) WithSessionToken(token string) *IsolationKey {
	// Hash the token to avoid storing sensitive data in memory
	if token != "" {
		hash := sha256.Sum256([]byte(token))
		k.SessionToken = hex.EncodeToString(hash[:])
	}
	return k
}

// String returns a string representation of the isolation key
// This is used for circuit pool lookups
func (k *IsolationKey) String() string {
	if k == nil || k.Level == IsolationNone {
		return "none"
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("level=%s", k.Level))

	switch k.Level {
	case IsolationDestination:
		if k.Destination != "" {
			parts = append(parts, fmt.Sprintf("dest=%s", k.Destination))
		}
	case IsolationCredential:
		if k.Credentials != "" {
			// Only show first 8 chars of hash for debugging
			shortHash := k.Credentials
			if len(shortHash) > 8 {
				shortHash = shortHash[:8] + "..."
			}
			parts = append(parts, fmt.Sprintf("creds=%s", shortHash))
		}
	case IsolationPort:
		if k.SourcePort != 0 {
			parts = append(parts, fmt.Sprintf("port=%d", k.SourcePort))
		}
	case IsolationSession:
		if k.SessionToken != "" {
			// Only show first 8 chars of hash for debugging
			shortHash := k.SessionToken
			if len(shortHash) > 8 {
				shortHash = shortHash[:8] + "..."
			}
			parts = append(parts, fmt.Sprintf("session=%s", shortHash))
		}
	}

	return strings.Join(parts, ",")
}

// Key returns a unique key for circuit pool lookups
// This combines all relevant isolation parameters into a single string
func (k *IsolationKey) Key() string {
	if k == nil || k.Level == IsolationNone {
		return ""
	}

	var parts []string

	switch k.Level {
	case IsolationDestination:
		parts = append(parts, "dest", k.Destination)
	case IsolationCredential:
		parts = append(parts, "creds", k.Credentials)
	case IsolationPort:
		parts = append(parts, "port", fmt.Sprintf("%d", k.SourcePort))
	case IsolationSession:
		parts = append(parts, "session", k.SessionToken)
	}

	return strings.Join(parts, ":")
}

// Equals checks if two isolation keys are equal
func (k *IsolationKey) Equals(other *IsolationKey) bool {
	if k == nil && other == nil {
		return true
	}
	if k == nil || other == nil {
		return false
	}

	if k.Level != other.Level {
		return false
	}

	// If level is none, all keys are equal
	if k.Level == IsolationNone {
		return true
	}

	// Compare based on level
	switch k.Level {
	case IsolationDestination:
		return k.Destination == other.Destination
	case IsolationCredential:
		return k.Credentials == other.Credentials
	case IsolationPort:
		return k.SourcePort == other.SourcePort
	case IsolationSession:
		return k.SessionToken == other.SessionToken
	default:
		return false
	}
}

// Validate checks if the isolation key is valid for its level
func (k *IsolationKey) Validate() error {
	if k == nil {
		return fmt.Errorf("isolation key is nil")
	}

	switch k.Level {
	case IsolationNone:
		// No validation needed
		return nil
	case IsolationDestination:
		if k.Destination == "" {
			return fmt.Errorf("destination isolation requires a destination")
		}
		// Basic validation: should contain host:port
		if !strings.Contains(k.Destination, ":") {
			return fmt.Errorf("invalid destination format: %s (expected host:port)", k.Destination)
		}
	case IsolationCredential:
		if k.Credentials == "" {
			return fmt.Errorf("credential isolation requires credentials")
		}
	case IsolationPort:
		if k.SourcePort == 0 {
			return fmt.Errorf("port isolation requires a non-zero source port")
		}
	case IsolationSession:
		if k.SessionToken == "" {
			return fmt.Errorf("session isolation requires a session token")
		}
	default:
		return fmt.Errorf("unknown isolation level: %d", k.Level)
	}

	return nil
}

// Clone creates a deep copy of the isolation key
func (k *IsolationKey) Clone() *IsolationKey {
	if k == nil {
		return nil
	}
	return &IsolationKey{
		Level:        k.Level,
		Destination:  k.Destination,
		Credentials:  k.Credentials,
		SourcePort:   k.SourcePort,
		SessionToken: k.SessionToken,
	}
}
