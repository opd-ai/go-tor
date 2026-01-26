package security

import (
	"bytes"
	"crypto/rand"
	"crypto/subtle"
	"fmt"
	"testing"
)

// TestConstantTimeCompareCorrectness verifies the constant-time comparison function
// produces correct results for various input combinations
func TestConstantTimeCompareCorrectness(t *testing.T) {
	tests := []struct {
		name string
		a    []byte
		b    []byte
		want bool
	}{
		{
			name: "Equal slices",
			a:    []byte("hello world"),
			b:    []byte("hello world"),
			want: true,
		},
		{
			name: "Different slices",
			a:    []byte("hello world"),
			b:    []byte("hello earth"),
			want: false,
		},
		{
			name: "Different lengths",
			a:    []byte("hello"),
			b:    []byte("hello world"),
			want: false,
		},
		{
			name: "Empty slices",
			a:    []byte{},
			b:    []byte{},
			want: true,
		},
		{
			name: "Nil vs empty",
			a:    nil,
			b:    []byte{},
			want: true, // Both have length 0, so they're equal
		},
		{
			name: "Both nil",
			a:    nil,
			b:    nil,
			want: true,
		},
		{
			name: "Single byte difference at start",
			a:    []byte{0x01, 0x02, 0x03, 0x04},
			b:    []byte{0xFF, 0x02, 0x03, 0x04},
			want: false,
		},
		{
			name: "Single byte difference at end",
			a:    []byte{0x01, 0x02, 0x03, 0x04},
			b:    []byte{0x01, 0x02, 0x03, 0xFF},
			want: false,
		},
		{
			name: "32-byte keys equal",
			a:    bytes.Repeat([]byte{0xAB}, 32),
			b:    bytes.Repeat([]byte{0xAB}, 32),
			want: true,
		},
		{
			name: "32-byte keys different (last byte)",
			a:    append(bytes.Repeat([]byte{0xAB}, 31), 0xAB),
			b:    append(bytes.Repeat([]byte{0xAB}, 31), 0xAC),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConstantTimeCompare(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("ConstantTimeCompare() = %v, want %v", got, tt.want)
			}

			// Verify consistency with stdlib subtle.ConstantTimeCompare
			if len(tt.a) == len(tt.b) {
				stdlibResult := subtle.ConstantTimeCompare(tt.a, tt.b) == 1
				if got != stdlibResult {
					t.Errorf("ConstantTimeCompare() = %v, stdlib = %v (inconsistent)", got, stdlibResult)
				}
			}
		})
	}
}

// TestConstantTimeCompareAgainstStdlib verifies our implementation matches stdlib behavior
func TestConstantTimeCompareAgainstStdlib(t *testing.T) {
	// Generate 1000 random test cases
	for i := 0; i < 1000; i++ {
		// Random lengths from 0 to 64 bytes
		length := i % 65

		a := make([]byte, length)
		b := make([]byte, length)

		if _, err := rand.Read(a); err != nil {
			t.Fatalf("Failed to generate random data: %v", err)
		}
		if _, err := rand.Read(b); err != nil {
			t.Fatalf("Failed to generate random data: %v", err)
		}

		// Test with equal slices
		got := ConstantTimeCompare(a, a)
		stdlibResult := subtle.ConstantTimeCompare(a, a) == 1
		if got != stdlibResult {
			t.Errorf("Equal slices: ConstantTimeCompare() = %v, stdlib = %v", got, stdlibResult)
		}
		if !got {
			t.Error("Equal slices should return true")
		}

		// Test with different slices (if they happen to be different)
		if !bytes.Equal(a, b) {
			got = ConstantTimeCompare(a, b)
			stdlibResult = subtle.ConstantTimeCompare(a, b) == 1
			if got != stdlibResult {
				t.Errorf("Different slices: ConstantTimeCompare() = %v, stdlib = %v", got, stdlibResult)
			}
		}
	}
}

// TestConstantTimeCompare_CryptographicUseCase verifies constant-time comparison
// works correctly for common cryptographic use cases
func TestConstantTimeCompare_CryptographicUseCase(t *testing.T) {
	t.Run("32-byte keys (AES-256)", func(t *testing.T) {
		key1 := make([]byte, 32)
		key2 := make([]byte, 32)
		rand.Read(key1)
		copy(key2, key1)

		// Same key
		if !ConstantTimeCompare(key1, key2) {
			t.Error("Same 32-byte keys should be equal")
		}

		// Different key (flip one bit)
		key2[15] ^= 0x01
		if ConstantTimeCompare(key1, key2) {
			t.Error("Different 32-byte keys should not be equal")
		}
	})

	t.Run("16-byte MACs (HMAC-SHA256 truncated)", func(t *testing.T) {
		mac1 := make([]byte, 16)
		mac2 := make([]byte, 16)
		rand.Read(mac1)
		copy(mac2, mac1)

		// Same MAC
		if !ConstantTimeCompare(mac1, mac2) {
			t.Error("Same 16-byte MACs should be equal")
		}

		// Different MAC
		mac2[0] ^= 0x80
		if ConstantTimeCompare(mac1, mac2) {
			t.Error("Different 16-byte MACs should not be equal")
		}
	})

	t.Run("4-byte digests (relay cell digest)", func(t *testing.T) {
		digest1 := []byte{0x12, 0x34, 0x56, 0x78}
		digest2 := []byte{0x12, 0x34, 0x56, 0x78}

		// Same digest
		if !ConstantTimeCompare(digest1, digest2) {
			t.Error("Same 4-byte digests should be equal")
		}

		// Different digest
		digest2[3] = 0x79
		if ConstantTimeCompare(digest1, digest2) {
			t.Error("Different 4-byte digests should not be equal")
		}
	})

	t.Run("Passwords (variable length)", func(t *testing.T) {
		password1 := []byte("MySecurePassword123!")
		password2 := []byte("MySecurePassword123!")

		// Same password
		if !ConstantTimeCompare(password1, password2) {
			t.Error("Same passwords should be equal")
		}

		// Different password (last char)
		password2[len(password2)-1] = '?'
		if ConstantTimeCompare(password1, password2) {
			t.Error("Different passwords should not be equal")
		}

		// Different password (first char)
		password3 := []byte("mySecurePassword123!")
		if ConstantTimeCompare(password1, password3) {
			t.Error("Different passwords (case) should not be equal")
		}

		// Different length
		password4 := []byte("MySecurePassword123")
		if ConstantTimeCompare(password1, password4) {
			t.Error("Different length passwords should not be equal")
		}
	})
}

// TestConstantTimeCompare_EdgeCases tests edge cases and error conditions
func TestConstantTimeCompare_EdgeCases(t *testing.T) {
	t.Run("Very large slices", func(t *testing.T) {
		// Test with 1MB slices
		size := 1024 * 1024
		a := make([]byte, size)
		b := make([]byte, size)

		rand.Read(a)
		copy(b, a)

		if !ConstantTimeCompare(a, b) {
			t.Error("Large equal slices should be equal")
		}

		// Flip one bit in the middle
		b[size/2] ^= 0x01
		if ConstantTimeCompare(a, b) {
			t.Error("Large different slices should not be equal")
		}
	})

	t.Run("All zero bytes", func(t *testing.T) {
		a := make([]byte, 32)
		b := make([]byte, 32)

		if !ConstantTimeCompare(a, b) {
			t.Error("All-zero slices should be equal")
		}
	})

	t.Run("All 0xFF bytes", func(t *testing.T) {
		a := bytes.Repeat([]byte{0xFF}, 32)
		b := bytes.Repeat([]byte{0xFF}, 32)

		if !ConstantTimeCompare(a, b) {
			t.Error("All-0xFF slices should be equal")
		}
	})

	t.Run("One zero, one non-zero", func(t *testing.T) {
		a := make([]byte, 32)
		b := bytes.Repeat([]byte{0x01}, 32)

		if ConstantTimeCompare(a, b) {
			t.Error("Zero and non-zero slices should not be equal")
		}
	})
}

// TestConstantTimeCompare_SecurityProperties verifies security-relevant properties
func TestConstantTimeCompare_SecurityProperties(t *testing.T) {
	t.Run("Early mismatch still processes all bytes", func(t *testing.T) {
		// This test verifies behavioral correctness
		// (We can't actually measure timing in a unit test reliably)

		a := []byte{0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
		b := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}

		// Even though first byte differs, function should return correct result
		if ConstantTimeCompare(a, b) {
			t.Error("Different slices should return false")
		}

		// Verify it matches stdlib behavior
		if ConstantTimeCompare(a, b) != (subtle.ConstantTimeCompare(a, b) == 1) {
			t.Error("Should match stdlib behavior")
		}
	})

	t.Run("Late mismatch behaves same as early mismatch", func(t *testing.T) {
		a := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xFF}
		b := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}

		// Last byte differs
		if ConstantTimeCompare(a, b) {
			t.Error("Different slices should return false")
		}

		// Both early and late mismatch should return false
		earlyMismatch := []byte{0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
		if ConstantTimeCompare(earlyMismatch, b) != ConstantTimeCompare(a, b) {
			t.Error("Early and late mismatch should both return false")
		}
	})

	t.Run("Nil handling is safe", func(t *testing.T) {
		// Nil slices should not cause panic
		var nilSlice []byte
		empty := []byte{}

		// nil vs nil (both length 0 = equal)
		if !ConstantTimeCompare(nilSlice, nilSlice) {
			t.Error("nil vs nil should be equal")
		}

		// nil vs empty (both length 0 = equal)
		if !ConstantTimeCompare(nilSlice, empty) {
			t.Error("nil vs empty should be equal (both length 0)")
		}

		// empty vs nil (both length 0 = equal)
		if !ConstantTimeCompare(empty, nilSlice) {
			t.Error("empty vs nil should be equal (both length 0)")
		}

		// Nil vs non-empty (different lengths = not equal)
		nonEmpty := []byte{0x01}
		if ConstantTimeCompare(nilSlice, nonEmpty) {
			t.Error("nil vs non-empty should not be equal")
		}
	})
}

// TestConstantTimeCompare_RegressionTests tests specific scenarios from the audit
func TestConstantTimeCompare_RegressionTests(t *testing.T) {
	t.Run("ntor AUTH MAC (32 bytes)", func(t *testing.T) {
		// Simulate ntor AUTH verification
		auth := make([]byte, 32)
		expectedAuth := make([]byte, 32)
		rand.Read(auth)
		copy(expectedAuth, auth)

		// Correct AUTH
		if !ConstantTimeCompare(auth, expectedAuth) {
			t.Error("Correct ntor AUTH should verify")
		}

		// Incorrect AUTH
		expectedAuth[15] ^= 0x01
		if ConstantTimeCompare(auth, expectedAuth) {
			t.Error("Incorrect ntor AUTH should fail")
		}
	})

	t.Run("Client auth MAC (16 bytes truncated)", func(t *testing.T) {
		// Simulate client authorization MAC verification
		mac := make([]byte, 16)
		computedMAC := make([]byte, 16)
		rand.Read(mac)
		copy(computedMAC, mac)

		// Correct MAC
		if !ConstantTimeCompare(mac, computedMAC) {
			t.Error("Correct client auth MAC should verify")
		}

		// Incorrect MAC
		computedMAC[0] ^= 0x80
		if ConstantTimeCompare(mac, computedMAC) {
			t.Error("Incorrect client auth MAC should fail")
		}
	})

	t.Run("Relay cell digest (4 bytes)", func(t *testing.T) {
		// Simulate circuit relay cell digest verification
		digest := []byte{0x12, 0x34, 0x56, 0x78}
		expected := []byte{0x12, 0x34, 0x56, 0x78}

		// Correct digest
		if !ConstantTimeCompare(digest, expected) {
			t.Error("Correct relay cell digest should verify")
		}

		// Incorrect digest
		expected[2] = 0x57
		if ConstantTimeCompare(digest, expected) {
			t.Error("Incorrect relay cell digest should fail")
		}
	})

	t.Run("Password comparison (variable length)", func(t *testing.T) {
		// This demonstrates the CORRECT way to compare passwords
		password := []byte("MySecurePassword123!")
		stored := []byte("MySecurePassword123!")

		// Correct password
		if !ConstantTimeCompare(password, stored) {
			t.Error("Correct password should verify")
		}

		// Incorrect password (first byte)
		wrongPassword := []byte("mySecurePassword123!")
		if ConstantTimeCompare(wrongPassword, stored) {
			t.Error("Incorrect password should fail")
		}

		// Incorrect password (last byte)
		wrongPassword2 := []byte("MySecurePassword123?")
		if ConstantTimeCompare(wrongPassword2, stored) {
			t.Error("Incorrect password should fail")
		}

		// Incorrect password (too short)
		wrongPassword3 := []byte("MySecurePassword123")
		if ConstantTimeCompare(wrongPassword3, stored) {
			t.Error("Short password should fail")
		}
	})
}

// BenchmarkConstantTimeCompareAudit benchmarks the constant-time comparison function
// for various sizes (renamed to avoid conflict with existing benchmark)
func BenchmarkConstantTimeCompareAudit(b *testing.B) {
	sizes := []int{4, 16, 32, 64, 256, 1024}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("size_%d_equal", size), func(b *testing.B) {
			a := make([]byte, size)
			rand.Read(a)
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_ = ConstantTimeCompare(a, a)
			}
		})

		b.Run(fmt.Sprintf("size_%d_different", size), func(b *testing.B) {
			a := make([]byte, size)
			c := make([]byte, size)
			rand.Read(a)
			rand.Read(c)
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_ = ConstantTimeCompare(a, c)
			}
		})
	}
}

// BenchmarkConstantTimeCompareVsStdlib compares performance with stdlib
func BenchmarkConstantTimeCompareVsStdlib(b *testing.B) {
	a := make([]byte, 32)
	c := make([]byte, 32)
	rand.Read(a)
	copy(c, a)

	b.Run("ConstantTimeCompare", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = ConstantTimeCompare(a, c)
		}
	})

	b.Run("subtle.ConstantTimeCompare", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = subtle.ConstantTimeCompare(a, c)
		}
	})
}
