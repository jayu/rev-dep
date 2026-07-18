package diag

import (
	"fmt"

	"rev-dep-go/internal/emoji"
)

var verbose bool

func SetVerbose(v bool) {
	verbose = v
}

func Warnf(format string, args ...interface{}) {
	if verbose {
		fmt.Printf(emoji.Warning+"  Warning: "+format+"\n", args...)
	}
}
