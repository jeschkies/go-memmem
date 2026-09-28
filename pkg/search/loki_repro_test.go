package search

import (
	"bytes"
	"math/rand"
	"testing"
)

// TestLokiRepro_ShortHaystackAgreesWithBytesIndex reproduces the stress test
// that found Index disagreeing with bytes.Index in ~8% of trials against the
// tagged v0.1.0 release (used by github.com/grafana/loki's
// pkg/arrowfilter). Line lengths deliberately span both sides of
// LOOP_SIZE_AVX2 (32 bytes), since that's the boundary where the AVX2 tail
// path (inlineMatchRemaining) previously overread past a short haystack's
// end - see commit be6c9df ("Fix AVX2 out-of-bounds reads and inverted
// dispatch") and f9dd68a for the fix this test is confirming.
//
// This test only calls the public Index API - it does not touch
// findInChunk, indexAvx2, or any assembly directly, and makes no changes to
// the implementation.
func TestLokiRepro_ShortHaystackAgreesWithBytesIndex(t *testing.T) {
	r := rand.New(rand.NewSource(2))
	alphabet := []byte("ABCabc GC")
	needle := []byte("GC")

	randHaystack := func(n int) []byte {
		b := make([]byte, n)
		for i := range b {
			b[i] = alphabet[r.Intn(len(alphabet))]
		}
		return b
	}

	var mismatches int
	const trials = 5000
	for trial := range trials {
		// 0-95 bytes: spans well below, right around, and well above the
		// 32-byte LOOP_SIZE_AVX2 threshold.
		haystack := randHaystack(r.Intn(96))

		want := int64(bytes.Index(haystack, needle))
		got := Index(haystack, needle)
		if want != got {
			mismatches++
			t.Logf("trial %d MISMATCH: haystack=%q (len=%d) want=%d got=%d",
				trial, haystack, len(haystack), want, got)
		}
	}

	if mismatches > 0 {
		t.Errorf("Index disagreed with bytes.Index on %d/%d trials (see logs above)", mismatches, trials)
	}
}

// TestLokiRepro_MultiLineBufferAgreesWithNaiveScan reproduces the exact
// failure shape observed in loki's pkg/arrowfilter: many short "lines"
// concatenated into one buffer, repeatedly calling Index on shrinking
// suffixes of that buffer (haystack[pos:]) exactly as a batch substring
// scan does, and comparing against a naive per-line bytes.Contains scan.
func TestLokiRepro_MultiLineBufferAgreesWithNaiveScan(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	alphabet := []byte("ABCabc GC")
	needle := []byte("GC")

	randLine := func(n int) []byte {
		b := make([]byte, n)
		for i := range b {
			b[i] = alphabet[r.Intn(len(alphabet))]
		}
		return b
	}

	var mismatches int
	const trials = 2000
	for trial := range trials {
		nLines := 1 + r.Intn(20)
		lines := make([][]byte, nLines)
		offsets := make([]int, nLines+1)
		var buf []byte
		for i := range lines {
			lines[i] = randLine(r.Intn(12))
			buf = append(buf, lines[i]...)
			offsets[i+1] = len(buf)
		}

		// Naive baseline: does each individual line contain the needle?
		wantRows := map[int]bool{}
		for i, l := range lines {
			if bytes.Contains(l, needle) {
				wantRows[i] = true
			}
		}

		// Batch scan: repeatedly call Index on shrinking suffixes, mapping
		// hits back to rows via offsets, mirroring
		// github.com/grafana/loki/v3/pkg/arrowfilter.Contains's algorithm,
		// including its check that a hit doesn't straddle a row boundary.
		gotRows := map[int]bool{}
		pos, row := 0, 0
		for row < nLines {
			hit := Index(buf[pos:], needle)
			if hit == -1 {
				break
			}
			hitStart := pos + int(hit)
			hitEnd := hitStart + len(needle)

			for row < nLines && offsets[row+1] <= hitStart {
				row++
			}
			if row >= nLines {
				break
			}

			rowEnd := offsets[row+1]
			if hitEnd <= rowEnd {
				gotRows[row] = true
				pos, row = rowEnd, row+1
				continue
			}
			pos = hitStart + 1
		}

		if len(wantRows) != len(gotRows) {
			mismatches++
			t.Logf("trial %d MISMATCH: lines=%q want=%v got=%v", trial, lines, wantRows, gotRows)
			continue
		}
		for k := range wantRows {
			if !gotRows[k] {
				mismatches++
				t.Logf("trial %d MISMATCH: lines=%q want=%v got=%v", trial, lines, wantRows, gotRows)
				break
			}
		}
	}

	if mismatches > 0 {
		t.Errorf("batch scan disagreed with naive per-line scan on %d/%d trials (see logs above)", mismatches, trials)
	}
}
