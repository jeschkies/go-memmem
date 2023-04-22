//go:build ignore

package main 

import (
	. "github.com/mmcloughlin/avo/build"
	//. "github.com/mmcloughlin/avo/operand"
)

func main() {
	TEXT("Search", NOSPLIT, "func(haystack, needle []byte) bool")
	Doc("Search checks if haystack contains needle.")
	//h := Load(Param("haystack").Len(), GP64())
	//n := Load(Param("needle").Len(), GP64())

	r := GP8()
	ORB(r, r)

	Store(r, ReturnIndex(0))
	RET()
	Generate()
}
