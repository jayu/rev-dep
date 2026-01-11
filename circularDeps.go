package main

import (
	"fmt"
	"strings"
)

// findCircularDependencies detects circular dependencies in the dependency tree
func FindCircularDependencies(deps MinimalDependencyTree, sortedFilesList []string) [][]string {
	var cycles [][]string
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	// Use shared path slice to avoid copying
	path := make([]string, 0, 64)

	var dfs func(node string) bool
	dfs = func(node string) bool {
		visited[node] = true
		recStack[node] = true

		// Add current node to path
		path = append(path, node)

		// Check dependencies
		if nodeDeps, exists := deps[node]; exists && nodeDeps != nil {
			for _, dep := range nodeDeps {
				if dep.ID == nil || *dep.ID == "" {
					continue
				}

				depPath := *dep.ID

				// Check if this dependency is in our recursion stack (cycle found)
				if recStack[depPath] {
					// Found a cycle - extract the cycle
					cycleStart := -1
					for i := len(path) - 1; i >= 0; i-- {
						if path[i] == depPath {
							cycleStart = i
							break
						}
					}
					if cycleStart >= 0 {
						cycle := make([]string, len(path)-cycleStart+1)
						copy(cycle, path[cycleStart:])
						cycle[len(cycle)-1] = depPath // Close the cycle
						cycles = append(cycles, cycle)
					}
					continue
				}

				// If not visited, continue DFS
				if !visited[depPath] {
					dfs(depPath)
				}
			}
		}

		// Remove current node from path when backtracking
		path = path[:len(path)-1]

		recStack[node] = false
		return false
	}

	// Run DFS from all unvisited nodes
	for _, node := range sortedFilesList {
		if !visited[node] {
			dfs(node)
		}
	}

	// Circular deps check is unstable, it returns different count, sometimes different by 10x, even after deduplication
	return deduplicateStringArrays(cycles)
}

// formatCircularDependencies formats circular dependencies for display
func FormatCircularDependencies(cycles [][]string, pathPrefix string, deps MinimalDependencyTree) string {
	if len(cycles) == 0 {
		return fmt.Sprintln("No circular dependencies found! ✅")
	}

	result := fmt.Sprintf("Found %d circular dependencies:\n\n", len(cycles))

	for i, cycle := range cycles {
		result += fmt.Sprintf("Circular Dependency %d:\n", i+1)
		for j, file := range cycle {
			// Clean the path
			cleanPath := file
			if strings.HasPrefix(file, pathPrefix) {
				cleanPath = strings.TrimPrefix(file, pathPrefix)
			}

			request := ""
			if j > 0 {
				// Keep linear search for small dependency lists
				if nodeDeps, exists := deps[cycle[j-1]]; exists {
					for _, imp := range nodeDeps {
						if imp.ID != nil && *imp.ID == file {
							request = imp.Request
							break
						}
					}
				}
			}

			indent := strings.Repeat(" ", j)
			if j == 0 {
				result += fmt.Sprintf("%s ➞ %s (cycle start)\n", indent, cleanPath)
			} else {
				result += fmt.Sprintf("%s ➞ %s ('%s')\n", indent, cleanPath, request)
			}
		}
		result += fmt.Sprintln()
	}
	return result
}

// deduplicateStringArrays deduplicates cycles
func deduplicateStringArrays(arr [][]string) [][]string {
	entries := make(map[string]struct{}, len(arr))
	result := make([][]string, 0, len(arr))

	for _, arrNested := range arr {
		key := strings.Join(arrNested, ",")
		if _, exists := entries[key]; !exists {
			result = append(result, arrNested)
			entries[key] = struct{}{}
		}
	}
	return result
}
