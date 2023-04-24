//go:build ignore

package main

import (
	. "github.com/mmcloughlin/avo/build"
	"github.com/mmcloughlin/avo/gotypes"
	. "github.com/mmcloughlin/avo/operand"
)

func main() {
	TEXT("Search", NOSPLIT, "func(haystack, needle []byte) bool")
	Doc("Search checks if haystack contains needle.")

	memcmp(Param("haystack"), Param("needle"))

	Generate()
}

func memcmp(x, y gotypes.Component) {
	Comment("compare two slices")

	xPtr := Load(x.Base(), GP64())
	xLen:= Load(x.Len(), GP64())
	yPtr := Load(y.Base(), GP64())
	yLen:= Load(y.Len(), GP64())

	ret, _ := ReturnIndex(0).Resolve()

	// TODO: compare more than one byte at a time.
	r := GP8()

	Label("memcmp_loop")

	CMPQ(xLen, Imm(0))
	JE(LabelRef("equal"))
	CMPQ(yLen, Imm(0))
	JE(LabelRef("equal"))

	MOVB(Mem{Base: yPtr}, r)
	CMPB(Mem{Base: xPtr}, r)

	/*
	JMP(LabelRef("not_equal"))
	*/

	ADDQ(Imm(1), xPtr)
	DECQ(xLen)
	ADDQ(Imm(1), yPtr)
	DECQ(yLen)
	JMP(LabelRef("memcmp_loop"))

	//Label("done")

	/*
	Comment("check if both are done")
	CMPQ(xLen, yLen)	
	JE(LabelRef("equal"))
	*/

	Label("not_equal")
	MOVB(U8(0), ret.Addr)
	RET()

	Label("equal")
	MOVB(U8(1), ret.Addr)
	RET()
}
