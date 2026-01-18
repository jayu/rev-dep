package main

// ModuleBoundaryViolation represents a module boundary violation
type ModuleBoundaryViolation struct {
	FilePath      string
	ImportPath    string
	RuleName      string
	ViolationType string // "denied" or "not_allowed"
}

// CheckModuleBoundariesFromTree checks for module boundary violations using a pre-built dependency tree
func CheckModuleBoundariesFromTree(
	minimalTree MinimalDependencyTree,
	files []string,
	boundaries []BoundaryRule,
	cwd string,
) []ModuleBoundaryViolation {
	var violations []ModuleBoundaryViolation

	// Compile matchers for all boundaries
	type CompiledBoundary struct {
		Rule            BoundaryRule
		PatternMatchers []GlobMatcher
		AllowMatchers   []GlobMatcher
		DenyMatchers    []GlobMatcher
	}

	compiledBoundaries := make([]CompiledBoundary, 0, len(boundaries))
	for _, boundary := range boundaries {
		cb := CompiledBoundary{
			Rule:            boundary,
			PatternMatchers: CreateGlobMatchers([]string{boundary.Pattern}, cwd),
			AllowMatchers:   CreateGlobMatchers(boundary.Allow, cwd),
			DenyMatchers:    CreateGlobMatchers(boundary.Deny, cwd),
		}
		compiledBoundaries = append(compiledBoundaries, cb)
	}

	// Check violations
	for _, filePath := range files {
		// Find which boundaries apply to this file
		for _, boundary := range compiledBoundaries {
			if MatchesAnyGlobMatcher(filePath, boundary.PatternMatchers, false) {
				// Check dependencies
				fileDeps, ok := minimalTree[filePath]
				if !ok {
					continue
				}

				for _, dep := range fileDeps {
					if dep.ID != nil && (dep.ResolvedType == UserModule || dep.ResolvedType == MonorepoModule) {
						resolvedPath := *dep.ID

						// Check if denied
						if len(boundary.DenyMatchers) > 0 && MatchesAnyGlobMatcher(resolvedPath, boundary.DenyMatchers, false) {
							violations = append(violations, ModuleBoundaryViolation{
								FilePath:      filePath,
								ImportPath:    resolvedPath,
								RuleName:      boundary.Rule.Name,
								ViolationType: "denied",
							})
							continue
						}

						// Check if allowed
						if len(boundary.AllowMatchers) > 0 {
							if !MatchesAnyGlobMatcher(resolvedPath, boundary.AllowMatchers, false) {
								violations = append(violations, ModuleBoundaryViolation{
									FilePath:      filePath,
									ImportPath:    resolvedPath,
									RuleName:      boundary.Rule.Name,
									ViolationType: "not_allowed",
								})
							}
						}
					}
				}
			}
		}
	}

	return violations
}
