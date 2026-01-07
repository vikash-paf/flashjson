---
title: FlashJSON Architecture Overview
order: 1
category: architecture
---

# FlashJSON Architecture Overview

This document describes the high-level architecture of FlashJSON and how its components work together.

## Design Goals

1. **Speed**: 5-10x faster than encoding/json
2. **Low Memory**: Near-zero GC pressure
3. **Compatibility**: Drop-in replacement for encoding/json
4. **Stability**: No crashes, even on malformed input
5. **Portability**: Works on x86-64 and ARM64

## Architecture Layers

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              USER CODE                                       │
│                                                                             │
│   result, err := flashjson.Unmarshal(data, &myStruct)                       │
│                                                                             │
└───────────────────────────────────────────┬─────────────────────────────────┘
                                            │
                                            ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         LAYER 3: PUBLIC API                                  │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  Marshal(v interface{}) ([]byte, error)                              │   │
│  │  Unmarshal(data []byte, v interface{}) error                         │   │
│  │  NewEncoder(w io.Writer) *Encoder                                    │   │
│  │  NewDecoder(r io.Reader) *Decoder                                    │   │
│  │                                                                      │   │
│  │  + json.Marshaler/Unmarshaler interface support                      │   │
│  │  + struct tag parsing (json:"name,omitempty")                        │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
└───────────────────────────────────────────┬─────────────────────────────────┘
                                            │
                                            ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                       LAYER 2: OPCODE INTERPRETER                            │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  Type Cache: map[reflect.Type]*CompiledSchema                        │   │
│  │                                                                      │   │
│  │  OpCodes:                                                            │   │
│  │    OP_OBJECT_START    → expect '{'                                   │   │
│  │    OP_FIND_KEY "name" → locate key in object                         │   │
│  │    OP_READ_STRING_AT  → extract string to field offset               │   │
│  │    OP_READ_INT_AT     → extract int to field offset                  │   │
│  │    OP_SKIP_VALUE      → skip unknown fields                          │   │
│  │    OP_OBJECT_END      → expect '}'                                   │   │
│  │                                                                      │   │
│  │  The "VM" executes opcodes against the Tape                          │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
└───────────────────────────────────────────┬─────────────────────────────────┘
                                            │
                                            ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                      LAYER 1: TAPE (STRUCTURAL INDEX)                        │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  Pass 1 Output: Flat array of structural positions                  │   │
│  │                                                                      │   │
│  │  ┌──────┬──────┬──────┬──────┬──────┬──────┬──────┬──────┐          │   │
│  │  │ OBJ  │ KEY  │ STR  │ KEY  │ NUM  │ KEY  │ BOOL │ END  │          │   │
│  │  │ @0   │ @1   │ @8   │ @16  │ @22  │ @25  │ @32  │ @37  │          │   │
│  │  └──────┴──────┴──────┴──────┴──────┴──────┴──────┴──────┘          │   │
│  │                                                                      │   │
│  │  Tape enables O(1) navigation: "skip this object" = jump to END     │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
└───────────────────────────────────────────┬─────────────────────────────────┘
                                            │
                                            ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                       LAYER 0: SIMD INDEXER                                  │
│                                                                             │
│  ┌──────────────────────┐  ┌──────────────────────┐  ┌──────────────────┐  │
│  │   AVX2 (x86-64)      │  │   NEON (ARM64)       │  │  Generic (Go)    │  │
│  │                      │  │                      │  │                  │  │
│  │  32 bytes/cycle      │  │  16 bytes/cycle      │  │  1 byte/cycle    │  │
│  │  Bitmask operations  │  │  Bitmask operations  │  │  Fallback        │  │
│  └──────────────────────┘  └──────────────────────┘  └──────────────────┘  │
│                                                                             │
│  CPU Feature Detection → Select fastest available implementation            │
└───────────────────────────────────────────┬─────────────────────────────────┘
                                            │
                                            ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                        MEMORY LAYER: ARENA                                   │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  sync.Pool → Arena (64KB) → All temporary allocations               │   │
│  │                                                                      │   │
│  │  On request:     arena := pool.Get()                                 │   │
│  │  During parse:   temp := arena.Alloc(n)                             │   │
│  │  On complete:    arena.Reset(); pool.Put(arena)                      │   │
│  │                                                                      │   │
│  │  Result: ~0 allocations per parse                                   │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Data Flow: Unmarshal

```
Input: []byte(`{"name":"Alice","age":30}`)
Target: *User{Name string; Age int}

┌─────────────────────────────────────────────────────────────────┐
│ STEP 1: Get Resources                                           │
│                                                                 │
│   arena := arenaPool.Get()                                      │
│   schema := typeCache.Get(reflect.TypeOf(User{}))               │
│                                                                 │
│   If schema not cached:                                         │
│     - Analyze type with reflection                              │
│     - Generate OpCodes                                          │
│     - Cache for future calls                                    │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ STEP 2: Build Tape (SIMD Pass)                                  │
│                                                                 │
│   tape := indexJSON(data)                                       │
│                                                                 │
│   Tape = [ {OBJ,0}, {KEY,1,4}, {STR,8,5}, {KEY,16,3}, ... ]    │
│                                                                 │
│   Time: ~50ns for this example (SIMD)                          │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ STEP 3: Execute OpCodes                                         │
│                                                                 │
│   OpCodes for User:                                            │
│     [0] OP_OBJECT_START                                        │
│     [1] OP_FIND_KEY "name" → field offset 0                    │
│     [2] OP_READ_STRING                                         │
│     [3] OP_FIND_KEY "age" → field offset 16                    │
│     [4] OP_READ_INT                                            │
│     [5] OP_OBJECT_END                                          │
│                                                                 │
│   VM walks Tape, matches OpCodes, writes to struct             │
│                                                                 │
│   Key lookup: Tape tells us exactly where each key starts      │
│   Value read: Tape tells us value start/end, extract directly  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ STEP 4: Cleanup                                                 │
│                                                                 │
│   arena.Reset()      // "Free" all temporary allocations       │
│   arenaPool.Put(arena)  // Reuse next request                  │
│                                                                 │
│   Return populated *User                                        │
└─────────────────────────────────────────────────────────────────┘
```

---

## Key Design Decisions

### 1. Two-Pass Architecture

**Why not single-pass like encoding/json?**

Single-pass must handle structure AND values simultaneously:
- Complex state machine
- Many branches
- Hard to optimize

Two-pass separates concerns:
- Pass 1 (SIMD): Find structure, no parsing
- Pass 2 (OpCode): Know exactly where values are, direct extraction

### 2. OpCode VM vs JIT

**Why not JIT like Sonic?**

JIT (Just-In-Time compilation):
- ✅ Maximum speed
- ❌ Memory heavy (generated code)
- ❌ Platform issues (code signing on macOS, etc.)
- ❌ Startup cost

OpCode VM:
- ✅ ~90% of JIT speed
- ✅ Minimal memory
- ✅ Portable
- ✅ Debuggable

### 3. Compatibility First

We support encoding/json's full feature set:
- `json:"name,omitempty"` tags
- `json.Marshaler` / `json.Unmarshaler` interfaces
- Anonymous embedded structs
- Pointer handling

Code that works with encoding/json works with FlashJSON:

```go
// Before
import "encoding/json"

// After
import json "github.com/vikash-paf/flashjson"

// No other changes needed!
```

---

## Directory Structure

```
flashjson/
├── docs/
│   ├── concepts/           # Learning materials
│   │   ├── 01-cpu-fundamentals.md
│   │   ├── 02-memory-management.md
│   │   └── 03-simd-deep-dive.md
│   ├── architecture/       # Design docs
│   │   └── 01-overview.md (this file)
│   └── implementation/     # Build guides
│
├── internal/
│   ├── arena/              # Memory allocator
│   │   ├── arena.go
│   │   └── arena_test.go
│   │
│   ├── tape/               # Structural index
│   │   ├── tape.go
│   │   ├── tape_test.go
│   │   └── types.go
│   │
│   ├── simd/               # SIMD implementations
│   │   ├── detect.go       # CPU feature detection
│   │   ├── indexer.go      # Go interface
│   │   ├── indexer_generic.go
│   │   ├── indexer_amd64.go
│   │   ├── indexer_amd64.s  # AVX2 assembly
│   │   ├── indexer_arm64.go
│   │   └── indexer_arm64.s  # NEON assembly
│   │
│   └── vm/                 # OpCode interpreter
│       ├── compiler.go     # Type → OpCodes
│       ├── opcodes.go      # OpCode definitions
│       └── executor.go     # VM execution
│
├── flashjson.go            # Public API
├── decoder.go              # Decoder implementation
├── encoder.go              # Encoder implementation
└── flashjson_test.go       # Tests and benchmarks
```

---

## Implementation Phases

### Phase 1: Foundation (Pure Go)
- Arena allocator
- Tape structure
- Generic (non-SIMD) indexer
- Basic OpCode VM

### Phase 2: Compatibility
- Full encoding/json API
- Interface support
- Tag parsing
- Edge cases

### Phase 3: Performance
- AVX2 assembly
- NEON assembly
- Benchmarking
- Profiling

### Phase 4: Hardening
- Fuzz testing
- Error handling
- Documentation
- Examples

---

## Next: Implementation Details

- [02-tape-design.md](./02-tape-design.md) - Tape format specification
- [03-opcode-design.md](./03-opcode-design.md) - OpCode instruction set
- [../implementation/01-getting-started.md](../implementation/01-getting-started.md) - Start coding!
