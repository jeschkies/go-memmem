//go:build ignore

package main

import (
	. "github.com/mmcloughlin/avo/build"
	. "github.com/mmcloughlin/avo/operand"
	"github.com/mmcloughlin/avo/reg"

	"github.com/jeschkies/go-memmem/pkg/search"
)

// main generates assembly code for AVX2 on AMD64.
func main() {
	TEXT("findInChunk", NOSPLIT, "func(needle []byte, haystack []byte) int64")
	Doc("findInChunk is only generated for testing.")
	hptr := Load(Param("haystack").Base(), GP64())
	needleLen := Load(Param("needle").Len(), GP64()); DECQ(needleLen)
	needle := Load(Param("needle").Base(), GP64())
	f, l := inlineSplat(needle, needleLen)

	offset := inlineFindInChunk("test", f, l, hptr, needle, needleLen)

	Store(offset, ReturnIndex(0))
	VZEROUPPER()
	RET()

	TEXT("indexAvx2", NOSPLIT, "func(haystack, needle []byte) int64")
	Doc("indexAvx2 returns the first position the needle is in the haystack. " +
		"The caller must ensure len(needle) >= 1 and " +
		"len(haystack) >= LOOP_SIZE_AVX2 + len(needle) - 1.")

	needlePtr := Load(Param("needle").Base(), GP64())
	needleLenMain := Load(Param("needle").Len(), GP64()); DECQ(needleLenMain)

	startPtr := Load(Param("haystack").Base(), GP64())
	haystackLen, _ := Param("haystack").Len().Resolve()

	endPtr := GP64(); MOVQ(startPtr, endPtr); ADDQ(haystackLen.Addr, endPtr)

	// maxPtr is the last position where both 32-byte vector loads
	// (at curPtr and at curPtr + len(needle) - 1) stay within the haystack.
	// maxPtr == endPtr - LOOP_SIZE_AVX2 - (len(needle) - 1)
	maxPtr := GP64(); MOVQ(endPtr, maxPtr)
	SUBQ(Imm(search.LOOP_SIZE_AVX2), maxPtr)
	SUBQ(needleLenMain, maxPtr)

	// TODO: align curPtr https://github.com/BurntSushi/memchr/blob/master/src/arch/generic/memchr.rs#L169
	curPtr := GP64(); MOVQ(startPtr, curPtr)

	// TODO: We might want to find the rare bytes instead. See https://github.com/BurntSushi/memchr/blob/master/src/memmem/rarebytes.rs#L47
	first, last := inlineSplat(needlePtr, needleLenMain)

	// Do-while style loop that always ends with a scan at curPtr == maxPtr,
	// so every candidate position in [startPtr, endPtr - len(needle)] is
	// covered without any partial-width tail scan.
	Label("chunk_loop")

	// TODO: unroll loop

	o := inlineFindInChunk("main", first, last, curPtr, needlePtr, needleLenMain)
	Comment("break early when offset is >=0.")
	CMPQ(o, Imm(0))
	JGE(LabelRef("matched"))

	Comment("if curPtr == maxPtr we just scanned the final window")
	CMPQ(curPtr, maxPtr)
	JGE(LabelRef("not_matched"))

	Comment("advance curPtr by LOOP_SIZE_AVX2, clamped to maxPtr")
	ADDQ(Imm(search.LOOP_SIZE_AVX2), curPtr)
	CMPQ(curPtr, maxPtr)
	JLE(LabelRef("chunk_loop"))
	MOVQ(maxPtr, curPtr)
	JMP(LabelRef("chunk_loop"))

	Label("matched")
	// Return true index
	inlineMatched(startPtr, curPtr, o)

	Label("not_matched")
	ret, _ := ReturnIndex(0).Resolve()
	MOVQ(o, ret.Addr)
	VZEROUPPER()
	RET()

	Generate()
}

// inlineSplat fills one 256bit register with repeated first neelde char and
// another with repeated last needle char.
func inlineSplat(needle0, needleLen reg.Register) (reg.VecVirtual, reg.VecVirtual) {
	Comment("create vector filled with first and last character")
	f := YMM()
	l := YMM()

	needle1 := GP64()
	LEAQ(Mem{Base: needle0, Index: needleLen, Scale: 1}, needle1)
	VPBROADCASTB(Mem{Base: needle0}, f)
	VPBROADCASTB(Mem{Base: needle1}, l)

	return f, l
}

// inlineMatched adjusts the offset and returns the true index.
func inlineMatched(startPtr, ptr, offset reg.Register) {
	Comment("adjust the offset and return the true index")
	i := GP64()
	MOVQ(ptr, i)
	SUBQ(startPtr, i)
	ADDQ(offset, i)
	Store(i, ReturnIndex(0))
	VZEROUPPER()
	RET()
}

// inlineFindInChunk compares chunks of the first and last byte with chunks in the haystack.
func inlineFindInChunk(caller string, first, last reg.VecVirtual, curPtr, needlePtr, needleLen reg.Register) reg.Register {
	Comment("begin " + caller + " find in chunk")
	chunk0 := YMM()
	chunk1 := YMM()

	// create chunk0 and chunk1
	c0 := curPtr
	c1 := GP64()
	LEAQ(Mem{Base: c0, Index: needleLen, Scale: 1}, c1)
	VMOVDQU(Mem{Base: c0}, chunk0)
	VMOVDQU(Mem{Base: c1}, chunk1)

	// compare first and last character with chunk0 and chunk1
	eq0 := YMM()
	eq1 := YMM()
	VPCMPEQB(first, chunk0, eq0)
	VPCMPEQB(last, chunk1, eq1)

	mask := YMM()
	VPAND(eq0, eq1, mask)

	Comment("calculate offsets")
	offsets := GP32()
	VPMOVMSKB(mask, offsets)
	offset := GP64()
	MOVQ(I64(-1), offset)

	// The mask already proves the first and last needle byte match, so the
	// memcmp only needs to verify the interior needleLen-2 bytes. Pre-shift
	// the needle pointer past byte 0 and the size down by one; the +1 on
	// the candidate pointer is folded into the LEAQ inside the offsets loop.
	Comment("pre-shift memcmp inputs to skip already-verified bytes")
	nPtrShifted := GP64()
	LEAQ(Mem{Base: needlePtr, Disp: 1}, nPtrShifted)
	sizeShifted := GP64()
	LEAQ(Mem{Base: needleLen, Disp: -1}, sizeShifted)

	Comment("loop over offsets, ie bit positions")
	Label(caller + "_offsets_loop")
	CMPL(offsets, Imm(0))
	JE(LabelRef(caller + "_offsets_loop_done"))

	TZCNTL(offsets, offset.As32())

	chunkPtr := GP64()
	LEAQ(Mem{Base: c0, Index: offset.As64(), Scale: 1, Disp: 1}, chunkPtr)

	Comment("test chunk (interior only)")
	cmpIndex := inlineMemcmp(caller, chunkPtr, nPtrShifted, sizeShifted)
	Comment("break early on a match")
	CMPQ(cmpIndex, Imm(0))
	JE(LabelRef(caller + "_chunk_match"))

	inlineClearLeftmostSet(offsets)
	JMP(LabelRef(caller + "_offsets_loop"))

	Label(caller + "_offsets_loop_done")
	// We have no match
	MOVQ(I64(-1), offset)

	Label(caller + "_chunk_match")
	Comment("end " + caller + " find in chunk")
	return offset
}

// inlineClearLeftmostSet clears the left most non-zero bit of mask
func inlineClearLeftmostSet(mask reg.Register) {
	// mask = mask & (mask -1)
	tmp := GP32(); MOVL(mask, tmp)
	DECL(tmp)
	ANDL(tmp, mask)
}

// inlineMemcmp compares the bytes in xPtr and yPtr. The returned register is
// the index where it left off. If it's 0 there's a match.
func inlineMemcmp(caller string, xPtr, yPtr, size reg.Register) reg.Register {
	Comment("compare two slices")

	i := GP64(); MOVQ(size, i)

	CMPQ(size, Imm(4))
	JGE(LabelRef(caller + "_compare_four_bytes"))

	inlineMemcmpOneByte(caller, xPtr, yPtr, size, i)
	JMP(LabelRef(caller + "_memcmp_done"))

	Label(caller + "_compare_four_bytes")
	inlineMemcmpFourBytes(caller, xPtr, yPtr, size, i)

	Label(caller + "_memcmp_done")

	return i
}

func inlineMemcmpOneByte(caller string, xPtr, yPtr, size, i reg.Register) {
	Comment("compare two slices one byte at a time")

	x := GP64(); MOVQ(xPtr, x)
	y := GP64(); MOVQ(yPtr, y)
	r := GP8()

	Label(caller + "_memcmp_one_loop")

	Comment("loop by one byte")
	CMPQ(i, Imm(0))
	JE(LabelRef(caller + "_memcmp_one_loop_done"))

	MOVB(Mem{Base: y}, r)
	CMPB(Mem{Base: x}, r)
	// Break early
	JNE(LabelRef(caller + "_memcmp_one_loop_done"))

	ADDQ(Imm(1), x)
	ADDQ(Imm(1), y)
	DECQ(i)
	JMP(LabelRef(caller + "_memcmp_one_loop"))

	// do not return anything
	Label(caller + "_memcmp_one_loop_done")
}

func inlineMemcmpFourBytes(caller string, xPtr, yPtr, size, i reg.Register) {
	Comment("compare two slices four bytes at a time")
	x := GP64(); MOVQ(xPtr, x)
	y := GP64(); MOVQ(yPtr, y)

	xEnd := GP64()
	LEAQ(Mem{Base: xPtr, Index: size, Scale: 1, Disp: -4}, xEnd)
	yEnd := GP64()
	LEAQ(Mem{Base: yPtr, Index: size, Scale: 1, Disp: -4}, yEnd)

	r := GP32()

	Comment("loop by four bytes")
	Label(caller + "_memcmp_four_loop")
	CMPQ(x, xEnd)
	JGE(LabelRef(caller + "_memcmp_four_loop_done"))

	MOVL(Mem{Base: y}, r)
	CMPL(Mem{Base: x}, r)
	// Break early
	JNE(LabelRef(caller + "_memcmp_four_done"))

	ADDQ(Imm(4), x)
	ADDQ(Imm(4), y)
	JMP(LabelRef(caller + "_memcmp_four_loop"))

	Label(caller + "_memcmp_four_loop_done")
	
	Comment("compare last four bytes")
	MOVL(Mem{Base: yEnd}, r)
	CMPL(Mem{Base: xEnd}, r)
	JNE(LabelRef(caller + "_memcmp_four_done"))
	XORQ(i, i) // 0 means equal

	Label(caller + "_memcmp_four_done")
}
