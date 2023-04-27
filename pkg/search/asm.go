//go:build ignore

package main

import (
	. "github.com/mmcloughlin/avo/build"
	. "github.com/mmcloughlin/avo/operand"
	"github.com/mmcloughlin/avo/reg"
)

const MIN_HAYSTACK = 32

func main() {
	TEXT("Mask", NOSPLIT, "func(needle []byte, haystack []byte) int64")
	f := YMM()
	l := YMM()
	ptr := Load(Param("haystack").Base(), GP64())
	neeldeLen := Load(Param("needle").Len(), GP64())
	DECQ(neeldeLen)
	needle0 := Load(Param("needle").Base(), GP64())
	needle1 := GP64(); MOVQ(needle0, needle1);
	ADDQ(neeldeLen, needle1)
	VPBROADCASTB(Mem{Base: needle0}, f)
	VPBROADCASTB(Mem{Base: needle1}, l)

	offset := inline_find_in_chunk(f, l, ptr, needle0, neeldeLen)
	//r, _ := ReturnIndex(0).Resolve()
	//MOVQ(U64(10), r.Addr) // TODO: return -1

	Store(offset, ReturnIndex(0))

	RET()

	TEXT("Search", NOSPLIT, "func(haystack, needle []byte) bool")
	Doc("Search checks if haystack contains needle.")

	needlePtr := Load(Param("needle").Base(), GP64())
	needleLen:= Load(Param("needle").Len(), GP64())

	startPtr := Load(Param("haystack").Base(), GP64())
	haystackLen, _ := Param("haystack").Len().Resolve()
	
	endPtr := GP64(); MOVQ(startPtr, endPtr); ADDQ(haystackLen.Addr, endPtr)
	maxPtr := GP64(); MOVQ(endPtr, maxPtr); SUBQ(Imm(MIN_HAYSTACK), maxPtr)
	ptr = GP64(); MOVQ(startPtr, ptr)

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

func inline_find_in_chunk(first, last reg.VecVirtual, ptr, needlePtr, needleLen reg.Register) reg.Register {
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

	Comment("loop over offsets, ie bit positions")
	Label("offsets_loop")
	CMPL(offsets, Imm(0))
	JE(LabelRef("offsets_loop_done"))

	TZCNTL(offsets, offset.As32())

	chunkPtr := GP64(); MOVQ(c0, chunkPtr)
	ADDQ(offset.As64(), chunkPtr)

	Comment("test chunk")
	cmpIndex := inline_memcmp(chunkPtr, needlePtr, needleLen)
	CMPQ(cmpIndex, Imm(0))
	JE(LabelRef("chunk_match"))

	inline_clear_leftmost_set(offsets)
	JMP(LabelRef("offsets_loop"))

	Label("offsets_loop_done")

	Label("chunk_match")

	return offset
}

// inline_clear_leftmost_set clears the left most non-zero bit of mask
func inline_clear_leftmost_set(mask reg.Register) {
	// mask = mask & (mask -1)
	tmp := GP32(); MOVL(mask, tmp)
	DECL(tmp)
	ANDL(tmp, mask)
}

// inline_memcmp compares the bytes in xPtr and yPtr. The returned register is
// the index where it left off. If it's 0 there's a match.
func inline_memcmp(xPtr, yPtr, size reg.Register) reg.Register {
	Comment("compare two slices")

	i := GP64(); MOVQ(size, i)
	x := GP64(); MOVQ(xPtr, x)
	y := GP64(); MOVQ(yPtr, y)
	// TODO: compare more than one byte at a time.
	r := GP8()

	Label("memcmp_loop")

	Comment("the loop is done; the chunks must be equal")
	CMPQ(i, Imm(0))
	JE(LabelRef("memcmp_loop_done"))

	MOVB(Mem{Base: y}, r)
	CMPB(Mem{Base: x}, r)
	// Break early
	JNE(LabelRef("memcmp_loop_done"))

	ADDQ(Imm(1), x)
	ADDQ(Imm(1), y)
	DECQ(i)
	JMP(LabelRef("memcmp_loop"))

	// do not return anything
	Label("memcmp_loop_done")
	return i
}
