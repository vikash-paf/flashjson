//go:build amd64

#include "textflag.h"

// Structural character constants for comparison
// { = 0x7B, } = 0x7D, [ = 0x5B, ] = 0x5D, : = 0x3A, , = 0x2C, " = 0x22

// func findStructuralAVX2(input []byte) uint64
// Finds structural characters in 64 bytes using AVX2.
// Returns bitmask where bit i = 1 if input[i] is structural.
TEXT ·findStructuralAVX2(SB), NOSPLIT, $0-32
    MOVQ input_base+0(FP), SI      // SI = pointer to input
    
    // Process first 32 bytes
    VMOVDQU (SI), Y0              // Y0 = input[0:32]
    
    // Create comparison vectors for each structural character
    // We'll compare against each and OR the results
    
    // Compare with '"' (0x22)
    MOVD $0x22222222, X1
    VPBROADCASTD X1, Y1           // Y1 = all 0x22
    VPCMPEQB Y1, Y0, Y2           // Y2 = (input == '"')
    
    // Compare with '{' (0x7B) 
    MOVD $0x7B7B7B7B, X1
    VPBROADCASTD X1, Y1
    VPCMPEQB Y1, Y0, Y3           // Y3 = (input == '{')
    VPOR Y2, Y3, Y2               // Y2 = Y2 | Y3
    
    // Compare with '}' (0x7D)
    MOVD $0x7D7D7D7D, X1
    VPBROADCASTD X1, Y1
    VPCMPEQB Y1, Y0, Y3           // Y3 = (input == '}')
    VPOR Y2, Y3, Y2
    
    // Compare with '[' (0x5B)
    MOVD $0x5B5B5B5B, X1
    VPBROADCASTD X1, Y1
    VPCMPEQB Y1, Y0, Y3           // Y3 = (input == '[')
    VPOR Y2, Y3, Y2
    
    // Compare with ']' (0x5D)
    MOVD $0x5D5D5D5D, X1
    VPBROADCASTD X1, Y1
    VPCMPEQB Y1, Y0, Y3           // Y3 = (input == ']')
    VPOR Y2, Y3, Y2
    
    // Compare with ':' (0x3A)
    MOVD $0x3A3A3A3A, X1
    VPBROADCASTD X1, Y1
    VPCMPEQB Y1, Y0, Y3           // Y3 = (input == ':')
    VPOR Y2, Y3, Y2
    
    // Compare with ',' (0x2C)
    MOVD $0x2C2C2C2C, X1
    VPBROADCASTD X1, Y1
    VPCMPEQB Y1, Y0, Y3           // Y3 = (input == ',')
    VPOR Y2, Y3, Y2
    
    // Extract 32-bit mask for first 32 bytes
    VPMOVMSKB Y2, AX              // AX = 32-bit mask for bytes 0-31
    MOVL AX, R8                   // R8 = low 32 bits of result
    
    // Process second 32 bytes (input[32:64])
    VMOVDQU 32(SI), Y0            // Y0 = input[32:64]
    
    // Repeat comparisons for second chunk
    MOVD $0x22222222, X1
    VPBROADCASTD X1, Y1
    VPCMPEQB Y1, Y0, Y2
    
    MOVD $0x7B7B7B7B, X1
    VPBROADCASTD X1, Y1
    VPCMPEQB Y1, Y0, Y3
    VPOR Y2, Y3, Y2
    
    MOVD $0x7D7D7D7D, X1
    VPBROADCASTD X1, Y1
    VPCMPEQB Y1, Y0, Y3
    VPOR Y2, Y3, Y2
    
    MOVD $0x5B5B5B5B, X1
    VPBROADCASTD X1, Y1
    VPCMPEQB Y1, Y0, Y3
    VPOR Y2, Y3, Y2
    
    MOVD $0x5D5D5D5D, X1
    VPBROADCASTD X1, Y1
    VPCMPEQB Y1, Y0, Y3
    VPOR Y2, Y3, Y2
    
    MOVD $0x3A3A3A3A, X1
    VPBROADCASTD X1, Y1
    VPCMPEQB Y1, Y0, Y3
    VPOR Y2, Y3, Y2
    
    MOVD $0x2C2C2C2C, X1
    VPBROADCASTD X1, Y1
    VPCMPEQB Y1, Y0, Y3
    VPOR Y2, Y3, Y2
    
    // Extract 32-bit mask for second 32 bytes
    VPMOVMSKB Y2, AX              // AX = 32-bit mask for bytes 32-63
    
    // Combine: result = (high32 << 32) | low32
    SHLQ $32, AX                  // AX = mask << 32
    ORQ R8, AX                    // AX = final 64-bit mask
    
    MOVQ AX, ret+24(FP)           // Store result
    VZEROUPPER                    // Clear upper YMM bits
    RET

// func findQuotesAVX2(input []byte) uint64
TEXT ·findQuotesAVX2(SB), NOSPLIT, $0-32
    MOVQ input_base+0(FP), SI
    
    // Load first 32 bytes
    VMOVDQU (SI), Y0
    
    // Broadcast '"' (0x22) to all bytes
    MOVD $0x22222222, X1
    VPBROADCASTD X1, Y1
    
    // Compare
    VPCMPEQB Y1, Y0, Y2
    VPMOVMSKB Y2, AX
    MOVL AX, R8
    
    // Load second 32 bytes
    VMOVDQU 32(SI), Y0
    VPCMPEQB Y1, Y0, Y2
    VPMOVMSKB Y2, AX
    
    SHLQ $32, AX
    ORQ R8, AX
    
    MOVQ AX, ret+24(FP)
    VZEROUPPER
    RET

// func findBackslashAVX2(input []byte) uint64
TEXT ·findBackslashAVX2(SB), NOSPLIT, $0-32
    MOVQ input_base+0(FP), SI
    
    // Load first 32 bytes
    VMOVDQU (SI), Y0
    
    // Broadcast '\' (0x5C) to all bytes
    MOVD $0x5C5C5C5C, X1
    VPBROADCASTD X1, Y1
    
    // Compare
    VPCMPEQB Y1, Y0, Y2
    VPMOVMSKB Y2, AX
    MOVL AX, R8
    
    // Load second 32 bytes
    VMOVDQU 32(SI), Y0
    VPCMPEQB Y1, Y0, Y2
    VPMOVMSKB Y2, AX
    
    SHLQ $32, AX
    ORQ R8, AX
    
    MOVQ AX, ret+24(FP)
    VZEROUPPER
    RET
