package search

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"
)

// TestUnrollBoundary_AgreesWithBytesIndex sweeps haystack lengths densely
// around multiples of LOOP_SIZE_AVX2 (32) and LOOP_SIZE_AVX2*2 (64), for
// several needle lengths, since that's exactly where chunk_loop2's new
// "two windows per iteration, clamped to a single window when fewer than
// two remain" control flow could disagree with the previous single-step
// loop if the boundary arithmetic (unrollMaxPtr) were off by even one byte.
func TestUnrollBoundary_AgreesWithBytesIndex(t *testing.T) {
	r := rand.New(rand.NewSource(7))
	alphabet := []byte("01")

	for _, needleLen := range []int{1, 2, 3, 4, 5, 8, 17, 33} {
		needle := make([]byte, needleLen)
		for i := range needle {
			needle[i] = '9' // not in the haystack alphabet, so never a real match
		}

		for base := 0; base <= 200; base++ {
			size := base
			t.Run(fmt.Sprintf("needleLen=%d/size=%d/nomatch", needleLen, size), func(t *testing.T) {
				haystack := make([]byte, size)
				for i := range haystack {
					haystack[i] = alphabet[r.Intn(len(alphabet))]
				}
				want := int64(bytes.Index(haystack, needle))
				got := Index(haystack, needle)
				if want != got {
					t.Fatalf("size=%d needleLen=%d: want=%d got=%d haystack=%q", size, needleLen, want, got, haystack)
				}
			})

			// Same size, but plant a genuine match at every possible offset
			// so both chunk_loop2 and the chunk_loop tail actually get to
			// report a real match, not just agree on "not found".
			for plantAt := 0; plantAt+needleLen <= size; plantAt += 7 {
				haystack := make([]byte, size)
				for i := range haystack {
					haystack[i] = alphabet[r.Intn(len(alphabet))]
				}
				copy(haystack[plantAt:], needle)
				want := int64(bytes.Index(haystack, needle))
				got := Index(haystack, needle)
				if want != got {
					t.Fatalf("size=%d needleLen=%d plantAt=%d: want=%d got=%d haystack=%q", size, needleLen, plantAt, want, got, haystack)
				}
			}
		}
	}
}
