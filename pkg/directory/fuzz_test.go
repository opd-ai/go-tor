package directory

import (
	"strings"
	"testing"
)

// FuzzParseConsensusWithMetadata exercises the consensus parser with arbitrary
// byte sequences. The fuzzer verifies that no panic occurs on malformed input;
// parse errors are expected and acceptable.
func FuzzParseConsensusWithMetadata(f *testing.F) {
	// Seed corpus: minimal valid consensus header.
	minimalConsensus := strings.Join([]string{
		"network-status-version 3",
		"vote-status consensus",
		"consensus-method 29",
		"valid-after 2023-01-01 00:00:00",
		"fresh-until 2023-01-01 01:00:00",
		"valid-until 2023-01-01 03:00:00",
		"voting-delay 300 300",
		"known-flags Authority BadExit Exit Fast Guard HSDir NoEdConsensus Running Stable StaleDesc V2Dir Valid",
		"dir-source moria1 9695DFC35FFEB861329B9F1AB04C46397020CE31 128.31.0.39 9101 80",
		"contact 1024D/28988BF5 arma mit edu",
		"vote-digest 1234567890ABCDEF",
		"dir-source moria2 9695DFC35FFEB861329B9F1AB04C46397020CE32 128.31.0.40 9102 80",
		"contact 1024D/28988BF6 arma2 mit edu",
		"vote-digest ABCDEF1234567890",
		"dir-source moria3 9695DFC35FFEB861329B9F1AB04C46397020CE33 128.31.0.41 9103 80",
		"contact 1024D/28988BF7 arma3 mit edu",
		"vote-digest ABCDEF12345678AB",
		"dir-source moria4 9695DFC35FFEB861329B9F1AB04C46397020CE34 128.31.0.42 9104 80",
		"contact 1024D/28988BF8 arma4 mit edu",
		"vote-digest ABCDEF12345678AC",
		"dir-source moria5 9695DFC35FFEB861329B9F1AB04C46397020CE35 128.31.0.43 9105 80",
		"contact 1024D/28988BF9 arma5 mit edu",
		"vote-digest ABCDEF12345678AD",
		"params CircuitPriorityHalflifeMsec=30000",
		"directory-footer",
		"directory-signature sha256 9695DFC35FFEB861329B9F1AB04C46397020CE31 A1B2C3D4",
		"-----BEGIN SIGNATURE-----",
		"AAAA",
		"-----END SIGNATURE-----",
		"directory-signature sha256 9695DFC35FFEB861329B9F1AB04C46397020CE32 B1B2C3D4",
		"-----BEGIN SIGNATURE-----",
		"BBBB",
		"-----END SIGNATURE-----",
		"directory-signature sha256 9695DFC35FFEB861329B9F1AB04C46397020CE33 C1B2C3D4",
		"-----BEGIN SIGNATURE-----",
		"CCCC",
		"-----END SIGNATURE-----",
		"directory-signature sha256 9695DFC35FFEB861329B9F1AB04C46397020CE34 D1B2C3D4",
		"-----BEGIN SIGNATURE-----",
		"DDDD",
		"-----END SIGNATURE-----",
		"directory-signature sha256 9695DFC35FFEB861329B9F1AB04C46397020CE35 E1B2C3D4",
		"-----BEGIN SIGNATURE-----",
		"EEEE",
		"-----END SIGNATURE-----",
	}, "\n")
	f.Add(minimalConsensus)

	// Seed corpus: empty input.
	f.Add("")

	// Seed corpus: garbage input.
	f.Add("not a consensus at all\x00\x01\x02\x03")

	// Seed corpus: truncated header.
	f.Add("network-status-version 3\nvote-status ")

	// Seed corpus: extremely long line.
	f.Add("network-status-version 3\n" + strings.Repeat("A", 100000))

	c := NewClient(nil)

	f.Fuzz(func(t *testing.T, data string) {
		r := strings.NewReader(data)
		// Must not panic; errors are acceptable.
		_, _, _ = c.parseConsensusWithMetadata(r)
	})
}
