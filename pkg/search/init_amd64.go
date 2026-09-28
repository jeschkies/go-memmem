package search

import (
	"bytes"

	"golang.org/x/sys/cpu"
)

func init() {
	if cpu.X86.HasAVX2 {
		index = func(haystack []byte, needle []byte) int64 {
			// The AVX2 implementation issues two overlapping 32-byte loads
			// per iteration: one at curPtr and one at curPtr + len(needle) - 1.
			// The haystack must be at least LOOP_SIZE_AVX2 + len(needle) - 1
			// bytes long for both loads to stay in bounds.
			//
			// Needles shorter than two bytes are also handed off: len == 0 has
			// no defined position, and len == 1 collapses to bytes.IndexByte —
			// which the stdlib already vectorizes, and SelectPair requires
			// len(needle) >= 2 anyway.
			if len(needle) < 2 || len(haystack) < LOOP_SIZE_AVX2+len(needle)-1 {
				return int64(bytes.Index(haystack, needle))
			}
			idx1, idx2 := SelectPair(needle)
			return indexAvx2(haystack, needle, idx1, idx2)
		}
	} else {
		index = func(haystack []byte, needle []byte) int64 { return int64(bytes.Index(haystack, needle)) }
	}
}
