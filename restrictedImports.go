package main

import (
	"slices"
	"strings"

	"github.com/gobwas/glob"
)

type RestrictedImportViolation struct {
	ViolationType string `json:"violationType"`
	ImporterFile  string `json:"importerFile"`
	EntryPoint    string `json:"entryPoint"`
	DeniedFile    string `json:"deniedFile,omitempty"`
	DeniedModule  string `json:"deniedModule,omitempty"`
	ImportRequest string `json:"importRequest,omitempty"`
}

// FindRestrictedImports checks dependency graph reachable from configured entry points
// and reports files/modules matching deny patterns.
func FindRestrictedImports(
	ruleTree MinimalDependencyTree,
	opts *RestrictedImportsDetectionOptions,
	rulePath string,
) []RestrictedImportViolation {
	if opts == nil || !opts.Enabled {
		return []RestrictedImportViolation{}
	}
	if len(opts.EntryPoints) == 0 || (len(opts.DenyFiles) == 0 && len(opts.DenyModules) == 0) {
		return []RestrictedImportViolation{}
	}

	entryPointMatchers := CreateGlobMatchers(opts.EntryPoints, rulePath)
	entryPoints := []string{}
	entryPointSet := map[string]bool{}

	for filePath := range ruleTree {
		if MatchesAnyGlobMatcher(filePath, entryPointMatchers, false) {
			entryPoints = append(entryPoints, filePath)
			entryPointSet[filePath] = true
		}
	}

	if len(entryPoints) == 0 {
		return []RestrictedImportViolation{}
	}

	slices.Sort(entryPoints) // ensure deterministic results

	graph := buildDepsGraphForMultiple(ruleTree, entryPoints, nil, false, opts.IgnoreTypeImports)

	denyFileMatchers := CreateGlobMatchers(opts.DenyFiles, rulePath)
	ignoreMatchers := CreateGlobMatchers(opts.IgnoreMatches, rulePath)
	denyModuleMatchers := compileModuleGlobMatchers(opts.DenyModules)

	violations := []RestrictedImportViolation{}
	seen := map[string]bool{}

	sortedFilePaths := make([]string, 0, len(graph.Vertices))
	for filePath := range graph.Vertices {
		sortedFilePaths = append(sortedFilePaths, filePath)
	}
	slices.Sort(sortedFilePaths)

	for _, filePath := range sortedFilePaths {
		vertex := graph.Vertices[filePath]
		entryPoint := ""
		getEntryPoint := func() string {
			if entryPoint == "" {
				entryPoint = FollowPathToGetEntryPoint(vertex, graph)
			}
			return entryPoint
		}

		if len(denyFileMatchers) > 0 &&
			!entryPointSet[filePath] &&
			MatchesAnyGlobMatcher(filePath, denyFileMatchers, false) &&
			!matchesIgnoredPattern(filePath, ignoreMatchers) {

			importerFile := ""
				if len(vertex.Parents) > 0 {
					importerFile = vertex.Parents[0]
				}
				entryPoint := getEntryPoint()

				dedupeKey := "file|" + entryPoint + "|" + filePath
				if !seen[dedupeKey] {
					violations = append(violations, RestrictedImportViolation{
						ViolationType: "file",
						ImporterFile:  importerFile,
						EntryPoint:    entryPoint,
					DeniedFile:    filePath,
				})
				seen[dedupeKey] = true
			}
		}

		if len(denyModuleMatchers) == 0 {
			continue
		}

		for _, moduleRequest := range vertex.Modules {
			moduleName := GetNodeModuleName(moduleRequest)
			if moduleName == "" || !isValidNodeModuleName(moduleName) {
				continue
			}
			if !matchesAnyModulePattern(denyModuleMatchers, moduleName, moduleRequest) {
				continue
			}
				if matchesIgnoredPattern(moduleName, ignoreMatchers) ||
					matchesIgnoredPattern(moduleRequest, ignoreMatchers) {
				continue
				}

				entryPoint := getEntryPoint()
				item := moduleRequest
				if item == "" {
					item = moduleName
				}
				dedupeKey := "module|" + entryPoint + "|" + item
				if !seen[dedupeKey] {
					violations = append(violations, RestrictedImportViolation{
						ViolationType: "module",
						ImporterFile:  filePath,
					EntryPoint:    entryPoint,
					DeniedModule:  moduleName,
					ImportRequest: moduleRequest,
				})
				seen[dedupeKey] = true
			}
		}
	}

	slices.SortFunc(violations, func(a, b RestrictedImportViolation) int {
		if a.ViolationType != b.ViolationType {
			return strings.Compare(a.ViolationType, b.ViolationType)
		}
		if a.ImporterFile != b.ImporterFile {
			return strings.Compare(a.ImporterFile, b.ImporterFile)
		}
		if a.DeniedFile != b.DeniedFile {
			return strings.Compare(a.DeniedFile, b.DeniedFile)
		}
		if a.DeniedModule != b.DeniedModule {
			return strings.Compare(a.DeniedModule, b.DeniedModule)
		}
		return strings.Compare(a.ImportRequest, b.ImportRequest)
	})

	return violations
}

func compileModuleGlobMatchers(patterns []string) []glob.Glob {
	matchers := make([]glob.Glob, 0, len(patterns))
	for _, pattern := range patterns {
		trimmed := strings.TrimSpace(pattern)
		if trimmed == "" {
			continue
		}
		matcher, err := glob.Compile(trimmed)
		if err != nil {
			continue
		}
		matchers = append(matchers, matcher)
	}
	return matchers
}

func matchesAnyModulePattern(matchers []glob.Glob, moduleName string, request string) bool {
	for _, matcher := range matchers {
		if matcher.Match(moduleName) || matcher.Match(request) {
			return true
		}
	}
	return false
}

func matchesIgnoredPattern(candidate string, ignoreMatchers []GlobMatcher) bool {
	return len(ignoreMatchers) > 0 && MatchesAnyGlobMatcher(candidate, ignoreMatchers, false)
}
