package search

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

//go:generate go run asm.go -out bytes.s -stubs bytes.go

func TestSimpleSearch(t *testing.T) {
	for _, tt := range []struct {
		haystack []byte 
		needle   []byte
		index    int64 
	}{
		//{[]byte(`foobar`), []byte(`foobaz`), false},
		//{[]byte(`foobar`), []byte(`foobar`), true},
		//{[]byte(`foo`), []byte(`foobar`), false},
		//{[]byte(`foobar`), []byte(`foo`), false},
		//{[]byte(`a cat tries`), []byte(`cat`), true},
		{
			[]byte(`Lorem ipsum dolor sit amet, consectetur adipiscing elit integer.`),
			[]byte(`amet`),
			int64(22),
		},
	} {
		tt := tt
		t.Run(fmt.Sprintf("`%s` in `%s`", tt.needle, tt.haystack), func(t *testing.T) {
			i := Index(tt.haystack, tt.needle)
			require.Equal(t, tt.index, i)
		})
	}
}

func TestMask(t *testing.T) {
	for _, tt := range []struct {
		name     string
		needle   []byte
		haystack [32]byte 
		index    int64 
	}{
		{
			"chunk second match",
			[]byte{4, 1, 3},
			[32]byte{
				0, 0, 0, 4, 2, 3, 0, 0,
				0, 4, 1, 3, 0, 0, 0, 0,
				0, 0, 0, 0, 0, 0, 0, 0,
				0, 0, 0, 0, 0, 0, 0, 0,
			},
			int64(9),
		},
		{
			"chunk first match",
			[]byte{4, 1, 3},
			[32]byte{
				0, 0, 0, 4, 1, 3, 0, 0,
				0, 4, 1, 3, 0, 0, 0, 0,
				0, 0, 0, 0, 0, 0, 0, 0,
				0, 0, 0, 0, 0, 0, 0, 0,
			},
			int64(3),
		},
		{
			"longer chunk",
			[]byte{4, 1, 3, 3},
			[32]byte{
				0, 0, 0, 4, 1, 3, 0, 0,
				0, 4, 1, 3, 3, 0, 0, 0,
				0, 0, 0, 0, 0, 0, 0, 0,
				0, 0, 0, 0, 0, 0, 0, 0,
			},
			int64(9),
		},
	} {
		tt := tt
		t.Run(fmt.Sprintf(tt.name), func(t *testing.T) {
			index := findInChunk(tt.needle, tt.haystack[:])
			require.Equal(t, tt.index, index)
			if index != -1 {
				end := index + int64(len(tt.needle))
				require.ElementsMatch(t, tt.needle, tt.haystack[index:end])
			}
		})
	}
}
