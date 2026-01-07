---
title: SIMD Programming in Go
order: 3
category: concepts
---

# SIMD Deep Dive: Vectorized JSON Parsing

SIMD (Single Instruction, Multiple Data) is the key technology that makes FlashJSON process JSON at 10-20GB/s instead of 500MB/s.

## What is SIMD?

SIMD allows the CPU to perform the **same operation** on **multiple data elements simultaneously**.

### Scalar vs Vector Operations

```
Scalar (Traditional):
─────────────────────
Operation: Add 1 to each element

Iteration 1: data[0] = data[0] + 1  → 1 cycle
Iteration 2: data[1] = data[1] + 1  → 1 cycle
Iteration 3: data[2] = data[2] + 1  → 1 cycle
Iteration 4: data[3] = data[3] + 1  → 1 cycle
                                     ─────────
                                     4 cycles total

Vector (SIMD):
──────────────
Operation: Add 1 to each element

┌─────────────────────────┐
│ data[0:4]               │  Load 4 values
└─────────────────────────┘
           +
┌─────────────────────────┐
│ [1, 1, 1, 1]            │  Constant vector
└─────────────────────────┘
           =
┌─────────────────────────┐
│ result[0:4]             │  All 4 results
└─────────────────────────┘
                             1 cycle total!
```

### SIMD Register Sizes

```
x86-64 Architecture:
───────────────────

SSE (128-bit) - 16 bytes per operation
┌────────────────────────────────────────────────────┐
│ byte byte byte byte byte byte byte byte           │
│ byte byte byte byte byte byte byte byte           │
└────────────────────────────────────────────────────┘

AVX2 (256-bit) - 32 bytes per operation
┌────────────────────────────────────────────────────────────────────────────────────────────┐
│ byte byte byte byte byte byte byte byte byte byte byte byte byte byte byte byte           │
│ byte byte byte byte byte byte byte byte byte byte byte byte byte byte byte byte           │
└────────────────────────────────────────────────────────────────────────────────────────────┘

AVX-512 (512-bit) - 64 bytes per operation
┌────────────────────────────────────────────────────────────────────────────────────────────┐
│ 64 bytes... (8 int64s, or 16 int32s, or 64 int8s)                                         │
└────────────────────────────────────────────────────────────────────────────────────────────┘
```

---

## SIMD for JSON: The Key Insight

JSON structural characters are: `{`, `}`, `[`, `]`, `:`, `,`, `"`, `\`

We can find ALL of them in a 64-byte chunk with just a few SIMD operations:

```
Input (64 bytes):
┌─────────────────────────────────────────────────────────────────────┐
│ { " n a m e " : " A l i c e " , " a g e " : 3 0 , " c i t y " : ... │
└─────────────────────────────────────────────────────────────────────┘
  0 1 2 3 4 5 6 7 8 9 ...

Step 1: Compare with '"' (quote)
─────────────────────────────────
Load 64 bytes into YMM register
Broadcast '"' to all 64 positions
Compare equal (VPCMPEQB)
Result: bitmask

Quote positions: 0b010000100000100010001000100010001...
                   1     6     13    18   22   27  ...

Step 2: Compare with '{' 
Step 3: Compare with '}'
Step 4: Compare with ':'
Step 5: Combine masks with OR
```

**Result: A bitmask where each 1-bit indicates a structural character.**

---

## How SIMD Instructions Work

### Loading Data

```asm
; AVX2: Load 32 bytes from memory into YMM0 register
VMOVDQU YMM0, [data]    ; YMM = 256-bit register

; Conceptually:
; YMM0 = data[0:32] as a single 256-bit value
```

### Broadcasting a Value

```asm
; Put the byte '"' (0x22) in every lane of a 256-bit register
VPBROADCASTB YMM1, '"'

; YMM1 now contains:
; [0x22, 0x22, 0x22, 0x22, ... 32 times]
```

### Parallel Comparison

```asm
; Compare YMM0 (data) with YMM1 (quotes) byte-by-byte
VPCMPEQB YMM2, YMM0, YMM1

; If data[i] == '"', then result[i] = 0xFF (all 1s)
; If data[i] != '"', then result[i] = 0x00 (all 0s)
```

### Extract Bitmask

```asm
; Convert 32-byte comparison result to 32-bit mask
VPMOVMSKB EAX, YMM2

; EAX now contains a 32-bit integer where:
; bit i = 1 if data[i] was a quote
; bit i = 0 otherwise
```

---

## Writing SIMD in Go

Go doesn't have native SIMD syntax. We have three options:

### Option 1: Assembly Files (.s)

Go supports inline assembly via `.s` files:

```
project/
├── simd_amd64.s      # Assembly for x86-64
├── simd_arm64.s      # Assembly for ARM
└── simd.go           # Go declarations
```

**simd.go** (declarations):
```go
//go:build amd64 || arm64

package flashjson

// Implemented in assembly
func findQuotesSIMD(data []byte) uint64

// Implemented in assembly
func findStructuralSIMD(data []byte) (quotes, braces, colons uint64)
```

**simd_amd64.s** (implementation):
```asm
#include "textflag.h"

// func findQuotesSIMD(data []byte) uint64
TEXT ·findQuotesSIMD(SB), NOSPLIT, $0-32
    MOVQ data_base+0(FP), SI    // SI = pointer to data
    MOVQ data_len+8(FP), CX      // CX = length
    
    // Broadcast '"' (0x22) to all 32 lanes
    MOVD $0x22, X0
    VPBROADCASTB Y0, X0
    
    // Load 32 bytes of data
    VMOVDQU Y1, (SI)
    
    // Compare
    VPCMPEQB Y2, Y1, Y0
    
    // Extract mask
    VPMOVMSKB AX, Y2
    
    // Return
    MOVQ AX, ret+24(FP)
    VZEROUPPER
    RET
```

### Option 2: Use Existing Libraries

Libraries that provide SIMD wrappers:

```go
import "github.com/klauspost/cpuid/v2"  // CPU feature detection
import "github.com/minio/simdjson-go"   // SIMD JSON (simdjson port)
```

### Option 3: Compiler Intrinsics (Future)

Go is exploring adding SIMD intrinsics. Not available yet.

---

## FlashJSON's SIMD Strategy

We'll use a hybrid approach:

```
┌─────────────────────────────────────────────────────────────────┐
│                      FlashJSON SIMD Layer                       │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────────┐    ┌─────────────────┐                    │
│  │   simd_amd64.s  │    │   simd_arm64.s  │                    │
│  │                 │    │                 │                    │
│  │  - AVX2 code    │    │  - NEON code    │                    │
│  │  - 32 bytes/op  │    │  - 16 bytes/op  │                    │
│  └────────┬────────┘    └────────┬────────┘                    │
│           │                      │                              │
│           └──────────┬───────────┘                              │
│                      │                                          │
│                      ▼                                          │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                  simd.go (Go interface)                  │   │
│  │                                                          │   │
│  │  //go:noescape                                          │   │
│  │  func indexJSON(data []byte, tape *Tape)                │   │
│  │                                                          │   │
│  │  // Fallback for unsupported CPUs                       │   │
│  │  func indexJSONGeneric(data []byte, tape *Tape)         │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### CPU Feature Detection

```go
import "github.com/klauspost/cpuid/v2"

var (
    hasAVX2   = cpuid.CPU.Has(cpuid.AVX2)
    hasAVX512 = cpuid.CPU.Has(cpuid.AVX512F)
    hasNEON   = runtime.GOARCH == "arm64"  // ARM always has NEON
)

func IndexJSON(data []byte, tape *Tape) {
    switch {
    case hasAVX512:
        indexJSONAVX512(data, tape)
    case hasAVX2:
        indexJSONAVX2(data, tape)
    case hasNEON:
        indexJSONNEON(data, tape)
    default:
        indexJSONGeneric(data, tape)
    }
}
```

---

## The Tape: SIMD Output Format

SIMD produces a "Tape" - a flat array describing JSON structure:

```go
type TapeEntry struct {
    Type   uint8   // Object, Array, String, Number, etc.
    Offset uint32  // Byte offset in input
    Length uint32  // Length for strings/numbers
    // Parent/sibling links for navigation
}

type Tape struct {
    entries []TapeEntry
    count   int
}
```

### Example

```json
{"name":"Alice","age":30}
```

```
Tape entries:
┌───────┬────────┬────────┬────────────────────┐
│ Index │  Type  │ Offset │ Description        │
├───────┼────────┼────────┼────────────────────┤
│   0   │ Object │   0    │ { at position 0    │
│   1   │ Key    │   1    │ "name" starts at 1 │
│   2   │ String │   8    │ "Alice" starts at 8│
│   3   │ Key    │   16   │ "age" starts at 16 │
│   4   │ Number │   22   │ 30 starts at 22    │
│   5   │ End    │   24   │ } at position 24   │
└───────┴────────┴────────┴────────────────────┘
```

**Why Tape?**
- SIMD produces offsets, not parsed values
- Tape is cache-friendly (sequential access)
- Second pass uses Tape to extract values without re-scanning

---

## SIMD Safety: Handling Edge Cases

### Problem 1: Quotes Inside Strings

```json
{"message":"He said \"hello\""}
```

The backslash escapes the quotes. SIMD sees all `"`s, but not all are structural.

**Solution: Escape Processing**

```
Step 1: Find all backslashes
mask_backslash = 0b000000010000000100000000...

Step 2: Find "odd" backslashes (not escaped themselves)
This requires sequential processing (tricky!)

Step 3: Mask out quotes after odd backslashes
```

This is the hardest part of SIMD JSON parsing!

### Problem 2: Buffer Boundaries

SIMD reads 32/64 bytes at a time. What if the JSON is 100 bytes?

```
JSON: 100 bytes
SIMD chunks: [0-31], [32-63], [64-95], [96-???]

Last chunk would read past the buffer!
```

**Solution: Padding or special last-chunk handling**

```go
func IndexJSON(data []byte) *Tape {
    n := len(data)
    
    // Process complete 64-byte chunks with SIMD
    for i := 0; i+64 <= n; i += 64 {
        indexChunkSIMD(data[i:i+64], tape)
    }
    
    // Handle remaining bytes with scalar code
    remaining := n % 64
    if remaining > 0 {
        indexChunkScalar(data[n-remaining:], tape)
    }
    
    return tape
}
```

---

## Performance Expectations

| Method | Throughput | Notes |
|--------|-----------|-------|
| byte-by-byte | 300-500 MB/s | Branch-limited |
| SIMD (AVX2) | 3-5 GB/s | For indexing only |
| SIMD (AVX-512) | 5-10 GB/s | Newest CPUs |
| Theoretical max | ~20 GB/s | Memory bandwidth limit |

FlashJSON target: **2-4 GB/s** for complete parse (index + bind)

---

## Next Steps

Before we implement SIMD, we need to understand:
- [04-unsafe-go.md](./04-unsafe-go.md) - Working with raw memory
- [../architecture/01-tape-design.md](../architecture/01-tape-design.md) - Detailed Tape format

Then we'll implement:
1. Generic (pure Go) indexer first
2. AVX2 assembly
3. NEON assembly
