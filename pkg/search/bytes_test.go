package search

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSimpleIndex(t *testing.T) {
	for _, tt := range []struct {
		needle   []byte
		haystack []byte
		index    int64
	}{
		{
			[]byte{4, 1, 3},
			[]byte{
				0, 0, 0, 4, 1, 3, 0, 0,
				0, 4, 1, 3, 0, 0, 0, 0,
				0, 0, 0, 0, 0, 0, 0, 0,
				0, 0, 0, 0, 0, 0, 0, 0,
			},
			int64(3),
		},
		{
			[]byte(`amet`),
			[]byte(`Lorem ipsum dolor sit amet, consectetur adipiscing elit integer.`),
			int64(22),
		},
		{
			[]byte(`consectetur`),
			[]byte(`Lorem ipsum dolor sit amet, consectetur adipiscing elit integer.`),
			int64(28),
		},
		{
			[]byte(`no match`),
			[]byte(`Lorem ipsum dolor sit amet, consectetur adipiscing elit integer.`),
			int64(-1),
		},
		{
			[]byte(`float`),
			[]byte(`Lorem ipsum dolor sit amet, cons|ctetur adipiscing elit integer float.`),
			int64(64),
		},
		{
			[]byte(` bar`),
			[]byte(`bazz bar some more words to hit thirty two bytes`),
			int64(4),
		},
		// Edge cases identified in AVX2 implementation analysis
		// 1) Zero-length needle should return 0 (bytes.Index behavior)
		{
			[]byte{},
			[]byte{1, 2, 3},
			int64(0),
		},
		// 2) Short haystack (< 32 + (needle_len-1)) with a match
		{
			[]byte{1, 2, 3},
			[]byte{0, 0, 1, 2, 3, 0, 0, 0, 9, 9, 9, 9},
			int64(2),
		},
		// 3) Short haystack (< 32 + (needle_len-1)) without a match
		{
			[]byte{7, 7, 7},
			[]byte{0, 1, 2, 3, 4, 5, 6, 8, 8, 8},
			int64(-1),
		},
		// 4) Needle longer than haystack
		{
			[]byte{1, 2, 3, 4, 5},
			[]byte{1, 2, 3, 4},
			int64(-1),
		},
		// 5) Match at the last candidate position — reachable only via the clamped final scan.
		{
			[]byte{9, 9, 9},
			append(bytes.Repeat([]byte{0}, 37), 9, 9, 9),
			int64(37),
		},
		// 6) Same layout with a first+last-byte hit in the tail; memcmp must reject.
		{
			[]byte{9, 9, 9},
			append(bytes.Repeat([]byte{0}, 37), 9, 0, 9),
			int64(-1),
		},
	} {
		t.Run(fmt.Sprintf("`%s` in `%s`", tt.needle, tt.haystack), func(t *testing.T) {
			i := Index(tt.haystack, tt.needle)
			require.Equal(t, tt.index, i)
		})
	}
}

func TestSpecial(t *testing.T) {
	// TODO: passes here but not in Loki.
	in := []byte(`foo buzz bar`)
	ls := []byte(`foo `)
	i := Index(in, ls)
	require.Equal(t, int64(0), i)

	in = in[len(ls):]
	require.Equal(t, string(in), "buzz bar")

	ls = []byte(` bar`)
	i = Index(in, ls)
	require.Equal(t, int64(4), i)
}

func TestMask(t *testing.T) {
	for _, tt := range []struct {
		name     string
		needle   []byte
		haystack []byte
		index    int64
	}{
		{
			"chunk second match",
			[]byte{4, 1, 3},
			[]byte{
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
			[]byte{
				0, 0, 0, 4, 1, 3, 0, 0,
				0, 4, 1, 3, 0, 0, 0, 0,
				0, 0, 0, 0, 0, 0, 0, 0,
				0, 0, 0, 0, 0, 0, 0, 0,
			},
			int64(3),
		},
		{
			"no match",
			[]byte{4, 5, 3},
			[]byte{
				0, 0, 0, 4, 1, 3, 0, 0,
				0, 4, 1, 3, 0, 0, 0, 0,
				0, 0, 0, 0, 0, 0, 0, 0,
				0, 0, 0, 0, 0, 0, 0, 0,
			},
			int64(-1),
		},
		{
			"longer chunk",
			[]byte{4, 1, 3, 3},
			[]byte{
				0, 0, 0, 4, 1, 3, 0, 0,
				0, 4, 1, 3, 3, 0, 0, 0,
				0, 0, 0, 0, 0, 0, 0, 0,
				0, 0, 0, 0, 0, 0, 0, 0,
			},
			int64(9),
		},
		{
			"text chunk",
			[]byte(`amet`),
			[]byte(`Lorem ipsum dolor sit amet, consectetur adipiscing elit integer.`),
			int64(22),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// Pad so the haystack is long enough for the vectorized path.
			haystack := append(append([]byte(nil), tt.haystack...), make([]byte, 32)...)
			index := Index(haystack, tt.needle)
			require.Equal(t, tt.index, index)
			if index != -1 {
				end := index + int64(len(tt.needle))
				require.ElementsMatch(t, tt.needle, tt.haystack[index:end])
			}
		})
	}
}
