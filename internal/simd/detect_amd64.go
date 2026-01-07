//go:build amd64

package simd

// detectAVX2 checks for AVX2 support on x86-64.
func detectAVX2() bool {
	// Call CPUID to check for AVX2
	// EAX=7, ECX=0: Extended Features
	// EBX bit 5 = AVX2
	eax, ebx, _, _ := cpuid(7, 0)
	_ = eax

	// Check AVX2 bit (bit 5 of EBX)
	hasAVX2 := (ebx & (1 << 5)) != 0

	// Also need to check that OS has enabled AVX state saving
	// via XGETBV with ECX=0, check bits 1 and 2
	if hasAVX2 {
		_, _, ecx, _ := cpuid(1, 0)
		osUsesXSAVE := (ecx & (1 << 27)) != 0

		if osUsesXSAVE {
			xcr0 := xgetbv(0)
			// Check XMM (bit 1) and YMM (bit 2) state are enabled
			hasAVX2 = (xcr0 & 0x6) == 0x6
		} else {
			hasAVX2 = false
		}
	}

	return hasAVX2
}

// cpuid executes the CPUID instruction.
// Implemented in assembly.
//
//go:noescape
func cpuid(eaxIn, ecxIn uint32) (eax, ebx, ecx, edx uint32)

// xgetbv executes XGETBV instruction to read extended control register.
// Implemented in assembly.
//
//go:noescape
func xgetbv(cxIn uint32) uint64
