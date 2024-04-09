package search

import (
	"bytes"

	"golang.org/x/sys/cpu"
)

const LOOP_SIZE_AVX2 = 32 // size of YMM == 256bit == 32b

func init() {
	if cpu.X86.HasAVX2 {
		index = func(haystack []byte, needle []byte) int64 {
			if LOOP_SIZE_AVX2 > len(haystack) {
				return indexAvx2(haystack, needle)
			}
			return int64(bytes.Index(haystack, needle)) 
		}
	} else {
		index = func(haystack []byte, needle []byte) int64 { return int64(bytes.Index(haystack, needle)) }
	}
}

