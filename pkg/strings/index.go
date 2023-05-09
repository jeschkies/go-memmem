package strings

import (
	"strings"

	"github.com/jeschkies/go-memmem/pkg/strings/simd"
	"golang.org/x/sys/cpu"
)

var useSIMD bool

func init() {
	useSIMD = cpu.X86.HasAVX && cpu.X86.HasAVX2 && cpu.X86.HasBMI1
}

func Index(s, substr string) int {
	if useSIMD {
		return int(simd.Index([]byte(s), []byte(substr)))
	}

	return strings.Index(s, substr)
}
