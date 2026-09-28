package search

import (
	"fmt"
	"math/rand"
	"testing"
)

// buildRareByteHaystack builds a haystack full of "lures": occurrences of
// needle's first and last byte at the right spacing to look like a match to
// a first/last-byte coarse filter, but that don't actually contain needle
// (a distinct byte partway through is different each time). This is the
// worst case for a fixed first/last-byte filter and the best case for
// rare-byte selection, when first/last are common letters but the needle
// has a genuinely rare byte in the interior.
func buildRareByteHaystack(size int, needle []byte, rareBytePos int, seed int64) (haystack []byte, totalBytes int) {
	r := rand.New(rand.NewSource(seed))
	lure := make([]byte, len(needle))
	copy(lure, needle)

	for len(haystack) < size {
		// Corrupt just the rare interior byte so this "lure" is a false
		// positive for a first/last filter but not a real match.
		lure[rareBytePos] = byte('0' + r.Intn(10))
		haystack = append(haystack, lure...)
		haystack = append(haystack, ' ') // avoid accidental cross-lure matches
	}
	return haystack, len(haystack)
}

// BenchmarkRareByteSelection compares indexAvx2 called with the old
// hardcoded first/last-byte pair (idx1=0, idx2=len(needle)-1) against the
// same function called with SelectPair's rare-byte pair, on a haystack
// specifically constructed to make the first/last filter's false-positive
// rate as bad as possible. This isolates what rare-byte selection alone
// buys, independent of anything else in the search loop.
func BenchmarkRareByteSelection(b *testing.B) {
	// "e...e": common first/last letters (rank 253), with a single rare
	// byte 'q' (rank 139) at index 10 - the position a first/last-only
	// filter can't use, but SelectPair will find.
	needle := []byte("eeeeeeeeeqeeeeeeeeee")
	rareBytePos := 9

	idx1, idx2 := SelectPair(needle)
	b.Logf("SelectPair chose idx1=%d idx2=%d (byte %q, %q)", idx1, idx2, needle[idx1], needle[idx2])
	if idx1 != rareBytePos && idx2 != rareBytePos {
		b.Fatalf("expected SelectPair to pick the rare byte at %d, got idx1=%d idx2=%d", rareBytePos, idx1, idx2)
	}

	sizes := []int{64 * 1024, 1024 * 1024, 8 * 1024 * 1024}
	for _, size := range sizes {
		haystack, totalBytes := buildRareByteHaystack(size, needle, rareBytePos, 42)

		b.Run(fmt.Sprintf("size=%d/FirstLast", size), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(totalBytes))
			for range b.N {
				_ = indexAvx2(haystack, needle, 0, len(needle)-1)
			}
		})

		b.Run(fmt.Sprintf("size=%d/RareByte", size), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(totalBytes))
			for range b.N {
				_ = indexAvx2(haystack, needle, idx1, idx2)
			}
		})
	}
}
