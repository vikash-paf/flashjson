// Package arena provides a high-performance bump allocator for temporary allocations.
// It is designed for near-zero GC pressure during JSON parsing operations.
//
// The arena allocates memory sequentially from a pre-allocated block, making allocation
// extremely fast (just a pointer increment). When parsing is complete, the entire arena
// is reset with a single operation, avoiding individual deallocations.
//
// Usage:
//
//	arena := arena.Get()
//	defer arena.Put()
//
//	// Allocate temporary buffers
//	buf := arena.Alloc(1024)
//
//	// ... use buf for parsing ...
//
//	// Reset happens automatically when Put() is called
package arena

import (
	"sync"
	"unsafe"
)

const (
	// DefaultSize is the default arena size (64KB).
	// This is chosen to fit in L2 cache while providing enough space for most JSON documents.
	DefaultSize = 64 * 1024

	// MaxSize is the maximum arena size (1MB).
	// Larger allocations should use standard Go allocation.
	MaxSize = 1024 * 1024

	// alignment ensures all allocations are properly aligned for any Go type.
	alignment = 8
)

// Arena is a bump allocator that allocates memory sequentially from a fixed-size block.
// It is not safe for concurrent use by multiple goroutines.
type Arena struct {
	buf    []byte // The underlying memory block
	offset int    // Current allocation offset
}

// New creates a new Arena with the specified size.
// The size should be a power of 2 for optimal memory alignment.
func New(size int) *Arena {
	if size <= 0 {
		size = DefaultSize
	}
	if size > MaxSize {
		size = MaxSize
	}
	return &Arena{
		buf:    make([]byte, size),
		offset: 0,
	}
}

// Alloc allocates n bytes from the arena and returns a slice pointing to that memory.
// The returned slice is zeroed.
// Panics if there is not enough space (use CanAlloc to check first if needed).
func (a *Arena) Alloc(n int) []byte {
	// Align the current offset
	aligned := (a.offset + alignment - 1) &^ (alignment - 1)

	if aligned+n > len(a.buf) {
		panic("arena: allocation exceeds capacity")
	}

	result := a.buf[aligned : aligned+n : aligned+n]
	a.offset = aligned + n

	// Zero the memory (important for correctness)
	for i := range result {
		result[i] = 0
	}

	return result
}

// AllocNoZero is like Alloc but does not zero the memory.
// Use only when you will immediately overwrite all bytes.
func (a *Arena) AllocNoZero(n int) []byte {
	aligned := (a.offset + alignment - 1) &^ (alignment - 1)

	if aligned+n > len(a.buf) {
		panic("arena: allocation exceeds capacity")
	}

	result := a.buf[aligned : aligned+n : aligned+n]
	a.offset = aligned + n

	return result
}

// AllocString allocates space for and copies a string into the arena.
// Returns a new string that points to arena memory.
// This is useful when you need strings to outlive the input buffer.
func (a *Arena) AllocString(s string) string {
	if len(s) == 0 {
		return ""
	}

	buf := a.AllocNoZero(len(s))
	copy(buf, s)

	// Convert []byte to string without allocation using unsafe
	return *(*string)(unsafe.Pointer(&buf))
}

// AllocBytes allocates space for and copies bytes into the arena.
func (a *Arena) AllocBytes(b []byte) []byte {
	if len(b) == 0 {
		return nil
	}

	buf := a.AllocNoZero(len(b))
	copy(buf, b)
	return buf
}

// Reset resets the arena, allowing all memory to be reused.
// This is an O(1) operation - just resetting the offset pointer.
// After Reset, any previously allocated slices point to memory that may be overwritten.
func (a *Arena) Reset() {
	a.offset = 0
}

// Used returns the number of bytes currently allocated from the arena.
func (a *Arena) Used() int {
	return a.offset
}

// Cap returns the total capacity of the arena in bytes.
func (a *Arena) Cap() int {
	return len(a.buf)
}

// Available returns the number of bytes available for allocation.
func (a *Arena) Available() int {
	return len(a.buf) - a.offset
}

// CanAlloc returns true if n bytes can be allocated from the arena.
func (a *Arena) CanAlloc(n int) bool {
	aligned := (a.offset + alignment - 1) &^ (alignment - 1)
	return aligned+n <= len(a.buf)
}

// --- Pool for arena reuse ---

var pool = sync.Pool{
	New: func() interface{} {
		return New(DefaultSize)
	},
}

// Get retrieves an Arena from the pool.
// The arena is ready to use and has been reset.
// Call Put() when done to return it to the pool.
func Get() *Arena {
	return pool.Get().(*Arena)
}

// Put returns an Arena to the pool for reuse.
// The arena is automatically reset.
// After calling Put, the arena should not be used.
func Put(a *Arena) {
	a.Reset()
	pool.Put(a)
}

// GetSized retrieves an Arena with at least the specified capacity.
// If the pooled arena is too small, a new one is created.
func GetSized(size int) *Arena {
	a := pool.Get().(*Arena)
	if a.Cap() < size {
		// Return the small one and create a larger one
		pool.Put(a)
		return New(size)
	}
	return a
}
