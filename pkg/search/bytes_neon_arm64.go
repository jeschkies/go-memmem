package search

import (
	"bytes"
)

// findInChunk is only generated for testing.
func findInChunk(needle []byte, haystack []byte) int64

// indexNeon returns the first position the needle is in the haystack.
func indexNeon (haystack []byte, needle []byte) int64 {
	// TODO: port to ARM64 Neon. This file will be generated.
	return int64(bytes.Index(haystack, needle))
}
