package search

import (
	"bytes"
	"math/rand"
	"testing"
)

// TestIndexAgreesWithBytesIndex compares Index with bytes.Index for every
// needle length that has a specialised verify path and haystack lengths on
// both sides of the vector width. The two-letter alphabet makes the
// first/last byte filter produce many false candidates.
func TestIndexAgreesWithBytesIndex(t *testing.T) {
	r := rand.New(rand.NewSource(42))
	for _, alphabet := range []int{2, 3, 256} {
		for k := 0; k <= 40; k++ {
			for _, extra := range []int{0, 1, 2, 31, 32, 33, 63, 64, 65, 100, 257} {
				n := k + extra
				for iter := 0; iter < 20; iter++ {
					haystack := make([]byte, n)
					for i := range haystack {
						haystack[i] = byte(r.Intn(alphabet))
					}
					var needle []byte
					if iter%2 == 0 && n >= k {
						// A needle taken from the haystack, so there is a match.
						start := r.Intn(n - k + 1)
						needle = append(needle, haystack[start:start+k]...)
					} else {
						needle = make([]byte, k)
						for i := range needle {
							needle[i] = byte(r.Intn(alphabet))
						}
					}
					want := int64(bytes.Index(haystack, needle))
					if got := Index(haystack, needle); got != want {
						t.Fatalf("alphabet=%d k=%d n=%d: Index = %d, bytes.Index = %d\nhaystack=%v\nneedle=%v",
							alphabet, k, n, got, want, haystack, needle)
					}
				}
			}
		}
	}
}

// TestIndexMatchAtEnd places the needle at the very end of the haystack,
// where the clamped final window must still find it.
func TestIndexMatchAtEnd(t *testing.T) {
	for k := 2; k <= 40; k++ {
		for _, n := range []int{k + 31, k + 32, k + 33, 200} {
			haystack := bytes.Repeat([]byte{'a'}, n)
			needle := bytes.Repeat([]byte{'b'}, k)
			copy(haystack[n-k:], needle)
			if got, want := Index(haystack, needle), int64(n-k); got != want {
				t.Fatalf("k=%d n=%d: Index = %d, want %d", k, n, got, want)
			}
		}
	}
}
