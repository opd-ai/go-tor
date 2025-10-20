// Package resources provides embedded resource management for go-tor.
// This package uses Go 1.16+ embed to bundle default configuration files
// and fallback directory authorities directly into the binary.
package resources

import (
	"bufio"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Embed default resources into the binary
//
//go:embed torrc.default fallback-dirs.txt
var embeddedFS embed.FS

// GetDefaultTorrc returns the default torrc configuration content.
func GetDefaultTorrc() (string, error) {
	data, err := embeddedFS.ReadFile("torrc.default")
	if err != nil {
		return "", fmt.Errorf("failed to read embedded torrc: %w", err)
	}
	return string(data), nil
}

// GetFallbackAuthorities returns the list of fallback directory authorities.
// Returns a slice of URLs for directory authorities.
func GetFallbackAuthorities() ([]string, error) {
	data, err := embeddedFS.ReadFile("fallback-dirs.txt")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded fallback directories: %w", err)
	}

	authorities := make([]string, 0, 10)
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Validate it looks like a URL
		if strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") {
			authorities = append(authorities, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to parse fallback directories: %w", err)
	}

	if len(authorities) == 0 {
		return nil, fmt.Errorf("no valid fallback directories found")
	}

	return authorities, nil
}

// ExtractDefaultTorrc extracts the default torrc to the specified path if it doesn't exist.
// Returns true if the file was extracted, false if it already existed.
func ExtractDefaultTorrc(destPath string) (bool, error) {
	// Check if file already exists
	if _, err := os.Stat(destPath); err == nil {
		return false, nil // File exists, don't overwrite
	}

	// Get default torrc content
	content, err := GetDefaultTorrc()
	if err != nil {
		return false, fmt.Errorf("failed to get default torrc: %w", err)
	}

	// Ensure parent directory exists
	dir := filepath.Dir(destPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return false, fmt.Errorf("failed to create directory: %w", err)
	}

	// Write file with secure permissions (user read/write only)
	if err := os.WriteFile(destPath, []byte(content), 0600); err != nil {
		return false, fmt.Errorf("failed to write torrc: %w", err)
	}

	return true, nil
}

// ExtractResource extracts an embedded resource to the specified destination path.
// This is a generic function for extracting any embedded resource.
func ExtractResource(resourcePath string, destPath string) error {
	// Read from embedded filesystem
	data, err := embeddedFS.ReadFile(resourcePath)
	if err != nil {
		return fmt.Errorf("failed to read embedded resource %s: %w", resourcePath, err)
	}

	// Ensure parent directory exists
	dir := filepath.Dir(destPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write file with secure permissions
	if err := os.WriteFile(destPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write resource: %w", err)
	}

	return nil
}

// ValidateExtraction verifies that an extracted file matches the embedded resource.
// Returns true if the file matches the embedded resource.
func ValidateExtraction(resourcePath string, destPath string) (bool, error) {
	// Read embedded resource
	embeddedData, err := embeddedFS.ReadFile(resourcePath)
	if err != nil {
		return false, fmt.Errorf("failed to read embedded resource: %w", err)
	}

	// Read extracted file
	extractedData, err := os.ReadFile(destPath)
	if err != nil {
		return false, fmt.Errorf("failed to read extracted file: %w", err)
	}

	// Compare content
	if len(embeddedData) != len(extractedData) {
		return false, nil
	}

	for i := range embeddedData {
		if embeddedData[i] != extractedData[i] {
			return false, nil
		}
	}

	return true, nil
}

// ListEmbeddedResources returns a list of all embedded resource paths.
func ListEmbeddedResources() ([]string, error) {
	var resources []string

	err := walkEmbedFS(embeddedFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			resources = append(resources, path)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list embedded resources: %w", err)
	}

	return resources, nil
}

// walkEmbedFS walks the embedded filesystem similar to filepath.WalkDir
func walkEmbedFS(fsys embed.FS, root string, fn func(path string, d fs.DirEntry, err error) error) error {
	entries, err := fsys.ReadDir(root)
	if err != nil {
		return fn(root, nil, err)
	}

	for _, entry := range entries {
		path := entry.Name()
		if root != "." {
			path = filepath.Join(root, entry.Name())
		}
		if err := fn(path, entry, nil); err != nil {
			return err
		}

		if entry.IsDir() {
			if err := walkEmbedFS(fsys, path, fn); err != nil {
				return err
			}
		}
	}

	return nil
}

// CopyEmbeddedFile copies an embedded file to a destination using streaming.
// This is useful for larger files to avoid loading everything into memory.
func CopyEmbeddedFile(resourcePath string, destPath string) error {
	// Open embedded file
	srcFile, err := embeddedFS.Open(resourcePath)
	if err != nil {
		return fmt.Errorf("failed to open embedded resource: %w", err)
	}
	defer srcFile.Close()

	// Ensure parent directory exists
	dir := filepath.Dir(destPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create destination file
	destFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	// Copy data
	if _, err := io.Copy(destFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy data: %w", err)
	}

	return nil
}
