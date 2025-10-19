package autoconfig

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestGetDefaultDataDir(t *testing.T) {
	dataDir, err := GetDefaultDataDir()
	if err != nil {
		t.Fatalf("GetDefaultDataDir() failed: %v", err)
	}

	if dataDir == "" {
		t.Error("GetDefaultDataDir() returned empty string")
	}

	// Verify it contains platform-specific path
	switch runtime.GOOS {
	case "windows":
		if !filepath.IsAbs(dataDir) {
			t.Error("Expected absolute path on Windows")
		}
	case "darwin":
		if !filepath.IsAbs(dataDir) {
			t.Error("Expected absolute path on macOS")
		}
		// Should contain "Library/Application Support"
		if filepath.Base(filepath.Dir(dataDir)) != "Application Support" {
			t.Logf("macOS path: %s", dataDir)
		}
	default:
		if !filepath.IsAbs(dataDir) {
			t.Error("Expected absolute path on Linux")
		}
	}

	t.Logf("Platform: %s, Data directory: %s", runtime.GOOS, dataDir)
}

func TestEnsureDataDir(t *testing.T) {
	// Create temporary directory for testing
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "test-go-tor")

	// Test creating new directory
	err := EnsureDataDir(testDir)
	if err != nil {
		t.Fatalf("EnsureDataDir() failed: %v", err)
	}

	// Verify directory exists
	info, err := os.Stat(testDir)
	if err != nil {
		t.Fatalf("Directory was not created: %v", err)
	}

	if !info.IsDir() {
		t.Error("Path is not a directory")
	}

	// Verify permissions on Unix systems
	if runtime.GOOS != "windows" {
		mode := info.Mode().Perm()
		if mode != 0700 {
			t.Errorf("Expected permissions 0700, got %o", mode)
		}
	}

	// Test calling again on existing directory (should succeed)
	err = EnsureDataDir(testDir)
	if err != nil {
		t.Errorf("EnsureDataDir() failed on existing directory: %v", err)
	}
}

func TestEnsureDataDirWithFile(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "testfile")

	// Create a file instead of directory
	f, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	f.Close()

	// Try to ensure directory at file path (should fail)
	err = EnsureDataDir(testFile)
	if err == nil {
		t.Error("Expected error when path is a file, got nil")
	}
}

func TestEnsureSubDir(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "test-go-tor")

	// Create parent directory
	err := EnsureDataDir(testDir)
	if err != nil {
		t.Fatalf("Failed to create parent directory: %v", err)
	}

	// Create subdirectory
	subDir, err := EnsureSubDir(testDir, "guards")
	if err != nil {
		t.Fatalf("EnsureSubDir() failed: %v", err)
	}

	expectedPath := filepath.Join(testDir, "guards")
	if subDir != expectedPath {
		t.Errorf("Expected subdirectory path %s, got %s", expectedPath, subDir)
	}

	// Verify subdirectory exists
	info, err := os.Stat(subDir)
	if err != nil {
		t.Fatalf("Subdirectory was not created: %v", err)
	}

	if !info.IsDir() {
		t.Error("Subdirectory path is not a directory")
	}
}

func TestCleanupTempFiles(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()

	// Create some temporary files
	tempFiles := []string{
		filepath.Join(tmpDir, "test.tmp"),
		filepath.Join(tmpDir, "data.temp"),
		filepath.Join(tmpDir, "lock.lock~"),
		filepath.Join(tmpDir, "keep.txt"), // Should not be deleted
	}

	for _, file := range tempFiles {
		f, err := os.Create(file)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		f.Close()
	}

	// Run cleanup
	err := CleanupTempFiles(tmpDir)
	if err != nil {
		t.Fatalf("CleanupTempFiles() failed: %v", err)
	}

	// Verify temp files are deleted
	for _, file := range tempFiles[:3] {
		if _, err := os.Stat(file); !os.IsNotExist(err) {
			t.Errorf("Temp file was not deleted: %s", file)
		}
	}

	// Verify non-temp file is kept
	if _, err := os.Stat(tempFiles[3]); err != nil {
		t.Errorf("Non-temp file was deleted: %s", tempFiles[3])
	}
}

func TestFindAvailablePort(t *testing.T) {
	// Test with a likely available port
	preferredPort := 19050
	port := FindAvailablePort(preferredPort)

	if port < preferredPort {
		t.Errorf("Returned port %d is less than preferred port %d", port, preferredPort)
	}

	if port > preferredPort+100 {
		t.Errorf("Returned port %d is too far from preferred port %d", port, preferredPort)
	}

	t.Logf("Preferred port: %d, Available port: %d", preferredPort, port)
}

func TestIsPortAvailable(t *testing.T) {
	// Test with a likely available port
	port := 19051
	available := isPortAvailable(port)

	t.Logf("Port %d available: %v", port, available)

	// The result depends on system state, so we just verify it doesn't panic
	// and returns a boolean value
}
