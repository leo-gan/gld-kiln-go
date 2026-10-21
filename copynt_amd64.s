#include "textflag.h"

// func avx2Enabled() bool
TEXT ·avx2Enabled(SB), NOSPLIT, $0-1
	MOVQ $1, AX
	XORQ CX, CX
	CPUID
	TESTL $(1<<27), CX
	JZ noavx
	TESTL $(1<<28), CX
	JZ noavx
	XORQ CX, CX
	XGETBV
	TESTL $6, AX
	JZ noavx
	MOVQ $7, AX
	XORQ CX, CX
	CPUID
	TESTL $(1<<5), BX
	JZ noavx
	MOVB $1, ret+0(FP)
	RET
noavx:
	MOVB $0, ret+0(FP)
	RET

// func copyNTSSE(dst, src unsafe.Pointer, n uintptr)
// dst must be 16-byte aligned. Streaming stores skip the write-allocate read.
TEXT ·copyNTSSE(SB), NOSPLIT, $0-24
	MOVQ dst+0(FP), DI
	MOVQ src+8(FP), SI
	MOVQ n+16(FP), BX

	MOVQ BX, CX
	SHRQ $6, CX
	TESTQ CX, CX
	JZ tail

loop64:
	MOVUPS (SI), X0
	MOVUPS 16(SI), X1
	MOVUPS 32(SI), X2
	MOVUPS 48(SI), X3
	MOVNTPS X0, (DI)
	MOVNTPS X1, 16(DI)
	MOVNTPS X2, 32(DI)
	MOVNTPS X3, 48(DI)
	ADDQ $64, SI
	ADDQ $64, DI
	DECQ CX
	JNZ loop64

tail:
	ANDQ $63, BX
	TESTQ BX, BX
	JZ fence
	MOVQ BX, CX
	REP
	MOVSB

fence:
	SFENCE
	RET

// func copyNTAVX2(dst, src unsafe.Pointer, n uintptr)
// dst must be 32-byte aligned.
TEXT ·copyNTAVX2(SB), NOSPLIT, $0-24
	MOVQ dst+0(FP), DI
	MOVQ src+8(FP), SI
	MOVQ n+16(FP), BX

	MOVQ BX, CX
	SHRQ $7, CX
	TESTQ CX, CX
	JZ avxTail

avxLoop:
	VMOVUPS (SI), Y0
	VMOVUPS 32(SI), Y1
	VMOVUPS 64(SI), Y2
	VMOVUPS 96(SI), Y3
	VMOVNTPS Y0, (DI)
	VMOVNTPS Y1, 32(DI)
	VMOVNTPS Y2, 64(DI)
	VMOVNTPS Y3, 96(DI)
	ADDQ $128, SI
	ADDQ $128, DI
	DECQ CX
	JNZ avxLoop
	VZEROUPPER

avxTail:
	ANDQ $127, BX
	TESTQ BX, BX
	JZ avxFence
	MOVQ BX, CX
	REP
	MOVSB

avxFence:
	SFENCE
	RET
