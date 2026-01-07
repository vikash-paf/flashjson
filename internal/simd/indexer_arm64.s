//go:build arm64

#include "textflag.h"

// NEON structural character detection for JSON parsing
// 
// ARM64 NEON uses 128-bit V registers (16 bytes)
// We process 64 bytes using 4 NEON operations
//
// Structural chars: { } [ ] : , "
// Hex values:       7B 7D 5B 5D 3A 2C 22

// func findStructuralNEON(input []byte) uint64
// Finds structural characters in 64 bytes using NEON.
// Returns bitmask where bit i = 1 if input[i] is structural.
TEXT ·findStructuralNEON(SB), NOSPLIT, $0-32
    // R0 = pointer to input (from slice header)
    MOVD input_base+0(FP), R0
    
    // Load 64 bytes in 4 chunks of 16 bytes
    VLD1 (R0), [V0.B16, V1.B16, V2.B16, V3.B16]
    
    // Create constant vectors for each structural character
    // Using VMOV to create vectors of repeated bytes
    
    // V16 = '"' (0x22)
    MOVD $0x2222222222222222, R1
    VMOV R1, V16.D[0]
    VMOV R1, V16.D[1]
    
    // V17 = '{' (0x7B)
    MOVD $0x7B7B7B7B7B7B7B7B, R1
    VMOV R1, V17.D[0]
    VMOV R1, V17.D[1]
    
    // V18 = '}' (0x7D)
    MOVD $0x7D7D7D7D7D7D7D7D, R1
    VMOV R1, V18.D[0]
    VMOV R1, V18.D[1]
    
    // V19 = '[' (0x5B)
    MOVD $0x5B5B5B5B5B5B5B5B, R1
    VMOV R1, V19.D[0]
    VMOV R1, V19.D[1]
    
    // V20 = ']' (0x5D)
    MOVD $0x5D5D5D5D5D5D5D5D, R1
    VMOV R1, V20.D[0]
    VMOV R1, V20.D[1]
    
    // V21 = ':' (0x3A)
    MOVD $0x3A3A3A3A3A3A3A3A, R1
    VMOV R1, V21.D[0]
    VMOV R1, V21.D[1]
    
    // V22 = ',' (0x2C)
    MOVD $0x2C2C2C2C2C2C2C2C, R1
    VMOV R1, V22.D[0]
    VMOV R1, V22.D[1]

    // Process chunk 0 (bytes 0-15)
    VCMEQ V0.B16, V16.B16, V24.B16   // V24 = (V0 == '"')
    VCMEQ V0.B16, V17.B16, V25.B16   // V25 = (V0 == '{')
    VORR  V24.B16, V25.B16, V24.B16  // V24 |= V25
    VCMEQ V0.B16, V18.B16, V25.B16   // V25 = (V0 == '}')
    VORR  V24.B16, V25.B16, V24.B16
    VCMEQ V0.B16, V19.B16, V25.B16   // V25 = (V0 == '[')
    VORR  V24.B16, V25.B16, V24.B16
    VCMEQ V0.B16, V20.B16, V25.B16   // V25 = (V0 == ']')
    VORR  V24.B16, V25.B16, V24.B16
    VCMEQ V0.B16, V21.B16, V25.B16   // V25 = (V0 == ':')
    VORR  V24.B16, V25.B16, V24.B16
    VCMEQ V0.B16, V22.B16, V25.B16   // V25 = (V0 == ',')
    VORR  V24.B16, V25.B16, V24.B16  // V24 = result for chunk 0

    // Process chunk 1 (bytes 16-31)
    VCMEQ V1.B16, V16.B16, V26.B16
    VCMEQ V1.B16, V17.B16, V25.B16
    VORR  V26.B16, V25.B16, V26.B16
    VCMEQ V1.B16, V18.B16, V25.B16
    VORR  V26.B16, V25.B16, V26.B16
    VCMEQ V1.B16, V19.B16, V25.B16
    VORR  V26.B16, V25.B16, V26.B16
    VCMEQ V1.B16, V20.B16, V25.B16
    VORR  V26.B16, V25.B16, V26.B16
    VCMEQ V1.B16, V21.B16, V25.B16
    VORR  V26.B16, V25.B16, V26.B16
    VCMEQ V1.B16, V22.B16, V25.B16
    VORR  V26.B16, V25.B16, V26.B16  // V26 = result for chunk 1

    // Process chunk 2 (bytes 32-47)
    VCMEQ V2.B16, V16.B16, V27.B16
    VCMEQ V2.B16, V17.B16, V25.B16
    VORR  V27.B16, V25.B16, V27.B16
    VCMEQ V2.B16, V18.B16, V25.B16
    VORR  V27.B16, V25.B16, V27.B16
    VCMEQ V2.B16, V19.B16, V25.B16
    VORR  V27.B16, V25.B16, V27.B16
    VCMEQ V2.B16, V20.B16, V25.B16
    VORR  V27.B16, V25.B16, V27.B16
    VCMEQ V2.B16, V21.B16, V25.B16
    VORR  V27.B16, V25.B16, V27.B16
    VCMEQ V2.B16, V22.B16, V25.B16
    VORR  V27.B16, V25.B16, V27.B16  // V27 = result for chunk 2

    // Process chunk 3 (bytes 48-63)
    VCMEQ V3.B16, V16.B16, V28.B16
    VCMEQ V3.B16, V17.B16, V25.B16
    VORR  V28.B16, V25.B16, V28.B16
    VCMEQ V3.B16, V18.B16, V25.B16
    VORR  V28.B16, V25.B16, V28.B16
    VCMEQ V3.B16, V19.B16, V25.B16
    VORR  V28.B16, V25.B16, V28.B16
    VCMEQ V3.B16, V20.B16, V25.B16
    VORR  V28.B16, V25.B16, V28.B16
    VCMEQ V3.B16, V21.B16, V25.B16
    VORR  V28.B16, V25.B16, V28.B16
    VCMEQ V3.B16, V22.B16, V25.B16
    VORR  V28.B16, V25.B16, V28.B16  // V28 = result for chunk 3

    // Now extract bitmasks from V24, V26, V27, V28
    // Each Vn.B16 has 0xFF or 0x00 in each byte
    // We need to convert to a 16-bit mask per chunk
    
    // Use USHR to get MSB, then pack
    VUSHR $7, V24.B16, V24.B16   // Shift right by 7: 0xFF -> 0x01
    VUSHR $7, V26.B16, V26.B16
    VUSHR $7, V27.B16, V27.B16
    VUSHR $7, V28.B16, V28.B16
    
    // Pack bytes to bits using multiplication trick
    // Multiply by powers of 2 and add horizontally
    
    // For simplicity, extract each byte and build mask manually
    // This is slower but correct - optimize later
    
    // Extract chunk 0 mask (16 bits)
    VMOV V24.B[0], R2
    VMOV V24.B[1], R3
    ORR R3<<1, R2, R2
    VMOV V24.B[2], R3
    ORR R3<<2, R2, R2
    VMOV V24.B[3], R3
    ORR R3<<3, R2, R2
    VMOV V24.B[4], R3
    ORR R3<<4, R2, R2
    VMOV V24.B[5], R3
    ORR R3<<5, R2, R2
    VMOV V24.B[6], R3
    ORR R3<<6, R2, R2
    VMOV V24.B[7], R3
    ORR R3<<7, R2, R2
    VMOV V24.B[8], R3
    ORR R3<<8, R2, R2
    VMOV V24.B[9], R3
    ORR R3<<9, R2, R2
    VMOV V24.B[10], R3
    ORR R3<<10, R2, R2
    VMOV V24.B[11], R3
    ORR R3<<11, R2, R2
    VMOV V24.B[12], R3
    ORR R3<<12, R2, R2
    VMOV V24.B[13], R3
    ORR R3<<13, R2, R2
    VMOV V24.B[14], R3
    ORR R3<<14, R2, R2
    VMOV V24.B[15], R3
    ORR R3<<15, R2, R2
    // R2 = mask for bytes 0-15
    
    // Extract chunk 1 mask
    VMOV V26.B[0], R4
    VMOV V26.B[1], R3
    ORR R3<<1, R4, R4
    VMOV V26.B[2], R3
    ORR R3<<2, R4, R4
    VMOV V26.B[3], R3
    ORR R3<<3, R4, R4
    VMOV V26.B[4], R3
    ORR R3<<4, R4, R4
    VMOV V26.B[5], R3
    ORR R3<<5, R4, R4
    VMOV V26.B[6], R3
    ORR R3<<6, R4, R4
    VMOV V26.B[7], R3
    ORR R3<<7, R4, R4
    VMOV V26.B[8], R3
    ORR R3<<8, R4, R4
    VMOV V26.B[9], R3
    ORR R3<<9, R4, R4
    VMOV V26.B[10], R3
    ORR R3<<10, R4, R4
    VMOV V26.B[11], R3
    ORR R3<<11, R4, R4
    VMOV V26.B[12], R3
    ORR R3<<12, R4, R4
    VMOV V26.B[13], R3
    ORR R3<<13, R4, R4
    VMOV V26.B[14], R3
    ORR R3<<14, R4, R4
    VMOV V26.B[15], R3
    ORR R3<<15, R4, R4
    // R4 = mask for bytes 16-31
    
    // Extract chunk 2 mask
    VMOV V27.B[0], R5
    VMOV V27.B[1], R3
    ORR R3<<1, R5, R5
    VMOV V27.B[2], R3
    ORR R3<<2, R5, R5
    VMOV V27.B[3], R3
    ORR R3<<3, R5, R5
    VMOV V27.B[4], R3
    ORR R3<<4, R5, R5
    VMOV V27.B[5], R3
    ORR R3<<5, R5, R5
    VMOV V27.B[6], R3
    ORR R3<<6, R5, R5
    VMOV V27.B[7], R3
    ORR R3<<7, R5, R5
    VMOV V27.B[8], R3
    ORR R3<<8, R5, R5
    VMOV V27.B[9], R3
    ORR R3<<9, R5, R5
    VMOV V27.B[10], R3
    ORR R3<<10, R5, R5
    VMOV V27.B[11], R3
    ORR R3<<11, R5, R5
    VMOV V27.B[12], R3
    ORR R3<<12, R5, R5
    VMOV V27.B[13], R3
    ORR R3<<13, R5, R5
    VMOV V27.B[14], R3
    ORR R3<<14, R5, R5
    VMOV V27.B[15], R3
    ORR R3<<15, R5, R5
    // R5 = mask for bytes 32-47
    
    // Extract chunk 3 mask
    VMOV V28.B[0], R6
    VMOV V28.B[1], R3
    ORR R3<<1, R6, R6
    VMOV V28.B[2], R3
    ORR R3<<2, R6, R6
    VMOV V28.B[3], R3
    ORR R3<<3, R6, R6
    VMOV V28.B[4], R3
    ORR R3<<4, R6, R6
    VMOV V28.B[5], R3
    ORR R3<<5, R6, R6
    VMOV V28.B[6], R3
    ORR R3<<6, R6, R6
    VMOV V28.B[7], R3
    ORR R3<<7, R6, R6
    VMOV V28.B[8], R3
    ORR R3<<8, R6, R6
    VMOV V28.B[9], R3
    ORR R3<<9, R6, R6
    VMOV V28.B[10], R3
    ORR R3<<10, R6, R6
    VMOV V28.B[11], R3
    ORR R3<<11, R6, R6
    VMOV V28.B[12], R3
    ORR R3<<12, R6, R6
    VMOV V28.B[13], R3
    ORR R3<<13, R6, R6
    VMOV V28.B[14], R3
    ORR R3<<14, R6, R6
    VMOV V28.B[15], R3
    ORR R3<<15, R6, R6
    // R6 = mask for bytes 48-63
    
    // Combine all masks
    // Result = R2 | (R4 << 16) | (R5 << 32) | (R6 << 48)
    ORR R4<<16, R2, R0
    ORR R5<<32, R0, R0
    ORR R6<<48, R0, R0
    
    MOVD R0, ret+24(FP)
    RET

// func findQuotesNEON(input []byte) uint64
TEXT ·findQuotesNEON(SB), NOSPLIT, $0-32
    MOVD input_base+0(FP), R0
    
    // Load 64 bytes
    VLD1 (R0), [V0.B16, V1.B16, V2.B16, V3.B16]
    
    // Create quote constant
    MOVD $0x2222222222222222, R1
    VMOV R1, V16.D[0]
    VMOV R1, V16.D[1]
    
    // Compare each chunk
    VCMEQ V0.B16, V16.B16, V0.B16
    VCMEQ V1.B16, V16.B16, V1.B16
    VCMEQ V2.B16, V16.B16, V2.B16
    VCMEQ V3.B16, V16.B16, V3.B16
    
    // Shift to get 0/1 values
    VUSHR $7, V0.B16, V0.B16
    VUSHR $7, V1.B16, V1.B16
    VUSHR $7, V2.B16, V2.B16
    VUSHR $7, V3.B16, V3.B16
    
    // Extract masks (simplified - use lookup table for production)
    // For now, return 0 - implement full extraction later
    MOVD $0, R0
    MOVD R0, ret+24(FP)
    RET

// func findBackslashNEON(input []byte) uint64
TEXT ·findBackslashNEON(SB), NOSPLIT, $0-32
    MOVD input_base+0(FP), R0
    
    // Load 64 bytes
    VLD1 (R0), [V0.B16, V1.B16, V2.B16, V3.B16]
    
    // Create backslash constant (0x5C)
    MOVD $0x5C5C5C5C5C5C5C5C, R1
    VMOV R1, V16.D[0]
    VMOV R1, V16.D[1]
    
    // Compare each chunk
    VCMEQ V0.B16, V16.B16, V0.B16
    VCMEQ V1.B16, V16.B16, V1.B16
    VCMEQ V2.B16, V16.B16, V2.B16
    VCMEQ V3.B16, V16.B16, V3.B16
    
    // Shift to get 0/1 values
    VUSHR $7, V0.B16, V0.B16
    VUSHR $7, V1.B16, V1.B16
    VUSHR $7, V2.B16, V2.B16
    VUSHR $7, V3.B16, V3.B16
    
    // Extract masks (simplified)
    MOVD $0, R0
    MOVD R0, ret+24(FP)
    RET
