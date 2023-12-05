package search

import (
	"bytes"

	"golang.org/x/sys/cpu"
)

var (
	index func([]byte, []byte) int64
)

func init() {
	if cpu.X86.HasAVX512 {
		index = indexAvx512
	} else {
		index = func(haystack []byte, needle []byte) int64 { return int64(bytes.Index(haystack, needle)) }
	}
}

// Index returns the first position the needle is in the haystack or -1 if
// needle was not found.
func Index(haystack []byte, needle []byte) int64 {
	return index(haystack, needle)
}

