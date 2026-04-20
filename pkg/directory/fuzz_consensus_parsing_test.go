// Package directory provides fuzzing tests for consensus document parsing.
//
// These fuzzing tests verify that consensus parsing never panics on arbitrary
// or malformed input, which is critical for a network-facing protocol
// implementation. Any panic in consensus parsing could be triggered by a
// malicious directory authority or man-in-the-middle attacker.
//
// Run the fuzz tests with:
//
//	go test -fuzz=FuzzParseConsensus -fuzztime=30s
//	go test -fuzz=FuzzParseConsensusParams -fuzztime=30s
//	go test -fuzz=FuzzParseMicrodescriptors -fuzztime=30s
//	go test -fuzz=FuzzParseAuthorityCert -fuzztime=30s
//
// Compliance: CWE-120 (Buffer Copy without Checking Size of Input),
// dir-spec.txt §3.4 (Consensus Format)
package directory

import (
	"strings"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/logger"
)

// newTestClient returns a minimal Client for fuzz testing.
func newTestClient() *Client {
	log := logger.NewDefault()
	return &Client{
		logger: log.Component("directory"),
		certCache: &AuthorityCertCache{
			certs:  make(map[string]*AuthorityCert),
			logger: log.Component("certcache"),
		},
	}
}

// FuzzParseConsensus verifies that parseConsensus never panics on
// arbitrary input. Consensus documents are fetched from untrusted
// directory authorities and must be parsed defensively.
func FuzzParseConsensus(f *testing.F) {
	c := newTestClient()

	// Seed: minimal valid consensus with one relay entry
	validConsensus := strings.Join([]string{
		"network-status-version 3 microdesc",
		"vote-status consensus",
		"valid-after 2026-01-01 00:00:00",
		"fresh-until 2026-01-01 01:00:00",
		"valid-until 2026-01-01 03:00:00",
		"params CircuitPriorityHalflifeMsec=30000",
		"r TestRelay AAAAAAAAAAAAAAAAAAAAAAAAAAAA 2026-01-01 00:00:00 127.0.0.1 9001 0",
		"m AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		"s Fast Guard Running Stable Valid",
		"w Bandwidth=1000",
	}, "\n")
	f.Add(validConsensus)

	// Seed: consensus with directory-signature blocks
	withSigs := strings.Join([]string{
		"network-status-version 3",
		"valid-after 2026-01-01 00:00:00",
		"valid-until 2026-01-01 03:00:00",
		"r Relay1 AAAA 2026-01-01 00:00:00 10.0.0.1 443 0",
		"s Running Valid",
		"directory-signature sha256 AAAA BBBB",
		"-----BEGIN SIGNATURE-----",
		"dGVzdA==",
		"-----END SIGNATURE-----",
		"directory-signature sha1 CCCC DDDD",
		"-----BEGIN SIGNATURE-----",
		"dGVzdDI=",
		"-----END SIGNATURE-----",
	}, "\n")
	f.Add(withSigs)

	// Seed: empty input
	f.Add("")

	// Seed: single line
	f.Add("r")

	// Seed: only header lines
	f.Add("network-status-version 3\nvalid-after 2026-01-01 00:00:00\n")

	// Seed: malformed relay entry (too few fields)
	f.Add("r short\nr also-short two\n")

	// Seed: relay with garbage port numbers
	f.Add("r Relay AAAA 2026-01-01 00:00:00 10.0.0.1 abc def\n")

	// Seed: relay with regular consensus format (9 fields)
	f.Add("r Relay AAAA digest 2026-01-01 00:00:00 10.0.0.1 443 80\n")

	// Seed: very long lines
	longLine := "r " + strings.Repeat("A", 10000) + "\n"
	f.Add(longLine)

	// Seed: consensus params with various formats
	f.Add("params foo=1 bar=2 baz=abc qux=-1\n")

	// Seed: bandwidth line edge cases
	f.Add("r R AAAA 2026-01-01 00:00:00 1.2.3.4 9001 0\nw Bandwidth=999999999999\n")

	// Seed: flag line edge cases
	f.Add("r R AAAA 2026-01-01 00:00:00 1.2.3.4 9001 0\ns\n")

	// Seed: mixed valid and malformed entries
	mixed := strings.Join([]string{
		"r Good AAAA 2026-01-01 00:00:00 1.2.3.4 443 0",
		"s Running Valid",
		"r",
		"r Bad",
		"r Good2 BBBB 2026-01-01 00:00:00 5.6.7.8 9001 0",
		"s Guard",
	}, "\n")
	f.Add(mixed)

	f.Fuzz(func(t *testing.T, data string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("parseConsensus panicked on %d bytes: %v",
					len(data), r)
			}
		}()

		_, _ = c.parseConsensus(strings.NewReader(data))
	})
}

// FuzzParseConsensusWithMetadata verifies that
// parseConsensusWithMetadata never panics on arbitrary input.
func FuzzParseConsensusWithMetadata(f *testing.F) {
	c := newTestClient()

	// Seed: valid consensus with metadata
	valid := strings.Join([]string{
		"network-status-version 3 microdesc",
		"valid-after 2026-01-01 00:00:00",
		"fresh-until 2026-01-01 01:00:00",
		"valid-until 2026-01-01 03:00:00",
		"params CircuitPriorityHalflifeMsec=30000",
		"r TestRelay AAAA 2026-01-01 00:00:00 127.0.0.1 9001 0",
		"s Running Valid",
		"w Bandwidth=500",
		"directory-signature sha256 IDENT SIGN",
		"-----BEGIN SIGNATURE-----",
		"c2ln",
		"-----END SIGNATURE-----",
	}, "\n")
	f.Add(valid)

	// Seed: empty
	f.Add("")

	// Seed: timestamps only
	f.Add("valid-after not-a-time\nfresh-until also-invalid\nvalid-until garbage\n")

	// Seed: signature with no PEM block
	f.Add("directory-signature sha256 A B\n")

	// Seed: directory-signature with only 2 parts
	f.Add("directory-signature A B\n")

	// Seed: nested signature blocks
	nested := strings.Join([]string{
		"directory-signature sha256 ID1 SK1",
		"-----BEGIN SIGNATURE-----",
		"directory-signature sha256 ID2 SK2",
		"-----END SIGNATURE-----",
	}, "\n")
	f.Add(nested)

	f.Fuzz(func(t *testing.T, data string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("parseConsensusWithMetadata panicked on %d bytes: %v",
					len(data), r)
			}
		}()

		_, _, _ = c.parseConsensusWithMetadata(strings.NewReader(data))
	})
}

// FuzzParseConsensusParams verifies that parseConsensusParams never
// panics on arbitrary parameter strings. These strings come from the
// "params" line of consensus documents.
func FuzzParseConsensusParams(f *testing.F) {
	// Seed: valid params
	f.Add("CircuitPriorityHalflifeMsec=30000 bwweightscale=10000")

	// Seed: empty
	f.Add("")

	// Seed: malformed params
	f.Add("noequals justkey=")
	f.Add("=nokey =")
	f.Add("key=notanumber key2=99999999999999999999")

	// Seed: duplicate keys
	f.Add("key=1 key=2 key=3")

	// Seed: very long key/value
	f.Add(strings.Repeat("k", 10000) + "=" + strings.Repeat("9", 10000))

	// Seed: special characters
	f.Add("key=value\x00null=test tab\t=space")

	f.Fuzz(func(t *testing.T, data string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("parseConsensusParams panicked on input %q: %v",
					data, r)
			}
		}()

		params := make(map[string]int)
		parseConsensusParams(data, params)
	})
}

// FuzzParseMicrodescriptors verifies that parseMicrodescriptors never
// panics on arbitrary microdescriptor data. These documents are fetched
// from directory mirrors and must be parsed defensively.
func FuzzParseMicrodescriptors(f *testing.F) {
	c := newTestClient()

	// Seed: valid microdescriptor
	valid := strings.Join([]string{
		"onion-key",
		"-----BEGIN RSA PUBLIC KEY-----",
		"MIGJAoGBAL0=",
		"-----END RSA PUBLIC KEY-----",
		"ntor-onion-key AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		"id ed25519",
		"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		"family $AAAA $BBBB",
		"",
	}, "\n")
	f.Add(valid)

	// Seed: empty
	f.Add("")

	// Seed: ntor key only (no identity key)
	f.Add("ntor-onion-key AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\n")

	// Seed: id line with no following key data
	f.Add("id ed25519\n")

	// Seed: invalid base64 in ntor key
	f.Add("ntor-onion-key !!!invalid-base64!!!\n")

	// Seed: invalid base64 in identity key
	f.Add("id ed25519\n!!!invalid-base64!!!\n")

	// Seed: ntor key wrong length
	f.Add("ntor-onion-key dGVzdA==\n")

	// Seed: multiple microdescriptors
	multi := strings.Join([]string{
		"ntor-onion-key AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		"id ed25519",
		"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		"",
		"ntor-onion-key BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB=",
		"id ed25519",
		"BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB=",
		"",
	}, "\n")
	f.Add(multi)

	// Seed: family with many members
	f.Add("family " + strings.Repeat("$AAAA ", 1000) + "\n")

	f.Fuzz(func(t *testing.T, data string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("parseMicrodescriptors panicked on %d bytes: %v",
					len(data), r)
			}
		}()

		digestMap := make(map[string][]*Relay)
		_ = c.parseMicrodescriptors([]byte(data), digestMap)
	})
}

// FuzzParseAuthorityCert verifies that parseAuthorityCert never panics
// on arbitrary certificate data. Authority certificates are fetched from
// remote servers and must be handled safely.
func FuzzParseAuthorityCert(f *testing.F) {
	log := logger.NewDefault()
	cache := &AuthorityCertCache{
		certs:  make(map[string]*AuthorityCert),
		logger: log.Component("certcache"),
	}

	// Seed: minimal cert structure
	validCert := strings.Join([]string{
		"dir-key-certificate-version 3",
		"fingerprint AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		"dir-key-expires 2027-01-01 00:00:00",
		"dir-signing-key",
		"-----BEGIN RSA PUBLIC KEY-----",
		"MIGJAoGBAL0=",
		"-----END RSA PUBLIC KEY-----",
	}, "\n")
	f.Add(validCert, "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")

	// Seed: empty data
	f.Add("", "TEST")

	// Seed: no fingerprint
	f.Add("dir-key-expires 2027-01-01 00:00:00\n", "MISSING")

	// Seed: malformed date
	f.Add("fingerprint TEST\ndir-key-expires not-a-date\n", "TEST")

	// Seed: no RSA key
	f.Add("fingerprint TEST\ndir-key-expires 2027-01-01 00:00:00\n", "TEST")

	// Seed: truncated PEM
	f.Add("fingerprint TEST\n-----BEGIN RSA PUBLIC KEY-----\n", "TEST")

	// Seed: garbage PEM content
	f.Add("fingerprint TEST\n-----BEGIN RSA PUBLIC KEY-----\nNOTBASE64!!!\n-----END RSA PUBLIC KEY-----\n", "TEST")

	f.Fuzz(func(t *testing.T, data string, identity string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("parseAuthorityCert panicked on %d bytes: %v",
					len(data), r)
			}
		}()

		_, _ = cache.parseAuthorityCert([]byte(data), identity)
	})
}

// FuzzValidateConsensusMetadata verifies that ValidateConsensusMetadata
// never panics on arbitrary metadata structures.
func FuzzValidateConsensusMetadata(f *testing.F) {
	// Seed: valid signature count and authority count
	f.Add(5, 5, true, true)

	// Seed: zero values
	f.Add(0, 0, false, false)

	// Seed: negative-like
	f.Add(-1, -1, true, true)

	// Seed: large values
	f.Add(1000, 1000, true, false)

	f.Fuzz(func(t *testing.T, sigCount int, authCount int, hasValidAfter bool, hasValidUntil bool) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("ValidateConsensusMetadata panicked: sigCount=%d authCount=%d: %v",
					sigCount, authCount, r)
			}
		}()

		meta := &ConsensusMetadata{
			SignatureCount: sigCount,
			AuthorityCount: authCount,
			Params:         make(map[string]int),
			Signatures:     make([]*ConsensusSignature, 0),
		}

		if hasValidAfter {
			meta.ValidAfter = time.Now().Add(-1 * time.Hour)
		}
		if hasValidUntil {
			meta.ValidUntil = time.Now().Add(2 * time.Hour)
		}

		// Populate signatures to match count
		if sigCount > 0 && sigCount < 100 {
			for i := range sigCount {
				meta.Signatures = append(meta.Signatures, &ConsensusSignature{
					Algorithm: "sha256",
					Identity:  strings.Repeat("A", 40),
					Signature: "test-sig-" + string(rune('A'+i)),
				})
			}
		}

		_ = ValidateConsensusMetadata(meta)
	})
}
