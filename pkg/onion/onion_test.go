package onion

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base32"
	"strings"
	"testing"
	"time"

	"github.com/opd-ai/go-tor/pkg/logger"
)

// TestParseV3Address tests parsing of v3 onion addresses
func TestParseV3Address(t *testing.T) {
	tests := []struct {
		name      string
		address   string
		wantErr   bool
		errString string
	}{
		{
			name: "valid v3 address with .onion",
			// This is a properly formed v3 address (generated below)
			address: generateValidV3Address(t),
			wantErr: false,
		},
		{
			name: "valid v3 address without .onion",
			// Will be generated and stripped of .onion
			address: strings.TrimSuffix(generateValidV3Address(t), ".onion"),
			wantErr: false,
		},
		{
			name:      "invalid length - too short",
			address:   "thisiswaytooshort.onion",
			wantErr:   true,
			errString: "unsupported onion address format",
		},
		{
			name:      "invalid length - too long",
			address:   "thisistoolongforanyonionaddressformatthatweknowabout.onion",
			wantErr:   true,
			errString: "unsupported onion address format",
		},
		{
			name:      "invalid base32 encoding",
			address:   "!!!invalid!!!base32!!!encoding!!!0123456789abcdef.onion",
			wantErr:   true,
			errString: "unsupported onion address format",
		},
		{
			name:      "invalid checksum",
			address:   generateInvalidChecksumAddress(t),
			wantErr:   true,
			errString: "invalid checksum",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr, err := ParseAddress(tt.address)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseAddress() expected error, got nil")
				} else if tt.errString != "" && !strings.Contains(err.Error(), tt.errString) {
					t.Errorf("ParseAddress() error = %v, want substring %v", err, tt.errString)
				}
				return
			}
			if err != nil {
				t.Errorf("ParseAddress() unexpected error = %v", err)
				return
			}
			if addr == nil {
				t.Errorf("ParseAddress() returned nil address")
				return
			}
			if addr.Version != V3 {
				t.Errorf("ParseAddress() version = %v, want V3", addr.Version)
			}
			if len(addr.Pubkey) != V3PubkeyLen {
				t.Errorf("ParseAddress() pubkey length = %d, want %d", len(addr.Pubkey), V3PubkeyLen)
			}
		})
	}
}

// TestAddressEncode tests encoding addresses back to string format
func TestAddressEncode(t *testing.T) {
	// Generate a valid v3 address
	original := generateValidV3Address(t)

	// Parse it
	addr, err := ParseAddress(original)
	if err != nil {
		t.Fatalf("ParseAddress() failed: %v", err)
	}

	// Encode it back
	encoded := addr.Encode()

	// Should match original (case-insensitive)
	if !strings.EqualFold(encoded, original) {
		t.Errorf("Encode() = %v, want %v", encoded, original)
	}

	// Parse again to verify round-trip
	addr2, err := ParseAddress(encoded)
	if err != nil {
		t.Fatalf("ParseAddress() second time failed: %v", err)
	}

	// Pubkeys should match
	if len(addr.Pubkey) != len(addr2.Pubkey) {
		t.Errorf("Pubkey length mismatch: %d vs %d", len(addr.Pubkey), len(addr2.Pubkey))
	}
	for i := range addr.Pubkey {
		if addr.Pubkey[i] != addr2.Pubkey[i] {
			t.Errorf("Pubkey mismatch at byte %d: %02x vs %02x", i, addr.Pubkey[i], addr2.Pubkey[i])
		}
	}
}

// TestAddressString tests the String() method
func TestAddressString(t *testing.T) {
	original := generateValidV3Address(t)
	addr, err := ParseAddress(original)
	if err != nil {
		t.Fatalf("ParseAddress() failed: %v", err)
	}

	str := addr.String()
	if !strings.HasSuffix(str, ".onion") {
		t.Errorf("String() = %v, want suffix .onion", str)
	}
	if len(strings.TrimSuffix(str, ".onion")) != V3AddressLength {
		t.Errorf("String() address part length = %d, want %d",
			len(strings.TrimSuffix(str, ".onion")), V3AddressLength)
	}
}

// TestIsOnionAddress tests the IsOnionAddress helper
func TestIsOnionAddress(t *testing.T) {
	tests := []struct {
		name string
		addr string
		want bool
	}{
		{"valid .onion", "test.onion", true},
		{"valid long .onion", generateValidV3Address(t), true},
		{"no .onion suffix", "example.com", false},
		{"partial .onion", "test.onio", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsOnionAddress(tt.addr); got != tt.want {
				t.Errorf("IsOnionAddress() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestV3ChecksumComputation tests the checksum computation
func TestV3ChecksumComputation(t *testing.T) {
	// Use a known pubkey
	pubkey := make([]byte, V3PubkeyLen)
	for i := range pubkey {
		pubkey[i] = byte(i)
	}

	// Compute checksum twice - should be deterministic
	checksum1 := computeV3Checksum(pubkey, V3Version)
	checksum2 := computeV3Checksum(pubkey, V3Version)

	if len(checksum1) != V3ChecksumLen {
		t.Errorf("computeV3Checksum() returned %d bytes, want %d", len(checksum1), V3ChecksumLen)
	}

	if checksum1[0] != checksum2[0] || checksum1[1] != checksum2[1] {
		t.Errorf("computeV3Checksum() not deterministic: %v vs %v", checksum1, checksum2)
	}

	// Different pubkey should give different checksum
	pubkey[0] = 0xFF
	checksum3 := computeV3Checksum(pubkey, V3Version)
	if checksum1[0] == checksum3[0] && checksum1[1] == checksum3[1] {
		t.Errorf("computeV3Checksum() same for different pubkeys")
	}
}

// TestRealWorldV3Address tests with a real-world v3 address format
func TestRealWorldV3Address(t *testing.T) {
	// DuckDuckGo's onion address (example of a real v3 address format)
	// Note: Using a similar format, not the actual address for testing
	realAddr := generateValidV3Address(t)

	addr, err := ParseAddress(realAddr)
	if err != nil {
		t.Fatalf("ParseAddress() failed for real-world format: %v", err)
	}

	if addr.Version != V3 {
		t.Errorf("ParseAddress() version = %v, want V3", addr.Version)
	}

	// Verify it can be encoded back
	encoded := addr.Encode()
	if !strings.EqualFold(encoded, realAddr) {
		t.Errorf("Round-trip failed: encoded = %v, original = %v", encoded, realAddr)
	}
}

// Helper function to generate a valid v3 address
func generateValidV3Address(t *testing.T) string {
	// Generate a random ed25519 public key
	pubkey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate ed25519 key: %v", err)
	}

	// Construct the address: pubkey || checksum || version
	checksum := computeV3Checksum(pubkey, V3Version)
	data := make([]byte, 0, V3PubkeyLen+V3ChecksumLen+1)
	data = append(data, pubkey...)
	data = append(data, checksum...)
	data = append(data, V3Version)

	// Encode to base32
	encoder := base32.StdEncoding.WithPadding(base32.NoPadding)
	encoded := strings.ToLower(encoder.EncodeToString(data))

	return encoded + ".onion"
}

// Helper function to generate an address with invalid checksum
func generateInvalidChecksumAddress(t *testing.T) string {
	// Generate a random ed25519 public key
	pubkey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate ed25519 key: %v", err)
	}

	// Construct with WRONG checksum
	wrongChecksum := []byte{0xFF, 0xFF}
	data := make([]byte, 0, V3PubkeyLen+V3ChecksumLen+1)
	data = append(data, pubkey...)
	data = append(data, wrongChecksum...)
	data = append(data, V3Version)

	// Encode to base32
	encoder := base32.StdEncoding.WithPadding(base32.NoPadding)
	encoded := strings.ToLower(encoder.EncodeToString(data))

	return encoded + ".onion"
}

// TestNewClient tests creating a new onion service client
func TestNewClient(t *testing.T) {
	client := NewClient(nil)
	if client == nil {
		t.Errorf("NewClient() returned nil")
	}
}

// Benchmark tests
func BenchmarkParseV3Address(b *testing.B) {
	addr := generateValidV3AddressBench(b)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = ParseAddress(addr)
	}
}

func BenchmarkEncodeV3Address(b *testing.B) {
	addr := generateValidV3AddressBench(b)
	parsed, err := ParseAddress(addr)
	if err != nil {
		b.Fatalf("Failed to parse address: %v", err)
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = parsed.Encode()
	}
}

func generateValidV3AddressBench(b *testing.B) string {
	pubkey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		b.Fatalf("Failed to generate ed25519 key: %v", err)
	}

	checksum := computeV3Checksum(pubkey, V3Version)
	data := make([]byte, 0, V3PubkeyLen+V3ChecksumLen+1)
	data = append(data, pubkey...)
	data = append(data, checksum...)
	data = append(data, V3Version)

	encoder := base32.StdEncoding.WithPadding(base32.NoPadding)
	encoded := strings.ToLower(encoder.EncodeToString(data))

	return encoded + ".onion"
}

// TestDescriptorCache tests the descriptor cache functionality
func TestDescriptorCache(t *testing.T) {
	cache := NewDescriptorCache(nil)

	// Create test address and descriptor
	addr, err := ParseAddress("vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd.onion")
	if err != nil {
		t.Fatalf("Failed to parse address: %v", err)
	}

	desc := &Descriptor{
		Version:         3,
		Address:         addr,
		BlindedPubkey:   addr.Pubkey,
		DescriptorID:    computeDescriptorID(addr.Pubkey),
		RevisionCounter: 1,
		CreatedAt:       time.Now(),
		Lifetime:        1 * time.Hour,
	}

	// Test cache miss
	if _, found := cache.Get(addr); found {
		t.Error("Expected cache miss for new address")
	}

	// Test cache put and hit
	cache.Put(addr, desc)
	if cache.Size() != 1 {
		t.Errorf("Expected cache size 1, got %d", cache.Size())
	}

	retrieved, found := cache.Get(addr)
	if !found {
		t.Error("Expected cache hit after Put")
	}
	if retrieved.Version != 3 {
		t.Errorf("Expected version 3, got %d", retrieved.Version)
	}

	// Test cache remove
	cache.Remove(addr)
	if cache.Size() != 0 {
		t.Errorf("Expected cache size 0 after remove, got %d", cache.Size())
	}

	if _, found := cache.Get(addr); found {
		t.Error("Expected cache miss after Remove")
	}

	// Test cache clear
	cache.Put(addr, desc)
	cache.Clear()
	if cache.Size() != 0 {
		t.Errorf("Expected cache size 0 after clear, got %d", cache.Size())
	}
}

// TestDescriptorCacheExpiration tests descriptor expiration
func TestDescriptorCacheExpiration(t *testing.T) {
	cache := NewDescriptorCache(nil)

	addr, err := ParseAddress("vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd.onion")
	if err != nil {
		t.Fatalf("Failed to parse address: %v", err)
	}

	// Create descriptor with very short lifetime
	desc := &Descriptor{
		Version:         3,
		Address:         addr,
		BlindedPubkey:   addr.Pubkey,
		DescriptorID:    computeDescriptorID(addr.Pubkey),
		RevisionCounter: 1,
		CreatedAt:       time.Now(),
		Lifetime:        100 * time.Millisecond, // Short lifetime for testing
	}

	cache.Put(addr, desc)

	// Should be in cache immediately
	if _, found := cache.Get(addr); !found {
		t.Error("Expected descriptor in cache before expiration")
	}

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Should be expired now
	if _, found := cache.Get(addr); found {
		t.Error("Expected descriptor to be expired")
	}

	// Test CleanExpired
	cache.Put(addr, desc)
	time.Sleep(150 * time.Millisecond)
	
	cleaned := cache.CleanExpired()
	if cleaned != 1 {
		t.Errorf("Expected 1 descriptor cleaned, got %d", cleaned)
	}
	if cache.Size() != 0 {
		t.Errorf("Expected cache size 0 after cleaning, got %d", cache.Size())
	}
}

// TestOnionClient tests the onion service client
func TestOnionClient(t *testing.T) {
	client := NewClient(nil)

	addr, err := ParseAddress("vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd.onion")
	if err != nil {
		t.Fatalf("Failed to parse address: %v", err)
	}

	ctx := context.Background()

	// Test descriptor fetching (currently returns mock descriptor)
	desc, err := client.GetDescriptor(ctx, addr)
	if err != nil {
		t.Fatalf("Failed to get descriptor: %v", err)
	}

	if desc == nil {
		t.Fatal("Expected non-nil descriptor")
	}

	if desc.Version != 3 {
		t.Errorf("Expected version 3, got %d", desc.Version)
	}

	// Second call should hit cache
	desc2, err := client.GetDescriptor(ctx, addr)
	if err != nil {
		t.Fatalf("Failed to get cached descriptor: %v", err)
	}

	if desc2 != desc {
		t.Error("Expected same descriptor instance from cache")
	}
}

// TestComputeBlindedPubkey tests blinded public key computation
func TestComputeBlindedPubkey(t *testing.T) {
	// Generate test key
	pubkey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	timePeriod := uint64(12345)

	// Compute blinded key
	blinded := ComputeBlindedPubkey(pubkey, timePeriod)

	if len(blinded) != 32 {
		t.Errorf("Expected blinded key length 32, got %d", len(blinded))
	}

	// Same inputs should produce same output
	blinded2 := ComputeBlindedPubkey(pubkey, timePeriod)
	if !bytes.Equal(blinded, blinded2) {
		t.Error("Expected deterministic blinded key computation")
	}

	// Different time period should produce different output
	blinded3 := ComputeBlindedPubkey(pubkey, timePeriod+1)
	if bytes.Equal(blinded, blinded3) {
		t.Error("Expected different blinded key for different time period")
	}
}

// TestGetTimePeriod tests time period calculation
func TestGetTimePeriod(t *testing.T) {
	// Test with known time
	// Unix timestamp: 1609459200 = 2021-01-01 00:00:00 UTC
	testTime := time.Unix(1609459200, 0)
	
	period := GetTimePeriod(testTime)
	
	// Verify period is non-zero
	if period == 0 {
		t.Error("Expected non-zero time period")
	}

	// Same time should give same period
	period2 := GetTimePeriod(testTime)
	if period != period2 {
		t.Error("Expected deterministic time period calculation")
	}

	// Time 24 hours later should give different period
	testTime2 := testTime.Add(24 * time.Hour)
	period3 := GetTimePeriod(testTime2)
	if period == period3 {
		t.Error("Expected different period after 24 hours")
	}
}

// TestParseDescriptor tests descriptor parsing
func TestParseDescriptor(t *testing.T) {
	rawDesc := []byte(`hs-descriptor 3
descriptor-lifetime 180
revision-counter 42
`)

	desc, err := ParseDescriptor(rawDesc)
	if err != nil {
		t.Fatalf("Failed to parse descriptor: %v", err)
	}

	if desc.Version != 3 {
		t.Errorf("Expected version 3, got %d", desc.Version)
	}

	if len(desc.RawDescriptor) == 0 {
		t.Error("Expected raw descriptor to be stored")
	}
}

// TestEncodeDescriptor tests descriptor encoding
func TestEncodeDescriptor(t *testing.T) {
	desc := &Descriptor{
		Version:         3,
		RevisionCounter: 123,
		Lifetime:        3 * time.Hour,
		DescriptorID:    make([]byte, 32),
	}

	encoded, err := EncodeDescriptor(desc)
	if err != nil {
		t.Fatalf("Failed to encode descriptor: %v", err)
	}

	if len(encoded) == 0 {
		t.Error("Expected non-empty encoded descriptor")
	}

	// Should contain version line
	if !bytes.Contains(encoded, []byte("hs-descriptor 3")) {
		t.Error("Expected encoded descriptor to contain version line")
	}
}

// TestComputeDescriptorID tests descriptor ID computation
func TestComputeDescriptorID(t *testing.T) {
	// Generate test key
	pubkey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	descID := computeDescriptorID(pubkey)

	if len(descID) != 32 {
		t.Errorf("Expected descriptor ID length 32, got %d", len(descID))
	}

	// Same input should produce same output
	descID2 := computeDescriptorID(pubkey)
	if !bytes.Equal(descID, descID2) {
		t.Error("Expected deterministic descriptor ID computation")
	}
}

// BenchmarkDescriptorCache benchmarks descriptor cache operations
func BenchmarkDescriptorCache(b *testing.B) {
	cache := NewDescriptorCache(nil)
	
	addr, _ := ParseAddress("vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd.onion")
	desc := &Descriptor{
		Version:         3,
		Address:         addr,
		BlindedPubkey:   addr.Pubkey,
		DescriptorID:    computeDescriptorID(addr.Pubkey),
		RevisionCounter: 1,
		CreatedAt:       time.Now(),
		Lifetime:        1 * time.Hour,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Put(addr, desc)
		cache.Get(addr)
	}
}

// BenchmarkComputeBlindedPubkey benchmarks blinded key computation
func BenchmarkComputeBlindedPubkey(b *testing.B) {
	pubkey, _, _ := ed25519.GenerateKey(rand.Reader)
	timePeriod := uint64(12345)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ComputeBlindedPubkey(pubkey, timePeriod)
	}
}

// TestHSDirSelection tests HSDir selection algorithm
func TestHSDirSelection(t *testing.T) {
	log := logger.NewDefault()
	hsdir := NewHSDir(log)

	// Create mock HSDirs
	hsdirs := []*HSDirectory{
		{Fingerprint: "0000000000000000000000000000000000000001", Address: "10.0.0.1", ORPort: 9001, HSDir: true},
		{Fingerprint: "0000000000000000000000000000000000000002", Address: "10.0.0.2", ORPort: 9001, HSDir: true},
		{Fingerprint: "0000000000000000000000000000000000000003", Address: "10.0.0.3", ORPort: 9001, HSDir: true},
		{Fingerprint: "0000000000000000000000000000000000000004", Address: "10.0.0.4", ORPort: 9001, HSDir: true},
		{Fingerprint: "0000000000000000000000000000000000000005", Address: "10.0.0.5", ORPort: 9001, HSDir: true},
	}

	// Generate a random descriptor ID
	descriptorID := make([]byte, 32)
	rand.Read(descriptorID)

	// Test replica 0
	selected := hsdir.SelectHSDirs(descriptorID, hsdirs, 0)
	if len(selected) != 3 {
		t.Errorf("Expected 3 selected HSDirs, got %d", len(selected))
	}

	// Test replica 1 - should select different HSDirs
	selected1 := hsdir.SelectHSDirs(descriptorID, hsdirs, 1)
	if len(selected1) != 3 {
		t.Errorf("Expected 3 selected HSDirs for replica 1, got %d", len(selected1))
	}

	// Test with fewer HSDirs than needed
	smallHSDirs := hsdirs[:2]
	selected2 := hsdir.SelectHSDirs(descriptorID, smallHSDirs, 0)
	if len(selected2) != 2 {
		t.Errorf("Expected 2 selected HSDirs (all available), got %d", len(selected2))
	}

	// Test with empty HSDir list
	selected3 := hsdir.SelectHSDirs(descriptorID, []*HSDirectory{}, 0)
	if selected3 != nil {
		t.Error("Expected nil for empty HSDir list")
	}
}

// TestComputeReplicaDescriptorID tests replica descriptor ID computation
func TestComputeReplicaDescriptorID(t *testing.T) {
	baseID := make([]byte, 32)
	rand.Read(baseID)

	// Replica 0 and replica 1 should produce different IDs
	replica0 := ComputeReplicaDescriptorID(baseID, 0)
	replica1 := ComputeReplicaDescriptorID(baseID, 1)

	if bytes.Equal(replica0, replica1) {
		t.Error("Expected different descriptor IDs for different replicas")
	}

	// Same replica should produce same ID
	replica0_again := ComputeReplicaDescriptorID(baseID, 0)
	if !bytes.Equal(replica0, replica0_again) {
		t.Error("Expected deterministic descriptor ID computation")
	}

	// Check length
	if len(replica0) != 32 {
		t.Errorf("Expected descriptor ID length 32, got %d", len(replica0))
	}
}

// TestComputeXORDistance tests XOR distance computation
func TestComputeXORDistance(t *testing.T) {
	tests := []struct {
		name     string
		a        []byte
		b        []byte
		expected []byte
	}{
		{
			name:     "same values",
			a:        []byte{0xFF, 0x00, 0xAA},
			b:        []byte{0xFF, 0x00, 0xAA},
			expected: []byte{0x00, 0x00, 0x00},
		},
		{
			name:     "different values",
			a:        []byte{0xFF, 0x00},
			b:        []byte{0x00, 0xFF},
			expected: []byte{0xFF, 0xFF},
		},
		{
			name:     "partial match",
			a:        []byte{0xF0, 0x0F},
			b:        []byte{0x0F, 0xF0},
			expected: []byte{0xFF, 0xFF},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := computeXORDistance(tt.a, tt.b)
			if !bytes.Equal(result, tt.expected) {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestCompareBytes tests byte comparison
func TestCompareBytes(t *testing.T) {
	tests := []struct {
		name     string
		a        []byte
		b        []byte
		expected int
	}{
		{
			name:     "equal",
			a:        []byte{0x01, 0x02, 0x03},
			b:        []byte{0x01, 0x02, 0x03},
			expected: 0,
		},
		{
			name:     "a less than b",
			a:        []byte{0x01, 0x02},
			b:        []byte{0x01, 0x03},
			expected: -1,
		},
		{
			name:     "a greater than b",
			a:        []byte{0x02, 0x01},
			b:        []byte{0x01, 0x01},
			expected: 1,
		},
		{
			name:     "a shorter than b",
			a:        []byte{0x01},
			b:        []byte{0x01, 0x02},
			expected: -1,
		},
		{
			name:     "a longer than b",
			a:        []byte{0x01, 0x02},
			b:        []byte{0x01},
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compareBytes(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, result)
			}
		})
	}
}

// TestHSDirFetchDescriptor tests descriptor fetching from HSDirs
func TestHSDirFetchDescriptor(t *testing.T) {
	log := logger.NewDefault()
	hsdir := NewHSDir(log)

	// Parse a valid onion address
	addr, err := ParseAddress("vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd.onion")
	if err != nil {
		t.Fatalf("Failed to parse address: %v", err)
	}

	// Create mock HSDirs
	hsdirs := []*HSDirectory{
		{Fingerprint: "0000000000000000000000000000000000000001", Address: "10.0.0.1", ORPort: 9001, HSDir: true},
		{Fingerprint: "0000000000000000000000000000000000000002", Address: "10.0.0.2", ORPort: 9001, HSDir: true},
		{Fingerprint: "0000000000000000000000000000000000000003", Address: "10.0.0.3", ORPort: 9001, HSDir: true},
	}

	ctx := context.Background()

	// Fetch descriptor
	desc, err := hsdir.FetchDescriptor(ctx, addr, hsdirs)
	if err != nil {
		t.Fatalf("Failed to fetch descriptor: %v", err)
	}

	// Verify descriptor properties
	if desc.Version != 3 {
		t.Errorf("Expected version 3, got %d", desc.Version)
	}

	if desc.Address == nil {
		t.Error("Expected address to be set")
	}

	if len(desc.BlindedPubkey) == 0 {
		t.Error("Expected blinded pubkey to be set")
	}

	if len(desc.DescriptorID) != 32 {
		t.Errorf("Expected descriptor ID length 32, got %d", len(desc.DescriptorID))
	}

	if desc.Lifetime != 3*time.Hour {
		t.Errorf("Expected lifetime 3h, got %v", desc.Lifetime)
	}
}

// TestClientUpdateHSDirs tests updating HSDir list in client
func TestClientUpdateHSDirs(t *testing.T) {
	log := logger.NewDefault()
	client := NewClient(log)

	// Initially should have no HSDirs
	if len(client.consensus) != 0 {
		t.Error("Expected empty consensus initially")
	}

	// Update with HSDirs
	hsdirs := []*HSDirectory{
		{Fingerprint: "0000000000000000000000000000000000000001", Address: "10.0.0.1", ORPort: 9001, HSDir: true},
		{Fingerprint: "0000000000000000000000000000000000000002", Address: "10.0.0.2", ORPort: 9001, HSDir: true},
	}

	client.UpdateHSDirs(hsdirs)

	if len(client.consensus) != 2 {
		t.Errorf("Expected 2 HSDirs in consensus, got %d", len(client.consensus))
	}
}

// TestClientGetDescriptorWithHSDirs tests descriptor fetching with HSDirs available
func TestClientGetDescriptorWithHSDirs(t *testing.T) {
	log := logger.NewDefault()
	client := NewClient(log)

	// Parse a valid onion address
	addr, err := ParseAddress("vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd.onion")
	if err != nil {
		t.Fatalf("Failed to parse address: %v", err)
	}

	// Update with HSDirs
	hsdirs := []*HSDirectory{
		{Fingerprint: "0000000000000000000000000000000000000001", Address: "10.0.0.1", ORPort: 9001, HSDir: true},
		{Fingerprint: "0000000000000000000000000000000000000002", Address: "10.0.0.2", ORPort: 9001, HSDir: true},
		{Fingerprint: "0000000000000000000000000000000000000003", Address: "10.0.0.3", ORPort: 9001, HSDir: true},
	}
	client.UpdateHSDirs(hsdirs)

	ctx := context.Background()

	// First fetch - should use HSDir protocol and cache result
	desc1, err := client.GetDescriptor(ctx, addr)
	if err != nil {
		t.Fatalf("Failed to get descriptor: %v", err)
	}

	if desc1 == nil {
		t.Fatal("Expected descriptor to be returned")
	}

	// Second fetch - should hit cache
	desc2, err := client.GetDescriptor(ctx, addr)
	if err != nil {
		t.Fatalf("Failed to get descriptor from cache: %v", err)
	}

	// Should be the same descriptor from cache
	if desc1 != desc2 {
		t.Error("Expected same descriptor instance from cache")
	}
}

// BenchmarkHSDirSelection benchmarks HSDir selection
func BenchmarkHSDirSelection(b *testing.B) {
	log := logger.NewDefault()
	hsdir := NewHSDir(log)

	// Create many mock HSDirs
	hsdirs := make([]*HSDirectory, 100)
	for i := 0; i < 100; i++ {
		hsdirs[i] = &HSDirectory{
			Fingerprint: string(make([]byte, 40)),
			Address:     "10.0.0.1",
			ORPort:      9001,
			HSDir:       true,
		}
	}

	descriptorID := make([]byte, 32)
	rand.Read(descriptorID)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hsdir.SelectHSDirs(descriptorID, hsdirs, 0)
	}
}

// BenchmarkFetchDescriptor benchmarks descriptor fetching
func BenchmarkFetchDescriptor(b *testing.B) {
	log := logger.NewDefault()
	hsdir := NewHSDir(log)

	addr, _ := ParseAddress("vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd.onion")

	hsdirs := []*HSDirectory{
		{Fingerprint: "0000000000000000000000000000000000000001", Address: "10.0.0.1", ORPort: 9001, HSDir: true},
		{Fingerprint: "0000000000000000000000000000000000000002", Address: "10.0.0.2", ORPort: 9001, HSDir: true},
		{Fingerprint: "0000000000000000000000000000000000000003", Address: "10.0.0.3", ORPort: 9001, HSDir: true},
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hsdir.FetchDescriptor(ctx, addr, hsdirs)
	}
}
