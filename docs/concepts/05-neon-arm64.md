---
title: NEON Assembly for ARM64
order: 5
category: concepts
---

# NEON Assembly for ARM64

NEON is ARM's SIMD extension, available on all ARM64 processors including Apple Silicon (M1/M2/M3) and AWS Graviton. This document explains how NEON differs from AVX2 and how we implement it for FlashJSON.

## NEON vs AVX2: Key Differences

| Feature | AVX2 (x86-64) | NEON (ARM64) |
|---------|---------------|--------------|
| Register size | 256-bit (YMM) | 128-bit (V) |
| Bytes per op | 32 | 16 |
| Register count | 16 YMM | 32 V registers |
| Endianness | Little | Little (default) |
| Always available? | No (need detection) | Yes (all ARM64) |

### Registers

```
AVX2 YMM registers (256-bit):
YMM0-YMM15: 32 bytes each

NEON V registers (128-bit):
V0-V31: 16 bytes each

NEON can also view registers as:
- Bn  = 1-byte scalar
- Hn  = 2-byte scalar  
- Sn  = 4-byte scalar
- Dn  = 8-byte scalar (also lower 64 bits of Vn)
- Vn  = 16-byte vector
```

## NEON for JSON Parsing

Since NEON processes 16 bytes at a time (vs AVX2's 32), we need 4 iterations to cover 64 bytes. But ARM64 has 32 registers, so we can process multiple chunks simultaneously!

### The Strategy

```
64 bytes = 4 × 16-byte chunks

Load all 4 chunks into separate registers:
  V0 = bytes[ 0:16]
  V1 = bytes[16:32]
  V2 = bytes[32:48]
  V3 = bytes[48:64]

Compare all in parallel:
  V4  = (V0 == '"')
  V5  = (V1 == '"')  
  V6  = (V2 == '"')
  V7  = (V3 == '"')

Extract and combine masks
```

## Key NEON Instructions

### VDUP: Broadcast a value

```asm
// Broadcast byte 0x22 ('"') to all 16 lanes
MOV W0, #0x22
DUP V16.16B, W0    // V16 = [0x22, 0x22, 0x22, ...] (16 times)
```

### CMEQ: Compare Equal

```asm
// Compare each byte: result is 0xFF if equal, 0x00 if not
CMEQ V4.16B, V0.16B, V16.16B    // V4[i] = (V0[i] == V16[i]) ? 0xFF : 0x00
```

### ORR: Bitwise OR

```asm
// Combine comparison results
ORR V4.16B, V4.16B, V5.16B    // V4 = V4 | V5
```

### Extracting the Bitmask (The Tricky Part)

NEON doesn't have a direct equivalent to x86's `VPMOVMSKB`. We need to build the mask manually:

```asm
// Step 1: Shrink 128 bits to 16 bits using shifts and ORs
// Each byte is either 0x00 or 0xFF
// We want to extract bit 7 of each byte

// Use USHR to shift bits, then narrow
USHR V5.16B, V4.16B, #7       // Shift right by 7: 0xFF -> 0x01, 0x00 -> 0x00

// Now pack 16 bytes into 16 bits using ADDV or bit manipulation
// This is complex - see implementation for details
```

### Alternative: Use Bit Fields

ARM64 has clever bit manipulation. We can use `SHRN` (shift right narrow) and `UMOV` to extract:

```asm
// After comparison, V4 has 0xFF or 0x00 in each byte
// Convert to packed bits in a general-purpose register

// Narrow each byte's MSB into 2 bytes
SHRN  V5.8B, V4.8H, #4    // Narrow even lanes
SHRN2 V5.16B, V4.8H, #4   // Narrow odd lanes

// Eventually get a 16-bit mask
UMOV  W0, V5.H[0]         // Move to general register
```

## Full Algorithm for 64 Bytes

```
Input: 64 bytes at address X0
Output: 64-bit mask in X0

1. Load 4 chunks:
   LDP Q0, Q1, [X0]        // Q0=V0, Q1=V1
   LDP Q2, Q3, [X0, #32]

2. Broadcast structural chars:
   DUP V16.16B, #0x22      // '"'
   DUP V17.16B, #0x7B      // '{'
   ...

3. Compare each chunk against each structural char:
   CMEQ V4.16B, V0.16B, V16.16B    // V0 == '"'
   CMEQ V5.16B, V0.16B, V17.16B    // V0 == '{'
   ORR  V4.16B, V4.16B, V5.16B     // Combine
   ...

4. Extract 16-bit mask from each chunk
5. Combine into 64-bit result:
   mask = (chunk3_mask << 48) | (chunk2_mask << 32) | (chunk1_mask << 16) | chunk0_mask
```

## Performance Considerations

### Why NEON is Competitive Despite Smaller Registers

1. **More registers** (32 vs 16): Can hold more data in flight
2. **Lower latency**: ARM cores often have shorter pipelines
3. **Efficient loads**: LDP loads 32 bytes in one instruction
4. **Memory bandwidth**: Modern M-series chips have excellent bandwidth

### Throughput

| Platform | Bytes/Cycle | Notes |
|----------|-------------|-------|
| AVX2 (Zen 3) | ~32 | 256-bit, 1 cycle |
| AVX2 (Intel) | ~32 | 256-bit, 1 cycle |
| NEON (M1/M2) | ~16-32 | 128-bit but fast | 
| NEON (Graviton2) | ~16 | 128-bit |

Apple Silicon often compensates with higher clock efficiency.

## Example: Finding Quotes in NEON

```asm
// func findQuotesNEON(input []byte) uint64
// Input: X0 = pointer to 64 bytes
// Output: X0 = 64-bit mask

findQuotesNEON:
    // Load 64 bytes in 4 chunks
    LDP Q0, Q1, [X0]         // V0=bytes[0:16], V1=bytes[16:32]
    LDP Q2, Q3, [X0, #32]    // V2=bytes[32:48], V3=bytes[48:64]
    
    // Broadcast '"' (0x22)
    MOVI V16.16B, #0x22
    
    // Compare each chunk
    CMEQ V0.16B, V0.16B, V16.16B
    CMEQ V1.16B, V1.16B, V16.16B
    CMEQ V2.16B, V2.16B, V16.16B
    CMEQ V3.16B, V3.16B, V16.16B
    
    // Extract masks (simplified - actual impl is more complex)
    // ... mask extraction code ...
    
    RET
```

## Why ARM64 Mask Extraction is Complex

x86 has `PMOVMSKB` which directly extracts the MSB of each byte into a register. ARM lacks this.

Options:
1. **Serial approach**: Check each byte individually (slow)
2. **Horizontal operations**: Use ADDV variants (medium)
3. **Bit manipulation**: Clever shifts and combines (fast but complex)

We use option 3 in FlashJSON for best performance.

## Integration with FlashJSON

The NEON implementation follows the same pattern as AVX2:

```go
//go:build arm64

func findStructuralNEON(input []byte) uint64  // Assembly
func findQuotesNEON(input []byte) uint64      // Assembly
```

At runtime:
- ARM64 always uses NEON (no detection needed)
- Automatically selected via build tags

## Summary

| Aspect | Challenge | Solution |
|--------|-----------|----------|
| Smaller registers | 16 bytes vs 32 | Process 4 chunks in parallel |
| No PMOVMSKB | Need to build mask | Bit manipulation tricks |
| Different syntax | AT&T vs ARM | Use Go's Plan 9 assembly |

NEON may require more instructions per 64-byte block, but ARM's efficient execution units and Apple Silicon's performance make it competitive with AVX2.

---

## Next Steps

See the implementation in `indexer_arm64.s` for the full NEON code.
