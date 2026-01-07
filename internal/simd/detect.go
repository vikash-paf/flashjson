// Package simd provides SIMD-accelerated JSON indexing.
// It uses AVX2 on x86-64 and NEON on ARM64 for maximum performance.
package simd

import (
	"runtime"
)

// CPU feature flags, detected at init time.
var (
	// HasAVX2 indicates AVX2 support (x86-64)
	HasAVX2 bool

	// HasAVX512 indicates AVX-512 support (x86-64, newer CPUs)
	HasAVX512 bool

	// HasNEON indicates NEON support (ARM64, always true on arm64)
	HasNEON bool

	// UsesSIMD indicates whether SIMD will be used for indexing
	UsesSIMD bool
)

func init() {
	detectCPUFeatures()
}

func detectCPUFeatures() {
	switch runtime.GOARCH {
	case "amd64":
		detectX86Features()
	case "arm64":
		// ARM64 always has NEON
		HasNEON = true
		UsesSIMD = true
	default:
		// No SIMD for other architectures
		UsesSIMD = false
	}
}

// detectX86Features detects x86-64 CPU features.
func detectX86Features() {
	HasAVX2 = detectAVX2()
	HasAVX512 = false // Conservative - requires more detection
	UsesSIMD = HasAVX2
}

// detectAVX2 is implemented in platform-specific files:
// - detect_amd64.go for x86-64 (uses CPUID)
// - detect_generic.go for other platforms (returns false)
