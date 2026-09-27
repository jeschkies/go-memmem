package search

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestIndexSmoke exercises the platform-selected Index dispatch. It covers
// the same cases as the amd64 detail tests but only uses the public API, so
// it runs on architectures without a SIMD implementation (e.g. arm64) and
// verifies the bytes.Index fallback wiring in init_arm64.go / init_amd64.go.
func TestIndexSmoke(t *testing.T) {
	haystack := []byte(`Lorem ipsum dolor sit amet, consectetur adipiscing elit integer.`)

	require.Equal(t, int64(22), Index(haystack, []byte(`amet`)))
	require.Equal(t, int64(28), Index(haystack, []byte(`consectetur`)))
	require.Equal(t, int64(-1), Index(haystack, []byte(`no match`)))

	// Empty needle: matches bytes.Index behaviour.
	require.Equal(t, int64(0), Index(haystack, []byte{}))

	// Needle longer than haystack.
	require.Equal(t, int64(-1), Index([]byte{1, 2, 3, 4}, []byte{1, 2, 3, 4, 5}))

	// Short haystack (below the AVX2 threshold on amd64) with a match.
	require.Equal(t, int64(2), Index([]byte{0, 0, 1, 2, 3, 0, 0, 0, 9, 9, 9, 9}, []byte{1, 2, 3}))
}
