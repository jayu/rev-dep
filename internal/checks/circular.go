package checks

import (
	"fmt"
	"sort"
	"strings"

	"rev-dep-go/internal/emoji"
)

// FindCircularDependencies detects circular dependencies using strongly connected components (SCCs).
// It returns one deterministic cycle representation per SCC.
func FindCircularDependencies(deps MinimalDependencyTree, sortedFilesList []string, ignoreTypeImports bool) [][]string {
	nodeSet := make(map[string]struct{}, len(sortedFilesList))
	for _, node := range sortedFilesList {
		nodeSet[node] = struct{}{}
	}

	adj := make(map[string][]string, len(sortedFilesList))
	for _, node := range sortedFilesList {
		if nodeDeps, exists := deps[node]; exists && nodeDeps != nil {
			for _, dep := range nodeDeps {
				if dep.ID == "" {
					continue
				}
				if ignoreTypeImports && dep.ImportKind == OnlyTypeImport {
					continue
				}
				if _, ok := nodeSet[dep.ID]; !ok {
					continue
				}
				adj[node] = append(adj[node], dep.ID)
			}
			sort.Strings(adj[node])
		}
	}

	index := 0
	indices := make(map[string]int, len(sortedFilesList))
	lowlink := make(map[string]int, len(sortedFilesList))
	onStack := make(map[string]bool, len(sortedFilesList))
	stack := make([]string, 0, len(sortedFilesList))
	sccs := make([][]string, 0, 16)

	var strongconnect func(v string)
	strongconnect = func(v string) {
		indices[v] = index
		lowlink[v] = index
		index++

		stack = append(stack, v)
		onStack[v] = true

		for _, w := range adj[v] {
			if _, ok := indices[w]; !ok {
				strongconnect(w)
				if lowlink[w] < lowlink[v] {
					lowlink[v] = lowlink[w]
				}
			} else if onStack[w] {
				if indices[w] < lowlink[v] {
					lowlink[v] = indices[w]
				}
			}
		}

		if lowlink[v] == indices[v] {
			scc := make([]string, 0, 8)
			for {
				n := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				onStack[n] = false
				scc = append(scc, n)
				if n == v {
					break
				}
			}
			sccs = append(sccs, scc)
		}
	}

	for _, v := range sortedFilesList {
		if _, ok := indices[v]; !ok {
			strongconnect(v)
		}
	}

	cycles := make([][]string, 0, len(sccs))
	for _, scc := range sccs {
		if len(scc) == 1 {
			if !hasSelfLoop(adj, scc[0]) {
				continue
			}
		}

		sort.Strings(scc)
		inSCC := make(map[string]struct{}, len(scc))
		for _, n := range scc {
			inSCC[n] = struct{}{}
		}

		start := scc[0]
		cycle := findDeterministicCycle(start, adj, inSCC)
		if len(cycle) == 0 {
			cycle = []string{start, start}
		}
		cycles = append(cycles, cycle)
	}

	sort.Slice(cycles, func(i, j int) bool {
		return strings.Join(cycles[i], "\x00") < strings.Join(cycles[j], "\x00")
	})

	return cycles
}

func hasSelfLoop(adj map[string][]string, node string) bool {
	for _, dep := range adj[node] {
		if dep == node {
			return true
		}
	}
	return false
}

func findDeterministicCycle(start string, adj map[string][]string, inSCC map[string]struct{}) []string {
	path := []string{start}
	onPath := map[string]bool{start: true}
	var result []string

	var dfs func(node string) bool
	dfs = func(node string) bool {
		for _, next := range adj[node] {
			if _, ok := inSCC[next]; !ok {
				continue
			}
			if next == start {
				result = append(append([]string{}, path...), start)
				return true
			}
			if !onPath[next] {
				onPath[next] = true
				path = append(path, next)
				if dfs(next) {
					return true
				}
				path = path[:len(path)-1]
				onPath[next] = false
			}
		}
		return false
	}

	if dfs(start) {
		return result
	}
	return nil
}

// formatCircularDependencies formats circular dependencies for display
func formatCircularDependencies(cycles [][]string, pathPrefix string, deps MinimalDependencyTree, includeHeader bool, baseIndentation int) string {
	if len(cycles) == 0 {
		if includeHeader {
			return fmt.Sprintln(emoji.Success + " No circular dependencies found! ")
		}
		return ""
	}

	var result string
	if includeHeader {
		result = fmt.Sprintf("Found %d circular dependencies:\n\n", len(cycles))
	}

	for i, cycle := range cycles {
		result += fmt.Sprintf("%sCircular Dependency %d:\n", strings.Repeat(" ", baseIndentation), i+1)
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
						if imp.ID == file {
							request = imp.Request
							break
						}
					}
				}
			}

			indent := strings.Repeat(" ", baseIndentation+j)
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

// FormatCircularDependencies formats circular dependencies with header (for backward compatibility)
func FormatCircularDependencies(cycles [][]string, pathPrefix string, deps MinimalDependencyTree) string {
	return formatCircularDependencies(cycles, pathPrefix, deps, true, 0)
}

// FormatCircularDependenciesWithoutHeader formats circular dependencies without header
func FormatCircularDependenciesWithoutHeader(cycles [][]string, pathPrefix string, deps MinimalDependencyTree, baseIndentation int) string {
	return formatCircularDependencies(cycles, pathPrefix, deps, false, baseIndentation)
}
