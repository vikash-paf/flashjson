// Package hash provides fast string hashing for O(1) field lookup.
// Uses FNV-1a which is fast and has good distribution.
package hash

// Hash64 computes FNV-1a hash of a string.
// FNV-1a is chosen for:
// 1. Speed: Just XOR and multiply per byte
// 2. Good distribution: Low collision rate
// 3. Simplicity: Easy to inline
func Hash64(s string) uint64 {
	const (
		offset64 = 14695981039346656037
		prime64  = 1099511628211
	)

	h := uint64(offset64)
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= prime64
	}
	return h
}

// HashBytes computes FNV-1a hash of bytes.
func HashBytes(b []byte) uint64 {
	const (
		offset64 = 14695981039346656037
		prime64  = 1099511628211
	)

	h := uint64(offset64)
	for _, c := range b {
		h ^= uint64(c)
		h *= prime64
	}
	return h
}

// FieldMap is a hash table for O(1) struct field lookup.
// Open addressing with linear probing for cache efficiency.
type FieldMap struct {
	entries []fieldEntry
	mask    uint64
}

type fieldEntry struct {
	Hash   uint64
	Name   string
	Offset uintptr
	Index  int
}

// NewFieldMap creates a field map with the given capacity.
// Capacity is rounded up to next power of 2.
func NewFieldMap(cap int) *FieldMap {
	// Round up to power of 2
	size := 1
	for size < cap*2 { // 50% load factor for good perf
		size *= 2
	}

	return &FieldMap{
		entries: make([]fieldEntry, size),
		mask:    uint64(size - 1),
	}
}

// Put adds a field to the map.
func (m *FieldMap) Put(name string, offset uintptr, index int) {
	h := Hash64(name)
	pos := h & m.mask

	for {
		if m.entries[pos].Name == "" {
			m.entries[pos] = fieldEntry{
				Hash:   h,
				Name:   name,
				Offset: offset,
				Index:  index,
			}
			return
		}
		pos = (pos + 1) & m.mask
	}
}

// Get looks up a field by name (string version).
func (m *FieldMap) Get(name string) (offset uintptr, index int, found bool) {
	h := Hash64(name)
	pos := h & m.mask

	for {
		e := &m.entries[pos]
		if e.Name == "" {
			return 0, 0, false
		}
		if e.Hash == h && e.Name == name {
			return e.Offset, e.Index, true
		}
		pos = (pos + 1) & m.mask
	}
}

// GetBytes looks up a field by name (bytes version, avoids allocation).
func (m *FieldMap) GetBytes(name []byte) (offset uintptr, index int, found bool) {
	h := HashBytes(name)
	pos := h & m.mask

	for {
		e := &m.entries[pos]
		if e.Name == "" {
			return 0, 0, false
		}
		if e.Hash == h && bytesEqualString(name, e.Name) {
			return e.Offset, e.Index, true
		}
		pos = (pos + 1) & m.mask
	}
}

// bytesEqualString compares bytes to string without allocation.
func bytesEqualString(b []byte, s string) bool {
	if len(b) != len(s) {
		return false
	}
	for i := 0; i < len(b); i++ {
		if b[i] != s[i] {
			return false
		}
	}
	return true
}
