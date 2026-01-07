---
title: Tape Design - Structural JSON Index
order: 2
category: architecture
---

# Tape Design: The Structural Index

The "Tape" is FlashJSON's core data structure - a flat array that describes JSON structure without storing the actual values.

## Why Tape?

Traditional JSON parsers build a tree:

```
Traditional AST (Tree):
                    ┌──────────┐
                    │  Object  │
                    └────┬─────┘
           ┌─────────────┼─────────────┐
           ▼             ▼             ▼
      ┌────────┐    ┌────────┐    ┌────────┐
      │"name": │    │ "age": │    │"city": │
      │"Alice" │    │   30   │    │ "NYC"  │
      └────────┘    └────────┘    └────────┘

Problems:
- Many small allocations (one per node)
- Pointer chasing (cache unfriendly)
- GC pressure
```

Tape is flat:

```
Tape (Flat Array):
┌─────┬─────┬─────┬─────┬─────┬─────┬─────┬─────┐
│OBJ  │KEY  │STR  │KEY  │NUM  │KEY  │STR  │END  │
│@0   │@1   │@8   │@16  │@22  │@25  │@32  │@37  │
└─────┴─────┴─────┴─────┴─────┴─────┴─────┴─────┘

Benefits:
- Single allocation (or from arena)
- Sequential access (cache friendly)
- Zero GC pressure
```

---

## Tape Entry Format

Each entry is 12 bytes:

```go
type Entry struct {
    Type   uint8   // What kind of value (1 byte)
    _      [3]byte // Padding for alignment
    Offset uint32  // Byte position in input (4 bytes)
    Length uint32  // Length for strings/numbers (4 bytes)
}

// Total: 12 bytes per entry (aligned to 4 bytes)
```

### Entry Types

```go
const (
    TypeInvalid     uint8 = iota  // 0: Should never appear
    TypeObjectStart               // 1: {
    TypeObjectEnd                 // 2: }
    TypeArrayStart                // 3: [
    TypeArrayEnd                  // 4: ]
    TypeKey                       // 5: Object key (string)
    TypeString                    // 6: String value
    TypeNumber                    // 7: Number (int or float)
    TypeTrue                      // 8: true
    TypeFalse                     // 9: false
    TypeNull                      // 10: null
)
```

---

## Example: JSON to Tape

### Input JSON

```json
{"name":"Alice","age":30,"active":true}
```

Byte positions:
```
Position: 0         1         2         3
          0123456789012345678901234567890123456789
Content:  {"name":"Alice","age":30,"active":true}
```

### Generated Tape

| Index | Type | Offset | Length | Description |
|-------|------|--------|--------|-------------|
| 0 | ObjectStart | 0 | 0 | `{` at position 0 |
| 1 | Key | 1 | 4 | `name` at pos 1, length 4 |
| 2 | String | 8 | 5 | `Alice` at pos 8, length 5 |
| 3 | Key | 16 | 3 | `age` at pos 16, length 3 |
| 4 | Number | 22 | 2 | `30` at pos 22, length 2 |
| 5 | Key | 26 | 6 | `active` at pos 26, length 6 |
| 6 | True | 35 | 4 | `true` at pos 35 |
| 7 | ObjectEnd | 39 | 0 | `}` at position 39 |

```
Visual:
┌──────────────┬──────────────┬──────────────┬──────────────┐
│ OBJ_START    │ KEY "name"   │ STR "Alice"  │ KEY "age"    │
│ off=0        │ off=1 len=4  │ off=8 len=5  │ off=16 len=3 │
└──────────────┴──────────────┴──────────────┴──────────────┘
┌──────────────┬──────────────┬──────────────┬──────────────┐
│ NUM "30"     │ KEY "active" │ TRUE         │ OBJ_END      │
│ off=22 len=2 │ off=26 len=6 │ off=35       │ off=39       │
└──────────────┴──────────────┴──────────────┴──────────────┘
```

---

## Nested Structures

### Nested Object

```json
{"user":{"name":"Bob","age":25}}
```

Tape:
```
[0] OBJ_START   off=0
[1] KEY         off=1   len=4   "user"
[2] OBJ_START   off=8           ← nested object
[3] KEY         off=9   len=4   "name"
[4] STRING      off=16  len=3   "Bob"
[5] KEY         off=22  len=3   "age"
[6] NUMBER      off=28  len=2   "25"
[7] OBJ_END     off=30          ← closes nested
[8] OBJ_END     off=31          ← closes outer
```

### Arrays

```json
{"items":[1,2,3]}
```

Tape:
```
[0] OBJ_START   off=0
[1] KEY         off=1   len=5   "items"
[2] ARR_START   off=9
[3] NUMBER      off=10  len=1   "1"
[4] NUMBER      off=12  len=1   "2"
[5] NUMBER      off=14  len=1   "3"
[6] ARR_END     off=15
[7] OBJ_END     off=16
```

---

## Navigation with Tape

The Tape enables efficient navigation:

### Skip a Value

If we don't need a field, we can skip it entirely:

```go
func (t *Tape) skipValue(index int) int {
    entry := t.Get(index)
    
    switch entry.Type {
    case TypeObjectStart:
        // Find matching ObjectEnd
        depth := 1
        i := index + 1
        for depth > 0 {
            e := t.Get(i)
            if e.Type == TypeObjectStart {
                depth++
            } else if e.Type == TypeObjectEnd {
                depth--
            }
            i++
        }
        return i
        
    case TypeArrayStart:
        // Similar for arrays
        
    default:
        // Primitives: just skip one entry
        return index + 1
    }
}
```

**Why this is fast:** We never look at the JSON bytes, just the Tape structure!

### Enhanced Tape: Jump Pointers

For even faster skipping, we can store jump pointers:

```go
type EnhancedEntry struct {
    Type     uint8
    _        [3]byte
    Offset   uint32
    Length   uint32
    JumpTo   uint32  // Index of matching END for START entries
}
```

Now skipping is O(1):

```go
func (t *Tape) skipValueFast(index int) int {
    entry := t.Get(index)
    
    if entry.Type == TypeObjectStart || entry.Type == TypeArrayStart {
        return int(entry.JumpTo) + 1  // Instant!
    }
    
    return index + 1
}
```

---

## Implementation

### Tape Structure

```go
type Tape struct {
    entries []Entry
    count   int
    arena   *Arena  // Optional: allocate from arena
}

func NewTape(capacity int) *Tape {
    return &Tape{
        entries: make([]Entry, capacity),
        count:   0,
    }
}

func NewTapeFromArena(a *Arena, capacity int) *Tape {
    // Allocate entries slice from arena
    size := capacity * int(unsafe.Sizeof(Entry{}))
    buf := a.Alloc(size)
    
    return &Tape{
        entries: (*[1 << 20]Entry)(unsafe.Pointer(&buf[0]))[:capacity:capacity],
        count:   0,
        arena:   a,
    }
}
```

### Append Entry

```go
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

func (t *Tape) grow() {
    // Double capacity
    newCap := len(t.entries) * 2
    newEntries := make([]Entry, newCap)
    copy(newEntries, t.entries[:t.count])
    t.entries = newEntries
}
```

### Reset for Reuse

```go
func (t *Tape) Reset() {
    t.count = 0
    // Don't deallocate - reuse the slice!
}
```

### Access Entries

```go
func (t *Tape) Get(i int) Entry {
    return t.entries[i]
}

func (t *Tape) Len() int {
    return t.count
}
```

---

## Memory Efficiency

### Comparison

For the JSON `{"name":"Alice","age":30}`:

**Standard AST:**
- Object node: ~40 bytes
- 3 key nodes: ~80 bytes each = 240 bytes
- 3 value nodes: ~40 bytes each = 120 bytes
- Total: ~400 bytes + allocator overhead
- Allocations: 7+

**Tape:**
- 8 entries × 12 bytes = 96 bytes
- Allocations: 1 (or 0 with arena)

**Savings: 4x less memory, 7x fewer allocations**

### Pooling

```go
var tapePool = sync.Pool{
    New: func() interface{} {
        return NewTape(256)  // Default capacity
    },
}

func GetTape() *Tape {
    return tapePool.Get().(*Tape)
}

func PutTape(t *Tape) {
    t.Reset()
    tapePool.Put(t)
}
```

---

## Tape + SIMD

SIMD produces the Tape directly:

```
JSON bytes → SIMD Scanner → Bitmasks → Tape
```

The SIMD layer:
1. Finds structural characters ({, }, [, ], :, ,, ")
2. Tracks string boundaries (handle escapes)
3. Outputs Tape entries with correct offsets

See [03-simd-deep-dive.md](../concepts/03-simd-deep-dive.md) for SIMD details.

---

## Tape + OpCode VM

The OpCode VM consumes the Tape:

```go
func execute(tape *Tape, opcodes []OpCode, ptr unsafe.Pointer) error {
    tapeIdx := 0
    
    for _, op := range opcodes {
        switch op.Type {
        case OpFindKey:
            // Walk tape entries looking for matching key
            for tapeIdx < tape.Len() {
                entry := tape.Get(tapeIdx)
                if entry.Type == TypeKey {
                    // Compare key name using offset/length
                    // ...
                }
                tapeIdx++
            }
            
        case OpReadString:
            entry := tape.Get(tapeIdx)
            // Extract string using entry.Offset and entry.Length
            // Write to struct field at op.FieldOffset
        }
    }
    
    return nil
}
```

---

## Summary

The Tape is the bridge between SIMD scanning and struct binding:

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│ JSON Bytes  │ ──▶ │    Tape     │ ──▶ │ Go Struct   │
│ (input)     │     │ (structure) │     │ (output)    │
└─────────────┘     └─────────────┘     └─────────────┘
      │                   │                   │
   32-64 bytes         12 bytes            Direct
   per SIMD op         per entry           field write
```

**Key properties:**
- Flat array (cache-friendly)
- Fixed-size entries (simple indexing)
- Positions point to original JSON (no copies)
- Enabling lazy parsing (skip unused fields)

---

## Next Steps

- [03-opcode-design.md](./03-opcode-design.md) - OpCode instruction set
- [../implementation/02-tape.md](../implementation/02-tape.md) - Implement the Tape
