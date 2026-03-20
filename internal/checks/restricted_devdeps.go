package checks

import (
	"slices"

	globutil "rev-dep-go/internal/glob"
	"rev-dep-go/internal/graph"
	"rev-dep-go/internal/module"
)

// RestrictedDevDependenciesUsageViolation represents a violation where a dev dependency is used in production code
type RestrictedDevDependenciesUsageViolation struct {
	DevDependency string `json:"devDependency"`
	FilePath      string `json:"filePath"`
	EntryPoint    string `json:"entryPoint"`
}

// FindDevDependenciesInProduction detects when dev dependencies are used in production entry points
func FindDevDependenciesInProduction(
	ruleTree MinimalDependencyTree,
	validEntryPoints []string,
	ignoreTypeImports bool,
	rulePath string,
	devDependencies map[string]bool,
) []RestrictedDevDependenciesUsageViolation {
	if len(validEntryPoints) == 0 {
		return []RestrictedDevDependenciesUsageViolation{}
	}

	// Create glob matchers for valid entry points
	entryPointGlobs := globutil.CreateGlobMatchers(validEntryPoints, rulePath)

	// Build reachable files map from entry points (like orphan files does)
	prodEntryPoints := []string{}

	// First pass: mark entry points as reachable
	for filePath := range ruleTree {
		if globutil.MatchesAnyGlobMatcher(filePath, entryPointGlobs, false) {
			prodEntryPoints = append(prodEntryPoints, filePath)
		}
	}

	slices.Sort(prodEntryPoints) // ensure deterministic results

	graph := graph.BuildDepsGraphForMultiple(ruleTree, prodEntryPoints, nil, false, ignoreTypeImports)

	var violations []RestrictedDevDependenciesUsageViolation

	// Check each reachable file for dev dependency usage
	for filePath, vertex := range graph.Vertices {
		for _, moduleRequest := range vertex.Modules {
			moduleName := module.GetNodeModuleName(moduleRequest)
			if moduleName == "" {
				continue
			}

			if devDependencies[moduleName] {
				entryPoint := FollowPathToGetEntryPoint(vertex, graph)

				violations = append(violations, RestrictedDevDependenciesUsageViolation{
					DevDependency: moduleName,
					FilePath:      filePath,
					EntryPoint:    entryPoint,
				})
			}
		}
	}

	return violations
}

func FollowPathToGetEntryPoint(vertex *graph.SerializableNode, graph graph.BuildDepsGraphResultMultiple) string {
	currentVertex := vertex
	for currentVertex != nil {
		if len(currentVertex.Parents) == 0 {
			return currentVertex.Path
		}
		// We assume only one path was resolved, so we take the first parent
		parentPath := currentVertex.Parents[0]
		currentVertex = graph.Vertices[parentPath]
	}

	return ""
}
