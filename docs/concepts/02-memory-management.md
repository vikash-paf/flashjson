---
title: Memory Management & Go's GC
order: 2
category: concepts
---

# Memory Management for High-Performance Go

Memory allocation is the **second biggest performance killer** after branch misprediction. This document explains why, and how we'll solve it.

## The Problem: Garbage Collection

Go uses a **tracing garbage collector**. It's concurrent and low-latency, but it's not free.

### How Go's GC Works (Simplified)

```
┌─────────────────────────────────────────────────────────────────┐
│                        Go Runtime                               │
│                                                                 │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐         │
│  │  Goroutine  │    │  Goroutine  │    │  Goroutine  │         │
│  │  (your code)│    │  (your code)│    │  (GC worker)│         │
│  └─────────────┘    └─────────────┘    └─────────────┘         │
│         │                 │                  │                  │
│         ▼                 ▼                  ▼                  │
│  ┌─────────────────────────────────────────────────────┐       │
│  │                     HEAP                             │       │
│  │  ┌───┐ ┌───┐ ┌───┐ ┌───┐ ┌───┐ ┌───┐ ┌───┐ ┌───┐   │       │
│  │  │ A │ │ B │ │ C │ │ D │ │ E │ │ F │ │ G │ │ H │   │       │
│  │  └───┘ └───┘ └───┘ └───┘ └───┘ └───┘ └───┘ └───┘   │       │
│  │    ↑           ↑                       ↑           │       │
│  │   Live       Live                    Live          │       │
│  │  (keep)     (keep)                  (keep)         │       │
│  │        Dead: B, D, E, F, H → will be freed         │       │
│  └─────────────────────────────────────────────────────┘       │
└─────────────────────────────────────────────────────────────────┘
```

### GC Phases

1. **Mark Phase**: Trace all reachable objects from roots (stacks, globals)
2. **Sweep Phase**: Free unreachable objects
3. **STW (Stop-The-World)**: Brief pauses where all goroutines stop

### The Cost

```go
// Each allocation has overhead:
// 1. Lock the memory allocator (contention)
// 2. Find a free slot of the right size
// 3. Zero the memory
// 4. Track for GC

result := make(map[string]interface{})  // Allocation
result["name"] = string(data[10:20])     // Another allocation
result["age"] = 30                        // Interface boxing = allocation
```

**Standard json.Unmarshal does HUNDREDS of allocations per parse.**

---

## Memory Layout in Go

Understanding how Go lays out data in memory helps us optimize.

### Stack vs Heap

```
┌─────────────────────────────────────────────────────────────────┐
│                         STACK                                   │
│   (per goroutine, ~2KB initial, grows automatically)            │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ Function frame: parseJSON()                              │   │
│  │   - local int: 8 bytes                                   │   │
│  │   - local [64]byte: 64 bytes (STAYS ON STACK!)          │   │
│  │   - pointer to heap: 8 bytes                            │   │
│  └─────────────────────────────────────────────────────────┘   │
│                           │                                     │
│                           │ pointer                             │
│                           ▼                                     │
├─────────────────────────────────────────────────────────────────┤
│                         HEAP                                    │
│   (shared, managed by GC)                                       │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ Large slice: make([]byte, 1024)                         │   │
│  │ Maps, channels, interfaces with pointers                │   │
│  └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

### Escape Analysis

Go's compiler decides whether variables go on stack or heap:

```go
func stackAlloc() int {
    x := 42        // Stays on stack (no escape)
    return x
}

func heapAlloc() *int {
    x := 42        // ESCAPES to heap (returned pointer)
    return &x      // Compiler: "x must outlive this function"
}
```

Check escape analysis:
```bash
go build -gcflags="-m" ./...
# Output shows what escapes to heap
```

---

## Solution 1: sync.Pool (Object Recycling)

`sync.Pool` lets you reuse objects instead of allocating new ones.

### How It Works

```
Request 1:                    Request 2:
─────────                    ─────────
pool.Get() → allocate new    pool.Get() → reuse from pool!
use buffer                   use buffer
pool.Put(buffer)             pool.Put(buffer)
         │                            │
         ▼                            ▼
    ┌─────────────────────────────────────────┐
    │              sync.Pool                   │
    │  ┌───────┐ ┌───────┐ ┌───────┐         │
    │  │Buffer1│ │Buffer2│ │Buffer3│ ...     │
    │  └───────┘ └───────┘ └───────┘         │
    └─────────────────────────────────────────┘
```

### Example: Reusable Byte Buffer

```go
var bufferPool = sync.Pool{
    New: func() interface{} {
        // Only called when pool is empty
        buf := make([]byte, 0, 4096)
        return &buf
    },
}

func processJSON(data []byte) (result []byte, err error) {
    // Get buffer from pool (reuses existing if available)
    bufPtr := bufferPool.Get().(*[]byte)
    buf := *bufPtr
    buf = buf[:0]  // Reset length, keep capacity
    
    defer func() {
        *bufPtr = buf
        bufferPool.Put(bufPtr)  // Return to pool for reuse
    }()
    
    // Use buf for processing...
    buf = append(buf, data...)
    
    return buf, nil
}
```

### Pool Behavior

- **Get**: Returns cached object OR calls New
- **Put**: Returns object to pool for reuse
- **GC**: Pool may be cleared during GC (that's okay)

---

## Solution 2: Arena Allocator (The Big Gun)

For maximum performance, we implement our own allocator that bypasses GC entirely.

### Concept

```
Traditional Allocation:          Arena Allocation:
────────────────────            ─────────────────
┌───┐                           ┌─────────────────────────────┐
│ A │ malloc                    │ Arena (4KB block)           │
└───┘                           │ ┌───┬───┬───┬───┬─────────┐ │
┌───┐                           │ │ A │ B │ C │ D │  free   │ │
│ B │ malloc                    │ └───┴───┴───┴───┴─────────┘ │
└───┘                           │       ↑                     │
┌───┐                           │    offset                   │
│ C │ malloc                    └─────────────────────────────┘
└───┘                           
┌───┐                           To "free" everything:
│ D │ malloc                    Just reset offset to 0!
└───┘                           Cost: ONE assignment
                                
4 allocations                   1 allocation, 0 frees
4 GC tracked objects            1 GC tracked object
```

### Implementation

```go
// Arena is a simple bump allocator
type Arena struct {
    buf    []byte      // The memory block
    offset int         // Current position
}

// NewArena creates an arena from a pooled buffer
func NewArena(size int) *Arena {
    return &Arena{
        buf:    make([]byte, size),
        offset: 0,
    }
}

// Alloc allocates n bytes from the arena
func (a *Arena) Alloc(n int) []byte {
    if a.offset+n > len(a.buf) {
        // Could grow or return error
        panic("arena overflow")
    }
    
    result := a.buf[a.offset : a.offset+n]
    a.offset += n
    
    // Align to 8 bytes for performance
    a.offset = (a.offset + 7) &^ 7
    
    return result
}

// Reset "frees" all allocations instantly
func (a *Arena) Reset() {
    a.offset = 0
    // That's it! No GC, no free(), nothing.
}
```

### Arena + sync.Pool = FlashJSON's Memory Strategy

```go
var arenaPool = sync.Pool{
    New: func() interface{} {
        return NewArena(64 * 1024)  // 64KB per arena
    },
}

func Unmarshal(data []byte, v interface{}) error {
    // Get arena from pool
    arena := arenaPool.Get().(*Arena)
    defer func() {
        arena.Reset()         // "Free" all temp allocations
        arenaPool.Put(arena)  // Reuse arena next time
    }()
    
    // All temporary allocations use arena
    // Result struct uses normal Go allocation
    return unmarshalWithArena(data, v, arena)
}
```

---

## Solution 3: Zero-Copy Strings (Dangerous but Fast)

### The Problem

```go
// JSON input
data := []byte(`{"name":"Alice"}`)

// Normal string extraction (COPIES bytes)
name := string(data[9:14])  // Allocates new memory, copies "Alice"
```

The copy is safe but slow. What if we could avoid it?

### Unsafe String Header

In Go, a string is just a header pointing to bytes:

```go
// From reflect package (conceptual)
type StringHeader struct {
    Data uintptr  // Pointer to bytes
    Len  int      // Length
}

type SliceHeader struct {
    Data uintptr  // Pointer to bytes
    Len  int      // Length
    Cap  int      // Capacity
}

// They're almost identical!
```

### Zero-Copy Conversion

```go
import "unsafe"

// WARNING: Dangerous! Only use when you know input won't be reused
func bytesToStringUnsafe(b []byte) string {
    return *(*string)(unsafe.Pointer(&b))
}

// The string now SHARES memory with the byte slice!
// If the byte slice changes, the string becomes garbage.
```

### Visual Representation

```
Safe (with copy):
─────────────────
data []byte:  ┌─────────────────┐
              │ A l i c e       │
              └─────────────────┘
                     │
                     │ copy bytes
                     ▼
name string:  ┌─────────────────┐
              │ A l i c e       │  (new memory)
              └─────────────────┘

Unsafe (zero-copy):
───────────────────
data []byte:  ┌─────────────────┐
              │ A l i c e       │
              └────────▲────────┘
                       │
                       │ pointer only (no copy!)
                       │
name string:  ┌────────┴────────┐
              │  points here    │
              └─────────────────┘
```

### FlashJSON's Approach

```go
type Config struct {
    // CopyStrings: if false, strings point directly into input
    // Faster, but input must not be reused until structs are done
    CopyStrings bool
}

// Default: safe mode
var DefaultConfig = Config{CopyStrings: true}

// For maximum speed when you control the input lifetime
var UnsafeConfig = Config{CopyStrings: false}
```

---

## Memory Alignment (Bonus Optimization)

CPUs access aligned memory faster than unaligned.

```
Memory addresses:
0x00  0x08  0x10  0x18  0x20  ...
  │     │     │     │     │
  └─────┴─────┴─────┴─────┴── 8-byte boundaries

Aligned access (fast):       Unaligned access (slow):
Reading int64 at 0x08        Reading int64 at 0x05
┌────────────────┐           ┌────────────────┐
│ One load       │           │ Load 0x00-0x07 │
└────────────────┘           │ Load 0x08-0x0F │
                             │ Combine bytes  │
                             └────────────────┘
```

### Struct Field Ordering

```go
// Bad: 24 bytes (due to padding)
type Bad struct {
    a bool    // 1 byte
    // 7 bytes padding to align b
    b int64   // 8 bytes
    c bool    // 1 byte
    // 7 bytes padding
}

// Good: 16 bytes
type Good struct {
    b int64   // 8 bytes
    a bool    // 1 byte
    c bool    // 1 byte
    // 6 bytes padding (less total)
}
```

Check struct sizes:
```go
fmt.Println(unsafe.Sizeof(Bad{}))   // 24
fmt.Println(unsafe.Sizeof(Good{}))  // 16
```

---

## Summary: FlashJSON Memory Strategy

| Technique | Use Case | Benefit |
|-----------|----------|---------|
| sync.Pool | Arena blocks, temp buffers | Reuse across requests |
| Arena Allocator | Temporary parse state | Zero GC for temps |
| Zero-Copy Strings | When input lifetime is known | No string copies |
| Alignment | Tape structure, OpCodes | Faster memory access |

### Expected Allocation Comparison

| Library | Allocations per Parse | GC Pressure |
|---------|----------------------|-------------|
| encoding/json | 100-500+ | High |
| json-iterator | 50-200 | Medium |
| FlashJSON | 1-5 | Near Zero |

---

## Next Steps

- [03-simd-deep-dive.md](./03-simd-deep-dive.md) - SIMD programming in Go
- [04-unsafe-go.md](./04-unsafe-go.md) - Safe usage of unsafe package
