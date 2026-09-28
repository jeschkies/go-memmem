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
	nPtrShiftedTest, sizeShiftedTest := inlinePreShiftNeedle(needle, needleLen)
	sTest := newChunkScratch()

	offset := inlineFindInChunk("test", f, l, hptr, needle, needleLen, nPtrShiftedTest, sizeShiftedTest, sTest)

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
	nPtrShifted, sizeShifted := inlinePreShiftNeedle(needlePtr, needleLenMain)
	s := newChunkScratch()

	// unrollMaxPtr is the last curPtr for which a second, full-width window
	// at curPtr+LOOP_SIZE_AVX2 is also still within bounds (i.e. <= maxPtr).
	// While curPtr <= unrollMaxPtr, chunk_loop2 below scans two independent
	// windows per iteration before its single loop-carried branch, instead
	// of one window per iteration with two branches (match check, then
	// advance-or-clamp) as the original loop did.
	unrollMaxPtr := GP64(); MOVQ(maxPtr, unrollMaxPtr)
	SUBQ(Imm(search.LOOP_SIZE_AVX2), unrollMaxPtr)

	Label("chunk_loop2")
	CMPQ(curPtr, unrollMaxPtr)
	JG(LabelRef("chunk_loop2_done"))

	o := inlineFindInChunk("main0", first, last, curPtr, needlePtr, needleLenMain, nPtrShifted, sizeShifted, s)
	CMPQ(o, Imm(0))
	JGE(LabelRef("matched"))

	ADDQ(Imm(search.LOOP_SIZE_AVX2), curPtr)
	o = inlineFindInChunk("main1", first, last, curPtr, needlePtr, needleLenMain, nPtrShifted, sizeShifted, s)
	CMPQ(o, Imm(0))
	JGE(LabelRef("matched"))

	Comment("if curPtr == maxPtr this pair's second window was the final one")
	CMPQ(curPtr, maxPtr)
	JGE(LabelRef("not_matched"))

	ADDQ(Imm(search.LOOP_SIZE_AVX2), curPtr)
	JMP(LabelRef("chunk_loop2"))

	Label("chunk_loop2_done")
	Comment("fewer than two full windows remain; finish one at a time exactly as before")

	// Do-while style loop that always ends with a scan at curPtr == maxPtr,
	// so every candidate position in [startPtr, endPtr - len(needle)] is
	// covered without any partial-width tail scan.
	Label("chunk_loop")

	o = inlineFindInChunk("main", first, last, curPtr, needlePtr, needleLenMain, nPtrShifted, sizeShifted, s)
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

// inlinePreShiftNeedle computes the memcmp inputs needed to verify a
// candidate's interior bytes, once per needle: the mask inlineFindInChunk
// applies already proves the first and last needle byte match, so the
// memcmp only needs to check the interior needleLen-2 bytes. Pre-shift the
// needle pointer past byte 0 and the size down by one; the +1 on the
// candidate pointer is folded into the LEAQ inside inlineFindInChunk's
// offsets loop.
func inlinePreShiftNeedle(needlePtr, needleLen reg.Register) (nPtrShifted, sizeShifted reg.Register) {
	Comment("pre-shift memcmp inputs to skip already-verified bytes")
	nPtrShifted = GP64()
	LEAQ(Mem{Base: needlePtr, Disp: 1}, nPtrShifted)
	sizeShifted = GP64()
	LEAQ(Mem{Base: needleLen, Disp: -1}, sizeShifted)
	return nPtrShifted, sizeShifted
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

// chunkScratch bundles every temporary register inlineFindInChunk (and the
// inlineMemcmp helpers it calls) needs. avo's register allocator sizes its
// budget by the total number of *declared* virtual registers in the
// function, not by how many are actually live at a given point - so
// duplicating a call site (e.g. for loop unrolling) by having it declare
// its own fresh scratch exhausts the allocator even though the two copies
// never execute concurrently. The fix is to declare scratch once and pass
// the same objects into every call site instead. This is safe on real
// hardware: reusing an architectural register name across two genuinely
// independent, sequential computations doesn't block the CPU's own
// out-of-order register renaming from overlapping their execution - it
// only reduces the allocation problem avo has to solve, not the ILP the
// generated code can exploit.
type chunkScratch struct {
	chunk0, chunk1, eq0, eq1, mask reg.VecVirtual
	c1, chunkPtr, offset           reg.GPVirtual // GP64
	offsets                        reg.GPVirtual // GP32
	tmp32                          reg.GPVirtual // GP32, for inlineClearLeftmostSet

	// memcmp scratch, reused across the one-byte and four-byte paths (they
	// are mutually exclusive per call, so sharing is safe); r is declared
	// GP64 and narrowed via .As8()/.As32() where a byte/dword view is
	// needed, per avo's convention for reusing one virtual register across
	// widths (see how `offset` is narrowed via .As32() below).
	cmpI, cmpX, cmpY, cmpXEnd, cmpYEnd, cmpR reg.GPVirtual
}

func newChunkScratch() *chunkScratch {
	return &chunkScratch{
		chunk0: YMM(), chunk1: YMM(), eq0: YMM(), eq1: YMM(), mask: YMM(),
		c1: GP64(), chunkPtr: GP64(), offset: GP64(),
		offsets: GP32(),
		tmp32:   GP32(),
		cmpI:    GP64(), cmpX: GP64(), cmpY: GP64(), cmpXEnd: GP64(), cmpYEnd: GP64(), cmpR: GP64(),
	}
}

// inlineFindInChunk compares chunks of the first and last byte with chunks
// in the haystack. nPtrShifted and sizeShifted are the needle pointer/size
// pre-shifted past the already-verified first byte (see inlinePreShiftNeedle).
// s is scratch space shared across every call site for a given TEXT block
// (see chunkScratch) - callers that invoke this repeatedly for the same
// needle (e.g. multiple unrolled windows per loop iteration) must reuse the
// same nPtrShifted/sizeShifted/s across every call.
func inlineFindInChunk(caller string, first, last reg.VecVirtual, curPtr, needlePtr, needleLen, nPtrShifted, sizeShifted reg.Register, s *chunkScratch) reg.Register {
	Comment("begin " + caller + " find in chunk")

	// create chunk0 and chunk1
	c0 := curPtr
	LEAQ(Mem{Base: c0, Index: needleLen, Scale: 1}, s.c1)
	VMOVDQU(Mem{Base: c0}, s.chunk0)
	VMOVDQU(Mem{Base: s.c1}, s.chunk1)

	// compare first and last character with chunk0 and chunk1
	VPCMPEQB(first, s.chunk0, s.eq0)
	VPCMPEQB(last, s.chunk1, s.eq1)

	VPAND(s.eq0, s.eq1, s.mask)

	Comment("calculate offsets")
	VPMOVMSKB(s.mask, s.offsets)
	MOVQ(I64(-1), s.offset)

	Comment("loop over offsets, ie bit positions")
	Label(caller + "_offsets_loop")
	CMPL(s.offsets, Imm(0))
	JE(LabelRef(caller + "_offsets_loop_done"))

	TZCNTL(s.offsets, s.offset.As32())

	LEAQ(Mem{Base: c0, Index: s.offset.As64(), Scale: 1, Disp: 1}, s.chunkPtr)

	Comment("test chunk (interior only)")
	cmpIndex := inlineMemcmp(caller, s.chunkPtr, nPtrShifted, sizeShifted, s)
	Comment("break early on a match")
	CMPQ(cmpIndex, Imm(0))
	JE(LabelRef(caller + "_chunk_match"))

	inlineClearLeftmostSet(s.offsets, s.tmp32)
	JMP(LabelRef(caller + "_offsets_loop"))

	Label(caller + "_offsets_loop_done")
	// We have no match
	MOVQ(I64(-1), s.offset)

	Label(caller + "_chunk_match")
	Comment("end " + caller + " find in chunk")
	return s.offset
}

// inlineClearLeftmostSet clears the left most non-zero bit of mask, using
// tmp as scratch.
func inlineClearLeftmostSet(mask, tmp reg.Register) {
	// mask = mask & (mask -1)
	MOVL(mask, tmp)
	DECL(tmp)
	ANDL(tmp, mask)
}

// inlineMemcmp compares the bytes in xPtr and yPtr. The returned register is
// the index where it left off. If it's 0 there's a match.
func inlineMemcmp(caller string, xPtr, yPtr, size reg.Register, s *chunkScratch) reg.Register {
	Comment("compare two slices")

	MOVQ(size, s.cmpI)

	CMPQ(size, Imm(4))
	JGE(LabelRef(caller + "_compare_four_bytes"))

	inlineMemcmpOneByte(caller, xPtr, yPtr, s)
	JMP(LabelRef(caller + "_memcmp_done"))

	Label(caller + "_compare_four_bytes")
	inlineMemcmpFourBytes(caller, xPtr, yPtr, size, s)

	Label(caller + "_memcmp_done")

	return s.cmpI
}

func inlineMemcmpOneByte(caller string, xPtr, yPtr reg.Register, s *chunkScratch) {
	Comment("compare two slices one byte at a time")

	MOVQ(xPtr, s.cmpX)
	MOVQ(yPtr, s.cmpY)
	r := s.cmpR.As8()

	Label(caller + "_memcmp_one_loop")

	Comment("loop by one byte")
	CMPQ(s.cmpI, Imm(0))
	JE(LabelRef(caller + "_memcmp_one_loop_done"))

	MOVB(Mem{Base: s.cmpY}, r)
	CMPB(Mem{Base: s.cmpX}, r)
	// Break early
	JNE(LabelRef(caller + "_memcmp_one_loop_done"))

	ADDQ(Imm(1), s.cmpX)
	ADDQ(Imm(1), s.cmpY)
	DECQ(s.cmpI)
	JMP(LabelRef(caller + "_memcmp_one_loop"))

	// do not return anything
	Label(caller + "_memcmp_one_loop_done")
}

func inlineMemcmpFourBytes(caller string, xPtr, yPtr, size reg.Register, s *chunkScratch) {
	Comment("compare two slices four bytes at a time")
	MOVQ(xPtr, s.cmpX)
	MOVQ(yPtr, s.cmpY)

	LEAQ(Mem{Base: xPtr, Index: size, Scale: 1, Disp: -4}, s.cmpXEnd)
	LEAQ(Mem{Base: yPtr, Index: size, Scale: 1, Disp: -4}, s.cmpYEnd)

	r := s.cmpR.As32()

	Comment("loop by four bytes")
	Label(caller + "_memcmp_four_loop")
	CMPQ(s.cmpX, s.cmpXEnd)
	JGE(LabelRef(caller + "_memcmp_four_loop_done"))

	MOVL(Mem{Base: s.cmpY}, r)
	CMPL(Mem{Base: s.cmpX}, r)
	// Break early
	JNE(LabelRef(caller + "_memcmp_four_done"))

	ADDQ(Imm(4), s.cmpX)
	ADDQ(Imm(4), s.cmpY)
	JMP(LabelRef(caller + "_memcmp_four_loop"))

	Label(caller + "_memcmp_four_loop_done")

	Comment("compare last four bytes")
	MOVL(Mem{Base: s.cmpYEnd}, r)
	CMPL(Mem{Base: s.cmpXEnd}, r)
	JNE(LabelRef(caller + "_memcmp_four_done"))
	XORQ(s.cmpI, s.cmpI) // 0 means equal

	Label(caller + "_memcmp_four_done")
}
