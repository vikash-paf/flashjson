//go:build arm64

package simd

// ARM64 NEON functions for structural character detection.
// NEON is always available on ARM64, no detection needed.

// findStructuralNEON finds structural characters in a 64-byte chunk.
// Returns a bitmask where bit i is set if input[i] is a structural character.
// Structural characters are: { } [ ] : , "
//
// NEON processes 16 bytes at a time, so we process 4 chunks and combine.
//
//go:noescape
func findStructuralNEON(input []byte) uint64

// findQuotesNEON finds all quote characters in a 64-byte chunk.
//
//go:noescape
func findQuotesNEON(input []byte) uint64

// On ARM64, we use NEON instead of AVX2
// These functions redirect to the NEON implementations

func findStructuralAVX2(input []byte) uint64 {
	return findStructuralNEON(input)
}

func findQuotesAVX2(input []byte) uint64 {
	return findQuotesNEON(input)
}

func findBackslashAVX2(input []byte) uint64 {
	return findBackslashNEON(input)
}

//go:noescape
func findBackslashNEON(input []byte) uint64
