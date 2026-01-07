//go:build arm64

package simd

// detectAVX2 returns false on ARM64 - we use NEON instead.
func detectAVX2() bool {
	return false
}
