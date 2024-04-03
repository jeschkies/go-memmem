package search

import (
	"archive/zip"
	//"bytes"
	"fmt"
	"io"
	"log"
	"testing"

	"github.com/stretchr/testify/require"
)

//go:generate go run asm.go -out bytes.s -stubs bytes.go

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
	} {
		tt := tt
		t.Run(fmt.Sprintf("`%s` in `%s`", tt.needle, tt.haystack), func(t *testing.T) {
			i := Index(tt.haystack, tt.needle)
			require.Equal(t, tt.index, i)
		})
	}
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
		tt := tt
		t.Run(fmt.Sprintf(tt.name), func(t *testing.T) {
			index := findInChunk(tt.needle, tt.haystack)
			require.Equal(t, tt.index, index)
			if index != -1 {
				end := index + int64(len(tt.needle))
				require.ElementsMatch(t, tt.needle, tt.haystack[index:end])
			}
		})
	}
}

func BenchmarkIndexSmall(b *testing.B) {
	needle := []byte("goldner7875")
	haystack, err := loadHaystack("small.log")
	if err != nil {
		log.Fatalf(`msg="could not open log file" err=%s`, err)
		b.Fail()
	}
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		//i := bytes.Index(haystack, needle)
		i := Index(haystack, needle)
		if i == -1 {
			b.Fail()
		}
		b.SetBytes(int64(i)+int64(len(needle)))
	}
}

func BenchmarkIndexBig(b *testing.B) {
	//needle := []byte("breitenberg1265")
	needle := []byte(`77.47.98.232 - - [02/May/2023:10:20:14 +0000] "GET /empower/e-business/whiteboard`)
	haystack, err := loadHaystack("big.log")
	if err != nil {
		log.Fatalf(`msg="could not open log file" err=%s`, err)
		b.Fail()
	}
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		//i := bytes.Index(haystack, needle)
		i := Index(haystack, needle)
		if i == -1 {
			b.Fail()
		}
		b.SetBytes(int64(i)+int64(len(needle)))
	}
}

func loadHaystack(name string) ([]byte, error) {
	r, err := zip.OpenReader("data.zip")
	if err != nil {
		return nil, err
	}
	defer r.Close()

	f, err := r.Open(name)
	if err != nil {
		return nil, err
	}
	return io.ReadAll(f)
}
