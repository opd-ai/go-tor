// Package crypto - Cross-implementation test vectors for protocol interoperability.
// Tests verify go-tor's cryptographic primitives against known-good vectors
// from C Tor (tor-spec.txt) and Arti (Rust Tor implementation).
//
// Vector files are in testdata/ at the repository root.
// Run with: go test -run TestCrossImpl ./pkg/crypto/...
package crypto

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
)

// testdataPath returns the path to the repository-root testdata directory
// relative to the package test working directory (pkg/crypto/ -> ../../testdata).
func testdataPath(elem ...string) string {
	parts := append([]string{"..", "..", "testdata"}, elem...)
	return filepath.Join(parts...)
}

// loadJSON reads and unmarshals a JSON test-vector file.
func loadJSON(t *testing.T, path string, v interface{}) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read vector file %s: %v", path, err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("failed to parse vector file %s: %v", path, err)
	}
}

// decodeHex decodes a hex string, failing the test on error.
func decodeHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("invalid hex %q: %v", s, err)
	}
	return b
}

// ---- SHA-1 vectors -------------------------------------------------------

type sha1VectorFile struct {
	Description string `json:"description"`
	Vectors     []struct {
		Comment string `json:"comment"`
		Input   string `json:"input"`
		Output  string `json:"output"`
	} `json:"vectors"`
}

// TestCrossImpl_SHA1_CTor verifies SHA-1 against C Tor compatible vectors.
func TestCrossImpl_SHA1_CTor(t *testing.T) {
	var vf sha1VectorFile
	loadJSON(t, testdataPath("ctor-vectors", "crypto", "sha1.json"), &vf)

	for _, vec := range vf.Vectors {
		vec := vec
		t.Run(vec.Comment, func(t *testing.T) {
			input := decodeHex(t, vec.Input)
			want := decodeHex(t, vec.Output)
			got := SHA1Hash(input)
			if !ConstantTimeCompare(got, want) {
				t.Errorf("SHA1Hash(%x):\n  got  %x\n  want %x", input, got, want)
			}
		})
	}
}

// ---- SHA-256 vectors -----------------------------------------------------

type sha256VectorFile struct {
	Description string `json:"description"`
	Vectors     []struct {
		Comment string `json:"comment"`
		Input   string `json:"input"`
		Output  string `json:"output"`
	} `json:"vectors"`
}

// TestCrossImpl_SHA256_CTor verifies SHA-256 against C Tor compatible vectors.
func TestCrossImpl_SHA256_CTor(t *testing.T) {
	var vf sha256VectorFile
	loadJSON(t, testdataPath("ctor-vectors", "crypto", "sha256.json"), &vf)

	for _, vec := range vf.Vectors {
		vec := vec
		t.Run(vec.Comment, func(t *testing.T) {
			input := decodeHex(t, vec.Input)
			want := decodeHex(t, vec.Output)
			got := SHA256Hash(input)
			if !ConstantTimeCompare(got, want) {
				t.Errorf("SHA256Hash(%x):\n  got  %x\n  want %x", input, got, want)
			}
		})
	}
}

// TestCrossImpl_SHA256_Arti verifies SHA-256 against Arti compatible vectors.
func TestCrossImpl_SHA256_Arti(t *testing.T) {
	var vf sha256VectorFile
	loadJSON(t, testdataPath("arti-vectors", "crypto", "sha256.json"), &vf)

	for _, vec := range vf.Vectors {
		vec := vec
		t.Run(vec.Comment, func(t *testing.T) {
			input := decodeHex(t, vec.Input)
			want := decodeHex(t, vec.Output)
			got := SHA256Hash(input)
			if !ConstantTimeCompare(got, want) {
				t.Errorf("SHA256Hash(%x):\n  got  %x\n  want %x", input, got, want)
			}
		})
	}
}

// ---- AES-CTR vectors -----------------------------------------------------

type aesCTRVectorFile struct {
	Description   string         `json:"description"`
	AES128Vectors []aesCTRVector `json:"aes128_vectors"`
	AES256Vectors []aesCTRVector `json:"aes256_vectors"`
}

type aesCTRVector struct {
	Comment    string `json:"comment"`
	Key        string `json:"key"`
	IV         string `json:"iv"`
	Plaintext  string `json:"plaintext"`
	Ciphertext string `json:"ciphertext"`
}

func runAESCTRVectors(t *testing.T, vecs []aesCTRVector) {
	t.Helper()
	for _, vec := range vecs {
		vec := vec
		t.Run(vec.Comment, func(t *testing.T) {
			key := decodeHex(t, vec.Key)
			iv := decodeHex(t, vec.IV)
			pt := decodeHex(t, vec.Plaintext)
			wantCT := decodeHex(t, vec.Ciphertext)

			// Test encryption
			cipher, err := NewAESCTRCipher(key, iv)
			if err != nil {
				t.Fatalf("NewAESCTRCipher: %v", err)
			}
			ct := make([]byte, len(pt))
			copy(ct, pt)
			cipher.Encrypt(ct)
			if !ConstantTimeCompare(ct, wantCT) {
				t.Errorf("Encrypt:\n  got  %x\n  want %x", ct, wantCT)
			}

			// Test decryption (CTR is symmetric)
			cipher2, err := NewAESCTRCipher(key, iv)
			if err != nil {
				t.Fatalf("NewAESCTRCipher: %v", err)
			}
			pt2 := make([]byte, len(wantCT))
			copy(pt2, wantCT)
			cipher2.Decrypt(pt2)
			if !ConstantTimeCompare(pt2, pt) {
				t.Errorf("Decrypt:\n  got  %x\n  want %x", pt2, pt)
			}
		})
	}
}

// TestCrossImpl_AES128CTR_CTor verifies AES-128-CTR against C Tor vectors.
func TestCrossImpl_AES128CTR_CTor(t *testing.T) {
	var vf aesCTRVectorFile
	loadJSON(t, testdataPath("ctor-vectors", "crypto", "aes_ctr.json"), &vf)
	runAESCTRVectors(t, vf.AES128Vectors)
}

// TestCrossImpl_AES256CTR_CTor verifies AES-256-CTR against C Tor vectors.
func TestCrossImpl_AES256CTR_CTor(t *testing.T) {
	var vf aesCTRVectorFile
	loadJSON(t, testdataPath("ctor-vectors", "crypto", "aes_ctr.json"), &vf)
	runAESCTRVectors(t, vf.AES256Vectors)
}

// TestCrossImpl_AES128CTR_Arti verifies AES-128-CTR against Arti vectors.
func TestCrossImpl_AES128CTR_Arti(t *testing.T) {
	var vf aesCTRVectorFile
	loadJSON(t, testdataPath("arti-vectors", "crypto", "aes_ctr.json"), &vf)
	runAESCTRVectors(t, vf.AES128Vectors)
}

// TestCrossImpl_AES256CTR_Arti verifies AES-256-CTR against Arti vectors.
func TestCrossImpl_AES256CTR_Arti(t *testing.T) {
	var vf aesCTRVectorFile
	loadJSON(t, testdataPath("arti-vectors", "crypto", "aes_ctr.json"), &vf)
	runAESCTRVectors(t, vf.AES256Vectors)
}

// ---- HKDF-SHA256 vectors -------------------------------------------------

type hkdfVectorFile struct {
	Description string `json:"description"`
	Vectors     []struct {
		Comment string `json:"comment"`
		IKM     string `json:"ikm"`
		Info    string `json:"info"`
		Length  int    `json:"length"`
		Output  string `json:"output"`
	} `json:"vectors"`
}

func runHKDFVectors(t *testing.T, path string) {
	t.Helper()
	var vf hkdfVectorFile
	loadJSON(t, path, &vf)

	for _, vec := range vf.Vectors {
		vec := vec
		t.Run(vec.Comment, func(t *testing.T) {
			ikm := decodeHex(t, vec.IKM)
			info := []byte(vec.Info)
			want := decodeHex(t, vec.Output)

			h := hkdf.New(sha256.New, ikm, nil, info)
			got := make([]byte, vec.Length)
			if _, err := io.ReadFull(h, got); err != nil {
				t.Fatalf("HKDF expand: %v", err)
			}
			if !ConstantTimeCompare(got, want) {
				t.Errorf("HKDF(%s):\n  got  %x\n  want %x", vec.Info, got, want)
			}
		})
	}
}

// TestCrossImpl_HKDF_CTor verifies HKDF-SHA256 against C Tor compatible vectors.
func TestCrossImpl_HKDF_CTor(t *testing.T) {
	runHKDFVectors(t, testdataPath("ctor-vectors", "crypto", "hkdf_ntor.json"))
}

// TestCrossImpl_HKDF_Arti verifies HKDF-SHA256 against Arti compatible vectors.
func TestCrossImpl_HKDF_Arti(t *testing.T) {
	runHKDFVectors(t, testdataPath("arti-vectors", "crypto", "hkdf_ntor.json"))
}

// ---- ntor handshake vectors ----------------------------------------------

type ntorVectorFile struct {
	Description string       `json:"description"`
	Vectors     []ntorVector `json:"vectors"`
}

type ntorVector struct {
	Comment        string `json:"comment"`
	NodeID         string `json:"node_id"`
	ServerIdentity string `json:"server_identity"`
	ServerBPrivate string `json:"server_b_private"`
	ServerBPublic  string `json:"server_b_public"`
	ClientXPrivate string `json:"client_x_private"`
	ClientXPublic  string `json:"client_x_public"`
	ServerYPrivate string `json:"server_y_private"`
	ServerYPublic  string `json:"server_y_public"`
	Auth           string `json:"auth"`
	KeyMaterial    string `json:"key_material"`
	HandshakeData  string `json:"handshake_data"`
	ServerResponse string `json:"server_response"`
}

func runNtorVectors(t *testing.T, path string) {
	t.Helper()
	var vf ntorVectorFile
	loadJSON(t, path, &vf)

	for _, vec := range vf.Vectors {
		vec := vec
		t.Run(vec.Comment, func(t *testing.T) {
			nodeID := decodeHex(t, vec.NodeID)
			serverIdentity := decodeHex(t, vec.ServerIdentity)
			serverBPrivate := decodeHex(t, vec.ServerBPrivate)
			serverBPublic := decodeHex(t, vec.ServerBPublic)
			clientXPrivate := decodeHex(t, vec.ClientXPrivate)
			clientXPublic := decodeHex(t, vec.ClientXPublic)
			serverYPrivate := decodeHex(t, vec.ServerYPrivate)
			serverYPublic := decodeHex(t, vec.ServerYPublic)
			wantAuth := decodeHex(t, vec.Auth)
			wantKeyMaterial := decodeHex(t, vec.KeyMaterial)
			wantServerResponse := decodeHex(t, vec.ServerResponse)
			wantHandshake := decodeHex(t, vec.HandshakeData)

			// Verify public keys derived from private keys
			var bPriv, bPub [32]byte
			copy(bPriv[:], serverBPrivate)
			curve25519.ScalarBaseMult(&bPub, &bPriv)
			if !ConstantTimeCompare(bPub[:], serverBPublic) {
				t.Errorf("server B public key mismatch:\n  got  %x\n  want %x", bPub[:], serverBPublic)
			}

			var xPriv, xPub [32]byte
			copy(xPriv[:], clientXPrivate)
			curve25519.ScalarBaseMult(&xPub, &xPriv)
			if !ConstantTimeCompare(xPub[:], clientXPublic) {
				t.Errorf("client X public key mismatch:\n  got  %x\n  want %x", xPub[:], clientXPublic)
			}

			var yPriv, yPub [32]byte
			copy(yPriv[:], serverYPrivate)
			curve25519.ScalarBaseMult(&yPub, &yPriv)
			if !ConstantTimeCompare(yPub[:], serverYPublic) {
				t.Errorf("server Y public key mismatch:\n  got  %x\n  want %x", yPub[:], serverYPublic)
			}

			// Verify handshake_data: nodeID[0:20] || serverBPublic || clientXPublic
			if len(wantHandshake) == 84 {
				gotHandshake := make([]byte, 84)
				copy(gotHandshake[0:20], nodeID[:20])
				copy(gotHandshake[20:52], serverBPublic)
				copy(gotHandshake[52:84], clientXPublic)
				if !ConstantTimeCompare(gotHandshake, wantHandshake) {
					t.Errorf("handshake_data mismatch:\n  got  %x\n  want %x", gotHandshake, wantHandshake)
				}
			}

			// Verify server_response contains the expected auth value
			if !ConstantTimeCompare(wantServerResponse[32:64], wantAuth) {
				t.Errorf("auth in server_response mismatch")
			}

			// Verify server response format: serverYPublic || auth
			gotServerResp := make([]byte, 64)
			copy(gotServerResp[0:32], serverYPublic)
			copy(gotServerResp[32:64], wantAuth)
			if !ConstantTimeCompare(gotServerResp, wantServerResponse) {
				t.Errorf("server_response mismatch:\n  got  %x\n  want %x",
					gotServerResp, wantServerResponse)
			}

			// Test server-side response and key material via ntorServerHandshakeWithKeys
			var handshakeData []byte
			if len(wantHandshake) == 84 {
				handshakeData = wantHandshake
			} else {
				handshakeData = make([]byte, 84)
				copy(handshakeData[0:20], nodeID[:20])
				copy(handshakeData[20:52], serverBPublic)
				copy(handshakeData[52:84], clientXPublic)
			}
			gotServerResp, serverKeyMat, err := ntorServerHandshakeWithKeys(
				handshakeData, serverBPrivate, serverIdentity, serverYPrivate,
			)
			if err != nil {
				t.Fatalf("ntorServerHandshakeWithKeys: %v", err)
			}
			if !ConstantTimeCompare(gotServerResp, wantServerResponse) {
				t.Errorf("server response mismatch:\n  got  %x\n  want %x",
					gotServerResp, wantServerResponse)
			}
			if !ConstantTimeCompare(serverKeyMat, wantKeyMaterial) {
				t.Errorf("server key_material mismatch:\n  got  %x\n  want %x",
					serverKeyMat, wantKeyMaterial)
			}

			// Test client-side key material via NtorProcessResponse
			clientKeyMat, err := NtorProcessResponse(
				wantServerResponse, clientXPrivate, serverBPublic, serverIdentity,
			)
			if err != nil {
				t.Fatalf("NtorProcessResponse: %v", err)
			}
			if !ConstantTimeCompare(clientKeyMat, wantKeyMaterial) {
				t.Errorf("client key_material mismatch:\n  got  %x\n  want %x",
					clientKeyMat, wantKeyMaterial)
			}
		})
	}
}

// TestCrossImpl_Ntor_CTor verifies the ntor handshake against C Tor vectors.
func TestCrossImpl_Ntor_CTor(t *testing.T) {
	runNtorVectors(t, testdataPath("ctor-vectors", "crypto", "ntor_handshake.json"))
}

// TestCrossImpl_Ntor_Arti verifies the ntor handshake against Arti vectors.
func TestCrossImpl_Ntor_Arti(t *testing.T) {
	runNtorVectors(t, testdataPath("arti-vectors", "crypto", "ntor_handshake.json"))
}

// ---- KDF-TOR vectors -----------------------------------------------------

type kdfTORVectorFile struct {
	Description string `json:"description"`
	Vectors     []struct {
		Comment string `json:"comment"`
		Secret  string `json:"secret"`
		KeyLen  int    `json:"key_len"`
		Output  string `json:"output"`
	} `json:"vectors"`
}

// TestCrossImpl_KDFTOR_CTor verifies KDF-TOR against C Tor compatible vectors.
func TestCrossImpl_KDFTOR_CTor(t *testing.T) {
	var vf kdfTORVectorFile
	loadJSON(t, testdataPath("ctor-vectors", "crypto", "kdf_tor.json"), &vf)

	for _, vec := range vf.Vectors {
		vec := vec
		t.Run(vec.Comment, func(t *testing.T) {
			secret := decodeHex(t, vec.Secret)
			want := decodeHex(t, vec.Output)

			got, err := DeriveKey(secret, vec.KeyLen)
			if err != nil {
				t.Fatalf("DeriveKey: %v", err)
			}
			if !ConstantTimeCompare(got, want) {
				t.Errorf("DeriveKey(%x, %d):\n  got  %x\n  want %x",
					secret, vec.KeyLen, got, want)
			}
		})
	}
}
