package checks

import (
	"slices"
	"strings"

	globutil "rev-dep-go/internal/glob"
	"rev-dep-go/internal/rules"
)

// ModuleBoundaryViolation represents a module boundary violation
type ModuleBoundaryViolation struct {
	FilePath      string
	ImportPath    string
	ImportRequest string
	RuleName      string
	ViolationType string // "denied" or "not_allowed"
}

// CheckModuleBoundariesFromTree checks for module boundary violations using a pre-built dependency tree
func CheckModuleBoundariesFromTree(
	minimalTree MinimalDependencyTree,
	files []string,
	boundaries []rules.BoundaryRule,
	cwd string,
) []ModuleBoundaryViolation {
	var violations []ModuleBoundaryViolation

	// Compile matchers for all boundaries
	type CompiledBoundary struct {
		Rule            rules.BoundaryRule
		PatternMatchers []globutil.GlobMatcher
		AllowMatchers   []globutil.GlobMatcher
		DenyMatchers    []globutil.GlobMatcher
	}

	compiledBoundaries := make([]CompiledBoundary, 0, len(boundaries))
	for _, boundary := range boundaries {
		cb := CompiledBoundary{
			Rule:            boundary,
			PatternMatchers: globutil.CreateGlobMatchers([]string{boundary.Pattern}, cwd),
			AllowMatchers:   globutil.CreateGlobMatchers(boundary.Allow, cwd),
			DenyMatchers:    globutil.CreateGlobMatchers(boundary.Deny, cwd),
		}
		compiledBoundaries = append(compiledBoundaries, cb)
	}

	// Check violations
	for _, filePath := range files {
		// Find which boundaries apply to this file
		for _, boundary := range compiledBoundaries {
			if globutil.MatchesAnyGlobMatcher(filePath, boundary.PatternMatchers, false) {
				// Check dependencies
				fileDeps, ok := minimalTree[filePath]
				if !ok {
					continue
				}

				for _, dep := range fileDeps {
					if dep.ID != "" && (dep.ResolvedType == UserModule || dep.ResolvedType == MonorepoModule) {
						resolvedPath := dep.ID

						// Check if denied
						if len(boundary.DenyMatchers) > 0 && globutil.MatchesAnyGlobMatcher(resolvedPath, boundary.DenyMatchers, false) {
							violations = append(violations, ModuleBoundaryViolation{
								FilePath:      filePath,
								ImportPath:    resolvedPath,
								ImportRequest: dep.Request,
								RuleName:      boundary.Rule.Name,
								ViolationType: "denied",
							})
							continue
						}

						// Check if allowed
						if len(boundary.AllowMatchers) > 0 {
							if !globutil.MatchesAnyGlobMatcher(resolvedPath, boundary.AllowMatchers, false) {
								violations = append(violations, ModuleBoundaryViolation{
									FilePath:      filePath,
									ImportPath:    resolvedPath,
									ImportRequest: dep.Request,
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

	// Sort violations for consistent output
	slices.SortFunc(violations, func(a, b ModuleBoundaryViolation) int {
		if a.FilePath != b.FilePath {
			return strings.Compare(a.FilePath, b.FilePath)
		}
		return strings.Compare(a.ImportPath, b.ImportPath)
	})

	return violations
}
