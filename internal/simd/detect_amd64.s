//go:build amd64

#include "textflag.h"

// func cpuid(eaxIn, ecxIn uint32) (eax, ebx, ecx, edx uint32)
TEXT ·cpuid(SB), NOSPLIT, $0-24
	MOVL eaxIn+0(FP), AX
	MOVL ecxIn+4(FP), CX
	CPUID
	MOVL AX, eax+8(FP)
	MOVL BX, ebx+12(FP)
	MOVL CX, ecx+16(FP)
	MOVL DX, edx+20(FP)
	RET

// func xgetbv(cxIn uint32) uint64
TEXT ·xgetbv(SB), NOSPLIT, $0-16
	MOVL cxIn+0(FP), CX
	XGETBV
	MOVL AX, ret+8(FP)
	MOVL DX, ret+12(FP)
	RET
