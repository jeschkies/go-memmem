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

	nPtr := Load(Param("needle").Base(), GP64())
	nLen:= Load(Param("needle").Len(), GP64())

	// TODO: We might want to find the rare bytes instead. See https://github.com/BurntSushi/memchr/blob/master/src/memmem/rarebytes.rs#L47
	first := YMM()
	last := YMM()
	VPBROADCASTB(NewParamAddr("needle", 0), first)
	k := GP64()
	MOVQ(NewParamAddr("needle", 0), k)
	ADDQ(nLen, k)
	DECQ(k)
	VPBROADCASTB(Mem{Base: k}, last)

	block_first := YMM()
	block_last := YMM()
	VMOVDQU(Mem{Base: nPtr}, block_first)
	VMOVDQU(Mem{Base: nPtr}, block_last)

	eq_first := YMM()
	eq_last := YMM()

	VPCMPEQB(first, block_first, eq_first)
	VPCMPEQB(last, block_last, eq_last)

	mask := YMM()
	VPAND(eq_first, eq_last, mask)
	VPMOVMSKB(mask)

	inline_memcmp(Param("haystack"), nPtr, nLen)

	Generate()
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
