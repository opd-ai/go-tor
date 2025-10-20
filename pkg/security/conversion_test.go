package security

import (
	"math"
	"testing"
	"time"
)

func TestSafeUnixToUint64(t *testing.T) {
	tests := []struct {
		name    string
		time    time.Time
		want    uint64
		wantErr bool
	}{
		{
			name:    "current time",
			time:    time.Unix(1700000000, 0), // Nov 2023
			want:    1700000000,
			wantErr: false,
		},
		{
			name:    "epoch",
			time:    time.Unix(0, 0),
			want:    0,
			wantErr: false,
		},
		{
			name:    "far future",
			time:    time.Unix(4102444800, 0), // Jan 2100
			want:    4102444800,
			wantErr: false,
		},
		{
			name:    "negative timestamp",
			time:    time.Unix(-1, 0),
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SafeUnixToUint64(tt.time)
			if (err != nil) != tt.wantErr {
				t.Errorf("SafeUnixToUint64() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("SafeUnixToUint64() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSafeUnixToUint32(t *testing.T) {
	tests := []struct {
		name    string
		time    time.Time
		want    uint32
		wantErr bool
	}{
		{
			name:    "current time",
			time:    time.Unix(1700000000, 0),
			want:    1700000000,
			wantErr: false,
		},
		{
			name:    "epoch",
			time:    time.Unix(0, 0),
			want:    0,
			wantErr: false,
		},
		{
			name:    "max uint32",
			time:    time.Unix(math.MaxUint32, 0),
			want:    math.MaxUint32,
			wantErr: false,
		},
		{
			name:    "exceeds uint32 (year 2106+)",
			time:    time.Unix(math.MaxUint32+1, 0),
			want:    0,
			wantErr: true,
		},
		{
			name:    "negative timestamp",
			time:    time.Unix(-1, 0),
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SafeUnixToUint32(tt.time)
			if (err != nil) != tt.wantErr {
				t.Errorf("SafeUnixToUint32() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("SafeUnixToUint32() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSafeIntToUint64(t *testing.T) {
	tests := []struct {
		name    string
		val     int
		want    uint64
		wantErr bool
	}{
		{
			name:    "positive value",
			val:     12345,
			want:    12345,
			wantErr: false,
		},
		{
			name:    "zero",
			val:     0,
			want:    0,
			wantErr: false,
		},
		{
			name:    "negative value",
			val:     -1,
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SafeIntToUint64(tt.val)
			if (err != nil) != tt.wantErr {
				t.Errorf("SafeIntToUint64() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("SafeIntToUint64() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSafeIntToUint16(t *testing.T) {
	tests := []struct {
		name    string
		val     int
		want    uint16
		wantErr bool
	}{
		{
			name:    "small positive value",
			val:     1234,
			want:    1234,
			wantErr: false,
		},
		{
			name:    "zero",
			val:     0,
			want:    0,
			wantErr: false,
		},
		{
			name:    "max uint16",
			val:     math.MaxUint16,
			want:    math.MaxUint16,
			wantErr: false,
		},
		{
			name:    "exceeds uint16",
			val:     math.MaxUint16 + 1,
			want:    0,
			wantErr: true,
		},
		{
			name:    "negative value",
			val:     -1,
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SafeIntToUint16(tt.val)
			if (err != nil) != tt.wantErr {
				t.Errorf("SafeIntToUint16() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("SafeIntToUint16() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSafeInt64ToUint64(t *testing.T) {
	tests := []struct {
		name    string
		val     int64
		want    uint64
		wantErr bool
	}{
		{
			name:    "positive value",
			val:     int64(1234567890),
			want:    uint64(1234567890),
			wantErr: false,
		},
		{
			name:    "zero",
			val:     0,
			want:    0,
			wantErr: false,
		},
		{
			name:    "max int64",
			val:     math.MaxInt64,
			want:    uint64(math.MaxInt64),
			wantErr: false,
		},
		{
			name:    "negative value",
			val:     -1,
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SafeInt64ToUint64(tt.val)
			if (err != nil) != tt.wantErr {
				t.Errorf("SafeInt64ToUint64() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("SafeInt64ToUint64() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSafeUint64ToInt64(t *testing.T) {
	tests := []struct {
		name    string
		val     uint64
		want    int64
		wantErr bool
	}{
		{
			name:    "positive value",
			val:     uint64(1234567890),
			want:    int64(1234567890),
			wantErr: false,
		},
		{
			name:    "zero",
			val:     0,
			want:    0,
			wantErr: false,
		},
		{
			name:    "max int64",
			val:     uint64(math.MaxInt64),
			want:    math.MaxInt64,
			wantErr: false,
		},
		{
			name:    "exceeds int64",
			val:     uint64(math.MaxInt64) + 1,
			want:    0,
			wantErr: true,
		},
		{
			name:    "max uint64",
			val:     math.MaxUint64,
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SafeUint64ToInt64(tt.val)
			if (err != nil) != tt.wantErr {
				t.Errorf("SafeUint64ToInt64() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("SafeUint64ToInt64() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSafeIntToUint32(t *testing.T) {
	tests := []struct {
		name    string
		val     int
		want    uint32
		wantErr bool
	}{
		{
			name:    "small positive value",
			val:     12345,
			want:    12345,
			wantErr: false,
		},
		{
			name:    "zero",
			val:     0,
			want:    0,
			wantErr: false,
		},
		{
			name:    "max uint32",
			val:     int(math.MaxUint32),
			want:    math.MaxUint32,
			wantErr: false,
		},
		{
			name:    "negative value",
			val:     -1,
			want:    0,
			wantErr: true,
		},
	}

	// Only test overflow on 64-bit systems where int can exceed uint32
	if ^uint(0) > math.MaxUint32 {
		tests = append(tests, struct {
			name    string
			val     int
			want    uint32
			wantErr bool
		}{
			name:    "exceeds uint32",
			val:     int(math.MaxUint32) + 1,
			want:    0,
			wantErr: true,
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SafeIntToUint32(tt.val)
			if (err != nil) != tt.wantErr {
				t.Errorf("SafeIntToUint32() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("SafeIntToUint32() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSafeLenToUint16(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		want    uint16
		wantErr bool
	}{
		{
			name:    "empty slice",
			data:    []byte{},
			want:    0,
			wantErr: false,
		},
		{
			name:    "small slice",
			data:    make([]byte, 100),
			want:    100,
			wantErr: false,
		},
		{
			name:    "max uint16 size",
			data:    make([]byte, math.MaxUint16),
			want:    math.MaxUint16,
			wantErr: false,
		},
		{
			name:    "exceeds uint16",
			data:    make([]byte, math.MaxUint16+1),
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SafeLenToUint16(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("SafeLenToUint16() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("SafeLenToUint16() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConstantTimeCompare(t *testing.T) {
	tests := []struct {
		name string
		a    []byte
		b    []byte
		want bool
	}{
		{
			name: "equal slices",
			a:    []byte{1, 2, 3, 4, 5},
			b:    []byte{1, 2, 3, 4, 5},
			want: true,
		},
		{
			name: "different slices",
			a:    []byte{1, 2, 3, 4, 5},
			b:    []byte{1, 2, 3, 4, 6},
			want: false,
		},
		{
			name: "different lengths",
			a:    []byte{1, 2, 3},
			b:    []byte{1, 2, 3, 4},
			want: false,
		},
		{
			name: "empty slices",
			a:    []byte{},
			b:    []byte{},
			want: true,
		},
		{
			name: "nil slices",
			a:    nil,
			b:    nil,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConstantTimeCompare(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("ConstantTimeCompare() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSecureZeroMemory(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "normal slice",
			data: []byte{1, 2, 3, 4, 5, 6, 7, 8},
		},
		{
			name: "single byte",
			data: []byte{42},
		},
		{
			name: "empty slice",
			data: []byte{},
		},
		{
			name: "nil slice",
			data: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SecureZeroMemory(tt.data)
			// Verify all bytes are zeroed
			for i, b := range tt.data {
				if b != 0 {
					t.Errorf("Byte at position %d not zeroed: got %d, want 0", i, b)
				}
			}
		})
	}
}
