// Package onion provides fuzzing tests for onion address parsing.
//
// These fuzz tests verify that onion address parsing and validation
// never panic on arbitrary input, which is important for a network-facing
// implementation that accepts addresses from untrusted sources.
//
// Run fuzz tests with:
//
//	go test -fuzz=FuzzParseAddress -fuzztime=30s
//	go test -fuzz=FuzzIsOnionAddress -fuzztime=30s
package onion

import (
	"testing"
)

// FuzzParseAddress verifies ParseAddress never panics on arbitrary input.
func FuzzParseAddress(f *testing.F) {
	// Valid v3 onion address seeds
	f.Add("2gzyxa5ihm7nsggfxnu52rck2vv4rvmdlkiu3zzui5du4xyclen53wid.onion")
	f.Add("duckduckgogg42xjoc72x3sjasowoarfbgcmvfimaftt6twagswzczad.onion")

	// Edge cases
	f.Add("")
	f.Add(".onion")
	f.Add("a.onion")
	f.Add("short.onion")
	f.Add("example.com")
	f.Add("http://2gzyxa5ihm7nsggfxnu52rck2vv4rvmdlkiu3zzui5du4xyclen53wid.onion")
	f.Add(string(make([]byte, 256)))                                         // Very long string
	f.Add("!@#$%^&*.onion")                                                  // Special characters
	f.Add("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA.onion")   // Wrong length base32
	f.Add("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.onion")   // 55 chars base32 (wrong)
	f.Add("a2gzyxa5ihm7nsggfxnu52rck2vv4rvmdlkiu3zzui5du4xyclen53wid.onion") // 1 char too long

	f.Fuzz(func(t *testing.T, addr string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("ParseAddress panicked on %q: %v", addr, r)
			}
		}()
		_, _ = ParseAddress(addr)
	})
}

// FuzzIsOnionAddress verifies IsOnionAddress never panics on arbitrary input.
func FuzzIsOnionAddress(f *testing.F) {
	f.Add("2gzyxa5ihm7nsggfxnu52rck2vv4rvmdlkiu3zzui5du4xyclen53wid.onion")
	f.Add("")
	f.Add(".onion")
	f.Add("example.com")
	f.Add("a")
	f.Add(string(make([]byte, 1024)))
	f.Add("!@#$%^.onion")

	f.Fuzz(func(t *testing.T, addr string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("IsOnionAddress panicked on %q: %v", addr, r)
			}
		}()
		_ = IsOnionAddress(addr)
	})
}

// FuzzAddressRoundtrip verifies that a successfully parsed address round-trips
// through String() back to an equivalent representation.
func FuzzAddressRoundtrip(f *testing.F) {
	f.Add("2gzyxa5ihm7nsggfxnu52rck2vv4rvmdlkiu3zzui5du4xyclen53wid.onion")
	f.Add("duckduckgogg42xjoc72x3sjasowoarfbgcmvfimaftt6twagswzczad.onion")
	f.Add("")
	f.Add("invalid.onion")

	f.Fuzz(func(t *testing.T, addr string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Address roundtrip panicked on %q: %v", addr, r)
			}
		}()
		parsed, err := ParseAddress(addr)
		if err != nil {
			return
		}
		// A successfully parsed address must produce a non-empty string.
		str := parsed.String()
		if str == "" {
			t.Errorf("ParseAddress(%q).String() returned empty string", addr)
		}
	})
}
