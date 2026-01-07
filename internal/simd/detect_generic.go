//go:build !amd64

package simd

// detectAVX2 returns false on non-amd64 platforms.
func detectAVX2() bool {
	return false
}
