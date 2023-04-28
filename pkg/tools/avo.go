//nolint
package tools 

// Declare dependency on avo so that `go mod tidy` won't ignore it.
import (
	_ "github.com/mmcloughlin/avo/build"
	_ "github.com/mmcloughlin/avo/operand"
)
