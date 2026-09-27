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
			// bytes long for both loads to stay in bounds. It also does not
			// handle a zero-length needle. Fall back to bytes.Index in those
			// cases.
			if len(needle) == 0 || len(haystack) < LOOP_SIZE_AVX2+len(needle)-1 {
				return int64(bytes.Index(haystack, needle))
			}
			return indexAvx2(haystack, needle)
		}
	} else {
		index = func(haystack []byte, needle []byte) int64 { return int64(bytes.Index(haystack, needle)) }
	}
}
