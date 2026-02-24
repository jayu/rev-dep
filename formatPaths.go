package main

import (
	"fmt"
	"strings"
)

// formatPaths formats the resolution paths in a nested tree structure
func FormatPaths(paths [][]string, pathPrefix string) {
	FormatPathsWithAdditionalItem(paths, pathPrefix, "")
}

func FormatPathsWithAdditionalItem(paths [][]string, pathPrefix, terminal string) {
	if len(paths) == 0 {
		fmt.Println("No paths found.")
		return
	}

	// Remove base path prefix from all paths for cleaner output
	cleanPaths := make([][]string, len(paths))
	for i, path := range paths {
		cleanPath := make([]string, len(path))
		for j, p := range path {
			// Remove the base path prefix to make output cleaner
			if strings.HasPrefix(p, pathPrefix) {
				cleanPath[j] = strings.TrimPrefix(p, pathPrefix)
			} else {
				cleanPath[j] = p
			}
		}
		cleanPaths[i] = cleanPath
	}

	// Print header
	if len(cleanPaths) > 0 && len(cleanPaths[0]) > 0 {
		fmt.Printf("%s (%d):\n\n", cleanPaths[0][0], len(cleanPaths))
	}

	// Print each path with proper nesting
	for i, path := range cleanPaths {
		fmt.Printf("Path %d:\n", i+1)
		for depth, file := range path {
			indent := strings.Repeat(" ", depth)
			fmt.Printf("%s ➞ %s\n", indent, file)
		}
		if terminal != "" {
			indent := strings.Repeat(" ", len(path))
			fmt.Printf("%s ➞ %s\n", indent, terminal)
		}
		fmt.Println()
	}

}
