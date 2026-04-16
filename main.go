package main

import "rev-dep-go/internal/cli"

func main() {
	if err := cli.Execute(); err != nil {
		// cli.Execute already prints errors when appropriate
		return
	}
}
