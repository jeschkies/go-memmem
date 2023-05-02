//go:build ignore

package main

import (
	. "github.com/mmcloughlin/avo/build"
	. "github.com/mmcloughlin/avo/operand"
	"github.com/mmcloughlin/avo/reg"
)

const MIN_HAYSTACK = 32

func main() {
	TEXT("findInChunk", NOSPLIT, "func(needle []byte, haystack []byte) int64")
	Doc("findInChunk is only generated for testing.")
	hptr := Load(Param("haystack").Base(), GP64())
	needleLen := Load(Param("needle").Len(), GP64()); DECQ(needleLen)
	needle := Load(Param("needle").Base(), GP64())
	f, l := inlineSplat(needle, needleLen)

	offset := inlineFindInChunk("test", f, l, hptr, needle, needleLen)

	Store(offset, ReturnIndex(0))
	RET()

	TEXT("Index", NOSPLIT, "func(haystack, needle []byte) int64")
	Doc("Index returns the first position the needle is in the haystack.")

	needlePtr := Load(Param("needle").Base(), GP64())
	needleLenMain := Load(Param("needle").Len(), GP64()); DECQ(needleLenMain)

	startPtr := Load(Param("haystack").Base(), GP64())
	haystackLen, _ := Param("haystack").Len().Resolve()
	
	endPtr := GP64(); MOVQ(startPtr, endPtr); ADDQ(haystackLen.Addr, endPtr)
	maxPtr := GP64(); MOVQ(endPtr, maxPtr);
	SUBQ(Imm(MIN_HAYSTACK), maxPtr)
	SUBQ(needleLenMain, maxPtr)
	ptr := GP64(); MOVQ(startPtr, ptr)

	// TODO: We might want to find the rare bytes instead. See https://github.com/BurntSushi/memchr/blob/master/src/memmem/rarebytes.rs#L47
	first, last := inlineSplat(needlePtr, needleLenMain)

	Label("chunk_loop")

	// while ptr <= max_ptr
	CMPQ(ptr, maxPtr)
	JG(LabelRef("chunk_loop_end"))

	o := inlineFindInChunk("main", first, last, ptr, needlePtr, needleLenMain)
	Comment("break early when offset is >=0.")
	CMPQ(o, Imm(0))
	JGE(LabelRef("matched"))

	// ptr += 32 // size of YMM == 256bit
	ADDQ(Imm(32), ptr)
	JMP(LabelRef("chunk_loop"))

	Label("matched")
	// Return true index
	inlineMatched(startPtr, ptr, o)

	Label("chunk_loop_end")
	Comment("match remaining bytes if any")
	CMPQ(ptr, endPtr)
	JGE(LabelRef("not_matched"))

	inlineMatchRemaining(first, last, ptr, endPtr, maxPtr, needlePtr, needleLenMain, o)
	CMPQ(o, Imm(0))
	JGE(LabelRef("matched"))

	Label("not_matched")
	ret, _ := ReturnIndex(0).Resolve()
	MOVQ(o, ret.Addr)
	RET()

	Generate()
}

// inlineSplat fills one 256bit register with repeated first neelde char and
// another with repeated last needle char.
func inlineSplat(needle0, needleLen reg.Register) (reg.VecVirtual, reg.VecVirtual) {
	Comment("create vector filled with first and last character")
	f := YMM()
	l := YMM()

	needle1 := GP64(); MOVQ(needle0, needle1);
	ADDQ(needleLen, needle1)
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
	RET()
}

func inlineFindInChunk(caller string, first, last reg.VecVirtual, ptr, needlePtr, needleLen reg.Register) reg.Register {
	chunk0 := YMM()
	chunk1 := YMM()

	// create chunk0 and chunk1
	c0 := ptr 
	c1:= GP64(); MOVQ(c0, c1);
	ADDQ(needleLen, c1)
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

	Comment("loop over offsets, ie bit positions")
	Label(caller + "_offsets_loop")
	CMPL(offsets, Imm(0))
	JE(LabelRef(caller + "_offsets_loop_done"))

	TZCNTL(offsets, offset.As32())

	chunkPtr := GP64(); MOVQ(c0, chunkPtr)
	ADDQ(offset.As64(), chunkPtr)

	Comment("test chunk")
	cmpIndex := inlineMemcmp(caller, chunkPtr, needlePtr, needleLen)
	Comment("break early on a match")
	CMPQ(cmpIndex, Imm(0))
	JE(LabelRef(caller + "_chunk_match"))

	inlineClearLeftmostSet(offsets)
	JMP(LabelRef(caller + "_offsets_loop"))

	Label(caller + "_offsets_loop_done")
	// We have no match
	MOVQ(I64(-1), offset)

	Label(caller + "_chunk_match")
	return offset
}

// inlineClearLeftmostSet clears the left most non-zero bit of mask
func inlineClearLeftmostSet(mask reg.Register) {
	// mask = mask & (mask -1)
	tmp := GP32(); MOVL(mask, tmp)
	DECL(tmp)
	ANDL(tmp, mask)
}

// inlineMatchRemaining searches the remaining bytes that are shorter than the
// vector size.
func inlineMatchRemaining(first, last reg.VecVirtual, ptr, endPtr, maxPtr, needlePtr, needleLen, offset reg.Register) {
	// TODO: Could use endPtr instead.
	remaining := GP64()
	MOVQ(endPtr, remaining)
	SUBQ(ptr, remaining)
	CMPQ(remaining, needleLen)
	JL(LabelRef("not_enough_bytes_left"))

	// Notice we are using maxPtr instead of ptr.
	MOVQ(maxPtr, ptr)
	o := inlineFindInChunk("remaining", first, last, ptr, needlePtr, needleLen)
	MOVQ(o, offset)
	JMP(LabelRef("match_remaining_done"))

	Label("not_enough_bytes_left")
	MOVQ(I64(-1), offset)

	Label("match_remaining_done")
	return
}

// inlineMemcmp compares the bytes in xPtr and yPtr. The returned register is
// the index where it left off. If it's 0 there's a match.
func inlineMemcmp(caller string, xPtr, yPtr, size reg.Register) reg.Register {
	Comment("compare two slices")

	i := GP64(); MOVQ(size, i)

	//CMPQ(size, Imm(4))
	//JGE(LabelRef(caller + "_compare_four_bytes"))

	inlineMemcmpOneByte(caller, xPtr, yPtr, size, i)
	//JMP(LabelRef(caller + "_memcmp_done"))

	//Label(caller + "_compare_four_bytes")
	//inlineMemcmpFourBytes(caller, xPtr, yPtr, size, i)

	//Label(caller + "_memcmp_done")

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

	xEnd := GP64(); MOVQ(xPtr, xEnd); ADDQ(size, xEnd); SUBQ(Imm(4), xEnd)
	yEnd := GP64(); MOVQ(yPtr, yEnd); ADDQ(size, yEnd); SUBQ(Imm(4), yEnd)

	r := GP32()

	Comment("loop by four bytes")
	Label(caller + "_memcmp_four_loop")
	CMPQ(x, xEnd)
	JGE(LabelRef(caller + "_memcmp_four_done"))

	MOVL(Mem{Base: y}, r)
	CMPL(Mem{Base: x}, r)
	// Break early
	JNE(LabelRef(caller + "_memcmp_four_not_equal"))

	ADDQ(Imm(4), x)
	ADDQ(Imm(4), y)
	SUBQ(Imm(4), i)
	JMP(LabelRef(caller + "_memcmp_four_loop"))

	Label(caller + "_memcmp_four_loop_done")
	
	Comment("compare last four bytes")
	MOVL(Mem{Base: yEnd}, r)
	CMPL(Mem{Base: xEnd}, r)
	JNE(LabelRef(caller + "_memcmp_four_not_equal"))
	// 0 means equal
	XORQ(i, i)
	JMP(LabelRef(caller + "_memcmp_four_done"))

	Label(caller + "_memcmp_four_not_equal")
	ADDQ(Imm(1), i)

	Label(caller + "_memcmp_four_done")
}
