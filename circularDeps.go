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

	var dfs func(node string, path []string) bool
	dfs = func(node string, path []string) bool {
		visited[node] = true
		recStack[node] = true

		// Add current node to path
		currentPath := append(path, node)

		// Check dependencies
		if nodeDeps, exists := deps[node]; exists && nodeDeps != nil {
			for _, dep := range nodeDeps {
				if dep.ID == nil || *dep.ID == "" {
					continue
				}

				depPath := *dep.ID

				// Check if this dependency is in our recursion stack (cycle found)
				if recStack[depPath] {
					// Found a cycle - extract the cycle from the path
					cycleStart := -1
					for i, pathNode := range currentPath {
						if pathNode == depPath {
							cycleStart = i
							break
						}
					}
					if cycleStart >= 0 {
						cycle := make([]string, len(currentPath)-cycleStart+1)
						copy(cycle, currentPath[cycleStart:])
						cycle[len(cycle)-1] = depPath // Close the cycle
						cycles = append(cycles, cycle)
					}
					continue
				}

				// If not visited, continue DFS
				if !visited[depPath] {
					dfs(depPath, currentPath)
				}
			}
		}

		recStack[node] = false
		return false
	}

	// Run DFS from all unvisited nodes
	for _, node := range sortedFilesList {
		if !visited[node] {
			dfs(node, []string{})
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
				for _, imp := range deps[cycle[j-1]] {
					if *imp.ID == file {
						request = imp.Request
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

func deduplicateStringArrays(arr [][]string) [][]string {
	entries := map[string]byte{}
	result := [][]string{}

	for _, arrNested := range arr {
		key := strings.Join(arrNested, ",")
		_, has := entries[key]
		if !has {
			result = append(result, arrNested)
			entries[key] = 0
		}
	}
	return result
}
