package search

import (
	"testing"

	"github.com/stretchr/testify/require"
)

//go:generate go run asm.go -out bytes.s -stubs bytes.go

func TestSimpleSearch(t *testing.T) {
	haystack := []byte(`foobar`)
	needle := []byte(`foobar`)
	r := Search(haystack, needle)
	require.True(t, r)
}
