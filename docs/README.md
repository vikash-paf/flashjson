---
title: FlashJSON Documentation Index
---

# FlashJSON Documentation

Welcome to the FlashJSON documentation! This library aims to be the world's fastest JSON encoder/decoder for Go while teaching you systems programming along the way.

## 📚 Concepts (Start Here!)

Learn the fundamental concepts that make high-performance JSON parsing possible:

1. **[CPU Fundamentals](./concepts/01-cpu-fundamentals.md)**
   - CPU pipelines and instruction-level parallelism
   - Branch prediction and why it matters
   - Cache hierarchy and memory access patterns
   - Introduction to SIMD

2. **[Memory Management](./concepts/02-memory-management.md)**
   - Go's garbage collector internals
   - Stack vs heap allocation
   - sync.Pool for object reuse
   - Arena allocators for zero-GC parsing
   - Unsafe string conversions

3. **[SIMD Deep Dive](./concepts/03-simd-deep-dive.md)**
   - Scalar vs vector operations
   - AVX2 and NEON instruction sets
   - Writing assembly in Go
   - SIMD for JSON structural indexing

## 🏗️ Architecture

Understand how FlashJSON is designed:

1. **[Architecture Overview](./architecture/01-overview.md)**
   - Four-layer design (SIMD → Tape → OpCode → API)
   - Data flow diagrams
   - Design decisions and tradeoffs
   - Directory structure

## 🔧 Implementation

Build it yourself:

(Coming as we implement each component)

1. Arena Allocator
2. Tape Structure
3. Generic Indexer
4. OpCode Compiler
5. SIMD Indexer (AVX2)
6. SIMD Indexer (NEON)
7. Public API

## 🎯 Quick Reference

| Document | Purpose |
|----------|---------|
| [CPU Fundamentals](./concepts/01-cpu-fundamentals.md) | Why byte-by-byte parsing is slow |
| [Memory Management](./concepts/02-memory-management.md) | How to avoid GC overhead |
| [SIMD Deep Dive](./concepts/03-simd-deep-dive.md) | Processing 32+ bytes at once |
| [Architecture](./architecture/01-overview.md) | How it all fits together |

## 🚀 Getting Started

```bash
# Clone the repository
git clone https://github.com/vikash-paf/flashjson

# Read the docs
cd flashjson/docs

# Run the (future) examples
go run examples/basic/main.go
```

## 📖 Recommended Reading Order

For learning systems programming through this project:

1. Read [CPU Fundamentals](./concepts/01-cpu-fundamentals.md) - understand the problem
2. Read [Memory Management](./concepts/02-memory-management.md) - understand allocation
3. Read [SIMD Deep Dive](./concepts/03-simd-deep-dive.md) - understand the solution
4. Read [Architecture Overview](./architecture/01-overview.md) - see the design
5. Follow implementation guides as we build each component

Happy learning! 🎓
