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

// Width classes for comparing the needle interior needle[1:k-1] (m = k-2
// bytes). The filter already proved the first and last byte, so a candidate
// only needs one or two integer compares of the interior. Two overlapping loads
// of the class width cover every m in the class without a byte loop.
const (
	interior1     = iota // m == 1:     one byte
	interior2to3         // m == 2..3:  two overlapping uint16
	interior4to7         // m == 4..7:  two overlapping uint32
	interior8to16        // m == 8..16: two overlapping uint64
	interiorLong         // m > 16:     first uint64, then bytes.Equal
)

// indexAVX2 returns the first position the needle is in the haystack. The
// caller must ensure len(needle) >= 2 and
// len(haystack) >= LOOP_SIZE_AVX2 + len(needle) - 1.
func indexAVX2(haystack, needle []byte) int64 {
	k := len(needle)
	first := archsimd.BroadcastUint8x32(needle[0])
	last := archsimd.BroadcastUint8x32(needle[k-1])
	base := unsafe.Pointer(unsafe.SliceData(haystack))

	// maxPos is the last window start where both 32-byte loads stay within the
	// haystack. Every candidate in [0, len(haystack)-k] is covered by the windows
	// at 0, 32, 64, ... and the final window clamped to maxPos, so there is no
	// scalar tail. The preconditions guarantee maxPos >= 0, which is why the
	// loads below need no bounds checks. They also guarantee p+k <= len(haystack)
	// for every candidate p, so the interior loads are in bounds too.
	maxPos := len(haystack) - LOOP_SIZE_AVX2 - (k - 1)

	// With two bytes the filter is exact: first and last are the whole needle.
	if k == 2 {
		for pos := 0; ; {
			if mask := candidates(base, pos, k, first, last); mask != 0 {
				return int64(pos + bits.TrailingZeros32(mask))
			}
			if pos >= maxPos {
				return -1
			}
			pos = min(pos+LOOP_SIZE_AVX2, maxPos)
		}
	}

	// Pick the interior width class once and load the needle words once. w0 is
	// the interior prefix and w1 the interior suffix at offset off, both
	// zero-extended to 64 bits. Verifying is done inline rather than in a called
	// function so the vector registers stay live across candidates.
	m := k - 2
	n := unsafe.Add(unsafe.Pointer(unsafe.SliceData(needle)), 1)
	var class int
	var w0, w1 uint64
	var off int
	switch {
	case m == 1:
		class, w0 = interior1, uint64(*(*uint8)(n))
	case m <= 3:
		class, off = interior2to3, m-2
		w0, w1 = uint64(*(*uint16)(n)), uint64(*(*uint16)(unsafe.Add(n, off)))
	case m <= 7:
		class, off = interior4to7, m-4
		w0, w1 = uint64(*(*uint32)(n)), uint64(*(*uint32)(unsafe.Add(n, off)))
	case m <= 16:
		class, off = interior8to16, m-8
		w0, w1 = *(*uint64)(n), *(*uint64)(unsafe.Add(n, off))
	default:
		class, w0 = interiorLong, *(*uint64)(n)
	}

	for pos := 0; ; {
		// Filter two windows (64 bytes) per iteration when both are in bounds and
		// verify their candidates through one shared 64-bit mask.
		mask := uint64(candidates(base, pos, k, first, last))
		step := LOOP_SIZE_AVX2
		if pos+LOOP_SIZE_AVX2 <= maxPos {
			mask |= uint64(candidates(base, pos+LOOP_SIZE_AVX2, k, first, last)) << 32
			step = 2 * LOOP_SIZE_AVX2
		}
		for ; mask != 0; mask &= mask - 1 {
			p := pos + bits.TrailingZeros64(mask)
			c := unsafe.Add(base, p+1)
			switch class {
			case interior1:
				if uint64(*(*uint8)(c)) == w0 {
					return int64(p)
				}
			case interior2to3:
				if uint64(*(*uint16)(c)) == w0 && uint64(*(*uint16)(unsafe.Add(c, off))) == w1 {
					return int64(p)
				}
			case interior4to7:
				if uint64(*(*uint32)(c)) == w0 && uint64(*(*uint32)(unsafe.Add(c, off))) == w1 {
					return int64(p)
				}
			case interior8to16:
				if *(*uint64)(c) == w0 && *(*uint64)(unsafe.Add(c, off)) == w1 {
					return int64(p)
				}
			default:
				if *(*uint64)(c) == w0 && equalLong(c, n, m) {
					return int64(p)
				}
			}
		}
		if pos >= maxPos {
			return -1
		}
		pos = min(pos+step, maxPos)
	}
}

// equalLong reports whether the m bytes at a and b are equal. It is a
// separate function only so the call sits outside the hot loop's inlined
// candidate check and does not keep the haystack and needle slices live there.
//
//go:noinline
func equalLong(a, b unsafe.Pointer, m int) bool {
	return bytes.Equal(unsafe.Slice((*byte)(a), m), unsafe.Slice((*byte)(b), m))
}

// candidates returns a bitmask with bit i set when haystack[pos+i] equals the
// first needle byte and haystack[pos+i+k-1] equals the last needle byte. The
// caller guarantees both 32-byte loads are within the haystack.
func candidates(base unsafe.Pointer, pos, k int, first, last archsimd.Uint8x32) uint32 {
	head := archsimd.LoadUint8x32Array((*[LOOP_SIZE_AVX2]uint8)(unsafe.Add(base, pos)))
	tail := archsimd.LoadUint8x32Array((*[LOOP_SIZE_AVX2]uint8)(unsafe.Add(base, pos+k-1)))
	return head.Equal(first).And(tail.Equal(last)).ToBits()
}
