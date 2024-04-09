package search

import (
	"bytes"

	"golang.org/x/sys/cpu"
)

const MIN_HAYSTACK = 32

func init() {
	// TODO: if len(haystack) < min_haystack -> bytes.Index()
	if cpu.X86.HasAVX2 {
		index = indexAvx2
	} else {
		index = func(haystack []byte, needle []byte) int64 { return int64(bytes.Index(haystack, needle)) }
	}
}

