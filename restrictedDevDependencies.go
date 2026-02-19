package main

// FindDevDependenciesInProduction detects when dev dependencies are used in production entry points
func FindDevDependenciesInProduction(
	ruleTree MinimalDependencyTree,
	validEntryPoints []string,
	rulePath string,
	monorepoContext *MonorepoContext,
) []RestrictedDevDependenciesUsageViolation {
	if len(validEntryPoints) == 0 {
		return []RestrictedDevDependenciesUsageViolation{}
	}

	// Create glob matchers for valid entry points
	entryPointGlobs := CreateGlobMatchers(validEntryPoints, rulePath)

	// Build reachable files map from entry points (like orphan files does)
	prodEntryPoints := []string{}

	// First pass: mark entry points as reachable
	for filePath := range ruleTree {
		if MatchesAnyGlobMatcher(filePath, entryPointGlobs, false) {
			prodEntryPoints = append(prodEntryPoints, filePath)
		}
	}

	graph := buildDepsGraphForMultiple(ruleTree, prodEntryPoints, nil, false)

	// Get dev dependencies from package.json in rule path
	devDependencies := make(map[string]bool)
	if monorepoContext != nil {
		if config, err := monorepoContext.GetPackageConfig(rulePath); err == nil {
			devDependencies = GetDevDependenciesFromConfig(config)
		}
	}

	var violations []RestrictedDevDependenciesUsageViolation

	// Check each reachable file for dev dependency usage
	for filePath, vertex := range graph.Vertices {

		// Get imports for this file
		if dependencies, exists := ruleTree[filePath]; exists {
			for _, dep := range dependencies {

				// Skip type-only imports as they don't affect production runtime
				if dep.ImportKind == OnlyTypeImport {
					continue
				}

				// Check if the imported module is a dev dependency
				moduleName := GetNodeModuleName(dep.Request)
				if moduleName == "" {
					continue // Not a node module
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
	}

	return violations
}

func FollowPathToGetEntryPoint(vertex *SerializableNode, graph BuildDepsGraphResultMultiple) string {
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
