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

	r := GP8()
	ORB(r, r)

	Store(r, ReturnIndex(0))
	RET()
	Generate()
}

func memcmp(x, y gotypes.Component) {
	Comment("compare two slices")

	xPtr := Load(x.Base(), GP64())
	xLen:= Load(x.Len(), GP64())
	yPtr := Load(y.Base(), GP64())
	yLen:= Load(y.Len(), GP64())


	Label("loop")

	CMPQ(xLen, Imm(0))
	JE(LabelRef("done"))
	CMPQ(yLen, Imm(0))
	JE(LabelRef("done"))

	CMPQ(Mem{})

	Label("done")
}
