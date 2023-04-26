//go:build ignore

package main

import (
	. "github.com/mmcloughlin/avo/build"
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

	inline_find_in_chunk(first, last, ptr, needlePtr, needleLen)

	// ptr += 32 // size of YMM == 256bit
	ADDQ(Imm(32), ptr)
	JMP(LabelRef("chunk_loop"))

	Label("chunk_loop_end")
	ret, _ := ReturnIndex(0).Resolve()
	MOVB(U8(0), ret.Addr)
	RET()

	Generate()
}

func inline_find_in_chunk(first, last reg.VecVirtual, ptr, needlePtr, needleLen reg.Register) {
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

 	// while match_offset != 0	
	Label("mask_loop")
	CMPL(match_offset, Imm(0))
	JE(LabelRef("mask_loop_done"))

	pos := GP32()
	TZCNTL(match_offset, pos)
	// Reset chunkPtr for each position in match offset
	chunkPtr := GP64(); MOVQ(ptr, chunkPtr)
	ADDQ(pos.As64(), chunkPtr)

	inline_memcmp(chunkPtr, needlePtr, needleLen)

	// update match offset: match_offset = match_offset & (match_offset -1)
	match_offset_b := GP32(); MOVL(match_offset, match_offset_b)
	DECL(match_offset_b)
	ANDL(match_offset_b, match_offset)


	JMP(LabelRef("mask_loop"))

	Label("mask_loop_done")
}

func inline_memcmp(xPtr, yPtr, size reg.Register) {
	Comment("compare two slices")

	i := GP64(); MOVQ(size, i)
	x := GP64(); MOVQ(xPtr, x)
	y := GP64(); MOVQ(yPtr, y)
	// TODO: compare more than one byte at a time.
	r := GP8()

	ret, _ := ReturnIndex(0).Resolve()

	Label("memcmp_loop")

	Comment("the loop is done; the chunks must be equal")
	CMPQ(i, Imm(0))
	JE(LabelRef("memcmp_equal"))

	MOVB(Mem{Base: y}, r)
	CMPB(Mem{Base: x}, r)
	JNE(LabelRef("memcmp_not_equal"))

	ADDQ(Imm(1), x)
	ADDQ(Imm(1), y)
	DECQ(i)
	JMP(LabelRef("memcmp_loop"))

	Label("memcmp_equal")
	MOVB(U8(1), ret.Addr)
	RET()

	// do not return anything
	Label("memcmp_not_equal")
}
