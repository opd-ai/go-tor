package pt

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// PTBinaryPaths holds common PT binary search paths.
var PTBinaryPaths = []string{
	"/usr/bin",
	"/usr/local/bin",
	"/opt/tor/bin",
}

// DiscoverPT searches for a PT binary in common locations.
// It returns the full path to the PT binary if found.
func DiscoverPT(name string) (string, error) {
	if runtime.GOOS == "windows" {
		name = name + ".exe"
	}

	if filepath.IsAbs(name) {
		if _, err := os.Stat(name); err == nil {
			return name, nil
		} else {
			return "", err
		}
	}

	path, err := exec.LookPath(name)
	if err == nil {
		return path, nil
	}

	for _, dir := range PTBinaryPaths {
		fullPath := filepath.Join(dir, name)
		if _, err := os.Stat(fullPath); err == nil {
			return fullPath, nil
		}
	}

	userHome, err := os.UserHomeDir()
	if err == nil {
		localPaths := []string{
			filepath.Join(userHome, ".local", "bin", name),
			filepath.Join(userHome, "bin", name),
		}
		for _, p := range localPaths {
			if _, err := os.Stat(p); err == nil {
				return p, nil
			}
		}
	}

	return "", os.ErrNotExist
}

// DiscoverCommonPTs discovers common PT binaries (obfs4proxy, snowflake-client, etc.).
func DiscoverCommonPTs() map[string]string {
	commonPTs := []string{
		"obfs4proxy",
		"snowflake-client",
		"meek-client",
		"lyrebird",
	}

	found := make(map[string]string)
	for _, name := range commonPTs {
		if path, err := DiscoverPT(name); err == nil {
			baseName := strings.TrimSuffix(name, ".exe")
			found[baseName] = path
		}
	}

	return found
}

// AddDefaultPaths adds platform-specific default PT search paths.
func AddDefaultPaths() {
	switch runtime.GOOS {
	case "darwin":
		PTBinaryPaths = append(PTBinaryPaths,
			"/Applications/TorBrowser.app/Contents/MacOS/Tor/PluggableTransports",
			filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "TorBrowser-Data", "Tor", "PluggableTransports"),
		)
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData != "" {
			PTBinaryPaths = append(PTBinaryPaths,
				filepath.Join(appData, "Tor Browser", "Browser", "TorBrowser", "Tor", "PluggableTransports"),
			)
		}
		programFiles := os.Getenv("ProgramFiles")
		if programFiles != "" {
			PTBinaryPaths = append(PTBinaryPaths,
				filepath.Join(programFiles, "Tor Browser", "Browser", "TorBrowser", "Tor", "PluggableTransports"),
			)
		}
	case "linux":
		home := os.Getenv("HOME")
		if home != "" {
			PTBinaryPaths = append(PTBinaryPaths,
				filepath.Join(home, ".local", "share", "torbrowser", "tbb", "x86_64", "tor-browser_en-US", "Browser", "TorBrowser", "Tor", "PluggableTransports"),
				filepath.Join(home, ".tor-browser", "tor-browser_en-US", "Browser", "TorBrowser", "Tor", "PluggableTransports"),
			)
		}
	}
}

func init() {
	AddDefaultPaths()
}
