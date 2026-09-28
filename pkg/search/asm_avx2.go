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
	needleLen := Load(Param("needle").Len(), GP64())
	needle := Load(Param("needle").Base(), GP64())

	// findInChunk always tests the needle's first and last byte, matching
	// its behavior before rare-byte selection existed - it's a testing-only
	// helper for a single chunk, not the real search entry point.
	idx1 := GP64(); MOVQ(I64(0), idx1)
	idx2 := GP64(); MOVQ(needleLen, idx2); DECQ(idx2)

	f, l := inlineSplat(needle, idx1, idx2)

	offset := inlineFindInChunk("test", f, l, hptr, idx1, idx2, needle, needleLen)

	Store(offset, ReturnIndex(0))
	VZEROUPPER()
	RET()

	TEXT("indexAvx2", NOSPLIT, "func(haystack, needle []byte, idx1, idx2 int) int64")
	Doc("indexAvx2 returns the first position the needle is in the haystack. " +
		"The caller must ensure len(needle) >= 1, idx1 and idx2 are distinct " +
		"valid indices into needle (see SelectPair), and " +
		"len(haystack) >= LOOP_SIZE_AVX2 + len(needle) - 1.")

	needlePtr := Load(Param("needle").Base(), GP64())
	needleLenFull := Load(Param("needle").Len(), GP64())

	idx1Main := Load(Param("idx1"), GP64())
	idx2Main := Load(Param("idx2"), GP64())

	startPtr := Load(Param("haystack").Base(), GP64())
	haystackLen, _ := Param("haystack").Len().Resolve()

	endPtr := GP64(); MOVQ(startPtr, endPtr); ADDQ(haystackLen.Addr, endPtr)

	// maxPtr is the last curPtr for which the memcmp confirming a candidate
	// match at the worst-case bit position (LOOP_SIZE_AVX2-1) never reads
	// past the haystack: that candidate's footprint is
	// [curPtr+LOOP_SIZE_AVX2-1, curPtr+LOOP_SIZE_AVX2-1+len(needle)), so we
	// need curPtr+LOOP_SIZE_AVX2-1+len(needle)-1 < endPtr. This bound
	// doesn't depend on idx1/idx2: since both are valid needle indices
	// (<= len(needle)-1), it's also always sufficient to keep both coarse
	// chunk loads (at curPtr+idx1 and curPtr+idx2) in bounds.
	// maxPtr == endPtr - LOOP_SIZE_AVX2 - (len(needle) - 1), computed as
	// endPtr - LOOP_SIZE_AVX2 - needleLenFull + 1 to avoid needing a
	// separate needleLenFull-1 register.
	maxPtr := GP64(); MOVQ(endPtr, maxPtr)
	SUBQ(Imm(search.LOOP_SIZE_AVX2), maxPtr)
	SUBQ(needleLenFull, maxPtr)
	ADDQ(Imm(1), maxPtr)

	// TODO: align curPtr https://github.com/BurntSushi/memchr/blob/master/src/arch/generic/memchr.rs#L169
	curPtr := GP64(); MOVQ(startPtr, curPtr)

	first, last := inlineSplat(needlePtr, idx1Main, idx2Main)

	// Do-while style loop that always ends with a scan at curPtr == maxPtr,
	// so every candidate position in [startPtr, endPtr - len(needle)] is
	// covered without any partial-width tail scan.
	Label("chunk_loop")

	// TODO: unroll loop

	o := inlineFindInChunk("main", first, last, curPtr, idx1Main, idx2Main, needlePtr, needleLenFull)
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

// inlineSplat fills one 256bit register with the needle byte at idx1
// repeated, and another with the byte at idx2 repeated - the two bytes the
// coarse filter treats as most predictive of a genuine match (see
// SelectPair). Unlike the needle's first/last byte, idx1 and idx2 can be any
// two distinct valid positions in the needle.
func inlineSplat(needlePtr, idx1, idx2 reg.Register) (reg.VecVirtual, reg.VecVirtual) {
	Comment("create vector filled with the byte at idx1, and another for idx2")
	f := YMM()
	l := YMM()

	p1 := GP64()
	LEAQ(Mem{Base: needlePtr, Index: idx1, Scale: 1}, p1)
	p2 := GP64()
	LEAQ(Mem{Base: needlePtr, Index: idx2, Scale: 1}, p2)
	VPBROADCASTB(Mem{Base: p1}, f)
	VPBROADCASTB(Mem{Base: p2}, l)

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

// inlineFindInChunk compares chunks at curPtr+idx1 and curPtr+idx2 against
// the needle bytes at those positions (splatted into first/last), and for
// any candidate that passes, confirms it with a full memcmp of the whole
// needle (needlePtr, needleLen). Two of those needleLen byte comparisons
// are strictly redundant (idx1 and idx2 are already known equal from the
// coarse filter), but re-checking them keeps the confirmation loop simple;
// with a well-chosen idx1/idx2 pair, confirmations should be rare enough
// that this is not the bottleneck.
func inlineFindInChunk(caller string, first, last reg.VecVirtual, curPtr, idx1, idx2, needlePtr, needleLen reg.Register) reg.Register {
	Comment("begin " + caller + " find in chunk")
	chunk0 := YMM()
	chunk1 := YMM()

	// create chunk0 and chunk1
	c0 := GP64()
	LEAQ(Mem{Base: curPtr, Index: idx1, Scale: 1}, c0)
	c1 := GP64()
	LEAQ(Mem{Base: curPtr, Index: idx2, Scale: 1}, c1)
	VMOVDQU(Mem{Base: c0}, chunk0)
	VMOVDQU(Mem{Base: c1}, chunk1)

	// compare the byte at idx1 and idx2 with chunk0 and chunk1
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

	Comment("candidate match start = curPtr + bit position")
	chunkPtr := GP64()
	LEAQ(Mem{Base: curPtr, Index: offset.As64(), Scale: 1}, chunkPtr)

	Comment("test chunk (full needle)")
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
