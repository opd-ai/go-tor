package crypto

import (
	"crypto/rand"
	"testing"
)

// BenchmarkAESCTREncrypt benchmarks AES-CTR encryption
func BenchmarkAESCTREncrypt(b *testing.B) {
	key := make([]byte, 16) // AES-128
	iv := make([]byte, 16)
	plaintext := make([]byte, 1024) // 1KB

	rand.Read(key)
	rand.Read(iv)
	rand.Read(plaintext)

	cipher, err := NewAESCTRCipher(key, iv)
	if err != nil {
		b.Fatal(err)
	}

	b.SetBytes(int64(len(plaintext)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		data := make([]byte, len(plaintext))
		copy(data, plaintext)
		cipher.Encrypt(data)
	}
}

// BenchmarkAESCTRDecrypt benchmarks AES-CTR decryption
func BenchmarkAESCTRDecrypt(b *testing.B) {
	key := make([]byte, 16)
	iv := make([]byte, 16)
	ciphertext := make([]byte, 1024)

	rand.Read(key)
	rand.Read(iv)
	rand.Read(ciphertext)

	cipher, err := NewAESCTRCipher(key, iv)
	if err != nil {
		b.Fatal(err)
	}

	b.SetBytes(int64(len(ciphertext)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		data := make([]byte, len(ciphertext))
		copy(data, ciphertext)
		cipher.Decrypt(data)
	}
}

// BenchmarkAESCTREncrypt8KB benchmarks AES-CTR with 8KB blocks
func BenchmarkAESCTREncrypt8KB(b *testing.B) {
	key := make([]byte, 16)
	iv := make([]byte, 16)
	plaintext := make([]byte, 8192) // 8KB

	rand.Read(key)
	rand.Read(iv)
	rand.Read(plaintext)

	cipher, err := NewAESCTRCipher(key, iv)
	if err != nil {
		b.Fatal(err)
	}

	b.SetBytes(int64(len(plaintext)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		data := make([]byte, len(plaintext))
		copy(data, plaintext)
		cipher.Encrypt(data)
	}
}

// BenchmarkSHA1 benchmarks SHA-1 hashing
func BenchmarkSHA1(b *testing.B) {
	data := make([]byte, 1024)
	rand.Read(data)

	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = SHA1Hash(data)
	}
}

// BenchmarkSHA256 benchmarks SHA-256 hashing
func BenchmarkSHA256(b *testing.B) {
	data := make([]byte, 1024)
	rand.Read(data)

	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = SHA256Hash(data)
	}
}

// BenchmarkKDFTOR benchmarks KDF-TOR key derivation
func BenchmarkKDFTOR(b *testing.B) {
	key := make([]byte, 32)
	rand.Read(key)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := DeriveKey(key, 100)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkAESCTREncryptParallel benchmarks parallel AES-CTR encryption
func BenchmarkAESCTREncryptParallel(b *testing.B) {
	key := make([]byte, 16)
	iv := make([]byte, 16)
	plaintext := make([]byte, 1024)

	rand.Read(key)
	rand.Read(iv)
	rand.Read(plaintext)

	cipher, err := NewAESCTRCipher(key, iv)
	if err != nil {
		b.Fatal(err)
	}

	b.SetBytes(int64(len(plaintext)))
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			data := make([]byte, len(plaintext))
			copy(data, plaintext)
			cipher.Encrypt(data)
		}
	})
}

// BenchmarkSHA256Parallel benchmarks parallel SHA-256 hashing
func BenchmarkSHA256Parallel(b *testing.B) {
	data := make([]byte, 1024)
	rand.Read(data)

	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = SHA256Hash(data)
		}
	})
}
