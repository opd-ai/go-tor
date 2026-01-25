package pt

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDiscoverPTAbsolutePath(t *testing.T) {
	tempDir := t.TempDir()
	mockBinary := filepath.Join(tempDir, "test-pt")

	if runtime.GOOS == "windows" {
		mockBinary += ".exe"
	}

	if err := os.WriteFile(mockBinary, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	path, err := DiscoverPT(mockBinary)
	if err != nil {
		t.Fatalf("DiscoverPT failed: %v", err)
	}

	if path != mockBinary {
		t.Errorf("DiscoverPT returned %s, want %s", path, mockBinary)
	}
}

func TestDiscoverPTNotFound(t *testing.T) {
	_, err := DiscoverPT("nonexistent-pt-binary-12345")
	if err == nil {
		t.Error("DiscoverPT should fail for nonexistent binary")
	}

	if !os.IsNotExist(err) {
		t.Errorf("DiscoverPT error should be os.ErrNotExist, got %v", err)
	}
}

func TestDiscoverPTInPath(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping PATH discovery test in short mode")
	}

	path, err := DiscoverPT("sh")
	if err != nil {
		t.Logf("sh not in PATH (expected on some systems): %v", err)
		return
	}

	if path == "" {
		t.Error("DiscoverPT returned empty path for sh")
	}
}

func TestDiscoverPTWindowsExtension(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific test")
	}

	tempDir := t.TempDir()
	mockBinary := filepath.Join(tempDir, "test-pt.exe")

	if err := os.WriteFile(mockBinary, []byte("mock"), 0o755); err != nil {
		t.Fatal(err)
	}

	origPaths := PTBinaryPaths
	PTBinaryPaths = []string{tempDir}
	defer func() { PTBinaryPaths = origPaths }()

	path, err := DiscoverPT("test-pt")
	if err != nil {
		t.Fatalf("DiscoverPT failed: %v", err)
	}

	if path != mockBinary {
		t.Errorf("DiscoverPT returned %s, want %s", path, mockBinary)
	}
}

func TestDiscoverPTInCustomPaths(t *testing.T) {
	tempDir := t.TempDir()
	customDir := filepath.Join(tempDir, "custom")
	if err := os.MkdirAll(customDir, 0o755); err != nil {
		t.Fatal(err)
	}

	mockName := "test-pt"
	if runtime.GOOS == "windows" {
		mockName += ".exe"
	}
	mockBinary := filepath.Join(customDir, mockName)

	if err := os.WriteFile(mockBinary, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	origPaths := PTBinaryPaths
	PTBinaryPaths = []string{customDir}
	defer func() { PTBinaryPaths = origPaths }()

	path, err := DiscoverPT(mockName)
	if err != nil {
		t.Fatalf("DiscoverPT failed: %v", err)
	}

	if path != mockBinary {
		t.Errorf("DiscoverPT returned %s, want %s", path, mockBinary)
	}
}

func TestDiscoverPTInUserHome(t *testing.T) {
	tempDir := t.TempDir()
	localBin := filepath.Join(tempDir, ".local", "bin")
	if err := os.MkdirAll(localBin, 0o755); err != nil {
		t.Fatal(err)
	}

	mockName := "test-pt"
	if runtime.GOOS == "windows" {
		mockName += ".exe"
	}
	mockBinary := filepath.Join(localBin, mockName)

	if err := os.WriteFile(mockBinary, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	origHome := os.Getenv("HOME")
	if runtime.GOOS == "windows" {
		origHome = os.Getenv("USERPROFILE")
		os.Setenv("USERPROFILE", tempDir)
		defer os.Setenv("USERPROFILE", origHome)
	} else {
		os.Setenv("HOME", tempDir)
		defer os.Setenv("HOME", origHome)
	}

	path, err := DiscoverPT(mockName)
	if err != nil {
		t.Fatalf("DiscoverPT failed: %v", err)
	}

	if path != mockBinary {
		t.Errorf("DiscoverPT returned %s, want %s", path, mockBinary)
	}
}

func TestDiscoverCommonPTs(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping common PT discovery in short mode")
	}

	pts := DiscoverCommonPTs()

	if pts == nil {
		t.Fatal("DiscoverCommonPTs returned nil")
	}

	t.Logf("Discovered %d common PTs", len(pts))
	for name, path := range pts {
		t.Logf("  %s: %s", name, path)
	}
}

func TestDiscoverCommonPTsWithMocks(t *testing.T) {
	tempDir := t.TempDir()

	mockPTs := []string{"obfs4proxy", "snowflake-client"}
	for _, name := range mockPTs {
		mockName := name
		if runtime.GOOS == "windows" {
			mockName += ".exe"
		}
		mockBinary := filepath.Join(tempDir, mockName)
		if err := os.WriteFile(mockBinary, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	origPaths := PTBinaryPaths
	PTBinaryPaths = []string{tempDir}
	defer func() { PTBinaryPaths = origPaths }()

	pts := DiscoverCommonPTs()

	expectedCount := len(mockPTs)
	if len(pts) != expectedCount {
		t.Errorf("DiscoverCommonPTs found %d PTs, want %d", len(pts), expectedCount)
	}

	for _, name := range mockPTs {
		if _, ok := pts[name]; !ok {
			t.Errorf("DiscoverCommonPTs did not find %s", name)
		}
	}
}

func TestAddDefaultPaths(t *testing.T) {
	origPaths := PTBinaryPaths
	PTBinaryPaths = []string{"/usr/bin"}

	AddDefaultPaths()

	if len(PTBinaryPaths) <= 1 {
		t.Error("AddDefaultPaths should add platform-specific paths")
	}

	t.Logf("Default PT paths (%s):", runtime.GOOS)
	for i, path := range PTBinaryPaths {
		t.Logf("  [%d] %s", i, path)
	}

	PTBinaryPaths = origPaths
}

func TestPTBinaryPathsInitialization(t *testing.T) {
	if len(PTBinaryPaths) == 0 {
		t.Error("PTBinaryPaths should be initialized")
	}

	t.Logf("Initialized with %d paths", len(PTBinaryPaths))
}

func TestDiscoverPTRelativeName(t *testing.T) {
	tempDir := t.TempDir()
	mockName := "relative-pt"
	if runtime.GOOS == "windows" {
		mockName += ".exe"
	}
	mockBinary := filepath.Join(tempDir, mockName)

	if err := os.WriteFile(mockBinary, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	origPaths := PTBinaryPaths
	PTBinaryPaths = []string{tempDir}
	defer func() { PTBinaryPaths = origPaths }()

	path, err := DiscoverPT(mockName)
	if err != nil {
		t.Fatalf("DiscoverPT failed: %v", err)
	}

	if path != mockBinary {
		t.Errorf("DiscoverPT returned %s, want %s", path, mockBinary)
	}
}

func TestDiscoverPTSearchOrder(t *testing.T) {
	tempDir1 := t.TempDir()
	tempDir2 := t.TempDir()

	mockName := "priority-pt"
	if runtime.GOOS == "windows" {
		mockName += ".exe"
	}

	mockBinary1 := filepath.Join(tempDir1, mockName)
	mockBinary2 := filepath.Join(tempDir2, mockName)

	if err := os.WriteFile(mockBinary1, []byte("first"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mockBinary2, []byte("second"), 0o755); err != nil {
		t.Fatal(err)
	}

	origPaths := PTBinaryPaths
	PTBinaryPaths = []string{tempDir1, tempDir2}
	defer func() { PTBinaryPaths = origPaths }()

	path, err := DiscoverPT(mockName)
	if err != nil {
		t.Fatalf("DiscoverPT failed: %v", err)
	}

	if path != mockBinary1 {
		t.Errorf("DiscoverPT returned %s, expected first path %s", path, mockBinary1)
	}
}
