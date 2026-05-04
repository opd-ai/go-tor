// Package cell - Cross-implementation test vectors for protocol interoperability.
// Tests verify go-tor's cell encoding against known-good wire format vectors
// from C Tor (tor-spec.txt §3) and Arti (Rust Tor implementation).
//
// Vector files are in testdata/ at the repository root.
// Run with: go test -run TestCrossImpl ./pkg/cell/...
package cell

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// testdataPath returns the path to the repository-root testdata directory
// relative to the package test working directory (pkg/cell/ -> ../../testdata).
func testdataPath(elem ...string) string {
	parts := append([]string{"..", "..", "testdata"}, elem...)
	return filepath.Join(parts...)
}

// loadCellJSON reads and unmarshals a JSON test-vector file.
func loadCellJSON(t *testing.T, path string, v interface{}) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read vector file %s: %v", path, err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("failed to parse vector file %s: %v", path, err)
	}
}

// decodeCellHex decodes a hex string, failing the test on error.
func decodeCellHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("invalid hex %q: %v", s, err)
	}
	return b
}

// ---- Fixed cell vectors --------------------------------------------------

type fixedCellVectorFile struct {
	Description string `json:"description"`
	Vectors     []struct {
		Comment          string `json:"comment"`
		CircID           uint32 `json:"circ_id"`
		Command          byte   `json:"command"`
		CommandName      string `json:"command_name"`
		PayloadPrefix    string `json:"payload_prefix"`
		PayloadPrefixLen int    `json:"payload_prefix_len"`
		TotalEncodedLen  int    `json:"total_encoded_len"`
		Encoded          string `json:"encoded"`
	} `json:"vectors"`
}

func runFixedCellVectors(t *testing.T, path string) {
	t.Helper()
	var vf fixedCellVectorFile
	loadCellJSON(t, path, &vf)

	for _, vec := range vf.Vectors {
		vec := vec
		t.Run(vec.Comment, func(t *testing.T) {
			wantEncoded := decodeCellHex(t, vec.Encoded)

			if len(wantEncoded) != vec.TotalEncodedLen {
				t.Fatalf("vector encoded length %d != total_encoded_len %d",
					len(wantEncoded), vec.TotalEncodedLen)
			}

			// Decode the known-good wire bytes
			decoded, err := DecodeCell(bytes.NewReader(wantEncoded))
			if err != nil {
				t.Fatalf("DecodeCell: %v", err)
			}

			// Verify decoded fields match the vector
			if decoded.CircID != vec.CircID {
				t.Errorf("CircID = %d, want %d", decoded.CircID, vec.CircID)
			}
			if byte(decoded.Command) != vec.Command {
				t.Errorf("Command = %d (%s), want %d (%s)",
					decoded.Command, decoded.Command.String(),
					vec.Command, vec.CommandName)
			}
			if decoded.Command.IsVariableLength() {
				t.Errorf("expected fixed-length cell but got variable: %s", decoded.Command)
			}
			if len(decoded.Payload) != PayloadLen {
				t.Errorf("payload length = %d, want %d", len(decoded.Payload), PayloadLen)
			}

			// Verify the payload prefix bytes from the vector match the decoded payload
			if vec.PayloadPrefix != "" {
				wantPrefix := decodeCellHex(t, vec.PayloadPrefix)
				if len(wantPrefix) != vec.PayloadPrefixLen {
					t.Fatalf("payload_prefix hex length %d != payload_prefix_len %d",
						len(wantPrefix), vec.PayloadPrefixLen)
				}
				if !bytes.Equal(decoded.Payload[:vec.PayloadPrefixLen], wantPrefix) {
					t.Errorf("payload prefix mismatch (first %d bytes):\n  got  %x\n  want %x",
						vec.PayloadPrefixLen,
						decoded.Payload[:vec.PayloadPrefixLen],
						wantPrefix)
				}
			}

			// Re-encode and verify the wire bytes match
			var buf bytes.Buffer
			if err := decoded.Encode(&buf); err != nil {
				t.Fatalf("Encode: %v", err)
			}
			gotEncoded := buf.Bytes()
			if !bytes.Equal(gotEncoded, wantEncoded) {
				t.Errorf("re-encode mismatch (first 10 bytes):\n  got  %x...\n  want %x...",
					gotEncoded[:min(10, len(gotEncoded))],
					wantEncoded[:min(10, len(wantEncoded))])
			}
		})
	}
}

// TestCrossImpl_FixedCell_CTor verifies fixed-size cell encoding against C Tor vectors.
func TestCrossImpl_FixedCell_CTor(t *testing.T) {
	runFixedCellVectors(t, testdataPath("ctor-vectors", "cell", "fixed_cell.json"))
}

// ---- Variable cell vectors -----------------------------------------------

type variableCellVectorFile struct {
	Description string `json:"description"`
	Vectors     []struct {
		Comment         string `json:"comment"`
		CircID          uint32 `json:"circ_id"`
		Command         byte   `json:"command"`
		CommandName     string `json:"command_name"`
		Payload         string `json:"payload"`
		PayloadLen      int    `json:"payload_len"`
		TotalEncodedLen int    `json:"total_encoded_len"`
		Encoded         string `json:"encoded"`
	} `json:"vectors"`
}

func runVariableCellVectors(t *testing.T, path string) {
	t.Helper()
	var vf variableCellVectorFile
	loadCellJSON(t, path, &vf)

	for _, vec := range vf.Vectors {
		vec := vec
		t.Run(vec.Comment, func(t *testing.T) {
			wantEncoded := decodeCellHex(t, vec.Encoded)

			if len(wantEncoded) != vec.TotalEncodedLen {
				t.Fatalf("vector encoded length %d != total_encoded_len %d",
					len(wantEncoded), vec.TotalEncodedLen)
			}

			// Decode the known-good wire bytes
			decoded, err := DecodeCell(bytes.NewReader(wantEncoded))
			if err != nil {
				t.Fatalf("DecodeCell: %v", err)
			}

			// Verify decoded fields
			if decoded.CircID != vec.CircID {
				t.Errorf("CircID = %d, want %d", decoded.CircID, vec.CircID)
			}
			if byte(decoded.Command) != vec.Command {
				t.Errorf("Command = %d (%s), want %d (%s)",
					decoded.Command, decoded.Command.String(),
					vec.Command, vec.CommandName)
			}
			if !decoded.Command.IsVariableLength() {
				t.Errorf("expected variable-length cell but got fixed: %s", decoded.Command)
			}
			if len(decoded.Payload) != vec.PayloadLen {
				t.Errorf("payload length = %d, want %d", len(decoded.Payload), vec.PayloadLen)
			}

			// Verify the exact payload bytes from the vector
			wantPayload := decodeCellHex(t, vec.Payload)
			if !bytes.Equal(decoded.Payload, wantPayload) {
				t.Errorf("payload bytes mismatch:\n  got  %x\n  want %x", decoded.Payload, wantPayload)
			}

			// Re-encode and verify the wire bytes match
			var buf bytes.Buffer
			if err := decoded.Encode(&buf); err != nil {
				t.Fatalf("Encode: %v", err)
			}
			gotEncoded := buf.Bytes()
			if !bytes.Equal(gotEncoded, wantEncoded) {
				t.Errorf("re-encode mismatch:\n  got  %x\n  want %x", gotEncoded, wantEncoded)
			}
		})
	}
}

// TestCrossImpl_VariableCell_CTor verifies variable-cell encoding against C Tor vectors.
func TestCrossImpl_VariableCell_CTor(t *testing.T) {
	runVariableCellVectors(t, testdataPath("ctor-vectors", "cell", "variable_cell.json"))
}

// ---- Arti cell encoding vectors ------------------------------------------

type artiCellVectorFile struct {
	Description      string `json:"description"`
	FixedCellVectors []struct {
		Comment string `json:"comment"`
		CircID  uint32 `json:"circ_id"`
		Command byte   `json:"command"`
		Encoded string `json:"encoded"`
	} `json:"fixed_cell_vectors"`
	VariableCellVectors []struct {
		Comment string `json:"comment"`
		CircID  uint32 `json:"circ_id"`
		Command byte   `json:"command"`
		Encoded string `json:"encoded"`
	} `json:"variable_cell_vectors"`
}

// TestCrossImpl_Cell_Arti verifies cell encoding against Arti compatible vectors.
func TestCrossImpl_Cell_Arti(t *testing.T) {
	var vf artiCellVectorFile
	loadCellJSON(t, testdataPath("arti-vectors", "cell", "cell_encoding.json"), &vf)

	t.Run("fixed", func(t *testing.T) {
		for _, vec := range vf.FixedCellVectors {
			vec := vec
			t.Run(vec.Comment, func(t *testing.T) {
				wantEncoded := decodeCellHex(t, vec.Encoded)
				decoded, err := DecodeCell(bytes.NewReader(wantEncoded))
				if err != nil {
					t.Fatalf("DecodeCell: %v", err)
				}
				if decoded.CircID != vec.CircID {
					t.Errorf("CircID = %d, want %d", decoded.CircID, vec.CircID)
				}
				if byte(decoded.Command) != vec.Command {
					t.Errorf("Command = %d, want %d", decoded.Command, vec.Command)
				}
				var buf bytes.Buffer
				if err := decoded.Encode(&buf); err != nil {
					t.Fatalf("Encode: %v", err)
				}
				if !bytes.Equal(buf.Bytes(), wantEncoded) {
					t.Errorf("re-encode mismatch")
				}
			})
		}
	})

	t.Run("variable", func(t *testing.T) {
		for _, vec := range vf.VariableCellVectors {
			vec := vec
			t.Run(vec.Comment, func(t *testing.T) {
				wantEncoded := decodeCellHex(t, vec.Encoded)
				decoded, err := DecodeCell(bytes.NewReader(wantEncoded))
				if err != nil {
					t.Fatalf("DecodeCell: %v", err)
				}
				if decoded.CircID != vec.CircID {
					t.Errorf("CircID = %d, want %d", decoded.CircID, vec.CircID)
				}
				if byte(decoded.Command) != vec.Command {
					t.Errorf("Command = %d, want %d", decoded.Command, vec.Command)
				}
				var buf bytes.Buffer
				if err := decoded.Encode(&buf); err != nil {
					t.Fatalf("Encode: %v", err)
				}
				if !bytes.Equal(buf.Bytes(), wantEncoded) {
					t.Errorf("re-encode mismatch:\n  got  %x\n  want %x", buf.Bytes(), wantEncoded)
				}
			})
		}
	})
}
