package autoconfig

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestEnsureDataDirNestedCreation(t *testing.T) {
	tmpDir := t.TempDir()
	nested := filepath.Join(tmpDir, "a", "b", "c", "data")

	if err := EnsureDataDir(nested); err != nil {
		t.Fatalf("EnsureDataDir nested creation failed: %v", err)
	}

	info, err := os.Stat(nested)
	if err != nil {
		t.Fatalf("Nested directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("Expected directory, got file")
	}
}

func TestEnsureDataDirAlreadyCorrectPerms(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Permission tests not applicable on Windows")
	}
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "correct-perms")

	if err := os.Mkdir(testDir, 0o700); err != nil {
		t.Fatalf("Failed to create test dir: %v", err)
	}

	if err := EnsureDataDir(testDir); err != nil {
		t.Fatalf("EnsureDataDir failed on dir with correct perms: %v", err)
	}

	info, _ := os.Stat(testDir)
	if info.Mode().Perm() != 0o700 {
		t.Errorf("Permissions changed unexpectedly: %o", info.Mode().Perm())
	}
}

func TestEnsureDataDirFixesWrongPerms(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Permission tests not applicable on Windows")
	}
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "wrong-perms")

	if err := os.Mkdir(testDir, 0o755); err != nil {
		t.Fatalf("Failed to create test dir: %v", err)
	}

	if err := EnsureDataDir(testDir); err != nil {
		t.Fatalf("EnsureDataDir failed: %v", err)
	}

	info, _ := os.Stat(testDir)
	if info.Mode().Perm() != 0o700 {
		t.Errorf("Expected permissions 0700, got %o", info.Mode().Perm())
	}
}

func TestEnsureDataDirEmptyPath(t *testing.T) {
	// Empty path resolves to current dir via MkdirAll; behavior is OS-defined.
	// Just verify it doesn't panic.
	_ = EnsureDataDir("")
}

func TestEnsureSubDirEmptyName(t *testing.T) {
	tmpDir := t.TempDir()
	// Empty subdir joins to parent; should succeed as parent exists.
	path, err := EnsureSubDir(tmpDir, "")
	if err != nil {
		t.Fatalf("EnsureSubDir with empty name failed: %v", err)
	}
	if path != tmpDir {
		t.Errorf("Expected %s, got %s", tmpDir, path)
	}
}

func TestEnsureSubDirNestedPath(t *testing.T) {
	tmpDir := t.TempDir()
	path, err := EnsureSubDir(tmpDir, filepath.Join("a", "b", "c"))
	if err != nil {
		t.Fatalf("EnsureSubDir nested failed: %v", err)
	}

	expected := filepath.Join(tmpDir, "a", "b", "c")
	if path != expected {
		t.Errorf("Expected %s, got %s", expected, path)
	}

	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		t.Error("Nested subdirectory was not created")
	}
}

func TestEnsureSubDirParentMissing(t *testing.T) {
	tmpDir := t.TempDir()
	missingParent := filepath.Join(tmpDir, "nonexistent")

	path, err := EnsureSubDir(missingParent, "child")
	if err != nil {
		t.Fatalf("EnsureSubDir with missing parent failed: %v", err)
	}

	expected := filepath.Join(missingParent, "child")
	if path != expected {
		t.Errorf("Expected %s, got %s", expected, path)
	}
}

func TestCleanupTempFilesEmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	if err := CleanupTempFiles(tmpDir); err != nil {
		t.Fatalf("CleanupTempFiles on empty dir failed: %v", err)
	}
}

func TestCleanupTempFilesNonTempOnly(t *testing.T) {
	tmpDir := t.TempDir()
	keep := []string{"data.txt", "config.json", "readme.md"}

	for _, name := range keep {
		os.WriteFile(filepath.Join(tmpDir, name), []byte("x"), 0o644)
	}

	if err := CleanupTempFiles(tmpDir); err != nil {
		t.Fatalf("CleanupTempFiles failed: %v", err)
	}

	for _, name := range keep {
		if _, err := os.Stat(filepath.Join(tmpDir, name)); err != nil {
			t.Errorf("Non-temp file %s was deleted", name)
		}
	}
}

func TestCleanupTempFilesMixed(t *testing.T) {
	tmpDir := t.TempDir()
	temps := []string{"a.tmp", "b.temp", "c.lock~"}
	keeps := []string{"data.dat", "info.log"}

	for _, name := range append(temps, keeps...) {
		os.WriteFile(filepath.Join(tmpDir, name), []byte("x"), 0o644)
	}

	if err := CleanupTempFiles(tmpDir); err != nil {
		t.Fatalf("CleanupTempFiles failed: %v", err)
	}

	for _, name := range temps {
		if _, err := os.Stat(filepath.Join(tmpDir, name)); !os.IsNotExist(err) {
			t.Errorf("Temp file %s was not deleted", name)
		}
	}
	for _, name := range keeps {
		if _, err := os.Stat(filepath.Join(tmpDir, name)); err != nil {
			t.Errorf("Non-temp file %s was incorrectly deleted", name)
		}
	}
}

func TestCleanupTempFilesMultiplePatterns(t *testing.T) {
	tmpDir := t.TempDir()
	files := []string{
		"one.tmp", "two.tmp", "three.temp",
		"four.temp", "five.lock~", "six.lock~",
	}

	for _, name := range files {
		os.WriteFile(filepath.Join(tmpDir, name), []byte("x"), 0o644)
	}

	if err := CleanupTempFiles(tmpDir); err != nil {
		t.Fatalf("CleanupTempFiles failed: %v", err)
	}

	for _, name := range files {
		if _, err := os.Stat(filepath.Join(tmpDir, name)); !os.IsNotExist(err) {
			t.Errorf("Temp file %s was not deleted", name)
		}
	}
}

func TestCleanupTempFilesNonExistentDir(t *testing.T) {
	tmpDir := t.TempDir()
	missing := filepath.Join(tmpDir, "does-not-exist")
	// filepath.Glob returns nil matches for non-existent dirs, no error.
	if err := CleanupTempFiles(missing); err != nil {
		t.Fatalf("CleanupTempFiles on non-existent dir should not error: %v", err)
	}
}

func TestFindAvailablePortZero(t *testing.T) {
	port := FindAvailablePort(0)
	// Port 0 may or may not be available; result should be non-negative.
	if port < 0 {
		t.Errorf("FindAvailablePort(0) returned negative port: %d", port)
	}
}

func TestFindAvailablePortInUse(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to bind port: %v", err)
	}
	defer ln.Close()

	busyPort := ln.Addr().(*net.TCPAddr).Port
	port := FindAvailablePort(busyPort)

	if port == busyPort {
		t.Errorf("FindAvailablePort returned busy port %d", busyPort)
	}
	if port < busyPort || port > busyPort+100 {
		t.Errorf("Port %d outside expected range [%d, %d]", port, busyPort+1, busyPort+100)
	}
}

func TestFindAvailablePortHighNumber(t *testing.T) {
	port := FindAvailablePort(65534)
	// Should return 65534 if available, or fall back to 65534 (since range overflows).
	if port < 65534 {
		t.Errorf("Unexpected port %d for high preferred port", port)
	}
}

func TestIsPortAvailableZero(t *testing.T) {
	// Port 0 with address "127.0.0.1:0" gets an ephemeral port from the OS.
	result := isPortAvailable(0)
	t.Logf("isPortAvailable(0) = %v", result)
}

func TestIsPortAvailableBound(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to bind: %v", err)
	}
	defer ln.Close()

	busyPort := ln.Addr().(*net.TCPAddr).Port
	if isPortAvailable(busyPort) {
		t.Errorf("isPortAvailable(%d) should be false for bound port", busyPort)
	}
}

func TestGetDefaultDataDirContainsGoTor(t *testing.T) {
	dir, err := GetDefaultDataDir()
	if err != nil {
		t.Fatalf("GetDefaultDataDir failed: %v", err)
	}
	if filepath.Base(dir) != "go-tor" {
		t.Errorf("Expected path to end with 'go-tor', got %s", dir)
	}
}

func TestGetDefaultDataDirIsAbsolute(t *testing.T) {
	dir, err := GetDefaultDataDir()
	if err != nil {
		t.Fatalf("GetDefaultDataDir failed: %v", err)
	}
	if !filepath.IsAbs(dir) {
		t.Errorf("Expected absolute path, got %s", dir)
	}
}

func TestEnsureDataDirPermissionsOnCreate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Permission tests not applicable on Windows")
	}
	tmpDir := t.TempDir()
	newDir := filepath.Join(tmpDir, "new-dir")

	if err := EnsureDataDir(newDir); err != nil {
		t.Fatalf("EnsureDataDir failed: %v", err)
	}

	info, _ := os.Stat(newDir)
	if info.Mode().Perm() != 0o700 {
		t.Errorf("Expected 0700, got %o", info.Mode().Perm())
	}
}

func TestEnsureDataDirNestedPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Permission tests not applicable on Windows")
	}
	tmpDir := t.TempDir()
	nested := filepath.Join(tmpDir, "x", "y", "z")

	if err := EnsureDataDir(nested); err != nil {
		t.Fatalf("EnsureDataDir nested failed: %v", err)
	}

	// Check leaf directory permissions
	info, _ := os.Stat(nested)
	if info.Mode().Perm() != 0o700 {
		t.Errorf("Leaf dir expected 0700, got %o", info.Mode().Perm())
	}

	// Check intermediate directory exists
	mid := filepath.Join(tmpDir, "x", "y")
	if _, err := os.Stat(mid); err != nil {
		t.Errorf("Intermediate directory not created: %v", err)
	}
}

func TestFindAvailablePortTableDriven(t *testing.T) {
	tests := []struct {
		name      string
		preferred int
	}{
		{"low_port", 1024},
		{"mid_port", 30000},
		{"high_port", 60000},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			port := FindAvailablePort(tc.preferred)
			if port < tc.preferred || port > tc.preferred+100 {
				// Fallback is preferred port itself
				if port != tc.preferred {
					t.Errorf("Port %d outside range [%d, %d]",
						port, tc.preferred, tc.preferred+100)
				}
			}
		})
	}
}

func TestIsPortAvailableAfterClose(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to bind: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	// After closing, port should be available again
	if !isPortAvailable(port) {
		t.Logf("Port %d not immediately available after close (OS delay)", port)
	}
}

// Ensure FindAvailablePort with consecutive busy ports finds the next free one.
func TestFindAvailablePortConsecutiveBusy(t *testing.T) {
	var listeners []net.Listener
	defer func() {
		for _, ln := range listeners {
			ln.Close()
		}
	}()

	base := 0
	// Bind 3 consecutive ports
	for i := 0; i < 3; i++ {
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", 0))
		if err != nil {
			t.Fatalf("Failed to bind port: %v", err)
		}
		if i == 0 {
			base = ln.Addr().(*net.TCPAddr).Port
			listeners = append(listeners, ln)
		} else {
			// Try to bind sequential ports from base
			ln.Close()
			ln2, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", base+i))
			if err != nil {
				t.Skipf("Cannot bind consecutive ports for test: %v", err)
			}
			listeners = append(listeners, ln2)
		}
	}

	port := FindAvailablePort(base)
	if port == base {
		t.Errorf("Should not return base port %d which is busy", base)
	}
}
