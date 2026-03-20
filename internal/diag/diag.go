package diag

import "fmt"

var verbose bool

func SetVerbose(v bool) {
	verbose = v
}

func Warnf(format string, args ...interface{}) {
	if verbose {
		fmt.Printf("⚠️  Warning: "+format+"\n", args...)
	}
}
