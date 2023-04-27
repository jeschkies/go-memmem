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
		match    bool
	}{
		//{[]byte(`foobar`), []byte(`foobaz`), false},
		//{[]byte(`foobar`), []byte(`foobar`), true},
		//{[]byte(`foo`), []byte(`foobar`), false},
		//{[]byte(`foobar`), []byte(`foo`), false},
		//{[]byte(`a cat tries`), []byte(`cat`), true},
		{
			[]byte(`Lorem ipsum dolor sit amet, consectetur adipiscing elit integer.`),
			[]byte(`amet`),
			true,
		},
	} {
		tt := tt
		t.Run(fmt.Sprintf("`%s` in `%s`", tt.needle, tt.haystack), func(t *testing.T) {
			r := Search(tt.haystack, tt.needle)
			require.Equal(t, tt.match, r)
		})
	}
}

func TestMask(t *testing.T) {
	needle := []byte{4, 1, 3}
	array := [32]byte{
		0, 0, 0, 4, 1, 3, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0,
	}
	found := Mask(needle, array[:])
	require.Equal(t, true, found)
	//require.Equal(t, int64(3), position)
}
