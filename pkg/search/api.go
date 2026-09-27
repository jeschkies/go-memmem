package search

import (
	"bytes"
)

// LOOP_SIZE_AVX2 is the size of a YMM register (256 bits == 32 bytes) and
// therefore the width of one vectorized loop iteration in the AVX2 code path.
// It lives here (rather than in init_amd64.go) so the assembly generator can
// import it on any host architecture.
const LOOP_SIZE_AVX2 = 32

var (
	index func([]byte, []byte) int64 = func(haystack []byte, needle []byte) int64 { return int64(bytes.Index(haystack, needle)) }
)

// Index returns the first position the needle is in the haystack or -1 if
// needle was not found.
func Index(haystack []byte, needle []byte) int64 {
	return index(haystack, needle)
}

