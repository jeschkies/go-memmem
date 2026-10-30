package search

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"

	"golang.org/x/sys/cpu"
)

const (
	Byte = 1
	KiB  = 1024 * Byte
	MiB  = 1024 * KiB
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
	needleLen := []int{2, 4, 8, 12, 16, 32, 64}
	haystackSizes := []int{256 * KiB, 512 * KiB, 1 * MiB, 2 * MiB, 4 * MiB}
	for _, n := range needleLen {
		for _, h := range haystackSizes {
			haystack := syntheticHaystack(h)

			// Needle is always at the end
			needle := append([]byte(nil), haystack[h-n:]...)
			want := int64(bytes.Index(haystack, needle))

			b.Run(fmt.Sprintf("needleLen=%d/haystackSize=%d/impl=avx2", n, h), func(b *testing.B) {
				if !cpu.X86.HasAVX2 {
					b.Skipf("platform does not support AVX2")
				}
				if !SIMDactive {
					b.Skipf("built without GOEXPERIMENT=simd")
				}

				b.SetBytes(int64(len(haystack)))
				for b.Loop() {
					if Index(haystack, needle) != want {
						b.Fatal("wrong index")
					}
				}
			})

			b.Run(fmt.Sprintf("needleLen=%d/haystackSize=%d/impl=stdlib", n, h), func(b *testing.B) {
				b.SetBytes(int64(len(haystack)))
				for b.Loop() {
					if int64(bytes.Index(haystack, needle)) != want {
						b.Fatal("wrong index")
					}
				}
			})
		}
	}
}
