//go:build ignore

package main

import (
	. "github.com/mmcloughlin/avo/build"
	"github.com/mmcloughlin/avo/gotypes"
	. "github.com/mmcloughlin/avo/operand"
	"github.com/mmcloughlin/avo/reg"
)

func main() {
	TEXT("Search", NOSPLIT, "func(haystack, needle []byte) bool")
	Doc("Search checks if haystack contains needle.")

	needlePtr := Load(Param("needle").Base(), GP64())
	needleLen:= Load(Param("needle").Len(), GP64())

	startPtr := Load(Param("haystack").Base(), GP64())
	endPtr := Load(Param("haystack").Len(), GP64())
	maxPtr := GP64(); MOVQ(endPtr, maxPtr); SUBQ(needleLen, maxPtr)
	ptr := GP64(); MOVQ(startPtr, ptr)

	// TODO: We might want to find the rare bytes instead. See https://github.com/BurntSushi/memchr/blob/master/src/memmem/rarebytes.rs#L47
	Comment("create vector filled with first and last character")
	first := YMM()
	last := YMM()
	VPBROADCASTB(NewParamAddr("needle", 0), first)
	k := GP64()
	MOVQ(NewParamAddr("needle", 0), k)
	ADDQ(needleLen, k)
	DECQ(k)
	VPBROADCASTB(Mem{Base: k}, last)

	Label("chunk_loop")

	// while ptr <= max_ptr
	CMPQ(ptr, maxPtr)
	JG(LabelRef("chunk_loop_end"))

	inline_find_in_chunk(first, last, ptr, needlePtr)

	// ptr += 32 // size of YMM == 256bit
	ADDQ(Imm(32), ptr)
	JMP(LabelRef("chunk_loop"))

	Label("chunk_loop_end")

	// TODO: move into loop
	inline_memcmp(Param("haystack"), needlePtr, needleLen)

	Generate()
}

func inline_find_in_chunk(first, last reg.VecVirtual, ptr, needlePtr reg.Register) {
	block_first := YMM()
	block_last := YMM()

	Comment("compare blocks against first and last character")
	VMOVDQU(Mem{Base: needlePtr}, block_first)
	VMOVDQU(Mem{Base: needlePtr}, block_last)

	eq_first := YMM()
	eq_last := YMM()

	VPCMPEQB(first, block_first, eq_first)
	VPCMPEQB(last, block_last, eq_last)

	Comment("create mask and determine position")
	mask := YMM()
	VPAND(eq_first, eq_last, mask)
	match_offset := GP32()
	VPMOVMSKB(mask, match_offset)

	Label("mask_loop")
	CMPL(match_offset, Imm(0))
	JE(LabelRef("mask_loop_done"))

	pos := GP32()

 	// while match_offset != 0	
	TZCNTL(match_offset, pos)

	// TODO: get chunk and compare 
	//inline_memcmp(Param("haystack"), needlePtr, needleLen)

	JMP(LabelRef("mask_loop"))

	Label("mask_loop_done")
}

func inline_memcmp(y gotypes.Component, xPtr, xLen reg.Register) {
	Comment("compare two slices")

	yPtr := Load(y.Base(), GP64())
	yLen:= Load(y.Len(), GP64())

	ret, _ := ReturnIndex(0).Resolve()

	// TODO: compare more than one byte at a time.
	r := GP8()

	Label("memcmp_loop")

	CMPQ(xLen, Imm(0))
	JE(LabelRef("done"))
	CMPQ(yLen, Imm(0))
	JE(LabelRef("done"))

	MOVB(Mem{Base: yPtr}, r)
	CMPB(Mem{Base: xPtr}, r)
	JNE(LabelRef("not_equal"))

	ADDQ(Imm(1), xPtr)
	DECQ(xLen)
	ADDQ(Imm(1), yPtr)
	DECQ(yLen)
	JMP(LabelRef("memcmp_loop"))

	Label("done")
	Comment("check if both are done")
	CMPQ(xLen, yLen)	
	JE(LabelRef("equal"))

	Label("not_equal")
	MOVB(U8(0), ret.Addr)
	RET()

	Label("equal")
	MOVB(U8(1), ret.Addr)
	RET()
}
