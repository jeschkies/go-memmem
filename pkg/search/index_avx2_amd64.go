//go:build goexperiment.simd && amd64

package search

import (
	"bytes"
	"math/bits"
	"simd/archsimd"
)

// LOOP_SIZE_AVX2 is the size of a YMM register (256 bits == 32 bytes) and
// therefore the width of one vectorized loop iteration in the AVX2 code path.
const LOOP_SIZE_AVX2 = 32

// The AVX2 implementation follows Wojciech Muła's "generic SIMD" substring
// search (http://0x80.pl/notesen/2016-11-28-simd-strfind.html): broadcast the first and
// last needle byte, compare them against two 32-byte loads of the haystack
// that are len(needle)-1 bytes apart, and verify only the positions where both
// bytes match.

func init() {
	if archsimd.X86.AVX2() {
		index = func(haystack []byte, needle []byte) int64 {
			if len(needle) < 2 || len(haystack) < LOOP_SIZE_AVX2+len(needle)-1 {
				return int64(bytes.Index(haystack, needle))
			}
			return indexAVX2(haystack, needle)
		}
	}
}

// indexAVX2 returns the first position the needle is in the haystack. The
// caller must ensure len(needle) >= 2 and
// len(haystack) >= LOOP_SIZE_AVX2 + len(needle) - 1.
func indexAVX2(haystack, needle []byte) int64 {
	k := len(needle)
	needleSub := needle[1 : k-1]

	first := archsimd.BroadcastUint8x32(needle[0])
	last := archsimd.BroadcastUint8x32(needle[k-1])
	n := len(haystack) - LOOP_SIZE_AVX2 - (k - 1)

	i := 0
	for ; i < n; i += LOOP_SIZE_AVX2 {
		head := archsimd.LoadUint8x32(haystack[i:])
		tail := archsimd.LoadUint8x32(haystack[i+k-1:])

		mask := head.Equal(first).And(tail.Equal(last)).ToBits()

		for ; mask != 0; mask &= mask - 1 {
			p := i + bits.TrailingZeros32(mask)
			c := haystack[p+1 : p+k-1]

			if bytes.Equal(c, needleSub) {
				return int64(p)
			}
		}
	}
	if r := bytes.Index(haystack[i:], needle); r >= 0 {
		return int64(i + r)
	}

	return -1
}
