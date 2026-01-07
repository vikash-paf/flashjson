//go:build !amd64 && !arm64

package simd

// findStructuralAVX2 is a stub for non-amd64 platforms.
// On non-x86 platforms, we use the generic fallback.
func findStructuralAVX2(input []byte) uint64 {
	// This should never be called on non-amd64 platforms
	// because UsesSIMD will be false
	var mask uint64
	for i := 0; i < len(input) && i < 64; i++ {
		if isStructural(input[i]) {
			mask |= 1 << uint(i)
		}
	}
	return mask
}

// findQuotesAVX2 is a stub for non-amd64 platforms.
func findQuotesAVX2(input []byte) uint64 {
	var mask uint64
	for i := 0; i < len(input) && i < 64; i++ {
		if input[i] == '"' {
			mask |= 1 << uint(i)
		}
	}
	return mask
}

// findBackslashAVX2 is a stub for non-amd64 platforms.
func findBackslashAVX2(input []byte) uint64 {
	var mask uint64
	for i := 0; i < len(input) && i < 64; i++ {
		if input[i] == '\\' {
			mask |= 1 << uint(i)
		}
	}
	return mask
}
