package search

import (
	"fmt"
	"math/rand"
	"testing"
)

// buildHaystack generates a haystack of size bytes from a small alphabet
// that deliberately excludes needle's bytes, so no accidental match occurs,
// then optionally plants needle at a given position.
func buildHaystack(size int, needle []byte, plantAt int, seed int64) []byte {
	r := rand.New(rand.NewSource(seed))
	alphabet := []byte("0123456789")
	h := make([]byte, size)
	for i := range h {
		h[i] = alphabet[r.Intn(len(alphabet))]
	}
	if plantAt >= 0 {
		copy(h[plantAt:], needle)
	}
	return h
}

// BenchmarkIndexNoMatch measures pure scan throughput with no match at all,
// the steady-state, branch-predictable case loop unrolling targets most
// directly (every chunk_loop iteration runs the full body, no early exit).
func BenchmarkIndexNoMatch(b *testing.B) {
	sizes := []int{64 * 1024, 1024 * 1024, 8 * 1024 * 1024}
	needleLens := []int{4, 16, 64}

	for _, size := range sizes {
		for _, nl := range needleLens {
			needle := make([]byte, nl)
			for i := range needle {
				needle[i] = 'X' // not in the haystack's alphabet
			}
			haystack := buildHaystack(size, needle, -1, 42)

			b.Run(fmt.Sprintf("size=%d/needleLen=%d", size, nl), func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(size))
				for range b.N {
					if Index(haystack, needle) != -1 {
						b.Fatal("expected no match")
					}
				}
			})
		}
	}
}

// BenchmarkIndexMatchAtEnd measures scan throughput when the needle is
// planted right at the very end of the haystack, i.e. the scan must
// traverse (almost) the entire buffer before finding it - similar in shape
// to BenchmarkIndexBig/Small but without depending on the LFS-tracked
// data.zip fixture.
func BenchmarkIndexMatchAtEnd(b *testing.B) {
	sizes := []int{64 * 1024, 1024 * 1024, 8 * 1024 * 1024}
	needleLens := []int{4, 16, 64}

	for _, size := range sizes {
		for _, nl := range needleLens {
			needle := make([]byte, nl)
			for i := range needle {
				needle[i] = 'X'
			}
			plantAt := size - nl - 8
			haystack := buildHaystack(size, needle, plantAt, 42)

			b.Run(fmt.Sprintf("size=%d/needleLen=%d", size, nl), func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(size))
				for range b.N {
					if Index(haystack, needle) == -1 {
						b.Fatal("expected a match")
					}
				}
			})
		}
	}
}
