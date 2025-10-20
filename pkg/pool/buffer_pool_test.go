package pool

import (
	"testing"
)

func TestBufferPool(t *testing.T) {
	pool := NewBufferPool(1024)

	// Get a buffer
	buf := pool.Get()
	if len(buf) != 1024 {
		t.Errorf("Expected buffer length 1024, got %d", len(buf))
	}
	if cap(buf) < 1024 {
		t.Errorf("Expected buffer capacity >= 1024, got %d", cap(buf))
	}

	// Put it back
	pool.Put(buf)

	// Get it again (should be reused)
	buf2 := pool.Get()
	if len(buf2) != 1024 {
		t.Errorf("Expected buffer length 1024, got %d", len(buf2))
	}
}

func TestBufferPoolConcurrent(t *testing.T) {
	pool := NewBufferPool(512)
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				buf := pool.Get()
				// Use the buffer
				buf[0] = byte(j)
				pool.Put(buf)
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestCellBufferPool(t *testing.T) {
	buf := CellBufferPool.Get()
	if len(buf) != 514 {
		t.Errorf("Expected cell buffer length 514, got %d", len(buf))
	}
	CellBufferPool.Put(buf)
}

func TestPayloadBufferPool(t *testing.T) {
	buf := PayloadBufferPool.Get()
	if len(buf) != 509 {
		t.Errorf("Expected payload buffer length 509, got %d", len(buf))
	}
	PayloadBufferPool.Put(buf)
}

func TestCryptoBufferPool(t *testing.T) {
	buf := CryptoBufferPool.Get()
	if len(buf) != 1024 {
		t.Errorf("Expected crypto buffer length 1024, got %d", len(buf))
	}
	CryptoBufferPool.Put(buf)
}

func BenchmarkBufferPoolGetPut(b *testing.B) {
	pool := NewBufferPool(1024)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := pool.Get()
		pool.Put(buf)
	}
}

func BenchmarkBufferPoolGetPutParallel(b *testing.B) {
	pool := NewBufferPool(1024)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := pool.Get()
			pool.Put(buf)
		}
	})
}

func BenchmarkNoPooling(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := make([]byte, 1024)
		_ = buf
	}
}

func TestBufferPoolSmallBuffer(t *testing.T) {
	pool := NewBufferPool(1024)

	// Get a buffer from the pool
	buf := pool.Get()

	// Try to put back a buffer that's too small
	smallBuf := make([]byte, 512)
	pool.Put(smallBuf)

	// Get another buffer - should be from original pool, not the small one
	buf2 := pool.Get()
	if len(buf2) != 1024 {
		t.Errorf("Expected buffer length 1024 after putting small buffer, got %d", len(buf2))
	}

	pool.Put(buf)
	pool.Put(buf2)
}

func TestBufferPoolLargeBuffer(t *testing.T) {
	pool := NewBufferPool(1024)

	// Create a larger buffer
	largeBuf := make([]byte, 2048)

	// Should accept large buffers (will be sliced down)
	pool.Put(largeBuf)

	// Get buffer - should work fine
	buf := pool.Get()
	if len(buf) != 1024 {
		t.Errorf("Expected buffer length 1024, got %d", len(buf))
	}

	pool.Put(buf)
}

func TestBufferPoolZeroSize(t *testing.T) {
	// Edge case: zero size pool
	pool := NewBufferPool(0)

	buf := pool.Get()
	if len(buf) != 0 {
		t.Errorf("Expected buffer length 0, got %d", len(buf))
	}

	pool.Put(buf)
}

func TestBufferPoolMultipleGetPut(t *testing.T) {
	pool := NewBufferPool(1024)

	// Get multiple buffers
	bufs := make([][]byte, 5)
	for i := 0; i < 5; i++ {
		bufs[i] = pool.Get()
		if len(bufs[i]) != 1024 {
			t.Errorf("Buffer %d: expected length 1024, got %d", i, len(bufs[i]))
		}
	}

	// Put them all back
	for i := 0; i < 5; i++ {
		pool.Put(bufs[i])
	}

	// Get them again
	for i := 0; i < 5; i++ {
		buf := pool.Get()
		if len(buf) != 1024 {
			t.Errorf("Reused buffer %d: expected length 1024, got %d", i, len(buf))
		}
		pool.Put(buf)
	}
}

func TestLargeCryptoBufferPool(t *testing.T) {
	buf := LargeCryptoBufferPool.Get()
	if len(buf) != 8192 {
		t.Errorf("Expected large crypto buffer length 8192, got %d", len(buf))
	}

	// Verify we can write to the buffer
	for i := 0; i < len(buf); i++ {
		buf[i] = byte(i % 256)
	}

	LargeCryptoBufferPool.Put(buf)
}
