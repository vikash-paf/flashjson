//go:build amd64

package simd

// findStructuralAVX2 finds structural characters in a 64-byte chunk.
// Returns a bitmask where bit i is set if input[i] is a structural character.
// Structural characters are: { } [ ] : , "
//
// This function uses AVX2 SIMD instructions to process 32 bytes at a time,
// running twice to cover the full 64-byte chunk.
//
//go:noescape
func findStructuralAVX2(input []byte) uint64

// findQuotesAVX2 finds all quote characters in a 64-byte chunk.
// Returns a bitmask where bit i is set if input[i] is '"'.
//
//go:noescape
func findQuotesAVX2(input []byte) uint64

// findBackslashAVX2 finds all backslash characters in a 64-byte chunk.
// Returns a bitmask where bit i is set if input[i] is '\'.
//
//go:noescape
func findBackslashAVX2(input []byte) uint64
