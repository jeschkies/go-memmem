//go:build ignore

package main

import (
	. "github.com/mmcloughlin/avo/build"
	. "github.com/mmcloughlin/avo/operand"
	"github.com/mmcloughlin/avo/reg"
)

// main generates assembly code for NEON on ARM aarch64.
func main() {
	TEXT("findInChunk", NOSPLIT, "func(needle []byte, haystack []byte) int64")
	Doc("findInChunk is only generated for testing.")
	//hptr := Load(Param("haystack").Base(), GP64())
	needleLen := Load(Param("needle").Len(), GP64()); DECQ(needleLen)
	needle := Load(Param("needle").Base(), GP64())
	_, _ = inlineSplat(needle, needleLen)

	//offset := inlineFindInChunk("test", f, l, hptr, needle, needleLen)
	offset := GP64()

	Store(offset, ReturnIndex(0))
	RET()
}

// inlineSplat fills one 128bit register with repeated first neelde char and
// another with repeated last needle char.
func inlineSplat(needle0, needleLen reg.Register) (reg.VecVirtual, reg.VecVirtual) {
	Comment("create vector filled with first and last character")
	f := YMM()
	l := YMM()

	VDUP

	needle1 := GP64(); MOVQ(needle0, needle1);
	ADDQ(needleLen, needle1)
	VPBROADCASTB(Mem{Base: needle0}, f)
	VPBROADCASTB(Mem{Base: needle1}, l)

	return f, l
}
