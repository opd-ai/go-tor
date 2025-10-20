package resources

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetDefaultTorrc(t *testing.T) {
	content, err := GetDefaultTorrc()
	if err != nil {
		t.Fatalf("GetDefaultTorrc() failed: %v", err)
	}

	if len(content) == 0 {
		t.Fatal("GetDefaultTorrc() returned empty content")
	}

	// Verify it contains expected configuration
	expectedStrings := []string{
		"ClientOnly",
		"SocksPort",
		"ControlPort",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(content, expected) {
			t.Errorf("torrc content missing expected string: %s", expected)
		}
	}
}

func TestGetFallbackAuthorities(t *testing.T) {
	authorities, err := GetFallbackAuthorities()
	if err != nil {
		t.Fatalf("GetFallbackAuthorities() failed: %v", err)
	}

	if len(authorities) == 0 {
		t.Fatal("GetFallbackAuthorities() returned empty list")
	}

	// Verify all entries are valid URLs
	for _, auth := range authorities {
		if !strings.HasPrefix(auth, "http://") && !strings.HasPrefix(auth, "https://") {
			t.Errorf("Invalid authority URL: %s", auth)
		}
	}

	t.Logf("Found %d fallback authorities", len(authorities))
}

func TestExtractDefaultTorrc(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "torrc")

	// First extraction should succeed
	extracted, err := ExtractDefaultTorrc(destPath)
	if err != nil {
		t.Fatalf("ExtractDefaultTorrc() failed: %v", err)
	}

	if !extracted {
		t.Error("ExtractDefaultTorrc() should return true on first extraction")
	}

	// Verify file exists
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Fatal("Extracted file does not exist")
	}

	// Verify file permissions (on Unix systems)
	if info, err := os.Stat(destPath); err == nil {
		mode := info.Mode().Perm()
		if mode != 0600 {
			t.Errorf("File permissions incorrect: got %o, want 0600", mode)
		}
	}

	// Second extraction should not overwrite
	extracted, err = ExtractDefaultTorrc(destPath)
	if err != nil {
		t.Fatalf("ExtractDefaultTorrc() second call failed: %v", err)
	}

	if extracted {
		t.Error("ExtractDefaultTorrc() should return false when file exists")
	}
}

func TestValidateExtraction(t *testing.T) {
	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "torrc")

	// Extract the file
	_, err := ExtractDefaultTorrc(destPath)
	if err != nil {
		t.Fatalf("ExtractDefaultTorrc() failed: %v", err)
	}

	// Validate extraction
	valid, err := ValidateExtraction("torrc.default", destPath)
	if err != nil {
		t.Fatalf("ValidateExtraction() failed: %v", err)
	}

	if !valid {
		t.Error("ValidateExtraction() returned false for valid extraction")
	}

	// Corrupt the file and test again
	if err := os.WriteFile(destPath, []byte("corrupted"), 0600); err != nil {
		t.Fatalf("Failed to corrupt file: %v", err)
	}

	valid, err = ValidateExtraction("torrc.default", destPath)
	if err != nil {
		t.Fatalf("ValidateExtraction() failed on corrupted file: %v", err)
	}

	if valid {
		t.Error("ValidateExtraction() should return false for corrupted file")
	}
}

func TestListEmbeddedResources(t *testing.T) {
	resources, err := ListEmbeddedResources()
	if err != nil {
		t.Fatalf("ListEmbeddedResources() failed: %v", err)
	}

	if len(resources) == 0 {
		t.Fatal("ListEmbeddedResources() returned empty list")
	}

	// Verify expected resources are present
	expectedResources := []string{
		"torrc.default",
		"fallback-dirs.txt",
	}

	for _, expected := range expectedResources {
		found := false
		for _, resource := range resources {
			if strings.Contains(resource, expected) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected resource not found: %s", expected)
		}
	}

	t.Logf("Found %d embedded resources", len(resources))
}

func TestExtractResource(t *testing.T) {
	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "test-file")

	// Extract a resource
	err := ExtractResource("torrc.default", destPath)
	if err != nil {
		t.Fatalf("ExtractResource() failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Fatal("Extracted file does not exist")
	}

	// Verify content matches
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Failed to read extracted file: %v", err)
	}

	originalContent, err := GetDefaultTorrc()
	if err != nil {
		t.Fatalf("Failed to get original content: %v", err)
	}

	if string(content) != originalContent {
		t.Error("Extracted content does not match original")
	}
}

func TestCopyEmbeddedFile(t *testing.T) {
	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "copied-file")

	// Copy a resource
	err := CopyEmbeddedFile("fallback-dirs.txt", destPath)
	if err != nil {
		t.Fatalf("CopyEmbeddedFile() failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Fatal("Copied file does not exist")
	}

	// Verify content
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Failed to read copied file: %v", err)
	}

	if len(content) == 0 {
		t.Error("Copied file is empty")
	}
}
