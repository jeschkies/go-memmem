// Code generated by command: go run asm.go -out bytes.s -stubs bytes.go. DO NOT EDIT.

#include "textflag.h"

// func Search(haystack []byte, needle []byte) bool
// Requires: AVX, AVX2, BMI
TEXT ·Search(SB), NOSPLIT, $0-49
	MOVQ needle_base+24(FP), AX
	MOVQ needle_len+32(FP), CX
	MOVQ haystack_base+0(FP), DX
	MOVQ haystack_len+8(FP), BX
	SUBQ CX, BX

	// create vector filled with first and last character
	VPBROADCASTB needle+0(FP), Y0
	MOVQ         needle+0(FP), SI
	ADDQ         CX, SI
	DECQ         SI
	VPBROADCASTB (SI), Y1

chunk_loop:
	CMPQ DX, BX
	JG   chunk_loop_end

	// compare blocks against first and last character
	VMOVDQU  (AX), Y2
	VMOVDQU  (AX), Y3
	VPCMPEQB Y0, Y2, Y2
	VPCMPEQB Y1, Y3, Y3

	// create mask and determine position
	VPAND     Y2, Y3, Y2
	VPMOVMSKB Y2, SI

mask_loop:
	CMPL    SI, $0x00
	JE      mask_loop_done
	TZCNTL  SI, DI
	MOVQ    DX, R8
	MOVLQSX DI, DI
	ADDQ    DI, R8

	// compare two slices
	MOVQ CX, DI
	MOVQ AX, R9

memcmp_loop:
	CMPQ DI, $0x00
	JE   memcmp_loop_done
	MOVB (R9), R10
	CMPB (R8), R10
	JNE  not_equal
	ADDQ $0x01, R8
	ADDQ $0x01, R9
	DECQ DI
	JMP  memcmp_loop

memcmp_loop_done:
	MOVB $0x01, ret+48(FP)
	RET

not_equal:
	JMP mask_loop

mask_loop_done:
	ADDQ $0x20, DX
	JMP  chunk_loop

chunk_loop_end:
	MOVB $0x00, ret+48(FP)
	RET
