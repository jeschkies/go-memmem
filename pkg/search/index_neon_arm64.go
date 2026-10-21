//go:build goexperiment.simd && arm64

package search

import (
	"bytes"
	"math/bits"
	"simd/archsimd"
)

// loopSizeNEON is the number of haystack bytes a single NEON register covers.
const loopSizeNEON = 16

// The NEON implementation follows the AArch64 variant of Wojciech Muła's
// "generic SIMD" substring search
// (http://0x80.pl/notesen/2016-11-28-simd-strfind.html#aarch64-64-bit-code):
// the filter is the same as on amd64, but NEON has no movemask, so the compare
// result is read as two 64-bit halves in which every matching byte is 0xff and
// the set bytes are walked in ascending order.

func init() {
	index = func(haystack []byte, needle []byte) int64 {
		// The filter issues two overlapping 16-byte loads per iteration: one at
		// pos and one at pos + len(needle) - 1, so the haystack must be at least
		// loopSizeNEON + len(needle) - 1 bytes long. Needles shorter than two
		// bytes are handed off as well, see index_avx2_amd64.go.
		if len(needle) < 2 || len(haystack) < loopSizeNEON+len(needle)-1 {
			return int64(bytes.Index(haystack, needle))
		}
		return indexNEON(haystack, needle)
	}
}

// indexNEON returns the first position of needle in haystack or -1.
// It requires len(needle) >= 2 and len(haystack) >= loopSizeNEON+len(needle)-1.
func indexNEON(haystack, needle []byte) int64 {
	k := len(needle)
	first := archsimd.BroadcastUint8x16(needle[0])
	last := archsimd.BroadcastUint8x16(needle[k-1])
	n := needle[1 : k-1]
	// The last window is clamped to maxPos instead of falling back to a scalar
	// tail. Positions it shares with the previous window were already verified.
	maxPos := len(haystack) - loopSizeNEON - (k - 1)
	for pos := 0; ; pos = min(pos+loopSizeNEON, maxPos) {
		head := archsimd.LoadUint8x16(haystack[pos:])
		tail := archsimd.LoadUint8x16(haystack[pos+k-1:])
		m := head.Equal(first).And(tail.Equal(last)).ToInt8x16().ToBits().ReshapeToUint64s()

		for half := uint8(0); half < 2; half++ {
			w := m.GetElem(half)
			for ; w != 0; w &^= 0xff << (bits.TrailingZeros64(w) &^ 7) {
				p := pos + int(half)*8 + bits.TrailingZeros64(w)/8
				if bytes.Equal(haystack[p+1:p+k-1], n) {
					return int64(p)
				}
			}
		}
		if pos >= maxPos {
			return -1
		}
	}
}
