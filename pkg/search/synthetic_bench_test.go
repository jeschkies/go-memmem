package search

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"
)

// syntheticHaystack builds deterministic access-log-like data of roughly size
// bytes. It stands in for data.zip, which needs Git LFS.
func syntheticHaystack(size int) []byte {
	r := rand.New(rand.NewSource(1))
	verbs := []string{"GET", "POST", "PUT", "DELETE"}
	var buf bytes.Buffer
	for buf.Len() < size {
		fmt.Fprintf(&buf, "%d.%d.%d.%d - user%d [02/May/2023:10:%02d:%02d +0000] \"%s /api/v%d/items/%d HTTP/2.0\" %d %d\n",
			r.Intn(256), r.Intn(256), r.Intn(256), r.Intn(256), r.Intn(10000),
			r.Intn(60), r.Intn(60), verbs[r.Intn(len(verbs))], r.Intn(3), r.Intn(100000),
			200+r.Intn(5)*100, r.Intn(100000))
	}
	return buf.Bytes()
}

// BenchmarkIndexNeedleLen searches for a needle of a given length placed near
// the end of a 16 MiB haystack.
func BenchmarkIndexNeedleLen(b *testing.B) {
	haystack := syntheticHaystack(16 << 20)
	tail := haystack[len(haystack)-4096:]
	for _, n := range []int{2, 3, 4, 5, 8, 9, 12, 13, 16, 32, 64} {
		needle := append([]byte(nil), tail[1000:1000+n]...)
		want := int64(bytes.Index(haystack, needle))
		b.Run(fmt.Sprintf("len=%d", n), func(b *testing.B) {
			b.SetBytes(int64(len(haystack)))
			for b.Loop() {
				if Index(haystack, needle) != want {
					b.Fatal("wrong index")
				}
			}
		})
	}
}
