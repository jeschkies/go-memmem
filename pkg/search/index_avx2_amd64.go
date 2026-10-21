//go:build goexperiment.simd && amd64

package search

import (
	"bytes"
	"math/bits"
	"simd/archsimd"
)

// The AVX2 implementation follows Wojciech Muła's "generic SIMD" substring
// search (http://0x80.pl/notesen/2016-11-28-simd-strfind.html): broadcast the first and
// last needle byte, compare them against two 32-byte loads of the haystack
// that are len(needle)-1 bytes apart, and verify only the positions where both
// bytes match.

func init() {
	if archsimd.X86.AVX2() {
		index = func(haystack []byte, needle []byte) int64 {
			// The filter issues two overlapping 32-byte loads per iteration: one at
			// pos and one at pos + len(needle) - 1. The haystack must be at least
			// LOOP_SIZE_AVX2 + len(needle) - 1 bytes long for both loads to stay in
			// bounds.
			//
			// Needles shorter than two bytes are also handed off: len == 0 has no
			// defined position and len == 1 is bytes.IndexByte, which the stdlib
			// already vectorizes.
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
	first := archsimd.BroadcastUint8x32(needle[0])
	last := archsimd.BroadcastUint8x32(needle[k-1])

	// The filter already proved the first and last byte, so a candidate only
	// needs needle[1:k-1] compared against the haystack at p+1.
	n := needle[1 : k-1]

	// maxPos is the last window start where both 32-byte loads stay within the
	// haystack. Every candidate in [0, len(haystack)-k] is covered by the windows
	// at 0, 32, 64, ... and the final window clamped to maxPos, so there is no
	// scalar tail. The preconditions guarantee maxPos >= 0, so the window loads
	// never fail, and p+k <= len(haystack) for every candidate p, so the interior
	// slices never fail either. The code is bounds checked rather than unsafe on
	// purpose: the loads below are slower than raw pointers but a violated
	// precondition panics instead of reading out of bounds.
	maxPos := len(haystack) - LOOP_SIZE_AVX2 - (k - 1)

	for pos := 0; ; {
		// Filter two windows (64 bytes) per iteration when both are in bounds and
		// verify their candidates through one shared 64-bit mask.
		mask := uint64(candidates(haystack, pos, k, first, last))
		step := LOOP_SIZE_AVX2
		if pos+LOOP_SIZE_AVX2 <= maxPos {
			mask |= uint64(candidates(haystack, pos+LOOP_SIZE_AVX2, k, first, last)) << 32
			step = 2 * LOOP_SIZE_AVX2
		}

		for ; mask != 0; mask &= mask - 1 {
			p := pos + bits.TrailingZeros64(mask)
			c := haystack[p+1 : p+k-1]

			if bytes.Equal(c, n) {
				return int64(p)
			}
		}

		if pos >= maxPos {
			return -1
		}
		pos = min(pos+step, maxPos)
	}
}

// candidates returns a bitmask with bit i set when haystack[pos+i] equals the
// first needle byte and haystack[pos+i+k-1] equals the last needle byte. Both
// 32-byte loads must be within the haystack or the slicing panics.
func candidates(haystack []byte, pos, k int, first, last archsimd.Uint8x32) uint32 {
	head := archsimd.LoadUint8x32(haystack[pos:])
	tail := archsimd.LoadUint8x32(haystack[pos+k-1:])
	return head.Equal(first).And(tail.Equal(last)).ToBits()
}
