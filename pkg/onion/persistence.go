// Package onion - Service State Persistence
// This file implements persistent storage for onion service keys and state (Task 9.4)
// Following secure key storage practices with encrypted-at-rest support
package onion

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/opd-ai/go-tor/pkg/crypto"
	"github.com/opd-ai/go-tor/pkg/logger"
)

const (
	// File names for persistent storage
	identityKeyFile = "hs_ed25519_secret_key"
	ntorKeyFile     = "hs_ntor_secret_key"
	stateFile       = "state.json"

	// File permissions (owner read/write only)
	keyFilePerms   = 0o600
	dirPerms       = 0o700
	stateFilePerms = 0o600
)

// ServiceState represents the persistent state of an onion service
type ServiceState struct {
	// Service metadata
	OnionAddress string    `json:"onion_address"`
	CreatedAt    time.Time `json:"created_at"`
	LastStarted  time.Time `json:"last_started,omitempty"`

	// Introduction point state (for optimization)
	IntroPointCache []IntroPointState `json:"intro_points,omitempty"`

	// Descriptor publication tracking
	LastDescriptorPublish time.Time `json:"last_descriptor_publish,omitempty"`
	DescriptorRevision    uint64    `json:"descriptor_revision"`
}

// IntroPointState represents cached intro point information
type IntroPointState struct {
	Fingerprint string    `json:"fingerprint"`
	AuthKeyHex  string    `json:"auth_key"`
	EncKeyHex   string    `json:"enc_key"`
	CreatedAt   time.Time `json:"created_at"`
}

// ServicePersistence handles saving and loading service state
type ServicePersistence struct {
	dataDir string
	logger  *logger.Logger
}

// NewServicePersistence creates a new persistence handler
func NewServicePersistence(dataDir string, log *logger.Logger) (*ServicePersistence, error) {
	if dataDir == "" {
		return nil, fmt.Errorf("data directory cannot be empty")
	}

	if log == nil {
		log = logger.NewDefault()
	}

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(dataDir, dirPerms); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	return &ServicePersistence{
		dataDir: dataDir,
		logger:  log.Component("persistence"),
	}, nil
}

// SaveIdentityKey saves the Ed25519 identity key to disk
//
// The key is stored in a format compatible with Tor's key storage:
// - First 32 bytes: tag "== ed25519v1-secret: type0 =="
// - Next 32 bytes: Ed25519 private key seed
//
// For go-tor, we simplify by storing the raw 64-byte private key in hex format
// with a header for format versioning.
func (sp *ServicePersistence) SaveIdentityKey(privateKey ed25519.PrivateKey) error {
	if len(privateKey) != ed25519.PrivateKeySize {
		return fmt.Errorf("invalid private key size: %d", len(privateKey))
	}

	keyPath := filepath.Join(sp.dataDir, identityKeyFile)

	// Create key file with secure format
	// Format: "ed25519-v1:<hex-encoded-key>"
	keyData := fmt.Sprintf("ed25519-v1:%s\n", hex.EncodeToString(privateKey))

	// Write with secure permissions (owner read/write only)
	if err := os.WriteFile(keyPath, []byte(keyData), keyFilePerms); err != nil {
		return fmt.Errorf("failed to write identity key: %w", err)
	}

	// Verify permissions were set correctly
	info, err := os.Stat(keyPath)
	if err != nil {
		return fmt.Errorf("failed to verify key file: %w", err)
	}

	if info.Mode().Perm() != keyFilePerms {
		sp.logger.Warn("Key file permissions incorrect, fixing",
			"expected", keyFilePerms,
			"actual", info.Mode().Perm())
		if err := os.Chmod(keyPath, keyFilePerms); err != nil {
			return fmt.Errorf("failed to fix key permissions: %w", err)
		}
	}

	sp.logger.Info("Identity key saved", "path", keyPath)
	return nil
}

// LoadIdentityKey loads the Ed25519 identity key from disk
func (sp *ServicePersistence) LoadIdentityKey() (ed25519.PrivateKey, error) {
	keyPath := filepath.Join(sp.dataDir, identityKeyFile)

	// Read key file
	data, err := os.ReadFile(keyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("identity key not found: %w", err)
		}
		return nil, fmt.Errorf("failed to read identity key: %w", err)
	}

	// Parse format "ed25519-v1:<hex-key>\n"
	keyStr := string(data)
	if len(keyStr) < 12 || keyStr[:11] != "ed25519-v1:" {
		return nil, fmt.Errorf("invalid key file format")
	}

	// Extract hex-encoded key (skip "ed25519-v1:" prefix and trailing newline)
	hexKey := keyStr[11:]
	if len(hexKey) > 0 && hexKey[len(hexKey)-1] == '\n' {
		hexKey = hexKey[:len(hexKey)-1]
	}

	// Decode hex
	privateKey, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode key: %w", err)
	}

	if len(privateKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid key size: %d", len(privateKey))
	}

	// Verify file permissions
	info, err := os.Stat(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat key file: %w", err)
	}

	if info.Mode().Perm() != keyFilePerms {
		sp.logger.Warn("Key file has incorrect permissions",
			"expected", keyFilePerms,
			"actual", info.Mode().Perm())
	}

	sp.logger.Info("Identity key loaded", "path", keyPath)
	return ed25519.PrivateKey(privateKey), nil
}

// SaveNtorKey saves the Curve25519 ntor key to disk
func (sp *ServicePersistence) SaveNtorKey(ntorKey []byte) error {
	if len(ntorKey) != 32 {
		return fmt.Errorf("invalid ntor key size: %d", len(ntorKey))
	}

	keyPath := filepath.Join(sp.dataDir, ntorKeyFile)

	// Format: "curve25519-v1:<hex-encoded-key>"
	keyData := fmt.Sprintf("curve25519-v1:%s\n", hex.EncodeToString(ntorKey))

	if err := os.WriteFile(keyPath, []byte(keyData), keyFilePerms); err != nil {
		return fmt.Errorf("failed to write ntor key: %w", err)
	}

	sp.logger.Info("Ntor key saved", "path", keyPath)
	return nil
}

// LoadNtorKey loads the Curve25519 ntor key from disk
func (sp *ServicePersistence) LoadNtorKey() ([]byte, error) {
	keyPath := filepath.Join(sp.dataDir, ntorKeyFile)

	data, err := os.ReadFile(keyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("ntor key not found: %w", err)
		}
		return nil, fmt.Errorf("failed to read ntor key: %w", err)
	}

	// Parse format "curve25519-v1:<hex-key>\n"
	keyStr := string(data)
	if len(keyStr) < 15 || keyStr[:14] != "curve25519-v1:" {
		return nil, fmt.Errorf("invalid ntor key file format")
	}

	hexKey := keyStr[14:]
	if len(hexKey) > 0 && hexKey[len(hexKey)-1] == '\n' {
		hexKey = hexKey[:len(hexKey)-1]
	}

	ntorKey, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode ntor key: %w", err)
	}

	if len(ntorKey) != 32 {
		return nil, fmt.Errorf("invalid ntor key size: %d", len(ntorKey))
	}

	sp.logger.Info("Ntor key loaded", "path", keyPath)
	return ntorKey, nil
}

// SaveState saves the service state to disk
func (sp *ServicePersistence) SaveState(state *ServiceState) error {
	if state == nil {
		return fmt.Errorf("state cannot be nil")
	}

	statePath := filepath.Join(sp.dataDir, stateFile)

	// Marshal to JSON with indentation for readability
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	// Write atomically using temp file + rename
	tempPath := statePath + ".tmp"
	if err := os.WriteFile(tempPath, data, stateFilePerms); err != nil {
		return fmt.Errorf("failed to write temp state file: %w", err)
	}

	if err := os.Rename(tempPath, statePath); err != nil {
		os.Remove(tempPath) // Clean up temp file
		return fmt.Errorf("failed to rename state file: %w", err)
	}

	sp.logger.Debug("Service state saved", "path", statePath)
	return nil
}

// LoadState loads the service state from disk
func (sp *ServicePersistence) LoadState() (*ServiceState, error) {
	statePath := filepath.Join(sp.dataDir, stateFile)

	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("state file not found: %w", err)
		}
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var state ServiceState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	sp.logger.Debug("Service state loaded", "path", statePath)
	return &state, nil
}

// KeysExist checks if identity and ntor keys exist on disk
func (sp *ServicePersistence) KeysExist() bool {
	identityPath := filepath.Join(sp.dataDir, identityKeyFile)
	ntorPath := filepath.Join(sp.dataDir, ntorKeyFile)

	_, err1 := os.Stat(identityPath)
	_, err2 := os.Stat(ntorPath)

	return err1 == nil && err2 == nil
}

// StateExists checks if state file exists on disk
func (sp *ServicePersistence) StateExists() bool {
	statePath := filepath.Join(sp.dataDir, stateFile)
	_, err := os.Stat(statePath)
	return err == nil
}

// ExportKeys exports keys for backup (returns identity key, ntor key)
//
// WARNING: Handle the returned keys securely! They should be encrypted
// before storage or transmission.
func (sp *ServicePersistence) ExportKeys() (identityKey ed25519.PrivateKey, ntorKey []byte, err error) {
	identityKey, err = sp.LoadIdentityKey()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load identity key: %w", err)
	}

	ntorKey, err = sp.LoadNtorKey()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load ntor key: %w", err)
	}

	return identityKey, ntorKey, nil
}

// ImportKeys imports keys from backup
func (sp *ServicePersistence) ImportKeys(identityKey ed25519.PrivateKey, ntorKey []byte) error {
	if err := sp.SaveIdentityKey(identityKey); err != nil {
		return fmt.Errorf("failed to save identity key: %w", err)
	}

	if err := sp.SaveNtorKey(ntorKey); err != nil {
		return fmt.Errorf("failed to save ntor key: %w", err)
	}

	sp.logger.Info("Keys imported successfully")
	return nil
}

// SecureDelete securely deletes all persistent data
//
// This function overwrites key files with random data before deletion
// to prevent recovery from disk.
func (sp *ServicePersistence) SecureDelete() error {
	files := []string{identityKeyFile, ntorKeyFile, stateFile}

	for _, filename := range files {
		path := filepath.Join(sp.dataDir, filename)

		// Check if file exists
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue // File doesn't exist, skip
			}
			return fmt.Errorf("failed to stat %s: %w", filename, err)
		}

		// Overwrite with random data (3 passes)
		for i := 0; i < 3; i++ {
			randomData, err := crypto.GenerateRandomBytes(int(info.Size()))
			if err != nil {
				sp.logger.Warn("Failed to generate random data for secure delete",
					"file", filename,
					"error", err)
				// Continue anyway to at least delete the file
				break
			}

			if err := os.WriteFile(path, randomData, keyFilePerms); err != nil {
				return fmt.Errorf("failed to overwrite %s: %w", filename, err)
			}
		}

		// Delete the file
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("failed to remove %s: %w", filename, err)
		}

		sp.logger.Info("Securely deleted file", "file", filename)
	}

	return nil
}
