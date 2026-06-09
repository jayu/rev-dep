package main

import (
	"os"

	"rev-dep-go/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		// cli.Execute (cobra) already prints the error to stderr. Exit with a
		// non-zero code so CI and the npm wrapper detect the failure and surface
		// the message — a bare `return` here exits 0, which made wrappers treat
		// failed runs (e.g. an invalid config) as success and swallow the output.
		os.Exit(1)
	}
}
