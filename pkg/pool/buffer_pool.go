// Package pool provides resource pooling for performance optimization.
// This package includes buffer pools for cell encoding/decoding and crypto operations.
package pool

import (
	"sync"
)

// BufferPool provides a pool of byte slices for reuse
type BufferPool struct {
	pool sync.Pool
	size int
}

// NewBufferPool creates a new buffer pool with the specified buffer size
func NewBufferPool(size int) *BufferPool {
	return &BufferPool{
		pool: sync.Pool{
			New: func() interface{} {
				buf := make([]byte, size)
				return &buf
			},
		},
		size: size,
	}
}

// Get retrieves a buffer from the pool
func (p *BufferPool) Get() []byte {
	// Safe type assertion with ok check (AUDIT-R-001: Fixed)
	obj := p.pool.Get()
	bufPtr, ok := obj.(*[]byte)
	if !ok {
		// This should never happen with our pool, but be defensive
		// Return a new buffer instead of panicking (AUDIT-R-001)
		// This prevents crashing the entire process on unexpected pool behavior
		buf := make([]byte, p.size)
		return buf
	}
	return (*bufPtr)[:p.size]
}

// Put returns a buffer to the pool
func (p *BufferPool) Put(buf []byte) {
	if cap(buf) < p.size {
		// Don't pool buffers that are too small
		return
	}
	buf = buf[:p.size]
	p.pool.Put(&buf)
}

// CellBufferPool is a pre-configured pool for Tor cell buffers (514 bytes)
var CellBufferPool = NewBufferPool(514)

// PayloadBufferPool is a pre-configured pool for cell payloads (509 bytes)
var PayloadBufferPool = NewBufferPool(509)

// CryptoBufferPool is a pre-configured pool for crypto operations (1KB)
var CryptoBufferPool = NewBufferPool(1024)

// LargeCryptoBufferPool is a pre-configured pool for larger crypto operations (8KB)
var LargeCryptoBufferPool = NewBufferPool(8192)
