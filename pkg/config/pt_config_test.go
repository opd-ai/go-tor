package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClientTransportPlugin_Parse(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		wantName  string
		wantPath  string
		wantOpts  map[string]string
		wantError bool
	}{
		{
			name:      "basic obfs4",
			value:     "obfs4 exec /usr/bin/obfs4proxy",
			wantName:  "obfs4",
			wantPath:  "/usr/bin/obfs4proxy",
			wantOpts:  map[string]string{},
			wantError: false,
		},
		{
			name:      "with options",
			value:     "obfs4 exec /usr/bin/obfs4proxy cert=abc123 iat-mode=1",
			wantName:  "obfs4",
			wantPath:  "/usr/bin/obfs4proxy",
			wantOpts:  map[string]string{"cert": "abc123", "iat-mode": "1"},
			wantError: false,
		},
		{
			name:      "meek transport",
			value:     "meek exec /usr/bin/meek-client url=https://example.com",
			wantName:  "meek",
			wantPath:  "/usr/bin/meek-client",
			wantOpts:  map[string]string{"url": "https://example.com"},
			wantError: false,
		},
		{
			name:      "missing exec keyword",
			value:     "obfs4 /usr/bin/obfs4proxy",
			wantError: true,
		},
		{
			name:      "invalid format",
			value:     "obfs4",
			wantError: true,
		},
		{
			name:      "wrong exec keyword",
			value:     "obfs4 run /usr/bin/obfs4proxy",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseClientTransportPlugin(tt.value)
			if (err != nil) != tt.wantError {
				t.Errorf("parseClientTransportPlugin() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if tt.wantError {
				return
			}

			if got.Name != tt.wantName {
				t.Errorf("Name = %v, want %v", got.Name, tt.wantName)
			}
			if got.BinaryPath != tt.wantPath {
				t.Errorf("BinaryPath = %v, want %v", got.BinaryPath, tt.wantPath)
			}

			// Check options
			if len(got.Options) != len(tt.wantOpts) {
				t.Errorf("Options count = %v, want %v", len(got.Options), len(tt.wantOpts))
			}
			for k, v := range tt.wantOpts {
				if got.Options[k] != v {
					t.Errorf("Options[%s] = %v, want %v", k, got.Options[k], v)
				}
			}
		})
	}
}

func TestServerTransportPlugin_Parse(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		wantName  string
		wantPath  string
		wantError bool
	}{
		{
			name:      "basic obfs4",
			value:     "obfs4 exec /usr/bin/obfs4proxy",
			wantName:  "obfs4",
			wantPath:  "/usr/bin/obfs4proxy",
			wantError: false,
		},
		{
			name:      "missing exec",
			value:     "obfs4 /usr/bin/obfs4proxy",
			wantError: true,
		},
		{
			name:      "invalid format",
			value:     "obfs4",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseServerTransportPlugin(tt.value)
			if (err != nil) != tt.wantError {
				t.Errorf("parseServerTransportPlugin() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if tt.wantError {
				return
			}

			if got.Name != tt.wantName {
				t.Errorf("Name = %v, want %v", got.Name, tt.wantName)
			}
			if got.BinaryPath != tt.wantPath {
				t.Errorf("BinaryPath = %v, want %v", got.BinaryPath, tt.wantPath)
			}
		})
	}
}

func TestServerTransportListenAddr_Parse(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		wantTrans string
		wantAddr  string
		wantError bool
	}{
		{
			name:      "ipv4 address",
			value:     "obfs4 0.0.0.0:9443",
			wantTrans: "obfs4",
			wantAddr:  "0.0.0.0:9443",
			wantError: false,
		},
		{
			name:      "localhost",
			value:     "obfs4 127.0.0.1:9443",
			wantTrans: "obfs4",
			wantAddr:  "127.0.0.1:9443",
			wantError: false,
		},
		{
			name:      "invalid format",
			value:     "obfs4",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			err := parseServerTransportListenAddr(cfg, tt.value)
			if (err != nil) != tt.wantError {
				t.Errorf("parseServerTransportListenAddr() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if tt.wantError {
				return
			}

			// Find the transport
			found := false
			for _, st := range cfg.ServerTransports {
				if st.Name == tt.wantTrans {
					found = true
					if st.BindAddr != tt.wantAddr {
						t.Errorf("BindAddr = %v, want %v", st.BindAddr, tt.wantAddr)
					}
					break
				}
			}
			if !found {
				t.Errorf("transport %s not found in config", tt.wantTrans)
			}
		})
	}
}

func TestServerTransportOptions_Parse(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		wantTrans string
		wantOpts  map[string]string
		wantError bool
	}{
		{
			name:      "single option",
			value:     "obfs4 iat-mode=1",
			wantTrans: "obfs4",
			wantOpts:  map[string]string{"iat-mode": "1"},
			wantError: false,
		},
		{
			name:      "multiple options",
			value:     "obfs4 iat-mode=1 drbg-seed=abc123",
			wantTrans: "obfs4",
			wantOpts:  map[string]string{"iat-mode": "1", "drbg-seed": "abc123"},
			wantError: false,
		},
		{
			name:      "invalid format",
			value:     "obfs4",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			err := parseServerTransportOptions(cfg, tt.value)
			if (err != nil) != tt.wantError {
				t.Errorf("parseServerTransportOptions() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if tt.wantError {
				return
			}

			// Find the transport
			found := false
			for _, st := range cfg.ServerTransports {
				if st.Name == tt.wantTrans {
					found = true
					for k, v := range tt.wantOpts {
						if st.Options[k] != v {
							t.Errorf("Options[%s] = %v, want %v", k, st.Options[k], v)
						}
					}
					break
				}
			}
			if !found {
				t.Errorf("transport %s not found in config", tt.wantTrans)
			}
		})
	}
}

func TestLoadFromFile_PTConfiguration(t *testing.T) {
	tests := []struct {
		name    string
		content string
		check   func(*testing.T, *Config)
	}{
		{
			name: "client transport only",
			content: `SocksPort 9050
DataDirectory /tmp/tor-data
ClientTransportPlugin obfs4 exec /usr/bin/obfs4proxy
`,
			check: func(t *testing.T, cfg *Config) {
				if len(cfg.ClientTransports) != 1 {
					t.Errorf("expected 1 client transport, got %d", len(cfg.ClientTransports))
					return
				}
				ct := cfg.ClientTransports[0]
				if ct.Name != "obfs4" {
					t.Errorf("Name = %v, want obfs4", ct.Name)
				}
				if ct.BinaryPath != "/usr/bin/obfs4proxy" {
					t.Errorf("BinaryPath = %v, want /usr/bin/obfs4proxy", ct.BinaryPath)
				}
			},
		},
		{
			name: "server transport with listen addr",
			content: `SocksPort 9050
DataDirectory /tmp/tor-data
ServerTransportPlugin obfs4 exec /usr/bin/obfs4proxy
ServerTransportListenAddr obfs4 0.0.0.0:9443
ServerTransportOptions obfs4 iat-mode=1
`,
			check: func(t *testing.T, cfg *Config) {
				if len(cfg.ServerTransports) != 1 {
					t.Errorf("expected 1 server transport, got %d", len(cfg.ServerTransports))
					return
				}
				st := cfg.ServerTransports[0]
				if st.Name != "obfs4" {
					t.Errorf("Name = %v, want obfs4", st.Name)
				}
				if st.BinaryPath != "/usr/bin/obfs4proxy" {
					t.Errorf("BinaryPath = %v, want /usr/bin/obfs4proxy", st.BinaryPath)
				}
				if st.BindAddr != "0.0.0.0:9443" {
					t.Errorf("BindAddr = %v, want 0.0.0.0:9443", st.BindAddr)
				}
				if st.Options["iat-mode"] != "1" {
					t.Errorf("Options[iat-mode] = %v, want 1", st.Options["iat-mode"])
				}
			},
		},
		{
			name: "transport proxy",
			content: `SocksPort 9050
DataDirectory /tmp/tor-data
TransportProxy socks5 127.0.0.1:9050
`,
			check: func(t *testing.T, cfg *Config) {
				if cfg.TransportProxy != "socks5 127.0.0.1:9050" {
					t.Errorf("TransportProxy = %v, want socks5 127.0.0.1:9050", cfg.TransportProxy)
				}
			},
		},
		{
			name: "multiple transports",
			content: `SocksPort 9050
DataDirectory /tmp/tor-data
ClientTransportPlugin obfs4 exec /usr/bin/obfs4proxy
ClientTransportPlugin meek exec /usr/bin/meek-client url=https://example.com
`,
			check: func(t *testing.T, cfg *Config) {
				if len(cfg.ClientTransports) != 2 {
					t.Errorf("expected 2 client transports, got %d", len(cfg.ClientTransports))
					return
				}
				// Check first transport
				if cfg.ClientTransports[0].Name != "obfs4" {
					t.Errorf("ClientTransports[0].Name = %v, want obfs4", cfg.ClientTransports[0].Name)
				}
				// Check second transport
				if cfg.ClientTransports[1].Name != "meek" {
					t.Errorf("ClientTransports[1].Name = %v, want meek", cfg.ClientTransports[1].Name)
				}
				if cfg.ClientTransports[1].Options["url"] != "https://example.com" {
					t.Errorf("ClientTransports[1].Options[url] = %v, want https://example.com", cfg.ClientTransports[1].Options["url"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file
			tmpfile, err := os.CreateTemp("", "torrc-*.conf")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(tmpfile.Name())

			if _, err := tmpfile.WriteString(tt.content); err != nil {
				t.Fatal(err)
			}
			if err := tmpfile.Close(); err != nil {
				t.Fatal(err)
			}

			// Load config
			cfg := DefaultConfig()
			if err := LoadFromFile(tmpfile.Name(), cfg); err != nil {
				t.Fatalf("LoadFromFile() error = %v", err)
			}

			// Run check
			tt.check(t, cfg)
		})
	}
}

func TestSaveToFile_PTConfiguration(t *testing.T) {
	// Create a config with PT settings
	cfg := DefaultConfig()
	cfg.ClientTransports = []ClientTransportConfig{
		{
			Name:       "obfs4",
			BinaryPath: "/usr/bin/obfs4proxy",
			Options:    map[string]string{"cert": "abc123"},
		},
	}
	cfg.ServerTransports = []ServerTransportConfig{
		{
			Name:       "obfs4",
			BinaryPath: "/usr/bin/obfs4proxy",
			BindAddr:   "0.0.0.0:9443",
			Options:    map[string]string{"iat-mode": "1"},
		},
	}
	cfg.TransportProxy = "socks5 127.0.0.1:9050"

	// Save to file
	tmpfile := filepath.Join(t.TempDir(), "torrc.conf")
	if err := SaveToFile(tmpfile, cfg); err != nil {
		t.Fatalf("SaveToFile() error = %v", err)
	}

	// Load back and verify
	loaded := DefaultConfig()
	if err := LoadFromFile(tmpfile, loaded); err != nil {
		t.Fatalf("LoadFromFile() error = %v", err)
	}

	// Verify client transports
	if len(loaded.ClientTransports) != 1 {
		t.Errorf("expected 1 client transport, got %d", len(loaded.ClientTransports))
	} else {
		ct := loaded.ClientTransports[0]
		if ct.Name != "obfs4" || ct.BinaryPath != "/usr/bin/obfs4proxy" {
			t.Errorf("client transport mismatch: %+v", ct)
		}
		if ct.Options["cert"] != "abc123" {
			t.Errorf("client transport option cert = %v, want abc123", ct.Options["cert"])
		}
	}

	// Verify server transports
	if len(loaded.ServerTransports) != 1 {
		t.Errorf("expected 1 server transport, got %d", len(loaded.ServerTransports))
	} else {
		st := loaded.ServerTransports[0]
		if st.Name != "obfs4" || st.BinaryPath != "/usr/bin/obfs4proxy" {
			t.Errorf("server transport mismatch: %+v", st)
		}
		if st.BindAddr != "0.0.0.0:9443" {
			t.Errorf("server transport bind addr = %v, want 0.0.0.0:9443", st.BindAddr)
		}
		if st.Options["iat-mode"] != "1" {
			t.Errorf("server transport option iat-mode = %v, want 1", st.Options["iat-mode"])
		}
	}

	// Verify transport proxy
	if loaded.TransportProxy != cfg.TransportProxy {
		t.Errorf("TransportProxy = %v, want %v", loaded.TransportProxy, cfg.TransportProxy)
	}
}

func TestPTConfiguration_Validation(t *testing.T) {
	// Test that PT configuration passes validation
	cfg := DefaultConfig()
	cfg.ClientTransports = []ClientTransportConfig{
		{Name: "obfs4", BinaryPath: "/usr/bin/obfs4proxy"},
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() with PT config failed: %v", err)
	}
}
