---
title: CPU Fundamentals for High-Performance Go
order: 1
category: concepts
---

# CPU Fundamentals for High-Performance Go

Understanding how CPUs work is **essential** for writing fast code. This document covers the concepts that directly impact JSON parsing performance.

## The CPU Pipeline

Modern CPUs don't execute one instruction at a time. They use a **pipeline** - think of it like an assembly line in a factory.

```
┌─────────┐   ┌─────────┐   ┌─────────┐   ┌─────────┐   ┌─────────┐
│  Fetch  │ → │ Decode  │ → │ Execute │ → │ Memory  │ → │  Write  │
│         │   │         │   │         │   │ Access  │   │  Back   │
└─────────┘   └─────────┘   └─────────┘   └─────────┘   └─────────┘
     ↓             ↓             ↓             ↓             ↓
   Inst 1       Inst 1       Inst 1       Inst 1       Inst 1
   Inst 2       Inst 2       Inst 2       Inst 2       
   Inst 3       Inst 3       Inst 3       
   Inst 4       Inst 4       
   Inst 5       
```

**Key insight:** While instruction 1 is writing results, instructions 2-5 are already in various stages. This is called **Instruction-Level Parallelism (ILP)**.

### Why This Matters for JSON Parsing

Traditional JSON parsers do this:

```go
for i := 0; i < len(data); i++ {
    switch data[i] {
    case '{':
        // handle object start
    case '}':
        // handle object end
    case '"':
        // handle string
    // ... many more cases
    }
}
```

This code has a **branch** (the switch statement) on every single byte. Branches are the enemy of pipelines.

---

## Branch Prediction (The CPU's Crystal Ball)

CPUs try to **predict** which way a branch will go before they know the answer. They start executing the predicted path speculatively.

### How It Works

```
                    ┌──────────────────┐
                    │ Branch Predictor │
                    │   (History Table)│
                    └────────┬─────────┘
                             │ "I think it's TRUE"
                             ▼
   ┌─────────────────────────────────────────────┐
   │ Pipeline continues with speculative work... │
   └─────────────────────────────────────────────┘
                             │
              ┌──────────────┴──────────────┐
              ▼                             ▼
       ✅ Correct!                    ❌ Wrong!
       (Continue)                    (FLUSH pipeline,
                                      start over)
```

### The Cost of Misprediction

When the CPU predicts wrong:
1. All speculative work is **thrown away**
2. Pipeline is **flushed** (emptied)
3. CPU starts over from the correct path

**Cost: 15-20 CPU cycles wasted per misprediction**

### JSON Parsing Problem

JSON characters are essentially random:

```json
{"name":"Alice","age":30,"active":true}
```

The CPU cannot predict whether the next byte is `{`, `"`, `:`, or something else. **Every branch is a coin flip.**

With ~50% misprediction rate and 20 cycles per miss:
- 1MB JSON ≈ 1 million bytes
- ~500,000 mispredictions
- ~10 million wasted cycles

**This is why byte-by-byte parsing is slow.**

---

## Cache Hierarchy (The Memory Wall)

CPUs are fast. Memory is slow. The gap is called the **Memory Wall**.

```
┌────────────────────────────────────────────────────────┐
│                    CPU Core                            │
│  ┌─────────────┐                                       │
│  │  Registers  │  ← 0 cycles (instant)                 │
│  └─────────────┘                                       │
│  ┌─────────────┐                                       │
│  │  L1 Cache   │  ← 4 cycles (~1ns) - 32KB             │
│  │  (per core) │                                       │
│  └─────────────┘                                       │
│  ┌─────────────┐                                       │
│  │  L2 Cache   │  ← 12 cycles (~3ns) - 256KB           │
│  │  (per core) │                                       │
│  └─────────────┘                                       │
└────────────────────────────────────────────────────────┘
┌────────────────────────────────────────────────────────┐
│  ┌─────────────┐                                       │
│  │  L3 Cache   │  ← 40 cycles (~10ns) - 8-32MB         │
│  │  (shared)   │                                       │
│  └─────────────┘                                       │
└────────────────────────────────────────────────────────┘
┌────────────────────────────────────────────────────────┐
│  ┌─────────────┐                                       │
│  │ Main Memory │  ← 200+ cycles (~60ns) - GBs          │
│  │   (RAM)     │                                       │
│  └─────────────┘                                       │
└────────────────────────────────────────────────────────┘
```

### Cache Lines

Memory is loaded in **64-byte chunks** called cache lines.

```go
// Accessing data[0] loads bytes 0-63 into cache
// Accessing data[1] is then FREE (already cached!)
// Accessing data[64] triggers another memory fetch
```

### Implications for FlashJSON

1. **Sequential access is fast** - CPU prefetches next cache lines
2. **Random access is slow** - each jump may miss cache
3. **Keep data together** - our "Tape" structure is a contiguous array

---

## SIMD: Single Instruction, Multiple Data

This is our **secret weapon**. Instead of processing 1 byte per instruction, we process 32 or 64 bytes.

### The Concept

```
Traditional (Scalar):
┌───┐ ┌───┐ ┌───┐ ┌───┐
│ A │ │ B │ │ C │ │ D │   4 instructions
└───┘ └───┘ └───┘ └───┘   to add 4 numbers
  +     +     +     +
  1     1     1     1
  =     =     =     =
  A+1   B+1   C+1   D+1

SIMD (Vector):
┌───────────────────┐
│   A   B   C   D   │     1 instruction
└───────────────────┘     to add 4 numbers
          +
┌───────────────────┐
│   1   1   1   1   │
└───────────────────┘
          =
┌───────────────────┐
│  A+1 B+1 C+1 D+1  │
└───────────────────┘
```

### SIMD for JSON

We can check 64 bytes at once for structural characters:

```
Input bytes (64 at a time):
┌─────────────────────────────────────────────────────────────────┐
│ { " n a m e " : " A l i c e " , " a g e " : 3 0 }  ...          │
└─────────────────────────────────────────────────────────────────┘

Step 1: Compare all 64 bytes with '"' simultaneously
┌─────────────────────────────────────────────────────────────────┐
│ 0 1 0 0 0 0 1 0 1 0 0 0 0 0 1 0 1 0 0 0 1 0 0 0 0  ...          │
└─────────────────────────────────────────────────────────────────┘
  (1 = match, 0 = no match)

Step 2: Compare all 64 bytes with '{' simultaneously
Step 3: Compare all 64 bytes with '}' simultaneously
Step 4: Combine masks with OR

Result: Bitmask of all structural character positions
```

**No branches. Just math. Perfect for the pipeline.**

### SIMD Instruction Sets

| Platform | Instruction Set | Register Size | Bytes/Op |
|----------|-----------------|---------------|----------|
| Intel/AMD (older) | SSE4.2 | 128-bit | 16 |
| Intel/AMD (modern) | AVX2 | 256-bit | 32 |
| Intel/AMD (newest) | AVX-512 | 512-bit | 64 |
| Apple Silicon / ARM | NEON | 128-bit | 16 |
| ARM (SVE) | SVE/SVE2 | Variable | 16-256 |

FlashJSON will use:
- **AVX2** for x86-64 (most common, good balance)
- **NEON** for ARM64 (Apple M-series, AWS Graviton)

---

## Practical Example: Finding a Quote Character

### Naive Go (Slow)

```go
func findQuotes(data []byte) []int {
    positions := make([]int, 0)
    for i, b := range data {
        if b == '"' {
            positions = append(positions, i)
        }
    }
    return positions
}
```

Problems:
- Branch on every byte
- Slice append may allocate
- One byte per iteration

### Conceptual SIMD (Fast)

```go
func findQuotesSIMD(data []byte) uint64 {
    // Load 64 bytes into SIMD register
    chunk := simd.Load(data[0:64])
    
    // Compare all 64 bytes with '"' at once
    // Returns a 64-bit mask where each bit = 1 if match
    mask := simd.CompareEqual(chunk, '"')
    
    return mask // e.g., 0b0100001000100010...
}

// To get positions from mask:
// mask = 0b01000010
// positions = [1, 6] (bit positions where 1 appears)
```

The actual implementation uses assembly, but this is the concept.

---

## Summary: Why FlashJSON Will Be Fast

| Problem | Standard Approach | FlashJSON Approach |
|---------|-------------------|-------------------|
| Branch misprediction | if/switch per byte | SIMD bitmasks (no branches) |
| Memory latency | Random struct access | Sequential Tape array |
| Instruction throughput | 1 byte/instruction | 32-64 bytes/instruction |
| Cache efficiency | Scattered allocations | Arena allocator (contiguous) |

---

## Next Steps

- [02-memory-management.md](./02-memory-management.md) - Understanding Go's GC and arena allocators
- [03-simd-deep-dive.md](./03-simd-deep-dive.md) - Writing SIMD code for Go
