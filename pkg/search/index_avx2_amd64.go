//go:build goexperiment.simd && amd64

package search

import (
	"bytes"
	"math/bits"
	"simd/archsimd"
	"unsafe"
)

// The AVX2 implementation follows Wojciech Muła's "generic SIMD" substring
// search (http://0x80.pl/articles/simd-strfind.html): broadcast the first and
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
	base := unsafe.Pointer(unsafe.SliceData(haystack))

	// The filter already proved the first and last byte, so a candidate only
	// needs needle[1:k-1], m bytes, compared against the haystack at p+1.
	m := k - 2
	n := unsafe.Add(unsafe.Pointer(unsafe.SliceData(needle)), 1)

	// maxPos is the last window start where both 32-byte loads stay within the
	// haystack. Every candidate in [0, len(haystack)-k] is covered by the windows
	// at 0, 32, 64, ... and the final window clamped to maxPos, so there is no
	// scalar tail. The preconditions guarantee maxPos >= 0, which is why the
	// loads below need no bounds checks. They also guarantee p+k <= len(haystack)
	// for every candidate p, so the interior loads are in bounds too.
	maxPos := len(haystack) - LOOP_SIZE_AVX2 - (k - 1)

	for pos := 0; ; {
		// Filter two windows (64 bytes) per iteration when both are in bounds and
		// verify their candidates through one shared 64-bit mask.
		mask := uint64(candidates(base, pos, k, first, last))
		step := LOOP_SIZE_AVX2
		if pos+LOOP_SIZE_AVX2 <= maxPos {
			mask |= uint64(candidates(base, pos+LOOP_SIZE_AVX2, k, first, last)) << 32
			step = 2 * LOOP_SIZE_AVX2
		}

	candidate:
		for ; mask != 0; mask &= mask - 1 {
			p := pos + bits.TrailingZeros64(mask)
			c := unsafe.Add(base, p+1)

			// Compare the interior like a memcmp: bytewise when shorter than four
			// bytes, otherwise four bytes at a time with a final overlapping load.
			if m < 4 {
				for i := 0; i < m; i++ {
					if *(*byte)(unsafe.Add(c, i)) != *(*byte)(unsafe.Add(n, i)) {
						continue candidate
					}
				}
				return int64(p)
			}
			tail := m - 4
			for i := 0; i < tail; i += 4 {
				if *(*uint32)(unsafe.Add(c, i)) != *(*uint32)(unsafe.Add(n, i)) {
					continue candidate
				}
			}
			if *(*uint32)(unsafe.Add(c, tail)) == *(*uint32)(unsafe.Add(n, tail)) {
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
// first needle byte and haystack[pos+i+k-1] equals the last needle byte. The
// caller guarantees both 32-byte loads are within the haystack.
func candidates(base unsafe.Pointer, pos, k int, first, last archsimd.Uint8x32) uint32 {
	head := archsimd.LoadUint8x32Array((*[LOOP_SIZE_AVX2]uint8)(unsafe.Add(base, pos)))
	tail := archsimd.LoadUint8x32Array((*[LOOP_SIZE_AVX2]uint8)(unsafe.Add(base, pos+k-1)))
	return head.Equal(first).And(tail.Equal(last)).ToBits()
}
