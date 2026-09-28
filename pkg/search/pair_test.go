package search

import (
	"bytes"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSelectPair(t *testing.T) {
	tt := []struct {
		name       string
		needle     string
		wantIdx1   int
		wantIdx2   int
		commentary string
	}{
		{
			name:       "two common letters, first is rarer",
			needle:     "at",
			wantIdx1:   0,
			wantIdx2:   1,
			commentary: "'a' (rank 249) is rarer than 't' (rank 251), so no swap: it stays idx1",
		},
		{
			name:     "rare byte in the middle beats first and last",
			needle:   "eeeqee",
			wantIdx1: 3,
			wantIdx2: 0,
			// 'q' (rank 139) is far rarer than 'e' (rank 253); every other
			// position is 'e', so idx2 just ends up as the first 'e' seen (0).
			commentary: "rare byte 'q' at index 3 must be selected even though it's neither first nor last",
		},
		{
			name:     "needle length 2, second byte is rarer",
			needle:   "ab",
			wantIdx1: 1,
			wantIdx2: 0,
			// 'b' (rank 216) is rarer than 'a' (rank 249), so the initial
			// (index0, index1) pair swaps.
			commentary: "'b' is rarer than 'a', so it becomes idx1 even at index 1",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			idx1, idx2 := SelectPair([]byte(tc.needle))
			require.Equal(t, tc.wantIdx1, idx1, "idx1: %s", tc.commentary)
			require.Equal(t, tc.wantIdx2, idx2, "idx2: %s", tc.commentary)
			require.NotEqual(t, idx1, idx2, "indices must always be distinct")
		})
	}
}

// TestSelectPair_AlwaysDistinctAndInBounds fuzzes SelectPair across many
// random needle lengths and alphabets, since a broken selection (out of
// bounds, or idx1 == idx2) would corrupt the AVX2 addressing math rather
// than just picking a suboptimal pair.
func TestSelectPair_AlwaysDistinctAndInBounds(t *testing.T) {
	r := rand.New(rand.NewSource(3))
	alphabet := []byte("ABCabc GC\x00\xff")

	for trial := range 2000 {
		n := 2 + r.Intn(300)
		needle := make([]byte, n)
		for i := range needle {
			needle[i] = alphabet[r.Intn(len(alphabet))]
		}

		idx1, idx2 := SelectPair(needle)
		require.NotEqual(t, idx1, idx2, "trial %d: needle=%q", trial, needle)
		require.True(t, idx1 >= 0 && idx1 < n, "trial %d: idx1=%d out of bounds for len=%d", trial, idx1, n)
		require.True(t, idx2 >= 0 && idx2 < n, "trial %d: idx2=%d out of bounds for len=%d", trial, idx2, n)
	}
}

// TestIndex_RarePositionNotFirstOrLast specifically exercises the case the
// old hardcoded first/last-byte filter couldn't handle: a needle whose most
// predictive (rarest) byte sits in the interior, with common bytes at both
// ends. If the coarse filter were still pinned to positions 0/len-1, this
// needle would still find the right answer (the confirmation memcmp is
// exhaustive either way) - this test is about SelectPair actually being
// wired into Index end-to-end, not about correctness of a fallback path.
func TestIndex_RarePositionNotFirstOrLast(t *testing.T) {
	needle := []byte("eeeqee") // see TestSelectPair: rarest byte 'q' is at index 3
	idx1, idx2 := SelectPair(needle)
	require.Equal(t, 3, idx1)

	haystack := bytes.Repeat([]byte("e"), 100)
	haystack = append(haystack, needle...)
	haystack = append(haystack, bytes.Repeat([]byte("e"), 100)...)

	want := bytes.Index(haystack, needle)
	require.GreaterOrEqual(t, want, 0)

	got := Index(haystack, needle)
	require.Equal(t, int64(want), got)
	_ = idx2
}

// TestIndex_MatchesBytesIndex is the same style of stress test as
// loki_repro_test.go's TestLokiRepro_ShortHaystackAgreesWithBytesIndex, but
// with a richer alphabet and a range of needle lengths (including >255, to
// exercise SelectPair's cap), specifically to catch any regression from
// generalizing the coarse filter to arbitrary idx1/idx2 positions instead
// of the old hardcoded first/last byte.
func TestIndex_MatchesBytesIndex(t *testing.T) {
	r := rand.New(rand.NewSource(11))
	alphabet := []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789 \t\n")

	randBytes := func(n int) []byte {
		b := make([]byte, n)
		for i := range b {
			b[i] = alphabet[r.Intn(len(alphabet))]
		}
		return b
	}

	const trials = 3000
	for trial := range trials {
		needleLen := 2 + r.Intn(300)
		needle := randBytes(needleLen)

		haystackLen := r.Intn(2000)
		haystack := randBytes(haystackLen)

		// Half the time, plant the needle so we exercise real matches too,
		// not just not-found cases.
		if r.Intn(2) == 0 && haystackLen >= needleLen {
			pos := r.Intn(haystackLen - needleLen + 1)
			copy(haystack[pos:], needle)
		}

		want := int64(bytes.Index(haystack, needle))
		got := Index(haystack, needle)
		require.Equal(t, want, got, "trial %d: needleLen=%d haystackLen=%d", trial, needleLen, haystackLen)
	}
}
