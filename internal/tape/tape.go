package tape

import (
	"sync"
)

// Entry represents a single element in the JSON structure.
// Size: 12 bytes (optimized for cache efficiency)
type Entry struct {
	Type   uint8   // Type of entry (see Type* constants)
	_      [3]byte // Padding for alignment
	Offset uint32  // Byte offset in the input JSON
	Length uint32  // Length in bytes (for strings, numbers, keys)
}

// Tape is a flat array of entries describing JSON structure.
// It is designed for sequential access and cache efficiency.
type Tape struct {
	entries []Entry
	count   int
}

// New creates a new Tape with the specified initial capacity.
// A good rule of thumb: capacity = expected_json_size / 8
func New(capacity int) *Tape {
	if capacity <= 0 {
		capacity = 64
	}
	return &Tape{
		entries: make([]Entry, capacity),
		count:   0,
	}
}

// Append adds a new entry to the tape.
func (t *Tape) Append(typ uint8, offset, length int) {
	if t.count >= len(t.entries) {
		t.grow()
	}
	t.entries[t.count] = Entry{
		Type:   typ,
		Offset: uint32(offset),
		Length: uint32(length),
	}
	t.count++
}

// AppendEntry adds a pre-constructed entry to the tape.
func (t *Tape) AppendEntry(e Entry) {
	if t.count >= len(t.entries) {
		t.grow()
	}
	t.entries[t.count] = e
	t.count++
}

// grow doubles the capacity of the tape.
func (t *Tape) grow() {
	newCap := len(t.entries) * 2
	if newCap < 64 {
		newCap = 64
	}
	newEntries := make([]Entry, newCap)
	copy(newEntries, t.entries[:t.count])
	t.entries = newEntries
}

// Get returns the entry at index i.
// Panics if i is out of bounds.
func (t *Tape) Get(i int) Entry {
	return t.entries[i]
}

// GetPtr returns a pointer to the entry at index i.
// This avoids copying the entry for read-modify-write patterns.
func (t *Tape) GetPtr(i int) *Entry {
	return &t.entries[i]
}

// Len returns the number of entries in the tape.
func (t *Tape) Len() int {
	return t.count
}

// Cap returns the capacity of the tape.
func (t *Tape) Cap() int {
	return len(t.entries)
}

// Reset clears the tape for reuse, keeping the underlying memory.
func (t *Tape) Reset() {
	t.count = 0
}

// Entries returns a slice of all entries.
// The returned slice should not be modified.
func (t *Tape) Entries() []Entry {
	return t.entries[:t.count]
}

// SkipValue returns the index after the value at the given index.
// For primitives, this is index+1.
// For objects/arrays, this skips to after the closing brace/bracket.
func (t *Tape) SkipValue(index int) int {
	if index >= t.count {
		return t.count
	}

	entry := t.entries[index]

	switch entry.Type {
	case TypeObjectStart:
		// Find matching ObjectEnd
		depth := 1
		i := index + 1
		for i < t.count && depth > 0 {
			switch t.entries[i].Type {
			case TypeObjectStart:
				depth++
			case TypeObjectEnd:
				depth--
			}
			i++
		}
		return i

	case TypeArrayStart:
		// Find matching ArrayEnd
		depth := 1
		i := index + 1
		for i < t.count && depth > 0 {
			switch t.entries[i].Type {
			case TypeArrayStart:
				depth++
			case TypeArrayEnd:
				depth--
			}
			i++
		}
		return i

	case TypeKey:
		// Key is followed by a value, skip both
		// First skip the key (this entry), then skip the value
		return t.SkipValue(index + 1)

	default:
		// Primitives: just skip one entry
		return index + 1
	}
}

// FindKey searches for a key in an object starting at objectIndex.
// Returns the index of the key's value, or -1 if not found.
// The objectIndex should point to an ObjectStart entry.
func (t *Tape) FindKey(objectIndex int, input []byte, key string) int {
	if objectIndex >= t.count {
		return -1
	}
	if t.entries[objectIndex].Type != TypeObjectStart {
		return -1
	}

	i := objectIndex + 1
	for i < t.count {
		entry := t.entries[i]

		switch entry.Type {
		case TypeObjectEnd:
			// Reached end of object, key not found
			return -1

		case TypeKey:
			// Compare key
			keyStart := int(entry.Offset)
			keyEnd := keyStart + int(entry.Length)
			if keyEnd <= len(input) {
				foundKey := string(input[keyStart:keyEnd])
				if foundKey == key {
					// Return index of the value (next entry)
					return i + 1
				}
			}
			// Skip this key's value
			i = t.SkipValue(i + 1)

		default:
			// Unexpected entry type in object
			i++
		}
	}

	return -1
}

// --- Pool for tape reuse ---

var pool = sync.Pool{
	New: func() interface{} {
		return New(256)
	},
}

// Get retrieves a Tape from the pool.
func Get() *Tape {
	return pool.Get().(*Tape)
}

// Put returns a Tape to the pool.
func Put(t *Tape) {
	t.Reset()
	pool.Put(t)
}

// GetSized retrieves a Tape with at least the specified capacity.
func GetSized(capacity int) *Tape {
	t := pool.Get().(*Tape)
	if t.Cap() < capacity {
		pool.Put(t)
		return New(capacity)
	}
	return t
}
